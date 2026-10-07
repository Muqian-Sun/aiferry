//go:build unit

package service

import (
	"bytes"
	"log"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// captureStdLog 重定向 stdlib log 输出到 buffer,返回该 buffer;通过 t.Cleanup 还原。
// 用于断言 GetModelPricing 的 fallback warn(log.Printf)打了几次。
func captureStdLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

func newTestBillingService() *BillingService {
	return NewBillingService()
}

// openAILadderCatalogJSON 镜像真实同步目录的形态：长上下文用 above_272k 绝对价字段表达，
// 由解析层折算成阈值+倍率。静态 Go 兜底价不再携带阶梯，阶梯计费一律走目录数据。
// 只有 unit 标签的用例（本文件与 billing_token_cost_request_test.go）用；放在无标签的
// pricing_stub_helpers_test.go 里会被默认标签下的 lint 判成未使用。
const openAILadderCatalogJSON = `{
	"gpt-5.4": {"litellm_provider": "openai", "mode": "chat",
		"input_cost_per_token": 2.5e-06, "output_cost_per_token": 1.5e-05,
		"cache_read_input_token_cost": 2.5e-07, "cache_creation_input_token_cost": 2.5e-06,
		"input_cost_per_token_above_272k_tokens": 5e-06,
		"output_cost_per_token_above_272k_tokens": 2.25e-05,
		"cache_read_input_token_cost_above_272k_tokens": 5e-07},
	"gpt-5.5-pro": {"litellm_provider": "openai", "mode": "chat",
		"input_cost_per_token": 3e-05, "output_cost_per_token": 1.8e-04,
		"input_cost_per_token_above_272k_tokens": 6e-05,
		"output_cost_per_token_above_272k_tokens": 2.7e-04}
}`

func newTestBillingServiceWithOpenAILadderCatalog(t *testing.T) *BillingService {
	t.Helper()
	return NewBillingService()
}

// newOpenAILadderSeededEnv 按 openAILadderCatalogJSON 播种 gpt-5.4（above_272k 阶梯换算成分段），走目录计费。
func newOpenAILadderSeededEnv(t *testing.T) (*BillingService, *ModelPricingResolver) {
	t.Helper()
	ps := newStubPricingServiceFromJSON(t, openAILadderCatalogJSON)
	bs := NewBillingService()
	return bs, newResolverWithSeededEntries(bs, seededLiteLLMEntry(t, ps, "gpt-5.4"))
}

func TestCalculateCost_BasicComputation(t *testing.T) {
	svc := newTestBillingService()

	// 使用 claude-sonnet-4 的回退价格：Input $3/MTok, Output $15/MTok
	tokens := UsageTokens{
		InputTokens:  1000,
		OutputTokens: 500,
	}
	cost, err := builtinCatalogCost(svc, "claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	// 1000 * 3e-6 = 0.003, 500 * 15e-6 = 0.0075
	expectedInput := 1000 * 3e-6
	expectedOutput := 500 * 15e-6
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.ActualCost, 1e-10)
}

func TestCalculateCost_WithCacheTokens(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheCreationTokens: 2000,
		CacheReadTokens:     3000,
	}
	cost, err := builtinCatalogCost(svc, "claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	expectedCacheCreation := 2000 * 3.75e-6
	expectedCacheRead := 3000 * 0.3e-6
	require.InDelta(t, expectedCacheCreation, cost.CacheCreationCost, 1e-10)
	require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10)

	expectedTotal := cost.InputCost + cost.OutputCost + expectedCacheCreation + expectedCacheRead
	require.InDelta(t, expectedTotal, cost.TotalCost, 1e-10)
}

func TestCalculateCost_RateMultiplier(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 500}

	cost1x, err := builtinCatalogCost(svc, "claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	cost2x, err := builtinCatalogCost(svc, "claude-sonnet-4", tokens, 2.0)
	require.NoError(t, err)

	// TotalCost 不受倍率影响，ActualCost 翻倍
	require.InDelta(t, cost1x.TotalCost, cost2x.TotalCost, 1e-10)
	require.InDelta(t, cost1x.ActualCost*2, cost2x.ActualCost, 1e-10)
}

// 回归:glm-5.2 必须命中自己的兜底价,不能被 strings.Contains("glm-5") 抢成 glm-5 价。
// 历史 bug:兜底表缺 glm-5.2 条目,使用记录按 $1.00/$3.20 计费,比官方 $1.40/$4.40 少收约 27%。
func TestGetModelPricing_GLM52UsesOwnPrice(t *testing.T) {
	svc := newTestBillingService()

	got, err := builtinPricing(svc, "glm-5.2")
	require.NoError(t, err)
	require.NotNil(t, got)

	// 国内站人民币价 ÷ 6.8、每百万 Token 4 位小数（muqian 2026-10-07）：¥8 / ¥28 → $1.1765 / $4.1176，缓存命中 ¥2 → $0.2941。
	require.InDelta(t, 1.1765e-6, got.InputPricePerToken, 1e-15)
	require.InDelta(t, 4.1176e-6, got.OutputPricePerToken, 1e-15)
	require.InDelta(t, 0.2941e-6, got.CacheReadPricePerToken, 1e-15)
}

func TestGetModelPricing_OpenAIGPT54Fallback(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := builtinPricing(svc, "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, pricing)
	require.InDelta(t, 2.5e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 15e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 0.25e-6, pricing.CacheReadPricePerToken, 1e-12)
}

// 价格文件的 above_272k 字段播种成一个按 token 分段：openai 严格大于，下界就是 272000；各价 = 基础价 × 倍数。
func TestSeed_OpenAIAboveTierFieldsBecomeTokenSegment(t *testing.T) {
	entry := seededLiteLLMEntry(t, newStubPricingServiceFromJSON(t, openAILadderCatalogJSON), "gpt-5.4")
	require.Len(t, entry.Intervals, 1)
	seg := entry.Intervals[0]
	require.Equal(t, 272000, seg.MinTokens)
	require.Nil(t, seg.MaxTokens)
	require.InDelta(t, 2.5e-6*2, *seg.InputPrice, 1e-15)
	require.InDelta(t, 15e-6*1.5, *seg.OutputPrice, 1e-15)
	require.InDelta(t, 2.5e-6*2, *seg.CacheWritePrice, 1e-15)
	require.InDelta(t, 0.25e-6*2, *seg.CacheReadPrice, 1e-15)
}

func TestGetModelPricing_OpenAIGPT54MiniFallback(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := builtinPricing(svc, "gpt-5.4-mini")
	require.NoError(t, err)
	require.NotNil(t, pricing)
	require.InDelta(t, 7.5e-7, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 4.5e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 7.5e-8, pricing.CacheReadPricePerToken, 1e-12)
}

func TestCalculateCost_OpenAIGPT54TokenSegmentAppliesToWholeRequest(t *testing.T) {
	bs, resolver := newOpenAILadderSeededEnv(t)

	tokens := UsageTokens{
		InputTokens:  300000,
		OutputTokens: 4000,
	}

	cost := costViaCatalog(t, bs, resolver, "gpt-5.4", tokens)

	expectedInput := float64(tokens.InputTokens) * 2.5e-6 * 2.0
	expectedOutput := float64(tokens.OutputTokens) * 15e-6 * 1.5
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.ActualCost, 1e-10)
}

func TestCalculateCost_OpenAIGPT55ProUsesGPT55PricingPolicy(t *testing.T) {
	bs, resolver := newSeededCatalogEnvFromJSON(t, openAILadderCatalogJSON, "gpt-5.5-pro")

	tokens := UsageTokens{
		InputTokens:  300000,
		OutputTokens: 4000,
	}

	cost := costViaCatalog(t, bs, resolver, "gpt-5.5-pro", tokens)

	expectedInput := float64(tokens.InputTokens) * 30e-6 * 2.0
	expectedOutput := float64(tokens.OutputTokens) * 180e-6 * 1.5
	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedInput+expectedOutput, cost.ActualCost, 1e-10)
}

func TestFallbackPricing_OpenAIGPT55UsesOfficialPrices(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := builtinPricing(svc, "gpt-5.5")
	require.NoError(t, err)
	require.InDelta(t, 5e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 30e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 0.5e-6, pricing.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, 5e-6, pricing.CacheCreationPricePerToken, 1e-12)
}

func TestFallbackPricing_OpenAIGPT55ProUsesOfficialPrices(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := builtinPricing(svc, "gpt-5.5-pro")
	require.NoError(t, err)
	require.InDelta(t, 30e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 180e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 30e-6, pricing.CacheReadPricePerToken, 1e-12)
	require.InDelta(t, 30e-6, pricing.CacheCreationPricePerToken, 1e-12)
}

// 回归测试 #2293：落在高段时 cache_read 也按高段价（= 基础价 × 输入倍数 2）；播种时换算进分段的 cache_read_price。
func TestCalculateCost_OpenAIGPT54TokenSegmentAppliesToCacheRead(t *testing.T) {
	bs, resolver := newOpenAILadderSeededEnv(t)

	// InputTokens + CacheReadTokens = 1000 + 300000 = 301000 > 272000 阈值
	tokens := UsageTokens{
		InputTokens:     1000,
		CacheReadTokens: 300000,
		OutputTokens:    1000,
	}

	cost := costViaCatalog(t, bs, resolver, "gpt-5.4", tokens)

	expectedInput := float64(tokens.InputTokens) * 2.5e-6 * 2.0
	expectedOutput := float64(tokens.OutputTokens) * 15e-6 * 1.5
	expectedCacheRead := float64(tokens.CacheReadTokens) * 0.25e-6 * 2.0

	require.InDelta(t, expectedInput, cost.InputCost, 1e-10)
	require.InDelta(t, expectedOutput, cost.OutputCost, 1e-10)
	require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10,
		"cache_read_cost should use the upper segment price (issue #2293)")

	expectedTotal := expectedInput + expectedOutput + expectedCacheRead
	require.InDelta(t, expectedTotal, cost.TotalCost, 1e-10)
	require.InDelta(t, expectedTotal, cost.ActualCost, 1e-10)
}

// 阴性测试：没到高段时 cache_read 按基础价。
func TestCalculateCost_OpenAIGPT54BelowSegmentKeepsCacheReadAtBasePrice(t *testing.T) {
	bs, resolver := newOpenAILadderSeededEnv(t)

	// InputTokens + CacheReadTokens = 1000 + 100000 = 101000 < 272000，落在基础段
	tokens := UsageTokens{
		InputTokens:     1000,
		CacheReadTokens: 100000,
		OutputTokens:    1000,
	}

	cost := costViaCatalog(t, bs, resolver, "gpt-5.4", tokens)

	expectedCacheRead := float64(tokens.CacheReadTokens) * 0.25e-6
	require.InDelta(t, expectedCacheRead, cost.CacheReadCost, 1e-10,
		"cache_read_cost should remain at base price below the segment")
}

// 回归测试 #2816 follow-up：落在高段时 cache_creation 也按高段价（= 基础价 × 输入倍数 2）。
func TestCalculateCost_OpenAIGPT54TokenSegmentAppliesToCacheCreation(t *testing.T) {
	bs, resolver := newOpenAILadderSeededEnv(t)

	// InputTokens + CacheReadTokens = 1000 + 300000 = 301000 > 272000 阈值
	tokens := UsageTokens{
		InputTokens:         1000,
		CacheReadTokens:     300000,
		CacheCreationTokens: 10000,
		OutputTokens:        1000,
	}

	cost := costViaCatalog(t, bs, resolver, "gpt-5.4", tokens)

	// gpt-5.4: 基础缓存写 2.5e-6，高段 = × 2
	expectedCacheCreation := float64(tokens.CacheCreationTokens) * 2.5e-6 * 2.0
	require.InDelta(t, expectedCacheCreation, cost.CacheCreationCost, 1e-10,
		"cache_creation_cost should use the upper segment price")
}

// 阴性测试：没到高段时 cache_creation 按基础价。
func TestCalculateCost_OpenAIGPT54BelowSegmentKeepsCacheCreationAtBasePrice(t *testing.T) {
	bs, resolver := newOpenAILadderSeededEnv(t)

	// InputTokens + CacheReadTokens = 1000 + 100000 = 101000 < 272000，落在基础段
	tokens := UsageTokens{
		InputTokens:         1000,
		CacheReadTokens:     100000,
		CacheCreationTokens: 10000,
		OutputTokens:        1000,
	}

	cost := costViaCatalog(t, bs, resolver, "gpt-5.4", tokens)

	expectedCacheCreation := float64(tokens.CacheCreationTokens) * 2.5e-6
	require.InDelta(t, expectedCacheCreation, cost.CacheCreationCost, 1e-10,
		"cache_creation_cost should remain at base price below the segment")
}

// 覆盖 5m / 1h ephemeral 分类计费路径：阶梯换算成分段时两档缓存写价都乘输入倍数，落在高段时各按高段价。
func TestCalculateCost_TokenSegmentAppliesToCacheCreation5mAnd1h(t *testing.T) {
	price := func(v float64) *float64 { return &v }
	entry := ModelCatalogEntry{
		ModelID:           "claude-sonnet-4",
		BillingMode:       BillingModeToken,
		InputPrice:        price(3e-6),
		OutputPrice:       price(15e-6),
		CacheReadPrice:    price(0.3e-6),
		CacheWritePrice:   price(4e-6),
		CacheWrite1hPrice: price(5e-6),
	}
	tokenLadder{threshold: 272000, inputMultiplier: 2.0, outputMultiplier: 1.5}.applyTo(&entry)
	require.Len(t, entry.Intervals, 1)
	bs := newTestBillingService()
	resolver := newResolverWithSeededEntries(bs, entry)

	// InputTokens + CacheReadTokens + CacheCreationTokens = 1000 + 300000 + 12000 > 272000
	tokens := UsageTokens{
		InputTokens:           1000,
		CacheReadTokens:       300000,
		CacheCreationTokens:   12000,
		CacheCreation5mTokens: 8000,
		CacheCreation1hTokens: 4000,
		OutputTokens:          1000,
	}

	cost := costViaCatalog(t, bs, resolver, "claude-sonnet-4", tokens)

	expected5m := float64(tokens.CacheCreation5mTokens) * 4e-6 * 2.0
	expected1h := float64(tokens.CacheCreation1hTokens) * 5e-6 * 2.0
	require.InDelta(t, expected5m+expected1h, cost.CacheCreationCost, 1e-10,
		"both 5m and 1h cache_creation prices should use the upper segment price")
}

// 豆包向量模型官网只有人民币价，按 1 美元 = 6.8 元换算（muqian 2026-10-06），每百万 Token 4 位小数（10-07）。
const (
	doubaoEmbeddingTextRate  = 0.1029e-6 // ¥0.7/MTok
	doubaoEmbeddingImageRate = 0.2647e-6 // ¥1.8/MTok
)

// doubao-embedding-vision 是首个图文不同价的 embedding：文本 ¥0.7/MTok、图片 ¥1.8/MTok。
// 验证内置价表同时携带文本与图片两档单价（播种进目录后按目录计费）。
func TestGetModelPricing_DoubaoEmbeddingVisionImageInputRate(t *testing.T) {
	svc := newTestBillingService()

	for _, model := range []string{"doubao-embedding-vision-251215"} {
		pricing, err := builtinPricing(svc, model)
		require.NoError(t, err, "model %s should resolve fallback pricing", model)
		require.NotNil(t, pricing)
		require.InDelta(t, doubaoEmbeddingTextRate, pricing.InputPricePerToken, 1e-12, "text input rate for %s", model)
		require.InDelta(t, doubaoEmbeddingImageRate, pricing.ImageInputPricePerToken, 1e-12, "image input rate for %s", model)
		require.Zero(t, pricing.OutputPricePerToken, "embedding has no output cost for %s", model)
	}
}

// 验证双档计费：InputCost = 文本token×文本价（不含图片），ImageInputCost = 图片token×图片价；
// 且 ImageInputTokens=0 时走原单价路径，ImageInputTokens>InputTokens 时不负计文本。
func TestCalculateCost_DoubaoEmbeddingVisionDifferentialInput(t *testing.T) {
	svc := newTestBillingService()

	// 图文混合：prompt_tokens=1340，其中 image_tokens=28、text_tokens=1312。
	mixed := UsageTokens{InputTokens: 1340, ImageInputTokens: 28}
	cost, err := builtinCatalogCost(svc, "doubao-embedding-vision-251215", mixed, 1.0)
	require.NoError(t, err)
	wantText := float64(1312) * doubaoEmbeddingTextRate
	wantImage := float64(28) * doubaoEmbeddingImageRate
	require.InDelta(t, wantText, cost.InputCost, 1e-15, "InputCost 仅计文本输入")
	require.InDelta(t, wantImage, cost.ImageInputCost, 1e-15, "ImageInputCost 单独计图片输入")
	require.InDelta(t, wantText+wantImage, cost.TotalCost, 1e-15, "TotalCost 口径不变")
	require.Zero(t, cost.OutputCost)

	// 纯文本：全部按文本档计费，与原单价路径一致，无图片输入费用。
	textOnly := UsageTokens{InputTokens: 1340}
	costText, err := builtinCatalogCost(svc, "doubao-embedding-vision-251215", textOnly, 1.0)
	require.NoError(t, err)
	require.InDelta(t, float64(1340)*doubaoEmbeddingTextRate, costText.InputCost, 1e-15)
	require.Zero(t, costText.ImageInputCost)

	// 健壮性：ImageInputTokens 超过 InputTokens 时，文本置 0、计费 token 不超过 InputTokens。
	weird := UsageTokens{InputTokens: 10, ImageInputTokens: 50}
	costWeird, err := builtinCatalogCost(svc, "doubao-embedding-vision-251215", weird, 1.0)
	require.NoError(t, err)
	require.Zero(t, costWeird.InputCost, "全为图片输入时文本费用为 0")
	require.InDelta(t, float64(10)*doubaoEmbeddingImageRate, costWeird.ImageInputCost, 1e-15)
	require.InDelta(t, float64(10)*doubaoEmbeddingImageRate, costWeird.TotalCost, 1e-15)
}

// 复现 issue #4386：gpt-image-2 /v1/images/edits 带 1 张输入图。
// 上游 usage：input_tokens=371（image_tokens=352 + text_tokens=19），
// output_tokens=439（全部图片输出）。官方定价：文本输入 $5/1M、图片输入 $8/1M、
// 文本输出 $10/1M、图片输出 $30/1M。修复前图片输入被并入文本价，单次偏低 ~6.6%。
func TestComputeTokenBreakdown_GptImage2ImageEditIssue4386(t *testing.T) {
	svc := newTestBillingService()

	pricing := &ModelPricing{
		InputPricePerToken:       5e-6,
		ImageInputPricePerToken:  8e-6,
		OutputPricePerToken:      10e-6,
		ImageOutputPricePerToken: 30e-6,
		ImageOutputPriceExplicit: true,
	}
	tokens := UsageTokens{
		InputTokens:       371,
		ImageInputTokens:  352,
		OutputTokens:      439,
		ImageOutputTokens: 439,
	}

	cost := svc.computeTokenBreakdown(pricing, tokens, 1.0)

	wantTextInput := float64(19) * 5e-6     // 0.000095
	wantImageInput := float64(352) * 8e-6   // 0.002816
	wantImageOutput := float64(439) * 30e-6 // 0.013170
	require.InDelta(t, wantTextInput, cost.InputCost, 1e-15, "InputCost 仅含文本输入")
	require.InDelta(t, wantImageInput, cost.ImageInputCost, 1e-15, "图片输入按 $8/1M 独立计费")
	require.Zero(t, cost.OutputCost, "输出全部为图片，文本输出费用为 0")
	require.InDelta(t, wantImageOutput, cost.ImageOutputCost, 1e-15)
	require.InDelta(t, 0.016081, cost.TotalCost, 1e-9, "总额应为 $0.016081（修复前为 $0.015025）")
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	svc := newTestBillingService()

	cost, err := builtinCatalogCost(svc, "claude-sonnet-4", UsageTokens{}, 1.0)
	require.NoError(t, err)
	require.Equal(t, 0.0, cost.TotalCost)
	require.Equal(t, 0.0, cost.ActualCost)
}

func TestGetModelPricing_Grok45OfficialFallback(t *testing.T) {
	svc := newTestBillingService()

	for _, model := range []string{"grok-4.5"} {
		model := model
		t.Run(model, func(t *testing.T) {
			pricing, err := builtinPricing(svc, model)
			require.NoError(t, err)
			require.InDelta(t, 2e-6, pricing.InputPricePerToken, 1e-12)
			require.InDelta(t, 6e-6, pricing.OutputPricePerToken, 1e-12)
			require.InDelta(t, 0.3e-6, pricing.CacheReadPricePerToken, 1e-12)
			require.False(t, pricing.SupportsCacheBreakdown)
		})
	}
}

func TestGetModelPricing_Grok46OfficialFallback(t *testing.T) {
	svc := newTestBillingService()

	for _, model := range []string{"grok-4.6"} {
		model := model
		t.Run(model, func(t *testing.T) {
			pricing, err := builtinPricing(svc, model)
			require.NoError(t, err)
			require.InDelta(t, 2e-6, pricing.InputPricePerToken, 1e-12)
			require.InDelta(t, 6e-6, pricing.OutputPricePerToken, 1e-12)
			require.InDelta(t, 0.5e-6, pricing.CacheReadPricePerToken, 1e-12)
			require.False(t, pricing.SupportsCacheBreakdown)
		})
	}
}

func TestGetModelPricing_GrokOfficialFamilyCards(t *testing.T) {
	svc := newTestBillingService()
	for _, tc := range []struct {
		model                 string
		input, cached, output float64
	}{
		{"grok-4.3", 1.25e-6, 0.2e-6, 2.5e-6},
		{"grok-4.20", 1.25e-6, 0.2e-6, 2.5e-6},
		{"grok-build-0.1", 1e-6, 0.2e-6, 2e-6},
	} {
		p, err := builtinPricing(svc, tc.model)
		require.NoError(t, err, tc.model)
		require.InDelta(t, tc.input, p.InputPricePerToken, 1e-12)
		require.InDelta(t, tc.cached, p.CacheReadPricePerToken, 1e-12)
		require.InDelta(t, tc.output, p.OutputPricePerToken, 1e-12)
	}
}

// 兜底价表的 grok 阶梯（200K 起输入 / 输出 ×2，达到即进高段）播种成分段后走目录计费。
func TestCalculateCostUnified_GrokFallbackLadderSeedsInclusiveSegment(t *testing.T) {
	svc := newTestBillingService()
	entry := seedEntryFromFallback("grok-4.5", svc.SnapshotFallbackPricing()["grok-4.5"])
	resolver := newResolverWithSeededEntries(svc, entry)
	calc := func(input int) *CostBreakdown {
		return costViaCatalog(t, svc, resolver, "grok-4.5", UsageTokens{InputTokens: input, OutputTokens: 1000})
	}

	below := calc(199_999)
	atThreshold := calc(200_000)
	above := calc(250_000)
	require.InDelta(t, below.InputCost/199_999*2, atThreshold.InputCost/200_000, 1e-15, "达到 200K 即进高段")
	require.InDelta(t, below.InputCost/199_999*2, above.InputCost/250_000, 1e-15)
	require.InDelta(t, below.OutputCost*2, above.OutputCost, 1e-12)
}

func TestCalculateCost_SupportsCacheBreakdown(t *testing.T) {
	svc := &BillingService{
		fallbackPrices: map[string]*ModelPricing{
			"claude-sonnet-4": {
				InputPricePerToken:         3e-6,
				OutputPricePerToken:        15e-6,
				CacheCreationPricePerToken: 4e-6,
				SupportsCacheBreakdown:     true,
				CacheCreation5mPrice:       4e-6, // per token
				CacheCreation1hPrice:       5e-6, // per token
			},
		},
	}

	tokens := UsageTokens{
		InputTokens:           1000,
		OutputTokens:          500,
		CacheCreation5mTokens: 100000,
		CacheCreation1hTokens: 50000,
	}
	cost, err := builtinCatalogCost(svc, "claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	expected5m := float64(tokens.CacheCreation5mTokens) * 4e-6
	expected1h := float64(tokens.CacheCreation1hTokens) * 5e-6
	require.InDelta(t, expected5m+expected1h, cost.CacheCreationCost, 1e-10)
}

func TestComputeCacheCreationCost_CapsContradictoryBreakdownAtAggregate(t *testing.T) {
	svc := &BillingService{}
	pricing := &ModelPricing{
		SupportsCacheBreakdown: true,
		CacheCreation5mPrice:   1,
		CacheCreation1hPrice:   1,
	}

	tokens := UsageTokens{
		CacheCreationTokens:   463184,
		CacheCreation5mTokens: 463184,
		CacheCreation1hTokens: 463184,
	}

	cost := svc.computeCacheCreationCost(pricing, tokens, 0)
	require.Equal(t, float64(tokens.CacheCreationTokens), cost,
		"billed cache-creation token equivalent must not exceed the positive aggregate")
}

func TestNormalizeCacheCreationBreakdown_BillingSafetyInvariant(t *testing.T) {
	tests := []struct {
		name   string
		tokens UsageTokens
		want5m int
		want1h int
	}{
		{
			name:   "preserves ratio when capping",
			tokens: UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: 90, CacheCreation1hTokens: 60},
			want5m: 60,
			want1h: 40,
		},
		{
			name:   "details below aggregate unchanged",
			tokens: UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: 30, CacheCreation1hTokens: 60},
			want5m: 30,
			want1h: 60,
		},
		{
			name:   "absent 5m detail unchanged",
			tokens: UsageTokens{CacheCreationTokens: 100, CacheCreation1hTokens: 60},
			want5m: 0,
			want1h: 60,
		},
		{
			name:   "absent 1h detail unchanged",
			tokens: UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: 30},
			want5m: 30,
			want1h: 0,
		},
		{
			name:   "negative detail clamped",
			tokens: UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: -50, CacheCreation1hTokens: 60},
			want5m: 0,
			want1h: 60,
		},
		{
			name:   "negative detail cannot hide oversized positive detail",
			tokens: UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: -50, CacheCreation1hTokens: 150},
			want5m: 0,
			want1h: 100,
		},
		{
			name:   "integer boundary details capped without overflow",
			tokens: UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: int(^uint(0) >> 1), CacheCreation1hTokens: int(^uint(0) >> 1)},
			want5m: 50,
			want1h: 50,
		},
		{
			name:   "integer boundary aggregate avoids float conversion overflow",
			tokens: UsageTokens{CacheCreationTokens: int(^uint(0) >> 1), CacheCreation5mTokens: int(^uint(0) >> 1), CacheCreation1hTokens: 1},
			want5m: int(^uint(0) >> 1),
			want1h: 0,
		},
		{
			name:   "zero aggregate unchanged",
			tokens: UsageTokens{CacheCreation5mTokens: 90, CacheCreation1hTokens: 60},
			want5m: 90,
			want1h: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got5m, got1h := normalizeCacheCreationBreakdown(tt.tokens)
			require.Equal(t, tt.want5m, got5m)
			require.Equal(t, tt.want1h, got1h)
		})
	}
}

func TestComputeCacheCreationCost_PreservesZeroDetailFallback(t *testing.T) {
	svc := &BillingService{}
	pricing := &ModelPricing{
		SupportsCacheBreakdown: true,
		CacheCreation5mPrice:   4e-6,
		CacheCreation1hPrice:   5e-6,
	}

	tests := []struct {
		name   string
		tokens UsageTokens
	}{
		{name: "zero details", tokens: UsageTokens{CacheCreationTokens: 100}},
		{name: "one negative detail", tokens: UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: -25}},
		{name: "both negative details", tokens: UsageTokens{CacheCreationTokens: 100, CacheCreation5mTokens: -25, CacheCreation1hTokens: -75}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := svc.computeCacheCreationCost(pricing, tt.tokens, 0)
			require.InDelta(t, 100*4e-6, cost, 1e-12)
		})
	}
}

func TestCalculateCost_LargeTokenCount(t *testing.T) {
	svc := newTestBillingService()

	tokens := UsageTokens{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	}
	cost, err := builtinCatalogCost(svc, "claude-sonnet-4", tokens, 1.0)
	require.NoError(t, err)

	// Input: 1M * 3e-6 = $3, Output: 1M * 15e-6 = $15
	require.InDelta(t, 3.0, cost.InputCost, 1e-6)
	require.InDelta(t, 15.0, cost.OutputCost, 1e-6)
	require.False(t, math.IsNaN(cost.TotalCost))
	require.False(t, math.IsInf(cost.TotalCost, 0))
}

// 价格文件的各项价播种进目录、再投影成计费价卡（计费只认目录）。
func TestGetModelPricing_MapsDynamicFieldsIntoBillingPricing(t *testing.T) {
	entry := seedEntryFromLiteLLM("dynamic-tier-model", &LiteLLMModelPricing{
		InputCostPerToken:                   1e-6,
		OutputCostPerToken:                  3e-6,
		CacheCreationInputTokenCost:         4e-6,
		CacheCreationInputTokenCostAbove1hr: 5e-6,
		CacheReadInputTokenCost:             7e-7,
	})
	pricing := &ModelPricing{}
	entry.ApplyToModelPricing(pricing)
	require.InDelta(t, 1e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 3e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 4e-6, pricing.CacheCreation5mPrice, 1e-12)
	require.InDelta(t, 5e-6, pricing.CacheCreation1hPrice, 1e-12)
	require.True(t, pricing.SupportsCacheBreakdown)
	require.InDelta(t, 7e-7, pricing.CacheReadPricePerToken, 1e-12)
}

// ---------------------------------------------------------------------------
// GetModelPricingWithChannel
// ---------------------------------------------------------------------------

func TestGetModelPricing_Fable51FallbackPricing(t *testing.T) {
	svc := newTestBillingService()

	pricing, err := builtinPricing(svc, "claude-fable-5-1")
	require.NoError(t, err)
	require.InDelta(t, 10e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 50e-6, pricing.OutputPricePerToken, 1e-12)
	require.InDelta(t, 12.5e-6, pricing.CacheCreation5mPrice, 1e-12)
	require.InDelta(t, 20e-6, pricing.CacheCreation1hPrice, 1e-12)
	require.InDelta(t, 0.25e-6, pricing.CacheReadPricePerToken, 1e-12)
	require.NotNil(t, pricing.MaxReasoningEffortMultiplier)
	require.Equal(t, 3.0, *pricing.MaxReasoningEffortMultiplier)
}

func TestComputeTokenBreakdown_ExplicitZeroImagePrice_NoFallback(t *testing.T) {
	svc := newTestBillingService()

	pricing := &ModelPricing{
		InputPricePerToken:       3e-6,
		OutputPricePerToken:      15e-6,
		ImageOutputPricePerToken: 0,
		ImageOutputPriceExplicit: true,
	}
	tokens := UsageTokens{
		InputTokens:       100,
		OutputTokens:      200,
		ImageOutputTokens: 50,
	}
	bd := svc.computeTokenBreakdown(pricing, tokens, 1.0)

	// ImageOutputTokens should NOT fall back to outputPrice
	require.Equal(t, 0.0, bd.ImageOutputCost)
	// textOutputTokens = 200 - 50 = 150
	require.InDelta(t, 150*15e-6, bd.OutputCost, 1e-12)
}

func TestComputeTokenBreakdown_NonExplicitZeroImagePrice_FallsBackToOutput(t *testing.T) {
	svc := newTestBillingService()

	pricing := &ModelPricing{
		InputPricePerToken:       3e-6,
		OutputPricePerToken:      15e-6,
		ImageOutputPricePerToken: 0,
		ImageOutputPriceExplicit: false,
	}
	tokens := UsageTokens{
		InputTokens:       100,
		OutputTokens:      200,
		ImageOutputTokens: 50,
	}
	bd := svc.computeTokenBreakdown(pricing, tokens, 1.0)

	// Should fall back to outputPrice since not explicit
	require.InDelta(t, 50*15e-6, bd.ImageOutputCost, 1e-12)
	// textOutputTokens = 200 - 50 = 150
	require.InDelta(t, 150*15e-6, bd.OutputCost, 1e-12)
}
