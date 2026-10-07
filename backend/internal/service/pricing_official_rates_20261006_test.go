//go:build unit

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// 2026-10-06 按各家官网补全目录（muqian 定：只补对话模型、有官方价的；价格与模型 ID 以官网为准；
// 正在打折的存官网现价）。这里锁住有代表性的条目，改价格文件时这些值要跟着官网一起改。
func TestPricingFileOfficialRates20261006(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	pricingData, err := (&PricingService{}).parsePricingData(data)
	require.NoError(t, err)

	type rates struct {
		input, output, cacheRead, cacheWrite, cacheWrite1h float64
		threshold                                          int
		inMul, outMul                                      float64
	}
	for model, want := range map[string]rates{
		// 新补
		"claude-opus-5-5": {input: 4e-6, output: 20e-6, cacheRead: 0.2e-6, cacheWrite: 5e-6, cacheWrite1h: 8e-6},
		"claude-sonnet-5": {input: 2e-6, output: 10e-6, cacheRead: 0.2e-6, cacheWrite: 2.5e-6, cacheWrite1h: 4e-6},
		"gpt-6-sol":       {input: 2e-6, output: 10e-6, cacheRead: 0.2e-6, cacheWrite: 2.5e-6, threshold: 272000, inMul: 2, outMul: 1.5},
		"grok-4.7":        {input: 2e-6, output: 6e-6, cacheRead: 0.5e-6, threshold: 200000, inMul: 2, outMul: 2},
		// 国内模型按国内站人民币价 ÷ 6.8（muqian 2026-10-07）；通义缓存价按国内站单模型页：
		// 有隐式缓存的命中按隐式价，只有显式缓存的按显式读价；缓存写 = 显式创建价
		"kimi-k2.7-code": {input: 6.5 / 6.8 * 1e-6, output: 27 / 6.8 * 1e-6, cacheRead: 1.3 / 6.8 * 1e-6},
		"qwen3.5-flash":  {input: 0.2 / 6.8 * 1e-6, output: 2 / 6.8 * 1e-6, cacheRead: 0.02 / 6.8 * 1e-6, cacheWrite: 0.25 / 6.8 * 1e-6}, // 只有显式缓存
		"qwen3.8-max":    {input: 12 / 6.8 * 1e-6, output: 36 / 6.8 * 1e-6, cacheRead: 1.5 / 6.8 * 1e-6, cacheWrite: 15 / 6.8 * 1e-6},    // 隐式 ¥1.5、显式创建 ¥15
		// 修正
		"gpt-4o-mini-tts":            {input: 0.6e-6, output: 12e-6}, // 官网只有音频输出价 $12
		"gpt-realtime-2":             {input: 4e-6, output: 24e-6, cacheRead: 0.4e-6},
		"gemini-2.5-pro-preview-tts": {input: 1e-6, output: 20e-6},
		// 优惠价（存官网现价）
		"gpt-5.6-sol":      {input: 4e-6, output: 20e-6, cacheRead: 0.4e-6, cacheWrite: 5e-6, threshold: 272000, inMul: 2, outMul: 1.5},
		"gemini-3.6-flash": {input: 0.75e-6, output: 3.75e-6, cacheRead: 0.075e-6},
	} {
		t.Run(model, func(t *testing.T) {
			got, ok := pricingData[model]
			require.True(t, ok, "价格文件里要有 %s", model)
			// 价格文件存 6 位有效数字（人民币换算的价不是整数美分）
			require.InDelta(t, want.input, got.InputCostPerToken, want.input*1e-5+1e-15)
			require.InDelta(t, want.output, got.OutputCostPerToken, want.output*1e-5+1e-15)
			require.InDelta(t, want.cacheRead, got.CacheReadInputTokenCost, want.cacheRead*1e-5+1e-15)
			require.InDelta(t, want.cacheWrite, got.CacheCreationInputTokenCost, want.cacheWrite*1e-5+1e-15)
			require.InDelta(t, want.cacheWrite1h, got.CacheCreationInputTokenCostAbove1hr, want.cacheWrite1h*1e-5+1e-15)
			require.Equal(t, want.threshold, got.LongContextInputTokenThreshold)
			require.InDelta(t, want.inMul, got.LongContextInputCostMultiplier, 1e-12)
			require.InDelta(t, want.outMul, got.LongContextOutputCostMultiplier, 1e-12)
		})
	}

	// 停服 / 第三方托管的不能被这次补进来（DeepSeek 停服名单见 TestDeepseekPricingFileMatchesOfficialRates）
	for _, absent := range []string{"deepseek-chat", "deepseek-reasoner", "zai-glm-5-3", "kimi-k2.5", "moonshot-v1-8k", "qwen-mt-plus",
		"gpt-5.3-codex-spark", // OpenAI 2026-09-14 退役、没有 API 价（muqian 10-06「删」）
		// 目录只收 11 家（muqian 10-06）：Mistral / Cohere / AI21 / Bedrock 写法都不收
		"mistral-large-latest", "jamba-large", "command-r-08-2024", "claude-sonnet-4-5-20250929-v1:0",
	} {
		_, ok := pricingData[absent]
		require.False(t, ok, "%s 不该出现在价格文件里", absent)
	}
}

// 新补的条目都要能播进目录（不被当成无效条目跳过）。
func TestPricingFileOfficialAdditionsSeedIntoCatalog(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	pricingData, err := (&PricingService{}).parsePricingData(data)
	require.NoError(t, err)
	entries := buildModelCatalogSeedEntries(ModelCatalogSeedInput{PricingService: &PricingService{pricingData: pricingData}})
	byID := make(map[string]ModelCatalogEntry, len(entries))
	for _, entry := range entries {
		entry.Normalize()
		require.NoError(t, entry.Validate(), entry.ModelID)
		byID[entry.ModelID] = entry
	}
	for _, model := range []string{"claude-opus-5-5", "gpt-6-astra", "grok-4.20-0309-reasoning", "glm-4.6v", "MiniMax-M2.5-highspeed", "qwen3.8-flash"} {
		entry, ok := byID[model]
		require.True(t, ok, model)
		require.NotNil(t, entry.InputPrice, model)
		require.NotNil(t, entry.OutputPrice, model)
	}
	require.Equal(t, "zhipu", byID["glm-4.6v"].Vendor, "智谱沿用目录里的 zhipu")
	require.NotEmpty(t, byID["grok-4.7"].Intervals, "xAI 长上下文换算成按 token 分段")
}
