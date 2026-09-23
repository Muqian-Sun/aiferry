package repository

import (
	"context"
	"errors"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplanmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// subscriptionPlanRepository 套餐仓储：主表 + subscription_plan_models 模型集。
type subscriptionPlanRepository struct {
	client *dbent.Client
}

func NewSubscriptionPlanRepository(client *dbent.Client) service.SubscriptionPlanRepository {
	return &subscriptionPlanRepository{client: client}
}

func (r *subscriptionPlanRepository) GetByID(ctx context.Context, id int64) (*service.SubscriptionPlan, error) {
	m, err := clientFromContext(ctx, r.client).SubscriptionPlan.Query().
		Where(subscriptionplan.IDEQ(id)).
		WithModels(func(q *dbent.SubscriptionPlanModelQuery) { q.WithEntry() }).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrPlanNotFound
		}
		return nil, err
	}
	return subscriptionPlanEntityToService(m), nil
}

func (r *subscriptionPlanRepository) List(ctx context.Context, forSaleOnly bool) ([]service.SubscriptionPlan, error) {
	q := clientFromContext(ctx, r.client).SubscriptionPlan.Query().
		WithModels(func(q *dbent.SubscriptionPlanModelQuery) { q.WithEntry() }).
		Order(subscriptionplan.BySortOrder(), subscriptionplan.ByID())
	if forSaleOnly {
		q = q.Where(subscriptionplan.ForSaleEQ(true))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.SubscriptionPlan, 0, len(rows))
	for _, m := range rows {
		out = append(out, *subscriptionPlanEntityToService(m))
	}
	return out, nil
}

func (r *subscriptionPlanRepository) Create(ctx context.Context, plan *service.SubscriptionPlan) error {
	return r.withTx(ctx, func(tx *dbent.Tx) error {
		b := tx.SubscriptionPlan.Create().
			SetName(plan.Name).
			SetDescription(plan.Description).
			SetPrice(plan.Price).
			SetNillableOriginalPrice(plan.OriginalPrice).
			SetValidityDays(plan.ValidityDays).
			SetValidityUnit(plan.ValidityUnit).
			SetFeatures(plan.Features).
			SetProductName(plan.ProductName).
			SetForSale(plan.ForSale).
			SetSortOrder(plan.SortOrder).
			SetNillableDailyLimitUsd(plan.DailyLimitUSD).
			SetNillableWeeklyLimitUsd(plan.WeeklyLimitUSD).
			SetNillableMonthlyLimitUsd(plan.MonthlyLimitUSD)
		m, err := b.Save(ctx)
		if err != nil {
			return err
		}
		plan.ID = m.ID
		plan.CreatedAt = m.CreatedAt
		plan.UpdatedAt = m.UpdatedAt
		return replacePlanModels(ctx, tx, plan.ID, plan.Models)
	})
}

func (r *subscriptionPlanRepository) Update(ctx context.Context, plan *service.SubscriptionPlan) error {
	return r.withTx(ctx, func(tx *dbent.Tx) error {
		u := tx.SubscriptionPlan.UpdateOneID(plan.ID).
			SetName(plan.Name).
			SetDescription(plan.Description).
			SetPrice(plan.Price).
			SetValidityDays(plan.ValidityDays).
			SetValidityUnit(plan.ValidityUnit).
			SetFeatures(plan.Features).
			SetProductName(plan.ProductName).
			SetForSale(plan.ForSale).
			SetSortOrder(plan.SortOrder)
		if plan.OriginalPrice != nil {
			u.SetOriginalPrice(*plan.OriginalPrice)
		} else {
			u.ClearOriginalPrice()
		}
		if plan.DailyLimitUSD != nil {
			u.SetDailyLimitUsd(*plan.DailyLimitUSD)
		} else {
			u.ClearDailyLimitUsd()
		}
		if plan.WeeklyLimitUSD != nil {
			u.SetWeeklyLimitUsd(*plan.WeeklyLimitUSD)
		} else {
			u.ClearWeeklyLimitUsd()
		}
		if plan.MonthlyLimitUSD != nil {
			u.SetMonthlyLimitUsd(*plan.MonthlyLimitUSD)
		} else {
			u.ClearMonthlyLimitUsd()
		}
		if _, err := u.Save(ctx); err != nil {
			if dbent.IsNotFound(err) {
				return service.ErrPlanNotFound
			}
			return err
		}
		return replacePlanModels(ctx, tx, plan.ID, plan.Models)
	})
}

// Delete 硬删套餐。user_subscriptions.plan_id 是 RESTRICT 外键：仍有订阅行（含软删）引用就报 PLAN_IN_USE。
func (r *subscriptionPlanRepository) Delete(ctx context.Context, id int64) error {
	err := clientFromContext(ctx, r.client).SubscriptionPlan.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrPlanNotFound
		}
		if isForeignKeyViolation(err) {
			return service.ErrPlanInUse.WithCause(err)
		}
		return err
	}
	return nil
}

// replacePlanModels 用整份列表覆盖套餐模型集；条目外键冲突翻译成 PLAN_MODEL_NOT_FOUND。
func replacePlanModels(ctx context.Context, tx *dbent.Tx, planID int64, models []service.SubscriptionPlanModel) error {
	if _, err := tx.SubscriptionPlanModel.Delete().
		Where(subscriptionplanmodel.PlanIDEQ(planID)).Exec(ctx); err != nil {
		return err
	}
	for _, m := range models {
		if _, err := tx.SubscriptionPlanModel.Create().
			SetPlanID(planID).
			SetEntryID(m.EntryID).
			Save(ctx); err != nil {
			return translatePlanModelError(err)
		}
	}
	return nil
}

func translatePlanModelError(err error) error {
	if !isForeignKeyViolation(err) {
		return err
	}
	var pgErr *pq.Error
	if errors.As(err, &pgErr) && strings.Contains(pgErr.Constraint, "entry") {
		return service.ErrPlanModelNotFound.WithCause(err)
	}
	return service.ErrPlanNotFound.WithCause(err)
}

func (r *subscriptionPlanRepository) withTx(ctx context.Context, fn func(tx *dbent.Tx) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(tx)
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	return tx.Commit()
}

// subscriptionPlanEntityToService 唯一的 ent→领域转换；user_subscription_repo / redeem_code_repo 复用。
func subscriptionPlanEntityToService(m *dbent.SubscriptionPlan) *service.SubscriptionPlan {
	if m == nil {
		return nil
	}
	plan := &service.SubscriptionPlan{
		ID:              m.ID,
		Name:            m.Name,
		Description:     m.Description,
		Price:           m.Price,
		OriginalPrice:   m.OriginalPrice,
		ValidityDays:    m.ValidityDays,
		ValidityUnit:    m.ValidityUnit,
		Features:        m.Features,
		ProductName:     m.ProductName,
		ForSale:         m.ForSale,
		SortOrder:       m.SortOrder,
		DailyLimitUSD:   m.DailyLimitUsd,
		WeeklyLimitUSD:  m.WeeklyLimitUsd,
		MonthlyLimitUSD: m.MonthlyLimitUsd,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if len(m.Edges.Models) > 0 {
		plan.Models = make([]service.SubscriptionPlanModel, 0, len(m.Edges.Models))
		for _, pm := range m.Edges.Models {
			item := service.SubscriptionPlanModel{EntryID: pm.EntryID}
			if pm.Edges.Entry != nil {
				item.ModelID = pm.Edges.Entry.ModelID
				item.DisplayName = pm.Edges.Entry.DisplayName
			}
			plan.Models = append(plan.Models, item)
		}
	}
	return plan
}
