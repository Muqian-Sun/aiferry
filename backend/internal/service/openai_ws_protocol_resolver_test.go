package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 渠道级 WS 开关 / 模式 2026-09-28 P5 删了：legacy 路径（mode_router_v2 关）恒走 HTTP，
// 库里残留的 openai_*_responses_websockets_v2_* / responses_websockets_v2_enabled / openai_ws_enabled
// 都不再打开 WS；要开 WS 用 mode_router_v2 + 全局 ingress_mode_default。
func TestOpenAIWSProtocolResolver_Resolve(t *testing.T) {
	baseCfg := &config.Config{}
	baseCfg.Gateway.OpenAIWS.Enabled = true
	baseCfg.Gateway.OpenAIWS.OAuthEnabled = true
	baseCfg.Gateway.OpenAIWS.APIKeyEnabled = true
	baseCfg.Gateway.OpenAIWS.ResponsesWebsockets = false
	baseCfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true

	openAIOAuth := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1}

	t.Run("legacy 路径恒走 HTTP，残留账号键不再打开 WS", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			account *Account
		}{
			{name: "oauth enabled key", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
				Extra: map[string]any{"openai_oauth_responses_websockets_v2_enabled": true, "openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeCtxPool}}},
			{name: "setup token enabled key", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeSetupToken, Concurrency: 1,
				Extra: map[string]any{"openai_oauth_responses_websockets_v2_enabled": true}}},
			{name: "legacy responses_websockets_v2_enabled", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
				Extra: map[string]any{"responses_websockets_v2_enabled": true}}},
			{name: "legacy openai_ws_enabled", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
				Extra: map[string]any{"openai_ws_enabled": true}}},
			{name: "api key enabled key", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
				Extra:             map[string]any{"openai_apikey_responses_websockets_v2_enabled": true, "openai_apikey_responses_websockets_v2_mode": OpenAIWSIngressModePassthrough},
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}}},
			{name: "no key", account: openAIOAuth},
		} {
			t.Run(tc.name, func(t *testing.T) {
				decision := NewOpenAIWSProtocolResolver(baseCfg).Resolve(tc.account)
				require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
				require.Equal(t, "account_disabled", decision.Reason)
			})
		}
	})

	t.Run("非OpenAI setup token不进入OAuth WS", func(t *testing.T) {
		account := &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken}
		decision := NewOpenAIWSProtocolResolver(baseCfg).Resolve(account)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "platform_not_openai", decision.Reason)
	})

	t.Run("账号级强制HTTP", func(t *testing.T) {
		account := *openAIOAuth
		account.Extra = map[string]any{"openai_ws_force_http": true}
		decision := NewOpenAIWSProtocolResolver(baseCfg).Resolve(&account)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "account_force_http", decision.Reason)
	})

	t.Run("全局强制HTTP无需启用mode router", func(t *testing.T) {
		cfg := *baseCfg
		cfg.Gateway.OpenAIWS.ForceHTTP = true

		decision := NewOpenAIWSProtocolResolver(&cfg).Resolve(openAIOAuth)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "global_force_http", decision.Reason)
	})

	t.Run("全局关闭保持HTTP", func(t *testing.T) {
		cfg := *baseCfg
		cfg.Gateway.OpenAIWS.Enabled = false
		decision := NewOpenAIWSProtocolResolver(&cfg).Resolve(openAIOAuth)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "global_disabled", decision.Reason)
	})

	t.Run("按账号类型开关控制", func(t *testing.T) {
		cfg := *baseCfg
		cfg.Gateway.OpenAIWS.OAuthEnabled = false
		decision := NewOpenAIWSProtocolResolver(&cfg).Resolve(openAIOAuth)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "oauth_disabled", decision.Reason)
	})

	t.Run("API Key 账号关闭开关时回退HTTP", func(t *testing.T) {
		cfg := *baseCfg
		cfg.Gateway.OpenAIWS.APIKeyEnabled = false
		account := &Account{
			Platform:          PlatformOpenAI,
			Type:              AccountTypeAPIKey,
			ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
		}
		decision := NewOpenAIWSProtocolResolver(&cfg).Resolve(account)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "apikey_disabled", decision.Reason)
	})

	t.Run("未知认证类型回退HTTP", func(t *testing.T) {
		account := &Account{Platform: PlatformOpenAI, Type: "unknown_type"}
		decision := NewOpenAIWSProtocolResolver(baseCfg).Resolve(account)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "unknown_auth_type", decision.Reason)
	})
}

func TestOpenAIWSProtocolResolver_Resolve_ModeRouterV2(t *testing.T) {
	newCfg := func(defaultMode string) *config.Config {
		cfg := &config.Config{}
		cfg.Gateway.OpenAIWS.Enabled = true
		cfg.Gateway.OpenAIWS.OAuthEnabled = true
		cfg.Gateway.OpenAIWS.APIKeyEnabled = true
		cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
		cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
		cfg.Gateway.OpenAIWS.IngressModeDefault = defaultMode
		return cfg
	}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1}

	t.Run("ctx_pool mode routes to ws v2", func(t *testing.T) {
		decision := NewOpenAIWSProtocolResolver(newCfg(OpenAIWSIngressModeCtxPool)).Resolve(account)
		require.Equal(t, OpenAIUpstreamTransportResponsesWebsocketV2, decision.Transport)
		require.Equal(t, "ws_v2_mode_ctx_pool", decision.Reason)
	})

	t.Run("OpenAI setup token 复用 OAuth 开关", func(t *testing.T) {
		setupToken := &Account{Platform: PlatformOpenAI, Type: AccountTypeSetupToken, Concurrency: 1}
		decision := NewOpenAIWSProtocolResolver(newCfg(OpenAIWSIngressModeCtxPool)).Resolve(setupToken)
		require.Equal(t, OpenAIUpstreamTransportResponsesWebsocketV2, decision.Transport)
		require.Equal(t, "ws_v2_mode_ctx_pool", decision.Reason)
	})

	t.Run("v2关闭时回退v1", func(t *testing.T) {
		cfg := newCfg(OpenAIWSIngressModeCtxPool)
		cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = false
		cfg.Gateway.OpenAIWS.ResponsesWebsockets = true
		decision := NewOpenAIWSProtocolResolver(cfg).Resolve(account)
		require.Equal(t, OpenAIUpstreamTransportResponsesWebsocket, decision.Transport)
		require.Equal(t, "ws_v1_mode_ctx_pool", decision.Reason)
	})

	t.Run("off mode routes to http", func(t *testing.T) {
		decision := NewOpenAIWSProtocolResolver(newCfg(OpenAIWSIngressModeOff)).Resolve(account)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "account_mode_off", decision.Reason)
	})

	t.Run("passthrough mode routes to ws v2", func(t *testing.T) {
		decision := NewOpenAIWSProtocolResolver(newCfg(OpenAIWSIngressModePassthrough)).Resolve(account)
		require.Equal(t, OpenAIUpstreamTransportResponsesWebsocketV2, decision.Transport)
		require.Equal(t, "ws_v2_mode_passthrough", decision.Reason)
	})

	t.Run("http_bridge mode routes to http_sse", func(t *testing.T) {
		decision := NewOpenAIWSProtocolResolver(newCfg(OpenAIWSIngressModeHTTPBridge)).Resolve(account)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "ws_v2_mode_http_bridge", decision.Reason)
	})

	t.Run("残留账号 mode 键不覆盖全局默认", func(t *testing.T) {
		legacy := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
			Extra: map[string]any{"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff, "openai_oauth_responses_websockets_v2_enabled": false}}
		decision := NewOpenAIWSProtocolResolver(newCfg(OpenAIWSIngressModeCtxPool)).Resolve(legacy)
		require.Equal(t, OpenAIUpstreamTransportResponsesWebsocketV2, decision.Transport)
		require.Equal(t, "ws_v2_mode_ctx_pool", decision.Reason)
	})

	t.Run("non-positive concurrency is rejected in v2 router", func(t *testing.T) {
		invalidConcurrency := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
		decision := NewOpenAIWSProtocolResolver(newCfg(OpenAIWSIngressModeCtxPool)).Resolve(invalidConcurrency)
		require.Equal(t, OpenAIUpstreamTransportHTTPSSE, decision.Transport)
		require.Equal(t, "account_concurrency_invalid", decision.Reason)
	})
}
