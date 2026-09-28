package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
