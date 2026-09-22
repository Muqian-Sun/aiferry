package service

// 请求级定价与利润门回归：请求级 pricingAt 定价上下文、门复用（failover 阈值稳定）、
// Responses 文本能力利润门、U 使用账号倍率且与探测新鲜度解耦、
// 用量记录定价时刻取值。

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

// WithOpenAIRequestPricingContext：装门 + 固定 pricingAt；显式抑制标记
// （媒体/count_tokens/live 等门范围外路径）跳门且防御性装门无法把门加回来。
func TestProfitControl_RequestPricingContext(t *testing.T) {
	svc := profitControlTestService(t, true, 0.5, 0)
	now := time.Now()
	expensive := upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 0.8, now.Add(-time.Minute), 30*time.Minute)
	profitControlTestAccountWithRate(expensive, 0.8)

	t.Run("installs gate and pricing instant", func(t *testing.T) {
		base := profitControlTestCtx(1)
		ctx, pricingAt := svc.WithOpenAIRequestPricingContext(base)
		require.False(t, pricingAt.IsZero())
		require.Equal(t, pricingAt, OpenAIPricingAtFromContext(ctx))
		vetoed, reason := OpenAIProfitControlVeto(ctx, expensive)
		require.True(t, vetoed)
		require.Equal(t, openAIProfitFilterReasonThreshold, reason)
	})

	t.Run("suppress marker skips gate everywhere", func(t *testing.T) {
		base := WithOpenAIProfitControlSuppressed(profitControlTestCtx(1))
		ctx, pricingAt := svc.WithOpenAIRequestPricingContext(base)
		require.False(t, pricingAt.IsZero(), "跳门时 pricingAt 仍需固定供计费共用")
		vetoed, _ := OpenAIProfitControlVeto(ctx, expensive)
		require.False(t, vetoed)
		// service 层防御性装门也必须被抑制标记挡住。
		reCtx := svc.withOpenAIProfitControlGate(ctx)
		vetoed, _ = OpenAIProfitControlVeto(reCtx, expensive)
		require.False(t, vetoed)
	})
}

// failover 重入复用同一门：请求中途设置变化不得改变本请求阈值。
func TestProfitControl_GateReuseKeepsThresholdAcrossFailover(t *testing.T) {
	svc := profitControlTestService(t, true, 0.5, 0)
	ctx := svc.withOpenAIProfitControlGate(profitControlTestCtx(1))
	gate, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	require.True(t, ok)
	require.InDelta(t, 0.5, gate.threshold, 1e-12)

	// 模拟请求进行中管理员改设置（缓存已失效）。
	require.NoError(t, svc.settingService.settingRepo.Set(context.Background(), SettingKeyProfitMinMargin, "0.9"))
	InvalidateProfitControlSettingsCache()
	reCtx := svc.withOpenAIProfitControlGate(ctx)
	reGate, ok := reCtx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	require.True(t, ok)
	require.Same(t, gate, reGate, "failover 重入必须复用同一门，阈值不得中途变化")
}

// D 固定在 pricingAt：门记录请求开始时刻，一个请求不会中途变价。
func TestProfitControl_GateKeepsPricingAt(t *testing.T) {
	svc := profitControlTestService(t, true, 0, 0)

	pricingAt := time.Date(2026, time.January, 15, 8, 30, 0, 0, timezone.Location())
	ctx := context.WithValue(profitControlTestCtx(3.0), openAIPricingAtCtxKey{}, pricingAt)
	gate := svc.resolveOpenAIProfitControlGate(ctx)
	require.NotNil(t, gate)
	require.InDelta(t, 3.0, gate.threshold, 1e-9, "阈值 = 用户倍率 3.0 × (1-0)")
	require.Equal(t, pricingAt, gate.pricingAt)
}

// U 只取账号倍率：探测快照内容和新鲜度不再直接参与利润判断。
func TestProfitControl_UsesAccountRateInsteadOfProbeSnapshot(t *testing.T) {
	gate := &openAIProfitControlGate{threshold: 0.5, pricingAt: time.Now().Add(-12 * time.Hour)}
	ctx := context.WithValue(context.Background(), openAIProfitControlGateCtxKey{}, gate)
	account := upstreamCostTestAccount(9, UpstreamBillingProbeStatusOK, 0.1, time.Now().Add(-3*time.Hour), 30*time.Minute)
	profitControlTestAccountWithRate(account, 0.8)
	vetoed, reason := openAIProfitControlVetoReason(ctx, account)
	require.True(t, vetoed)
	require.Equal(t, openAIProfitFilterReasonThreshold, reason)
}

// 账号倍率缺失一律视为非法保守拒绝；手工或同步维护了倍率的任意账号类型都按
// 同一阈值判断（OAuth 与 API Key 无差别）。
func TestProfitControl_AccountRateSemantics(t *testing.T) {
	now := time.Now()
	missing := upstreamCostTestOAuthAccount(2)
	manualOAuth := profitControlTestAccountWithRate(upstreamCostTestOAuthAccount(3), 0.3)
	expensive := profitControlTestAccountWithRate(upstreamCostTestAccount(4, UpstreamBillingProbeStatusOK, 0.1, now.Add(-3*time.Hour), 30*time.Minute), 0.8)

	base := context.WithValue(profitControlTestCtx(1), openAIPricingAtCtxKey{}, now)
	gate := profitControlTestService(t, true, 0.5, 0).resolveOpenAIProfitControlGate(base)
	require.NotNil(t, gate)
	gateCtx := context.WithValue(base, openAIProfitControlGateCtxKey{}, gate)

	vetoed, reason := openAIProfitControlVetoReason(gateCtx, missing)
	require.True(t, vetoed, "缺失账号倍率必须保守拒绝")
	require.Equal(t, openAIProfitFilterReasonInvalidAccountRate, reason)

	vetoed, _ = openAIProfitControlVetoReason(gateCtx, manualOAuth)
	require.False(t, vetoed, "手工维护的 OAuth 倍率应正常准入")

	vetoed, reason = openAIProfitControlVetoReason(gateCtx, expensive)
	require.True(t, vetoed)
	require.Equal(t, openAIProfitFilterReasonThreshold, reason)
}

// 用量记录定价时刻：优先请求级 PricingAt，未装配回退记录时刻。
func TestOpenAIUsagePricingAt(t *testing.T) {
	fixed := time.Now().Add(-2 * time.Hour)
	require.Equal(t, fixed, openAIUsagePricingAt(&OpenAIRecordUsageInput{PricingAt: fixed}))
	fallback := openAIUsagePricingAt(&OpenAIRecordUsageInput{})
	require.WithinDuration(t, timezone.Now(), fallback, 5*time.Second)
	require.WithinDuration(t, timezone.Now(), openAIUsagePricingAt(nil), 5*time.Second)
}

// WithOpenAITurnPricingContext：长连接 turn 边界重新冻结 pricingAt 并按当前
// 设置重装门（区别于请求级同门复用）。
func TestProfitControl_TurnPricingContext(t *testing.T) {
	expensive := upstreamCostTestAccount(3, UpstreamBillingProbeStatusOK, 0.8, time.Now().Add(-time.Minute), 30*time.Minute)
	profitControlTestAccountWithRate(expensive, 0.8)

	t.Run("refreshes instant and re-resolves gate config", func(t *testing.T) {
		svc := profitControlTestService(t, true, 0.5, 0)
		connCtx, connAt := svc.WithOpenAIRequestPricingContext(profitControlTestCtx(1))
		vetoed, _ := OpenAIProfitControlVeto(connCtx, expensive)
		require.True(t, vetoed)

		// 连接中途运营者放宽 margin：turn 级重装必须生效（请求级复用不生效）。
		require.NoError(t, svc.settingService.settingRepo.Set(context.Background(), SettingKeyProfitMinMargin, "0.1"))
		InvalidateProfitControlSettingsCache()
		turnCtx, turnAt := svc.WithOpenAITurnPricingContext(connCtx)
		require.False(t, turnAt.Before(connAt))
		require.Equal(t, turnAt, OpenAIPricingAtFromContext(turnCtx))
		vetoed, _ = OpenAIProfitControlVeto(turnCtx, expensive)
		require.False(t, vetoed, "turn 级重装应采用最新设置")
	})

	t.Run("suppress marker only refreshes instant", func(t *testing.T) {
		svc := profitControlTestService(t, true, 0.5, 0)
		base := WithOpenAIProfitControlSuppressed(profitControlTestCtx(1))
		turnCtx, turnAt := svc.WithOpenAITurnPricingContext(base)
		require.False(t, turnAt.IsZero())
		vetoed, _ := OpenAIProfitControlVeto(turnCtx, expensive)
		require.False(t, vetoed)
	})

	t.Run("clears gate when profit control is disabled mid-connection", func(t *testing.T) {
		svc := profitControlTestService(t, true, 0.5, 0)
		connCtx, _ := svc.WithOpenAIRequestPricingContext(profitControlTestCtx(1))
		require.NoError(t, svc.settingService.settingRepo.Set(context.Background(), SettingKeyProfitControlEnabled, "false"))
		InvalidateProfitControlSettingsCache()
		turnCtx, _ := svc.WithOpenAITurnPricingContext(connCtx)
		vetoed, _ := OpenAIProfitControlVeto(turnCtx, expensive)
		require.False(t, vetoed, "关门后 turn 级复核应放行")
	})
}
