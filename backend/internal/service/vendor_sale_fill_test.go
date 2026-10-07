//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 按厂商填售价（muqian 2026-10-07：一次性批量填并保存）：这个厂商的全部按 token 计费模型，售价按「官方价 × 比例」重填，
// 每百万 Token 4 位小数；分段按计费的分段规则展开；售价忙闲时、最高推理倍率不动；别的厂商不动。
func TestFillVendorSalePricesByRatio(t *testing.T) {
	ctx := context.Background()
	tiered := upstreamCostTestEntry() // 官方 5 / 30，缓存读 0.5；>272K 一段输入 10、输出 45（缓存读没填，随输入同比例 = 1.0）
	tiered.Vendor = "dashscope"
	tiered.SalePrices = CatalogSalePrices{
		InputPrice:                   upstreamCostPtr(9e-6), // 原来单独填过的售价会被覆盖
		TimePricing:                  &TimePricingSpec{Periods: []TimePricingSpecPeriod{}},
		MaxReasoningEffortMultiplier: upstreamCostPtr(1),
	}
	flat := ModelCatalogEntry{ID: 2, ModelID: "qwen3.8-max", Vendor: "dashscope", BillingMode: BillingModeToken, Status: ModelCatalogStatusUnlisted, ManagedBy: ModelCatalogManagedBySeed,
		InputPrice: upstreamCostPtr(1.7647e-6), OutputPrice: upstreamCostPtr(5.2941e-6)}
	other := ModelCatalogEntry{ID: 3, ModelID: "glm-5.3", Vendor: "zhipu", BillingMode: BillingModeToken, Status: ModelCatalogStatusListed,
		InputPrice: upstreamCostPtr(1.1765e-6), OutputPrice: upstreamCostPtr(4.1176e-6)}
	svc, repo := newTestModelCatalogService(tiered, flat, other)

	updated, err := svc.FillVendorSalePricesByRatio(ctx, " DashScope ", 1.0/15)
	require.NoError(t, err)
	require.Equal(t, 2, updated)

	byID := map[int64]ModelCatalogEntry{}
	for _, e := range repo.entries {
		byID[e.ID] = e
	}
	sale := byID[1].SalePrices
	require.InDelta(t, 0.3333e-6, *sale.InputPrice, 1e-18, "5 / 15，4 位小数")
	require.InDelta(t, 2e-6, *sale.OutputPrice, 1e-18)
	require.InDelta(t, 0.0333e-6, *sale.CacheReadPrice, 1e-18)
	require.Nil(t, sale.CacheWritePrice, "官方价没有的项不填")
	require.Len(t, sale.Segments, 1)
	require.Equal(t, 272000, sale.Segments[0].MinTokens)
	require.InDelta(t, 0.6667e-6, *sale.Segments[0].InputPrice, 1e-18)
	require.InDelta(t, 3e-6, *sale.Segments[0].OutputPrice, 1e-18)
	require.InDelta(t, 0.0667e-6, *sale.Segments[0].CacheReadPrice, 1e-18, "段内没填的缓存读随输入同比例：1.0 / 15")
	require.NotNil(t, sale.TimePricing, "售价忙闲时不动")
	require.Equal(t, 1.0, *sale.MaxReasoningEffortMultiplier, "最高推理倍率不动")

	require.InDelta(t, 0.1176e-6, *byID[2].SalePrices.InputPrice, 1e-18, "1.7647 / 15")
	require.True(t, byID[3].SalePrices.IsZero(), "别的厂商不动")
	require.Equal(t, ModelCatalogManagedBySeed, byID[2].ManagedBy, "只改售价，不改归属")

	_, err = svc.FillVendorSalePricesByRatio(ctx, "dashscope", 0)
	require.ErrorContains(t, err, "ratio must be > 0")
	_, err = svc.FillVendorSalePricesByRatio(ctx, "volcengine", 0.1)
	require.ErrorContains(t, err, "has no token-billed models")
}
