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

// 标签为 openai、只配了 anthropic 协议地址的 key 在 Anthropic 网关上：透传开关、Bearer
// 认证方式与请求头覆写都跟 anthropic 标签的 key 一样生效。

func openAILabelledAnthropicKey(extra map[string]any) *Account {
	return &Account{
		ID:          9301,
		Name:        "openai-labelled-anthropic-key",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                    "relay-key",
			credKeyHeaderOverrideEnabled: true,
			credKeyHeaderOverrides:       map[string]any{"x-relay-tenant": "tenant-1"},
		},
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
		Extra:             extra,
		Status:            StatusActive,
		Schedulable:       true,
	}
}

func newAnthropicKeyForwardFixture(respBody string) (*gin.Context, *GatewayService, *anthropicHTTPUpstreamRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}}
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}
	svc := &GatewayService{
		cfg:                  cfg,
		responseHeaderFilter: compileResponseHeaderFilter(cfg),
		httpUpstream:         upstream,
		rateLimitService:     &RateLimitService{},
		deferredService:      &DeferredService{},
	}
	return c, svc, upstream
}

func TestGatewayServiceForward_OpenAILabelledKeyOnAnthropicProtocolUsesPassthroughBearerAndOverrides(t *testing.T) {
	c, svc, upstream := newAnthropicKeyForwardFixture(`{"id":"msg_1","type":"message","model":"claude-sonnet-4-5","usage":{"input_tokens":3,"output_tokens":1}}`)
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	parsed := &ParsedRequest{Body: NewRequestBodyRef(body), Model: "claude-sonnet-4-5"}
	account := openAILabelledAnthropicKey(map[string]any{
		"anthropic_passthrough":        true,
		"anthropic_apikey_auth_scheme": AnthropicAPIKeyAuthSchemeAuthorizationBearer,
	})

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	passthrough, _ := c.Get("anthropic_passthrough")
	require.Equal(t, true, passthrough, "passthrough must be enabled for a key of any label")
	require.Equal(t, "https://anthropic-relay.example.com/v1/messages?beta=true", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer relay-key", getHeaderRaw(upstream.lastReq.Header, "authorization"))
	require.Empty(t, getHeaderRaw(upstream.lastReq.Header, "x-api-key"))
	require.Equal(t, "tenant-1", getHeaderRaw(upstream.lastReq.Header, "x-relay-tenant"))
}

func TestGatewayServiceBuildUpstreamRequest_OpenAILabelledKeyUsesBearerAndOverrides(t *testing.T) {
	c, svc, _ := newAnthropicKeyForwardFixture(`{}`)
	account := openAILabelledAnthropicKey(map[string]any{
		"anthropic_apikey_auth_scheme": AnthropicAPIKeyAuthSchemeAuthorizationBearer,
	})

	req, _, err := svc.buildUpstreamRequest(context.Background(), c, account,
		[]byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`),
		"relay-key", "apikey", "claude-sonnet-4-5", false, false)

	require.NoError(t, err)
	require.Equal(t, "https://anthropic-relay.example.com/v1/messages?beta=true", req.URL.String())
	require.Equal(t, "Bearer relay-key", getHeaderRaw(req.Header, "authorization"))
	require.Empty(t, getHeaderRaw(req.Header, "x-api-key"))
	require.Equal(t, "tenant-1", getHeaderRaw(req.Header, "x-relay-tenant"))
}
