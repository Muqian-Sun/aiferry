//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// 渠道测试的模型下拉只认承接关系：列这个渠道承接的条目（上架与否都列），别的渠道的不列。
func TestModelCatalogServiceListAccountEntries_OnlyEntriesBoundToTheAccount(t *testing.T) {
	svc, repo := newTestModelCatalogService(
		ModelCatalogEntry{ID: 1, ModelID: "gpt-5.5", Status: ModelCatalogStatusListed},
		ModelCatalogEntry{ID: 2, ModelID: "deepseek-v4.1-flash", Status: ModelCatalogStatusUnlisted},
		ModelCatalogEntry{ID: 3, ModelID: "claude-sonnet-4-6", Status: ModelCatalogStatusListed},
	)
	repo.bindings = map[int64][]ModelCatalogBinding{
		1: {{EntryID: 1, AccountID: 7}, {EntryID: 1, AccountID: 8}},
		2: {{EntryID: 2, AccountID: 7, UpstreamModel: "deepseek-v4-1-flash"}},
		3: {{EntryID: 3, AccountID: 8}},
	}

	entries, err := svc.ListAccountEntries(context.Background(), 7)
	require.NoError(t, err)
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ModelID)
	}
	require.Equal(t, []string{"gpt-5.5", "deepseek-v4.1-flash"}, ids)
}

// 没有承接就是空列表（不是 nil、也不回退到别的表）。
func TestModelCatalogServiceListAccountEntries_NoBindingsIsEmpty(t *testing.T) {
	svc, repo := newTestModelCatalogService(ModelCatalogEntry{ID: 1, ModelID: "gpt-5.5"})
	repo.bindings = map[int64][]ModelCatalogBinding{1: {{EntryID: 1, AccountID: 8}}}

	entries, err := svc.ListAccountEntries(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, entries)
	require.Empty(t, entries)
}

func TestModelCatalogServiceListAccountEntries_PropagatesRepoError(t *testing.T) {
	svc, repo := newTestModelCatalogService()
	repo.listErr = errors.New("db down")

	_, err := svc.ListAccountEntries(context.Background(), 7)
	require.EqualError(t, err, "db down")
}
