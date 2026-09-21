//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func seedInputForTest(litellm map[string]*LiteLLMModelPricing, fallback map[string]*ModelPricing) ModelCatalogSeedInput {
	var pricing *PricingService
	if litellm != nil {
		pricing = newStubPricingServiceFromMap(litellm)
	}
	bs := &BillingService{cfg: &config.Config{}, pricingService: pricing, fallbackPrices: map[string]*ModelPricing{}}
	for name, card := range fallback {
		bs.fallbackPrices[name] = card
	}
	return ModelCatalogSeedInput{PricingService: pricing, BillingService: bs}
}

func seedEntriesByModelID(entries []ModelCatalogEntry) map[string]ModelCatalogEntry {
	out := make(map[string]ModelCatalogEntry, len(entries))
	for _, entry := range entries {
		out[entry.ModelID] = entry
	}
	return out
}

func TestSeed_InsertsFromPricingFileAndFallbackTable(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"claude-sonnet-4": {
				LiteLLMProvider:    "anthropic",
				InputCostPerToken:  3e-6,
				OutputCostPerToken: 15e-6,
			},
		},
		map[string]*ModelPricing{
			"glm-5.2": {InputPricePerToken: 0.6e-6, OutputPricePerToken: 2.2e-6},
		},
	))

	result, err := svc.Seed(context.Background())
	require.NoError(t, err)
	// 2 条来自价格文件 / 兜底表 + 5 条 xAI Imagine 媒体种子
	require.Equal(t, 2+len(xaiImagineSeeds()), result.Inserted)
	require.Zero(t, result.Refreshed)
	require.Zero(t, result.SkippedAdmin)

	entries := seedEntriesByModelID(repo.entries)
	require.Len(t, entries, 2+len(xaiImagineSeeds()))

	sonnet := entries["claude-sonnet-4"]
	require.Equal(t, ModelCatalogManagedBySeed, sonnet.ManagedBy)
	require.Equal(t, "anthropic", sonnet.Vendor)
	require.Equal(t, BillingModeToken, sonnet.BillingMode)
	require.InDelta(t, 3e-6, *sonnet.InputPrice, 1e-12)
	require.InDelta(t, 15e-6, *sonnet.OutputPrice, 1e-12)

	glm := entries["glm-5.2"]
	require.Equal(t, ModelCatalogManagedBySeed, glm.ManagedBy)
	require.InDelta(t, 0.6e-6, *glm.InputPrice, 1e-12)
}

// 今天的解析顺序是「价格文件 → 硬编码兜底价」，播种不能把这个优先级翻过来。
func TestSeed_PricingFileWinsOverFallbackTableForSameModel(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"claude-sonnet-4": {LiteLLMProvider: "anthropic", InputCostPerToken: 3e-6},
		},
		map[string]*ModelPricing{
			"claude-sonnet-4": {InputPricePerToken: 99e-6},
		},
	))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	require.Len(t, entries, 1+len(xaiImagineSeeds()))
	require.InDelta(t, 3e-6, *entries["claude-sonnet-4"].InputPrice, 1e-12)
}

// 价格文件的查表阶梯（去日期后缀等）也能定到价时，今天走的是价格文件那一份。
// 把兜底价播进目录会让目录反过来压住它。
func TestSeed_SkipsFallbackWhenPricingFileCanAlreadyPriceIt(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"claude-sonnet-4-20250514": {LiteLLMProvider: "anthropic", InputCostPerToken: 3e-6},
		},
		map[string]*ModelPricing{
			// 价格文件能通过去日期后缀定到 claude-sonnet-4-20250514。
			"claude-sonnet-4-20250514-thinking": {InputPricePerToken: 99e-6},
		},
	))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	require.Contains(t, entries, "claude-sonnet-4-20250514")
	require.NotContains(t, entries, "claude-sonnet-4-20250514-thinking")
}

// 仅有图片价、没有 token 价的条目今天会被 getModelPricingAt 明确拒绝
// （否则 token 流量按 $0 计费）。播种它们等于把这条拒绝绕过去。
func TestSeed_SkipsTokenPricingAbsentEntries(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"image-only": {
				LiteLLMProvider:         "gemini",
				OutputCostPerImageToken: 30e-6,
				TokenPricingAbsent:      true,
			},
			"normal": {LiteLLMProvider: "gemini", InputCostPerToken: 1e-6},
		},
		nil,
	))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	require.NotContains(t, entries, "image-only")
	require.Contains(t, entries, "normal")
}

// 可重复执行：seed 条目按最新价格文件刷新，admin 条目原样跳过。
func TestSeed_RefreshesSeedEntriesAndNeverOverwritesAdminEdits(t *testing.T) {
	repo := &stubModelCatalogRepo{entries: []ModelCatalogEntry{
		{ID: 1, ModelID: "seeded", BillingMode: BillingModeToken, Status: ModelCatalogStatusListed,
			ManagedBy: ModelCatalogManagedBySeed, InputPrice: testPtrFloat64(1e-9)},
		{ID: 2, ModelID: "hand-tuned", BillingMode: BillingModeToken, Status: ModelCatalogStatusListed,
			ManagedBy: ModelCatalogManagedByAdmin, InputPrice: testPtrFloat64(42e-6)},
	}}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"seeded":     {LiteLLMProvider: "anthropic", InputCostPerToken: 3e-6},
			"hand-tuned": {LiteLLMProvider: "anthropic", InputCostPerToken: 7e-6},
		},
		nil,
	))

	result, err := svc.Seed(context.Background())
	require.NoError(t, err)
	require.Equal(t, len(xaiImagineSeeds()), result.Inserted, "只有 Imagine 媒体种子是新插入的")
	require.Equal(t, 1, result.Refreshed)
	require.Equal(t, 1, result.SkippedAdmin)

	entries := seedEntriesByModelID(repo.entries)
	require.InDelta(t, 3e-6, *entries["seeded"].InputPrice, 1e-12, "seed 条目应被刷新到当前价格文件")
	require.InDelta(t, 42e-6, *entries["hand-tuned"].InputPrice, 1e-12, "admin 条目不得被播种覆盖")
}

// 播种中途中止（ctx 到期）：已写进去的条目要立刻可查，错误照样返回给调用方。
func TestSeed_PartialWriteStillInvalidatesSnapshot(t *testing.T) {
	repo := &stubModelCatalogRepo{seedAbortAfter: 1}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"a-first":  {LiteLLMProvider: "anthropic", InputCostPerToken: 1e-6},
			"b-second": {LiteLLMProvider: "anthropic", InputCostPerToken: 2e-6},
		},
		nil,
	))
	// 先把空目录装进快照，验证播种后快照确实被失效。
	require.Nil(t, svc.LookupPricingEntry(context.Background(), "a-first"))

	result, err := svc.Seed(context.Background())

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, 1, result.Inserted)
	require.NotNil(t, svc.LookupPricingEntry(context.Background(), "a-first"), "rows written before the abort must be visible")
}

// 5m/1h 分档只在 1h 价严格高于 5m 价时成立，与 getModelPricingAt 同口径。
func TestSeed_OmitsCacheWrite1hWhenNotHigherThan5m(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"higher": {
				LiteLLMProvider:                     "anthropic",
				InputCostPerToken:                   3e-6,
				CacheCreationInputTokenCost:         3.75e-6,
				CacheCreationInputTokenCostAbove1hr: 6e-6,
			},
			"not-higher": {
				LiteLLMProvider:                     "anthropic",
				InputCostPerToken:                   3e-6,
				CacheCreationInputTokenCost:         3.75e-6,
				CacheCreationInputTokenCostAbove1hr: 1e-6,
			},
		},
		nil,
	))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	require.NotNil(t, entries["higher"].CacheWrite1hPrice)
	require.Nil(t, entries["not-higher"].CacheWrite1hPrice)
}

// 价格文件用零值表示缺失。播种成 0 会让目录把「没配这一项」变成「显式收 0」，
// 进而关掉下游的回退（如图片输出价回退到文本输出价）。
func TestSeed_TreatsZeroPricesAsUnset(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"m": {LiteLLMProvider: "anthropic", InputCostPerToken: 3e-6},
		},
		nil,
	))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entry := seedEntriesByModelID(repo.entries)["m"]
	require.NotNil(t, entry.InputPrice)
	require.Nil(t, entry.OutputPrice)
	require.Nil(t, entry.CacheWritePrice)
	require.Nil(t, entry.ImageOutputPrice)
}

// xAI 的长上下文阈值语义是「达到即进高档」，其余提供商严格大于。
func TestSeed_MarksInclusiveLongContextThresholdForXAI(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"grok-4": {
				LiteLLMProvider:                "xai",
				InputCostPerToken:              3e-6,
				LongContextInputTokenThreshold: 128000,
				LongContextInputCostMultiplier: 2,
			},
			"claude-x": {
				LiteLLMProvider:                "anthropic",
				InputCostPerToken:              3e-6,
				LongContextInputTokenThreshold: 200000,
				LongContextInputCostMultiplier: 2,
			},
		},
		nil,
	))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	require.True(t, entries["grok-4"].LongContextThresholdInclusive)
	require.False(t, entries["claude-x"].LongContextThresholdInclusive)
	require.Equal(t, 128000, *entries["grok-4"].LongContextInputThreshold)
}

// 播种出来的条目是平台默认价卡，不是运营者定价：DeepSeek 官方价强制覆盖与
// 峰谷倍率必须照旧生效——否则播种一上线，官方价政策就会整体失效。
func TestSeededCatalogEntryStaysPlatformDefaultPricing(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	// 价格文件里故意放一个远低于官方价的数，用来证明官方价强制覆盖确实跑了。
	seed := seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"deepseek-flash": {LiteLLMProvider: "deepseek", InputCostPerToken: 1e-9, OutputCostPerToken: 1e-9},
		},
		nil,
	)
	svc := NewModelCatalogService(repo, nil, seed)
	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	resolver := NewModelPricingResolver(svc, seed.BillingService)
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "deepseek-flash"})
	require.Equal(t, PricingSourceCatalog, resolved.Source)
	require.False(t, resolved.operatorPricing, "播种条目不是运营者定价")

	// 低谷时段（北京时间周日全天低谷）：官方 Flash 低谷价 $0.15/MTok。
	offPeak := time.Date(2026, time.August, 23, 12, 0, 0, 0, time.UTC)
	cost, err := seed.BillingService.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "deepseek-flash",
		Tokens: UsageTokens{InputTokens: 1_000_000}, RateMultiplier: 1,
		PricingAt: offPeak, Resolver: resolver, Resolved: resolved,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.15, cost.TotalCost, 1e-9, "官方 DeepSeek 价必须强制覆盖播种进来的价")

	// 高峰时段（工作日 02:00 UTC）：2× 低谷价。
	peak := time.Date(2026, time.August, 24, 2, 0, 0, 0, time.UTC)
	peakCost, err := seed.BillingService.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "deepseek-flash",
		Tokens: UsageTokens{InputTokens: 1_000_000}, RateMultiplier: 1,
		PricingAt: peak, Resolver: resolver, Resolved: resolved,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.30, peakCost.TotalCost, 1e-9, "峰谷倍率必须照旧叠加在播种条目上")
}

// 管理员改过的目录条目是运营者定价：官方价强制覆盖与峰谷倍率都不再叠加。
func TestAdminEditedDeepSeekEntryKeepsOperatorPrice(t *testing.T) {
	bs := &BillingService{cfg: &config.Config{}, fallbackPrices: map[string]*ModelPricing{}}
	svc, _ := newTestModelCatalogService(ModelCatalogEntry{
		ID: 1, ModelID: "deepseek-flash", BillingMode: BillingModeToken,
		Status: ModelCatalogStatusListed, ManagedBy: ModelCatalogManagedByAdmin,
		InputPrice: testPtrFloat64(1e-6),
	})
	resolver := NewModelPricingResolver(svc, bs)
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "deepseek-flash"})
	require.True(t, resolved.operatorPricing)

	peak := time.Date(2026, time.August, 24, 2, 0, 0, 0, time.UTC)
	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "deepseek-flash",
		Tokens: UsageTokens{InputTokens: 1_000_000}, RateMultiplier: 1,
		PricingAt: peak, Resolver: resolver, Resolved: resolved,
	})
	require.NoError(t, err)
	require.InDelta(t, 1.0, cost.TotalCost, 1e-9, "运营者定价不被官方价覆盖，也不叠加峰谷倍率")
}

func TestAdminEditedCatalogEntryIsOperatorPricing(t *testing.T) {
	bs := &BillingService{fallbackPrices: map[string]*ModelPricing{}}
	svc, _ := newTestModelCatalogService(ModelCatalogEntry{
		ID: 1, ModelID: "m", BillingMode: BillingModeToken, Status: ModelCatalogStatusListed,
		ManagedBy: ModelCatalogManagedByAdmin, InputPrice: testPtrFloat64(1e-6),
	})
	resolver := NewModelPricingResolver(svc, bs)

	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "m"})
	require.Equal(t, PricingSourceCatalog, resolved.Source)
	require.True(t, resolved.operatorPricing)
}
