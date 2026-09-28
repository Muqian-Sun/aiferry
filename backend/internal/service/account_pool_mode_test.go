//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 池模式同渠道重试次数写死 3（2026-09-28 P5，channel_features.go），渠道上的 pool_mode_retry_count 不再读。
func TestGetPoolModeRetryCount(t *testing.T) {
	endpoints := map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"}
	tests := []struct {
		name        string
		credentials map[string]any
	}{
		{name: "not_pool_mode", credentials: map[string]any{}},
		{name: "pool_mode_without_retry_count", credentials: map[string]any{"pool_mode": true}},
		{name: "legacy_retry_count_ignored", credentials: map[string]any{"pool_mode": true, "pool_mode_retry_count": float64(5)}},
		{name: "legacy_zero_retry_count_ignored", credentials: map[string]any{"pool_mode": true, "pool_mode_retry_count": 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Credentials: tt.credentials, ProtocolEndpoints: endpoints}
			require.Equal(t, 3, account.GetPoolModeRetryCount())
		})
	}
}
