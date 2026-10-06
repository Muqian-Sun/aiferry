//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func upstreamCostPtr(v float64) *float64 { return &v }

// upstreamCostTestEntry 官方价：输入 5、输出 30、缓存读 0.5（$ / 百万 Token）；>272K 一段输入 10、输出 45，
// 缓存读留空（按本段输入价同比例折算成 1）。
func upstreamCostTestEntry() ModelCatalogEntry {
	return ModelCatalogEntry{
		ID:             1,
		ModelID:        "upstream-test-model",
		BillingMode:    BillingModeToken,
		Status:         ModelCatalogStatusListed,
		ManagedBy:      ModelCatalogManagedByAdmin,
		InputPrice:     upstreamCostPtr(5e-6),
		OutputPrice:    upstreamCostPtr(30e-6),
		CacheReadPrice: upstreamCostPtr(0.5e-6),
		Intervals: []PricingInterval{
			{MinTokens: 272000, InputPrice: upstreamCostPtr(10e-6), OutputPrice: upstreamCostPtr(45e-6)},
		},
	}
}

// upstreamCostTestBinding 上游价 = 官方价的 3%：输入 0.15、输出 0.9、缓存读 0.015；>272K 一段输入 0.3、输出 1.35。
func upstreamCostTestBinding(accountID int64) ModelCatalogBinding {
	return ModelCatalogBinding{
		EntryID:        1,
		AccountID:      accountID,
		InputPrice:     0.15e-6,
		OutputPrice:    0.9e-6,
		CacheReadPrice: upstreamCostPtr(0.015e-6),
		Intervals: []PricingInterval{
			{MinTokens: 272000, InputPrice: upstreamCostPtr(0.3e-6), OutputPrice: upstreamCostPtr(1.35e-6)},
		},
	}
}

func newUpstreamCostTestResolver(t *testing.T, bs *BillingService, entry ModelCatalogEntry, bindings ...ModelCatalogBinding) *ModelPricingResolver {
	t.Helper()
	catalog, repo := newTestModelCatalogService(entry)
	repo.bindings = map[int64][]ModelCatalogBinding{entry.ID: bindings}
	return NewModelPricingResolver(catalog, bs)
}

func TestBindingCostRatio(t *testing.T) {
	entry := upstreamCostTestEntry()

	t.Run("same share of every item", func(t *testing.T) {
		b := upstreamCostTestBinding(7)
		ratio, ok := bindingCostRatio(&entry, &b)
		require.True(t, ok)
		require.InDelta(t, 0.03, ratio, 1e-12)
	})

	t.Run("most expensive item wins", func(t *testing.T) {
		b := upstreamCostTestBinding(7)
		b.OutputPrice = 1.2e-6 // 30 的 4%
		ratio, ok := bindingCostRatio(&entry, &b)
		require.True(t, ok)
		require.InDelta(t, 0.04, ratio, 1e-12)
	})

	t.Run("upstream segment is compared at its own cut point", func(t *testing.T) {
		b := upstreamCostTestBinding(7)
		// 上游 >100K 起输入 0.6：官方这里还是基础价 5 → 0.12
		b.Intervals = append([]PricingInterval{{MinTokens: 100000, MaxTokens: intPtr(272000), InputPrice: upstreamCostPtr(0.6e-6)}}, b.Intervals...)
		ratio, ok := bindingCostRatio(&entry, &b)
		require.True(t, ok)
		require.InDelta(t, 0.12, ratio, 1e-12)
	})

	t.Run("official segment with blank cache price scales by input ratio", func(t *testing.T) {
		b := upstreamCostTestBinding(7)
		// 官方 >272K 段缓存读留空 → 0.5 × (10 / 5) = 1；上游这段缓存读填 0.9 → 0.9（不折算会是 0.9 / 0.5 = 1.8）
		b.Intervals = []PricingInterval{{MinTokens: 272000, InputPrice: upstreamCostPtr(0.3e-6), OutputPrice: upstreamCostPtr(1.35e-6), CacheReadPrice: upstreamCostPtr(0.9e-6)}}
		ratio, ok := bindingCostRatio(&entry, &b)
		require.True(t, ok)
		require.InDelta(t, 0.9, ratio, 1e-12)
	})

	t.Run("items the official price lacks are skipped", func(t *testing.T) {
		b := upstreamCostTestBinding(7)
		b.CacheWritePrice = upstreamCostPtr(99e-6) // 官方没配缓存写
		ratio, ok := bindingCostRatio(&entry, &b)
		require.True(t, ok)
		require.InDelta(t, 0.03, ratio, 1e-12)
	})

	t.Run("nothing comparable", func(t *testing.T) {
		bare := ModelCatalogEntry{ID: 2, ModelID: "bare"}
		b := upstreamCostTestBinding(7)
		_, ok := bindingCostRatio(&bare, &b)
		require.False(t, ok)
		_, ok = bindingCostRatio(&entry, nil)
		require.False(t, ok)
	})
}

func TestCalculateUpstreamCost(t *testing.T) {
	bs := NewBillingService(nil, nil)
	entry := upstreamCostTestEntry()
	binding := upstreamCostTestBinding(7)
	resolver := newUpstreamCostTestResolver(t, bs, entry, binding)
	found, b := resolver.LookupUpstreamPrice(context.Background(), entry.ModelID, 7)
	require.NotNil(t, b)

	t.Run("base segment", func(t *testing.T) {
		// 1000 × 0.15 + 200 × 0.9 + 3000 × 0.015（$ / 百万 Token）
		cost, err := bs.CalculateUpstreamCost(context.Background(), resolver, found, b,
			UsageTokens{InputTokens: 1000, OutputTokens: 200, CacheReadTokens: 3000}, time.Time{}, "")
		require.NoError(t, err)
		require.InDelta(t, 0.000375, cost, 1e-12)
	})

	t.Run("upstream segment", func(t *testing.T) {
		// 300000 × 0.3 + 1000 × 1.35
		cost, err := bs.CalculateUpstreamCost(context.Background(), resolver, found, b,
			UsageTokens{InputTokens: 300000, OutputTokens: 1000}, time.Time{}, "")
		require.NoError(t, err)
		require.InDelta(t, 0.09135, cost, 1e-12)
	})

	t.Run("not bound", func(t *testing.T) {
		_, missing := resolver.LookupUpstreamPrice(context.Background(), entry.ModelID, 8)
		require.Nil(t, missing)
		_, err := bs.CalculateUpstreamCost(context.Background(), resolver, found, nil, UsageTokens{InputTokens: 1}, time.Time{}, "")
		require.ErrorIs(t, err, ErrUpstreamPriceNotFound)
	})
}

// DeepSeek 的「强制官方价」与高峰加价是官方售价的规则，不能盖掉上游价。
func TestCalculateUpstreamCost_DeepSeekKeepsUpstreamPrice(t *testing.T) {
	bs := NewBillingService(nil, nil)
	entry := ModelCatalogEntry{
		ID: 1, ModelID: "deepseek-v4.1-flash", BillingMode: BillingModeToken, Status: ModelCatalogStatusListed,
		ManagedBy: ModelCatalogManagedBySeed, InputPrice: upstreamCostPtr(0.14e-6), OutputPrice: upstreamCostPtr(0.28e-6),
	}
	binding := ModelCatalogBinding{EntryID: 1, AccountID: 7, InputPrice: 0.01e-6, OutputPrice: 0.02e-6}
	resolver := newUpstreamCostTestResolver(t, bs, entry, binding)
	found, b := resolver.LookupUpstreamPrice(context.Background(), entry.ModelID, 7)
	require.NotNil(t, b)

	// 1,000,000 × 0.01 + 1,000,000 × 0.02
	cost, err := bs.CalculateUpstreamCost(context.Background(), resolver, found, b,
		UsageTokens{InputTokens: 1_000_000, OutputTokens: 1_000_000}, time.Time{}, "")
	require.NoError(t, err)
	require.InDelta(t, 0.03, cost, 1e-12)
}

// 记账：渠道成本按承接关系上的上游价算好落进用量行；没有承接关系的渠道记 0。
func TestGatewayServiceRecordUsage_AccountCostFromUpstreamPrice(t *testing.T) {
	record := func(t *testing.T, accountID int64) *UsageLog {
		usageRepo := &openAIRecordUsageBestEffortLogRepoStub{}
		svc, apiKey := newGatewayRecordUsageServiceWithResolverForTest(usageRepo)
		svc.resolver = newUpstreamCostTestResolver(t, svc.billingService, upstreamCostTestEntry(), upstreamCostTestBinding(7))
		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result: &ForwardResult{
				RequestID: "account_cost_test",
				Usage:     ClaudeUsage{InputTokens: 1000, OutputTokens: 200, CacheReadInputTokens: 3000},
				Model:     "upstream-test-model",
				Duration:  time.Second,
			},
			APIKey:  apiKey,
			User:    &User{ID: 1, RateMultiplier: officialRate(0.1)},
			Account: &Account{ID: accountID, Platform: PlatformAnthropic},
		})
		require.NoError(t, err)
		require.NotNil(t, usageRepo.lastLog)
		return usageRepo.lastLog
	}

	t.Run("bound channel", func(t *testing.T) {
		log := record(t, 7)
		// 官方价：1000 × 5 + 200 × 30 + 3000 × 0.5 = 0.0125；用户付 × 0.1；渠道成本按上游价 0.000375
		require.InDelta(t, 0.0125, log.TotalCost, 1e-12)
		require.InDelta(t, 0.00125, log.ActualCost, 1e-12)
		require.InDelta(t, 0.000375, log.AccountCost, 1e-12)
	})

	t.Run("unbound channel records zero", func(t *testing.T) {
		log := record(t, 8)
		require.InDelta(t, 0.0125, log.TotalCost, 1e-12)
		require.Zero(t, log.AccountCost)
	})
}

// 渠道额度按渠道成本累计（原来是官方价 × 渠道倍率）。
func TestBuildUsageBillingCommand_AccountQuotaUsesAccountCost(t *testing.T) {
	cmd := buildUsageBillingCommand("req-1", &UsageLog{Model: "m"}, &postUsageBillingParams{
		Cost:        &CostBreakdown{TotalCost: 0.0125, ActualCost: 0.00125},
		User:        &User{ID: 1},
		APIKey:      &APIKey{ID: 2},
		Account:     &Account{ID: 3, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 10.0}},
		AccountCost: 0.000375,
	})
	require.NotNil(t, cmd)
	require.InDelta(t, 0.000375, cmd.AccountQuotaCost, 1e-12)
}
