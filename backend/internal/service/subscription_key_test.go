//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 订阅 key 随订阅生成：名字 = 套餐名、无分组、绑到订阅行；续期 / 恢复时缺了才补。

func TestCreateSubscription_GeneratesBoundKey(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{Name: "E2E Pro"}}
	subRepo := newSubscriptionUserSubRepoStub()
	keyRepo := &subscriptionKeyRepoStub{}
	cfg := &config.Config{}
	cfg.Default.APIKeyPrefix = "tf-"
	svc := NewSubscriptionService(planRepo, subRepo, keyRepo, nil, nil, cfg)

	sub, err := svc.AssignSubscription(context.Background(), &AssignSubscriptionInput{UserID: 1001, PlanID: 7, ValidityDays: 30})
	require.NoError(t, err)
	require.Len(t, keyRepo.created, 1)
	key := keyRepo.created[0]
	require.NotNil(t, key.SubscriptionID)
	require.Equal(t, sub.ID, *key.SubscriptionID)
	require.Equal(t, int64(1001), key.UserID)
	require.Equal(t, "E2E Pro", key.Name)
	require.Equal(t, StatusActive, key.Status)
	require.True(t, strings.HasPrefix(key.Key, "tf-"), "前缀来自 cfg.Default.APIKeyPrefix，got %q", key.Key)
	require.Len(t, key.Key, len("tf-")+64)
}

func TestEnsureSubscriptionKey_SkipsWhenExists(t *testing.T) {
	keyRepo := &subscriptionKeyRepoStub{exists: map[int64]bool{10: true}}
	svc := NewSubscriptionService(planRepoNoop{}, userSubRepoNoop{}, keyRepo, nil, nil, nil)
	require.NoError(t, svc.ensureSubscriptionKey(context.Background(), &UserSubscription{ID: 10, UserID: 1}, "plan"))
	require.Empty(t, keyRepo.created)
}

func TestEnsureSubscriptionKey_RecreatesAfterAdminDelete(t *testing.T) {
	keyRepo := &subscriptionKeyRepoStub{exists: map[int64]bool{10: false}}
	svc := NewSubscriptionService(planRepoNoop{}, userSubRepoNoop{}, keyRepo, nil, nil, nil)
	require.NoError(t, svc.ensureSubscriptionKey(context.Background(), &UserSubscription{ID: 10, UserID: 1}, "plan"))
	require.Len(t, keyRepo.created, 1)
	require.True(t, strings.HasPrefix(keyRepo.created[0].Key, "sk-"), "cfg 为空按 sk-")
}

// 续期同一订阅行时若 key 被管理端删了要补一把
func TestAssignOrExtend_RenewalRecreatesMissingKey(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{ID: 10, UserID: 1001, PlanID: 1, StartsAt: start, ExpiresAt: start.AddDate(0, 0, 30), Status: SubscriptionStatusActive})
	keyRepo := &subscriptionKeyRepoStub{}
	svc := NewSubscriptionService(planRepo, subRepo, keyRepo, nil, nil, nil)

	_, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{UserID: 1001, PlanID: 1, ValidityDays: 10})
	require.NoError(t, err)
	require.True(t, reused)
	require.Len(t, keyRepo.created, 1)
	require.Equal(t, int64(10), *keyRepo.created[0].SubscriptionID)
}

func TestRestoreSubscription_RecreatesMissingKey(t *testing.T) {
	planRepo := &subscriptionPlanRepoStub{plan: &SubscriptionPlan{Name: "plan"}}
	subRepo := newSubscriptionUserSubRepoStub()
	deletedAt := time.Now().Add(-time.Minute)
	subRepo.deleted = &UserSubscription{ID: 10, UserID: 1001, PlanID: 1, StartsAt: time.Now().Add(-2 * time.Hour), ExpiresAt: time.Now().Add(24 * time.Hour), Status: SubscriptionStatusActive, DeletedAt: &deletedAt}
	keyRepo := &subscriptionKeyRepoStub{}
	svc := NewSubscriptionService(planRepo, subRepo, keyRepo, nil, nil, nil)

	restored, err := svc.RestoreSubscription(context.Background(), 10)
	require.NoError(t, err)
	require.True(t, subRepo.restored)
	require.Nil(t, restored.DeletedAt)
	require.Len(t, keyRepo.created, 1)
	require.Equal(t, "plan", keyRepo.created[0].Name)
}
