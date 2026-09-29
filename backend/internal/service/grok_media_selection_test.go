package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// grok 视频状态轮询只认任务归属账号：归属账号可用就选中它（抢槽 / 等待计划），
// 不可用（停调 / 不在分组 / 不存在 / 无效 id）就是无候选——绝不落到别的账号，也不刷新归属键。
func TestSelectGrokMediaVideoRequestAccountPreservesOwner(t *testing.T) {
	for _, state := range []string{"available", "full", "unavailable", "missing", "invalid id"} {
		t.Run(state, func(t *testing.T) {
			ownerID := int64(1)
			// 视频任务的归属账号是 Grok 成品号（xAI 媒体只对成品号，第三方 key 一律按中转，2026-09-29）。
			owner := Account{ID: ownerID, Platform: PlatformGrok, Type: AccountTypeOAuth,
				Status: StatusActive, Schedulable: true, Concurrency: 50}
			other := owner
			other.ID = 2
			if state == "unavailable" {
				until := time.Now().Add(time.Minute)
				owner.TempUnschedulableUntil = &until
			}
			accounts := []Account{owner, other}
			if state == "missing" {
				accounts = []Account{other}
			}
			if state == "invalid id" {
				ownerID = 0
			}
			var acquired, released []int64
			cache := &schedulerTestGatewayCache{}
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.StickySessionWaitTimeout = time.Second
			cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
			repo := schedulerTestOpenAIAccountRepo{accounts: accounts}
			concurrency := NewConcurrencyService(schedulerTestConcurrencyCache{
				acquireResults: map[int64]bool{1: state != "full", 2: true},
				acquiredIDs:    &acquired, releasedIDs: &released,
			})
			scheduler := &GatewayService{
				accountRepo:        repo,
				cache:              cache,
				cfg:                cfg,
				concurrencyService: concurrency,
			}
			svc := &OpenAIGatewayService{accountRepo: repo, cache: cache, cfg: cfg, concurrencyService: concurrency, scheduler: scheduler}
			ctx := context.Background()
			require.NoError(t, svc.BindGrokMediaVideoRequestAccount(ctx, "task", 10, 20, 1))
			sessionHash := GrokMediaVideoRequestSessionHash("task", 10, 20)
			for range 20 {
				selection, err := svc.SelectGrokMediaVideoRequestAccount(ctx, sessionHash, ownerID, "")
				switch state {
				case "available":
					require.NoError(t, err)
					require.Equal(t, int64(1), selection.Account.ID)
					require.True(t, selection.Acquired)
					selection.ReleaseFunc()
				case "full":
					require.NoError(t, err)
					require.Equal(t, int64(1), selection.Account.ID)
					require.False(t, selection.Acquired)
					require.Nil(t, selection.ReleaseFunc)
					require.Equal(t, int64(1), selection.WaitPlan.AccountID)
				default:
					require.ErrorIs(t, err, ErrNoAvailableAccounts)
					require.Nil(t, selection)
				}
				bound, err := svc.ResolveGrokMediaVideoRequestAccount(ctx, "task", 10, 20)
				require.NoError(t, err)
				require.Equal(t, int64(1), bound)
			}
			require.NotContains(t, acquired, int64(2), "the pool must never touch a non-owner account")
			require.Empty(t, cache.deletedSessions)
			if state == "available" {
				require.Len(t, released, 20)
			} else {
				require.Empty(t, released)
			}
		})
	}
}
