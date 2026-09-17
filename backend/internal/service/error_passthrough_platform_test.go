//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 错误透传规则按平台配置：成品号按账号平台匹配（行为不变），第三方 key 的平台只是展示
// 标签，按请求所在网关平台匹配。

func TestErrorPassthroughRulePlatform(t *testing.T) {
	key := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	antigravityOAuth := &Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth}

	require.Equal(t, PlatformAnthropic, ErrorPassthroughRulePlatform(key, PlatformAnthropic))
	// 混合调度：anthropic 网关上的 antigravity 成品号仍按 antigravity 规则匹配。
	require.Equal(t, PlatformAntigravity, ErrorPassthroughRulePlatform(antigravityOAuth, PlatformAnthropic))
}

func TestAnthropicGatewayRequestPlatform(t *testing.T) {
	antigravityGroupKey := &APIKey{Group: &Group{Platform: PlatformAntigravity}}

	require.Equal(t, PlatformAnthropic, AnthropicGatewayRequestPlatform(context.Background(), nil))
	require.Equal(t, PlatformAntigravity, AnthropicGatewayRequestPlatform(context.Background(), antigravityGroupKey))

	forced := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformGemini)
	require.Equal(t, PlatformGemini, AnthropicGatewayRequestPlatform(forced, antigravityGroupKey))
	// Messages 走兜底分组时强制平台被清成空串，改按分组平台。
	cleared := context.WithValue(context.Background(), ctxkey.ForcePlatform, "")
	require.Equal(t, PlatformAntigravity, AnthropicGatewayRequestPlatform(cleared, antigravityGroupKey))

	composite := WithResolvedTargetPlatform(context.Background(), PlatformGemini)
	require.Equal(t, PlatformGemini, AnthropicGatewayRequestPlatform(composite, &APIKey{Group: &Group{Platform: PlatformComposite}}))
}

func TestOpenAICompatibleRequestPlatform(t *testing.T) {
	require.Equal(t, PlatformOpenAI, OpenAICompatibleRequestPlatform(context.Background(), nil))
	require.Equal(t, PlatformKimi, OpenAICompatibleRequestPlatform(context.Background(), &APIKey{Group: &Group{Platform: PlatformKimi}}))
	composite := WithResolvedTargetPlatform(context.Background(), PlatformGrok)
	require.Equal(t, PlatformGrok, OpenAICompatibleRequestPlatform(composite, &APIKey{Group: &Group{Platform: PlatformComposite}}))
	require.Equal(t, PlatformOpenAI, OpenAICompatibleRequestPlatform(WithResolvedTargetPlatform(context.Background(), PlatformAnthropic), nil))
}

const passthroughPlatformTestRuleStatus = http.StatusTeapot

func newPassthroughPlatformTestContext(method, path string, body []byte, groupPlatform string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if groupPlatform != "" {
		c.Set("api_key", &APIKey{ID: 1, Group: &Group{ID: 1, Platform: groupPlatform}})
	}
	return c, rec
}

func passthroughPlatformTestKey(label string, endpoints map[string]string) *Account {
	return &Account{
		ID:                301,
		Name:              "passthrough-platform-key",
		Platform:          label,
		Type:              AccountTypeAPIKey,
		Concurrency:       1,
		Credentials:       map[string]any{"api_key": "sk-test"},
		ProtocolEndpoints: endpoints,
	}
}

// 每个用例跑同一个错误路径两次：规则只配网关平台时命中，只配 key 的标签平台时不命中。
func TestErrorPassthroughRules_KeyMatchesGatewayPlatformNotLabel(t *testing.T) {
	const keyword = "passthrough-platform-probe"
	errorBody := []byte(`{"error":{"message":"` + keyword + `"}}`)
	failedSSE := "data: " + `{"type":"response.failed","response":{"id":"resp_err","object":"response","status":"failed","error":{"code":"context_length_exceeded","type":"invalid_request_error","message":"` + keyword + ` context window"}}}` + "\n\n"

	errorResponse := func() *http.Response {
		return &http.Response{StatusCode: http.StatusUnprocessableEntity, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(errorBody))}
	}
	sseResponse := func() *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(failedSSE))}
	}
	anthropicKey := func(label string) *Account {
		return passthroughPlatformTestKey(label, map[string]string{APIProtocolAnthropic: "https://relay.example.com"})
	}
	responsesKey := func(label string) *Account {
		return passthroughPlatformTestKey(label, map[string]string{APIProtocolResponses: "http://upstream.example"})
	}
	openAISvc := func(upstream *http.Response) *OpenAIGatewayService {
		return &OpenAIGatewayService{
			cfg:          rawChatCompletionsTestConfig(),
			httpUpstream: &httpUpstreamRecorder{resp: upstream},
		}
	}
	streamSvc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}

	tests := []struct {
		name            string
		label           string
		groupPlatform   string
		gatewayPlatform string
		run             func(c *gin.Context, label string)
	}{
		{
			name: "anthropic gateway error response", label: PlatformOpenAI, groupPlatform: PlatformAntigravity, gatewayPlatform: PlatformAntigravity,
			run: func(c *gin.Context, label string) {
				_, _ = (&GatewayService{}).handleErrorResponse(context.Background(), errorResponse(), c, anthropicKey(label))
			},
		},
		{
			name: "anthropic gateway retry exhausted", label: PlatformOpenAI, groupPlatform: PlatformAnthropic, gatewayPlatform: PlatformAnthropic,
			run: func(c *gin.Context, label string) {
				_, _ = (&GatewayService{}).handleRetryExhaustedError(context.Background(), errorResponse(), c, anthropicKey(label))
			},
		},
		{
			name: "openai gateway compat error response", label: PlatformAnthropic, groupPlatform: PlatformKimi, gatewayPlatform: PlatformKimi,
			run: func(c *gin.Context, label string) {
				_, _ = (&OpenAIGatewayService{}).handleCompatErrorResponse(errorResponse(), c, responsesKey(label), writeChatCompletionsError)
			},
		},
		{
			name: "openai images error response", label: PlatformAnthropic, groupPlatform: PlatformKimi, gatewayPlatform: PlatformOpenAI,
			run: func(c *gin.Context, label string) {
				_, _ = (&OpenAIGatewayService{}).handleOpenAIImagesErrorResponse(context.Background(), errorResponse(), c, responsesKey(label))
			},
		},
		{
			name: "grok media error response", label: PlatformOpenAI, groupPlatform: PlatformOpenAI, gatewayPlatform: PlatformGrok,
			run: func(c *gin.Context, label string) {
				key := passthroughPlatformTestKey(label, map[string]string{APIProtocolChatCompletions: "https://api.x.ai/v1", APIProtocolResponses: "https://api.x.ai/v1"})
				_, _ = (&OpenAIGatewayService{}).handleGrokMediaErrorResponse(context.Background(), errorResponse(), c, key, "", "grok-imagine-image")
			},
		},
		{
			name: "chat completions buffered response.failed", label: PlatformAnthropic, gatewayPlatform: PlatformOpenAI,
			run: func(c *gin.Context, label string) {
				body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
				_, _ = openAISvc(sseResponse()).ForwardAsChatCompletions(context.Background(), c, responsesKey(label), body, "", "")
			},
		},
		{
			name: "chat completions streaming response.failed", label: PlatformAnthropic, gatewayPlatform: PlatformOpenAI,
			run: func(c *gin.Context, label string) {
				body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
				_, _ = openAISvc(sseResponse()).ForwardAsChatCompletions(context.Background(), c, responsesKey(label), body, "", "")
			},
		},
		{
			name: "messages buffered response.failed", label: PlatformGemini, gatewayPlatform: PlatformOpenAI,
			run: func(c *gin.Context, label string) {
				body := []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
				_, _ = openAISvc(sseResponse()).ForwardAsAnthropic(context.Background(), c, responsesKey(label), body, "", "")
			},
		},
		{
			name: "messages streaming response.failed", label: PlatformGemini, gatewayPlatform: PlatformOpenAI,
			run: func(c *gin.Context, label string) {
				body := []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":true}`)
				_, _ = openAISvc(sseResponse()).ForwardAsAnthropic(context.Background(), c, responsesKey(label), body, "", "")
			},
		},
		{
			name: "responses streaming response.failed", label: PlatformAnthropic, gatewayPlatform: PlatformOpenAI,
			run: func(c *gin.Context, label string) {
				_, _ = streamSvc.handleStreamingResponse(c.Request.Context(), sseResponse(), c, responsesKey(label), time.Now(), "gpt-5", "gpt-5")
			},
		},
		{
			name: "responses passthrough streaming response.failed", label: PlatformAnthropic, gatewayPlatform: PlatformOpenAI,
			run: func(c *gin.Context, label string) {
				_, _ = streamSvc.handleStreamingResponsePassthrough(c.Request.Context(), sseResponse(), c, responsesKey(label), time.Now(), "gpt-5", "gpt-5")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newPassthroughPlatformTestContext(http.MethodPost, "/v1/test", nil, tt.groupPlatform)
			bindPassthroughRule(c, tt.gatewayPlatform, []string{keyword}, passthroughPlatformTestRuleStatus)
			tt.run(c, tt.label)
			require.Equal(t, passthroughPlatformTestRuleStatus, rec.Code, "规则配了请求所在网关平台，key 应命中")

			c, rec = newPassthroughPlatformTestContext(http.MethodPost, "/v1/test", nil, tt.groupPlatform)
			bindPassthroughRule(c, tt.label, []string{keyword}, passthroughPlatformTestRuleStatus)
			tt.run(c, tt.label)
			require.NotEqual(t, passthroughPlatformTestRuleStatus, rec.Code, "规则只配了 key 的平台标签，不应命中")
		})
	}
}
