//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRateLimitService_ApplyAccountSchedulingThreshold_SetsTempUnschedulable(t *testing.T) {
	setGatewayPolicyForTest(t, &accountSchedulingThresholds, map[string]int{PlatformOpenAI: 80, PlatformAnthropic: 100, PlatformGrok: 100})

	accountRepo := &rateLimitAccountRepoStub{}
	rl := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	until := time.Now().UTC().Add(6 * time.Hour)
	account := &Account{
		ID:          1001,
		Platform:    PlatformOpenAI,
		Status:      StatusActive,
		Schedulable: true,
		Extra: map[string]any{
			"codex_7d_used_percent": 91.5,
			"codex_7d_reset_at":     until.Format(time.RFC3339),
		},
	}

	blocked := rl.ApplyAccountSchedulingThreshold(context.Background(), account)

	require.True(t, blocked)
	require.Equal(t, 1, accountRepo.tempCalls)
	require.NotNil(t, account.TempUnschedulableUntil)
	require.WithinDuration(t, until, *account.TempUnschedulableUntil, time.Second)
	require.True(t, IsAccountSchedulingThresholdReason(accountRepo.lastTempReason))

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(accountRepo.lastTempReason), &payload))
	require.Equal(t, PlatformOpenAI, payload["platform"])
	require.Equal(t, "7d", payload["window"])
	require.Equal(t, float64(80), payload["threshold_percent"])
	require.Equal(t, float64(91.5), payload["used_percent"])
	require.Contains(t, payload["error_message"], "91.5% used >= 80%")
}

func TestRateLimitService_ApplyAccountSchedulingThreshold_UsesAccountOverrideInReason(t *testing.T) {
	setGatewayPolicyForTest(t, &accountSchedulingThresholds, map[string]int{PlatformOpenAI: 90, PlatformAnthropic: 100, PlatformGrok: 100})

	accountRepo := &rateLimitAccountRepoStub{}
	rl := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	until := time.Now().UTC().Add(6 * time.Hour)
	account := &Account{
		ID:          1003,
		Platform:    PlatformOpenAI,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"account_scheduling_threshold": 80,
		},
		Extra: map[string]any{
			"codex_7d_used_percent": 85.5,
			"codex_7d_reset_at":     until.Format(time.RFC3339),
		},
	}

	blocked := rl.ApplyAccountSchedulingThreshold(context.Background(), account)

	require.True(t, blocked)
	require.Equal(t, 1, accountRepo.tempCalls)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(accountRepo.lastTempReason), &payload))
	require.Equal(t, float64(80), payload["threshold_percent"])
	require.Equal(t, float64(85.5), payload["used_percent"])
	require.Contains(t, payload["error_message"], "85.5% used >= 80%")
}

// 代码里的平台阈值是 openai / anthropic / grok 全 100（= 不停调）：用量再高、窗口没重置也不停。
// 由 TestGetAccountSchedulingThresholds_NilRepoReturnsDefaults（断言默认阈值全 100）改来：读取函数删了，改成断言默认值下的行为。
func TestRateLimitService_ApplyAccountSchedulingThreshold_CodeDefaultsNeverPause(t *testing.T) {
	until := time.Now().UTC().Add(6 * time.Hour)
	accounts := []*Account{
		{ID: 1101, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Extra: map[string]any{
			"codex_7d_used_percent": 99.0,
			"codex_7d_reset_at":     until.Format(time.RFC3339),
		}},
		{ID: 1102, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Extra: map[string]any{
			"passive_usage_7d_utilization": 0.99,
			"passive_usage_7d_reset":       float64(until.Unix()),
		}},
		{ID: 1103, Platform: PlatformGrok, Status: StatusActive, Schedulable: true, Extra: map[string]any{
			"grok_sched_utilization": 99.0,
			"grok_sched_reset_at":    until.Format(time.RFC3339),
		}},
	}
	for _, account := range accounts {
		accountRepo := &rateLimitAccountRepoStub{}
		rl := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

		require.False(t, rl.ApplyAccountSchedulingThreshold(context.Background(), account), account.Platform)
		require.Zero(t, accountRepo.tempCalls, account.Platform)
		require.Nil(t, account.TempUnschedulableUntil, account.Platform)
	}
}

type fableSchedulingThresholdRepoStub struct {
	rateLimitAccountRepoStub
	modelCalls      int
	lastModelScope  string
	lastModelReset  time.Time
	lastModelReason string
}

func (r *fableSchedulingThresholdRepoStub) SetModelRateLimit(_ context.Context, _ int64, scope string, resetAt time.Time, reason ...string) error {
	r.modelCalls++
	r.lastModelScope = scope
	r.lastModelReset = resetAt
	if len(reason) > 0 {
		r.lastModelReason = reason[0]
	}
	return nil
}

func TestRateLimitService_ApplyAccountSchedulingThreshold_FableOnlyLimitsFableModels(t *testing.T) {
	accountRepo := &fableSchedulingThresholdRepoStub{}
	rl := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	until := time.Now().UTC().Add(4 * 24 * time.Hour).Truncate(time.Second)
	account := &Account{
		ID:          1004,
		Platform:    PlatformAnthropic,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"account_scheduling_threshold": 60,
		},
		Extra: map[string]any{
			"passive_usage_7d_utilization":    0.40,
			"passive_usage_7d_reset":          float64(time.Now().UTC().Add(3 * 24 * time.Hour).Unix()),
			"passive_usage_7d_oi_utilization": 0.61,
			"passive_usage_7d_oi_reset":       float64(until.Unix()),
		},
	}

	blocked := rl.ApplyAccountSchedulingThreshold(context.Background(), account)

	require.False(t, blocked, "the Fable-only window must not pause the whole account")
	require.Zero(t, accountRepo.tempCalls)
	require.Equal(t, 1, accountRepo.modelCalls)
	require.Equal(t, anthropicFableRateLimitKey, accountRepo.lastModelScope)
	require.WithinDuration(t, until, accountRepo.lastModelReset, time.Second)
	require.True(t, IsAccountSchedulingThresholdReason(accountRepo.lastModelReason))
	require.False(t, account.IsSchedulableForModel("claude-fable-5"))
	require.False(t, account.IsSchedulableForModel("claude-fable-5[1m]"))
	require.True(t, account.IsSchedulableForModel("claude-opus-4-8"))
	require.True(t, account.IsSchedulableForModel("claude-sonnet-4-6"))

	blocked = rl.ApplyAccountSchedulingThreshold(context.Background(), account)
	require.False(t, blocked)
	require.Equal(t, 1, accountRepo.modelCalls, "an active model limit should not be persisted twice")
}

func TestRateLimitService_ApplyAccountSchedulingThreshold_SkipsDuplicateTempUnschedulable(t *testing.T) {
	setGatewayPolicyForTest(t, &accountSchedulingThresholds, map[string]int{PlatformOpenAI: 80, PlatformAnthropic: 100, PlatformGrok: 100})

	accountRepo := &rateLimitAccountRepoStub{}
	rl := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	until := time.Now().UTC().Add(6 * time.Hour).Truncate(time.Second)
	existingReason := BuildDetailedAccountSchedulingThresholdReason(AccountSchedulingThresholdReasonInput{
		Platform:         PlatformOpenAI,
		Window:           "7d",
		ThresholdPercent: 80,
		UsedPercent:      91.5,
		Until:            until,
		Now:              until.Add(-time.Hour),
	})
	account := &Account{
		ID:                      1002,
		Platform:                PlatformOpenAI,
		Status:                  StatusActive,
		Schedulable:             true,
		TempUnschedulableUntil:  &until,
		TempUnschedulableReason: existingReason,
		Extra: map[string]any{
			"codex_7d_used_percent": 91.5,
			"codex_7d_reset_at":     until.Format(time.RFC3339),
		},
	}

	blocked := rl.ApplyAccountSchedulingThreshold(context.Background(), account)

	require.True(t, blocked)
	require.Equal(t, 0, accountRepo.tempCalls)
	require.Equal(t, existingReason, account.TempUnschedulableReason)
	require.NotNil(t, account.TempUnschedulableUntil)
	require.True(t, until.Equal(*account.TempUnschedulableUntil))
}

func TestRateLimitService_ApplyAccountSchedulingThreshold_UnsupportedPlatformDoesNotBlock(t *testing.T) {
	setGatewayPolicyForTest(t, &accountSchedulingThresholds, map[string]int{PlatformOpenAI: 80, PlatformAnthropic: 100, PlatformGrok: 100})

	accountRepo := &rateLimitAccountRepoStub{}
	rl := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	account := &Account{
		ID:          2002,
		Platform:    PlatformKiro,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"account_scheduling_threshold": 1,
		},
		Extra: map[string]any{
			"kiro_sched_utilization": 99.0,
			"kiro_sched_reset_at":    time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		},
	}

	blocked := rl.ApplyAccountSchedulingThreshold(context.Background(), account)

	require.False(t, blocked)
	require.Equal(t, 0, accountRepo.tempCalls)
	require.Nil(t, account.TempUnschedulableUntil)
	require.Empty(t, account.TempUnschedulableReason)
}
