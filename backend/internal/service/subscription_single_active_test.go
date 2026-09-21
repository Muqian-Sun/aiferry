//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// 同一用户同一时间只允许一条有效订阅：已持有别的套餐 → 拒；同套餐 → 续期；别的套餐已过期 → 不拦。

func TestAssignOrExtend_RejectsDifferentPlanWhenActive(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID: 10, UserID: 1001, PlanID: 1,
		StartsAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(24 * time.Hour),
		Status: SubscriptionStatusActive,
	})
	keyRepo := &subscriptionKeyRepoStub{}
	svc := NewSubscriptionService(planRepo, subRepo, keyRepo, nil, nil, nil)

	for name, assign := range map[string]func() error{
		"assign": func() error {
			_, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{UserID: 1001, PlanID: 2, ValidityDays: 30})
			return err
		},
		"assign_or_extend": func() error {
			_, _, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{UserID: 1001, PlanID: 2, ValidityDays: 30})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := assign()
			require.ErrorIs(t, err, ErrSubscriptionAlreadyActive)
			require.Equal(t, infraerrors.Code(ErrSubscriptionAlreadyActive), infraerrors.Code(err))
			require.Equal(t, 0, subRepo.createCalls, "被拒时不能建新订阅")
			require.Empty(t, keyRepo.created, "被拒时不能发 key")
		})
	}
}

func TestAssignOrExtend_SamePlanRenews(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID: 10, UserID: 1001, PlanID: 1,
		StartsAt: start, ExpiresAt: start.AddDate(0, 0, 30),
		Status: SubscriptionStatusActive,
	})
	keyRepo := &subscriptionKeyRepoStub{exists: map[int64]bool{10: true}}
	svc := NewSubscriptionService(planRepo, subRepo, keyRepo, nil, nil, nil)

	sub, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{UserID: 1001, PlanID: 1, ValidityDays: 10})
	require.NoError(t, err)
	require.True(t, reused)
	require.Equal(t, int64(10), sub.ID)
	require.Equal(t, start.AddDate(0, 0, 40), sub.ExpiresAt, "同套餐续期在原到期上累加")
	require.Equal(t, 0, subRepo.createCalls)
	require.Empty(t, keyRepo.created, "已有订阅 key 不再补发")
}

func TestAssignOrExtend_ExpiredOtherPlanDoesNotBlock(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	// 别的套餐：status 仍 active（过期批处理没跑），但 expires_at 已过
	subRepo.seed(&UserSubscription{
		ID: 10, UserID: 1001, PlanID: 1,
		StartsAt: time.Now().AddDate(0, 0, -40), ExpiresAt: time.Now().AddDate(0, 0, -10),
		Status: SubscriptionStatusActive,
	})
	keyRepo := &subscriptionKeyRepoStub{}
	svc := NewSubscriptionService(planRepo, subRepo, keyRepo, nil, nil, nil)

	sub, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{UserID: 1001, PlanID: 2, ValidityDays: 30})
	require.NoError(t, err)
	require.False(t, reused)
	require.Equal(t, int64(2), sub.PlanID)
	require.Equal(t, 1, subRepo.createCalls)
}

func TestRestoreSubscription_RejectsWhenAnotherActive(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID: 20, UserID: 1001, PlanID: 2,
		StartsAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(24 * time.Hour),
		Status: SubscriptionStatusActive,
	})
	deletedAt := time.Now().Add(-time.Minute)
	subRepo.deleted = &UserSubscription{
		ID: 10, UserID: 1001, PlanID: 1,
		StartsAt: time.Now().Add(-2 * time.Hour), ExpiresAt: time.Now().Add(24 * time.Hour),
		Status: SubscriptionStatusActive, DeletedAt: &deletedAt,
	}
	svc := NewSubscriptionService(planRepo, subRepo, &subscriptionKeyRepoStub{}, nil, nil, nil)

	_, err := svc.RestoreSubscription(context.Background(), 10)
	require.ErrorIs(t, err, ErrSubscriptionAlreadyActive)
	require.False(t, subRepo.restored, "被拒时不能恢复行")
}
