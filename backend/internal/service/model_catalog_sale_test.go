//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 售价每项单独填（muqian 2026-10-06）：填了的项按售价收、没填的按官方价 × 1/15；用户倍率 = 在售价上再打折。

// saleTestEntry 官方价：输入 5、输出 30、缓存读 0.5（$ / 百万 Token）；>272K 一段输入 10、输出 45。
// 售价只填了基础输入 0.5（官方 × 1/10）；输出、缓存读、分段都没单独填。
func saleTestEntry() ModelCatalogEntry {
	entry := upstreamCostTestEntry()
	entry.SalePrices = CatalogSalePrices{InputPrice: upstreamCostPtr(0.5e-6)}
	return entry
}

func saleCost(t *testing.T, entry ModelCatalogEntry, user *User, tokens UsageTokens, at time.Time) *CostBreakdown {
	t.Helper()
	bs := NewBillingService()
	got, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx: context.Background(), Model: entry.ModelID, Tokens: tokens,
		RateMultiplier: UserRateMultiplier(user), PricingAt: at,
		Resolver: newResolverWithSeededEntries(bs, entry),
	})
	require.NoError(t, err)
	return got
}

func TestSalePrices_BillingChargesSaleTimesDiscount(t *testing.T) {
	entry := saleTestEntry()
	tokens := UsageTokens{InputTokens: 100_000, OutputTokens: 10_000, CacheReadTokens: 100_000} // 输入侧 20 万，落在基础价

	cost := saleCost(t, entry, &User{}, tokens, time.Time{})
	// 输入按售价 0.5；输出、缓存读没单独定：官方价 × 1/15
	want := 100_000*0.5e-6 + 10_000*30e-6/15 + 100_000*0.5e-6/15
	require.InDelta(t, want, cost.ActualCost, 1e-12)
	require.InDelta(t, 100_000*5e-6+10_000*30e-6+100_000*0.5e-6, cost.TotalCost, 1e-12, "官方价合计照旧")

	discounted := saleCost(t, entry, &User{RateMultiplier: customRate(0.5)}, tokens, time.Time{})
	require.InDelta(t, want*0.5, discounted.ActualCost, 1e-12, "用户折扣乘在售价上")

	// 一项售价都没填：与以前（官方价 × 1/15）一模一样
	plain := upstreamCostTestEntry()
	require.InDelta(t, cost.TotalCost/15, saleCost(t, plain, &User{}, tokens, time.Time{}).ActualCost, 1e-12)
}

func TestSalePrices_SegmentsFollowOfficialRatioOrOwnPrice(t *testing.T) {
	tokens := UsageTokens{InputTokens: 300_000, OutputTokens: 1000}

	// 落在 >272K：段内没单独定，按「基础售价 × 本段官方价 ÷ 基础官方价」= 0.5 × 10 / 5 = 1
	entry := saleTestEntry()
	cost := saleCost(t, entry, &User{}, tokens, time.Time{})
	require.InDelta(t, 300_000*1e-6+1000*45e-6/15, cost.ActualCost, 1e-12)

	// 段内单独定了输入 0.8：按它
	entry.SalePrices.Segments = []CatalogSaleSegment{{MinTokens: 272000, InputPrice: upstreamCostPtr(0.8e-6)}}
	cost = saleCost(t, entry, &User{}, tokens, time.Time{})
	require.InDelta(t, 300_000*0.8e-6+1000*45e-6/15, cost.ActualCost, 1e-12)

	// 没落进分段：段的售价不影响基础价
	low := saleCost(t, entry, &User{}, UsageTokens{InputTokens: 1000}, time.Time{})
	require.InDelta(t, 1000*0.5e-6, low.ActualCost, 1e-15)
}

// DeepSeek 目录存闲时价、条目带官方忙闲时（高峰 × 2）；售价也按闲时价填，高峰同样加倍，官方价合计一起加倍。
func TestSalePrices_EntryPeakDoublesSalePrice(t *testing.T) {
	entry := deepseekTestEntry()
	entry.SalePrices = CatalogSalePrices{InputPrice: upstreamCostPtr(0.02e-6)}
	peak := time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC)    // 周三北京 10:00，官方高峰
	offPeak := time.Date(2026, 9, 19, 2, 0, 0, 0, time.UTC) // 周六，全天闲时

	tokens := UsageTokens{InputTokens: 1_000_000}
	off := saleCost(t, entry, &User{}, tokens, offPeak)
	require.InDelta(t, 0.02, off.ActualCost, 1e-12)
	require.InDelta(t, 0.15, off.TotalCost, 1e-12)
	on := saleCost(t, entry, &User{}, tokens, peak)
	require.InDelta(t, 0.04, on.ActualCost, 1e-12)
	require.InDelta(t, 0.30, on.TotalCost, 1e-12)

	// 条目没有忙闲时就不加价：不按模型名套高峰
	entry.TimePricing = nil
	require.InDelta(t, 0.02, saleCost(t, entry, &User{}, tokens, peak).ActualCost, 1e-12)
}

// 利润门与价格页毛利和售价比：售价定高了，同一条上游价就能过门。
func TestSalePrices_ProfitGateComparesUpstreamWithSalePrice(t *testing.T) {
	entry := upstreamCostTestEntry()
	b := upstreamCostTestBinding(7) // 上游价 = 官方价的 3%
	threshold := clampProfitControlThreshold(DefaultSalePriceRatio * (1 - 0.6))

	ratio, ok := bindingCostRatio(&entry, &b)
	require.True(t, ok)
	require.InDelta(t, 0.03, ratio, 1e-12, "没定售价：比官方价")
	rejected, _ := profitGateRejectsBinding(&entry, &b, threshold, time.Time{}, false)
	require.True(t, rejected, "售价 = 官方 / 15，上游占 45%，毛利 55% 不到 60%")

	// 输入、输出、缓存读、>272K 一段都把售价定成官方价的 1/10：上游占售价的 30%，毛利 70%
	entry.SalePrices = CatalogSalePrices{
		InputPrice: upstreamCostPtr(0.5e-6), OutputPrice: upstreamCostPtr(3e-6), CacheReadPrice: upstreamCostPtr(0.05e-6),
		Segments: []CatalogSaleSegment{{MinTokens: 272000, InputPrice: upstreamCostPtr(1e-6), OutputPrice: upstreamCostPtr(4.5e-6)}},
	}
	ratio, ok = bindingCostRatio(&entry, &b)
	require.True(t, ok)
	require.InDelta(t, 0.3*DefaultSalePriceRatio, ratio, 1e-12)
	rejected, _ = profitGateRejectsBinding(&entry, &b, threshold, time.Time{}, false)
	require.False(t, rejected)

	// 有一项售价定低了（>272K 输出 2.25 = 官方 / 20）：取最差的一项，又过不了门
	entry.SalePrices.Segments[0].OutputPrice = upstreamCostPtr(2.25e-6)
	rejected, _ = profitGateRejectsBinding(&entry, &b, threshold, time.Time{}, false)
	require.True(t, rejected)
}

// 模型广场展示售价口径：定了售价的项 × 计费倍率 = 售价 × 折扣；分段换成绝对价。
func TestSalePrices_PlazaCardUsesSaleBasis(t *testing.T) {
	entry := saleTestEntry()
	card := entry.SaleEquivalentPricingCard()
	require.InDelta(t, 0.5e-6, *card.InputPrice*DefaultSalePriceRatio, 1e-18, "输入展示 = 售价")
	require.InDelta(t, 30e-6, *card.OutputPrice, 1e-18, "输出没单独定：官方价（展示时 × 1/15）")
	require.Len(t, card.Intervals, 1)
	require.InDelta(t, 1e-6, *card.Intervals[0].InputPrice*DefaultSalePriceRatio, 1e-18, "段内按官方同比例推")

	plain := upstreamCostTestEntry()
	require.Equal(t, plain.PricingCard(), plain.SaleEquivalentPricingCard(), "没定售价：与官方价卡相同")
}

func TestSalePrices_Validation(t *testing.T) {
	valid := func() *ModelCatalogEntry {
		e := saleTestEntry()
		e.Normalize()
		return &e
	}
	require.NoError(t, valid().Validate())

	e := valid()
	e.SalePrices.OutputPrice = upstreamCostPtr(-1)
	require.ErrorContains(t, e.Validate(), "sale output_price must be >= 0")

	e = valid()
	e.SalePrices.Segments = []CatalogSaleSegment{{MinTokens: 128000, InputPrice: upstreamCostPtr(1e-6)}}
	require.ErrorContains(t, e.Validate(), "no matching official segment")

	e = valid()
	e.SalePrices.Segments = []CatalogSaleSegment{{MinTokens: 272000, InputPrice: upstreamCostPtr(1e-6)}, {MinTokens: 272000, OutputPrice: upstreamCostPtr(1e-6)}}
	require.ErrorContains(t, e.Validate(), "set twice")

	e = valid()
	e.BillingMode = BillingModeImage
	e.PerRequestPrice = upstreamCostPtr(0.05)
	e.Intervals = nil
	require.ErrorContains(t, e.Validate(), "only for token-billed models")
}

// 价格页只改售价：条目归属不变（官方价照旧跟着价格文件刷新）；空段丢掉、各段按下界排好。
func TestModelCatalogService_SaveEntryPricingWritesSalePrices(t *testing.T) {
	ctx := context.Background()
	seed := upstreamCostTestEntry()
	seed.ManagedBy = ModelCatalogManagedBySeed
	svc, _ := newTestModelCatalogService(seed)
	official := OfficialPrices{
		InputPrice: seed.InputPrice, OutputPrice: seed.OutputPrice, CacheReadPrice: seed.CacheReadPrice,
		Intervals: seed.Intervals,
	}
	sale := CatalogSalePrices{
		InputPrice: upstreamCostPtr(0.5e-6),
		Segments: []CatalogSaleSegment{
			{MinTokens: 272000, OutputPrice: upstreamCostPtr(4.5e-6)},
			{MinTokens: 272000},
		},
	}
	got, err := svc.SaveEntryPricing(ctx, 1, official, sale, nil, newPricingTestAccounts())
	require.NoError(t, err)
	require.Equal(t, ModelCatalogManagedBySeed, got.ManagedBy, "只改售价不改归属")
	require.InDelta(t, 0.5e-6, *got.SalePrices.InputPrice, 1e-18)
	require.Len(t, got.SalePrices.Segments, 1, "空段丢掉")

	stored := svc.LookupPricingEntry(ctx, seed.ModelID)
	require.NotNil(t, stored)
	require.InDelta(t, 4.5e-6, *stored.SalePrices.Segments[0].OutputPrice, 1e-18)
}
