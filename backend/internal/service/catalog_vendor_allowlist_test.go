//go:build unit

package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
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
		BillingService: NewBillingService(&config.Config{}, pricingSvc),
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
