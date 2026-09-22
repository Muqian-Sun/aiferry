//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// planLimitCacheStub 订阅用量来自缓存；余额分支只看 balance。记两个分支各自被调的次数。
type planLimitCacheStub struct {
	billingCacheWorkerStub

	sub          *SubscriptionCacheData
	balance      float64
	subCalls     atomic.Int64
	balanceCalls atomic.Int64
}

func (s *planLimitCacheStub) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	s.subCalls.Add(1)
	return s.sub, nil
}

func (s *planLimitCacheStub) GetUserBalance(context.Context, int64) (float64, error) {
	s.balanceCalls.Add(1)
	return s.balance, nil
}

// 订阅 key：限额取 sub.Plan，不再看分组（group 传 nil 也能判）
func TestCheckBillingEligibility_SubscriptionUsesPlanLimits(t *testing.T) {
	limit := 1.0
	cache := &planLimitCacheStub{sub: &SubscriptionCacheData{
		Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour), DailyUsage: 1.0,
	}}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	sub := &UserSubscription{UserID: 1, PlanID: 20, Plan: &SubscriptionPlan{ID: 20, DailyLimitUSD: &limit}}
	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, sub)
	require.ErrorIs(t, err, ErrDailyLimitExceeded)
	require.EqualValues(t, 1, cache.subCalls.Load())
	require.Zero(t, cache.balanceCalls.Load(), "订阅 key 不查余额")

	// 套餐无限额 → 放行
	sub.Plan = &SubscriptionPlan{ID: 20}
	require.NoError(t, svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, sub))

	// 周 / 月限额同样从套餐取
	cache.sub.WeeklyUsage = 5
	sub.Plan = &SubscriptionPlan{ID: 20, WeeklyLimitUSD: &limit}
	require.ErrorIs(t, svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, sub), ErrWeeklyLimitExceeded)
	cache.sub.MonthlyUsage = 5
	sub.Plan = &SubscriptionPlan{ID: 20, MonthlyLimitUSD: &limit}
	require.ErrorIs(t, svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, sub), ErrMonthlyLimitExceeded)
}

// 余额 key（ctx 无订阅）：走余额，订阅缓存零调用
func TestCheckBillingEligibility_BalanceKeyIgnoresSubscription(t *testing.T) {
	cache := &planLimitCacheStub{balance: 0}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, cfg)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, &APIKey{ID: 5}, nil, nil)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.EqualValues(t, 1, cache.balanceCalls.Load())
	require.Zero(t, cache.subCalls.Load())
}
