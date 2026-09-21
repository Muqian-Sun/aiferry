//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// classifyFailoverExhausted 的分类顺序是三个聊天入站共用的：每一行对应一条分支，
// 断言 Kind / 状态码 / 错误类型 / 文案，以及只有 OpenAI 模型不存在 400 会原样写出。
func TestClassifyFailoverExhausted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	teapot := http.StatusTeapot
	customMessage := "rule says hi"
	passthrough := service.NewErrorPassthroughService(staticErrorPassthroughRuleRepo{rules: []*model.ErrorPassthroughRule{{
		ID:            1,
		Name:          "keyword-rule",
		Enabled:       true,
		Priority:      1,
		Keywords:      []string{"quota-keyword"},
		MatchMode:     model.MatchModeAny,
		Platforms:     []string{service.PlatformOpenAI},
		ResponseCode:  &teapot,
		CustomMessage: &customMessage,
	}}}, nil)

	tests := []struct {
		name        string
		err         *service.UpstreamFailoverError
		opts        exhaustedClassifyOptions
		wantKind    string
		wantStatus  int
		wantErrType string
		wantMessage string
		wantWritten bool
		wantRetry   string
	}{
		{
			name:        "nil error maps to 502",
			err:         nil,
			wantKind:    "mapped",
			wantStatus:  http.StatusBadGateway,
			wantErrType: "upstream_error",
			wantMessage: "Upstream service temporarily unavailable",
		},
		{
			name:        "request body too large",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusRequestEntityTooLarge, Reason: service.GatewayFailureReason("openai_request_body_too_large")},
			wantKind:    "body_too_large",
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantErrType: "invalid_request_error",
			wantMessage: service.OpenAIRequestBodyTooLargeClientMessage,
		},
		{
			name:        "http continuation unsupported keeps client message",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusBadRequest, Reason: service.OpenAIHTTPContinuationUnsupportedReason, ClientMessage: "previous_response_id requires X"},
			wantKind:    "continuation_unsupported",
			wantStatus:  http.StatusBadRequest,
			wantErrType: "invalid_request_error",
			wantMessage: "previous_response_id requires X",
		},
		{
			name:        "credential failure",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusUnauthorized, Stage: service.GatewayFailureStageAccountAuth, Reason: service.GatewayFailureReason("grok_oauth_revoked")},
			wantKind:    "credential",
			wantStatus:  http.StatusServiceUnavailable,
			wantErrType: "upstream_error",
			wantMessage: service.GrokCredentialUnavailableClientMessage,
		},
		{
			name: "capacity shed keeps typed client fields and Retry-After",
			err: &service.UpstreamFailoverError{
				StatusCode:             http.StatusServiceUnavailable,
				ResponseHeaders:        http.Header{"Retry-After": []string{"7"}},
				ResponseBody:           []byte(`{"error":{"code":"server_is_overloaded","message":"busy"}}`),
				RequestScopedTransient: true,
				ClientStatusCode:       http.StatusServiceUnavailable,
				ClientMessage:          "Upstream is shedding load",
			},
			wantKind:    "capacity_shed",
			wantStatus:  http.StatusServiceUnavailable,
			wantErrType: "server_error",
			wantMessage: "Upstream is shedding load",
			wantRetry:   "7",
		},
		{
			name:        "openai model_not_found 400 written raw",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusBadRequest, ResponseBody: []byte(`{"error":{"type":"invalid_request_error","code":"model_not_found","message":"The model is unavailable"}}`)},
			opts:        exhaustedClassifyOptions{RawUpstream400: true},
			wantKind:    "raw_upstream_400",
			wantStatus:  http.StatusBadRequest,
			wantErrType: "invalid_request_error",
			wantMessage: "The model is unavailable",
			wantWritten: true,
		},
		{
			name:        "openai model_not_found 400 after stream started falls to mapping",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusBadRequest, ResponseBody: []byte(`{"error":{"code":"model_not_found","message":"The model is unavailable"}}`)},
			opts:        exhaustedClassifyOptions{RawUpstream400: true, StreamStarted: true},
			wantKind:    "mapped",
			wantStatus:  http.StatusBadGateway,
			wantErrType: "upstream_error",
			wantMessage: "Upstream request failed",
		},
		{
			name:        "anthropic-shaped inbound never writes raw 400",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusBadRequest, ResponseBody: []byte(`{"error":{"code":"model_not_found","message":"The model is unavailable"}}`)},
			opts:        exhaustedClassifyOptions{RawUpstream400: false},
			wantKind:    "mapped",
			wantStatus:  http.StatusBadGateway,
			wantErrType: "upstream_error",
			wantMessage: "Upstream request failed",
		},
		{
			name:        "silent refusal",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusOK, ResponseBody: []byte(`{"error":{"code":"openai_silent_refusal","message":"x"}}`)},
			wantKind:    "silent_refusal",
			wantStatus:  http.StatusBadGateway,
			wantErrType: "upstream_error",
			wantMessage: service.OpenAISilentRefusalClientMessage(),
		},
		{
			name:        "passthrough rule by platform",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusInternalServerError, ResponseBody: []byte(`{"error":{"message":"quota-keyword hit"}}`)},
			opts:        exhaustedClassifyOptions{Platform: service.PlatformOpenAI, Passthrough: passthrough},
			wantKind:    "passthrough_rule",
			wantStatus:  http.StatusTeapot,
			wantErrType: "upstream_error",
			wantMessage: customMessage,
		},
		{
			name:        "passthrough rule skipped on other platform",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusInternalServerError, ResponseBody: []byte(`{"error":{"message":"quota-keyword hit"}}`)},
			opts:        exhaustedClassifyOptions{Platform: service.PlatformAnthropic, Passthrough: passthrough},
			wantKind:    "mapped",
			wantStatus:  http.StatusBadGateway,
			wantErrType: "upstream_error",
			wantMessage: "Upstream service temporarily unavailable",
		},
		{
			name:        "default mapping 429",
			err:         &service.UpstreamFailoverError{StatusCode: http.StatusTooManyRequests, ResponseHeaders: http.Header{"Retry-After": []string{"3"}}, ResponseBody: []byte(`{"error":{"message":"slow down"}}`)},
			wantKind:    "mapped",
			wantStatus:  http.StatusTooManyRequests,
			wantErrType: "rate_limit_error",
			wantMessage: "Upstream rate limit exceeded, please retry later",
			wantRetry:   "3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			opts := tt.opts
			opts.MapUpstream = openAIMapUpstreamError

			resp := classifyFailoverExhausted(c, tt.err, opts)

			require.Equal(t, tt.wantKind, resp.Kind)
			require.Equal(t, tt.wantStatus, resp.Status)
			require.Equal(t, tt.wantErrType, resp.ErrType)
			require.Equal(t, tt.wantMessage, resp.Message)
			require.Equal(t, tt.wantWritten, resp.Written)
			require.Equal(t, tt.wantRetry, rec.Header().Get("Retry-After"))
			if tt.wantWritten {
				require.Equal(t, http.StatusBadRequest, rec.Code)
				require.Equal(t, "model_not_found", gjson.GetBytes(rec.Body.Bytes(), "error.code").String())
			} else {
				require.Zero(t, rec.Body.Len(), "only raw_upstream_400 writes the body itself")
			}
		})
	}
}

// Anthropic 形状的入站把凭据 / 容量降载换成 api_error，其余照 OpenAI 的类型。
func TestFailoverExhaustedResponse_AnthropicErrType(t *testing.T) {
	require.Equal(t, "api_error", failoverExhaustedResponse{Kind: "credential", ErrType: "upstream_error"}.anthropicErrType())
	require.Equal(t, "api_error", failoverExhaustedResponse{Kind: "capacity_shed", ErrType: "server_error"}.anthropicErrType())
	require.Equal(t, "rate_limit_error", failoverExhaustedResponse{Kind: "mapped", ErrType: "rate_limit_error"}.anthropicErrType())
}

// body-signal compact 心跳已把响应头提交为 200 后，Responses 形状的错误必须降级为 response.failed 终止事件，
// 不能再写 JSON 错误体与已提交的 SSE 流交错（#3887）。
func TestGatewayResponsesErrorResponse_CompactKeepaliveCommittedWritesResponseFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointResponses, nil)
	service.MarkOpenAICompactClientStream(c)

	stop := service.StartOpenAICompactSSEKeepalive(c, 5*time.Millisecond)
	defer stop()
	require.Eventually(t, c.Writer.Written, time.Second, time.Millisecond)

	(&GatewayHandler{}).responsesErrorResponse(c, http.StatusServiceUnavailable, "compact_not_supported", "no compact")

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "event: response.failed\n")
	require.Contains(t, w.Body.String(), "compact_not_supported")
	require.NotContains(t, w.Body.String(), `{"error":{"type"`)
}

// 未提交心跳时按 OpenAI Responses 形状写 JSON：error.type + error.message。
func TestGatewayResponsesErrorResponse_JSONShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointResponses, nil)

	(&GatewayHandler{}).responsesErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "bad")

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(w.Body.Bytes(), "error.type").String())
	require.Equal(t, "bad", gjson.GetBytes(w.Body.Bytes(), "error.message").String())
}
