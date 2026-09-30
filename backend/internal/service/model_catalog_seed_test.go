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

// 价格文件的长上下文阶梯播种成按 token 分段：xAI「达到即进高段」下界取阈值 - 1，其余提供商严格大于、下界就是阈值。
func TestSeed_LongContextLadderBecomesTokenSegment(t *testing.T) {
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
	grok := entries["grok-4"].Intervals
	require.Len(t, grok, 1)
	require.Equal(t, 127999, grok[0].MinTokens)
	require.Nil(t, grok[0].MaxTokens)
	require.InDelta(t, 6e-6, *grok[0].InputPrice, 1e-15)
	require.Nil(t, grok[0].OutputPrice, "基础价没配的项分段也不配")
	claude := entries["claude-x"].Intervals
	require.Len(t, claude, 1)
	require.Equal(t, 200000, claude[0].MinTokens)
}

// 兜底价表的阶梯表（fallbackSeedLadders）每个模型都得在兜底价表里、播种出恰好一个分段；拼错模型名会静默失效，这里 fail-closed。
func TestFallbackSeedLaddersCoverFallbackModels(t *testing.T) {
	fallback := NewBillingService(&config.Config{}, nil).SnapshotFallbackPricing()
	require.NotEmpty(t, fallbackSeedLadders)
	for name := range fallbackSeedLadders {
		pricing, ok := fallback[name]
		require.True(t, ok, "fallbackSeedLadders has %q but the fallback table does not", name)
		entry := seedEntryFromFallback(name, pricing)
		require.Len(t, entry.Intervals, 1, name)
	}
	astra := seedEntryFromFallback("gpt-6-astra", fallback["gpt-6-astra"]).Intervals[0]
	require.Equal(t, 272000, astra.MinTokens)
	require.InDelta(t, 10e-6*2, *astra.InputPrice, 1e-15)
	require.InDelta(t, 50e-6*1.5, *astra.OutputPrice, 1e-15)
	require.Equal(t, 199999, seedEntryFromFallback("grok-4.5", fallback["grok-4.5"]).Intervals[0].MinTokens)
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

// 价格文件里 mode=image_generation 且带每张价（output_cost_per_image）的条目播成 image 模式，
// 默认按次价 = 每张价；token 价照抄，同一模型经对话入口没有图片输出时仍按 token 算。
func TestSeed_ImageGenerationWithPerImagePriceSeedsImageMode(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"gemini-2.5-flash-image": {
				LiteLLMProvider: "gemini", Mode: "image_generation",
				InputCostPerToken: 3e-7, OutputCostPerToken: 2.5e-6,
				OutputCostPerImage: 0.039, OutputCostPerImageToken: 3e-5,
			},
			// gpt-image 系只有 token 价：仍是 token 模式
			"gpt-image-2": {
				LiteLLMProvider: "openai", Mode: "image_generation",
				InputCostPerToken: 5e-6, OutputCostPerImageToken: 4e-5,
			},
			// chat 模型即使带 output_cost_per_image 也不按张（mode 不是 image_generation）
			"chatty": {LiteLLMProvider: "openai", Mode: "chat", InputCostPerToken: 1e-6, OutputCostPerImage: 0.5},
		},
		nil,
	))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	flashImage := entries["gemini-2.5-flash-image"]
	require.Equal(t, BillingModeImage, flashImage.BillingMode)
	require.NotNil(t, flashImage.PerRequestPrice)
	require.InDelta(t, 0.039, *flashImage.PerRequestPrice, 1e-12)
	require.NotNil(t, flashImage.OutputPrice)
	require.InDelta(t, 2.5e-6, *flashImage.OutputPrice, 1e-15)
	require.Equal(t, BillingModeToken, entries["gpt-image-2"].BillingMode)
	require.Nil(t, entries["gpt-image-2"].PerRequestPrice)
	require.Equal(t, BillingModeToken, entries["chatty"].BillingMode)
	require.Nil(t, entries["chatty"].PerRequestPrice)
}

// 模型内置搜索价只存 medium 档（拍的）；没有 search_context_cost_per_query 的条目留空。
func TestSeed_SearchContextCostSeedsSearchPricePerCall(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"gpt-5.6": {
				LiteLLMProvider: "openai", InputCostPerToken: 1e-6,
				SearchContextCostPerQuery: map[string]float64{
					"search_context_size_low": 0.008, "search_context_size_medium": 0.01, "search_context_size_high": 0.012,
				},
			},
			"plain": {LiteLLMProvider: "openai", InputCostPerToken: 1e-6},
		},
		nil,
	))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	require.NotNil(t, entries["gpt-5.6"].SearchPricePerCall)
	require.InDelta(t, 0.01, *entries["gpt-5.6"].SearchPricePerCall, 1e-12)
	require.Nil(t, entries["plain"].SearchPricePerCall)
}

// xAI Imagine 种子与硬编码兜底价同一优先级：价格文件已有该模型时不播。
func TestSeed_ImagineSeedsSkippedWhenPricingFileHasModel(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"grok-imagine-image-quality": {LiteLLMProvider: "xai", Mode: "image_generation", OutputCostPerImage: 0.09, InputCostPerToken: 1e-6},
		},
		nil,
	))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	quality := entries["grok-imagine-image-quality"]
	require.InDelta(t, 0.09, *quality.PerRequestPrice, 1e-12, "价格文件的价赢过种子")
	require.Empty(t, quality.Intervals, "价格文件条目不带种子的分档")
	// 其余四条 Imagine 种子照常播入
	require.Contains(t, entries, "grok-imagine-video-1.5")
	require.Len(t, entries, len(xaiImagineSeeds()))
}

// Imagine 种子带分档与别名一起落库；再次播种时分档随种子刷新、别名只补不删。
func TestSeed_ImagineSeedsCarryIntervalsAndAliases(t *testing.T) {
	repo := &stubModelCatalogRepo{}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(map[string]*LiteLLMModelPricing{
		"claude-sonnet-4": {LiteLLMProvider: "anthropic", InputCostPerToken: 3e-6},
	}, nil))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	quality := entries["grok-imagine-image-quality"]
	require.Equal(t, BillingModeImage, quality.BillingMode)
	require.InDelta(t, 0.05, *quality.PerRequestPrice, 1e-12)
	require.Len(t, quality.Intervals, 2)
	require.Equal(t, ImageBillingSize1K, quality.Intervals[0].TierLabel)
	require.InDelta(t, 0.05, *quality.Intervals[0].PerRequestPrice, 1e-12)
	require.Equal(t, ImageBillingSize2K, quality.Intervals[1].TierLabel)
	require.InDelta(t, 0.07, *quality.Intervals[1].PerRequestPrice, 1e-12)
	aliases := make([]string, 0, len(quality.Aliases))
	for _, alias := range quality.Aliases {
		require.Equal(t, ModelCatalogAliasSourceSeed, alias.Source)
		aliases = append(aliases, alias.Alias)
	}
	require.ElementsMatch(t, []string{"grok-imagine", "grok-imagine-1", "grok-imagine-edit"}, aliases)

	video15 := entries["grok-imagine-video-1.5"]
	require.Equal(t, BillingModeVideo, video15.BillingMode)
	require.Len(t, video15.Intervals, 3)
	require.Equal(t, VideoBillingResolution1080P, video15.Intervals[2].TierLabel)
	require.InDelta(t, 0.25, *video15.Intervals[2].PerRequestPrice, 1e-12)

	// 别名解析：经目录快照 grok-imagine → quality 条目
	catalog, _ := newTestModelCatalogService(repo.entries...)
	resolved := catalog.LookupPricingEntry(context.Background(), "grok-imagine")
	require.NotNil(t, resolved)
	require.Equal(t, "grok-imagine-image-quality", resolved.ModelID)

	// 重播：条目刷新而不是重复插入，分档与别名保持
	second, err := svc.Seed(context.Background())
	require.NoError(t, err)
	require.Zero(t, second.Inserted)
	requality := seedEntriesByModelID(repo.entries)["grok-imagine-image-quality"]
	require.Len(t, requality.Intervals, 2)
	require.Len(t, requality.Aliases, 3)
}

// 管理员已手建同名别名指向别的条目时，种子别名跳过、不覆盖。
func TestSeed_AliasConflictKeepsAdminAlias(t *testing.T) {
	repo := &stubModelCatalogRepo{entries: []ModelCatalogEntry{
		{ID: 1, ModelID: "my-image", BillingMode: BillingModeImage, Status: ModelCatalogStatusListed,
			ManagedBy: ModelCatalogManagedByAdmin, PerRequestPrice: testPtrFloat64(0.5),
			Aliases: []ModelCatalogAlias{{EntryID: 1, Alias: "grok-imagine", Source: ModelCatalogAliasSourceManual}}},
	}}
	svc := NewModelCatalogService(repo, nil, seedInputForTest(nil, nil))

	_, err := svc.Seed(context.Background())
	require.NoError(t, err)

	entries := seedEntriesByModelID(repo.entries)
	quality := entries["grok-imagine-image-quality"]
	aliases := make([]string, 0, len(quality.Aliases))
	for _, alias := range quality.Aliases {
		aliases = append(aliases, alias.Alias)
	}
	require.ElementsMatch(t, []string{"grok-imagine-1", "grok-imagine-edit"}, aliases, "被占用的 grok-imagine 不写")
	require.Equal(t, "grok-imagine", entries["my-image"].Aliases[0].Alias, "管理员别名不动")
}

func TestValidateIntervals_ImageVideoTiersRequireLabelAndPrice(t *testing.T) {
	price := 0.1
	cases := []struct {
		name      string
		mode      BillingMode
		intervals []PricingInterval
		wantErr   string
	}{
		{"image ok", BillingModeImage, []PricingInterval{{TierLabel: "1K", PerRequestPrice: &price}, {TierLabel: "2K", PerRequestPrice: &price}}, ""},
		{"image missing label", BillingModeImage, []PricingInterval{{PerRequestPrice: &price}}, "requires a tier_label"},
		{"image unknown label", BillingModeImage, []PricingInterval{{TierLabel: "1k", PerRequestPrice: &price}}, "unknown image tier_label"},
		{"image duplicate label", BillingModeImage, []PricingInterval{{TierLabel: "1K", PerRequestPrice: &price}, {TierLabel: "1K", PerRequestPrice: &price}}, "duplicate image tier_label"},
		{"image missing price", BillingModeImage, []PricingInterval{{TierLabel: "1K"}}, "requires a per_request_price"},
		{"video ok", BillingModeVideo, []PricingInterval{{TierLabel: "480p", PerRequestPrice: &price}, {TierLabel: "1080p", PerRequestPrice: &price}}, ""},
		{"video unknown label", BillingModeVideo, []PricingInterval{{TierLabel: "4K", PerRequestPrice: &price}}, "unknown video tier_label"},
		{"per_request free label", BillingModePerRequest, []PricingInterval{{TierLabel: "tts", PerRequestPrice: &price}}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateIntervals(tc.intervals, tc.mode)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// 图片 / 视频条目上架必须带默认按次价（计费路径没有默认价可退）；unlisted 与 token 模式不受影响。
func TestModelCatalogEntry_Validate_ListedImageVideoRequiresPerRequestPrice(t *testing.T) {
	price := 0.05
	base := func(mode BillingMode, status string, perRequest *float64) ModelCatalogEntry {
		entry := ModelCatalogEntry{ModelID: "m", BillingMode: mode, Status: status, ManagedBy: ModelCatalogManagedByAdmin,
			PerRequestPrice: perRequest, InputPrice: &price,
			Intervals: []PricingInterval{{TierLabel: "1K", PerRequestPrice: &price}}}
		if mode == BillingModeVideo {
			entry.Intervals[0].TierLabel = "480p"
		}
		entry.Normalize()
		return entry
	}
	for _, mode := range []BillingMode{BillingModeImage, BillingModeVideo} {
		listed := base(mode, ModelCatalogStatusListed, nil)
		require.ErrorContains(t, listed.Validate(), "must have a per_request_price", string(mode))
		withPrice := base(mode, ModelCatalogStatusListed, &price)
		require.NoError(t, withPrice.Validate())
		unlisted := base(mode, ModelCatalogStatusUnlisted, nil)
		require.NoError(t, unlisted.Validate())
	}
	token := ModelCatalogEntry{ModelID: "t", BillingMode: BillingModeToken, Status: ModelCatalogStatusListed, ManagedBy: ModelCatalogManagedByAdmin, InputPrice: &price}
	token.Normalize()
	require.NoError(t, token.Validate())
}

// 兜底价表的每个模型都要标上厂商（2026-09-29 E2E：grok / glm / claude-fable 等播出来厂商为空，
// 用户站模型页归不到厂商）。表里新增模型族时漏了 modelFamilyVendors 这里会红。
func TestFallbackSeedEntriesAllHaveVendor(t *testing.T) {
	billing := NewBillingService(&config.Config{}, nil)
	fallback := billing.SnapshotFallbackPricing()
	require.NotEmpty(t, fallback)
	for name, pricing := range fallback {
		entry := seedEntryFromFallback(name, pricing)
		require.NotEmpty(t, entry.Vendor, "fallback seed %q has no vendor", name)
	}
	require.Equal(t, "xai", seedEntryFromFallback("grok-4.5", fallback["grok-4.5"]).Vendor)
	require.Equal(t, "zhipu", seedEntryFromFallback("glm-5.3", fallback["glm-5.3"]).Vendor)
	grok := seedEntryFromFallback("grok-4.6", fallback["grok-4.6"])
	require.Equal(t, PlatformGrok, CatalogVendorPlatform(&grok), "grok 条目归到 Grok 平台")
}
