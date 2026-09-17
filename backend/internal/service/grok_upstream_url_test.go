//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestGrokAPIKeyURLPolicyFollowsGlobalSecurityConfig(t *testing.T) {
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "http://grok.example.test/v1",
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "http://grok.example.test/v1", APIProtocolResponses: "http://grok.example.test/v1"},
	}

	t.Run("insecure HTTP enabled with allowlist disabled", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Security.URLAllowlist.Enabled = false
		cfg.Security.URLAllowlist.AllowInsecureHTTP = true

		responsesURL, err := buildGrokResponsesURL(account, cfg)
		require.NoError(t, err)
		require.Equal(t, "http://grok.example.test/v1/responses", responsesURL)

		chatURL, err := buildGrokChatCompletionsURL(account, cfg)
		require.NoError(t, err)
		require.Equal(t, "http://grok.example.test/v1/chat/completions", chatURL)

		mediaURL, err := buildGrokMediaURL(account, cfg, GrokMediaEndpointImagesGenerations, "")
		require.NoError(t, err)
		require.Equal(t, "http://grok.example.test/v1/images/generations", mediaURL)

		contentURL, err := buildGrokMediaURL(account, cfg, GrokMediaEndpointVideoContent, "request 123")
		require.NoError(t, err)
		require.Equal(t, "http://grok.example.test/v1/videos/request%20123/content", contentURL)
	})

	t.Run("insecure HTTP disabled", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Security.URLAllowlist.Enabled = false
		cfg.Security.URLAllowlist.AllowInsecureHTTP = false

		_, err := buildGrokResponsesURL(account, cfg)
		require.EqualError(t, err, "invalid base url: base URL rejected by URL security policy")
	})

	t.Run("enabled allowlist remains HTTPS only", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Security.URLAllowlist.Enabled = true
		cfg.Security.URLAllowlist.AllowInsecureHTTP = true
		cfg.Security.URLAllowlist.UpstreamHosts = []string{"grok.example.test"}

		_, err := buildGrokResponsesURL(account, cfg)
		require.EqualError(t, err, "invalid base url: base URL rejected by URL security policy")
	})
}

func TestGrokAPIKeyURLPolicyAppliesAllowlistAndPrivateHostControls(t *testing.T) {
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://grok.example.test/v1",
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://grok.example.test/v1", APIProtocolResponses: "https://grok.example.test/v1"},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = true
	cfg.Security.URLAllowlist.UpstreamHosts = []string{"grok.example.test"}

	target, err := buildGrokResponsesURL(account, cfg)
	require.NoError(t, err)
	require.Equal(t, "https://grok.example.test/v1/responses", target)

	cfg.Security.URLAllowlist.UpstreamHosts = []string{"other.example.test"}
	_, err = buildGrokResponsesURL(account, cfg)
	require.EqualError(t, err, "invalid base url: base URL rejected by URL security policy")

	account.ProtocolEndpoints[APIProtocolResponses] = "https://127.0.0.1/v1"
	cfg.Security.URLAllowlist.UpstreamHosts = []string{"127.0.0.1"}
	_, err = buildGrokResponsesURL(account, cfg)
	require.EqualError(t, err, "invalid base url: base URL rejected by URL security policy")

	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	target, err = buildGrokResponsesURL(account, cfg)
	require.NoError(t, err)
	require.Equal(t, "https://127.0.0.1/v1/responses", target)
}

func TestGrokAPIKeyURLPolicyRedactsMalformedConfiguredURL(t *testing.T) {
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://%zz:secret@grok.example.test/v1",
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://%zz:secret@grok.example.test/v1", APIProtocolResponses: "https://%zz:secret@grok.example.test/v1"},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true

	_, err := buildGrokResponsesURL(account, cfg)
	require.EqualError(t, err, "invalid base url: base URL rejected by URL security policy")
	require.NotContains(t, err.Error(), "secret")
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
