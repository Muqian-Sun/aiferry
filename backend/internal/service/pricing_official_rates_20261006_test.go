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
		"claude-opus-5-5":      {input: 4e-6, output: 20e-6, cacheRead: 0.2e-6, cacheWrite: 5e-6, cacheWrite1h: 8e-6},
		"claude-sonnet-5":      {input: 2e-6, output: 10e-6, cacheRead: 0.2e-6, cacheWrite: 2.5e-6, cacheWrite1h: 4e-6},
		"gpt-6-sol":            {input: 2e-6, output: 10e-6, cacheRead: 0.2e-6, cacheWrite: 2.5e-6, threshold: 272000, inMul: 2, outMul: 1.5},
		"grok-4.7":             {input: 2e-6, output: 6e-6, cacheRead: 0.5e-6, threshold: 200000, inMul: 2, outMul: 2},
		"mistral-large-latest": {input: 0.5e-6, output: 1.5e-6, cacheRead: 0.05e-6},
		"kimi-k2.7-code":       {input: 0.95e-6, output: 4e-6, cacheRead: 0.19e-6},
		"qwen3.5-flash":        {input: 0.1e-6, output: 0.4e-6, cacheRead: 0.02e-6}, // 隐式缓存命中 = 输入价 20%
		"qwen3.8-max":          {input: 2e-6, output: 6e-6, cacheRead: 2e-6},        // 命中价官网未给：按输入原价
		"jamba-large":          {input: 2e-6, output: 8e-6},
		// 修正
		"gpt-4o-mini-tts":            {input: 0.6e-6, output: 10e-6},
		"gpt-realtime-2":             {input: 4e-6, output: 24e-6, cacheRead: 0.4e-6},
		"gemini-2.5-pro-preview-tts": {input: 1e-6, output: 20e-6},
		// 优惠价（存官网现价）
		"gpt-5.6-sol":      {input: 4e-6, output: 20e-6, cacheRead: 0.4e-6, cacheWrite: 5e-6, threshold: 272000, inMul: 2, outMul: 1.5},
		"gemini-3.6-flash": {input: 0.75e-6, output: 3.75e-6, cacheRead: 0.075e-6},
	} {
		t.Run(model, func(t *testing.T) {
			got, ok := pricingData[model]
			require.True(t, ok, "价格文件里要有 %s", model)
			require.InDelta(t, want.input, got.InputCostPerToken, 1e-15)
			require.InDelta(t, want.output, got.OutputCostPerToken, 1e-15)
			require.InDelta(t, want.cacheRead, got.CacheReadInputTokenCost, 1e-15)
			require.InDelta(t, want.cacheWrite, got.CacheCreationInputTokenCost, 1e-15)
			require.InDelta(t, want.cacheWrite1h, got.CacheCreationInputTokenCostAbove1hr, 1e-15)
			require.Equal(t, want.threshold, got.LongContextInputTokenThreshold)
			require.InDelta(t, want.inMul, got.LongContextInputCostMultiplier, 1e-12)
			require.InDelta(t, want.outMul, got.LongContextOutputCostMultiplier, 1e-12)
		})
	}

	// 停服 / 第三方托管的不能被这次补进来（DeepSeek 停服名单见 TestDeepseekPricingFileMatchesOfficialRates）
	for _, absent := range []string{"deepseek-chat", "deepseek-reasoner", "zai-glm-5-3", "kimi-k2.5", "moonshot-v1-8k", "qwen-mt-plus",
		"gpt-5.3-codex-spark", // OpenAI 2026-09-14 退役、没有 API 价（muqian 10-06「删」）
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
	for _, model := range []string{"claude-opus-5-5", "gpt-6-astra", "grok-4.20-0309-reasoning", "glm-4.6v", "minimax-m2.5-highspeed", "command-r-08-2024", "qwen3-vl-8b-instruct"} {
		entry, ok := byID[model]
		require.True(t, ok, model)
		require.NotNil(t, entry.InputPrice, model)
		require.NotNil(t, entry.OutputPrice, model)
	}
	require.Equal(t, "zhipu", byID["glm-4.6v"].Vendor, "智谱沿用目录里的 zhipu")
	require.NotEmpty(t, byID["grok-4.7"].Intervals, "xAI 长上下文换算成按 token 分段")
}
