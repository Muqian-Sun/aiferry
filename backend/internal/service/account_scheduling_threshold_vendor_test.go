//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 调度阈值与额度快照是上游厂商的窗口：按 Vendor 评估，第三方 key 的平台标签不参与。

func thresholdTestKey(label string, endpoints map[string]string, extra map[string]any) *Account {
	return &Account{
		ID:                9200,
		Platform:          label,
		Type:              AccountTypeAPIKey,
		Status:            StatusActive,
		Schedulable:       true,
		Credentials:       map[string]any{"account_mode": AccountModeCoding},
		ProtocolEndpoints: endpoints,
		Extra:             extra,
	}
}

var thresholdTestKimiCoding = map[string]string{
	APIProtocolChatCompletions: DefaultKimiCodingBaseURL,
	APIProtocolAnthropic:       DefaultKimiCodingAnthropicBaseURL,
}

func TestEvaluateAccountSchedulingThreshold_KeysFollowVendorNotLabel(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	reset := now.Add(3 * time.Hour)
	kimiSnapshot := map[string]any{
		"kimi_5h_used_percent": 90.0,
		"kimi_5h_reset_at":     reset.Format(time.RFC3339),
	}
	codexSnapshot := map[string]any{
		"codex_usage_updated_at": now.Add(-time.Minute).Format(time.RFC3339),
		"codex_5h_used_percent":  100.0,
		"codex_5h_reset_at":      reset.Format(time.RFC3339),
	}
	thresholds := map[string]int{PlatformKimi: 80, PlatformOpenAI: 80}

	t.Run("openai label on official kimi coding address uses kimi snapshot and threshold", func(t *testing.T) {
		key := thresholdTestKey(PlatformOpenAI, thresholdTestKimiCoding, kimiSnapshot)
		require.Equal(t, PlatformKimi, key.Vendor())

		decision := EvaluateAccountSchedulingThreshold(key, thresholds, now)
		require.True(t, decision.ShouldPause)
		require.Equal(t, PlatformKimi, decision.Platform)
		require.Equal(t, 80, decision.ThresholdPercent)
		require.True(t, reset.Equal(*decision.Until))
	})

	t.Run("openai label on relay does not pause on forwarded codex snapshot", func(t *testing.T) {
		key := thresholdTestKey(PlatformOpenAI, map[string]string{APIProtocolResponses: "https://relay.example.com/v1"}, codexSnapshot)
		require.Empty(t, key.Vendor())

		decision := EvaluateAccountSchedulingThreshold(key, thresholds, now)
		require.False(t, decision.ShouldPause)
		require.Empty(t, decision.Platform)
	})

	t.Run("anthropic label on relay does not pause on forwarded fable window", func(t *testing.T) {
		key := thresholdTestKey(PlatformAnthropic, map[string]string{APIProtocolAnthropic: "https://relay.example.com"}, map[string]any{
			"passive_usage_7d_oi_utilization": 0.95,
			"passive_usage_7d_oi_reset":       float64(reset.Unix()),
		})
		require.Empty(t, key.Vendor())

		decision := evaluateAnthropicFableSchedulingThreshold(key, map[string]int{PlatformAnthropic: 80}, now)
		require.False(t, decision.ShouldPause)
	})

	t.Run("deepseek label on official anthropic address evaluates fable window", func(t *testing.T) {
		key := thresholdTestKey(PlatformDeepseek, map[string]string{APIProtocolAnthropic: "https://api.anthropic.com"}, map[string]any{
			"passive_usage_7d_oi_utilization": 0.95,
			"passive_usage_7d_oi_reset":       float64(reset.Unix()),
		})
		require.Equal(t, PlatformAnthropic, key.Vendor())

		decision := evaluateAnthropicFableSchedulingThreshold(key, map[string]int{PlatformAnthropic: 80}, now)
		require.True(t, decision.ShouldPause)
	})
}

func TestCNProviderQuotaSnapshotReset_ReadsVendorPrefixNotLabel(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	reset := now.Add(2 * time.Hour)
	key := thresholdTestKey(PlatformOpenAI, thresholdTestKimiCoding, map[string]any{
		"kimi_5h_reset_at": reset.Format(time.RFC3339),
		// 同名但以标签为前缀的快照不应被读取。
		"openai_5h_reset_at": now.Add(time.Hour).Format(time.RFC3339),
	})
	require.Equal(t, PlatformKimi, key.Vendor())

	got := cnProviderQuotaSnapshotReset(key, now)
	require.NotNil(t, got)
	require.True(t, reset.Equal(*got))
}
