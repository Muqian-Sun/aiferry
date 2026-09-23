//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func schedulingStateFixture() *Account {
	return &Account{
		ID:          1,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
	}
}

func TestSchedulingState_AggregatesEachRawField(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)

	tests := []struct {
		name       string
		mutate     func(a *Account)
		wantReason string
		wantUntil  *time.Time
	}{
		{name: "clean", mutate: func(a *Account) {}, wantReason: ""},
		{name: "disabled", mutate: func(a *Account) { a.Status = StatusDisabled }, wantReason: "disabled"},
		{name: "manual unschedulable", mutate: func(a *Account) { a.Schedulable = false }, wantReason: "unschedulable"},
		{name: "expired with auto pause", mutate: func(a *Account) { a.AutoPauseOnExpired = true; a.ExpiresAt = &past }, wantReason: "expired"},
		{name: "expired without auto pause stays schedulable", mutate: func(a *Account) { a.ExpiresAt = &past }, wantReason: ""},
		{name: "overloaded", mutate: func(a *Account) { a.OverloadUntil = &future }, wantReason: "overloaded", wantUntil: &future},
		{name: "overload elapsed", mutate: func(a *Account) { a.OverloadUntil = &past }, wantReason: ""},
		{name: "rate limited", mutate: func(a *Account) { a.RateLimitResetAt = &future }, wantReason: "rate_limited", wantUntil: &future},
		{name: "temp unschedulable", mutate: func(a *Account) { a.TempUnschedulableUntil = &future; a.TempUnschedulableReason = "window_cost_limit" }, wantReason: "temp_unschedulable", wantUntil: &future},
		{name: "temp unschedulable elapsed", mutate: func(a *Account) { a.TempUnschedulableUntil = &past }, wantReason: ""},
		{name: "quota counter no longer blocks here", mutate: func(a *Account) {
			a.Type = AccountTypeAPIKey
			a.Extra = map[string]any{"quota_limit": 1.0, "quota_used": 2.0}
		}, wantReason: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := schedulingStateFixture()
			tt.mutate(account)
			state := account.SchedulingState(now)
			require.Equal(t, tt.wantReason, state.Reason)
			require.Equal(t, tt.wantReason != "", state.Blocked)
			require.Equal(t, tt.wantReason == "", state.Allows(now))
			if tt.wantUntil != nil {
				require.NotNil(t, state.BlockedUntil)
				require.True(t, tt.wantUntil.Equal(*state.BlockedUntil))
			} else {
				require.Nil(t, state.BlockedUntil)
			}
			if tt.name == "temp unschedulable" {
				require.Equal(t, "window_cost_limit", state.Detail)
			}
		})
	}
}

func TestSchedulingState_NilAccountIsMissing(t *testing.T) {
	var account *Account
	state := account.SchedulingState(time.Now())
	require.True(t, state.Blocked)
	require.Equal(t, "missing", state.Reason)
	require.False(t, account.SchedulingAllows(context.Background(), "claude-sonnet-4-5", time.Now()))
}

func TestSchedulingState_ModelBlocksOnlyBlockTheirScope(t *testing.T) {
	now := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	account := schedulingStateFixture()
	account.Extra = map[string]any{
		"model_rate_limits": map[string]any{
			"claude-opus-4":    map[string]any{"rate_limit_reset_at": now.Add(time.Hour).Format(time.RFC3339)},
			"claude-haiku-4-5": map[string]any{"rate_limit_reset_at": now.Add(-time.Hour).Format(time.RFC3339)},
		},
	}

	state := account.SchedulingState(now)
	require.False(t, state.Blocked)
	require.Len(t, state.ModelBlocks, 1, "elapsed scopes are dropped")
	require.False(t, state.Allows(now, "claude-opus-4"))
	require.True(t, state.Allows(now, "claude-haiku-4-5"))
	require.True(t, state.Allows(now))

	ctx := context.Background()
	require.False(t, account.SchedulingAllows(ctx, "claude-opus-4", now))
	require.True(t, account.SchedulingAllows(ctx, "claude-sonnet-4-5", now))
	require.True(t, account.SchedulingAllows(ctx, "", now))
}

func TestSchedulingState_AntigravityOveragesWaiveModelBlocks(t *testing.T) {
	// 积分耗尽判定（isRateLimitActiveForKey）按真实时钟比较，now 必须跟着走，不能钉死日期。
	now := time.Now().UTC()
	account := schedulingStateFixture()
	account.Platform = PlatformAntigravity
	account.Extra = map[string]any{
		"allow_overages": true,
		"model_rate_limits": map[string]any{
			"claude-sonnet-4-5": map[string]any{"rate_limit_reset_at": now.Add(time.Hour).Format(time.RFC3339)},
		},
	}

	require.True(t, account.SchedulingState(now).ModelBlocksWaived)
	require.True(t, account.SchedulingAllows(context.Background(), "claude-sonnet-4-5", now))

	// 积分耗尽后不再放行。
	account.Extra["model_rate_limits"].(map[string]any)[creditsExhaustedKey] = map[string]any{"rate_limit_reset_at": now.Add(time.Hour).Format(time.RFC3339)}
	require.False(t, account.SchedulingState(now).ModelBlocksWaived)
	require.False(t, account.SchedulingAllows(context.Background(), "claude-sonnet-4-5", now))

	// 标签为 antigravity 的第三方 key 不享有积分放行。
	key := schedulingStateFixture()
	key.Platform = PlatformAntigravity
	key.Type = AccountTypeAPIKey
	key.ProtocolEndpoints = map[string]string{APIProtocolAnthropic: "https://relay.example.com"}
	key.Extra = map[string]any{
		"allow_overages": true,
		"model_rate_limits": map[string]any{
			"claude-sonnet-4-5": map[string]any{"rate_limit_reset_at": now.Add(time.Hour).Format(time.RFC3339)},
		},
	}
	require.False(t, key.SchedulingState(now).ModelBlocksWaived)
}

// 旧入口是薄封装：结论必须与 SchedulingState 一致。
func TestIsSchedulableWrapsSchedulingState(t *testing.T) {
	future := time.Now().Add(time.Hour)
	account := schedulingStateFixture()
	require.True(t, account.IsSchedulable())
	require.Equal(t, "", SchedulingBlockedReason(account))
	account.RateLimitResetAt = &future
	require.False(t, account.IsSchedulable())
	require.Equal(t, "rate_limited", SchedulingBlockedReason(account))
	require.False(t, account.IsSchedulableForModelWithContext(context.Background(), "claude-sonnet-4-5"))
}
