//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// newTokenCostTestEnv 构造带渠道定价的计费环境：group 100 挂一个渠道，定价由 pricing 指定。
// newTokenCostTestEnv 搭一个「运营者显式配了价」的环境。价卡从渠道搬到了模型目录，
// groupPlatform 只保留签名兼容（目录是全局的，不按平台隔离）。
func newTokenCostTestEnv(t *testing.T, _ string, pricing []PricingCard, catalog *PricingService) (*BillingService, *ModelPricingResolver) {
	t.Helper()
	bs := NewBillingService(&config.Config{}, catalog)
	return bs, newResolverWithCatalogCards(bs, pricing...)
}

// geminiCatalogStub 无阶梯字段的 gemini 目录条目（用于验证"无数据即无阶梯"）。
func geminiCatalogStub() *PricingService {
	return newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"gemini-2.5-pro": {
			Mode:                    "chat",
			InputCostPerToken:       1.25e-6,
			OutputCostPerToken:      10e-6,
			CacheReadInputTokenCost: 0.3125e-6,
		},
	})
}

// geminiLadderCatalogJSON 镜像真实目录的 gemini-2.5-pro 条目（含 cache_creation 基础价修正后的
// 数值）：above_200k 绝对价由解析层折算成 200K 阈值 + 输入 ×2 / 输出 ×1.5；cache 侧 above 档
// 恰为基础价 × 输入倍率（缓存写入按标准输入价，Google 对缓存写入不另收费）。
const geminiLadderCatalogJSON = `{
	"gemini-2.5-pro": {"litellm_provider": "vertex_ai-language-models", "mode": "chat",
		"input_cost_per_token": 1.25e-06, "output_cost_per_token": 1e-05,
		"cache_read_input_token_cost": 1.25e-07,
		"cache_creation_input_token_cost": 1.25e-06,
		"input_cost_per_token_above_200k_tokens": 2.5e-06,
		"output_cost_per_token_above_200k_tokens": 1.5e-05,
		"cache_read_input_token_cost_above_200k_tokens": 2.5e-07,
		"cache_creation_input_token_cost_above_200k_tokens": 2.5e-06}
}`

func geminiLadderCatalogStub(t *testing.T) *PricingService {
	t.Helper()
	return newStubPricingServiceFromJSON(t, geminiLadderCatalogJSON)
}

// 目录条目只配了平价、没配分段时，超过价格文件阶梯阈值也按平价：分段只认目录条目自己的，
// 不从价格文件继承（阶梯只在播种 / 导入时换算成分段写进条目）。
func TestCalculateTokenCostForRequest_CatalogFlatPriceDoesNotInheritPriceFileLadder(t *testing.T) {
	bs, resolver := newTokenCostTestEnv(t, PlatformGemini, []PricingCard{{
		Models: []string{"gemini-2.5-pro"}, BillingMode: BillingModeToken,
		InputPrice: testPtrFloat64(10e-6), OutputPrice: testPtrFloat64(40e-6),
	}}, geminiLadderCatalogStub(t))
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "gemini-2.5-pro"})
	require.Equal(t, PricingSourceCatalog, resolved.Source)

	tokens := UsageTokens{InputTokens: 300000, OutputTokens: 1000}
	got, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx: context.Background(), Model: "gemini-2.5-pro", Tokens: tokens, RateMultiplier: 1,
		Resolver: resolver, Resolved: resolved,
	})
	require.NoError(t, err)
	require.InDelta(t, 300000*10e-6, got.InputCost, 1e-9)
	require.InDelta(t, 1000*40e-6, got.OutputCost, 1e-9)
}

// 目录条目的分段按请求输入侧 token 数整条取价，价格文件的阶梯不再叠加。
func TestCalculateTokenCostForRequest_CatalogIntervalsPriceWholeRequest(t *testing.T) {
	bs, resolver := newTokenCostTestEnv(t, PlatformGemini, []PricingCard{{
		Models: []string{"gemini-2.5-pro"}, BillingMode: BillingModeToken,
		Intervals: []PricingInterval{{MinTokens: 0, InputPrice: testPtrFloat64(10e-6), OutputPrice: testPtrFloat64(40e-6)}},
	}}, geminiLadderCatalogStub(t))
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "gemini-2.5-pro"})
	require.Equal(t, PricingSourceCatalog, resolved.Source)
	require.NotEmpty(t, resolved.Intervals)

	tokens := UsageTokens{InputTokens: 300000, OutputTokens: 1000}
	got, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx: context.Background(), Model: "gemini-2.5-pro", Tokens: tokens, RateMultiplier: 1,
		Resolver: resolver, Resolved: resolved,
	})
	require.NoError(t, err)
	require.InDelta(t, 300000*10e-6, got.InputCost, 1e-9)
	require.InDelta(t, 1000*40e-6, got.OutputCost, 1e-9)
}

// 价格文件的阶梯（above_200k：输入 ×2、输出 ×1.5）播种成分段后走目录计费：超阈值整单按高段价。
func TestCalculateTokenCostForRequest_SeededLadderSegmentAppliesToWholeRequest(t *testing.T) {
	ps := geminiLadderCatalogStub(t)
	bs := NewBillingService(&config.Config{}, ps)
	resolver := newResolverWithSeededEntries(bs, seededLiteLLMEntry(t, ps, "gemini-2.5-pro"))

	got := costViaCatalog(t, bs, resolver, "gemini-2.5-pro", UsageTokens{InputTokens: 300000, OutputTokens: 1000})
	// 300K × 1.25e-6 × 2 = 0.75；1000 × 10e-6 × 1.5 = 0.015
	require.InDelta(t, 0.765, got.ActualCost, 1e-9)
}

// 播种出的分段对缓存分项同样生效：cache_read / cache_creation 随输入倍数整单换段，
// 分段按全部输入侧 token（input + cache_creation + cache_read）判定。
func TestCalculateTokenCostForRequest_SeededLadderSegmentAppliesToCacheItems(t *testing.T) {
	ps := geminiLadderCatalogStub(t)
	bs := NewBillingService(&config.Config{}, ps)
	resolver := newResolverWithSeededEntries(bs, seededLiteLLMEntry(t, ps, "gemini-2.5-pro"))
	calc := func(tokens UsageTokens) *CostBreakdown {
		return costViaCatalog(t, bs, resolver, "gemini-2.5-pro", tokens)
	}

	// 输入侧合计 90K + 100K + 20K = 210K > 200K：所有分项按高段计。
	// 不计 cache_creation 时只有 110K，不会过阈值——用例同时守住"缓存写入 token 计入分段判定"。
	above := calc(UsageTokens{InputTokens: 90000, CacheCreationTokens: 100000, CacheReadTokens: 20000, OutputTokens: 1000})
	require.InDelta(t, 90000*1.25e-6*2, above.InputCost, 1e-9)
	require.InDelta(t, 100000*1.25e-6*2, above.CacheCreationCost, 1e-9)
	require.InDelta(t, 20000*1.25e-7*2, above.CacheReadCost, 1e-9)
	require.InDelta(t, 1000*1e-5*1.5, above.OutputCost, 1e-9)

	// 输入侧合计 50K + 100K + 40K = 190K ≤ 200K：按基础价计
	below := calc(UsageTokens{InputTokens: 50000, CacheCreationTokens: 100000, CacheReadTokens: 40000, OutputTokens: 1000})
	require.InDelta(t, 50000*1.25e-6, below.InputCost, 1e-9)
	require.InDelta(t, 100000*1.25e-6, below.CacheCreationCost, 1e-9)
	require.InDelta(t, 40000*1.25e-7, below.CacheReadCost, 1e-9)
}

// 价格数据没有阶梯时不产生分段。
func TestCalculateTokenCostForRequest_NoLadderFieldsMeansNoLadder(t *testing.T) {
	bs, resolver := newTokenCostTestEnv(t, PlatformGemini, nil, geminiCatalogStub())
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "gemini-2.5-pro"})

	tokens := UsageTokens{InputTokens: 300000, OutputTokens: 1000}
	got, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx: context.Background(), Model: "gemini-2.5-pro", Tokens: tokens, RateMultiplier: 1,
		Resolver: resolver, Resolved: resolved,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.385, got.ActualCost, 1e-9)
}

func TestCalculateTokenCostForRequest_BuiltInPricingUsesUnifiedPath(t *testing.T) {
	bs, resolver := newTokenCostTestEnv(t, PlatformOpenAI, nil, newStubPricingServiceFromJSON(t, openAILadderCatalogJSON))
	tokens := UsageTokens{InputTokens: 300000, OutputTokens: 1000}
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "gpt-5.4"})

	got, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx: context.Background(), Model: "gpt-5.4", Tokens: tokens, RateMultiplier: 1,
		Resolver: resolver, Resolved: resolved,
	})
	require.NoError(t, err)
	want, err := bs.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "gpt-5.4", Tokens: tokens,
		RequestCount: 1, RateMultiplier: 1, Resolver: resolver,
	})
	require.NoError(t, err)
	require.Equal(t, want, got)
	// 没有目录条目、直接按价格文件计费时没有分段（分段只存在于模型目录）：超 272K 仍按基础价
	require.InDelta(t, 300000*2.5e-6, got.InputCost, 1e-9)
}

func TestCalculateTokenCostForRequest_NoResolverFallsBackToCatalog(t *testing.T) {
	bs := NewBillingService(&config.Config{}, nil)
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 10}
	got, err := bs.CalculateTokenCostForRequest(TokenCostRequest{Model: "gpt-5.4", Tokens: tokens, RateMultiplier: 1})
	require.NoError(t, err)
	want, err := bs.CalculateCost("gpt-5.4", tokens, 1)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestCalculateTokenCostForRequest_Fable51MaxEffortUsesDefaultMultiplier(t *testing.T) {
	bs := NewBillingService(&config.Config{}, nil)
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 10}
	standard, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Model: "claude-fable-5-1", Tokens: tokens, RateMultiplier: 1, ReasoningEffort: "xhigh",
	})
	require.NoError(t, err)
	max, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Model: "claude-fable-5-1", Tokens: tokens, RateMultiplier: 1, ReasoningEffort: "max",
	})
	require.NoError(t, err)
	require.InDelta(t, standard.TotalCost*3, max.TotalCost, 1e-12)
	require.InDelta(t, standard.ActualCost*3, max.ActualCost, 1e-12)
	require.InDelta(t, standard.InputCost*3, max.InputCost, 1e-12)
	require.InDelta(t, standard.OutputCost*3, max.OutputCost, 1e-12)
}

func TestCalculateTokenCostForRequest_ChannelOverridesFable51MaxEffortMultiplier(t *testing.T) {
	configured := 1.5
	bs, resolver := newTokenCostTestEnv(t, PlatformAnthropic, []PricingCard{{
		Models: []string{"claude-fable-5-1"}, BillingMode: BillingModeToken,
		InputPrice: testPtrFloat64(10e-6), OutputPrice: testPtrFloat64(50e-6),
		MaxReasoningEffortMultiplier: &configured,
	}}, nil)
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "claude-fable-5-1"})

	got, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx: context.Background(), Model: "claude-fable-5-1",
		Tokens: UsageTokens{InputTokens: 1000}, RateMultiplier: 1, ReasoningEffort: "max",
		Resolver: resolver, Resolved: resolved,
	})
	require.NoError(t, err)
	require.InDelta(t, 1000*10e-6*configured, got.TotalCost, 1e-12)
	require.InDelta(t, got.TotalCost, got.ActualCost, 1e-12)
}

// 分段里留空的缓存价 = 基础价 × 本段输入价 / 基础输入价：只填输入 / 输出时缓存读、缓存写 5 分钟 / 1 小时都同比例；
// 只填了缓存写 5 分钟时，留空的 1 小时照样同比例（不按 5 分钟价算）。
func TestCalculateTokenCostForRequest_BlankSegmentCachePricesFollowInputRatio(t *testing.T) {
	price := func(v float64) *float64 { return &v }
	base := func(model string, seg PricingInterval) ModelCatalogEntry {
		return ModelCatalogEntry{
			ModelID: model, BillingMode: BillingModeToken,
			InputPrice: price(3e-6), OutputPrice: price(15e-6),
			CacheWritePrice: price(3.75e-6), CacheWrite1hPrice: price(6e-6), CacheReadPrice: price(0.3e-6),
			Intervals: []PricingInterval{seg},
		}
	}
	onlyInputOutput := base("seg-io", PricingInterval{MinTokens: 200000, InputPrice: price(6e-6), OutputPrice: price(22.5e-6)})
	explicit5m := base("seg-5m", PricingInterval{MinTokens: 200000, InputPrice: price(6e-6), OutputPrice: price(22.5e-6), CacheWritePrice: price(7.5e-6)})
	bs := NewBillingService(&config.Config{}, nil)
	resolver := newResolverWithSeededEntries(bs, onlyInputOutput, explicit5m)
	tokens := UsageTokens{InputTokens: 100000, CacheReadTokens: 100000, CacheCreationTokens: 30000,
		CacheCreation5mTokens: 20000, CacheCreation1hTokens: 10000, OutputTokens: 1000}

	for _, model := range []string{"seg-io", "seg-5m"} {
		got := costViaCatalog(t, bs, resolver, model, tokens)
		require.InDelta(t, 100000*6e-6, got.InputCost, 1e-10, model)
		require.InDelta(t, 100000*0.3e-6*2, got.CacheReadCost, 1e-10, model)
		require.InDelta(t, 20000*3.75e-6*2+10000*6e-6*2, got.CacheCreationCost, 1e-10, model)
	}
}
