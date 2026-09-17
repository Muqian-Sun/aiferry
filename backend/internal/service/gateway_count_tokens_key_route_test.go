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
)

// count_tokens 的本地 404 只给 Antigravity 成品号；标签选了 antigravity 的第三方 key
// 按 Anthropic 协议地址照常转发。

func newCountTokensRouteContext() (*gin.Context, *httptest.ResponseRecorder, *ParsedRequest) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	return c, rec, &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-5"}
}

func TestForwardCountTokens_AntigravityLabelledKeyForwardsToAnthropicEndpoint(t *testing.T) {
	c, rec, parsed := newCountTokensRouteContext()
	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"input_tokens":7}`)),
	}}
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	svc := &GatewayService{cfg: cfg, responseHeaderFilter: compileResponseHeaderFilter(cfg), httpUpstream: upstream, rateLimitService: &RateLimitService{}}
	account := &Account{
		ID:                9201,
		Platform:          PlatformAntigravity,
		Type:              AccountTypeAPIKey,
		Concurrency:       1,
		Credentials:       map[string]any{"api_key": "relay-key", "model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"}},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
	}

	require.NoError(t, svc.ForwardCountTokens(context.Background(), c, account, parsed))

	require.NotNil(t, upstream.lastReq, "key must be forwarded instead of answered with a local 404")
	require.Equal(t, "https://anthropic-relay.example.com/v1/messages/count_tokens?beta=true", upstream.lastReq.URL.String())
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestForwardCountTokens_AntigravitySubscriptionGetsLocal404(t *testing.T) {
	c, rec, parsed := newCountTokensRouteContext()
	upstream := &anthropicHTTPUpstreamRecorder{}
	svc := &GatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 9202, Platform: PlatformAntigravity, Type: AccountTypeOAuth, Concurrency: 1}

	require.NoError(t, svc.ForwardCountTokens(context.Background(), c, account, parsed))

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Nil(t, upstream.lastReq)
}
