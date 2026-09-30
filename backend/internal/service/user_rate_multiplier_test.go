//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserRateMultiplier(t *testing.T) {
	require.InDelta(t, 1.0/15, NewUserRateMultiplier, 1e-15, "全站默认倍率 = 官方价的十五分之一")
	require.Equal(t, NewUserRateMultiplier, UserRateMultiplier(&User{}), "没单独设 = 跟全站默认")
	require.Equal(t, 1.5, UserRateMultiplier(&User{RateMultiplier: customRate(1.5)}))
	require.Equal(t, 0.0, UserRateMultiplier(&User{RateMultiplier: customRate(0)}))
	require.Equal(t, 0.0, UserRateMultiplier(&User{RateMultiplier: customRate(-1)}), "negative leaks clamp to free")
	require.Equal(t, 1.0, UserRateMultiplierFromContext(context.Background()), "no authenticated user → 1")
	require.Equal(t, 0.5, UserRateMultiplierFromContext(WithUserRateMultiplier(context.Background(), &User{RateMultiplier: customRate(0.5)})))
}

// 用户价 = 目录价 × users.rate_multiplier：分组上的倍率 / 峰值 / config 默认倍率都不参与。
func TestRecordUsage_ChargesCatalogPriceTimesUserMultiplier(t *testing.T) {
	inputPrice, outputPrice := 1e-6, 2e-6
	tokens := ClaudeUsage{InputTokens: 100, OutputTokens: 50}
	expected := (100*inputPrice + 50*outputPrice) * 2 // 4e-4

	t.Run("anthropic gateway", func(t *testing.T) {
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		userRepo := &openAIRecordUsageUserRepoStub{}
		svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{})
		svc.resolver = newResolverWithCatalogCards(svc.billingService, PricingCard{
			Models: []string{"claude-sonnet-4-5"}, BillingMode: BillingModeToken,
			InputPrice: &inputPrice, OutputPrice: &outputPrice,
		})
		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result:  &ForwardResult{RequestID: "user-rate-gw", Model: "claude-sonnet-4-5", Usage: tokens, Duration: time.Second},
			APIKey:  &APIKey{ID: 1},
			User:    &User{ID: 2, RateMultiplier: customRate(2)},
			Account: &Account{ID: 3, Platform: PlatformAnthropic},
		})
		require.NoError(t, err)
		require.InDelta(t, expected, usageRepo.lastLog.ActualCost, 1e-12)
		require.InDelta(t, 2, usageRepo.lastLog.RateMultiplier, 1e-12)
		require.InDelta(t, expected, userRepo.lastAmount, 1e-12)
	})

	t.Run("openai gateway", func(t *testing.T) {
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		userRepo := &openAIRecordUsageUserRepoStub{}
		svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{})
		svc.resolver = newResolverWithCatalogCards(svc.billingService, PricingCard{
			Models: []string{"gpt-5.6"}, BillingMode: BillingModeToken,
			InputPrice: &inputPrice, OutputPrice: &outputPrice,
		})
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result:  &OpenAIForwardResult{RequestID: "user-rate-oa", Model: "gpt-5.6", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}, Duration: time.Second},
			APIKey:  &APIKey{ID: 1},
			User:    &User{ID: 2, RateMultiplier: customRate(2)},
			Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
		})
		require.NoError(t, err)
		require.InDelta(t, expected, usageRepo.lastLog.ActualCost, 1e-12)
		require.InDelta(t, 2, usageRepo.lastLog.RateMultiplier, 1e-12)
		require.InDelta(t, expected, userRepo.lastAmount, 1e-12)
	})

	// 绝大多数用户没单独设倍率：按全站默认（官方价的 1/15）计费，用量里记生效倍率
	t.Run("default user pays official price × 1/15", func(t *testing.T) {
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		userRepo := &openAIRecordUsageUserRepoStub{}
		svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{})
		svc.resolver = newResolverWithCatalogCards(svc.billingService, PricingCard{
			Models: []string{"gpt-5.6"}, BillingMode: BillingModeToken,
			InputPrice: &inputPrice, OutputPrice: &outputPrice,
		})
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result:  &OpenAIForwardResult{RequestID: "user-rate-default", Model: "gpt-5.6", Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}, Duration: time.Second},
			APIKey:  &APIKey{ID: 1},
			User:    &User{ID: 2},
			Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
		})
		require.NoError(t, err)
		require.InDelta(t, (100*inputPrice+50*outputPrice)/15, usageRepo.lastLog.ActualCost, 1e-15)
		require.InDelta(t, 1.0/15, usageRepo.lastLog.RateMultiplier, 1e-12)
	})

	t.Run("zero multiplier is free", func(t *testing.T) {
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		userRepo := &openAIRecordUsageUserRepoStub{}
		svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{})
		svc.resolver = newResolverWithCatalogCards(svc.billingService, PricingCard{
			Models: []string{"claude-sonnet-4-5"}, BillingMode: BillingModeToken,
			InputPrice: &inputPrice, OutputPrice: &outputPrice,
		})
		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result:  &ForwardResult{RequestID: "user-rate-free", Model: "claude-sonnet-4-5", Usage: tokens, Duration: time.Second},
			APIKey:  &APIKey{ID: 1},
			User:    &User{ID: 2, RateMultiplier: customRate(0)},
			Account: &Account{ID: 3, Platform: PlatformAnthropic},
		})
		require.NoError(t, err)
		require.Zero(t, usageRepo.lastLog.ActualCost)
		require.InDelta(t, 100*inputPrice+50*outputPrice, usageRepo.lastLog.TotalCost, 1e-12, "list price is still recorded")
	})
}
