package service

import (
	"context"
	"math"

	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func profitControlTestGroup(id int64, margin, buffer float64) *Group {
	return &Group{
		ID:                   id,
		Platform:             PlatformOpenAI,
		Status:               StatusActive,
		Hydrated:             true,
		RateMultiplier:       1.0,
		SubscriptionType:     SubscriptionTypeStandard,
		ProfitControlEnabled: true,
		ProfitMinMargin:      margin,
		ProfitSafetyBuffer:   buffer,
	}
}

// profitControlTestCtx 模拟认证后的请求上下文：D 取用户倍率（用夹具分组的数当用户倍率）。
func profitControlTestCtx(group *Group) context.Context {
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	return WithUserRateMultiplier(ctx, &User{ID: 1, RateMultiplier: group.RateMultiplier})
}

func profitControlTestAccountWithRate(account *Account, rate float64) *Account {
	account.RateMultiplier = &rate
	return account
}

func TestResolveOpenAIProfitControlGate(t *testing.T) {
	svc := &OpenAIGatewayService{}
	groupID := int64(7)

	t.Run("nil group id yields no gate", func(t *testing.T) {
		require.Nil(t, svc.resolveOpenAIProfitControlGate(context.Background(), nil))
	})

	t.Run("no ctx group and no snapshot yields no gate", func(t *testing.T) {
		require.Nil(t, svc.resolveOpenAIProfitControlGate(context.Background(), &groupID))
	})

	t.Run("disabled group yields no gate", func(t *testing.T) {
		group := profitControlTestGroup(groupID, 0.3, 0)
		group.ProfitControlEnabled = false
		require.Nil(t, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(group), &groupID))
	})

	t.Run("non openai or grok platform yields no gate even if enabled", func(t *testing.T) {
		group := profitControlTestGroup(groupID, 0.3, 0)
		group.Platform = PlatformAnthropic
		require.Nil(t, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(group), &groupID))
	})

	t.Run("grok group routed through openai handler installs gate", func(t *testing.T) {
		group := profitControlTestGroup(groupID, 0.3, 0.05)
		group.Platform = PlatformGrok
		group.RateMultiplier = 0.5
		gate := svc.resolveOpenAIProfitControlGate(profitControlTestCtx(group), &groupID)
		require.NotNil(t, gate)
		require.Equal(t, PlatformGrok, gate.platform)
		require.InDelta(t, 0.5*(1-0.35), gate.threshold, 1e-12)
	})

	t.Run("ctx group id mismatch without snapshot yields no gate", func(t *testing.T) {
		group := profitControlTestGroup(groupID+1, 0.3, 0)
		require.Nil(t, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(group), &groupID))
	})

	t.Run("threshold composes margin and buffer from downstream rate", func(t *testing.T) {
		group := profitControlTestGroup(groupID, 0.3, 0.05)
		group.RateMultiplier = 2.0
		gate := svc.resolveOpenAIProfitControlGate(profitControlTestCtx(group), &groupID)
		require.NotNil(t, gate)
		require.InDelta(t, 2.0*(1-0.35), gate.threshold, 1e-12)
		require.Equal(t, PlatformOpenAI, gate.platform)
		require.False(t, gate.pricingAt.IsZero())
		require.Equal(t, groupID, gate.groupID)
	})

	t.Run("threshold uses the user rate multiplier exactly like billing", func(t *testing.T) {
		group := profitControlTestGroup(groupID, 0.5, 0)
		ctx := WithUserRateMultiplier(profitControlTestCtx(group), &User{ID: 1, RateMultiplier: 3.0})
		gate := svc.resolveOpenAIProfitControlGate(ctx, &groupID)
		require.NotNil(t, gate)
		require.InDelta(t, 3.0*0.5, gate.threshold, 1e-9)
		require.Equal(t, PlatformOpenAI, gate.platform)
	})
}

func TestOpenAIProfitControlVetoReason(t *testing.T) {
	now := time.Now()
	gateCtx := func(threshold float64) context.Context {
		return context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, &openAIProfitControlGate{
			threshold: threshold,
			pricingAt: now,
		})
	}

	t.Run("no gate admits everything", func(t *testing.T) {
		vetoed, reason := openAIProfitControlVetoReason(context.Background(), upstreamCostTestOAuthAccount(1))
		require.False(t, vetoed)
		require.Empty(t, reason)
	})

	t.Run("fresh rate below threshold admits", func(t *testing.T) {
		account := profitControlTestAccountWithRate(upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 99, now.Add(-time.Minute), 30*time.Minute), 0.5)
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0.7), account)
		require.False(t, vetoed)
	})

	t.Run("rate exactly at threshold admits via epsilon", func(t *testing.T) {
		account := profitControlTestAccountWithRate(upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 99, now.Add(-time.Minute), 30*time.Minute), 0.7)
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0.7), account)
		require.False(t, vetoed)
	})

	t.Run("rate within float noise above threshold admits", func(t *testing.T) {
		account := profitControlTestAccountWithRate(upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 99, now.Add(-time.Minute), 30*time.Minute), 0.7+1e-12)
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0.7), account)
		require.False(t, vetoed)
	})

	t.Run("rate above threshold is vetoed", func(t *testing.T) {
		account := profitControlTestAccountWithRate(upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 0.1, now.Add(-time.Minute), 30*time.Minute), 0.8)
		vetoed, reason := openAIProfitControlVetoReason(gateCtx(0.7), account)
		require.True(t, vetoed)
		require.Equal(t, openAIProfitFilterReasonThreshold, reason)
	})

	t.Run("zero threshold only admits free upstream", func(t *testing.T) {
		free := profitControlTestAccountWithRate(upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 99, now.Add(-time.Minute), 30*time.Minute), 0)
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0), free)
		require.False(t, vetoed)
		paid := profitControlTestAccountWithRate(upstreamCostTestAccount(2, UpstreamBillingProbeStatusOK, 0, now.Add(-time.Minute), 30*time.Minute), 0.01)
		vetoed, reason := openAIProfitControlVetoReason(gateCtx(0), paid)
		require.True(t, vetoed)
		require.Equal(t, openAIProfitFilterReasonThreshold, reason)
	})

	t.Run("missing account rate is invalid", func(t *testing.T) {
		vetoed, reason := openAIProfitControlVetoReason(gateCtx(0.7), upstreamCostTestOAuthAccount(1))
		require.True(t, vetoed)
		require.Equal(t, openAIProfitFilterReasonInvalidAccountRate, reason)
	})

	t.Run("oauth account with manual rate is priceable", func(t *testing.T) {
		account := profitControlTestAccountWithRate(upstreamCostTestOAuthAccount(1), 0.2)
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0.7), account)
		require.False(t, vetoed)
	})

	t.Run("stale probe does not affect manual account rate", func(t *testing.T) {
		account := profitControlTestAccountWithRate(upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 99, now.Add(-3*time.Hour), 30*time.Minute), 0.1)
		vetoed, _ := openAIProfitControlVetoReason(gateCtx(0.7), account)
		require.False(t, vetoed)
	})

	t.Run("negative and non-finite rates are invalid", func(t *testing.T) {
		for _, rate := range []float64{-1, math.NaN(), math.Inf(1)} {
			account := profitControlTestAccountWithRate(upstreamCostTestOAuthAccount(1), rate)
			vetoed, reason := openAIProfitControlVetoReason(gateCtx(0.7), account)
			require.True(t, vetoed)
			require.Equal(t, openAIProfitFilterReasonInvalidAccountRate, reason)
		}
	})
}

func TestValidateProfitControlConfig(t *testing.T) {
	require.NoError(t, ValidateProfitControlConfig(PlatformAnthropic, false, 0, 0))
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformGrok, PlatformAntigravity} {
		require.NoError(t, ValidateProfitControlConfig(platform, true, 0.3, 0.05))
		require.NoError(t, ValidateProfitControlConfig(platform, true, 0, 0))
	}

	require.Error(t, ValidateProfitControlConfig(PlatformComposite, true, 0.3, 0))
	require.Error(t, ValidateProfitControlConfig(PlatformOpenAI, true, -0.1, 0))
	require.Error(t, ValidateProfitControlConfig(PlatformOpenAI, true, 1.0, 0))
	require.Error(t, ValidateProfitControlConfig(PlatformOpenAI, true, 0, 1.0))
	require.Error(t, ValidateProfitControlConfig(PlatformOpenAI, true, 0.6, 0.4))
}

func TestNormalizeProfitControlConfig(t *testing.T) {
	t.Run("unsupported platform resets everything", func(t *testing.T) {
		enabled, margin, buffer := NormalizeProfitControlConfig(PlatformComposite, true, 0.3, 0.1)
		require.False(t, enabled)
		require.Zero(t, margin)
		require.Zero(t, buffer)
	})

	t.Run("all five platforms retain configuration", func(t *testing.T) {
		for _, platform := range []string{PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformGrok, PlatformAntigravity} {
			enabled, margin, buffer := NormalizeProfitControlConfig(platform, true, 0.3, 0.1)
			require.True(t, enabled)
			require.InDelta(t, 0.3, margin, 1e-12)
			require.InDelta(t, 0.1, buffer, 1e-12)
		}
	})

	t.Run("openai disabled keeps legal values and cleans dirty ones", func(t *testing.T) {
		enabled, margin, buffer := NormalizeProfitControlConfig(PlatformOpenAI, false, 0.3, 0.05)
		require.False(t, enabled)
		require.InDelta(t, 0.3, margin, 1e-12)
		require.InDelta(t, 0.05, buffer, 1e-12)

		_, margin, buffer = NormalizeProfitControlConfig(PlatformOpenAI, false, -1, 1.5)
		require.Zero(t, margin)
		require.Zero(t, buffer)
	})

	t.Run("openai enabled passes through for validation", func(t *testing.T) {
		enabled, margin, buffer := NormalizeProfitControlConfig(PlatformOpenAI, true, 0.3, 0.05)
		require.True(t, enabled)
		require.InDelta(t, 0.3, margin, 1e-12)
		require.InDelta(t, 0.05, buffer, 1e-12)
	})
}
