//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// OpenAI Responses 协议特性（续链、推理回放清理、parallel_tool_calls、WSv2、透传、
// Responses Lite、分组策略等）对第三方 key 的规则：厂商是官方 OpenAI 或通用中转时启用，
// 其他已知厂商不启用，平台标签不参与。
//
// 夹具故意把标签与地址错开：kimi 标签挂通用中转（应启用），openai 标签挂智谱官方地址
// （应不启用）。

func featureRelayKey(endpoints map[string]string) *Account {
	return keyProtocolTestAccount(PlatformKimi, endpoints)
}

func featureZhipuKey(endpoints map[string]string) *Account {
	return keyProtocolTestAccount(PlatformOpenAI, endpoints)
}

func featureRelayEndpoints() map[string]string {
	return map[string]string{
		APIProtocolChatCompletions: "http://relay.example/v1",
		APIProtocolResponses:       "http://relay.example/v1",
	}
}

func featureZhipuEndpoints() map[string]string {
	return map[string]string{
		APIProtocolChatCompletions: DefaultZhipuPayGBaseURL,
		APIProtocolResponses:       DefaultZhipuPayGBaseURL,
	}
}

func requireFeatureFixtures(t *testing.T, relay, vendor *Account) {
	t.Helper()
	require.Equal(t, "", relay.Vendor(), "fixture: relay host must not be recognised as a vendor")
	require.Equal(t, PlatformZhipu, vendor.Vendor(), "fixture: open.bigmodel.cn must be recognised as zhipu")
}

func TestOpenAIProtocolFeaturesApply(t *testing.T) {
	for _, tc := range []struct {
		name      string
		account   *Account
		features  bool
		keyScoped bool
	}{
		{name: "nil", account: nil},
		{name: "openai oauth", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, features: true},
		{name: "grok oauth", account: &Account{Platform: PlatformGrok, Type: AccountTypeOAuth}},
		{name: "kimi label on a relay", account: featureRelayKey(featureRelayEndpoints()), features: true, keyScoped: true},
		{name: "deepseek label on api.openai.com", account: keyProtocolTestAccount(PlatformDeepseek, map[string]string{APIProtocolResponses: "https://api.openai.com"}), features: true, keyScoped: true},
		{name: "openai label on zhipu", account: featureZhipuKey(featureZhipuEndpoints())},
		{name: "openai label on api.x.ai", account: keyProtocolTestAccount(PlatformOpenAI, map[string]string{APIProtocolResponses: xaiOfficialTestBaseURL})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.features, openAIProtocolFeaturesApply(tc.account))
			require.Equal(t, tc.keyScoped, keyUsesOpenAIProtocolFeatures(tc.account))
		})
	}
}

func TestKeyKeepsHTTPPreviousResponseID(t *testing.T) {
	require.True(t, AccountKeepsHTTPPreviousResponseID(featureRelayKey(featureRelayEndpoints())))
	require.False(t, AccountKeepsHTTPPreviousResponseID(featureRelayKey(map[string]string{APIProtocolChatCompletions: "http://relay.example/v1"})),
		"a key whose Responses traffic is converted to chat completions cannot continue a response chain")
	require.False(t, AccountKeepsHTTPPreviousResponseID(featureZhipuKey(featureZhipuEndpoints())))
	require.False(t, AccountKeepsHTTPPreviousResponseID(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
}

func TestOpenAIToolSchemaPlatform(t *testing.T) {
	require.Equal(t, PlatformGrok, openAIToolSchemaPlatform(&Account{Platform: PlatformGrok, Type: AccountTypeOAuth}, ""))
	require.Equal(t, PlatformOpenAI, openAIToolSchemaPlatform(featureRelayKey(featureRelayEndpoints()), APIProtocolResponses))
	require.Equal(t, PlatformAnthropic, openAIToolSchemaPlatform(featureRelayKey(featureRelayEndpoints()), APIProtocolAnthropic))
	require.Equal(t, PlatformZhipu, openAIToolSchemaPlatform(featureZhipuKey(featureZhipuEndpoints()), APIProtocolResponses))
}

func TestOpenAIKeyFeatureSwitchesFollowVendorNotLabel(t *testing.T) {
	extra := func() map[string]any {
		return map[string]any{
			"openai_passthrough":                            true,
			"openai_apikey_responses_websockets_v2_enabled": true,
			"openai_apikey_responses_websockets_v2_mode":    OpenAIWSIngressModePassthrough,
			"openai_ws_force_http":                          true,
			"openai_ws_allow_store_recovery":                true,
			"openai_compact_mode":                           OpenAICompactModeForceOn,
		}
	}
	relay := featureRelayKey(featureRelayEndpoints())
	relay.Extra = extra()
	vendor := featureZhipuKey(featureZhipuEndpoints())
	vendor.Extra = extra()
	requireFeatureFixtures(t, relay, vendor)

	require.True(t, relay.IsOpenAIPassthroughEnabled())
	require.True(t, relay.IsOpenAIResponsesWebSocketV2Enabled())
	require.Equal(t, OpenAIWSIngressModePassthrough, relay.ResolveOpenAIResponsesWebSocketV2Mode(OpenAIWSIngressModeOff))
	require.True(t, relay.IsOpenAIWSForceHTTPEnabled())
	require.True(t, relay.IsOpenAIWSAllowStoreRecoveryEnabled())
	require.Equal(t, OpenAICompactModeForceOn, relay.GetOpenAICompactMode())
	supported, known := relay.OpenAICompactSupportKnown()
	require.True(t, supported)
	require.True(t, known)
	require.True(t, relay.AllowsOpenAICompact())

	require.False(t, vendor.IsOpenAIPassthroughEnabled())
	require.False(t, vendor.IsOpenAIResponsesWebSocketV2Enabled())
	require.Equal(t, OpenAIWSIngressModeOff, vendor.ResolveOpenAIResponsesWebSocketV2Mode(OpenAIWSIngressModeCtxPool))
	require.False(t, vendor.IsOpenAIWSForceHTTPEnabled())
	require.False(t, vendor.IsOpenAIWSAllowStoreRecoveryEnabled())
	require.Equal(t, OpenAICompactModeAuto, vendor.GetOpenAICompactMode())
	_, known = vendor.OpenAICompactSupportKnown()
	require.False(t, known)
	require.False(t, vendor.AllowsOpenAICompact())
}

func TestOpenAIWSProtocolResolverKeyFollowsVendorAndResponsesEndpoint(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	wsExtra := func() map[string]any {
		return map[string]any{"openai_apikey_responses_websockets_v2_enabled": true}
	}
	resolver := NewOpenAIWSProtocolResolver(cfg)

	relay := featureRelayKey(featureRelayEndpoints())
	relay.Extra = wsExtra()
	decision := resolver.Resolve(relay)
	require.Equal(t, OpenAIUpstreamTransportResponsesWebsocketV2, decision.Transport)

	chatOnly := featureRelayKey(map[string]string{APIProtocolChatCompletions: "http://relay.example/v1"})
	chatOnly.Extra = wsExtra()
	decision = resolver.Resolve(chatOnly)
	require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
	require.Equal(t, "responses_endpoint_missing", decision.Reason)

	vendor := featureZhipuKey(featureZhipuEndpoints())
	vendor.Extra = wsExtra()
	decision = resolver.Resolve(vendor)
	require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
	require.Equal(t, "platform_not_openai", decision.Reason)
}

func TestShouldPreserveNoneReasoningEffortForKeysFollowsOfficialOpenAIHost(t *testing.T) {
	require.True(t, shouldPreserveOpenAIResponsesNoneReasoningEffort(keyProtocolTestAccount(PlatformKimi, map[string]string{APIProtocolResponses: "https://api.openai.com"})))
	require.False(t, shouldPreserveOpenAIResponsesNoneReasoningEffort(keyProtocolTestAccount(PlatformOpenAI, featureRelayEndpoints())))
}

func TestOpenAIResponsesNamespaceHandlingFollowsVendorNotLabel(t *testing.T) {
	relay := featureRelayKey(featureRelayEndpoints())
	vendor := featureZhipuKey(featureZhipuEndpoints())
	requireFeatureFixtures(t, relay, vendor)
	namespaceBody := []byte(`{"tools":[{"type":"namespace","name":"mcp","tools":[]}]}`)

	require.True(t, shouldStripOpenAIResponsesInputNamespaces(relay, OpenAIUpstreamTransportHTTPSSE, false))
	require.False(t, shouldStripOpenAIResponsesInputNamespaces(vendor, OpenAIUpstreamTransportHTTPSSE, false))
	require.True(t, shouldKeepOpenAIResponsesToolCallNamespaces(relay, OpenAIUpstreamTransportHTTPSSE, false, false, namespaceBody))
	require.False(t, shouldKeepOpenAIResponsesToolCallNamespaces(vendor, OpenAIUpstreamTransportHTTPSSE, false, false, namespaceBody))
}

// TestOpenAIGatewayKeyResponsesFeaturesFollowVendorNotLabel 走完整的 Responses 转发，
// 看上游请求体上的协议特性处理。
func TestOpenAIGatewayKeyResponsesFeaturesFollowVendorNotLabel(t *testing.T) {
	relay := featureRelayKey(featureRelayEndpoints())
	vendor := featureZhipuKey(featureZhipuEndpoints())
	requireFeatureFixtures(t, relay, vendor)

	ingress := keyProtocolResponsesIngress
	ingress.body = []byte(`{"model":"gpt-5.4","stream":false,"previous_response_id":"resp_prev","parallel_tool_calls":true,` +
		`"input":[{"type":"reasoning","id":"rs_1","summary":[],"content":[{"type":"reasoning_text","text":"visible"}]},{"type":"message","role":"user","content":"hi"}]}`)

	t.Run("relay keeps the response chain and applies OpenAI replay fixes", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, relay, ingress)
		require.Equal(t, "resp_prev", gjson.GetBytes(upstream.lastBody, "previous_response_id").String())
		require.False(t, gjson.GetBytes(upstream.lastBody, "parallel_tool_calls").Exists())
		require.False(t, gjson.GetBytes(upstream.lastBody, "input.0.content").Exists())
	})
	t.Run("other vendor drops the response chain and keeps the body as sent", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, vendor, ingress)
		require.False(t, gjson.GetBytes(upstream.lastBody, "previous_response_id").Exists())
		require.True(t, gjson.GetBytes(upstream.lastBody, "parallel_tool_calls").Bool())
		require.True(t, gjson.GetBytes(upstream.lastBody, "input.0.content").IsArray())
	})

	schemaIngress := keyProtocolResponsesIngress
	schemaIngress.body = []byte(`{"model":"gpt-5.4","stream":false,"input":"hi",` +
		`"tools":[{"type":"function","name":"search","parameters":{"type":"object","properties":{"q":{"type":"string","pattern":"^(?=.*foo)[a-z]+$"}}}}]}`)
	t.Run("relay gets the OpenAI tool schema pattern rule", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, relay, schemaIngress)
		require.False(t, gjson.GetBytes(upstream.lastBody, "tools.0.parameters.properties.q.pattern").Exists())
	})
	t.Run("other vendor keeps the pattern", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, vendor, schemaIngress)
		require.True(t, gjson.GetBytes(upstream.lastBody, "tools.0.parameters.properties.q.pattern").Exists())
	})

	liteIngress := keyProtocolResponsesIngress
	liteIngress.setup = func(c *gin.Context) { c.Request.Header.Set(responsesLiteHeader, "true") }
	t.Run("relay applies Responses Lite", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, relay, liteIngress)
		parallel := gjson.GetBytes(upstream.lastBody, "parallel_tool_calls")
		require.True(t, parallel.Exists())
		require.False(t, parallel.Bool())
	})
	t.Run("other vendor ignores the Responses Lite header", func(t *testing.T) {
		upstream := captureKeyProtocolRequest(t, vendor, liteIngress)
		require.False(t, gjson.GetBytes(upstream.lastBody, "parallel_tool_calls").Exists())
	})
}

// TestOpenAIResponsesWebSocketCompatibilityBodyFollowsVendorNotLabel：WS 与透传共用的
// Responses 兼容归一化（推理回放、parallel_tool_calls、工具 schema）按厂商启用。
func TestOpenAIResponsesWebSocketCompatibilityBodyFollowsVendorNotLabel(t *testing.T) {
	relay := featureRelayKey(featureRelayEndpoints())
	vendor := featureZhipuKey(featureZhipuEndpoints())
	requireFeatureFixtures(t, relay, vendor)
	body := []byte(`{"model":"gpt-5.4","parallel_tool_calls":true,` +
		`"input":[{"type":"reasoning","id":"rs_1","summary":[],"content":[{"type":"reasoning_text","text":"visible"}]},{"type":"message","role":"user","content":"hi"}],` +
		`"tools":[{"type":"function","name":"search","parameters":{"type":"object","properties":{"q":{"type":"string","pattern":"^(?=.*foo)[a-z]+$"}}}}]}`)

	normalized, changed, err := normalizeOpenAIResponsesWebSocketCompatibilityBody(body, relay, false)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, gjson.GetBytes(normalized, "input.0.content").Exists())
	require.True(t, gjson.GetBytes(normalized, "parallel_tool_calls").Exists(), "tools are declared, so parallel_tool_calls stays")
	require.False(t, gjson.GetBytes(normalized, "tools.0.parameters.properties.q.pattern").Exists())

	withoutTools := []byte(`{"model":"gpt-5.4","parallel_tool_calls":true,"input":"hi"}`)
	normalized, _, err = normalizeOpenAIResponsesWebSocketCompatibilityBody(withoutTools, relay, false)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(normalized, "parallel_tool_calls").Exists())

	normalized, changed, err = normalizeOpenAIResponsesWebSocketCompatibilityBody(body, vendor, false)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, string(body), string(normalized))
}

func TestOpenAIGatewayKeyPassthroughFollowsVendorNotLabel(t *testing.T) {
	relay := featureRelayKey(featureRelayEndpoints())
	relay.Extra = map[string]any{"openai_passthrough": true}
	ingress := keyProtocolResponsesIngress
	ingress.body = []byte(`{"model":"gpt-5.4","stream":false,"input":"hi","tools":[{"type":"custom","name":"apply_patch","description":"patch"}]}`)

	upstream := captureKeyProtocolRequest(t, relay, ingress)
	require.Equal(t, "function", gjson.GetBytes(upstream.lastBody, "tools.0.type").String(),
		"passthrough on a standard Responses upstream lowers Codex custom tools to function tools")
}

func TestOpenAIGatewayKeyCompatPromptCacheKeyFollowsVendorNotLabel(t *testing.T) {
	relay := featureRelayKey(map[string]string{APIProtocolResponses: "http://relay.example/v1"})
	vendor := featureZhipuKey(map[string]string{APIProtocolResponses: DefaultZhipuPayGBaseURL})
	requireFeatureFixtures(t, relay, vendor)
	ingress := keyProtocolChatIngress
	ingress.body = []byte(`{"model":"gpt-5.4","messages":[{"role":"system","content":"be brief"},{"role":"user","content":"hello"}],"stream":false}`)

	upstream := captureKeyProtocolRequest(t, relay, ingress)
	require.Equal(t, "http://relay.example/v1/responses", upstream.lastReq.URL.String())
	require.NotEmpty(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())

	upstream = captureKeyProtocolRequest(t, vendor, ingress)
	require.Empty(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
}

// TestOpenAIGatewayKeyEmptyCompletedFailoverFollowsVendorNotLabel：空 response.completed
// 视为静默拒绝而 failover 是 OpenAI Responses 流语义，通用中转同样适用，其他厂商不套用。
func TestOpenAIGatewayKeyEmptyCompletedFailoverFollowsVendorNotLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.4","input":"hi","stream":true}`)
	forward := func(account *Account) error {
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_empty\",\"status\":\"completed\",\"output\":[]}}\n\ndata: [DONE]\n\n",
			)),
		}}
		svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
		_, err := svc.Forward(context.Background(), adaptiveProtocolTestContext("/v1/responses", body), account, body)
		return err
	}
	relay := featureRelayKey(featureRelayEndpoints())
	vendor := featureZhipuKey(featureZhipuEndpoints())
	requireFeatureFixtures(t, relay, vendor)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, forward(relay), &failoverErr)
	require.False(t, errors.As(forward(vendor), &failoverErr))
}
