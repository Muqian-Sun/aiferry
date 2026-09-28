//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 渠道级停调阈值覆盖已删（2026-09-28 P5），阈值只来自全站表；这里用显式表评估，
// 验证候选经过 Redis 快照投影（精简 meta 与完整账号两条读路径）后，阈值判断的用量输入仍然完整。
func TestSchedulerCacheAnthropicThresholdAdmission(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	reset := now.Add(time.Hour)
	cases := []struct {
		name          string
		fiveHour      float64
		sevenDay      float64
		fable         float64
		threshold     int
		expired       bool
		accountPaused bool
	}{
		{name: "shared 5h", fiveHour: .60, threshold: 60, accountPaused: true},
		{name: "shared 7d", sevenDay: .66, threshold: 60, accountPaused: true},
		{name: "below threshold", fiveHour: .59, sevenDay: .59, fable: .59, threshold: 60},
		{name: "expired windows", fiveHour: .75, sevenDay: .75, fable: .75, threshold: 60, expired: true},
		{name: "platform threshold off", fiveHour: .75, sevenDay: .75, fable: .75, threshold: 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			end := reset
			if tc.expired {
				end = now.Add(-time.Hour)
			}
			account := service.Account{
				ID: 3, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth,
				Status: service.StatusActive, Schedulable: true, SessionWindowEnd: &end,
				Credentials: map[string]any{"access_token": "test-only-token"},
				Extra: map[string]any{
					"session_window_utilization":   tc.fiveHour,
					"passive_usage_7d_utilization": tc.sevenDay, "passive_usage_7d_reset": end.Unix(),
					"passive_usage_7d_oi_utilization": tc.fable, "passive_usage_7d_oi_reset": end.Unix(),
					"unrelated_large_payload": "drop me",
				},
			}
			cache := newSchedulerCacheUnit(t)
			bucket := service.SchedulerBucket{PoolID: 8, Platform: service.PlatformAnthropic, Mode: service.SchedulerModeSingle}
			token, err := cache.CaptureBucketWriteToken(ctx, bucket)
			require.NoError(t, err)
			require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))
			candidates, hit, err := cache.GetSnapshot(ctx, bucket)
			require.NoError(t, err)
			require.True(t, hit)
			require.Len(t, candidates, 1)
			require.NotContains(t, candidates[0].Credentials, "access_token")
			require.NotContains(t, candidates[0].Extra, "unrelated_large_payload")
			full, err := cache.GetAccount(ctx, account.ID)
			require.NoError(t, err)
			require.NotNil(t, full)
			thresholds := map[string]int{service.PlatformAnthropic: tc.threshold}
			for _, candidate := range []*service.Account{candidates[0], full} {
				decision := service.EvaluateAccountSchedulingThreshold(candidate, thresholds, now)
				require.Equal(t, tc.accountPaused, decision.ShouldPause)
				if tc.accountPaused {
					require.NotNil(t, decision.Until)
					require.True(t, end.Equal(*decision.Until))
				}
				// Fable 专属窗口的输入也要随投影保留
				require.EqualValues(t, account.Extra["passive_usage_7d_oi_utilization"], candidate.Extra["passive_usage_7d_oi_utilization"])
				require.EqualValues(t, account.Extra["passive_usage_7d_oi_reset"], candidate.Extra["passive_usage_7d_oi_reset"])
			}
		})
	}
}

func TestSchedulerCacheAnthropicUsageRefresh(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	end := now.Add(time.Hour)
	cache := newSchedulerCacheUnit(t)
	bucket := service.SchedulerBucket{PoolID: 8, Platform: service.PlatformAnthropic, Mode: service.SchedulerModeSingle}
	account := service.Account{
		ID: 3, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth,
		Status: service.StatusActive, Schedulable: true,
		Extra: map[string]any{"passive_usage_7d_utilization": .59, "passive_usage_7d_reset": end.Unix()},
	}
	token, err := cache.CaptureBucketWriteToken(ctx, bucket)
	require.NoError(t, err)
	require.NoError(t, cache.SetSnapshot(ctx, bucket, token, []service.Account{account}))
	for _, used := range []float64{.59, .66, .10} {
		account.Extra["passive_usage_7d_utilization"] = used
		// UpdateExtra refreshes account payloads without rebuilding bucket membership.
		require.NoError(t, cache.SetAccount(ctx, &account))
		candidates, hit, err := cache.GetSnapshot(ctx, bucket)
		require.NoError(t, err)
		require.True(t, hit)
		require.Len(t, candidates, 1)
		decision := service.EvaluateAccountSchedulingThreshold(candidates[0], map[string]int{service.PlatformAnthropic: 60}, now)
		require.Equal(t, used >= .60, decision.ShouldPause)
	}
}
