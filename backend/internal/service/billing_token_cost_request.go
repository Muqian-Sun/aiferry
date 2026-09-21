package service

import (
	"context"
	"time"
)

// TokenCostRequest 通用网关 token 计费请求。
type TokenCostRequest struct {
	Ctx             context.Context
	Model           string
	Tokens          UsageTokens
	RateMultiplier  float64
	PricingAt       time.Time
	ServiceTier     string
	ReasoningEffort string
	Resolver        *ModelPricingResolver
	// Resolved 为调用方预先解析的定价（Resolver.Resolve 的结果），nil 表示未解析。
	Resolved *ResolvedPricing
}

// CalculateTokenCostForRequest 按通用网关的路径选择计算 token 费用：
//  1. 有解析器（分组价卡 → 模型目录 → 价格文件，区间与长上下文阶梯均在其中）
//     或带推理等级 → 统一计费；
//  2. 否则直接按价格文件计费。
//
// 模型广场的阶梯表查询与网关使用同一入口，保证展示与扣费同源。
func (s *BillingService) CalculateTokenCostForRequest(req TokenCostRequest) (*CostBreakdown, error) {
	if req.Resolver != nil || req.Resolved != nil || req.ReasoningEffort != "" {
		return s.CalculateCostUnified(s.tokenCostInput(req, req.Resolved))
	}
	return s.CalculateCost(req.Model, req.Tokens, req.RateMultiplier)
}

func (s *BillingService) tokenCostInput(req TokenCostRequest, resolved *ResolvedPricing) CostInput {
	input := CostInput{
		Ctx:             req.Ctx,
		Model:           req.Model,
		Tokens:          req.Tokens,
		RequestCount:    1,
		RateMultiplier:  req.RateMultiplier,
		PricingAt:       req.PricingAt,
		ServiceTier:     req.ServiceTier,
		ReasoningEffort: req.ReasoningEffort,
		Resolver:        req.Resolver,
		Resolved:        resolved,
	}
	return input
}
