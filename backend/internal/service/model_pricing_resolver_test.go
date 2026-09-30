//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestBillingServiceForResolver() *BillingService {
	bs := &BillingService{
		fallbackPrices: make(map[string]*ModelPricing),
	}
	bs.fallbackPrices["claude-sonnet-4"] = &ModelPricing{
		InputPricePerToken:         3e-6,
		OutputPricePerToken:        15e-6,
		CacheCreationPricePerToken: 3.75e-6,
		CacheReadPricePerToken:     0.3e-6,
		SupportsCacheBreakdown:     false,
	}
	return bs
}

func TestResolve_NoGroupID(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	require.NotNil(t, resolved)
	require.Equal(t, BillingModeToken, resolved.Mode)
	require.NotNil(t, resolved.BasePricing)
	require.InDelta(t, 3e-6, resolved.BasePricing.InputPricePerToken, 1e-12)
	// BillingService.GetModelPricing uses fallback internally, but resolveBasePricing
	// reports "litellm" when GetModelPricing succeeds (regardless of internal source)
	require.Equal(t, "litellm", resolved.Source)
}

func TestResolve_UnknownModel(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "unknown-model-xyz",
	})

	require.NotNil(t, resolved)
	require.Nil(t, resolved.BasePricing)
	// Unknown model: GetModelPricing returns error, source is "fallback"
	require.Equal(t, "fallback", resolved.Source)
}

func TestGetIntervalPricing_NoIntervals(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	basePricing := &ModelPricing{InputPricePerToken: 5e-6}
	resolved := &ResolvedPricing{
		Mode:        BillingModeToken,
		BasePricing: basePricing,
		Intervals:   nil,
	}

	result := r.GetIntervalPricing(resolved, 50000)
	require.Equal(t, basePricing, result)
}

func TestGetIntervalPricing_MatchesInterval(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	resolved := &ResolvedPricing{
		Mode:                   BillingModeToken,
		BasePricing:            &ModelPricing{InputPricePerToken: 5e-6},
		SupportsCacheBreakdown: true,
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(128000), InputPrice: testPtrFloat64(1e-6), OutputPrice: testPtrFloat64(2e-6)},
			{MinTokens: 128000, MaxTokens: nil, InputPrice: testPtrFloat64(3e-6), OutputPrice: testPtrFloat64(6e-6)},
		},
	}

	result := r.GetIntervalPricing(resolved, 50000)
	require.NotNil(t, result)
	require.InDelta(t, 1e-6, result.InputPricePerToken, 1e-12)
	require.InDelta(t, 2e-6, result.OutputPricePerToken, 1e-12)
	require.True(t, result.SupportsCacheBreakdown)

	result2 := r.GetIntervalPricing(resolved, 200000)
	require.NotNil(t, result2)
	require.InDelta(t, 3e-6, result2.InputPricePerToken, 1e-12)
}

func TestGetIntervalPricing_NoMatch_FallsBackToBase(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	basePricing := &ModelPricing{InputPricePerToken: 99e-6}
	resolved := &ResolvedPricing{
		Mode:        BillingModeToken,
		BasePricing: basePricing,
		Intervals: []PricingInterval{
			{MinTokens: 10000, MaxTokens: testPtrInt(50000), InputPrice: testPtrFloat64(1e-6)},
		},
	}

	result := r.GetIntervalPricing(resolved, 5000)
	require.Equal(t, basePricing, result)
}

func TestGPT56ExplicitZeroCacheWritePriceIsPreserved(t *testing.T) {
	bs := &BillingService{}
	resolver := NewModelPricingResolver(nil, bs)
	zero := 0.0

	t.Run("flat catalog price", func(t *testing.T) {
		pricing := &ModelPricing{InputPricePerToken: 5e-6, OutputPricePerToken: 30e-6}
		applyChannelTokenPriceOverrides(pricing, &PricingCard{CacheWritePrice: &zero})
		resolved := &ResolvedPricing{Mode: BillingModeToken, BasePricing: pricing}

		require.True(t, resolved.BasePricing.CacheCreationPriceExplicit)
		cost, err := bs.CalculateCostUnified(CostInput{
			Model:          "gpt-5.6-sol",
			Tokens:         UsageTokens{CacheCreationTokens: 100},
			RateMultiplier: 1,
			Resolver:       resolver,
			Resolved:       resolved,
		})
		require.NoError(t, err)
		require.Zero(t, cost.CacheCreationCost)
	})

	t.Run("interval price", func(t *testing.T) {
		pricing := intervalToModelPricing(&PricingInterval{CacheWritePrice: &zero}, &ModelPricing{}, nil, false)
		require.True(t, pricing.CacheCreationPriceExplicit)

		cost, err := bs.CalculateCostUnified(CostInput{
			Model:          "gpt-5.6-sol",
			Tokens:         UsageTokens{CacheCreationTokens: 100},
			RateMultiplier: 1,
			Resolver:       resolver,
			Resolved: &ResolvedPricing{
				Mode:        BillingModeToken,
				BasePricing: pricing,
			},
		})
		require.NoError(t, err)
		require.Zero(t, cost.CacheCreationCost)
	})
}

func TestGetRequestTierPrice(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	resolved := &ResolvedPricing{
		Mode: BillingModePerRequest,
		RequestTiers: []PricingInterval{
			{TierLabel: "1K", PerRequestPrice: testPtrFloat64(0.04)},
			{TierLabel: "2K", PerRequestPrice: testPtrFloat64(0.08)},
		},
	}

	require.InDelta(t, 0.04, r.GetRequestTierPrice(resolved, "1K"), 1e-12)
	require.InDelta(t, 0.08, r.GetRequestTierPrice(resolved, "2K"), 1e-12)
	require.InDelta(t, 0.0, r.GetRequestTierPrice(resolved, "4K"), 1e-12)
}

func TestGetRequestTierPriceByContext(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	resolved := &ResolvedPricing{
		Mode: BillingModePerRequest,
		RequestTiers: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(128000), PerRequestPrice: testPtrFloat64(0.05)},
			{MinTokens: 128000, MaxTokens: nil, PerRequestPrice: testPtrFloat64(0.10)},
		},
	}

	require.InDelta(t, 0.05, r.GetRequestTierPriceByContext(resolved, 50000), 1e-12)
	require.InDelta(t, 0.10, r.GetRequestTierPriceByContext(resolved, 200000), 1e-12)
}

func TestGetRequestTierPrice_NilPerRequestPrice(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	resolved := &ResolvedPricing{
		Mode: BillingModePerRequest,
		RequestTiers: []PricingInterval{
			{TierLabel: "1K", PerRequestPrice: nil},
		},
	}

	require.InDelta(t, 0.0, r.GetRequestTierPrice(resolved, "1K"), 1e-12)
}

// ===========================================================================
// Channel override tests — exercises applyChannelOverrides via Resolve
// ===========================================================================

// newResolverWithCatalog 用目录条目搭解析器：价卡能力已从渠道搬到 model_catalog，
// 这些用例跟着搬，语义保持「运营者显式配了价」（managed_by = admin）。
func newResolverWithCatalog(t *testing.T, pricing []PricingCard) *ModelPricingResolver {
	t.Helper()
	bs := newTestBillingServiceForResolver()
	return newResolverWithCatalogCards(bs, pricing...)
}

// ---------------------------------------------------------------------------
// 1. Token mode overrides
// ---------------------------------------------------------------------------

func TestResolve_WithChannelOverride_TokenFlat(t *testing.T) {
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(10e-6),
		OutputPrice: testPtrFloat64(50e-6),
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	require.NotNil(t, resolved)
	require.Equal(t, BillingModeToken, resolved.Mode)
	require.Equal(t, "catalog", resolved.Source)
	require.NotNil(t, resolved.BasePricing)
	require.InDelta(t, 10e-6, resolved.BasePricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 50e-6, resolved.BasePricing.OutputPricePerToken, 1e-12)
}

func TestResolve_WithChannelOverride_TokenPartialOverride(t *testing.T) {
	// Channel only sets InputPrice; OutputPrice should remain from the base (LiteLLM/fallback).
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(20e-6),
		// OutputPrice intentionally nil
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	require.NotNil(t, resolved)
	require.Equal(t, "catalog", resolved.Source)
	require.NotNil(t, resolved.BasePricing)
	// InputPrice overridden by channel
	require.InDelta(t, 20e-6, resolved.BasePricing.InputPricePerToken, 1e-12)
	// OutputPrice kept from base (fallback: 15e-6)
	require.InDelta(t, 15e-6, resolved.BasePricing.OutputPricePerToken, 1e-12)
}

func TestResolve_WithChannelOverride_TokenWithIntervals(t *testing.T) {
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeToken,
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(128000), InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(8e-6)},
			{MinTokens: 128000, MaxTokens: nil, InputPrice: testPtrFloat64(4e-6), OutputPrice: testPtrFloat64(16e-6)},
		},
	}})

	resolved := r.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-4"})

	require.NotNil(t, resolved)
	require.Equal(t, "catalog", resolved.Source)
	require.Len(t, resolved.Intervals, 2)

	// GetIntervalPricing should use channel intervals
	iv := r.GetIntervalPricing(resolved, 50000)
	require.NotNil(t, iv)
	require.InDelta(t, 2e-6, iv.InputPricePerToken, 1e-12)
	require.InDelta(t, 8e-6, iv.OutputPricePerToken, 1e-12)

	iv2 := r.GetIntervalPricing(resolved, 200000)
	require.NotNil(t, iv2)
	require.InDelta(t, 4e-6, iv2.InputPricePerToken, 1e-12)
	require.InDelta(t, 16e-6, iv2.OutputPricePerToken, 1e-12)
}

func TestResolve_WithChannelOverride_TokenNilBasePricing(t *testing.T) {
	// Base pricing is nil (unknown model), channel has flat prices → creates new BasePricing.
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"unknown-model-xyz"},
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(7e-6),
		OutputPrice: testPtrFloat64(21e-6),
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "unknown-model-xyz",
	})

	require.NotNil(t, resolved)
	require.Equal(t, "catalog", resolved.Source)
	// BasePricing was nil from resolveBasePricing but applyTokenOverrides creates a new one
	require.NotNil(t, resolved.BasePricing)
	require.InDelta(t, 7e-6, resolved.BasePricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 21e-6, resolved.BasePricing.OutputPricePerToken, 1e-12)
}

// ---------------------------------------------------------------------------
// 2. Per-request mode overrides
// ---------------------------------------------------------------------------

func TestResolve_WithChannelOverride_PerRequest(t *testing.T) {
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:          []string{"claude-sonnet-4"},
		BillingMode:     BillingModePerRequest,
		PerRequestPrice: testPtrFloat64(0.05),
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(128000), PerRequestPrice: testPtrFloat64(0.03)},
			{MinTokens: 128000, MaxTokens: nil, PerRequestPrice: testPtrFloat64(0.10)},
		},
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	require.NotNil(t, resolved)
	require.Equal(t, BillingModePerRequest, resolved.Mode)
	require.Equal(t, "catalog", resolved.Source)
	require.InDelta(t, 0.05, resolved.DefaultPerRequestPrice, 1e-12)
	require.Len(t, resolved.RequestTiers, 2)

	// Verify tier lookups
	require.InDelta(t, 0.03, r.GetRequestTierPriceByContext(resolved, 50000), 1e-12)
	require.InDelta(t, 0.10, r.GetRequestTierPriceByContext(resolved, 200000), 1e-12)
}

func TestResolve_WithChannelOverride_PerRequestNilPrice(t *testing.T) {
	// PerRequestPrice nil → DefaultPerRequestPrice stays 0.
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModePerRequest,
		// PerRequestPrice intentionally nil
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(128000), PerRequestPrice: testPtrFloat64(0.02)},
		},
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	require.NotNil(t, resolved)
	require.Equal(t, BillingModePerRequest, resolved.Mode)
	require.InDelta(t, 0.0, resolved.DefaultPerRequestPrice, 1e-12)
	require.Len(t, resolved.RequestTiers, 1)
}

// ---------------------------------------------------------------------------
// 3. Image mode overrides
// ---------------------------------------------------------------------------

func TestResolve_WithChannelOverride_Image(t *testing.T) {
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:          []string{"claude-sonnet-4"},
		BillingMode:     BillingModeImage,
		PerRequestPrice: testPtrFloat64(0.08),
		Intervals: []PricingInterval{
			{TierLabel: "1K", PerRequestPrice: testPtrFloat64(0.04)},
			{TierLabel: "2K", PerRequestPrice: testPtrFloat64(0.08)},
			{TierLabel: "4K", PerRequestPrice: testPtrFloat64(0.16)},
		},
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	require.NotNil(t, resolved)
	require.Equal(t, BillingModeImage, resolved.Mode)
	require.Equal(t, "catalog", resolved.Source)
	require.InDelta(t, 0.08, resolved.DefaultPerRequestPrice, 1e-12)
	require.Len(t, resolved.RequestTiers, 3)
}

func TestResolve_WithChannelOverride_ImageTierLabels(t *testing.T) {
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeImage,
		Intervals: []PricingInterval{
			{TierLabel: "1K", PerRequestPrice: testPtrFloat64(0.04)},
			{TierLabel: "2K", PerRequestPrice: testPtrFloat64(0.08)},
			{TierLabel: "4K", PerRequestPrice: testPtrFloat64(0.16)},
		},
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	require.InDelta(t, 0.04, r.GetRequestTierPrice(resolved, "1K"), 1e-12)
	require.InDelta(t, 0.08, r.GetRequestTierPrice(resolved, "2K"), 1e-12)
	require.InDelta(t, 0.16, r.GetRequestTierPrice(resolved, "4K"), 1e-12)
	require.InDelta(t, 0.0, r.GetRequestTierPrice(resolved, "8K"), 1e-12) // not found
}

// ---------------------------------------------------------------------------
// 4. Source tracking & default mode
// ---------------------------------------------------------------------------

func TestResolve_WithChannelOverride_SourceIsChannel(t *testing.T) {
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(1e-6),
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	require.Equal(t, "catalog", resolved.Source)
}

func TestResolve_WithChannelOverride_DefaultMode(t *testing.T) {
	// Channel pricing with empty BillingMode → defaults to BillingModeToken.
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: "", // intentionally empty
		InputPrice:  testPtrFloat64(5e-6),
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	require.Equal(t, "catalog", resolved.Source)
	require.Equal(t, BillingModeToken, resolved.Mode)
	require.NotNil(t, resolved.BasePricing)
	require.InDelta(t, 5e-6, resolved.BasePricing.InputPricePerToken, 1e-12)
}

// ---------------------------------------------------------------------------
// 5. GetIntervalPricing integration after channel override
// ---------------------------------------------------------------------------

func TestGetIntervalPricing_WithChannelIntervals(t *testing.T) {
	// Channel provides intervals that override the base pricing path.
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeToken,
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(100000), InputPrice: testPtrFloat64(1e-6), OutputPrice: testPtrFloat64(5e-6)},
			{MinTokens: 100000, MaxTokens: nil, InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(10e-6)},
		},
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	// Token count 50000 matches first interval
	pricing := r.GetIntervalPricing(resolved, 50000)
	require.NotNil(t, pricing)
	require.InDelta(t, 1e-6, pricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 5e-6, pricing.OutputPricePerToken, 1e-12)

	// Token count 150000 matches second interval
	pricing2 := r.GetIntervalPricing(resolved, 150000)
	require.NotNil(t, pricing2)
	require.InDelta(t, 2e-6, pricing2.InputPricePerToken, 1e-12)
	require.InDelta(t, 10e-6, pricing2.OutputPricePerToken, 1e-12)
}

func TestGetIntervalPricing_ChannelIntervalsNoMatch(t *testing.T) {
	// Channel intervals don't match token count → falls back to BasePricing.
	r := newResolverWithCatalog(t, []PricingCard{{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(4e-6),
		Intervals: []PricingInterval{
			// Only covers tokens > 50000
			{MinTokens: 50000, MaxTokens: testPtrInt(200000), InputPrice: testPtrFloat64(9e-6)},
		},
	}})

	resolved := r.Resolve(context.Background(), PricingInput{
		Model: "claude-sonnet-4",
	})

	// Token count 1000 doesn't match any interval (1000 <= 50000 minTokens)
	pricing := r.GetIntervalPricing(resolved, 1000)
	// Should fall back to BasePricing after applying the channel default.
	require.NotNil(t, pricing)
	require.Equal(t, resolved.BasePricing, pricing)
	require.InDelta(t, 4e-6, pricing.InputPricePerToken, 1e-12)
}

// ===========================================================================
// 6. Error path tests
// ===========================================================================

// TestResolve_CatalogLoadError 目录读库失败时不能 panic，也不能把「查不到目录」
// 变成「查不到价」：必须退回价格文件 / 硬编码兜底价。
func TestResolve_CatalogLoadError(t *testing.T) {
	repo := &stubModelCatalogRepo{listErr: errors.New("database unavailable")}
	catalog := NewModelCatalogService(repo, nil, ModelCatalogSeedInput{})
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(catalog, bs)

	resolved := r.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-4"})

	require.NotNil(t, resolved)
	require.NotEqual(t, PricingSourceCatalog, resolved.Source)
	require.NotNil(t, resolved.BasePricing)
	require.InDelta(t, 3e-6, resolved.BasePricing.InputPricePerToken, 1e-12)
}

// ===========================================================================
// 7. GetRequestTierPriceByContext boundary tests
// ===========================================================================

func TestGetRequestTierPriceByContext_EmptyTiers(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	resolved := &ResolvedPricing{
		Mode:         BillingModePerRequest,
		RequestTiers: nil, // empty
	}

	price := r.GetRequestTierPriceByContext(resolved, 50000)
	require.InDelta(t, 0.0, price, 1e-12)

	// Also test with explicit empty slice
	resolved2 := &ResolvedPricing{
		Mode:         BillingModePerRequest,
		RequestTiers: []PricingInterval{},
	}

	price2 := r.GetRequestTierPriceByContext(resolved2, 50000)
	require.InDelta(t, 0.0, price2, 1e-12)
}

func TestGetRequestTierPriceByContext_ExactBoundary(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := NewModelPricingResolver(nil, bs)

	resolved := &ResolvedPricing{
		Mode: BillingModePerRequest,
		RequestTiers: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(128000), PerRequestPrice: testPtrFloat64(0.05)},
			{MinTokens: 128000, MaxTokens: nil, PerRequestPrice: testPtrFloat64(0.10)},
		},
	}

	// totalContextTokens = 128000 exactly:
	// FindMatchingInterval checks: totalTokens > MinTokens && totalTokens <= MaxTokens
	// For first interval: 128000 > 0 (true) && 128000 <= 128000 (true) → matches first interval
	price := r.GetRequestTierPriceByContext(resolved, 128000)
	require.InDelta(t, 0.05, price, 1e-12)

	// totalContextTokens = 128001 should match second interval
	// For first interval: 128001 > 0 (true) && 128001 <= 128000 (false) → no match
	// For second interval: 128001 > 128000 (true) && MaxTokens == nil → matches
	price2 := r.GetRequestTierPriceByContext(resolved, 128001)
	require.InDelta(t, 0.10, price2, 1e-12)
}

// ===========================================================================
// 8. filterValidIntervals
// ===========================================================================

func TestFilterValidIntervals(t *testing.T) {
	tests := []struct {
		name      string
		intervals []PricingInterval
		wantLen   int
	}{
		{
			name:      "empty list",
			intervals: nil,
			wantLen:   0,
		},
		{
			name: "all-nil interval filtered out",
			intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: testPtrInt(128000)},
			},
			wantLen: 0,
		},
		{
			name: "interval with only InputPrice kept",
			intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: testPtrInt(128000), InputPrice: testPtrFloat64(1e-6)},
			},
			wantLen: 1,
		},
		{
			name: "interval with only OutputPrice kept",
			intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: testPtrInt(128000), OutputPrice: testPtrFloat64(2e-6)},
			},
			wantLen: 1,
		},
		{
			name: "interval with only CacheWritePrice kept",
			intervals: []PricingInterval{
				{MinTokens: 0, CacheWritePrice: testPtrFloat64(3e-6)},
			},
			wantLen: 1,
		},
		{
			name: "interval with only CacheReadPrice kept",
			intervals: []PricingInterval{
				{MinTokens: 0, CacheReadPrice: testPtrFloat64(0.5e-6)},
			},
			wantLen: 1,
		},
		{
			name: "interval with only PerRequestPrice kept",
			intervals: []PricingInterval{
				{TierLabel: "1K", PerRequestPrice: testPtrFloat64(0.04)},
			},
			wantLen: 1,
		},
		{
			name: "interval with only multiplier kept",
			intervals: []PricingInterval{
				{MinTokens: 272000, InputMultiplier: testPtrFloat64(2)},
			},
			wantLen: 1,
		},
		{
			name: "mixed valid and invalid",
			intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: testPtrInt(128000), InputPrice: testPtrFloat64(1e-6)},
				{MinTokens: 128000, MaxTokens: nil}, // all-nil → filtered out
				{MinTokens: 256000, OutputPrice: testPtrFloat64(5e-6)},
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterValidIntervals(tt.intervals)
			require.Len(t, result, tt.wantLen)
		})
	}
}

// ===========================================================================
// 9. 图片输出价：目录是基准价（不归零），分组价卡是运营者定价（覆盖一切）
// ===========================================================================

// newBillingServiceWithImageOutputPrice 造一份带图片输出价的基准价卡，
// 用来区分「目录没配图片价 → 保留基准价」与「分组价卡没配 → 归零」。
func newBillingServiceWithImageOutputPrice() *BillingService {
	bs := newTestBillingServiceForResolver()
	bs.fallbackPrices["claude-sonnet-4"].ImageOutputPricePerToken = 40e-6
	return bs
}

// TestCatalogTokenCard_KeepsBaseImageOutputPrice：目录条目没配图片输出价时，
// 必须保留价格文件里的图片输出价，也不能置 Explicit——否则图片 token 会被
// 静默按 $0 计费（今天是回退到文本输出价）。
func TestCatalogTokenCard_KeepsBaseImageOutputPrice(t *testing.T) {
	bs := newBillingServiceWithImageOutputPrice()
	r := newResolverWithCatalogCards(bs, PricingCard{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(3e-6),
		OutputPrice: testPtrFloat64(15e-6),
		// ImageOutputPrice 故意留空
	})

	resolved := r.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-4"})

	require.Equal(t, PricingSourceCatalog, resolved.Source)
	require.False(t, resolved.BasePricing.ImageOutputPriceExplicit)
	require.InDelta(t, 40e-6, resolved.BasePricing.ImageOutputPricePerToken, 1e-12)
}

// TestCatalogTokenCard_ImageOutputPriceSetsExplicit：目录条目显式配了图片输出价时
// 生效并置 Explicit（显式 0 也按 0 收）。
func TestCatalogTokenCard_ImageOutputPriceSetsExplicit(t *testing.T) {
	bs := newBillingServiceWithImageOutputPrice()
	r := newResolverWithCatalogCards(bs, PricingCard{
		Models:           []string{"claude-sonnet-4"},
		BillingMode:      BillingModeToken,
		InputPrice:       testPtrFloat64(3e-6),
		OutputPrice:      testPtrFloat64(15e-6),
		ImageOutputPrice: testPtrFloat64(50e-6),
	})

	resolved := r.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-4"})

	require.True(t, resolved.BasePricing.ImageOutputPriceExplicit)
	require.InDelta(t, 50e-6, resolved.BasePricing.ImageOutputPricePerToken, 1e-12)
}

// TestCatalogIntervalCard_KeepsBaseImageOutputPrice：命中分档时同样不得归零。
func TestCatalogIntervalCard_KeepsBaseImageOutputPrice(t *testing.T) {
	bs := newBillingServiceWithImageOutputPrice()
	r := newResolverWithCatalogCards(bs, PricingCard{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeToken,
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(100000), InputPrice: testPtrFloat64(3e-6), OutputPrice: testPtrFloat64(15e-6)},
		},
	})

	resolved := r.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-4"})
	require.False(t, resolved.BasePricing.ImageOutputPriceExplicit)

	pricing := r.GetIntervalPricing(resolved, 50000)
	require.False(t, pricing.ImageOutputPriceExplicit)
	require.InDelta(t, 40e-6, pricing.ImageOutputPricePerToken, 1e-12)
}

// ===========================================================================
// 10. 回归：价卡覆盖不得污染共享的 fallbackPrices 指针
// ===========================================================================

// TestCatalogFlatCard_DoesNotPolluteFallbackPrices 目录平价覆盖前必须先克隆
// BasePricing，否则会写穿共享的 fallbackPrices 条目。
func TestCatalogFlatCard_DoesNotPolluteFallbackPrices(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	r := newResolverWithCatalogCards(bs, PricingCard{
		Models:      []string{"claude-sonnet-4"},
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(10e-6), // base is 3e-6
		OutputPrice: testPtrFloat64(50e-6), // base is 15e-6
	})

	resolved := r.Resolve(context.Background(), PricingInput{Model: "claude-sonnet-4"})

	require.NotNil(t, resolved)
	require.InDelta(t, 10e-6, resolved.BasePricing.InputPricePerToken, 1e-12)
	require.InDelta(t, 50e-6, resolved.BasePricing.OutputPricePerToken, 1e-12)

	fp := r.billingService.fallbackPrices["claude-sonnet-4"]
	require.InDelta(t, 3e-6, fp.InputPricePerToken, 1e-12, "fallback InputPricePerToken polluted")
	require.InDelta(t, 15e-6, fp.OutputPricePerToken, 1e-12, "fallback OutputPricePerToken polluted")
	require.False(t, fp.CacheCreationPriceExplicit, "fallback CacheCreationPriceExplicit polluted")
}

func TestCalculateCostUnified_UsesContinuousMediaUnits(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	price := 0.08
	r := newResolverWithCatalogCards(bs, PricingCard{
		Models: []string{"grok-voice-think-fast-2.0"}, BillingMode: BillingModePerRequest,
		PerRequestPrice: &price,
	})
	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "grok-voice-think-fast-2.0",
		UsageUnits: 1.5, RateMultiplier: 1, Resolver: r,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.12, cost.TotalCost, 1e-12)
}
