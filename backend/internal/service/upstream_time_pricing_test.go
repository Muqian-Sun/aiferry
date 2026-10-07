//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 上游忙闲时（muqian 2026-10-06）：算成本按承接上填的上游价，上游有没有忙闲时在承接上定；忙时整单乘倍数；
// 利润门按请求当时的价判断。

// 北京时间工作日 10:00（DeepSeek 官方高峰）与周六 10:00（全天闲时）。
var (
	upstreamPeakAt    = time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC)
	upstreamOffPeakAt = time.Date(2026, 9, 19, 2, 0, 0, 0, time.UTC)
)

// beijingDaytimeDouble 北京时间工作日 09:00–18:00 × 2（比 DeepSeek 官方高峰宽：12–14 点也算忙时）。
func beijingDaytimeDouble() *TimePricing {
	return &TimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true,
		Periods: []TimePricingPeriod{{StartTime: "09:00", EndTime: "18:00", Multiplier: 2}}}
}

// testDeepSeekPeak DeepSeek 官方忙闲时（与价格文件里 DeepSeek 条目的 time_pricing 相同）：北京时间工作日 09–12、14–18 × 2。
func testDeepSeekPeak() *TimePricing {
	return &TimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true, Periods: []TimePricingPeriod{
		{StartTime: "09:00", EndTime: "12:00", Multiplier: 2},
		{StartTime: "14:00", EndTime: "18:00", Multiplier: 2},
	}}
}

// deepseekTestEntry 目录里的 DeepSeek 条目：低谷价 0.15 / 0.60，带官方忙闲时（播种自价格文件）。
func deepseekTestEntry() ModelCatalogEntry {
	return ModelCatalogEntry{
		ID: 1, ModelID: "deepseek-flash", BillingMode: BillingModeToken, Status: ModelCatalogStatusListed,
		ManagedBy:  ModelCatalogManagedBySeed,
		InputPrice: upstreamCostPtr(0.15e-6), OutputPrice: upstreamCostPtr(0.6e-6),
		TimePricing: testDeepSeekPeak(),
	}
}

func deepseekTestBinding(tp *TimePricing) ModelCatalogBinding {
	return ModelCatalogBinding{EntryID: 1, AccountID: 7, InputPrice: 0.15e-6 * 0.03, OutputPrice: 0.6e-6 * 0.03, TimePricing: tp}
}

// 渠道成本：承接上的忙闲时按请求时刻整单乘倍数；条目自己的 DeepSeek 高峰不叠到上游价上。
func TestCalculateUpstreamCost_BindingTimePricing(t *testing.T) {
	bs := NewBillingService()
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 200, CacheReadTokens: 3000}
	const base = 0.000375 // 1000 × 0.15 + 200 × 0.9 + 3000 × 0.015（$ / 百万 Token）

	entry := upstreamCostTestEntry()
	binding := upstreamCostTestBinding(7)
	binding.TimePricing = beijingDaytimeDouble()
	resolver := newUpstreamCostTestResolver(t, bs, entry, binding)
	found, b := resolver.LookupUpstreamPrice(context.Background(), entry.ModelID, 7)
	require.NotNil(t, b)
	require.NotNil(t, b.TimePricing, "仓储读回的承接带着忙闲时")

	for name, tc := range map[string]struct {
		at   time.Time
		want float64
	}{
		"peak":        {upstreamPeakAt, base * 2},
		"weekend":     {upstreamOffPeakAt, base},
		"after hours": {time.Date(2026, 9, 16, 11, 0, 0, 0, time.UTC), base}, // 北京 19:00
	} {
		t.Run(name, func(t *testing.T) {
			cost, err := bs.CalculateUpstreamCost(context.Background(), resolver, found, b, tokens, tc.at, "")
			require.NoError(t, err)
			require.InDelta(t, tc.want, cost, 1e-12)
		})
	}

	t.Run("deepseek binding without upstream peak", func(t *testing.T) {
		ds := deepseekTestEntry()
		dsBinding := deepseekTestBinding(nil)
		dsResolver := newUpstreamCostTestResolver(t, bs, ds, dsBinding)
		dsFound, dsB := dsResolver.LookupUpstreamPrice(context.Background(), ds.ModelID, 7)
		cost, err := bs.CalculateUpstreamCost(context.Background(), dsResolver, dsFound, dsB, UsageTokens{InputTokens: 1_000_000}, upstreamPeakAt, "")
		require.NoError(t, err)
		require.InDelta(t, 0.15e-6*0.03*1_000_000, cost, 1e-12, "上游不分忙闲时：我们高峰加价不影响成本")
	})
}

func TestBindingCostRatioAt(t *testing.T) {
	t.Run("upstream peak raises the ratio only at peak", func(t *testing.T) {
		entry := upstreamCostTestEntry()
		b := upstreamCostTestBinding(7)
		b.TimePricing = beijingDaytimeDouble()
		for at, want := range map[time.Time]float64{time.Time{}: 0.03, upstreamOffPeakAt: 0.03, upstreamPeakAt: 0.06} {
			got, ok := bindingCostRatioAt(&entry, &b, at)
			require.True(t, ok)
			require.InDelta(t, want, got, 1e-12, at.String())
		}
	})

	t.Run("deepseek: both sides double at peak", func(t *testing.T) {
		entry := deepseekTestEntry()
		b := deepseekTestBinding(testDeepSeekPeak())
		got, ok := bindingCostRatioAt(&entry, &b, upstreamPeakAt)
		require.True(t, ok)
		require.InDelta(t, 0.03, got, 1e-12)

		flat := deepseekTestBinding(nil)
		got, _ = bindingCostRatioAt(&entry, &flat, upstreamPeakAt)
		require.InDelta(t, 0.015, got, 1e-12, "上游不涨、我们涨：忙时成本比减半")
	})

	t.Run("entry without time pricing does not double", func(t *testing.T) {
		entry := deepseekTestEntry()
		entry.TimePricing = nil
		b := deepseekTestBinding(testDeepSeekPeak())
		got, _ := bindingCostRatioAt(&entry, &b, upstreamPeakAt)
		require.InDelta(t, 0.06, got, 1e-12, "与计费同口径：只看目录条目的忙闲时，不按模型名套高峰")
	})

	t.Run("entry time pricing raises our side", func(t *testing.T) {
		entry := upstreamCostTestEntry()
		entry.TimePricing = &TimePricing{Timezone: "Asia/Shanghai", Periods: []TimePricingPeriod{{StartTime: "00:00", EndTime: "12:00", Multiplier: 1.5}}}
		b := upstreamCostTestBinding(7)
		b.TimePricing = beijingDaytimeDouble()
		got, _ := bindingCostRatioAt(&entry, &b, upstreamPeakAt) // 北京 10:00：上游 × 2、我们 × 1.5
		require.InDelta(t, 0.04, got, 1e-12)
	})
}

// 价格页的忙时毛利：一周里最差的时段；不比平时差时不显示。
func TestBindingPeakCostRatio(t *testing.T) {
	preset := testDeepSeekPeak()
	cases := []struct {
		name    string
		entry   ModelCatalogEntry
		binding ModelCatalogBinding
		want    float64 // 0 = 不比平时差
	}{
		{name: "no time pricing anywhere", entry: upstreamCostTestEntry(), binding: upstreamCostTestBinding(7)},
		{name: "upstream peak", entry: upstreamCostTestEntry(),
			binding: func() ModelCatalogBinding {
				b := upstreamCostTestBinding(7)
				b.TimePricing = beijingDaytimeDouble()
				return b
			}(),
			want: 0.06},
		{name: "deepseek preset matches our peak", entry: deepseekTestEntry(), binding: deepseekTestBinding(preset)},
		{name: "deepseek upstream flat", entry: deepseekTestEntry(), binding: deepseekTestBinding(nil)},
		// 上游北京 09–18 点都 × 2，我们只在 09–12、14–18 点翻倍：12–14 点最差
		{name: "deepseek upstream wider than ours", entry: deepseekTestEntry(), binding: deepseekTestBinding(beijingDaytimeDouble()), want: 0.06},
		// 上游按 UTC 00:00–01:00 × 3 = 北京 08:00–09:00，我们那时还是闲时：时区不同也能找到
		{name: "deepseek upstream in another timezone", entry: deepseekTestEntry(),
			binding: deepseekTestBinding(&TimePricing{Timezone: "UTC", Periods: []TimePricingPeriod{{StartTime: "00:00", EndTime: "01:00", Multiplier: 3}}}),
			want:    0.09},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := bindingPeakCostRatio(&tc.entry, &tc.binding)
			if tc.want == 0 {
				require.False(t, ok, "got %v", got)
				return
			}
			require.True(t, ok)
			require.InDelta(t, tc.want, got, 1e-12)
		})
	}
}

// 利润门按请求当时的价：上游忙时涨价，只在忙时跳过这条承接。
func TestProfitGate_RejectsOnlyAtUpstreamPeak(t *testing.T) {
	entry := upstreamCostTestEntry()
	// 售价都定成官方价的 1/10：上游占售价的 30%，毛利 70%
	entry.SalePrices = CatalogSalePrices{
		InputPrice: upstreamCostPtr(0.5e-6), OutputPrice: upstreamCostPtr(3e-6), CacheReadPrice: upstreamCostPtr(0.05e-6),
		Segments: []CatalogSaleSegment{{MinTokens: 272000, InputPrice: upstreamCostPtr(1e-6), OutputPrice: upstreamCostPtr(4.5e-6)}},
	}
	b := upstreamCostTestBinding(7)
	b.TimePricing = beijingDaytimeDouble()
	threshold := clampProfitControlThreshold(DefaultSalePriceRatio * (1 - 0.6))

	for at, want := range map[time.Time]bool{time.Time{}: false, upstreamOffPeakAt: false, upstreamPeakAt: true} {
		rejected, _ := profitGateRejectsBinding(&entry, &b, threshold, at, false)
		require.Equal(t, want, rejected, "at %s：忙时上游占售价 60%%，毛利 40%% 不到 60%%", at)
	}
}

func TestModelCatalogBinding_ValidateTimePricing(t *testing.T) {
	entry := upstreamCostTestEntry()
	b := upstreamCostTestBinding(7)
	b.TimePricing = beijingDaytimeDouble()
	require.NoError(t, b.ValidateAgainst(&entry))

	b.TimePricing = &TimePricing{Timezone: "Mars/Olympus", Periods: []TimePricingPeriod{{StartTime: "09:00", EndTime: "12:00", Multiplier: 2}}}
	require.ErrorContains(t, b.ValidateAgainst(&entry), "upstream time_pricing: timezone")

	b.TimePricing = &TimePricing{Timezone: "Asia/Shanghai", Periods: []TimePricingPeriod{{StartTime: "12:00", EndTime: "09:00", Multiplier: 2}}}
	require.ErrorContains(t, b.ValidateAgainst(&entry), "upstream time_pricing")
}

// 保存时没有时段的忙闲时存成 nil（= 上游不分忙闲时），有时段的原样存下。
func TestModelCatalogService_SaveEntryPricingStoresBindingTimePricing(t *testing.T) {
	ctx := context.Background()
	seed := upstreamCostTestEntry()
	svc, repo := newTestModelCatalogService(seed)
	official := OfficialPrices{InputPrice: seed.InputPrice, OutputPrice: seed.OutputPrice, CacheReadPrice: seed.CacheReadPrice, Intervals: seed.Intervals}

	withPeak := upstreamCostTestBinding(1)
	withPeak.TimePricing = &TimePricing{Timezone: " Asia/Shanghai ", WeekdaysOnly: true,
		Periods: []TimePricingPeriod{{StartTime: "09:00", EndTime: "18:00", Multiplier: 2}}}
	empty := upstreamCostTestBinding(3)
	empty.TimePricing = &TimePricing{Timezone: "Asia/Shanghai"}

	_, err := svc.SaveEntryPricing(ctx, 1, official, CatalogSalePrices{}, []ModelCatalogBinding{withPeak, empty}, newPricingTestAccounts())
	require.NoError(t, err)
	stored := map[int64]ModelCatalogBinding{}
	for _, b := range repo.bindings[1] {
		stored[b.AccountID] = b
	}
	require.NotNil(t, stored[1].TimePricing)
	require.Equal(t, "Asia/Shanghai", stored[1].TimePricing.Timezone)
	require.Len(t, stored[1].TimePricing.Periods, 1)
	require.Nil(t, stored[3].TimePricing, "没有时段 = 不分忙闲时")
}
