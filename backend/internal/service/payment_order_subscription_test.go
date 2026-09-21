//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// 购买订阅：已持有别的套餐有效 → 409 SUBSCRIPTION_ALREADY_ACTIVE；同套餐（续费）放行；非在售套餐 404。
func TestValidateSubOrder_RejectsWhenAnotherPlanActive(t *testing.T) {
	planRepo := &planCRUDRepoStub{stored: &SubscriptionPlan{ID: 2, Name: "P2", ForSale: true, Models: []SubscriptionPlanModel{{EntryID: 27}}}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID: 10, UserID: 1001, PlanID: 1,
		StartsAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(24 * time.Hour),
		Status: SubscriptionStatusActive,
	})
	svc := &PaymentService{
		configService:   newPlanTestService(planRepo, &planCacheInvalidatorStub{}),
		subscriptionSvc: NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil),
	}

	_, err := svc.validateSubOrder(context.Background(), CreateOrderRequest{UserID: 1001, OrderType: payment.OrderTypeSubscription, PlanID: 2})
	require.ErrorIs(t, err, ErrSubscriptionAlreadyActive)
	require.Equal(t, "SUBSCRIPTION_ALREADY_ACTIVE", infraerrors.Reason(err))

	// 同套餐续费：plan 1 在售
	planRepo.stored = &SubscriptionPlan{ID: 1, Name: "P1", ForSale: true, Models: []SubscriptionPlanModel{{EntryID: 199}}}
	plan, err := svc.validateSubOrder(context.Background(), CreateOrderRequest{UserID: 1001, OrderType: payment.OrderTypeSubscription, PlanID: 1})
	require.NoError(t, err)
	require.Equal(t, int64(1), plan.ID)

	// 下架套餐
	planRepo.stored.ForSale = false
	_, err = svc.validateSubOrder(context.Background(), CreateOrderRequest{UserID: 1001, OrderType: payment.OrderTypeSubscription, PlanID: 1})
	require.Equal(t, "PLAN_NOT_AVAILABLE", infraerrors.Reason(err))
}
