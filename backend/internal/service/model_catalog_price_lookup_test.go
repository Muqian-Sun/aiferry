//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 「添加模型」按模型 ID 自动带价（2026-09-25）：与播种同一来源、同一优先级。
func TestLookupPriceFileEntry(t *testing.T) {
	svc := NewModelCatalogService(&stubModelCatalogRepo{}, nil, seedInputForTest(
		map[string]*LiteLLMModelPricing{
			"claude-sonnet-4": {LiteLLMProvider: "anthropic", InputCostPerToken: 3e-6, OutputCostPerToken: 15e-6},
			// 只有图片价、按 token 计费：播种会跳过，带价也不带
			"image-only-token": {LiteLLMProvider: "openai", TokenPricingAbsent: true},
		},
		map[string]*ModelPricing{
			"glm-5.2":         {InputPricePerToken: 0.6e-6, OutputPricePerToken: 2.2e-6},
			"claude-sonnet-4": {InputPricePerToken: 9e-6, OutputPricePerToken: 9e-6},
		},
	))

	entry, ok := svc.LookupPriceFileEntry("claude-sonnet-4")
	require.True(t, ok)
	require.Equal(t, "anthropic", entry.Vendor)
	require.Equal(t, BillingModeToken, entry.BillingMode)
	require.InDelta(t, 3e-6, *entry.InputPrice, 1e-12, "价格文件优先于硬编码兜底价")
	require.Equal(t, ModelCatalogStatusUnlisted, entry.Status)

	entry, ok = svc.LookupPriceFileEntry("  GLM-5.2 ")
	require.True(t, ok, "兜底表按规范化后的标识匹配")
	require.Equal(t, "GLM-5.2", entry.ModelID, "带出的条目保留管理员输入的写法")
	require.InDelta(t, 0.6e-6, *entry.InputPrice, 1e-12)

	seeds := xaiImagineSeeds()
	require.NotEmpty(t, seeds)
	entry, ok = svc.LookupPriceFileEntry(seeds[0].ModelID)
	require.True(t, ok, "xAI Imagine 官方媒体价")
	require.Equal(t, seeds[0].BillingMode, entry.BillingMode)

	_, ok = svc.LookupPriceFileEntry("image-only-token")
	require.False(t, ok, "按 token 计费却没有 token 价的不带出")
	_, ok = svc.LookupPriceFileEntry("definitely-unknown-model")
	require.False(t, ok)
	_, ok = svc.LookupPriceFileEntry("   ")
	require.False(t, ok)
}
