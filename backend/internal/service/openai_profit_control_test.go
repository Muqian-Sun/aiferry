//go:build unit

package service

import (
	"context"
	"math"

	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// profitControlTestService 造一个只带利润门全局设置（最低毛利率；0 = 关）的 OpenAI 网关服务。
func profitControlTestService(t *testing.T, minMargin float64) *OpenAIGatewayService {
	t.Helper()
	return &OpenAIGatewayService{settingService: profitControlTestSettingService(t, minMargin)}
}

// profitControlTestCtx 模拟认证后的请求上下文：D = 用户倍率。
func profitControlTestCtx(userRate float64) context.Context {
	return WithUserRateMultiplier(context.Background(), &User{ID: 1, RateMultiplier: customRate(userRate)})
}

// upstreamCostTestAccount / upstreamCostTestOAuthAccount 只有 unit 标签的用例（本文件与
// openai_profit_control_pricing_test.go）用；放在无标签的 scheduler_test_stubs_test.go 里会被默认标签下的 lint 判成未使用。
func upstreamCostTestAccount(id int64, status string, rate float64, receivedAt time.Time, interval time.Duration) *Account {
	return &Account{
		ID:       id,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			UpstreamBillingProbeExtraKey: map[string]any{
				"status": status,
				"data": map[string]any{
					"billing_scope":             "token",
					"resolved_rate_multiplier":  rate,
					"peak_rate_enabled":         false,
					"effective_rate_multiplier": rate,
				},
				"received_at":     receivedAt.UTC().Format(time.RFC3339Nano),
				"fresh_until":     receivedAt.Add(2 * interval).UTC().Format(time.RFC3339Nano),
				"last_attempt_at": receivedAt.UTC().Format(time.RFC3339Nano),
				"next_probe_at":   receivedAt.Add(interval).UTC().Format(time.RFC3339Nano),
			},
		},
		ProtocolEndpoints: map[string]string{APIProtocolChatCompletions: "https://api.openai.com", APIProtocolResponses: "https://api.openai.com"},
	}
}

func upstreamCostTestOAuthAccount(id int64) *Account {
	return &Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
}

func profitControlTestAccountWithRate(account *Account, rate float64) *Account {
	account.RateMultiplier = &rate
	return account
}

func TestResolveOpenAIProfitControlGate(t *testing.T) {
	t.Run("no setting service yields no gate", func(t *testing.T) {
		svc := &OpenAIGatewayService{}
		require.Nil(t, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)))
	})

	t.Run("zero min margin yields no gate", func(t *testing.T) {
		svc := profitControlTestService(t, 0)
		require.Nil(t, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)))
	})

	t.Run("threshold is the user rate times one minus min margin", func(t *testing.T) {
		svc := profitControlTestService(t, 0.3)
		gate := svc.resolveOpenAIProfitControlGate(profitControlTestCtx(2.0))
		require.NotNil(t, gate)
		require.InDelta(t, 2.0*(1-0.3), gate.threshold, 1e-12)
		require.False(t, gate.pricingAt.IsZero())
	})

	t.Run("no user identity prices at rate 1", func(t *testing.T) {
		svc := profitControlTestService(t, 0.5)
		gate := svc.resolveOpenAIProfitControlGate(context.Background())
		require.NotNil(t, gate)
		require.InDelta(t, 0.5, gate.threshold, 1e-12)
	})

	t.Run("settings change is visible after cache invalidation", func(t *testing.T) {
		svc := profitControlTestService(t, 0.5)
		require.InDelta(t, 0.5, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)).threshold, 1e-12)
		require.NoError(t, svc.settingService.settingRepo.Set(context.Background(), SettingKeyProfitMinMargin, "0.1"))
		require.InDelta(t, 0.5, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)).threshold, 1e-12, "60s 缓存内仍是旧值")
		InvalidateProfitControlSettingsCache()
		require.InDelta(t, 0.9, svc.resolveOpenAIProfitControlGate(profitControlTestCtx(1)).threshold, 1e-12)
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
