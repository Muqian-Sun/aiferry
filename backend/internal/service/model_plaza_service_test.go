//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelPlazaService_ListModelsProjectsListedEntries(t *testing.T) {
	repo := &stubModelCatalogRepo{entries: []ModelCatalogEntry{
		{ID: 1, ModelID: "gpt-5.6", DisplayName: "GPT-5.6", Vendor: "openai", Status: ModelCatalogStatusListed,
			InputPrice: testPtrFloat64(1e-6), OutputPrice: testPtrFloat64(2e-6),
			TimePricing: &TimePricing{Timezone: "Asia/Shanghai", Periods: []TimePricingPeriod{{StartTime: "09:00", EndTime: "18:00", Multiplier: 1.5}}}},
		{ID: 2, ModelID: "claude-fable-5-1", Vendor: "anthropic", Status: ModelCatalogStatusListed, InputPrice: testPtrFloat64(3e-6)},
		{ID: 3, ModelID: "hidden", Vendor: "openai", Status: ModelCatalogStatusUnlisted, InputPrice: testPtrFloat64(1e-6)},
	}}
	catalog := NewModelCatalogService(repo, nil, ModelCatalogSeedInput{})
	svc := NewModelPlazaService(catalog, catalog)

	models := svc.ListModels(context.Background())
	require.Len(t, models, 2, "unlisted entries are not shown")
	require.Equal(t, "claude-fable-5-1", models[0].ModelID)
	require.Nil(t, models[0].Pricing.MaxReasoningEffortMultiplier, "only the catalog's own multiplier is shown (none here)")
	require.Equal(t, "gpt-5.6", models[1].ModelID)
	require.Equal(t, "GPT-5.6", models[1].DisplayName)
	require.Equal(t, "openai", models[1].Vendor)
	require.Equal(t, BillingModeToken, models[1].BillingMode)
	require.Equal(t, 1e-6, *models[1].Pricing.InputPrice)
	require.Equal(t, "Asia/Shanghai", models[1].TimePricing.Timezone)
}
