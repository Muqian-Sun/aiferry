package middleware

import (
	"context"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// stubPlanRepo 固定套餐；plan 为 nil → ErrPlanNotFound
type stubPlanRepo struct {
	plan *service.SubscriptionPlan
}

func (r *stubPlanRepo) GetByID(context.Context, int64) (*service.SubscriptionPlan, error) {
	if r == nil || r.plan == nil {
		return nil, service.ErrPlanNotFound
	}
	cp := *r.plan
	return &cp, nil
}
func (*stubPlanRepo) List(context.Context, bool) ([]service.SubscriptionPlan, error) {
	return nil, errors.New("not implemented")
}
func (*stubPlanRepo) Create(context.Context, *service.SubscriptionPlan) error {
	return errors.New("not implemented")
}
func (*stubPlanRepo) Update(context.Context, *service.SubscriptionPlan) error {
	return errors.New("not implemented")
}
func (*stubPlanRepo) Delete(context.Context, int64) error { return errors.New("not implemented") }
