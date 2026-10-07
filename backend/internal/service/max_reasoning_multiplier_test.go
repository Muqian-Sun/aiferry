//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 最高推理倍率分官方 / 售价 / 成本三套（muqian 2026-10-07：与忙闲时一样，售价与成本默认跟官方，可单独设）：
// 向用户收钱按售价的（没定跟官方），渠道成本按承接上的（没填跟官方，不跟售价）。

func TestSaleMaxReasoningMultiplier_FollowsOfficialUnlessSet(t *testing.T) {
	entry := upstreamCostTestEntry()
	require.Nil(t, entry.SaleMaxReasoningMultiplier(), "官方没设、售价没设：不加价")

	entry.MaxReasoningEffortMultiplier = upstreamCostPtr(3)
	require.Equal(t, 3.0, *entry.SaleMaxReasoningMultiplier(), "没单独定：跟官方")

	entry.SalePrices.MaxReasoningEffortMultiplier = upstreamCostPtr(1)
	require.Nil(t, entry.SaleMaxReasoningMultiplier(), "售价定成 1 = 不加价")
	require.False(t, entry.SalePrices.IsZero(), "只定了最高推理倍率也算定了售价")

	entry.SalePrices.MaxReasoningEffortMultiplier = upstreamCostPtr(2)
	require.Equal(t, 2.0, *entry.SaleMaxReasoningMultiplier())
}

// 向用户收钱：官方 × 3；售价没定跟官方 × 3，定成 1 不加、定成 2 按 2；不是 max 档一律不加。
func TestBilling_UsesSaleMaxReasoningMultiplier(t *testing.T) {
	bs := NewBillingService()
	tokens := UsageTokens{InputTokens: 100_000} // 不到 272K 分段
	const base = 0.5                            // 输入 $5 / 百万
	for name, tc := range map[string]struct {
		sale   *float64
		effort string
		want   float64
	}{
		"follow official": {nil, "max", base * 3},
		"sale flat":       {upstreamCostPtr(1), "max", base},
		"sale custom":     {upstreamCostPtr(2), "max", base * 2},
		"not max":         {upstreamCostPtr(2), "high", base},
	} {
		t.Run(name, func(t *testing.T) {
			entry := upstreamCostTestEntry()
			entry.MaxReasoningEffortMultiplier = upstreamCostPtr(3)
			entry.SalePrices.MaxReasoningEffortMultiplier = tc.sale
			resolver := newResolverWithSeededEntries(bs, entry)
			cost, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
				Ctx: context.Background(), Model: entry.ModelID, Tokens: tokens, RateMultiplier: 1,
				ReasoningEffort: tc.effort, Resolver: resolver,
			})
			require.NoError(t, err)
			require.InDelta(t, tc.want, cost.TotalCost, 1e-9)
		})
	}
}

// 渠道成本：承接没填跟官方（不跟售价）；填 1 不加、填 2 按 2。
func TestCalculateUpstreamCost_BindingMaxReasoningMultiplier(t *testing.T) {
	bs := NewBillingService()
	tokens := UsageTokens{InputTokens: 100_000} // 不到 272K 分段
	const base = 0.015                          // 上游输入 $0.15 / 百万
	for name, tc := range map[string]struct {
		upstream *float64
		effort   string
		want     float64
	}{
		"follow official":  {nil, "max", base * 3},
		"upstream flat":    {upstreamCostPtr(1), "max", base},
		"upstream custom":  {upstreamCostPtr(2), "max", base * 2},
		"not max":          {upstreamCostPtr(2), "xhigh", base},
		"no effort at all": {nil, "", base},
	} {
		t.Run(name, func(t *testing.T) {
			entry := upstreamCostTestEntry()
			entry.MaxReasoningEffortMultiplier = upstreamCostPtr(3)
			entry.SalePrices.MaxReasoningEffortMultiplier = upstreamCostPtr(1.5) // 售价单独定的不影响成本
			binding := upstreamCostTestBinding(7)
			binding.MaxReasoningEffortMultiplier = tc.upstream
			resolver := newUpstreamCostTestResolver(t, bs, entry, binding)
			found, b := resolver.LookupUpstreamPrice(context.Background(), entry.ModelID, 7)
			require.NotNil(t, b)
			cost, err := bs.CalculateUpstreamCost(context.Background(), resolver, found, b, tokens, upstreamOffPeakAt, tc.effort)
			require.NoError(t, err)
			require.InDelta(t, tc.want, cost, 1e-9)
		})
	}
}

func TestMaxReasoningMultiplier_MustBePositive(t *testing.T) {
	entry := upstreamCostTestEntry()
	b := upstreamCostTestBinding(7)
	b.MaxReasoningEffortMultiplier = upstreamCostPtr(0)
	require.ErrorContains(t, b.ValidateAgainst(&entry), "upstream max_reasoning_effort_multiplier must be > 0")
	b.MaxReasoningEffortMultiplier = upstreamCostPtr(1)
	require.NoError(t, b.ValidateAgainst(&entry))

	entry.SalePrices.MaxReasoningEffortMultiplier = upstreamCostPtr(-1)
	require.ErrorContains(t, entry.Validate(), "sale max_reasoning_effort_multiplier must be > 0")
}

// 价格页按模型保存：官方的写条目（真改了才转为运营者定价），售价的写 sale_prices，上游的写承接。
func TestModelCatalogService_SaveEntryPricingStoresMaxReasoningMultipliers(t *testing.T) {
	ctx := context.Background()
	seed := upstreamCostTestEntry()
	seed.ManagedBy = ModelCatalogManagedBySeed
	seed.MaxReasoningEffortMultiplier = upstreamCostPtr(3)
	svc, repo := newTestModelCatalogService(seed)
	official := OfficialPrices{InputPrice: seed.InputPrice, OutputPrice: seed.OutputPrice, CacheReadPrice: seed.CacheReadPrice,
		Intervals: seed.Intervals, MaxReasoningEffortMultiplier: upstreamCostPtr(3)}

	binding := upstreamCostTestBinding(1)
	binding.MaxReasoningEffortMultiplier = upstreamCostPtr(2)
	_, err := svc.SaveEntryPricing(ctx, 1, official, CatalogSalePrices{MaxReasoningEffortMultiplier: upstreamCostPtr(1)},
		[]ModelCatalogBinding{binding, upstreamCostTestBinding(3)}, newPricingTestAccounts())
	require.NoError(t, err)
	require.Equal(t, 3.0, *repo.entries[0].MaxReasoningEffortMultiplier)
	require.Equal(t, 1.0, *repo.entries[0].SalePrices.MaxReasoningEffortMultiplier)
	require.Equal(t, ModelCatalogManagedBySeed, repo.entries[0].ManagedBy, "官方没改：售价、上游倍率不改条目归属")
	stored := map[int64]ModelCatalogBinding{}
	for _, b := range repo.bindings[1] {
		stored[b.AccountID] = b
	}
	require.Equal(t, 2.0, *stored[1].MaxReasoningEffortMultiplier)
	require.Nil(t, stored[3].MaxReasoningEffortMultiplier, "没填 = 跟官方")

	official.MaxReasoningEffortMultiplier = nil
	_, err = svc.SaveEntryPricing(ctx, 1, official, CatalogSalePrices{}, nil, newPricingTestAccounts())
	require.NoError(t, err)
	require.Nil(t, repo.entries[0].MaxReasoningEffortMultiplier, "官方改成不加价")
	require.Equal(t, ModelCatalogManagedByAdmin, repo.entries[0].ManagedBy, "官方倍率改了 = 运营者定价，播种不再刷新")
}

// 模型广场给用户看的最高推理倍率 = 实际向用户收的（售价的）；售价定成 1 = 不加价，不显示。
func TestModelPlazaService_ShowsSaleMaxReasoningMultiplier(t *testing.T) {
	custom := upstreamCostTestEntry()
	custom.MaxReasoningEffortMultiplier = upstreamCostPtr(3)
	custom.SalePrices.MaxReasoningEffortMultiplier = upstreamCostPtr(2)
	flat := upstreamCostTestEntry()
	flat.ID, flat.ModelID = 2, "flat-model"
	flat.MaxReasoningEffortMultiplier = upstreamCostPtr(3)
	flat.SalePrices.MaxReasoningEffortMultiplier = upstreamCostPtr(1)
	catalog := NewModelCatalogService(&stubModelCatalogRepo{entries: []ModelCatalogEntry{custom, flat}}, nil, ModelCatalogSeedInput{})

	models := NewModelPlazaService(catalog, catalog).ListModels(context.Background())
	require.Len(t, models, 2)
	byID := map[string]PlazaCatalogModel{}
	for _, m := range models {
		byID[m.ModelID] = m
	}
	require.Equal(t, 2.0, *byID[custom.ModelID].Pricing.MaxReasoningEffortMultiplier)
	require.Nil(t, byID["flat-model"].Pricing.MaxReasoningEffortMultiplier, "不加价：广场不显示")
}
