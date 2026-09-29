//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 中转 key 的三个计 token 入口：先发上游，上游不支持（404 / 405 / 501）就本地估算回 200；其余错误照常回错
// （2026-09-29 muqian 定：「上游不支持就本地算」）。

func relayCountTokensUpstream(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

var relayUnsupportedStatuses = []int{http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusNotImplemented}

// Anthropic count_tokens 转 Anthropic 协议的中转（GatewayService.ForwardCountTokens）。
func TestForwardCountTokens_RelayUnsupportedFallsBackLocally(t *testing.T) {
	account := &Account{
		ID: 9301, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials:       map[string]any{"api_key": "relay-key", "model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"}},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
	}
	newSvc := func(resp *http.Response) (*GatewayService, *anthropicHTTPUpstreamRecorder) {
		upstream := &anthropicHTTPUpstreamRecorder{resp: resp}
		cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
		return &GatewayService{cfg: cfg, responseHeaderFilter: compileResponseHeaderFilter(cfg), httpUpstream: upstream, rateLimitService: &RateLimitService{}}, upstream
	}

	for _, status := range relayUnsupportedStatuses {
		c, rec, parsed := newCountTokensRouteContext()
		svc, upstream := newSvc(relayCountTokensUpstream(status, `{"error":{"message":"not found"}}`))
		require.NoError(t, svc.ForwardCountTokens(context.Background(), c, account, parsed), "status=%d", status)
		require.NotNil(t, upstream.lastReq, "status=%d: 先发上游", status)
		require.Equal(t, http.StatusOK, rec.Code, "status=%d: 上游不支持应本地估算", status)
		require.Positive(t, gjson.Get(rec.Body.String(), "input_tokens").Int(), "status=%d", status)
	}

	c, rec, parsed := newCountTokensRouteContext()
	svc, _ := newSvc(relayCountTokensUpstream(http.StatusInternalServerError, `{"error":{"message":"boom"}}`))
	require.Error(t, svc.ForwardCountTokens(context.Background(), c, account, parsed))
	require.Equal(t, http.StatusInternalServerError, rec.Code, "其余错误照常回错，不本地估算")
	require.False(t, gjson.Get(rec.Body.String(), "input_tokens").Exists())
}

// Anthropic count_tokens 转 OpenAI 协议的中转（ForwardCountTokensAsAnthropic → /v1/responses/input_tokens）。
func TestForwardCountTokensAsAnthropic_RelayUnsupportedFallsBackLocally(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello world"}]}`)
	run := func(account *Account, resp *http.Response) (*httpUpstreamRecorder, *httptest.ResponseRecorder, error) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
		upstream := &httpUpstreamRecorder{resp: resp}
		svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, rateLimitService: &RateLimitService{}}
		err := svc.ForwardCountTokensAsAnthropic(context.Background(), c, account, body)
		return upstream, rec, err
	}
	relay := keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolResponses: "https://relay.example/v1"})

	for _, status := range relayUnsupportedStatuses {
		upstream, rec, err := run(relay, relayCountTokensUpstream(status, `{"error":{"message":"unknown route"}}`))
		require.NoError(t, err, "status=%d", status)
		require.NotNil(t, upstream.lastReq, "status=%d: 先发上游", status)
		require.Equal(t, http.StatusOK, rec.Code, "status=%d: 上游不支持应本地估算", status)
		require.Positive(t, gjson.Get(rec.Body.String(), "input_tokens").Int(), "status=%d", status)
	}

	_, rec, err := run(relay, relayCountTokensUpstream(http.StatusInternalServerError, `{"error":{"message":"boom"}}`))
	require.Error(t, err)
	require.NotEqual(t, http.StatusOK, rec.Code, "其余错误照常回错，不本地估算")

	chatOnly := keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolChatCompletions: "https://relay.example/v1"})
	upstream, rec, err := run(chatOnly, nil)
	require.NoError(t, err)
	require.Nil(t, upstream.lastReq, "没有 responses 地址就没有 input_tokens 端点，直接本地估算")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Positive(t, gjson.Get(rec.Body.String(), "input_tokens").Int())
}

// Responses 的 input_tokens（ForwardResponsesInputTokens）：中转上游 405 / 501 同样本地估算（404 原本就会）。
func TestForwardResponsesInputTokens_RelayUnsupportedFallsBackLocally(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.4","input":"hello world"}`)
	relay := keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolResponses: "https://relay.example/v1"})
	for _, status := range []int{http.StatusMethodNotAllowed, http.StatusNotImplemented} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/input_tokens", nil)
		upstream := &httpUpstreamRecorder{resp: relayCountTokensUpstream(status, `{"error":{"message":"not implemented"}}`)}
		svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, rateLimitService: &RateLimitService{}}

		require.NoError(t, svc.ForwardResponsesInputTokens(context.Background(), c, relay, body), "status=%d", status)
		require.NotNil(t, upstream.lastReq, "status=%d: 先发上游", status)
		require.Equal(t, http.StatusOK, rec.Code, "status=%d", status)
		require.Equal(t, "response.input_tokens", gjson.Get(rec.Body.String(), "object").String())
		require.Positive(t, gjson.Get(rec.Body.String(), "input_tokens").Int())
	}
}
