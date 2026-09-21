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
	svc := &OpenAIGatewayService{}
	groupID := int64(61)
	now := time.Now()
	expensive := upstreamCostTestAccount(1, UpstreamBillingProbeStatusOK, 0.8, now.Add(-time.Minute), 30*time.Minute)
	profitControlTestAccountWithRate(expensive, 0.8)

	t.Run("installs gate and pricing instant", func(t *testing.T) {
		base := profitControlTestCtx(profitControlTestGroup(groupID, 0.5, 0))
		ctx, pricingAt := svc.WithOpenAIRequestPricingContext(base, &groupID)
		require.False(t, pricingAt.IsZero())
		require.Equal(t, pricingAt, OpenAIPricingAtFromContext(ctx))
		vetoed, reason := OpenAIProfitControlVeto(ctx, expensive)
		require.True(t, vetoed)
		require.Equal(t, openAIProfitFilterReasonThreshold, reason)
	})

	t.Run("suppress marker skips gate everywhere", func(t *testing.T) {
		base := WithOpenAIProfitControlSuppressed(profitControlTestCtx(profitControlTestGroup(groupID, 0.5, 0)))
		ctx, pricingAt := svc.WithOpenAIRequestPricingContext(base, &groupID)
		require.False(t, pricingAt.IsZero(), "跳门时 pricingAt 仍需固定供计费共用")
		vetoed, _ := OpenAIProfitControlVeto(ctx, expensive)
		require.False(t, vetoed)
		// service 层防御性装门也必须被抑制标记挡住。
		reCtx := svc.withOpenAIProfitControlGate(ctx, &groupID)
		vetoed, _ = OpenAIProfitControlVeto(reCtx, expensive)
		require.False(t, vetoed)
	})
}

// failover 重入复用同一门：请求中途分组配置变化不得改变本请求阈值。
func TestProfitControl_GateReuseKeepsThresholdAcrossFailover(t *testing.T) {
	svc := &OpenAIGatewayService{}
	groupID := int64(62)
	group := profitControlTestGroup(groupID, 0.5, 0)
	ctx := svc.withOpenAIProfitControlGate(profitControlTestCtx(group), &groupID)
	gate, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	require.True(t, ok)
	require.InDelta(t, 0.5, gate.threshold, 1e-12)

	// 模拟请求进行中管理员改配置（ctx 分组为同一指针，与 auth 快照语义一致）。
	group.ProfitMinMargin = 0.9
	reCtx := svc.withOpenAIProfitControlGate(ctx, &groupID)
	reGate, ok := reCtx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	require.True(t, ok)
	require.Same(t, gate, reGate, "failover 重入必须复用同一门，阈值不得中途变化")

	// 换分组（composite/模型路由成员调度）重新解析；成员分组无门时必须清除
	// 父分组门，阈值不得跨组泄漏。
	otherID := int64(63)
	otherCtx := svc.withOpenAIProfitControlGate(reCtx, &otherID)
	otherGate, _ := otherCtx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	require.Nil(t, otherGate, "成员分组未启用利润控制时父分组门必须清除")
	now := time.Now()
	expensive := upstreamCostTestAccount(8, UpstreamBillingProbeStatusOK, 0.9, now.Add(-time.Minute), 30*time.Minute)
	vetoed, _ := openAIProfitControlVetoReason(otherCtx, expensive)
	require.False(t, vetoed)
}

// D 固定在 pricingAt：门记录请求开始时刻，一个请求不会中途变价。
func TestProfitControl_GateKeepsPricingAt(t *testing.T) {
	svc := &OpenAIGatewayService{}
	groupID := int64(64)
	group := profitControlTestGroup(groupID, 0, 0)

	pricingAt := time.Date(2026, time.January, 15, 8, 30, 0, 0, timezone.Location())
	ctx := context.WithValue(profitControlTestCtx(group), openAIPricingAtCtxKey{}, pricingAt)
	ctx = WithUserRateMultiplier(ctx, &User{ID: 1, RateMultiplier: 3.0})
	gate := svc.resolveOpenAIProfitControlGate(ctx, &groupID)
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

	group := profitControlTestGroup(77, 0.5, 0)
	group.RateMultiplier = 1
	base := context.WithValue(profitControlTestCtx(group), openAIPricingAtCtxKey{}, now)
	gate := (&OpenAIGatewayService{}).resolveOpenAIProfitControlGate(base, &group.ID)
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
// 配置重装门（区别于请求级同门复用）；已装门时以门所属调度分组为准。
func TestProfitControl_TurnPricingContext(t *testing.T) {
	svc := &OpenAIGatewayService{}
	groupID := int64(63)
	expensive := upstreamCostTestAccount(3, UpstreamBillingProbeStatusOK, 0.8, time.Now().Add(-time.Minute), 30*time.Minute)
	profitControlTestAccountWithRate(expensive, 0.8)

	t.Run("refreshes instant and re-resolves gate config", func(t *testing.T) {
		group := profitControlTestGroup(groupID, 0.5, 0)
		base := profitControlTestCtx(group)
		connCtx, connAt := svc.WithOpenAIRequestPricingContext(base, &groupID)
		vetoed, _ := OpenAIProfitControlVeto(connCtx, expensive)
		require.True(t, vetoed)

		// 连接中途运营者放宽 margin：turn 级重装必须生效（请求级复用不生效）。
		group.ProfitMinMargin = 0.1
		turnCtx, turnAt := svc.WithOpenAITurnPricingContext(connCtx, &groupID)
		require.False(t, turnAt.Before(connAt))
		require.Equal(t, turnAt, OpenAIPricingAtFromContext(turnCtx))
		vetoed, _ = OpenAIProfitControlVeto(turnCtx, expensive)
		require.False(t, vetoed, "turn 级重装应采用最新分组配置")
	})

	t.Run("keeps scheduled group of the existing gate", func(t *testing.T) {
		scheduledGroupID := int64(64)
		scheduled := profitControlTestGroup(scheduledGroupID, 0.5, 0)
		connCtx, _ := svc.WithOpenAIRequestPricingContext(profitControlTestCtx(scheduled), &scheduledGroupID)
		// 入口分组与调度分组不同（composite 成员分组场景）：turn 重装取门的分组。
		entryGroupID := int64(65)
		turnCtx, _ := svc.WithOpenAITurnPricingContext(connCtx, &entryGroupID)
		gate, ok := turnCtx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
		require.True(t, ok)
		require.NotNil(t, gate)
		require.Equal(t, scheduledGroupID, gate.groupID)
	})

	t.Run("suppress marker only refreshes instant", func(t *testing.T) {
		base := WithOpenAIProfitControlSuppressed(profitControlTestCtx(profitControlTestGroup(groupID, 0.5, 0)))
		turnCtx, turnAt := svc.WithOpenAITurnPricingContext(base, &groupID)
		require.False(t, turnAt.IsZero())
		vetoed, _ := OpenAIProfitControlVeto(turnCtx, expensive)
		require.False(t, vetoed)
	})

	t.Run("clears gate when group disables profit control mid-connection", func(t *testing.T) {
		group := profitControlTestGroup(groupID, 0.5, 0)
		connCtx, _ := svc.WithOpenAIRequestPricingContext(profitControlTestCtx(group), &groupID)
		group.ProfitControlEnabled = false
		turnCtx, _ := svc.WithOpenAITurnPricingContext(connCtx, &groupID)
		vetoed, _ := OpenAIProfitControlVeto(turnCtx, expensive)
		require.False(t, vetoed, "关门后 turn 级复核应放行")
	})
}
