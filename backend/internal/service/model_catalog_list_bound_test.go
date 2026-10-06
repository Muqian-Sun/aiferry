//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 一键上架这个渠道已承接、还没上架的模型（muqian 2026-10-07）：只改上架状态、不改归属；别的渠道的、已上架的不动；
// 上架校验不过的跳过并带回原因。
func TestModelCatalogService_ListEntriesBoundToAccount(t *testing.T) {
	price := func(v float64) *float64 { return &v }
	entry := func(id int64, model, status, managedBy string, withPrice bool) ModelCatalogEntry {
		e := ModelCatalogEntry{ID: id, ModelID: model, Vendor: "openai", BillingMode: BillingModeToken, Status: status, ManagedBy: managedBy}
		if withPrice {
			e.InputPrice, e.OutputPrice = price(1e-6), price(2e-6)
		}
		return e
	}
	svc, repo := newTestModelCatalogService(
		entry(1, "bound-unlisted", ModelCatalogStatusUnlisted, ModelCatalogManagedByAdmin, true),
		entry(2, "bound-listed", ModelCatalogStatusListed, ModelCatalogManagedByAdmin, true),
		entry(3, "other-channel", ModelCatalogStatusUnlisted, ModelCatalogManagedByAdmin, true),
		entry(4, "bound-no-price", ModelCatalogStatusUnlisted, ModelCatalogManagedByAdmin, false),
		entry(5, "bound-seed", ModelCatalogStatusUnlisted, ModelCatalogManagedBySeed, true),
	)
	repo.bindings = map[int64][]ModelCatalogBinding{
		1: {{EntryID: 1, AccountID: 7}},
		2: {{EntryID: 2, AccountID: 7}},
		3: {{EntryID: 3, AccountID: 8}},
		4: {{EntryID: 4, AccountID: 7}},
		5: {{EntryID: 5, AccountID: 7}},
	}

	got, err := svc.ListEntriesBoundToAccount(context.Background(), 7)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"bound-unlisted", "bound-seed"}, got.Listed)
	require.Len(t, got.Skipped, 1)
	require.Equal(t, "bound-no-price", got.Skipped[0].ModelID)
	require.Contains(t, got.Skipped[0].Reason, "price")

	status := map[string]string{}
	managed := map[string]string{}
	for _, e := range repo.entries {
		status[e.ModelID], managed[e.ModelID] = e.Status, e.ManagedBy
	}
	require.Equal(t, ModelCatalogStatusListed, status["bound-unlisted"])
	require.Equal(t, ModelCatalogStatusListed, status["bound-seed"])
	require.Equal(t, ModelCatalogManagedBySeed, managed["bound-seed"], "只改上架状态：播种条目照旧跟价格文件刷新")
	require.Equal(t, ModelCatalogStatusUnlisted, status["other-channel"], "别的渠道承接的不动")
	require.Equal(t, ModelCatalogStatusUnlisted, status["bound-no-price"])

	again, err := svc.ListEntriesBoundToAccount(context.Background(), 7)
	require.NoError(t, err)
	require.Empty(t, again.Listed, "都上架了再点一次没有要上架的")
}
