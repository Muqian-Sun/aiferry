//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 售价、成本价每百万 Token 最多保留 4 位小数（muqian 2026-10-07）：价格页保存时取整，按模型、按渠道两种保存都一样。
func TestSaveEntryPricingRoundsSaleAndCostPrices(t *testing.T) {
	ctx := context.Background()
	seed := upstreamCostTestEntry()
	svc, repo := newTestModelCatalogService(seed)
	official := OfficialPrices{InputPrice: seed.InputPrice, OutputPrice: seed.OutputPrice, CacheReadPrice: seed.CacheReadPrice, Intervals: seed.Intervals}
	sale := CatalogSalePrices{
		InputPrice:     upstreamCostPtr(0.123456e-6),
		CacheReadPrice: upstreamCostPtr(0.016667e-6),
		Segments:       []CatalogSaleSegment{{MinTokens: 272000, OutputPrice: upstreamCostPtr(1.23456789e-6)}},
	}
	b := upstreamCostTestBinding(1)
	b.InputPrice = 0.0044204e-6
	b.CacheReadPrice = upstreamCostPtr(0.000442e-6)
	b.Intervals[0].InputPrice = upstreamCostPtr(0.00884e-6)

	_, err := svc.SaveEntryPricing(ctx, 1, official, sale, []ModelCatalogBinding{b}, newPricingTestAccounts())
	require.NoError(t, err)
	got := repo.entries[0].SalePrices
	require.InDelta(t, 0.1235e-6, *got.InputPrice, 1e-18)
	require.InDelta(t, 0.0167e-6, *got.CacheReadPrice, 1e-18)
	require.InDelta(t, 1.2346e-6, *got.Segments[0].OutputPrice, 1e-18)
	stored := repo.bindings[1][0]
	require.InDelta(t, 0.0044e-6, stored.InputPrice, 1e-18)
	require.InDelta(t, 0.0004e-6, *stored.CacheReadPrice, 1e-18)
	require.InDelta(t, 0.0088e-6, *stored.Intervals[0].InputPrice, 1e-18)
	require.InDelta(t, 0.9e-6, stored.OutputPrice, 1e-18, "本来就不超过 4 位的不变")
}

func TestSaveAccountPricingRoundsCostPrices(t *testing.T) {
	ctx := context.Background()
	seed := upstreamCostTestEntry()
	svc, repo := newTestModelCatalogService(seed)
	b := upstreamCostTestBinding(1)
	b.OutputPrice = 0.026520e-6
	b.CacheReadPrice = upstreamCostPtr(0.005525e-6)
	b.Intervals[0].OutputPrice = upstreamCostPtr(0.03978e-6)

	saved, err := svc.SaveAccountPricing(ctx, 1, []ModelCatalogBinding{b}, newPricingTestAccounts())
	require.NoError(t, err)
	require.Len(t, saved, 1)
	require.InDelta(t, 0.0265e-6, saved[0].OutputPrice, 1e-18)
	require.InDelta(t, 0.0055e-6, *saved[0].CacheReadPrice, 1e-18)
	require.InDelta(t, 0.0398e-6, *saved[0].Intervals[0].OutputPrice, 1e-18)
	require.InDelta(t, 0.0265e-6, repo.bindings[1][0].OutputPrice, 1e-18)
}
