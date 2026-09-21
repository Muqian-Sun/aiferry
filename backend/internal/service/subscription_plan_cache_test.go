//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// countingPlanRepo 记录 GetByID 回源次数：套餐 L1 命中时不该再查库
type countingPlanRepo struct {
	planRepoNoop
	plan  *SubscriptionPlan
	calls int
}

func (r *countingPlanRepo) GetByID(_ context.Context, id int64) (*SubscriptionPlan, error) {
	r.calls++
	cp := *r.plan
	cp.ID = id
	return &cp, nil
}

// 改套餐只失效 plan:<id> 这一个键；失效前命中 L1 不回源，失效后再取触发第二次回源。
func TestInvalidatePlanCache_DropsL1(t *testing.T) {
	planRepo := &countingPlanRepo{plan: &SubscriptionPlan{Name: "plan"}}
	svc := NewSubscriptionService(planRepo, userSubRepoNoop{}, nil, nil, nil, &config.Config{
		SubscriptionCache: config.SubscriptionCacheConfig{L1Size: 1024, L1TTLSeconds: 60},
	})
	t.Cleanup(svc.Stop)

	_, err := svc.GetPlan(context.Background(), 20)
	require.NoError(t, err)
	svc.subCacheL1.Wait()
	_, err = svc.GetPlan(context.Background(), 20)
	require.NoError(t, err)
	require.Equal(t, 1, planRepo.calls, "第二次应命中 L1")

	require.NoError(t, svc.InvalidatePlanCache(context.Background(), 20))
	_, err = svc.GetPlan(context.Background(), 20)
	require.NoError(t, err)
	require.Equal(t, 2, planRepo.calls, "失效后应回源")
}

// 订阅 key 鉴权取订阅：订阅与套餐各自 L1；套餐挂在返回的浅拷贝上
func TestGetActiveSubscription_AttachesPlanFromCache(t *testing.T) {
	planRepo := &countingPlanRepo{plan: &SubscriptionPlan{Name: "plan"}}
	repo := &revokeCacheUserSubRepoStub{sub: &UserSubscription{ID: 1, UserID: 10, PlanID: 20, Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)}}
	svc := NewSubscriptionService(planRepo, repo, nil, nil, nil, &config.Config{
		SubscriptionCache: config.SubscriptionCacheConfig{L1Size: 1024, L1TTLSeconds: 60},
	})
	t.Cleanup(svc.Stop)

	sub, err := svc.GetActiveSubscription(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, sub.Plan)
	require.Equal(t, int64(20), sub.Plan.ID)
	require.Equal(t, "plan", sub.Plan.Name)
	svc.subCacheL1.Wait()

	again, err := svc.GetActiveSubscription(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, repo.getActiveCalls, "订阅命中 L1")
	require.Equal(t, 1, planRepo.calls, "套餐命中 L1")
	again.DailyUsageUSD = 99
	third, _ := svc.GetActiveSubscription(context.Background(), 1)
	require.Zero(t, third.DailyUsageUSD, "返回浅拷贝，改字段不污染缓存")
}

// GetActiveSubscription 的错误分类：订阅无效（NotFound）与基础设施错误要能区分
func TestIsSubscriptionInactiveError(t *testing.T) {
	require.True(t, IsSubscriptionInactiveError(ErrSubscriptionNotFound))
	require.True(t, IsSubscriptionInactiveError(ErrSubscriptionExpired))
	require.True(t, IsSubscriptionInactiveError(ErrSubscriptionSuspended))
	require.False(t, IsSubscriptionInactiveError(context.DeadlineExceeded))
	require.False(t, IsSubscriptionInactiveError(ErrPlanNotFound))
}
