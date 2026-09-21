//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// planCRUDRepoStub 记录 Create / Update 的入参；GetByID 回放最近一次写入
type planCRUDRepoStub struct {
	planRepoNoop
	stored      *SubscriptionPlan
	createCalls int
	updateCalls int
}

func (r *planCRUDRepoStub) GetByID(_ context.Context, id int64) (*SubscriptionPlan, error) {
	if r.stored == nil || r.stored.ID != id {
		return nil, ErrPlanNotFound
	}
	cp := *r.stored
	return &cp, nil
}

func (r *planCRUDRepoStub) Create(_ context.Context, plan *SubscriptionPlan) error {
	r.createCalls++
	plan.ID = 7
	cp := *plan
	r.stored = &cp
	return nil
}

func (r *planCRUDRepoStub) Update(_ context.Context, plan *SubscriptionPlan) error {
	r.updateCalls++
	cp := *plan
	r.stored = &cp
	return nil
}

type planCacheInvalidatorStub struct{ ids []int64 }

func (s *planCacheInvalidatorStub) InvalidatePlanCache(_ context.Context, id int64) error {
	s.ids = append(s.ids, id)
	return nil
}

func newPlanTestService(repo SubscriptionPlanRepository, cache PlanCacheInvalidator) *PaymentConfigService {
	return NewPaymentConfigService(nil, nil, []byte("0123456789abcdef0123456789abcdef"), repo, cache)
}

// 模型集不允许为空：空切片 / 全是非法 ID 都拒，且不落库
func TestCreatePlan_RequiresModels(t *testing.T) {
	repo := &planCRUDRepoStub{}
	svc := newPlanTestService(repo, &planCacheInvalidatorStub{})
	base := CreatePlanRequest{Name: "Pro", Price: 9.9, ValidityDays: 30, ValidityUnit: "day"}

	for name, ids := range map[string][]int64{"empty": {}, "nil": nil, "all_invalid": {0, -1}} {
		t.Run(name, func(t *testing.T) {
			req := base
			req.EntryIDs = ids
			_, err := svc.CreatePlan(context.Background(), req)
			require.ErrorIs(t, err, ErrPlanModelsRequired)
			require.Zero(t, repo.createCalls)
		})
	}

	req := base
	req.EntryIDs = []int64{199, 199, 57}
	limit := 1.5
	req.DailyLimitUSD = &limit
	neg := -1.0
	req.WeeklyLimitUSD = &neg
	plan, err := svc.CreatePlan(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 1, repo.createCalls)
	require.Equal(t, []int64{199, 57}, plan.EntryIDs(), "去重保序")
	require.Equal(t, &limit, plan.DailyLimitUSD)
	require.Nil(t, plan.WeeklyLimitUSD, "负数 = 不限")
}

// 改套餐：写库后失效 plan:<id>；EntryIDs nil 不动模型集、空切片拒；限额 <=0 清成不限
func TestUpdatePlan_InvalidatesPlanCache(t *testing.T) {
	limit := 2.0
	repo := &planCRUDRepoStub{stored: &SubscriptionPlan{ID: 7, Name: "Pro", Price: 9.9, ValidityDays: 30, ValidityUnit: "day",
		DailyLimitUSD: &limit, Models: []SubscriptionPlanModel{{EntryID: 199}}}}
	cache := &planCacheInvalidatorStub{}
	svc := newPlanTestService(repo, cache)

	zero := 0.0
	name := "Pro+"
	plan, err := svc.UpdatePlan(context.Background(), 7, UpdatePlanRequest{Name: &name, DailyLimitUSD: &zero})
	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, []int64{7}, cache.ids, "改套餐后失效套餐缓存")
	require.Equal(t, "Pro+", plan.Name)
	require.Nil(t, plan.DailyLimitUSD, "0 = 清成不限")
	require.Equal(t, []int64{199}, plan.EntryIDs(), "EntryIDs nil 不动模型集")

	_, err = svc.UpdatePlan(context.Background(), 7, UpdatePlanRequest{EntryIDs: []int64{}})
	require.ErrorIs(t, err, ErrPlanModelsRequired)
	require.Equal(t, 1, repo.updateCalls, "被拒不写库")
	require.Len(t, cache.ids, 1, "被拒不失效")

	plan, err = svc.UpdatePlan(context.Background(), 7, UpdatePlanRequest{EntryIDs: []int64{57}})
	require.NoError(t, err)
	require.Equal(t, []int64{57}, plan.EntryIDs(), "非空覆盖模型集")
	require.Equal(t, []int64{7, 7}, cache.ids)

	_, err = svc.UpdatePlan(context.Background(), 8, UpdatePlanRequest{Name: &name})
	require.ErrorIs(t, err, ErrPlanNotFound)
}
