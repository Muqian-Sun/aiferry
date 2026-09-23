//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// subscriptionUsageKeyCacheStub 只记录订阅用量增量落在哪个 (user, plan) 键上。
type subscriptionUsageKeyCacheStub struct {
	billingCacheWorkerStub

	mu   sync.Mutex
	keys [][2]int64
}

func (s *subscriptionUsageKeyCacheStub) UpdateSubscriptionUsage(_ context.Context, userID, planID int64, _ float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = append(s.keys, [2]int64{userID, planID})
	return nil
}

func (s *subscriptionUsageKeyCacheStub) recorded() [][2]int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][2]int64(nil), s.keys...)
}

// 订阅用量缓存的键是 (user, plan)——读侧 GetSubscriptionStatus 用 subscription.PlanID 拼键。
// 这里曾经传 apiKey.GroupID：4b 解耦订阅与分组后两者不是同一 ID 空间，增量会落到
// 没人读的键上，无分组 key 更是被 GroupID != nil 守卫整个跳过。
func TestFinalizePostUsageBillingQueuesSubscriptionUsageByPlanID(t *testing.T) {
	cache := &subscriptionUsageKeyCacheStub{}
	billing := NewBillingCacheService(cache, nil, nil, nil, nil, &config.Config{})
	t.Cleanup(billing.Stop)

	params := &postUsageBillingParams{
		User:               &User{ID: 42},
		APIKey:             &APIKey{ID: 7},
		Account:            &Account{ID: 3},
		Subscription:       &UserSubscription{ID: 900, UserID: 42, PlanID: 55},
		IsSubscriptionBill: true,
		Cost:               &CostBreakdown{ActualCost: 1.25},
	}
	deps := &billingDeps{billingCacheService: billing, deferredService: &DeferredService{}}

	finalizePostUsageBilling(context.Background(), params, deps, nil)

	require.Eventually(t, func() bool { return len(cache.recorded()) > 0 }, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, [][2]int64{{42, 55}}, cache.recorded())
}
