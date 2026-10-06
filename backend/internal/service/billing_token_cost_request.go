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
	ReasoningEffort string
	Resolver        *ModelPricingResolver
	// Resolved 为调用方预先解析的定价（Resolver.Resolve 的结果），nil 表示未解析。
	Resolved *ResolvedPricing
}

// CalculateTokenCostForRequest 计算 token 费用：只按模型目录（muqian 2026-10-06：所有模型的计费都从模型目录出发），
// 没有解析器 / 预解析结果就没有价。模型广场的阶梯表查询与网关使用同一入口，保证展示与扣费同源。
func (s *BillingService) CalculateTokenCostForRequest(req TokenCostRequest) (*CostBreakdown, error) {
	return s.CalculateCostUnified(s.tokenCostInput(req, req.Resolved))
}

func (s *BillingService) tokenCostInput(req TokenCostRequest, resolved *ResolvedPricing) CostInput {
	input := CostInput{
		Ctx:             req.Ctx,
		Model:           req.Model,
		Tokens:          req.Tokens,
		RequestCount:    1,
		RateMultiplier:  req.RateMultiplier,
		PricingAt:       req.PricingAt,
		ReasoningEffort: req.ReasoningEffort,
		Resolver:        req.Resolver,
		Resolved:        resolved,
	}
	return input
}
