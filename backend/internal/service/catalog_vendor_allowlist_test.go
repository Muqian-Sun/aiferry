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
	// 同一厂商族（同一平台）只能有一种厂商串：价格页 / 模型页 / 按厂商填售价都按厂商串筛，
	// 两种写法就会把同一家拆成两个厂商（2026-10-08 Google 拆成 gemini 与 vertex_ai-language-models）。
	vendorsByPlatform := map[string]map[string]bool{}
	for _, entry := range entries {
		require.True(t, CatalogVendorAllowed(entry.Vendor), "%s 的厂商 %q 不在目录白名单里", entry.ModelID, entry.Vendor)
		platform := CatalogVendorPlatform(&entry)
		if platform == "" {
			continue
		}
		if vendorsByPlatform[platform] == nil {
			vendorsByPlatform[platform] = map[string]bool{}
		}
		vendorsByPlatform[platform][entry.Vendor] = true
	}
	for platform, vendors := range vendorsByPlatform {
		require.Len(t, vendors, 1, "平台 %s 的条目有多种厂商串：%v", platform, vendors)
	}
	require.NotEmpty(t, vendorsByPlatform[PlatformGemini], "价格文件里有 Google 的模型")
	// 价格文件里也不该再有白名单外的条目（不然每次启动都打一条跳过日志）
	for name, pricing := range pricingData {
		require.True(t, CatalogVendorAllowed(catalogSeedVendor(pricing.LiteLLMProvider)), "价格文件里 %s 的 provider %q 不在目录白名单里", name, pricing.LiteLLMProvider)
	}
}

// 价格文件的 provider 串归一成目录厂商串：Google、OpenAI 各只留一种写法，其余原样。
func TestCatalogSeedVendor(t *testing.T) {
	for provider, want := range map[string]string{
		"vertex_ai-language-models":  "gemini",
		"vertex_ai-embedding-models": "gemini",
		"Vertex_AI-Image-Models":     "gemini",
		"gemini":                     "gemini",
		"text-completion-openai":     "openai",
		"openai":                     "openai",
		" Anthropic ":                "anthropic",
		"dashscope":                  "dashscope",
		"":                           "",
	} {
		require.Equal(t, want, catalogSeedVendor(provider), provider)
	}

	pricingSvc := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"gemini-9-flash":  {InputCostPerToken: 1e-6, OutputCostPerToken: 4e-6, LiteLLMProvider: "vertex_ai-language-models", Mode: "chat"},
		"gemini-9-embed":  {InputCostPerToken: 1e-7, LiteLLMProvider: "vertex_ai-embedding-models", Mode: "embedding"},
		"gpt-9-instruct":  {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6, LiteLLMProvider: "text-completion-openai", Mode: "completion"},
		"gemini-9-pro":    {InputCostPerToken: 2e-6, OutputCostPerToken: 8e-6, LiteLLMProvider: "gemini", Mode: "chat"},
		"claude-9-sonnet": {InputCostPerToken: 3e-6, OutputCostPerToken: 15e-6, LiteLLMProvider: "anthropic", Mode: "chat"},
	}}
	entries := buildModelCatalogSeedEntries(ModelCatalogSeedInput{PricingService: pricingSvc})
	vendors := make(map[string]string, len(entries))
	for _, entry := range entries {
		if _, fromFile := pricingSvc.pricingData[entry.ModelID]; fromFile {
			vendors[entry.ModelID] = entry.Vendor
		}
	}
	require.Equal(t, map[string]string{
		"gemini-9-flash":  "gemini",
		"gemini-9-embed":  "gemini",
		"gpt-9-instruct":  "openai",
		"gemini-9-pro":    "gemini",
		"claude-9-sonnet": "anthropic",
	}, vendors, "播种按归一后的厂商串写条目，也按它过白名单")

	// 「添加模型」按价格文件带出的条目同一口径
	svc := NewModelCatalogService(&stubModelCatalogRepo{}, nil, ModelCatalogSeedInput{PricingService: pricingSvc})
	looked, ok := svc.LookupPriceFileEntry("gemini-9-flash")
	require.True(t, ok)
	require.Equal(t, "gemini", looked.Vendor)
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
	require.True(t, CatalogVendorAllowed(" Gemini "), "不分大小写、去首尾空白")
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
