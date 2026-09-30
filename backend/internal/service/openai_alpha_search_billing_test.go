//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCalculateWebSearchCostDefaultAndOverride(t *testing.T) {
	t.Parallel()
	s := &BillingService{}

	// 内置单价：官方 $10/1000 次 = 0.01/次
	cost := s.CalculateWebSearchCost(1, nil, 1.0)
	require.InDelta(t, 0.01, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.01, cost.ActualCost, 1e-12)
	require.Equal(t, string(BillingModePerRequest), cost.BillingMode)

	// 条目 search_price_per_call 覆盖 + 倍率
	cost = s.CalculateWebSearchCost(1, float64Ptr(0.02), 2.5)
	require.InDelta(t, 0.02, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.05, cost.ActualCost, 1e-12)

	// 0 = 免费（区别于 nil = 内置单价）
	cost = s.CalculateWebSearchCost(1, float64Ptr(0), 3.0)
	require.Zero(t, cost.TotalCost)
	require.Zero(t, cost.ActualCost)

	// 负数倍率按 0 处理，避免按 1x 误扣
	cost = s.CalculateWebSearchCost(1, nil, -1)
	require.InDelta(t, 0.01, cost.TotalCost, 1e-12)
	require.Zero(t, cost.ActualCost)

	// 次数 <= 0 不产生费用
	cost = s.CalculateWebSearchCost(0, float64Ptr(0.02), 1.0)
	require.Zero(t, cost.TotalCost)
	require.Empty(t, cost.BillingMode)
}

func TestCalculateOpenAIRecordUsageCostWebSearchPerCall(t *testing.T) {
	t.Parallel()
	bs := NewBillingService(&config.Config{}, nil)
	svc := &OpenAIGatewayService{billingService: bs}

	// 目录没有该条目：alpha search 按内置单价 0.01 × 用户倍率。
	apiKey := &APIKey{ID: 1}
	result := &OpenAIForwardResult{Model: "gpt-5.6-sol", UpstreamModel: "gpt-5.6-sol", WebSearchCalls: 1}
	cost, err := svc.calculateOpenAIRecordUsageCost(context.Background(), result, apiKey, []string{"gpt-5.6-sol"}, 2.0, UsageTokens{}, time.Time{})
	require.NoError(t, err)
	require.Equal(t, string(BillingModePerRequest), cost.BillingMode)
	require.InDelta(t, 0.01, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.02, cost.ActualCost, 1e-12)

	// 目录条目配了 search_price_per_call 0.005 → 覆盖内置单价
	searchPrice := 0.005
	inputPrice := 1e-6
	svc.resolver = newResolverWithCatalogCards(bs, PricingCard{
		Models:             []string{"gpt-5.6-sol"},
		BillingMode:        BillingModeToken,
		InputPrice:         &inputPrice,
		SearchPricePerCall: &searchPrice,
	})
	cost, err = svc.calculateOpenAIRecordUsageCost(context.Background(), result, apiKey, []string{"gpt-5.6-sol"}, 1.0, UsageTokens{}, time.Time{})
	require.NoError(t, err)
	require.InDelta(t, 0.005, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.005, cost.ActualCost, 1e-12)

	// WebSearchCalls = 0 时不得走按次分支：回落到 token 路径按条目 token 价计。
	result.WebSearchCalls = 0
	cost, err = svc.calculateOpenAIRecordUsageCost(context.Background(), result, apiKey, []string{"gpt-5.6-sol"}, 1.0, UsageTokens{InputTokens: 10}, time.Time{})
	require.NoError(t, err)
	require.Equal(t, string(BillingModeToken), cost.BillingMode)
	require.InDelta(t, 10*inputPrice, cost.TotalCost, 1e-12)
}
