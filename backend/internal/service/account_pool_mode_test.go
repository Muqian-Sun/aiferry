//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPoolModeRetryCount(t *testing.T) {
	tests := []struct {
		name     string
		account  *Account
		expected int
	}{
		{
			name: "default_when_not_pool_mode",
			account: &Account{
				Type:              AccountTypeAPIKey,
				Platform:          PlatformOpenAI,
				Credentials:       map[string]any{},
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
			},
			expected: defaultPoolModeRetryCount,
		},
		{
			name: "default_when_missing_retry_count",
			account: &Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode": true,
				},
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
			},
			expected: defaultPoolModeRetryCount,
		},
		{
			name: "supports_float64_from_json_credentials",
			account: &Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":             true,
					"pool_mode_retry_count": float64(5),
				},
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
			},
			expected: 5,
		},
		{
			name: "supports_json_number",
			account: &Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":             true,
					"pool_mode_retry_count": json.Number("4"),
				},
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
			},
			expected: 4,
		},
		{
			name: "supports_string_value",
			account: &Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":             true,
					"pool_mode_retry_count": "2",
				},
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
			},
			expected: 2,
		},
		{
			name: "negative_value_is_clamped_to_zero",
			account: &Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":             true,
					"pool_mode_retry_count": -1,
				},
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
			},
			expected: 0,
		},
		{
			name: "oversized_value_is_clamped_to_max",
			account: &Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":             true,
					"pool_mode_retry_count": 99,
				},
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
			},
			expected: maxPoolModeRetryCount,
		},
		{
			name: "invalid_value_falls_back_to_default",
			account: &Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":             true,
					"pool_mode_retry_count": "oops",
				},
				ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
			},
			expected: defaultPoolModeRetryCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.account.GetPoolModeRetryCount())
		})
	}
}
