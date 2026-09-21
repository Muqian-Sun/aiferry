//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func quotaStateTestService(t *testing.T) (*RateLimitService, *rateLimitAccountRepoStub) {
	t.Helper()
	accountSchedulingThresholdsSF.Forget(SettingKeyAccountSchedulingThresholds)
	accountSchedulingThresholdsCache.Store(&cachedAccountSchedulingThresholds{})
	repo := &rateLimitAccountRepoStub{}
	rl := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	rl.SetSettingService(NewSettingService(newMockSettingRepo(), &config.Config{}))
	return rl, repo
}

func openAIQuotaStateAccount(id int64, extra map[string]any) *Account {
	return &Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: extra}
}

// 原来在选号路径上按 Codex 5h/7d 快照判「跳过」，现在在状态写入点判「停调到窗口重置」；夹具沿用。
func TestOpenAIQuotaPauseDecision_Thresholds(t *testing.T) {
	now := time.Now().UTC()
	fresh := now.Add(-time.Minute).Format(time.RFC3339)
	resetIn := now.Add(time.Hour).Truncate(time.Second)
	tests := []struct {
		name       string
		extra      map[string]any
		settings   OpsOpenAIAccountQuotaAutoPauseSettings
		wantPause  bool
		wantWindow string
	}{
		{name: "5h at account threshold pauses", extra: map[string]any{"codex_5h_used_percent": 95.0, "auto_pause_5h_threshold": 0.95}, wantPause: true, wantWindow: "5h"},
		{name: "5h below threshold allows", extra: map[string]any{"codex_5h_used_percent": 80.0, "auto_pause_5h_threshold": 0.95}},
		{name: "7d at account threshold pauses", extra: map[string]any{"codex_7d_used_percent": 95.0, "auto_pause_7d_threshold": 0.95}, wantPause: true, wantWindow: "7d"},
		{name: "no threshold configured keeps legacy behavior", extra: map[string]any{"codex_5h_used_percent": 99.0, "codex_7d_used_percent": 99.0}},
		{name: "global default threshold applies", extra: map[string]any{"codex_5h_used_percent": 95.0}, settings: OpsOpenAIAccountQuotaAutoPauseSettings{DefaultThreshold5h: 0.95}, wantPause: true, wantWindow: "5h"},
		{name: "per-account disable overrides global default", extra: map[string]any{"codex_5h_used_percent": 99.0, "auto_pause_5h_disabled": true}, settings: OpsOpenAIAccountQuotaAutoPauseSettings{DefaultThreshold5h: 0.95}},
		{name: "disable is per window", extra: map[string]any{"codex_5h_used_percent": 99.0, "codex_7d_used_percent": 99.0, "auto_pause_5h_disabled": true, "auto_pause_7d_threshold": 0.95}, wantPause: true, wantWindow: "7d"},
		{name: "window already reset skips pause", extra: map[string]any{"codex_5h_used_percent": 99.0, "auto_pause_5h_threshold": 0.95, "codex_5h_reset_at": now.Add(-time.Minute).Format(time.RFC3339)}},
		{name: "fresh window still pauses", extra: map[string]any{"codex_5h_used_percent": 99.0, "auto_pause_5h_threshold": 0.95, "codex_5h_reset_at": resetIn.Format(time.RFC3339)}, wantPause: true, wantWindow: "5h"},
		{name: "stale snapshot skips pause (#2994)", extra: map[string]any{"codex_5h_used_percent": 99.0, "auto_pause_5h_threshold": 0.95, "codex_5h_reset_at": resetIn.Format(time.RFC3339), "codex_usage_updated_at": now.Add(-3 * time.Hour).Format(time.RFC3339)}},
		{name: "fresh exhausted snapshot still pauses (#2994)", extra: map[string]any{"codex_5h_used_percent": 99.0, "auto_pause_5h_threshold": 0.95, "codex_5h_reset_at": resetIn.Format(time.RFC3339), "codex_usage_updated_at": fresh}, wantPause: true, wantWindow: "5h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := openAIQuotaStateAccount(1, tt.extra)
			until, reason, paused := openAIQuotaPauseDecision(account, tt.settings, now)
			require.Equal(t, tt.wantPause, paused)
			if !tt.wantPause {
				return
			}
			require.Contains(t, reason, "codex "+tt.wantWindow+" window")
			if _, hasReset := tt.extra["codex_"+tt.wantWindow+"_reset_at"]; hasReset {
				require.True(t, until.Equal(resetIn), "停到窗口重置时间")
			} else {
				require.WithinDuration(t, now.Add(quotaPauseFallback), until, time.Second, "没有重置时间就停一小段等下次快照")
			}
		})
	}
}

func TestOpenAIQuotaPauseDecision_OnlyOpenAISubscriptionsAndKeys(t *testing.T) {
	now := time.Now().UTC()
	extra := map[string]any{"codex_5h_used_percent": 99.0, "auto_pause_5h_threshold": 0.95}
	gemini := &Account{ID: 2, Platform: PlatformGemini, Type: AccountTypeOAuth, Extra: extra}
	_, _, paused := openAIQuotaPauseDecision(gemini, OpsOpenAIAccountQuotaAutoPauseSettings{}, now)
	require.False(t, paused)
}

func TestGrokQuotaPauseDecision_UntilFollowsRetryAfterAndWindowReset(t *testing.T) {
	now := time.Now().UTC()
	zero, limit := int64(0), int64(10)
	retryAfter := 30
	resetUnix := now.Add(10 * time.Minute).Unix()

	retry := &Account{ID: 3, Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{
		grokQuotaSnapshotExtraKey: xai.QuotaSnapshot{RetryAfterSeconds: &retryAfter, UpdatedAt: now.Format(time.RFC3339)},
	}}
	until, reason, paused := grokQuotaPauseDecision(retry, now)
	require.True(t, paused)
	require.Contains(t, reason, "retry_after")
	require.WithinDuration(t, now.Add(30*time.Second), until, time.Second)

	exhausted := &Account{ID: 4, Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{
		grokQuotaSnapshotExtraKey: xai.QuotaSnapshot{
			Requests:  &xai.QuotaWindow{Limit: &limit, Remaining: &zero, ResetUnix: &resetUnix},
			UpdatedAt: now.Format(time.RFC3339),
		},
	}}
	until, reason, paused = grokQuotaPauseDecision(exhausted, now)
	require.True(t, paused)
	require.Contains(t, reason, "requests window")
	require.Equal(t, resetUnix, until.Unix())

	key := &Account{ID: 5, Platform: PlatformGrok, Type: AccountTypeAPIKey, Extra: exhausted.Extra}
	_, _, paused = grokQuotaPauseDecision(key, now)
	require.False(t, paused, "xAI 配额快照只有成品号才有")
}

func TestQuotaCounterPauseDecision(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

	t.Run("total exceeded pauses until admin resets", func(t *testing.T) {
		account := &Account{ID: 6, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 10.0, "quota_used": 10.0}}
		until, reason, paused := quotaCounterPauseDecision(account, now)
		require.True(t, paused)
		require.Contains(t, reason, "total quota")
		require.True(t, until.After(now.AddDate(50, 0, 0)))
	})
	t.Run("total below limit allows", func(t *testing.T) {
		account := &Account{ID: 6, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 10.0, "quota_used": 9.99}}
		_, _, paused := quotaCounterPauseDecision(account, now)
		require.False(t, paused)
	})
	t.Run("daily rolling exceeded pauses until period end", func(t *testing.T) {
		start := now.Add(-2 * time.Hour)
		account := &Account{ID: 7, Type: AccountTypeAPIKey, Extra: map[string]any{
			"quota_daily_limit": 1.0, "quota_daily_used": 1.5, "quota_daily_start": start.Format(time.RFC3339),
		}}
		until, reason, paused := quotaCounterPauseDecision(account, now)
		require.True(t, paused)
		require.Contains(t, reason, "daily quota")
		require.True(t, until.Equal(start.Add(24*time.Hour)))
	})
	t.Run("daily period expired does not pause", func(t *testing.T) {
		account := &Account{ID: 7, Type: AccountTypeAPIKey, Extra: map[string]any{
			"quota_daily_limit": 1.0, "quota_daily_used": 1.5, "quota_daily_start": now.Add(-25 * time.Hour).Format(time.RFC3339),
		}}
		_, _, paused := quotaCounterPauseDecision(account, now)
		require.False(t, paused)
	})
	t.Run("daily fixed exceeded pauses until next fixed reset", func(t *testing.T) {
		account := &Account{ID: 8, Type: AccountTypeAPIKey, Extra: map[string]any{
			"quota_daily_limit": 1.0, "quota_daily_used": 1.0, "quota_daily_start": now.Add(-time.Hour).Format(time.RFC3339),
			"quota_daily_reset_mode": "fixed", "quota_daily_reset_hour": 8.0, "quota_reset_timezone": "UTC",
		}}
		until, _, paused := quotaCounterPauseDecision(account, now)
		require.True(t, paused)
		require.True(t, until.Equal(time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)))
	})
	t.Run("weekly rolling exceeded pauses until period end", func(t *testing.T) {
		start := now.Add(-3 * 24 * time.Hour)
		account := &Account{ID: 9, Type: AccountTypeAPIKey, Extra: map[string]any{
			"quota_weekly_limit": 5.0, "quota_weekly_used": 5.0, "quota_weekly_start": start.Format(time.RFC3339),
		}}
		until, reason, paused := quotaCounterPauseDecision(account, now)
		require.True(t, paused)
		require.Contains(t, reason, "weekly quota")
		require.True(t, until.Equal(start.Add(7*24*time.Hour)))
	})
	t.Run("applies to oauth accounts too", func(t *testing.T) {
		account := &Account{ID: 10, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{"quota_limit": 1.0, "quota_used": 2.0}}
		_, _, paused := quotaCounterPauseDecision(account, now)
		require.True(t, paused)
	})
}

func TestApplyAccountQuotaState_OpenAI5hThresholdPauses(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	resetAt := time.Now().UTC().Add(time.Hour)
	account := openAIQuotaStateAccount(1001, map[string]any{
		"codex_5h_used_percent": 96.0, "auto_pause_5h_threshold": 0.95, "codex_5h_reset_at": resetAt.Format(time.RFC3339),
	})

	require.True(t, rl.ApplyAccountQuotaState(context.Background(), account))

	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, int64(1001), repo.lastTempID)
	require.NotNil(t, account.TempUnschedulableUntil)
	require.WithinDuration(t, resetAt, *account.TempUnschedulableUntil, time.Second)
	payload, ok := parseTempUnschedReasonPayload(repo.lastTempReason)
	require.True(t, ok)
	require.Equal(t, openAIQuotaAutoPauseSource, payload.Source)
	require.Contains(t, payload.ErrorMessage, "codex 5h window")
	require.False(t, account.SchedulingState(time.Now()).Allows(time.Now()), "调度器读到的状态已是停调")
}

func TestApplyAccountQuotaState_OpenAIAutoPause5hDisabled(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	account := openAIQuotaStateAccount(1002, map[string]any{
		"codex_5h_used_percent": 99.0, "auto_pause_5h_threshold": 0.95, "auto_pause_5h_disabled": true,
	})

	require.False(t, rl.ApplyAccountQuotaState(context.Background(), account))
	require.Equal(t, 0, repo.tempCalls)
	require.Nil(t, account.TempUnschedulableUntil)
}

func TestApplyAccountQuotaState_GrokRetryAfterPauses(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	retryAfter := 45
	account := &Account{ID: 1003, Platform: PlatformGrok, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: map[string]any{
		grokQuotaSnapshotExtraKey: xai.QuotaSnapshot{RetryAfterSeconds: &retryAfter, UpdatedAt: time.Now().UTC().Format(time.RFC3339)},
	}}

	require.True(t, rl.ApplyAccountQuotaState(context.Background(), account))
	require.Equal(t, 1, repo.tempCalls)
	payload, ok := parseTempUnschedReasonPayload(repo.lastTempReason)
	require.True(t, ok)
	require.Equal(t, grokQuotaAutoPauseSource, payload.Source)
}

func TestApplyAccountQuotaState_QuotaCounterPausesAnyType(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	account := &Account{ID: 1004, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: map[string]any{
		"quota_limit": 1.0, "quota_used": 1.0,
	}}

	require.True(t, rl.ApplyAccountQuotaState(context.Background(), account))
	require.Equal(t, 1, repo.tempCalls)
	payload, ok := parseTempUnschedReasonPayload(repo.lastTempReason)
	require.True(t, ok)
	require.Equal(t, quotaCounterSource, payload.Source)
}

func TestApplyAccountQuotaState_SameReasonNotRewritten(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	resetAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	account := openAIQuotaStateAccount(1005, map[string]any{
		"codex_5h_used_percent": 96.0, "auto_pause_5h_threshold": 0.95, "codex_5h_reset_at": resetAt.Format(time.RFC3339),
	})

	require.True(t, rl.ApplyAccountQuotaState(context.Background(), account))
	require.Equal(t, 1, repo.tempCalls)
	// 同一快照再评一次：同 until 同原因，不再写。
	require.True(t, rl.ApplyAccountQuotaState(context.Background(), account))
	require.Equal(t, 1, repo.tempCalls)
}

func TestApplyAccountQuotaState_AlreadyBlockedNotOverwritten(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	rateLimitReset := time.Now().Add(10 * time.Minute)
	account := openAIQuotaStateAccount(1006, map[string]any{
		"codex_5h_used_percent": 96.0, "auto_pause_5h_threshold": 0.95,
	})
	account.RateLimitResetAt = &rateLimitReset

	require.False(t, rl.ApplyAccountQuotaState(context.Background(), account))
	require.Equal(t, 0, repo.tempCalls)
}

func TestApplyAccountQuotaState_InactiveAccountSkipped(t *testing.T) {
	rl, repo := quotaStateTestService(t)
	account := openAIQuotaStateAccount(1007, map[string]any{"codex_5h_used_percent": 99.0, "auto_pause_5h_threshold": 0.95})
	account.Status = StatusDisabled

	require.False(t, rl.ApplyAccountQuotaState(context.Background(), account))
	require.Equal(t, 0, repo.tempCalls)
}
