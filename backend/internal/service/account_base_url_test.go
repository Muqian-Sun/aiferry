//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestGetBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "non-apikey type returns empty",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformAnthropic,
			},
			expected: "",
		},
		{
			name: "apikey with configured anthropic endpoint",
			account: Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformAnthropic,
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://custom.example.com"},
			},
			expected: "https://custom.example.com",
		},
		{
			name: "协议地址首尾空白会被去掉",
			account: Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformAnthropic,
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "  https://custom.example.com  "},
			},
			expected: "https://custom.example.com",
		},
		{
			name: "antigravity apikey 按填写值原样返回，不补 /antigravity",
			account: Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformAntigravity,
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://upstream.example.com"},
			},
			expected: "https://upstream.example.com",
		},
		{
			name: "antigravity apikey 已带 /antigravity 不会被重复拼接",
			account: Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformAntigravity,
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://upstream.example.com/antigravity"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			name: "antigravity non-apikey returns empty",
			account: Account{
				Type:              AccountTypeOAuth,
				Platform:          PlatformAntigravity,
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://upstream.example.com"},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.account.GetBaseURL()
			if result != tt.expected {
				t.Errorf("GetBaseURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetGeminiBaseURL(t *testing.T) {
	const defaultGeminiURL = "https://generativelanguage.googleapis.com"

	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			// 第三方 key 未配 gemini 端点时返回空，由调用方按配置错误处理，
			// 不再回落官方地址。
			name: "apikey without gemini endpoint returns empty",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformGemini,
				Credentials: map[string]any{},
			},
			expected: "",
		},
		{
			name: "apikey with configured gemini endpoint",
			account: Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformGemini,
				ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://custom-gemini.example.com"},
			},
			expected: "https://custom-gemini.example.com",
		},
		{
			name: "antigravity apikey 按填写值原样返回，不补 /antigravity",
			account: Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformAntigravity,
				ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://upstream.example.com"},
			},
			expected: "https://upstream.example.com",
		},
		{
			name: "antigravity apikey 已带 /antigravity 不会被重复拼接",
			account: Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformAntigravity,
				ProtocolEndpoints: map[string]string{APIProtocolGemini: "https://upstream.example.com/antigravity"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			// 成品号只走官方地址：凭据里残留的 base_url 不参与取址。
			name: "antigravity oauth ignores stored base_url",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com"},
			},
			expected: defaultGeminiURL,
		},
		{
			name: "oauth without base_url returns default",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{},
			},
			expected: defaultGeminiURL,
		},
		{
			name: "nil credentials returns empty for third-party key",
			account: Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformGemini,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.account.GetGeminiBaseURL(defaultGeminiURL)
			if result != tt.expected {
				t.Errorf("GetGeminiBaseURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetGrokBaseURLOAuthIgnoresStoredAddress(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "oauth without base_url uses CLI subscription proxy",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformGrok,
				Credentials: map[string]any{},
			},
			expected: xai.DefaultCLIBaseURL,
		},
		{
			// 区域选择由站点级 grok_default_base_url_mode 决定，账号不能覆盖。
			name: "oauth stored official API endpoint is ignored",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": xai.DefaultBaseURL,
				},
			},
			expected: xai.DefaultCLIBaseURL,
		},
		{
			name: "oauth stored regional API endpoint is ignored",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": "https://us-west-2.api.x.ai/v1",
				},
			},
			expected: xai.DefaultCLIBaseURL,
		},
		{
			name: "oauth stored CLI proxy yields the CLI proxy",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": xai.DefaultCLIBaseURL,
				},
			},
			expected: xai.DefaultCLIBaseURL,
		},
		{
			name: "oauth unparseable base_url is ignored",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": "not a url",
				},
			},
			expected: xai.DefaultCLIBaseURL,
		},
		{
			// 成品号不走中转：要走中转请按第三方 key 建号。
			name: "oauth stored custom relay is ignored",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": "https://relay.example.com/xai/v1",
				},
			},
			expected: xai.DefaultCLIBaseURL,
		},
		{
			// 第三方 key 不回落官方端点：没配协议映射就是空，由调用方按缺地址报错。
			name: "API key without protocol endpoints has no fallback",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformGrok,
				Credentials: map[string]any{"base_url": xai.DefaultBaseURL},
			},
			expected: "",
		},
		{
			name: "API key uses its protocol endpoint",
			account: Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformGrok,
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://grok-relay.example.com/v1"},
			},
			expected: "https://grok-relay.example.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.account.GetGrokBaseURL())
		})
	}
}

func TestGetGrokBaseURLIgnoresOAuthCustomEvenWithUnsafeOverrides(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	account := Account{
		Type:     AccountTypeOAuth,
		Platform: PlatformGrok,
		Credentials: map[string]any{
			"base_url": "https://custom.example.com/v1",
		},
	}

	require.Equal(t, xai.DefaultCLIBaseURL, account.GetGrokBaseURL())
}

func TestGetGrokMediaBaseURLRedirectsCLIGatewayToOfficialAPI(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "oauth without base_url uses official media API",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformGrok,
				Credentials: map[string]any{},
			},
			expected: xai.DefaultBaseURL,
		},
		{
			name: "oauth stored CLI proxy is separated from the media API",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": xai.DefaultCLIBaseURL,
				},
			},
			expected: xai.DefaultBaseURL,
		},
		{
			name: "oauth stored CLI proxy variant is canonicalized to the media API",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": "HTTPS://CLI-CHAT-PROXY.GROK.COM:443/%76%31/",
				},
			},
			expected: xai.DefaultBaseURL,
		},
		{
			name: "oauth unparseable base_url falls back to official media API",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": "not a url",
				},
			},
			expected: xai.DefaultBaseURL,
		},
		{
			name: "oauth stored regional API endpoint is ignored for media",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": "https://us-west-2.api.x.ai/v1",
				},
			},
			expected: xai.DefaultBaseURL,
		},
		{
			name: "oauth stored custom relay is ignored for media",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformGrok,
				Credentials: map[string]any{
					"base_url": "https://custom.example.com/v1",
				},
			},
			expected: xai.DefaultBaseURL,
		},
		{
			name: "API key retains its configured media API",
			account: Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformGrok,
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://grok.example.com/v1"},
			},
			expected: "https://grok.example.com/v1",
		},
		{
			name: "non-Grok account has no Grok media base URL",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformOpenAI,
				Credentials: map[string]any{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.account.GetGrokMediaBaseURL())
		})
	}
}

func TestGetGrokMediaBaseURLIgnoresOAuthCustomEvenWithUnsafeOverrides(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	account := Account{
		Type:     AccountTypeOAuth,
		Platform: PlatformGrok,
		Credentials: map[string]any{
			"base_url": "https://custom.example.com/v1",
		},
	}

	require.Equal(t, xai.DefaultBaseURL, account.GetGrokMediaBaseURL())
}
