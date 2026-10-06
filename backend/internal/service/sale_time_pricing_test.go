//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 售价忙闲时（muqian 2026-10-06：售价单独一套）：没单独定跟官方忙闲时；定成「全天一个价」就不加；
// 自定义的按自己的时段与倍数。计费、利润门、模型广场都按它。

func saleFlat() *TimePricingSpec { return &TimePricingSpec{Periods: []TimePricingSpecPeriod{}} }

func saleCustom(multiplier float64) *TimePricingSpec {
	return &TimePricingSpec{Timezone: "Asia/Shanghai", WeekdaysOnly: true,
		Periods: []TimePricingSpecPeriod{{StartTime: "09:00", EndTime: "18:00", Multiplier: multiplier}}}
}

func TestSaleTimePricing_FollowsOfficialUnlessSet(t *testing.T) {
	entry := deepseekTestEntry()
	require.Equal(t, entry.TimePricing, entry.SaleTimePricing(), "没单独定：跟官方忙闲时")

	entry.SalePrices.TimePricing = saleFlat()
	require.Nil(t, entry.SaleTimePricing(), "全天一个价")

	entry.SalePrices.TimePricing = saleCustom(1.5)
	got := entry.SaleTimePricing()
	require.NotNil(t, got)
	require.Equal(t, "Asia/Shanghai", got.Timezone)
	require.Equal(t, []TimePricingPeriod{{StartTime: "09:00", EndTime: "18:00", Multiplier: 1.5}}, got.Periods)
}

// 向用户收钱：DeepSeek 官方高峰 × 2；售价定成全天一个价就不翻倍，自定义 × 1.5 就按 1.5。
func TestBilling_UsesSaleTimePricing(t *testing.T) {
	bs := NewBillingService()
	tokens := UsageTokens{InputTokens: 1_000_000}
	const base = 0.15 // 低谷价 $0.15 / 百万
	for name, tc := range map[string]struct {
		sale *TimePricingSpec
		want float64
	}{
		"follow official": {nil, base * 2},
		"flat":            {saleFlat(), base},
		"custom":          {saleCustom(1.5), base * 1.5},
	} {
		t.Run(name, func(t *testing.T) {
			entry := deepseekTestEntry()
			entry.SalePrices.TimePricing = tc.sale
			resolver := newResolverWithSeededEntries(bs, entry)
			cost, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
				Ctx: context.Background(), Model: entry.ModelID, Tokens: tokens, RateMultiplier: 1,
				PricingAt: upstreamPeakAt, Resolver: resolver,
			})
			require.NoError(t, err)
			require.InDelta(t, tc.want, cost.TotalCost, 1e-12)
		})
	}
}

// 利润门与价格页忙时毛利：我们这边的倍数按售价忙闲时。上游跟官方一样忙时 × 2，售价全天一个价 → 忙时成本比翻倍。
func TestUpstreamCostRatio_UsesSaleTimePricing(t *testing.T) {
	entry := deepseekTestEntry()
	b := deepseekTestBinding(testDeepSeekPeak())

	got, ok := bindingCostRatioAt(&entry, &b, upstreamPeakAt)
	require.True(t, ok)
	require.InDelta(t, 0.03, got, 1e-12, "跟官方：两边一起翻倍")
	_, worse := bindingPeakCostRatio(&entry, &b)
	require.False(t, worse)

	entry.SalePrices.TimePricing = saleFlat()
	got, _ = bindingCostRatioAt(&entry, &b, upstreamPeakAt)
	require.InDelta(t, 0.06, got, 1e-12, "售价不翻倍、上游翻倍")
	peak, worse := bindingPeakCostRatio(&entry, &b)
	require.True(t, worse)
	require.InDelta(t, 0.06, peak, 1e-12)

	entry.SalePrices.TimePricing = saleCustom(1.5)
	got, _ = bindingCostRatioAt(&entry, &b, upstreamPeakAt)
	require.InDelta(t, 0.04, got, 1e-12, "上游 × 2、售价 × 1.5")
}

// 保存：全天一个价存成空时段；自定义的时区去空白；时段不对报 sale_prices.time_pricing。
func TestModelCatalogService_SaveEntryPricingStoresSaleTimePricing(t *testing.T) {
	ctx := context.Background()
	seed := deepseekTestEntry()
	svc, repo := newTestModelCatalogService(seed)
	official := OfficialPrices{InputPrice: seed.InputPrice, OutputPrice: seed.OutputPrice, TimePricing: seed.TimePricing}

	custom := saleCustom(1.5)
	custom.Timezone = " Asia/Shanghai "
	_, err := svc.SaveEntryPricing(ctx, 1, official, CatalogSalePrices{TimePricing: custom}, nil, newPricingTestAccounts())
	require.NoError(t, err)
	stored := repo.entries[0].SalePrices.TimePricing
	require.NotNil(t, stored)
	require.Equal(t, "Asia/Shanghai", stored.Timezone)
	require.Equal(t, ModelCatalogManagedBySeed, repo.entries[0].ManagedBy, "改售价忙闲时不改条目归属")

	_, err = svc.SaveEntryPricing(ctx, 1, official, CatalogSalePrices{TimePricing: &TimePricingSpec{Timezone: "Asia/Shanghai"}}, nil, newPricingTestAccounts())
	require.NoError(t, err)
	require.NotNil(t, repo.entries[0].SalePrices.TimePricing)
	require.Empty(t, repo.entries[0].SalePrices.TimePricing.Periods)
	require.Nil(t, repo.entries[0].SaleTimePricing(), "全天一个价")

	bad := saleCustom(2)
	bad.Periods[0].EndTime = "08:00"
	_, err = svc.SaveEntryPricing(ctx, 1, official, CatalogSalePrices{TimePricing: bad}, nil, newPricingTestAccounts())
	require.ErrorContains(t, err, "sale_prices.time_pricing")
}

// 模型广场给用户看的忙闲时 = 实际向用户收的（售价忙闲时）。
func TestModelPlazaService_ShowsSaleTimePricing(t *testing.T) {
	entry := deepseekTestEntry()
	entry.SalePrices.TimePricing = saleFlat()
	custom := deepseekTestEntry()
	custom.ID, custom.ModelID = 2, "deepseek-v4-pro"
	custom.SalePrices.TimePricing = saleCustom(1.5)
	catalog := NewModelCatalogService(&stubModelCatalogRepo{entries: []ModelCatalogEntry{entry, custom}}, nil, ModelCatalogSeedInput{})

	models := NewModelPlazaService(catalog, catalog).ListModels(context.Background())
	require.Len(t, models, 2)
	byID := map[string]PlazaCatalogModel{}
	for _, m := range models {
		byID[m.ModelID] = m
	}
	require.Nil(t, byID["deepseek-flash"].TimePricing, "售价全天一个价：广场不显示忙闲时")
	require.NotNil(t, byID["deepseek-v4-pro"].TimePricing)
	require.Equal(t, 1.5, byID["deepseek-v4-pro"].TimePricing.Periods[0].Multiplier)
}
