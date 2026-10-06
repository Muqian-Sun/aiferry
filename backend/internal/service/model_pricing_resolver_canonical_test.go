//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 只在目录里有价的模型（价格文件查不到）必须被判定为「有价」，否则会被回退到具体模型。
func TestHasResolvableTokenPricing_CatalogOnlyModel(t *testing.T) {
	bs := NewBillingService()
	entry := catalogEntryFromCard("team/only", ModelCatalogManagedBySeed, PricingCard{
		BillingMode: BillingModeToken,
		InputPrice:  float64Ptr(1e-6),
		OutputPrice: float64Ptr(2e-6),
	})
	entry.ID = 1
	catalog, _ := newTestModelCatalogService(entry)
	svc := &GatewayService{billingService: bs, resolver: NewModelPricingResolver(catalog)}
	ctx := context.Background()

	require.True(t, svc.hasResolvableTokenPricing(ctx, "team/only", &APIKey{}))
	require.False(t, svc.hasResolvableTokenPricing(ctx, "team/none", &APIKey{}))
	require.Equal(t, "team/only", svc.billableModelWithFallback(ctx, &APIKey{}, "team/only", "claude-sonnet-4"))
}

// 没有分组、也没有推理等级时，只要有解析器就必须走目录，不能退回价格文件直查。
func TestCalculateTokenCostForRequest_NoGroupUsesCatalog(t *testing.T) {
	bs := NewBillingService()
	entry := catalogEntryFromCard("team/only", ModelCatalogManagedBySeed, PricingCard{
		BillingMode: BillingModeToken,
		InputPrice:  float64Ptr(1e-6),
		OutputPrice: float64Ptr(2e-6),
	})
	entry.ID = 1
	catalog, _ := newTestModelCatalogService(entry)

	breakdown, err := bs.CalculateTokenCostForRequest(TokenCostRequest{
		Ctx:            context.Background(),
		Model:          "team/only",
		Tokens:         UsageTokens{InputTokens: 1000, OutputTokens: 500},
		RateMultiplier: 1,
		Resolver:       NewModelPricingResolver(catalog),
	})

	require.NoError(t, err)
	require.InDelta(t, 1e-6*1000+2e-6*500, breakdown.TotalCost, 1e-12)
}
