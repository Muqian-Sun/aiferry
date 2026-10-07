//go:build unit

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// 模型目录只收 11 家（muqian 2026-10-06）：价格文件与兜底价表播进目录的每一条都要在白名单里。
func TestCatalogSeedOnlyAllowlistedVendors(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	pricingData, err := (&PricingService{}).parsePricingData(data)
	require.NoError(t, err)
	pricingSvc := &PricingService{pricingData: pricingData}
	entries := buildModelCatalogSeedEntries(ModelCatalogSeedInput{
		PricingService: pricingSvc,
		BillingService: NewBillingService(),
	})
	require.NotEmpty(t, entries)
	for _, entry := range entries {
		require.True(t, CatalogVendorAllowed(entry.Vendor), "%s 的厂商 %q 不在目录白名单里", entry.ModelID, entry.Vendor)
	}
	// 价格文件里也不该再有白名单外的条目（不然每次启动都打一条跳过日志）
	for name, pricing := range pricingData {
		require.True(t, CatalogVendorAllowed(pricing.LiteLLMProvider), "价格文件里 %s 的 provider %q 不在目录白名单里", name, pricing.LiteLLMProvider)
	}
}

// 白名单外的厂商即使出现在价格文件里也不播进目录。
func TestCatalogSeedSkipsVendorsOutsideAllowlist(t *testing.T) {
	pricingSvc := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"mistral-large-latest": {InputCostPerToken: 5e-7, OutputCostPerToken: 1.5e-6, LiteLLMProvider: "mistral", Mode: "chat"},
		"gpt-6-sol":            {InputCostPerToken: 2e-6, OutputCostPerToken: 1e-5, LiteLLMProvider: "openai", Mode: "chat"},
	}}
	entries := buildModelCatalogSeedEntries(ModelCatalogSeedInput{PricingService: pricingSvc})
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ModelID)
	}
	require.Contains(t, ids, "gpt-6-sol")
	require.NotContains(t, ids, "mistral-large-latest", "Mistral 不在 11 家里，不播进目录")
	require.True(t, CatalogVendorAllowed("Vertex_AI-Language-Models"), "不分大小写")
	require.False(t, CatalogVendorAllowed(""), "没有厂商的不收")
}

// 联网查官方 ID 只认白名单里的厂商，写进条目的厂商串也要在白名单里。
func TestOfficialModelProvidersWithinAllowlist(t *testing.T) {
	for provider, vendor := range officialModelProviders {
		require.True(t, CatalogVendorAllowed(vendor), "%s → %s", provider, vendor)
	}
	for _, outside := range []string{"mistral", "cohere", "ai21", "perplexity", "openrouter", "bedrock"} {
		_, ok := officialModelProviders[outside]
		require.False(t, ok, outside)
	}
}

// 目录只留官网在售、没宣布停服、ID 固定的（2026-10-06 按官网核对）：排除清单里的一个都不播进目录，
// 价格文件来的（o3、gpt-image-1、gemini-2.0-flash）与兜底价表来的（kimi-k2.5、claude-3-5-haiku、gemini-3.1-pro-high）都算。
func TestCatalogSeedSkipsExcludedModels(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "model-pricing", "model_prices_and_context_window.json"))
	require.NoError(t, err)
	pricingData, err := (&PricingService{}).parsePricingData(data)
	require.NoError(t, err)
	pricingSvc := &PricingService{pricingData: pricingData}
	entries := buildModelCatalogSeedEntries(ModelCatalogSeedInput{
		PricingService: pricingSvc,
		BillingService: NewBillingService(),
	})
	byID := make(map[string]ModelCatalogEntry, len(entries))
	for _, entry := range entries {
		require.False(t, catalogModelExcluded(entry.ModelID), "%s 在排除清单里，不该播进目录", entry.ModelID)
		byID[entry.ModelID] = entry
	}
	for _, removed := range []string{"o3", "gpt-image-1", "gemini-2.0-flash", "gpt-4", "chat-latest", "grok-4.5-latest", "kimi-k2.5", "claude-3-5-haiku", "gemini-3.1-pro-high", "claude-opus-4-6-thinking", "doubao-embedding-vision", "qwq-plus", "qwen3-32b", "qwen3-vl-8b-instruct",
		// muqian 2026-10-07：豆包文本模型不接入；国内站价目没有的、按输出长度另价的不收
		"doubao-seed-2-1-pro-260915", "doubao-seed-character-260628", "qwen-plus-character-ja", "glm-4.6", "glm-4.5", "glm-4.7", "glm-4.5-air"} {
		require.True(t, catalogModelExcluded(removed), removed)
		_, ok := byID[removed]
		require.False(t, ok, removed)
	}
	for _, kept := range []string{"gpt-5.5", "gpt-image-2", "claude-haiku-4-5", "claude-opus-4-6", "gemini-3.6-flash", "grok-4.7", "grok-imagine-video-1.5", "deepseek-flash", "deepseek-v4-flash", "glm-5-turbo", "kimi-k3", "qwen3.8-flash", "qwen-max"} {
		_, ok := byID[kept]
		require.True(t, ok, "%s 官网在售，要留在目录里", kept)
	}

	// 国内模型一律按国内站人民币价 ÷ 6.8（muqian 2026-10-07）：小米 MiMo 国内定价 ¥3 / ¥6
	mimo := byID["mimo-v2.6-pro"]
	require.Equal(t, "xiaomi", mimo.Vendor)
	require.InDelta(t, cnyPerMillion(3), *mimo.InputPrice, 1e-15)
	require.InDelta(t, cnyPerMillion(6), *mimo.OutputPrice, 1e-15)
	embedding, ok := byID["doubao-embedding-vision-251215"]
	require.True(t, ok, "豆包向量模型用带版本号的官方 ID")
	require.Equal(t, "volcengine", embedding.Vendor)

	for id, reason := range catalogExcludedModels {
		require.Contains(t, []string{"deprecated", "shutdown", "retired", "not_listed", "not_priced", "not_official", "moving_alias", "not_integrated", "not_domestic", "output_tiered"}, reason, id)
	}
}
