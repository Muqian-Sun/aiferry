//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 探测到的上游模型对到目录：规范化模型名、去掉厂商前缀；目录里没有的一键导入为未上架条目（2026-09-29）。
// 目录不存别名（2026-10-06）：上游的别名对不上目录，要在承接行配上游模型名。

func TestMatchUpstreamModels(t *testing.T) {
	svc, _ := newTestModelCatalogService(
		ModelCatalogEntry{ID: 1, ModelID: "claude-sonnet-4-6", Status: ModelCatalogStatusListed},
		ModelCatalogEntry{ID: 2, ModelID: "gpt-5.4", Status: ModelCatalogStatusUnlisted},
	)

	got, err := svc.MatchUpstreamModels(context.Background(), []string{
		"claude-sonnet-4.6",           // 规范化：claude 的 "." → "-"
		"GPT-5.4",                     // 大小写不敏感
		"gpt-5.4-2026-03-01",          // 目录里没有这个 ID（别名不算）
		"anthropic/claude-sonnet-4-6", // 聚合平台的厂商前缀
		"brand-new-model",             // 目录里没有
	})
	require.NoError(t, err)
	require.Equal(t, []ProbedUpstreamModel{
		{ID: "claude-sonnet-4.6", EntryID: 1, EntryModelID: "claude-sonnet-4-6", Listed: true},
		{ID: "GPT-5.4", EntryID: 2, EntryModelID: "gpt-5.4"},
		{ID: "gpt-5.4-2026-03-01"},
		{ID: "anthropic/claude-sonnet-4-6", EntryID: 1, EntryModelID: "claude-sonnet-4-6", Listed: true},
		{ID: "brand-new-model"},
	}, got)
}

func TestImportUpstreamModels(t *testing.T) {
	svc, repo := newTestModelCatalogService(
		ModelCatalogEntry{ID: 1, ModelID: "claude-sonnet-4-6", Status: ModelCatalogStatusListed},
	)

	got, err := svc.ImportUpstreamModels(context.Background(), []string{
		"brand-new-model",
		"anthropic/claude-sonnet-4-6", // 已有：原样返回，不重复建
		"Brand-New-Model",             // 与第一个规范化后相同：去重
		"  ",
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "brand-new-model", got[0].ModelID)
	require.Equal(t, ModelCatalogStatusUnlisted, got[0].Status, "导入的一律未上架")
	require.NotZero(t, got[0].ID)
	require.Equal(t, int64(1), got[1].ID)

	entries, err := repo.ListEntries(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 2, "只新建了一条")
}

// 价格文件查不到的上游模型按模型族补厂商（2026-09-29 E2E：导入的 deepseek-v4.1-flash、glm-5.3-flash 厂商为空，
// 用户站模型页没有图标、也归不到厂商）；带组织前缀的按最后一段认；认不出的留空。
func TestImportUpstreamModelsFillsVendorByModelFamily(t *testing.T) {
	svc, _ := newTestModelCatalogService()

	got, err := svc.ImportUpstreamModels(context.Background(), []string{
		"deepseek-v4.1-flash",
		"glm-5.3-flash",
		"deepseek-ai/DeepSeek-V9",
		"brand-new-model",
	})
	require.NoError(t, err)
	require.Len(t, got, 4)
	require.Equal(t, "deepseek", got[0].Vendor)
	require.Equal(t, "zhipu", got[1].Vendor)
	require.Equal(t, "deepseek", got[2].Vendor)
	require.Empty(t, got[3].Vendor)
}
