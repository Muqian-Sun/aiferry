package service

// 分组利润控制（配套 migration 192/193 的 groups.profit_* 字段）。
//
// 定位：利润控制是"候选准入过滤"，只决定账号能否进入调度候选池；既有的排序、
// 评分、粘性、熔断、负载均衡在合格账号之间照常工作，本文件不改变它们的行为。
//
// 准入条件：
//
//	U(尝试时刻) <= D(pricingAt) × (1 − profit_min_margin)
//
//   - D（用户售价倍率）= 认证用户的 rate_multiplier（ctx 里由认证中间件放入），
//     与 RecordUsage 完全同源，一个请求不会中途变价。
//   - profit_min_margin（最低毛利率）是全站一档的后台设置。门一直开着：
//     填 0 = 不能亏本（muqian 2026-10-07），U <= D。
//   - 最高推理档（请求的推理强度 = max，见 WithRequestedReasoningEffort）时 U 再乘
//     「上游最高推理倍率 ÷ 售价最高推理倍率」（maxReasoningCostFactor；muqian 2026-10-07：利润门也算进去）。
//   - U（上游成本比）= 这个渠道给这个模型的上游价 ÷ 官方价，逐项、逐段取最高的一个
//     （bindingCostRatio；D3，muqian 2026-09-30）。模型取本请求的目录路由；找不到这个渠道的
//     承接关系（没有上游价）时保守拒绝。
//
// 装门点（gate 随 ctx 传播，请求内复用，覆盖等待/重试/failover/抢槽后终检）：
//   - handler 各文本入口经 WithOpenAIRequestPricingContext 在请求开始统一装门并
//     固定 pricingAt。生图意图只用于能力路由与图片计费，不决定是否装门：混合
//     /v1/responses 请求（含声明了生图工具的请求）的 token 计费部分仍受利润门
//     保护，请求体内的任何工具声明都不能作为关门开关。
//   - 门范围之外的路径显式携带 WithOpenAIProfitControlSuppressed 标记：独立
//     图片/视频端点与 Grok 媒体（媒体计费另有倍率来源）、count_tokens（不计费）、
//     live（按时长计费）。所有装门点（含防御性装门）都尊重该标记。
//   - selectAccountWithScheduler 顶部：唯一文本调度入口的防御性装门（ctx 已有
//     同分组门则复用，保证 failover 阈值稳定）；requiredImageCapability != ""
//     的专用图片调度不装门。
//   - 公开 SelectAccountWithLoadAwareness / SelectAccountByPreviousResponseID：
//     防御性装门，保证不经唯一入口的调用方无法绕过。
//   - 选号结果通过 AccountSelectionResult 携带真实生效的门（composite/fallback
//     路由可能解析出与入口分组不同的门），handler 经
//     ContextWithSelectionProfitGate 重放后再做抢槽后终检与准入后粘性绑定。
//   - 长连接（Responses WS）在每个 turn 开始经 WithOpenAITurnPricingContext
//     重新冻结 pricingAt 并重装门：turn 的准入与计费同源，跨峰谷边界不再共用
//     建连时刻的 D。
//
// 否决点（消费 gate，任何 fallback 都无法把已排除账号重新放回）：
//   - defaultOpenAIAccountScheduler.isAccountRequestCompatibleReason：候选池
//     过滤 + 调度器内抢槽后终检共用，named reason 进入 openAISelectionFilterStats。
//   - isOpenAICompatibleAccountEligibleForRequest：legacy 引擎与 DB recheck 共用。
//   - resolveAccountByPreviousResponseIDForCapability：previous_response 粘连
//     两阶段校验；与 quota auto-pause 同语义，跳过复用但不删除绑定。
//   - handler 槽位获取后终检（OpenAIProfitControlVeto）：快速抢槽与 WaitPlan
//     排队成功后复核，越线则释放槽位、加入本请求排除集重新选号，全池耗尽才
//     返回标准 no available accounts。
//
// 失败语义：设置读取失败时按最低毛利率 0 装门并告警：不要求毛利，但仍不派亏本的渠道；
// 瞬时 DB 抖动不会放大成全站不可调度。
//
// 可观测性：按分组和平台累计装门/threshold 否决/invalid-rate 否决/终检刷新
// 失败计数，≥5 分钟采样输出一条 Info（profit_control_activity），无逐请求日志。

import (
	"context"
	"log/slog"
	"math"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	// profitControlRateEpsilon 吸收 decimal(10,4) 落库与浮点乘法的边界误差：
	// U 与阈值的相对差在该量级内视为相等（即 U == 阈值 判定为合格）。
	profitControlRateEpsilon = 1e-9

	// 进入 openAISelectionFilterStats 的排除原因，全排除时出现在
	// "no available accounts" 的内部统计摘要里（与 quota_auto_pause 等同通道）。
	openAIProfitFilterReasonThreshold            = "profit_threshold"
	openAIProfitFilterReasonMissingUpstreamPrice = "profit_missing_upstream_price"

	// profitControlActivityLogInterval 是按分组采样输出累计计数的最小间隔。
	profitControlActivityLogInterval = 5 * time.Minute
)

type openAIProfitControlGateCtxKey struct{}

// openAIProfitControlSuppressCtxKey 标记本请求显式跳过利润门（独立图片/视频
// 端点、Grok 媒体、count_tokens、live 等利润门范围外流量）。所有装门点看到该
// 标记后一律不装门，防止 service 层防御性装门把边界外流量重新拉回利润过滤。
type openAIProfitControlSuppressCtxKey struct{}

// openAIPricingAtCtxKey 携带请求级定价时刻 pricingAt：门的 D 与 RecordUsage
// 的高峰因子共用，保证一个请求从准入到扣费不中途变价。
type openAIPricingAtCtxKey struct{}

// clampProfitControlThreshold 归一化利润门阈值。设置保存与读取已保证
// min_margin ≤ ProfitControlRatioMax < 1，这里只对存量脏数据兜底：阈值非有限或为负时按 0 处理
// （等价于只放行免费上游）。装门点与 profit-preview 共用，避免口径漂移。
func clampProfitControlThreshold(threshold float64) float64 {
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0 {
		return 0
	}
	return threshold
}

// profitControlOverThreshold 是"上游倍率越线"的唯一定义：相对 epsilon 吸收
// decimal(10,4) 落库与浮点乘法的边界误差，U == 阈值 判定为合格。
// 线上否决点与 profit-preview 共用，两者不得各自实现。
func profitControlOverThreshold(upstream, threshold float64) bool {
	return upstream-threshold > profitControlRateEpsilon*math.Max(1, math.Abs(threshold))
}

// openAIProfitControlGate 是一个请求的利润准入门。除 pricingAt 外全部为预计算
// 标量：候选过滤热路径上每账号只做一次快照解码与一次浮点比较。
type openAIProfitControlGate struct {
	// threshold = D(pricingAt) × (1 − min_margin)，上游成本比必须 <= 它。
	// min_margin 是全站一档的全局设置；同一请求的 failover 重入复用同一个门。
	threshold float64
	// pricingAt 是本请求的统一定价时刻（D 侧）。
	pricingAt time.Time
}

// WithOpenAIRequestPricingContext 在请求开始处装配请求级定价上下文：固定
// pricingAt（返回给调用方，供 RecordUsage 入参共用同一时刻），并按全局设置安装
// 利润门。ctx 携带 WithOpenAIProfitControlSuppressed 标记（门范围外流量）时
// 只固定 pricingAt、不装门。handler 各文本入口应在选号循环前调用一次。
func (s *OpenAIGatewayService) WithOpenAIRequestPricingContext(ctx context.Context) (context.Context, time.Time) {
	pricingAt := timezone.Now()
	ctx = context.WithValue(ctx, openAIPricingAtCtxKey{}, pricingAt)
	return s.withOpenAIProfitControlGate(ctx), pricingAt
}

// WithOpenAIProfitControlSuppressed 标记本请求在利润门范围之外（独立图片/视频
// 端点、Grok 媒体、count_tokens、live）。所有装门点（含 service 层防御性装门）
// 都尊重该标记；它只关闭利润准入过滤，不影响定价上下文与计费。
func WithOpenAIProfitControlSuppressed(ctx context.Context) context.Context {
	return context.WithValue(ctx, openAIProfitControlSuppressCtxKey{}, struct{}{})
}

// WithOpenAITurnPricingContext 在长连接（Responses WS）的每个 turn 开始重新
// 冻结 pricingAt 并按当前设置重装利润门，使 turn 的准入与计费同源：峰前建连
// 保活不再让后续 turn 继续按建连时刻的谷价定价。抑制标记下只刷新 pricingAt。
func (s *OpenAIGatewayService) WithOpenAITurnPricingContext(ctx context.Context) (context.Context, time.Time) {
	pricingAt := timezone.Now()
	ctx = context.WithValue(ctx, openAIPricingAtCtxKey{}, pricingAt)
	if _, suppressed := ctx.Value(openAIProfitControlSuppressCtxKey{}).(struct{}); suppressed {
		return ctx, pricingAt
	}
	gate := s.resolveOpenAIProfitControlGate(ctx)
	if gate == nil {
		// 没有设置服务（测试 / 内部构造）：清除旧 turn 的门，后续 turn 按无门放行，
		// 与 HTTP 路径一致。
		if existing, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && existing != nil {
			return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, (*openAIProfitControlGate)(nil)), pricingAt
		}
		return ctx, pricingAt
	}
	openAIProfitControlObserverInstance.recordInstall(gate.threshold)
	return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, gate), pricingAt
}

// openAIPricingAtFromContext 返回请求级定价时刻；未装配（内部调用、非文本
// 入口）时 ok=false，调用方回退 timezone.Now() 保持既有行为。
func openAIPricingAtFromContext(ctx context.Context) (time.Time, bool) {
	pricingAt, ok := ctx.Value(openAIPricingAtCtxKey{}).(time.Time)
	return pricingAt, ok && !pricingAt.IsZero()
}

// OpenAIPricingAtFromContext 是 handler 侧读取请求级定价时刻的公开入口（未装配
// 时为零值，RecordUsage 对零值回退记录时刻）。供跨函数传递 pricingAt 不便的
// 记录路径直接从请求 ctx 取值。
func OpenAIPricingAtFromContext(ctx context.Context) time.Time {
	pricingAt, _ := openAIPricingAtFromContext(ctx)
	return pricingAt
}

// withOpenAIProfitControlGate 按全局设置把预计算好的准入门装进 ctx。抑制标记、
// 没有设置服务时原样返回 ctx（门不存在，全部否决点自动放行）。ctx 已有门时
// 直接复用：同一请求的全部 failover 重入共享同一阈值。
func (s *OpenAIGatewayService) withOpenAIProfitControlGate(ctx context.Context) context.Context {
	if _, suppressed := ctx.Value(openAIProfitControlSuppressCtxKey{}).(struct{}); suppressed {
		return ctx
	}
	if existing, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && existing != nil {
		return ctx
	}
	gate := s.resolveOpenAIProfitControlGate(ctx)
	if gate == nil {
		return ctx
	}
	openAIProfitControlObserverInstance.recordInstall(gate.threshold)
	return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, gate)
}

func (s *OpenAIGatewayService) resolveOpenAIProfitControlGate(ctx context.Context) *openAIProfitControlGate {
	if s == nil || s.settingService == nil {
		return nil
	}
	settings := s.settingService.GetProfitControlSettings(ctx)

	pricingAt, ok := openAIPricingAtFromContext(ctx)
	if !ok {
		pricingAt = timezone.Now()
	}
	// D = 用户倍率（用户价 = 目录价 × 它），与 RecordUsage 同源；最低毛利率是全站一档。
	downstream := UserRateMultiplierFromContext(ctx)

	threshold := clampProfitControlThreshold(downstream * (1 - settings.MinMargin))
	return &openAIProfitControlGate{
		threshold: threshold,
		pricingAt: pricingAt,
	}
}

// attachSelectionProfitGate 把调度上下文里生效的利润门记录到选号结果上。门在
// 选号函数内部的局部 ctx 上安装（composite/fallback 路由还可能解析出与入口
// 分组不同的门），不随返回值离开调度栈；结果携带后 handler 才能对"真实过滤了
// 候选的那个门"做抢槽后终检与准入后绑定。
func attachSelectionProfitGate(ctx context.Context, sel *AccountSelectionResult) *AccountSelectionResult {
	if sel == nil {
		return nil
	}
	if gate, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && gate != nil {
		sel.profitGate = gate
	}
	return sel
}

// ContextWithSelectionProfitGate 把选号时真实生效的利润门重放到 ctx 上。
// handler 在拿到选号结果后必须用返回的 ctx 做抢槽后终检
// （ProfitControlVetoLatest / GatewayProfitControlVetoLatest）与准入后粘性
// 绑定，否则这两步会因为看不到调度栈内安装的门而退化为空操作。
func ContextWithSelectionProfitGate(ctx context.Context, sel *AccountSelectionResult) context.Context {
	if sel == nil || sel.profitGate == nil {
		return ctx
	}
	if existing, ok := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate); ok && existing == sel.profitGate {
		return ctx
	}
	return context.WithValue(ctx, openAIProfitControlGateCtxKey{}, sel.profitGate)
}

// openAIProfitControlVetoReason 报告利润门是否否决该账号。ctx 中没有门
// （分组未启用利润控制或本请求跳门）或账号为 nil 时一律放行。
func openAIProfitControlVetoReason(ctx context.Context, account *Account) (bool, string) {
	gate, _ := ctx.Value(openAIProfitControlGateCtxKey{}).(*openAIProfitControlGate)
	if gate == nil || account == nil {
		return false, ""
	}
	// 门只装在目录路由的 token 请求上；没有路由就没有可比的官方价，不拦。
	route, ok := CatalogRouteFromContext(ctx)
	if !ok || route.Entry == nil {
		return false, ""
	}
	if rejected, reason := profitGateRejectsBinding(route.Entry, route.Entry.BindingFor(account.ID), gate.threshold, gate.pricingAt, requestIsMaxReasoningEffort(ctx)); rejected {
		openAIProfitControlObserverInstance.recordVeto(gate.threshold, reason)
		return true, reason
	}
	return false, ""
}

// profitGateRejectsBinding 利润门是否跳过这条承接：上游成本比算不出（缺上游价 / 官方价）或超过阈值。
// 按 at 时刻比（muqian 2026-10-06：按请求当时的价判断）：上游忙时涨、售价没涨就在忙时跳过；at 为零值 = 平时。
// maxEffort = 这次请求是最高推理档：上游在 max 档比售价涨得多就在 max 档跳过。
// 调度的否决点与上架提示（SchedulableBindings）共用这一个判断，两边口径不会分叉。
func profitGateRejectsBinding(entry *ModelCatalogEntry, b *ModelCatalogBinding, threshold float64, at time.Time, maxEffort bool) (bool, string) {
	upstream, ok := bindingCostRatioFor(entry, b, at, maxEffort)
	if !ok {
		return true, openAIProfitFilterReasonMissingUpstreamPrice
	}
	if profitControlOverThreshold(upstream, threshold) {
		return true, openAIProfitFilterReasonThreshold
	}
	return false, ""
}

// requestIsMaxReasoningEffort 这次请求要的是最高推理档（handler 在调度前把请求体里的推理强度放进 ctx）。
func requestIsMaxReasoningEffort(ctx context.Context) bool {
	effort := RequestedReasoningEffortFromContext(ctx)
	return effort != nil && NormalizeMaxReasoningEffort(*effort) == "max"
}

// OpenAIProfitControlVeto 是 handler 层槽位获取后终检的公开入口：语义与调度
// 内否决点完全一致。ctx 必须是经 WithOpenAIRequestPricingContext 装配过的
// 请求上下文，否则视为无门放行。
func OpenAIProfitControlVeto(ctx context.Context, account *Account) (bool, string) {
	return openAIProfitControlVetoReason(ctx, account)
}

// ProfitControlVetoLatest performs the handler-side terminal check after a
// concurrency slot is actually acquired. The latest cached account replaces
// the selection snapshot when available; the upstream cost ratio comes from the
// request's catalog route (see openAIProfitControlVetoReason).
func (s *OpenAIGatewayService) ProfitControlVetoLatest(ctx context.Context, selected *Account) (*Account, bool, string) {
	if s == nil {
		return selected, false, ""
	}
	return profitControlVetoLatest(ctx, selected, s.schedulerSnapshot)
}

// ---- 可观测性：全站累计计数 + 采样日志（无逐请求输出） ----
//
// 计数按"每次准入评估"累计，而非每请求：粘性层校验与候选池过滤可能对同一账号
// 各评估一次，failover 重入也会再次计数。计数用于确认门在真实流量上生效及否决
// 构成，不能当作精确的请求数或账号数。

type openAIProfitControlStats struct {
	installs         atomic.Int64
	vetoThreshold    atomic.Int64
	vetoMissingPrice atomic.Int64
	refreshFailures  atomic.Int64
	lastLogUnixMilli atomic.Int64
}

type openAIProfitControlObserver struct {
	stats openAIProfitControlStats
}

var openAIProfitControlObserverInstance = &openAIProfitControlObserver{}

func (o *openAIProfitControlObserver) recordInstall(threshold float64) {
	o.stats.installs.Add(1)
	o.maybeLog(threshold)
}

func (o *openAIProfitControlObserver) recordVeto(threshold float64, reason string) {
	switch reason {
	case openAIProfitFilterReasonThreshold:
		o.stats.vetoThreshold.Add(1)
	case openAIProfitFilterReasonMissingUpstreamPrice:
		o.stats.vetoMissingPrice.Add(1)
	}
	o.maybeLog(threshold)
}

func (o *openAIProfitControlObserver) recordRefreshFailure(threshold float64) {
	o.stats.refreshFailures.Add(1)
	o.maybeLog(threshold)
}

// maybeLog 以 CAS 保证 ≥ profitControlActivityLogInterval 才输出一条
// 累计计数 Info；计数为进程内累计值，用于确认门在真实流量上生效及否决构成。
func (o *openAIProfitControlObserver) maybeLog(threshold float64) {
	s := &o.stats
	now := time.Now().UnixMilli()
	last := s.lastLogUnixMilli.Load()
	if last != 0 && now-last < profitControlActivityLogInterval.Milliseconds() {
		return
	}
	if !s.lastLogUnixMilli.CompareAndSwap(last, now) {
		return
	}
	slog.Info("profit_control_activity",
		"threshold", threshold,
		"installs_total", s.installs.Load(),
		"veto_threshold_total", s.vetoThreshold.Load(),
		"veto_missing_upstream_price_total", s.vetoMissingPrice.Load(),
		"refresh_failure_total", s.refreshFailures.Load(),
	)
}
