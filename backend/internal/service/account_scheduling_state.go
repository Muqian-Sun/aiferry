package service

import (
	"context"
	"strings"
	"time"
)

// SchedulingState 是调度层看到的唯一资源状态：状态服务（RateLimitService）与管理端写原始字段，
// 调度器只通过它判断「现在能不能派给这个资源」，不直接读任何一个原始字段。
// 第三方 key 与成品号同一个对象——种类差异在写方（谁把它停掉、停到什么时候），不在读方。
type SchedulingState struct {
	// Blocked 整体不可调度。
	Blocked bool
	// BlockedUntil 解除时间；nil 且 Blocked 表示直到管理员干预（禁用 / 手动不可调度 / 过期）。
	BlockedUntil *time.Time
	// Reason 不可调度的类别：disabled / unschedulable / expired / overloaded / rate_limited / temp_unschedulable；
	// 可调度时为空串。
	Reason string
	// Detail 写方留下的具体原因（temp_unschedulable_reason），没有时为空串。
	Detail string
	// ModelBlocks 模型级限流：scope → 解除时间，只含此刻仍生效的。
	ModelBlocks map[string]time.Time
	// ModelBlocksWaived 模型级限流被放行：Antigravity 成品号开了 overages 且积分未耗尽，
	// 命中模型级限流仍可派单（积分是厂商能力，按 Vendor 判定）。
	ModelBlocksWaived bool
}

// SchedulingState 从账号字段聚合此刻的调度状态；now 由调用方传，便于测试。
func (a *Account) SchedulingState(now time.Time) SchedulingState {
	state := SchedulingState{}
	if a == nil {
		state.Blocked = true
		state.Reason = "missing"
		return state
	}
	switch {
	case !a.IsActive():
		state.Blocked, state.Reason = true, "disabled"
	case !a.Schedulable:
		state.Blocked, state.Reason = true, "unschedulable"
	case a.ExpiresAt != nil && !now.Before(*a.ExpiresAt):
		state.Blocked, state.Reason = true, "expired"
	case a.OverloadUntil != nil && now.Before(*a.OverloadUntil):
		state.Blocked, state.Reason, state.BlockedUntil = true, "overloaded", cloneTimePtr(a.OverloadUntil)
	case a.RateLimitResetAt != nil && now.Before(*a.RateLimitResetAt):
		state.Blocked, state.Reason, state.BlockedUntil = true, "rate_limited", cloneTimePtr(a.RateLimitResetAt)
	case a.TempUnschedulableUntil != nil && now.Before(*a.TempUnschedulableUntil):
		state.Blocked, state.Reason, state.BlockedUntil = true, "temp_unschedulable", cloneTimePtr(a.TempUnschedulableUntil)
		state.Detail = strings.TrimSpace(a.TempUnschedulableReason)
	}
	state.ModelBlocks = a.activeModelRateLimits(now)
	if len(state.ModelBlocks) > 0 {
		state.ModelBlocksWaived = a.Vendor() == PlatformAntigravity && a.IsOveragesEnabled() && !a.isCreditsExhausted()
	}
	return state
}

// Allows 报告此刻能否把请求派给该资源；scopes 是本次请求命中的模型级限流 scope
// （由 Account.modelRateLimitKeysForRequest 解析，空 = 只看整体）。
func (st SchedulingState) Allows(now time.Time, scopes ...string) bool {
	if st.Blocked {
		return false
	}
	if st.ModelBlocksWaived {
		return true
	}
	for _, scope := range scopes {
		if until, ok := st.ModelBlocks[scope]; ok && now.Before(until) {
			return false
		}
	}
	return true
}

// SchedulingAllows 是调度器的准入判定：整体状态 + 本次请求命中的模型级限流。
// requestedModel 为空表示只看整体。
func (a *Account) SchedulingAllows(ctx context.Context, requestedModel string, now time.Time) bool {
	if a == nil {
		return false
	}
	state := a.SchedulingState(now)
	if requestedModel == "" {
		return state.Allows(now)
	}
	return state.Allows(now, a.modelRateLimitKeysForRequest(ctx, requestedModel)...)
}

// activeModelRateLimits 读出此刻仍生效的模型级限流：scope → 解除时间。
func (a *Account) activeModelRateLimits(now time.Time) map[string]time.Time {
	if a == nil || a.Extra == nil {
		return nil
	}
	rawLimits, ok := a.Extra[modelRateLimitsKey].(map[string]any)
	if !ok || len(rawLimits) == 0 {
		return nil
	}
	var active map[string]time.Time
	for scope := range rawLimits {
		resetAt := a.modelRateLimitResetAt(scope)
		if resetAt == nil || !now.Before(*resetAt) {
			continue
		}
		if active == nil {
			active = make(map[string]time.Time, len(rawLimits))
		}
		active[scope] = *resetAt
	}
	return active
}
