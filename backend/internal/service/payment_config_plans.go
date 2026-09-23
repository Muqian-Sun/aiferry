package service

import (
	"context"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// validatePlanRequired checks that all required fields for a plan are provided.
func validatePlanRequired(name string, price float64, validityDays int, validityUnit string, originalPrice *float64, entryIDs []int64) error {
	if strings.TrimSpace(name) == "" {
		return infraerrors.BadRequest("PLAN_NAME_REQUIRED", "plan name is required")
	}
	if price <= 0 {
		return infraerrors.BadRequest("PLAN_PRICE_INVALID", "price must be > 0")
	}
	if validityDays <= 0 {
		return infraerrors.BadRequest("PLAN_VALIDITY_REQUIRED", "validity days must be > 0")
	}
	if strings.TrimSpace(validityUnit) == "" {
		return infraerrors.BadRequest("PLAN_VALIDITY_UNIT_REQUIRED", "validity unit is required")
	}
	if originalPrice != nil && *originalPrice < 0 {
		return infraerrors.BadRequest("PLAN_ORIGINAL_PRICE_INVALID", "original price must be >= 0")
	}
	if len(entryIDs) == 0 {
		return ErrPlanModelsRequired
	}
	return nil
}

// validatePlanPatch validates only the non-nil fields in a patch update.
func validatePlanPatch(req UpdatePlanRequest) error {
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return infraerrors.BadRequest("PLAN_NAME_REQUIRED", "plan name is required")
	}
	if req.Price != nil && *req.Price <= 0 {
		return infraerrors.BadRequest("PLAN_PRICE_INVALID", "price must be > 0")
	}
	if req.ValidityDays != nil && *req.ValidityDays <= 0 {
		return infraerrors.BadRequest("PLAN_VALIDITY_REQUIRED", "validity days must be > 0")
	}
	if req.ValidityUnit != nil && strings.TrimSpace(*req.ValidityUnit) == "" {
		return infraerrors.BadRequest("PLAN_VALIDITY_UNIT_REQUIRED", "validity unit is required")
	}
	if req.OriginalPrice != nil && *req.OriginalPrice < 0 {
		return infraerrors.BadRequest("PLAN_ORIGINAL_PRICE_INVALID", "original price must be >= 0")
	}
	// nil = 不改；空切片 = 想清空模型集，不允许
	if req.EntryIDs != nil && len(req.EntryIDs) == 0 {
		return ErrPlanModelsRequired
	}
	return nil
}

// normalizePlanLimit 限额输入：nil 或 <= 0 → 不限（nil）
func normalizePlanLimit(v *float64) *float64 {
	if v == nil || *v <= 0 {
		return nil
	}
	return v
}

// planModelsFromEntryIDs 去重保序，只填 EntryID；名字由仓储回读时补上
func planModelsFromEntryIDs(entryIDs []int64) []SubscriptionPlanModel {
	seen := make(map[int64]struct{}, len(entryIDs))
	models := make([]SubscriptionPlanModel, 0, len(entryIDs))
	for _, id := range entryIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, SubscriptionPlanModel{EntryID: id})
	}
	return models
}

// --- Plan CRUD ---

func (s *PaymentConfigService) ListPlans(ctx context.Context) ([]SubscriptionPlan, error) {
	return s.planRepo.List(ctx, false)
}

func (s *PaymentConfigService) ListPlansForSale(ctx context.Context) ([]SubscriptionPlan, error) {
	return s.planRepo.List(ctx, true)
}

func (s *PaymentConfigService) CreatePlan(ctx context.Context, req CreatePlanRequest) (*SubscriptionPlan, error) {
	if err := validatePlanRequired(req.Name, req.Price, req.ValidityDays, req.ValidityUnit, req.OriginalPrice, req.EntryIDs); err != nil {
		return nil, err
	}
	models := planModelsFromEntryIDs(req.EntryIDs)
	if len(models) == 0 {
		return nil, ErrPlanModelsRequired
	}
	plan := &SubscriptionPlan{
		Name:            req.Name,
		Description:     req.Description,
		Price:           req.Price,
		OriginalPrice:   req.OriginalPrice,
		ValidityDays:    req.ValidityDays,
		ValidityUnit:    req.ValidityUnit,
		Features:        req.Features,
		ProductName:     req.ProductName,
		ForSale:         req.ForSale,
		SortOrder:       req.SortOrder,
		DailyLimitUSD:   normalizePlanLimit(req.DailyLimitUSD),
		WeeklyLimitUSD:  normalizePlanLimit(req.WeeklyLimitUSD),
		MonthlyLimitUSD: normalizePlanLimit(req.MonthlyLimitUSD),
		Models:          models,
	}
	if err := s.planRepo.Create(ctx, plan); err != nil {
		return nil, err
	}
	// 回读拿模型名
	return s.planRepo.GetByID(ctx, plan.ID)
}

// UpdatePlan updates a subscription plan by ID (patch semantics).
// 限额：nil = 不改，<= 0 = 清成不限。EntryIDs：nil = 不改，非空 = 覆盖模型集。
func (s *PaymentConfigService) UpdatePlan(ctx context.Context, id int64, req UpdatePlanRequest) (*SubscriptionPlan, error) {
	if err := validatePlanPatch(req); err != nil {
		return nil, err
	}
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		plan.Name = *req.Name
	}
	if req.Description != nil {
		plan.Description = *req.Description
	}
	if req.Price != nil {
		plan.Price = *req.Price
	}
	if req.OriginalPrice != nil {
		plan.OriginalPrice = req.OriginalPrice
	}
	if req.ValidityDays != nil {
		plan.ValidityDays = *req.ValidityDays
	}
	if req.ValidityUnit != nil {
		plan.ValidityUnit = *req.ValidityUnit
	}
	if req.Features != nil {
		plan.Features = *req.Features
	}
	if req.ProductName != nil {
		plan.ProductName = *req.ProductName
	}
	if req.ForSale != nil {
		plan.ForSale = *req.ForSale
	}
	if req.SortOrder != nil {
		plan.SortOrder = *req.SortOrder
	}
	if req.DailyLimitUSD != nil {
		plan.DailyLimitUSD = normalizePlanLimit(req.DailyLimitUSD)
	}
	if req.WeeklyLimitUSD != nil {
		plan.WeeklyLimitUSD = normalizePlanLimit(req.WeeklyLimitUSD)
	}
	if req.MonthlyLimitUSD != nil {
		plan.MonthlyLimitUSD = normalizePlanLimit(req.MonthlyLimitUSD)
	}
	if req.EntryIDs != nil {
		models := planModelsFromEntryIDs(req.EntryIDs)
		if len(models) == 0 {
			return nil, ErrPlanModelsRequired
		}
		plan.Models = models
	}
	if err := s.planRepo.Update(ctx, plan); err != nil {
		return nil, err
	}
	if err := s.invalidatePlanCache(ctx, id); err != nil {
		return nil, err
	}
	return s.planRepo.GetByID(ctx, id)
}

func (s *PaymentConfigService) DeletePlan(ctx context.Context, id int64) error {
	count, err := s.countPendingOrdersByPlan(ctx, id)
	if err != nil {
		return fmt.Errorf("check pending orders: %w", err)
	}
	if count > 0 {
		return infraerrors.Conflict("PENDING_ORDERS",
			fmt.Sprintf("this plan has %d in-progress orders and cannot be deleted — wait for orders to complete first", count))
	}
	if err := s.planRepo.Delete(ctx, id); err != nil {
		return err
	}
	return s.invalidatePlanCache(ctx, id)
}

// GetPlan returns a subscription plan by ID（带模型集）.
func (s *PaymentConfigService) GetPlan(ctx context.Context, id int64) (*SubscriptionPlan, error) {
	return s.planRepo.GetByID(ctx, id)
}

func (s *PaymentConfigService) invalidatePlanCache(ctx context.Context, id int64) error {
	return s.planCache.InvalidatePlanCache(ctx, id)
}
