package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccount_IsAnthropicAPIKeyPassthroughEnabled(t *testing.T) {
	t.Run("Anthropic API Key 开启", func(t *testing.T) {
		account := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"anthropic_passthrough": true,
			},
			ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
		}
		require.True(t, account.IsAnthropicAPIKeyPassthroughEnabled())
	})

	t.Run("Anthropic API Key 关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"anthropic_passthrough": false,
			},
			ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
		}
		require.False(t, account.IsAnthropicAPIKeyPassthroughEnabled())
	})

	t.Run("字段类型非法默认关闭", func(t *testing.T) {
		account := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeAPIKey,
			Extra: map[string]any{
				"anthropic_passthrough": "true",
			},
			ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
		}
		require.False(t, account.IsAnthropicAPIKeyPassthroughEnabled())
	})

	t.Run("成品号始终关闭", func(t *testing.T) {
		oauth := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"anthropic_passthrough": true,
			},
		}
		require.False(t, oauth.IsAnthropicAPIKeyPassthroughEnabled())
	})

	t.Run("任何标签的第三方 key 开启即生效", func(t *testing.T) {
		for _, label := range []string{PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformKimi} {
			key := &Account{
				Platform: label,
				Type:     AccountTypeAPIKey,
				Extra: map[string]any{
					"anthropic_passthrough": true,
				},
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
			}
			require.True(t, key.IsAnthropicAPIKeyPassthroughEnabled(), "label %s", label)
		}
	})
}

func TestAccount_GetAnthropicAPIKeyAuthScheme(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    string
	}{
		{
			name: "missing extra defaults to x-api-key",
			account: &Account{
				Platform:          PlatformAnthropic,
				Type:              AccountTypeAPIKey,
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
			},
			want: AnthropicAPIKeyAuthSchemeXAPIKey,
		},
		{
			name: "explicit bearer",
			account: &Account{
				Platform: PlatformAnthropic,
				Type:     AccountTypeAPIKey,
				Extra: map[string]any{
					"anthropic_apikey_auth_scheme": AnthropicAPIKeyAuthSchemeAuthorizationBearer,
				},
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
			},
			want: AnthropicAPIKeyAuthSchemeAuthorizationBearer,
		},
		{
			name: "invalid value defaults to x-api-key",
			account: &Account{
				Platform: PlatformAnthropic,
				Type:     AccountTypeAPIKey,
				Extra: map[string]any{
					"anthropic_apikey_auth_scheme": "bearer",
				},
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"},
			},
			want: AnthropicAPIKeyAuthSchemeXAPIKey,
		},
		{
			name: "key of another label honours bearer",
			account: &Account{
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Extra: map[string]any{
					"anthropic_apikey_auth_scheme": AnthropicAPIKeyAuthSchemeAuthorizationBearer,
				},
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
			},
			want: AnthropicAPIKeyAuthSchemeAuthorizationBearer,
		},
		{
			name: "gemini-labelled key honours bearer",
			account: &Account{
				Platform: PlatformGemini,
				Type:     AccountTypeAPIKey,
				Extra: map[string]any{
					"anthropic_apikey_auth_scheme": AnthropicAPIKeyAuthSchemeAuthorizationBearer,
				},
				ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://anthropic-relay.example.com"},
			},
			want: AnthropicAPIKeyAuthSchemeAuthorizationBearer,
		},
		{
			name: "subscription account ignores the setting",
			account: &Account{
				Platform: PlatformAnthropic,
				Type:     AccountTypeOAuth,
				Extra: map[string]any{
					"anthropic_apikey_auth_scheme": AnthropicAPIKeyAuthSchemeAuthorizationBearer,
				},
			},
			want: AnthropicAPIKeyAuthSchemeXAPIKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.GetAnthropicAPIKeyAuthScheme())
		})
	}
}
