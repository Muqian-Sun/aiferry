//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 目录条目 + 别名的 fixture：请求名走别名，价格政策与基准价都要落到条目的 model_id 上。
func newCatalogWithAlias(modelID, managedBy, alias string, card ChannelModelPricing) *ModelCatalogService {
	entry := catalogEntryFromCard(modelID, managedBy, card)
	entry.ID = 1
	entry.Aliases = []ModelCatalogAlias{{ID: 1, EntryID: 1, Alias: alias, Source: ModelCatalogAliasSourceManual}}
	catalog, _ := newTestModelCatalogService(entry)
	return catalog
}

// 别名命中目录后，基准价必须按条目的 model_id 查价格文件：别名本身在价格文件里查不到
// （故意不带家族词，避免被价格表的家族模糊匹配兜住），条目又没显式配价时，此前会得到「无价」。
func TestResolve_AliasUsesEntryModelIDForBasePricing(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	catalog := newCatalogWithAlias("claude-sonnet-4", ModelCatalogManagedBySeed, "team/best", ChannelModelPricing{})
	r := NewModelPricingResolver(catalog, bs)

	resolved := r.Resolve(context.Background(), PricingInput{Model: "team/best"})

	require.Equal(t, PricingSourceCatalog, resolved.Source)
	require.Equal(t, "claude-sonnet-4", resolved.CanonicalModel)
	require.NotNil(t, resolved.BasePricing)
	require.InDelta(t, 3e-6, resolved.BasePricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 15e-6, resolved.BasePricing.OutputPricePerToken, 1e-12)
}

// 分组价卡是覆盖层：没显式配的项要沿用目录基准价，而不是跳过目录直接落到价格文件。
func TestResolve_GroupCardStacksOnCatalogBase(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	catalog := newCatalogWithAlias("claude-sonnet-4", ModelCatalogManagedByAdmin, "team/best", ChannelModelPricing{
		BillingMode: BillingModeToken,
		InputPrice:  float64Ptr(9e-6),
	})
	r := NewModelPricingResolver(catalog, bs)
	group := &Group{ID: 1, ModelPricing: []ChannelModelPricing{{
		Models:      []string{"team/best"},
		BillingMode: BillingModeToken,
		OutputPrice: float64Ptr(20e-6),
	}}}

	resolved := r.Resolve(context.Background(), PricingInput{Model: "team/best", Group: group})

	require.Equal(t, PricingSourceGroup, resolved.Source)
	require.True(t, resolved.operatorPricing)
	require.Equal(t, "claude-sonnet-4", resolved.CanonicalModel)
	require.InDelta(t, 9e-6, resolved.BasePricing.InputPricePerToken, 1e-12, "input must come from the catalog entry, not the pricing file")
	require.InDelta(t, 20e-6, resolved.BasePricing.OutputPricePerToken, 1e-12, "output must come from the group card")
}

// 厂商价格政策（这里取 DeepSeek 高峰倍率）按条目 model_id 判定：请求用别名时也要生效。
func TestCalculateTokenCost_AliasAppliesVendorPolicyByCanonicalModel(t *testing.T) {
	bs := NewBillingService(&config.Config{}, nil)
	catalog := newCatalogWithAlias("deepseek-v4-flash", ModelCatalogManagedBySeed, "ds/flash", ChannelModelPricing{})
	r := NewModelPricingResolver(catalog, bs)
	// 2026-09-16 是周三，02:00 UTC 落在官方高峰段（01:00–04:00 UTC）。
	peak := time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC)
	require.Equal(t, 2.0, deepseekPeakMultiplierAt(peak))

	breakdown, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "ds/flash",
		Tokens:         UsageTokens{InputTokens: 1_000_000},
		RateMultiplier: 1,
		PricingAt:      peak,
		Resolver:       r,
	})

	require.NoError(t, err)
	require.InDelta(t, deepseekFlashOffPeakInputPrice*1_000_000*2, breakdown.InputCost, 1e-9,
		"alias request must be billed with the DeepSeek peak multiplier of its catalog entry")
}

// 只在目录里有价的模型（价格文件查不到）必须被判定为「有价」，否则会被回退到具体模型。
func TestHasResolvableTokenPricing_CatalogOnlyModel(t *testing.T) {
	bs := NewBillingService(&config.Config{}, nil)
	entry := catalogEntryFromCard("team/only", ModelCatalogManagedBySeed, ChannelModelPricing{
		BillingMode: BillingModeToken,
		InputPrice:  float64Ptr(1e-6),
		OutputPrice: float64Ptr(2e-6),
	})
	entry.ID = 1
	catalog, _ := newTestModelCatalogService(entry)
	svc := &GatewayService{billingService: bs, resolver: NewModelPricingResolver(catalog, bs)}
	ctx := context.Background()

	require.True(t, svc.hasResolvableTokenPricing(ctx, "team/only", &APIKey{}))
	require.False(t, svc.hasResolvableTokenPricing(ctx, "team/none", &APIKey{}))
	require.Equal(t, "team/only", svc.billableModelWithFallback(ctx, &APIKey{}, "team/only", "claude-sonnet-4"))

	identified, channelPriced := svc.hasIdentifiedResponseModelPricing(ctx, "team/only", &APIKey{})
	require.True(t, identified)
	require.False(t, channelPriced, "a seeded catalog entry is platform pricing, not operator pricing")

	openai := &OpenAIGatewayService{billingService: bs, resolver: svc.resolver}
	identified, channelPriced = openai.hasIdentifiedOpenAIResponsePricing(ctx, "team/only", &APIKey{})
	require.True(t, identified)
	require.False(t, channelPriced)
}

// 没有分组、也没有推理等级时，只要有解析器就必须走目录，不能退回价格文件直查。
func TestCalculateTokenCostForRequest_NoGroupUsesCatalog(t *testing.T) {
	bs := NewBillingService(&config.Config{}, nil)
	entry := catalogEntryFromCard("team/only", ModelCatalogManagedBySeed, ChannelModelPricing{
		BillingMode: BillingModeToken,
		InputPrice:  float64Ptr(1e-6),
		OutputPrice: float64Ptr(2e-6),
	})
	entry.ID = 1
	catalog, _ := newTestModelCatalogService(entry)

	breakdown, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx:            context.Background(),
		Model:          "team/only",
		Tokens:         UsageTokens{InputTokens: 1000, OutputTokens: 500},
		RateMultiplier: 1,
		Resolver:       NewModelPricingResolver(catalog, bs),
	})

	require.NoError(t, err)
	require.InDelta(t, 1e-6*1000+2e-6*500, breakdown.TotalCost, 1e-12)
}
