//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

// Grok 链路（Responses / Chat / 媒体 / 语音地址）只对 Grok 成品号：第三方 key 一律按中转、走通用链路，
// 指向 api.x.ai 的 key 也一样（2026-09-29 海外四家不再有官方 key），构造 Grok 地址直接报错。
func TestGrokURLBuildersRejectThirdPartyKeys(t *testing.T) {
	cfg := &config.Config{}
	for name, account := range map[string]*Account{
		"key on api.x.ai":     {Platform: PlatformGrok, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolResponses: xai.DefaultBaseURL}},
		"grok-labelled relay": {Platform: PlatformGrok, Type: AccountTypeAPIKey, ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://grok.example.test/v1"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := buildGrokResponsesURL(account, cfg)
			require.EqualError(t, err, "grok account is required")
			_, err = buildGrokChatCompletionsURL(account, cfg)
			require.EqualError(t, err, "grok account is required")
			_, err = buildGrokMediaURL(account, cfg, GrokMediaEndpointImagesGenerations, "")
			require.EqualError(t, err, "grok account is required")
			_, err = buildGrokVoiceURL(account, cfg, "tts")
			require.EqualError(t, err, "grok account is required")
		})
	}
}

// xAI 搜索（/v1/web_search）同样只对 Grok 成品号：贴 grok 标签、地址是 api.x.ai 的 key 直接拒绝，不发上游。
func TestDoGrokNativeResponsesJSONRejectsThirdPartyKeys(t *testing.T) {
	upstream := &httpUpstreamRecorder{}
	svc := &GatewayService{httpUpstream: upstream}
	key := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai"},
		ProtocolEndpoints: map[string]string{APIProtocolResponses: xai.DefaultBaseURL}}

	_, err := svc.DoGrokNativeResponsesJSON(context.Background(), key, []byte(`{"input":"q"}`))
	require.EqualError(t, err, "grok account required")
	require.Empty(t, upstream.requests)
}

func TestGrokOAuthURLPolicy(t *testing.T) {
	t.Run("default CLI gateway always allowed under restrictive allowlist", func(t *testing.T) {
		account := &Account{
			Platform:    PlatformGrok,
			Type:        AccountTypeOAuth,
			Credentials: map[string]any{},
		}
		cfg := &config.Config{}
		cfg.Security.URLAllowlist.Enabled = true
		cfg.Security.URLAllowlist.UpstreamHosts = []string{"other.example.test"}

		target, err := buildGrokResponsesURL(account, cfg)
		require.NoError(t, err)
		require.Equal(t, xai.DefaultCLIBaseURL+"/responses", target)
	})

	t.Run("stored addresses are ignored even under restrictive allowlist", func(t *testing.T) {
		// 成品号只走官方地址：残留的官方、区域或中转地址都不参与取址，也就不会
		// 让 OAuth bearer 被发往任何非官方主机。
		for _, stored := range []string{
			xai.DefaultBaseURL,
			"https://us-west-2.api.x.ai/v1",
			"https://relay.example.test/xai/v1",
			"http://relay.example.test/v1",
		} {
			account := &Account{
				Platform:    PlatformGrok,
				Type:        AccountTypeOAuth,
				Credentials: map[string]any{"base_url": stored},
			}
			cfg := &config.Config{}
			cfg.Security.URLAllowlist.Enabled = true
			cfg.Security.URLAllowlist.UpstreamHosts = []string{"other.example.test"}

			target, err := buildGrokResponsesURL(account, cfg)
			require.NoError(t, err, stored)
			require.Equal(t, xai.DefaultCLIBaseURL+"/responses", target, stored)
		}
	})

	t.Run("unsafe override switch cannot route OAuth traffic to a stored host", func(t *testing.T) {
		// XAI_ALLOW_UNSAFE_URL_OVERRIDES relaxes the trusted-host validator to
		// accept-any; OAuth accounts carry no per-account address, so a stored
		// host still cannot receive the bearer token.
		t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
		cfg := &config.Config{}
		cfg.Security.URLAllowlist.Enabled = true
		cfg.Security.URLAllowlist.UpstreamHosts = []string{"cli-chat-proxy.grok.com"}

		custom := &Account{
			Platform: PlatformGrok,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"base_url": "http://10.0.0.1/v1",
			},
		}
		target, err := buildGrokResponsesURL(custom, cfg)
		require.NoError(t, err)
		require.Equal(t, xai.DefaultCLIBaseURL+"/responses", target)

		// The official gateway resolves under the restrictive allowlist.
		official := &Account{
			Platform:    PlatformGrok,
			Type:        AccountTypeOAuth,
			Credentials: map[string]any{},
		}
		target, err = buildGrokResponsesURL(official, cfg)
		require.NoError(t, err)
		require.Equal(t, xai.DefaultCLIBaseURL+"/responses", target)
	})
}

func TestBuildGrokBillingURLUsesCLIForOfficialAPIHosts(t *testing.T) {
	for _, baseURL := range []string{xai.DefaultBaseURL, "https://us-west-2.api.x.ai/v1"} {
		account := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{"base_url": baseURL}}

		weekly, err := buildGrokBillingURL(account, &config.Config{}, true)
		require.NoError(t, err)
		require.Equal(t, xai.DefaultCLIBaseURL+xai.BillingWeeklyPath, weekly)
	}
}

func TestBuildGrokBillingURLIgnoresStoredRelay(t *testing.T) {
	account := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{
		"base_url": "https://relay.example.test/xai/v1",
	}}

	monthly, err := buildGrokBillingURL(account, &config.Config{}, false)
	require.NoError(t, err)
	require.Equal(t, xai.DefaultCLIBaseURL+xai.BillingMonthlyPath, monthly)
}

func TestGrokBillingURLFollowsAccountBaseURL(t *testing.T) {
	t.Run("oauth default stays on CLI gateway", func(t *testing.T) {
		account := &Account{
			Platform:    PlatformGrok,
			Type:        AccountTypeOAuth,
			Credentials: map[string]any{},
		}

		weeklyURL, err := buildGrokBillingURL(account, nil, true)
		require.NoError(t, err)
		require.Equal(t, xai.DefaultCLIBaseURL+"/billing?format=credits", weeklyURL)

		monthlyURL, err := buildGrokBillingURL(account, nil, false)
		require.NoError(t, err)
		require.Equal(t, xai.DefaultCLIBaseURL+"/billing", monthlyURL)
	})

	t.Run("oauth stored relay does not carry billing probes", func(t *testing.T) {
		// 计费探测与转发同一口径：成品号只打官方网关，残留的中转地址不会收到 OAuth bearer。
		account := &Account{
			Platform: PlatformGrok,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"base_url": "https://relay.example.test/v1",
			},
		}
		cfg := &config.Config{}
		cfg.Security.URLAllowlist.Enabled = true
		cfg.Security.URLAllowlist.UpstreamHosts = []string{"cli-chat-proxy.grok.com"}

		weeklyURL, err := buildGrokBillingURL(account, cfg, true)
		require.NoError(t, err)
		require.Equal(t, xai.DefaultCLIBaseURL+"/billing?format=credits", weeklyURL)
	})
}
