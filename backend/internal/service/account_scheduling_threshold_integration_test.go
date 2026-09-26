//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type thresholdSelectionAccountRepoStub struct {
	rateLimitAccountRepoStub
	accounts []Account
}

func (r *thresholdSelectionAccountRepoStub) ListSchedulingCandidates(_ context.Context, platforms []string) ([]Account, error) {
	filtered := make([]Account, 0, len(r.accounts))
	for _, account := range r.accounts {
		if schedulingCandidateMatchesForTest(account, platforms) {
			filtered = append(filtered, account)
		}
	}
	return filtered, nil
}

func (r *thresholdSelectionAccountRepoStub) ListSchedulingCandidatesByCatalogEntry(context.Context, int64) ([]Account, error) {
	return nil, nil
}

func (r *thresholdSelectionAccountRepoStub) ListSchedulingCandidatesByGroupID(ctx context.Context, _ int64, platforms []string) ([]Account, error) {
	return r.ListSchedulingCandidates(ctx, platforms)
}

func TestGatewayService_ListSchedulableAccounts_DoesNotFilterUnsupportedThresholdPlatforms(t *testing.T) {
	setGatewayPolicyForTest(t, &accountSchedulingThresholds, map[string]int{PlatformOpenAI: 90, PlatformAnthropic: 100, PlatformGrok: 100})

	accountRepo := &thresholdSelectionAccountRepoStub{
		accounts: []Account{
			{
				ID:          3101,
				Platform:    PlatformKiro,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{
					"account_scheduling_threshold": 1,
				},
				Extra: map[string]any{
					"kiro_sched_utilization": 95.0,
					"kiro_sched_reset_at":    time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
				},
			},
			{
				ID:          3102,
				Platform:    PlatformKiro,
				Status:      StatusActive,
				Schedulable: true,
				Extra: map[string]any{
					"kiro_sched_utilization": 42.0,
					"kiro_sched_reset_at":    time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
				},
			},
		},
	}

	rateLimitService := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	svc := &GatewayService{
		accountRepo:      accountRepo,
		cfg:              &config.Config{},
		rateLimitService: rateLimitService,
	}

	accounts, err := svc.listSchedulableAccounts(context.Background(), PlatformKiro, false)

	require.NoError(t, err)
	require.Len(t, accounts, 2)
	require.Equal(t, int64(3101), accounts[0].ID)
	require.Equal(t, int64(3102), accounts[1].ID)
	require.Equal(t, 0, accountRepo.tempCalls)
}

// 阈值评估不在选号路径上：候选装载不评估阈值、不写状态（tempCalls 为 0）；
// 已被状态服务停调的账号由选号循环按 SchedulingState 跳过（见 *_LoadBalanceTopKExcludesTempUnschedulable）。
func TestGatewayService_ListSchedulableAccounts_OpenAIPool_ReadsStateNotThresholds(t *testing.T) {
	setGatewayPolicyForTest(t, &accountSchedulingThresholds, map[string]int{PlatformOpenAI: 85, PlatformAnthropic: 100, PlatformGrok: 100})

	accountRepo := &thresholdSelectionAccountRepoStub{
		accounts: []Account{
			{
				ID:          4101,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Extra: map[string]any{
					"codex_7d_used_percent": 91.0,
					"codex_7d_reset_at":     time.Now().UTC().Add(12 * time.Hour).Format(time.RFC3339),
				},
			},
			{
				ID:                      4103,
				Platform:                PlatformOpenAI,
				Status:                  StatusActive,
				Schedulable:             true,
				TempUnschedulableUntil:  ptrTime(time.Now().Add(12 * time.Hour)),
				TempUnschedulableReason: BuildTempUnschedReasonPayload(AccountSchedulingThresholdReasonSource, "paused by state service"),
			},
			{
				ID:          4102,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Extra: map[string]any{
					"codex_7d_used_percent": 40.0,
					"codex_7d_reset_at":     time.Now().UTC().Add(12 * time.Hour).Format(time.RFC3339),
				},
			},
		},
	}

	rateLimitService := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	svc := &GatewayService{
		accountRepo:      accountRepo,
		cfg:              &config.Config{},
		rateLimitService: rateLimitService,
	}

	accounts, err := svc.listSchedulableAccounts(context.Background(), PlatformOpenAI, false)

	require.NoError(t, err)
	require.Len(t, accounts, 3, "装载不按阈值过滤")
	require.Equal(t, 0, accountRepo.tempCalls, "选号路径不写状态")
}
