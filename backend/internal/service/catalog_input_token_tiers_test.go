//go:build unit

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// 人民币价按 1 美元 = 6.8 元换算（muqian 2026-10-06：有美元官网价用美元，只有人民币价的按 6.8）。
func cnyPerMillion(yuan float64) float64 { return yuan / 6.8 / 1e6 }

func usdPerMillion(usd float64) float64 { return usd / 1e6 }

// 价格文件存 6 位有效数字：按相对误差比。
func requirePrice(t *testing.T, want float64, got *float64, msgAndArgs ...any) {
	t.Helper()
	require.NotNil(t, got, msgAndArgs...)
	require.InEpsilon(t, want, *got, 1e-5, msgAndArgs...)
}

func readBuiltinPricingFile(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	return data
}

func seededBuiltinCatalog(t *testing.T) map[string]ModelCatalogEntry {
	t.Helper()
	pricingData, err := (&PricingService{}).parsePricingData(readBuiltinPricingFile(t))
	require.NoError(t, err)
	pricingSvc := &PricingService{pricingData: pricingData}
	entries := buildModelCatalogSeedEntries(ModelCatalogSeedInput{
		PricingService: pricingSvc,
		BillingService: NewBillingService(),
	})
	byID := make(map[string]ModelCatalogEntry, len(entries))
	for _, entry := range entries {
		byID[entry.ModelID] = entry
	}
	return byID
}

// 价格跟官网对齐，包括阶梯价（muqian 2026-10-06）：官网逐段写明的价播成目录的按 token 分段，每段都是绝对价。
func TestCatalogSeedInputTokenTiersFollowOfficialPrices(t *testing.T) {
	byID := seededBuiltinCatalog(t)

	// 豆包角色扮演：输入 (32K, 128K] 输入 1.2 元、输出 6 元，缓存命中仍是 0.16 元（不随段涨）
	character := byID["doubao-seed-character-260628"]
	requirePrice(t, cnyPerMillion(0.8), character.InputPrice)
	require.Len(t, character.Intervals, 1)
	require.Equal(t, 32000, character.Intervals[0].MinTokens)
	require.Nil(t, character.Intervals[0].MaxTokens, "最后一段不封顶")
	requirePrice(t, cnyPerMillion(1.2), character.Intervals[0].InputPrice)
	requirePrice(t, cnyPerMillion(6.0), character.Intervals[0].OutputPrice)
	requirePrice(t, cnyPerMillion(0.16), character.Intervals[0].CacheReadPrice)

	// 豆包 2.0 lite：三段 32K / 128K / 256K；音频输入基础价 9 元
	lite := byID["doubao-seed-2-0-lite-260428"]
	requirePrice(t, cnyPerMillion(0.6), lite.InputPrice)
	requirePrice(t, cnyPerMillion(9.0), lite.AudioInputPrice)
	require.Len(t, lite.Intervals, 2)
	require.Equal(t, 32000, lite.Intervals[0].MinTokens)
	require.Equal(t, 128000, *lite.Intervals[0].MaxTokens)
	requirePrice(t, cnyPerMillion(0.9), lite.Intervals[0].InputPrice)
	requirePrice(t, cnyPerMillion(5.4), lite.Intervals[0].OutputPrice)
	requirePrice(t, cnyPerMillion(0.18), lite.Intervals[0].CacheReadPrice)
	require.Equal(t, 128000, lite.Intervals[1].MinTokens)
	requirePrice(t, cnyPerMillion(1.8), lite.Intervals[1].InputPrice)
	requirePrice(t, cnyPerMillion(10.8), lite.Intervals[1].OutputPrice)
	_, deprecated := byID["doubao-seed-2-0-lite-260215"]
	require.False(t, deprecated, "260215 官网标了即将下线，不收")

	// 豆包 2.1 改按 6.8 换算
	requirePrice(t, cnyPerMillion(6.0), byID["doubao-seed-2-1-pro-260915"].InputPrice)
	requirePrice(t, cnyPerMillion(30.0), byID["doubao-seed-2-1-pro-260915"].OutputPrice)

	// 通义（新加坡价）：qwen3.7-flash 三段，带缓存折扣的缓存命中按每段输入价 20%
	flash := byID["qwen3.7-flash"]
	requirePrice(t, usdPerMillion(0.03), flash.InputPrice)
	requirePrice(t, usdPerMillion(0.006), flash.CacheReadPrice)
	require.Len(t, flash.Intervals, 2)
	require.Equal(t, 32000, flash.Intervals[0].MinTokens)
	require.Equal(t, 256000, *flash.Intervals[0].MaxTokens)
	requirePrice(t, usdPerMillion(0.1), flash.Intervals[0].InputPrice)
	requirePrice(t, usdPerMillion(0.02), flash.Intervals[0].CacheReadPrice)
	requirePrice(t, usdPerMillion(0.8), flash.Intervals[1].OutputPrice)
	// 快照没有缓存折扣
	require.Nil(t, byID["qwen3.7-flash-2026-07-15"].CacheReadPrice)
	// qwen3.7-plus 官网限时 8 折，存现价；快照原价
	requirePrice(t, usdPerMillion(0.32), byID["qwen3.7-plus"].InputPrice)
	requirePrice(t, usdPerMillion(3.84), byID["qwen3.7-plus"].Intervals[0].OutputPrice)
	requirePrice(t, usdPerMillion(0.4), byID["qwen3.7-plus-2026-05-26"].InputPrice)
	coder := byID["qwen3-coder-flash"]
	require.Len(t, coder.Intervals, 3)
	requirePrice(t, usdPerMillion(9.6), coder.Intervals[2].OutputPrice)
	// qwen-plus 系列开思考另价，目录分不出来：先不收（muqian 2026-10-06）
	for _, model := range []string{"qwen-plus", "qwen-plus-2025-12-01", "qwen-plus-2025-09-11", "qwen-plus-2025-07-28"} {
		_, ok := byID[model]
		require.False(t, ok, model)
	}

	// MiniMax：目录 ID 用官网写法；M3 超过 512K 加价；缓存写入 0.375
	m3, ok := byID["MiniMax-M3"]
	require.True(t, ok, "目录 ID 用官网写法 MiniMax-M3")
	_, lower := byID["minimax-m3"]
	require.False(t, lower)
	require.Equal(t, "minimax", m3.Vendor)
	require.Len(t, m3.Intervals, 1)
	require.Equal(t, 512000, m3.Intervals[0].MinTokens)
	requirePrice(t, usdPerMillion(0.6), m3.Intervals[0].InputPrice)
	requirePrice(t, usdPerMillion(2.4), m3.Intervals[0].OutputPrice)
	requirePrice(t, usdPerMillion(0.12), m3.Intervals[0].CacheReadPrice)
	for _, model := range []string{"MiniMax-M2.7", "MiniMax-M2.7-highspeed", "MiniMax-M2.5", "MiniMax-M2.5-highspeed", "MiniMax-M2.1", "MiniMax-M2.1-highspeed", "MiniMax-M2"} {
		entry, ok := byID[model]
		require.True(t, ok, model)
		requirePrice(t, usdPerMillion(0.375), entry.CacheWritePrice, model)
	}

	// 零散修正：gpt-image-2 文字没有输出价；gpt-4o-mini-tts 输出 $12；gemini-embedding-2 图片 / 音频输入价
	require.Nil(t, byID["gpt-image-2"].OutputPrice)
	require.Nil(t, byID["gpt-image-2-2026-04-21"].OutputPrice)
	requirePrice(t, usdPerMillion(12), byID["gpt-4o-mini-tts"].OutputPrice)
	requirePrice(t, usdPerMillion(0.45), byID["gemini-embedding-2"].ImageInputPrice)
	requirePrice(t, usdPerMillion(6.5), byID["gemini-embedding-2"].AudioInputPrice)
}

// 输入落在高段时整条按高段计：文本、缓存命中、输出用这一段写明的价，音频输入按本段输入价的比例加价。
func TestCalculateTokenCost_InputTokenTierAppliesToWholeRequest(t *testing.T) {
	ps := newStubPricingServiceFromJSON(t, string(readBuiltinPricingFile(t)))
	bs := NewBillingService()
	resolver := newResolverWithSeededEntries(bs, seededLiteLLMEntry(t, ps, "doubao-seed-2-0-lite-260428"))

	// 输入侧 100K + 缓存命中 10K = 110K，落在 (32K, 128K]
	high := costViaCatalog(t, bs, resolver, "doubao-seed-2-0-lite-260428", UsageTokens{
		InputTokens: 100_000, AudioInputTokens: 1_000, CacheReadTokens: 10_000, OutputTokens: 1_000,
	})
	require.InEpsilon(t, 99_000*cnyPerMillion(0.9)+1_000*cnyPerMillion(13.5), high.InputCost, 1e-5)
	require.InEpsilon(t, 1_000*cnyPerMillion(13.5), high.AudioInputCost, 1e-5, "音频 9 元 × 1.5 = 13.5 元")
	require.InEpsilon(t, 10_000*cnyPerMillion(0.18), high.CacheReadCost, 1e-5)
	require.InEpsilon(t, 1_000*cnyPerMillion(5.4), high.OutputCost, 1e-5)

	// 20K 落在第一段：基础价
	low := costViaCatalog(t, bs, resolver, "doubao-seed-2-0-lite-260428", UsageTokens{
		InputTokens: 20_000, AudioInputTokens: 1_000, OutputTokens: 1_000,
	})
	require.InEpsilon(t, 1_000*cnyPerMillion(9.0), low.AudioInputCost, 1e-5)
	require.InEpsilon(t, 1_000*cnyPerMillion(3.6), low.OutputCost, 1e-5)
}

// model_id 与 input_token_tiers 是我们自己加的字段：写错的条目整条不收（fail-closed），不按基础价放行。
func TestParsePricingDataRejectsInvalidTiersAndModelID(t *testing.T) {
	body := `{
	  "good": {"litellm_provider": "dashscope", "mode": "chat", "input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6,
	    "input_token_tiers": [{"max_input_tokens": 32000, "input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6},
	                          {"input_cost_per_token": 3e-6, "output_cost_per_token": 6e-6}]},
	  "minimax-m9": {"model_id": "MiniMax-M9", "litellm_provider": "minimax", "mode": "chat", "input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6},
	  "single-tier": {"input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6,
	    "input_token_tiers": [{"input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6}]},
	  "missing-middle-max": {"input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6,
	    "input_token_tiers": [{"input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6},
	                          {"input_cost_per_token": 3e-6, "output_cost_per_token": 6e-6}]},
	  "not-increasing": {"input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6,
	    "input_token_tiers": [{"max_input_tokens": 32000, "input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6},
	                          {"max_input_tokens": 32000, "input_cost_per_token": 3e-6, "output_cost_per_token": 6e-6}]},
	  "first-not-base": {"input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6,
	    "input_token_tiers": [{"max_input_tokens": 32000, "input_cost_per_token": 2e-6, "output_cost_per_token": 2e-6},
	                          {"input_cost_per_token": 3e-6, "output_cost_per_token": 6e-6}]},
	  "missing-output": {"input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6,
	    "input_token_tiers": [{"max_input_tokens": 32000, "input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6},
	                          {"input_cost_per_token": 3e-6}]},
	  "with-ladder": {"input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6,
	    "long_context_input_token_threshold": 200000, "long_context_input_cost_multiplier": 2,
	    "input_token_tiers": [{"max_input_tokens": 32000, "input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6},
	                          {"input_cost_per_token": 3e-6, "output_cost_per_token": 6e-6}]},
	  "id-mismatch": {"model_id": "Other-Model", "input_cost_per_token": 1e-6, "output_cost_per_token": 2e-6}
	}`
	data, err := (&PricingService{}).parsePricingData([]byte(body))
	require.NoError(t, err)

	good := data["good"]
	require.NotNil(t, good)
	require.Len(t, good.InputTokenTiers, 2)
	require.Equal(t, 32000, good.InputTokenTiers[0].MaxInputTokens)
	require.Zero(t, good.InputTokenTiers[1].MaxInputTokens, "最后一段不写上限 = 不封顶")
	require.Equal(t, "MiniMax-M9", data["minimax-m9"].ModelID, "键小写用来查价，model_id 保留官网写法")

	for _, invalid := range []string{"single-tier", "missing-middle-max", "not-increasing", "first-not-base", "missing-output", "with-ladder", "id-mismatch"} {
		require.NotContains(t, data, invalid, invalid)
	}
}
