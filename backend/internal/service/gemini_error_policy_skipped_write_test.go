//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ErrorPolicySkipped（池模式）的客户端写出契约（与 OpenAI 网关路径对齐）：
//   - 不可 failover 的 4xx 按上游原始状态码/响应体保真写出，不改写成 5xx；
//   - 可 failover 的状态码一律换号，不透传。
// 渠道级自定义错误码 2026-09-28 P5 已删：库里旧行留着的配置不再把 4xx 改写成 500。
// ---------------------------------------------------------------------------

const geminiSkippedTestUpstreamMsg = "antigravity executor: invalid Gemini function call history"

func geminiSkippedTestUpstreamBody() string {
	return `{"error":{"code":null,"message":"` + geminiSkippedTestUpstreamMsg + `","param":"","type":"invalid_request_error"}}`
}

func newGeminiSkippedWriteService(status int, body string) (*GeminiMessagesCompatService, *geminiCompatHTTPUpstreamStub) {
	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		},
	}
	svc := &GeminiMessagesCompatService{
		httpUpstream:     httpStub,
		cfg:              &config.Config{},
		rateLimitService: NewRateLimitService(&errorPolicyRepoStub{}, nil, &config.Config{}, nil, nil),
	}
	return svc, httpStub
}

func geminiPoolModeAPIKeyAccount() *Account {
	return &Account{
		ID:                700,
		Platform:          PlatformGemini,
		Type:              AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://generativelanguage.googleapis.com"},
		Credentials: map[string]any{
			"api_key":   "test-key",
			"pool_mode": true,
		},
	}
}

func geminiLegacyCustomCodesAPIKeyAccount() *Account {
	return &Account{
		ID:                701,
		Platform:          PlatformGemini,
		Type:              AccountTypeAPIKey,
		ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://generativelanguage.googleapis.com"},
		Credentials:       legacyErrorPolicyCredentials(map[string]any{"api_key": "test-key", "custom_error_codes": []any{float64(429)}}),
	}
}

func newGeminiNativeTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-flash:generateContent", strings.NewReader("{}"))
	return c, rec
}

func TestGeminiForwardNative_PoolModeSkipped400PassthroughRealStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamBody := geminiSkippedTestUpstreamBody()
	svc, _ := newGeminiSkippedWriteService(http.StatusBadRequest, upstreamBody)
	c, rec := newGeminiNativeTestContext(t)

	result, err := svc.ForwardNative(context.Background(), c, geminiPoolModeAPIKeyAccount(),
		"gemini-2.5-flash", "generateContent", false, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "池模式 400 不应换号")
	require.Contains(t, err.Error(), "gemini upstream error: 400")
	require.Equal(t, http.StatusBadRequest, rec.Code, "状态码应保真为上游 400")
	require.Equal(t, upstreamBody, rec.Body.String(), "响应体应原样透传")
}

func TestGeminiForwardNative_PoolModeSkipped503Failover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newGeminiSkippedWriteService(http.StatusServiceUnavailable, `{"error":{"message":"Upstream service temporarily unavailable"}}`)
	c, rec := newGeminiNativeTestContext(t)

	result, err := svc.ForwardNative(context.Background(), c, geminiPoolModeAPIKeyAccount(),
		"gemini-2.5-flash", "generateContent", false, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr), "池模式 503 应换号")
	require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
	require.Zero(t, rec.Body.Len(), "换号场景不应写客户端响应")
}

func TestGeminiForwardNative_LegacyCustomCodes400NotHiddenAs500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamBody := geminiSkippedTestUpstreamBody()
	svc, _ := newGeminiSkippedWriteService(http.StatusBadRequest, upstreamBody)
	c, rec := newGeminiNativeTestContext(t)

	result, err := svc.ForwardNative(context.Background(), c, geminiLegacyCustomCodesAPIKeyAccount(),
		"gemini-2.5-flash", "generateContent", false, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))

	require.Nil(t, result)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "custom error codes")
	require.Equal(t, http.StatusBadRequest, rec.Code, "旧的自定义错误码配置不再把 400 改写成 500")
	require.Contains(t, rec.Body.String(), geminiSkippedTestUpstreamMsg)
}

func TestGeminiForwardAsChatCompletions_LegacyCustomCodes400NotHiddenAs500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newGeminiSkippedWriteService(http.StatusBadRequest, geminiSkippedTestUpstreamBody())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, geminiLegacyCustomCodesAPIKeyAccount(), body)

	require.Nil(t, result)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "custom error codes")
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, geminiSkippedTestUpstreamMsg, errObj["message"], "应回传上游 message")
}

func TestGeminiForwardAsChatCompletions_PoolMode400KeepsUpstreamMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newGeminiSkippedWriteService(http.StatusBadRequest, geminiSkippedTestUpstreamBody())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gemini-2.5-flash","messages":[{"role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(body)))

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, geminiPoolModeAPIKeyAccount(), body)

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code, "状态码应保真为上游 400")

	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "invalid_request_error", errObj["type"])
	require.Equal(t, geminiSkippedTestUpstreamMsg, errObj["message"], "应回传上游 message")
}

func TestWriteGeminiMappedError_400KeepsUpstreamMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GeminiMessagesCompatService{cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	err := svc.writeGeminiMappedError(c, &Account{ID: 702, Platform: PlatformGemini}, http.StatusBadRequest, "req-1", []byte(geminiSkippedTestUpstreamBody()))

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, geminiSkippedTestUpstreamMsg, errObj["message"], "应回传上游 message")
}
