//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 探测到的上游模型对到目录：规范化模型名、别名、去掉厂商前缀；目录里没有的一键导入为未上架条目（2026-09-29）。

func TestMatchUpstreamModels(t *testing.T) {
	svc, _ := newTestModelCatalogService(
		ModelCatalogEntry{ID: 1, ModelID: "claude-sonnet-4-6", Status: ModelCatalogStatusListed},
		ModelCatalogEntry{ID: 2, ModelID: "gpt-5.4", Status: ModelCatalogStatusUnlisted,
			Aliases: []ModelCatalogAlias{{EntryID: 2, Alias: "gpt-5.4-2026-03-01"}}},
	)

	got, err := svc.MatchUpstreamModels(context.Background(), []string{
		"claude-sonnet-4.6",           // 规范化：claude 的 "." → "-"
		"GPT-5.4-2026-03-01",          // 别名，大小写不敏感
		"anthropic/claude-sonnet-4-6", // 聚合平台的厂商前缀
		"brand-new-model",             // 目录里没有
	})
	require.NoError(t, err)
	require.Equal(t, []ProbedUpstreamModel{
		{ID: "claude-sonnet-4.6", EntryID: 1, EntryModelID: "claude-sonnet-4-6", Listed: true},
		{ID: "GPT-5.4-2026-03-01", EntryID: 2, EntryModelID: "gpt-5.4"},
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
