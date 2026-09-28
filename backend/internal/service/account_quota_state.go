package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// 本文件是资源额度的状态评估：把「这个资源的额度到没到该停调的程度」在状态写入点算清楚，
// 写成账号状态（temp_unschedulable / 模型级限流）。调度器不评估额度，只读 SchedulingState。
// 第三方 key 与成品号走同一个入口，差别只在各自的额度来源（上游快照 / 自己的计数 / 本地用量）。

const (
	openAIQuotaAutoPauseSource = "openai_quota_auto_pause"
	grokQuotaAutoPauseSource   = "grok_quota_auto_pause"
	quotaCounterSource         = "quota_counter"
	// quotaCounterIndefinitePause 总额度超限没有自动解除点：停到管理员重置配额（ResetAccountQuota 会清掉），
	// 用一个足够远的时间表达「直到干预」。
	quotaCounterIndefinitePause = 100 * 365 * 24 * time.Hour
	// quotaPauseFallback 快照里取不到窗口重置时间时的停调时长（拍的）：靠下一次快照写入重评。
	quotaPauseFallback = 5 * time.Minute
)

// ApplyAccountQuotaState 在账号的额度状态变化时评估「该不该停调」，把结论写成账号状态。
//
// 调用点是状态写入点：上游额度快照落库后（Codex 5h/7d 头与探测、xAI 配额与账单探测、
// Anthropic 会话窗口与被动用量采样、国产供应商额度 / 余额）和用量入账后（ApplyAccountUsageState）。
// account 必须带上刚写入的字段（调用方在内存对象上改完 Extra 再传；只有 ID 的先 GetByID）。
// 返回账号此刻是否处于本函数写下的停调。
func (s *RateLimitService) ApplyAccountQuotaState(ctx context.Context, account *Account) bool {
	if s == nil || s.accountRepo == nil || account == nil || account.ID <= 0 {
		return false
	}
	if !account.IsActive() || !account.Schedulable {
		return false
	}
	if s.ApplyAccountSchedulingThreshold(ctx, account) {
		return true
	}
	now := time.Now()
	if until, reason, paused := openAIQuotaPauseDecision(account, s.openAIQuotaAutoPauseSettings(ctx), now); paused {
		return s.pauseAccountUntil(ctx, account, until, reason, openAIQuotaAutoPauseSource)
	}
	if until, reason, paused := grokQuotaPauseDecision(account, now); paused {
		return s.pauseAccountUntil(ctx, account, until, reason, grokQuotaAutoPauseSource)
	}
	if until, reason, paused := quotaCounterPauseDecision(account, now); paused {
		return s.pauseAccountUntil(ctx, account, until, reason, quotaCounterSource)
	}
	return false
}

// ApplyAccountQuotaStateAfterExtraUpdate 是快照写入点的钩子：把刚落库的 Extra 更新合并到内存对象后评估。
// 调用方持有的 account 往往还是写入前的样子，不合并就会按旧快照评估。
func (s *RateLimitService) ApplyAccountQuotaStateAfterExtraUpdate(ctx context.Context, account *Account, updates map[string]any) bool {
	if account == nil {
		return false
	}
	if len(updates) > 0 {
		if account.Extra == nil {
			account.Extra = make(map[string]any, len(updates))
		}
		for key, value := range updates {
			account.Extra[key] = value
		}
	}
	return s.ApplyAccountQuotaState(ctx, account)
}

// ApplyAccountQuotaStateByID 是只持有账号 ID 的写入点的钩子：重新读一遍账号再评估。
func (s *RateLimitService) ApplyAccountQuotaStateByID(ctx context.Context, accountID int64) bool {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return false
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return false
	}
	return s.ApplyAccountQuotaState(ctx, account)
}

func (s *RateLimitService) openAIQuotaAutoPauseSettings(ctx context.Context) OpsOpenAIAccountQuotaAutoPauseSettings {
	if s == nil || s.settingService == nil {
		return OpsOpenAIAccountQuotaAutoPauseSettings{}
	}
	return s.settingService.GetOpenAIQuotaAutoPauseSettings(ctx)
}

// pauseAccountUntil 把「停调到 until」写成账号状态：内存对象、运行时阻断器、DB、temp-unsched 缓存。
// reason 是 BuildTempUnschedReasonPayload 生成的 JSON；同一 until 与同一原因已写过就不重复写。
// 已被别的原因停调的账号不覆盖（先到先得，解除后下一次写入再评）。返回账号是否处于停调。
func (s *RateLimitService) pauseAccountUntil(ctx context.Context, account *Account, until time.Time, reason, source string) bool {
	if s == nil || s.accountRepo == nil || account == nil || account.ID <= 0 || !until.After(time.Now()) {
		return false
	}
	if accountHasSamePause(account, until, reason) {
		return true
	}
	if !account.SchedulingState(time.Now()).Allows(time.Now()) {
		return false
	}

	account.TempUnschedulableUntil = cloneTimePtr(&until)
	account.TempUnschedulableReason = reason
	s.notifyAccountSchedulingBlocked(account, until, source)

	if err := s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason); err != nil {
		slog.Warn("account_quota_state_set_temp_unsched_failed",
			"account_id", account.ID,
			"source", source,
			"until", until.UTC(),
			"error", err)
	} else if s.tempUnschedCache != nil {
		if state := tempUnschedStateFromStoredReason(reason, until.Unix()); state != nil {
			if err := s.tempUnschedCache.SetTempUnsched(ctx, account.ID, state); err != nil {
				slog.Warn("account_quota_state_cache_set_failed", "account_id", account.ID, "error", err)
			}
		}
	}

	slog.Info("account_quota_state_temp_unschedulable",
		"account_id", account.ID,
		"source", source,
		"until", until.UTC())
	return true
}

// accountHasSamePause 报告账号是否已经因同一原因停调到同一时刻（触发时间戳不计）。
func accountHasSamePause(account *Account, until time.Time, reason string) bool {
	if account == nil || account.TempUnschedulableUntil == nil {
		return false
	}
	if account.TempUnschedulableUntil.UTC().Unix() != until.UTC().Unix() {
		return false
	}
	existing, ok := parseTempUnschedReasonPayload(account.TempUnschedulableReason)
	if !ok {
		return false
	}
	next, ok := parseTempUnschedReasonPayload(reason)
	if !ok {
		return false
	}
	existing.TriggeredAtUnix = 0
	next.TriggeredAtUnix = 0
	return existing == next
}

// openAIQuotaPauseDecision 按 Codex 用量快照（codex_5h/7d_*）判断账号是否该停调，停到赢的那个窗口重置。
//
// 适用 OpenAI 成品号，以及任意平台标签的第三方 key：key 的快照只可能来自上游回传的
// x-codex-* 响应头（透传中转），有快照就按阈值停，与它选的平台标签无关。
// 自动用卡有独立阈值：达到消费阈值时必须先退出调度；仅达到普通停调阈值时，
// 只有新鲜状态明确存在可用卡才继续放行到消费阈值。
func openAIQuotaPauseDecision(account *Account, settings OpsOpenAIAccountQuotaAutoPauseSettings, now time.Time) (time.Time, string, bool) {
	if account == nil || (!account.IsThirdPartyKey() && !account.IsOpenAI()) {
		return time.Time{}, "", false
	}
	pause := func(window string, threshold, utilization float64, detail string) (time.Time, string, bool) {
		until, ok := openAICodexWindowResetAt(account.Extra, window)
		if !ok || !until.After(now) {
			until = now.Add(quotaPauseFallback)
		}
		message := fmt.Sprintf("codex %s window %.1f%% used >= %.1f%% (%s); paused until %s",
			window, utilization*100, threshold*100, detail, until.UTC().Format(time.RFC3339))
		return until, BuildTempUnschedReasonPayload(openAIQuotaAutoPauseSource, message), true
	}
	disabled5h := resolveAccountExtraBool(account.Extra, "auto_pause_5h_disabled")
	disabled7d := resolveAccountExtraBool(account.Extra, "auto_pause_7d_disabled")
	if config := ResolveOpenAIAutoResetCreditConfig(account); config.Enabled {
		utilization5h, has5h := resolveOpenAIQuotaUtilization(account.Extra, "5h", now)
		utilization7d, has7d := resolveOpenAIQuotaUtilization(account.Extra, "7d", now)
		if has5h && utilization5h >= config.Threshold5h {
			notifyOpenAIAutoReset(account.ID)
			return pause("5h", config.Threshold5h, utilization5h, "quota_auto_reset_pending_5h")
		}
		if has7d && utilization7d >= config.Threshold7d {
			notifyOpenAIAutoReset(account.ID)
			return pause("7d", config.Threshold7d, utilization7d, "quota_auto_reset_pending_7d")
		}

		pause5h, pause7d := resolveOpenAIQuotaAutoPauseThresholds(account, settings)
		pauseReached5h := !disabled5h && pause5h > 0 && has5h && utilization5h >= pause5h
		pauseReached7d := !disabled7d && pause7d > 0 && has7d && utilization7d >= pause7d
		if pauseReached5h || pauseReached7d {
			state := openAIAutoResetStateFromExtra(account.Extra)
			if state != nil && state.Status == OpenAIAutoResetStatusAvailable && state.AvailableCount > 0 && !openAIAutoResetStateStale(state, now) {
				return time.Time{}, "", false
			}
			notifyOpenAIAutoReset(account.ID)
			if pauseReached5h {
				return pause("5h", pause5h, utilization5h, "quota_auto_reset_credit_check_5h")
			}
			return pause("7d", pause7d, utilization7d, "quota_auto_reset_credit_check_7d")
		}
	}
	// 账号级显式禁用优先于全局默认阈值：账号阈值留空表示「用全局默认」，
	// 没有禁用开关管理员就无法把单个账号从自动停调里豁免出来；开关按窗口分开。
	threshold5h, threshold7d := resolveOpenAIQuotaAutoPauseThresholds(account, settings)
	if !disabled5h && threshold5h > 0 {
		if utilization, ok := resolveOpenAIQuotaUtilization(account.Extra, "5h", now); ok && utilization >= threshold5h {
			return pause("5h", threshold5h, utilization, "quota_auto_pause")
		}
	}
	if !disabled7d && threshold7d > 0 {
		if utilization, ok := resolveOpenAIQuotaUtilization(account.Extra, "7d", now); ok && utilization >= threshold7d {
			return pause("7d", threshold7d, utilization, "quota_auto_pause")
		}
	}
	return time.Time{}, "", false
}

// resolveOpenAIQuotaAutoPauseThresholds 返回账号生效的 5h / 7d 自动停调阈值：账号自己设了就用账号的，
// 否则回到全局默认。
func resolveOpenAIQuotaAutoPauseThresholds(account *Account, settings OpsOpenAIAccountQuotaAutoPauseSettings) (float64, float64) {
	threshold5h, _ := resolveAccountExtraNumber(account.Extra, "auto_pause_5h_threshold")
	threshold7d, _ := resolveAccountExtraNumber(account.Extra, "auto_pause_7d_threshold")
	threshold5h = clamp01(threshold5h)
	threshold7d = clamp01(threshold7d)
	if threshold5h > 0 && threshold7d > 0 {
		return threshold5h, threshold7d
	}
	if threshold5h <= 0 {
		threshold5h = clamp01(settings.DefaultThreshold5h)
	}
	if threshold7d <= 0 {
		threshold7d = clamp01(settings.DefaultThreshold7d)
	}
	return threshold5h, threshold7d
}

// grokQuotaPauseDecision 按 xAI 配额快照判断 Grok 成品号是否该停调：retry_after 生效期内、
// 或请求 / token 窗口用尽（到窗口重置）。快照过期不作数。
func grokQuotaPauseDecision(account *Account, now time.Time) (time.Time, string, bool) {
	if account == nil || !account.IsGrok() || account.Type != AccountTypeOAuth {
		return time.Time{}, "", false
	}
	snapshot, err := grokQuotaSnapshotFromExtra(account.Extra)
	if err != nil || snapshot == nil {
		return time.Time{}, "", false
	}
	if grokQuotaSnapshotStaleForPause(snapshot, now) {
		return time.Time{}, "", false
	}
	pause := func(until time.Time, message string) (time.Time, string, bool) {
		if !until.After(now) {
			until = now.Add(quotaPauseFallback)
		}
		return until, BuildTempUnschedReasonPayload(grokQuotaAutoPauseSource, message+"; paused until "+until.UTC().Format(time.RFC3339)), true
	}
	if until, active := grokQuotaRetryAfterUntil(snapshot, now); active {
		return pause(until, "xai retry_after active")
	}
	if until, exhausted := grokQuotaWindowExhaustedUntil(snapshot.Requests, now); exhausted {
		return pause(until, "xai requests window exhausted")
	}
	if until, exhausted := grokQuotaWindowExhaustedUntil(snapshot.Tokens, now); exhausted {
		return pause(until, "xai tokens window exhausted")
	}
	return time.Time{}, "", false
}

// grokQuotaRetryAfterUntil 报告快照里的 retry_after 是否仍生效，及其结束时刻。
// 没有 updated_at（或解析不了）时按生效处理，结束时刻由调用方兜底。
func grokQuotaRetryAfterUntil(snapshot *xai.QuotaSnapshot, now time.Time) (time.Time, bool) {
	if snapshot == nil || snapshot.RetryAfterSeconds == nil || *snapshot.RetryAfterSeconds <= 0 {
		return time.Time{}, false
	}
	updatedAt, err := parseTime(snapshot.UpdatedAt)
	if err != nil {
		return time.Time{}, true
	}
	until := updatedAt.Add(time.Duration(*snapshot.RetryAfterSeconds) * time.Second)
	return until, now.Before(until)
}

// grokQuotaWindowExhaustedUntil 报告配额窗口是否已用尽（未到重置点），及其重置时刻。
func grokQuotaWindowExhaustedUntil(window *xai.QuotaWindow, now time.Time) (time.Time, bool) {
	if window == nil || window.Limit == nil || window.Remaining == nil || *window.Limit <= 0 {
		return time.Time{}, false
	}
	var resetAt time.Time
	if window.ResetUnix != nil && *window.ResetUnix > 0 {
		resetAt = time.Unix(*window.ResetUnix, 0)
		if !now.Before(resetAt) {
			return time.Time{}, false
		}
	}
	utilization := float64(*window.Limit-*window.Remaining) / float64(*window.Limit)
	if *window.Remaining <= 0 || utilization >= 1 {
		return resetAt, true
	}
	return time.Time{}, false
}

// grokQuotaSnapshotStaleForPause 报告 xAI 配额快照是否已旧到不能据以停调。
func grokQuotaSnapshotStaleForPause(snapshot *xai.QuotaSnapshot, now time.Time) bool {
	if snapshot == nil || snapshot.UpdatedAt == "" {
		return false
	}
	updatedAt, err := parseTime(snapshot.UpdatedAt)
	if err != nil {
		return false
	}
	return now.Sub(updatedAt) >= openAICodexAutoPauseStaleAfter
}

// quotaCounterPauseDecision 按账号自己的配额计数（管理员设的总 / 日 / 周额度，用量入账时递增）判断是否该停调：
// 总额度超限停到管理员重置配额；日 / 周额度超限停到本周期结束。任何设了额度的资源都算，不问类型。
func quotaCounterPauseDecision(account *Account, now time.Time) (time.Time, string, bool) {
	if account == nil {
		return time.Time{}, "", false
	}
	pause := func(until time.Time, message string) (time.Time, string, bool) {
		return until, BuildTempUnschedReasonPayload(quotaCounterSource, message), true
	}
	if limit := account.GetQuotaLimit(); limit > 0 && account.GetQuotaUsed() >= limit {
		return pause(now.Add(quotaCounterIndefinitePause), fmt.Sprintf("total quota %.4f used >= %.4f; paused until the quota is reset", account.GetQuotaUsed(), limit))
	}
	if limit := account.GetQuotaDailyLimit(); limit > 0 {
		if end, active := account.quotaDailyPeriodEnd(now); active && account.GetQuotaDailyUsed() >= limit {
			return pause(end, fmt.Sprintf("daily quota %.4f used >= %.4f; paused until %s", account.GetQuotaDailyUsed(), limit, end.UTC().Format(time.RFC3339)))
		}
	}
	if limit := account.GetQuotaWeeklyLimit(); limit > 0 {
		if end, active := account.quotaWeeklyPeriodEnd(now); active && account.GetQuotaWeeklyUsed() >= limit {
			return pause(end, fmt.Sprintf("weekly quota %.4f used >= %.4f; paused until %s", account.GetQuotaWeeklyUsed(), limit, end.UTC().Format(time.RFC3339)))
		}
	}
	return time.Time{}, "", false
}

// quotaDailyPeriodEnd 返回当前日配额周期的结束时刻（滚动窗口：周期起点 + 24 小时）；
// 周期未开始或已过期（下次递增会重置）返回 false。
func (a *Account) quotaDailyPeriodEnd(now time.Time) (time.Time, bool) {
	start := a.getExtraTime("quota_daily_start")
	if start.IsZero() {
		return time.Time{}, false
	}
	end := start.Add(24 * time.Hour)
	return end, now.Before(end)
}

// quotaWeeklyPeriodEnd 返回当前周配额周期的结束时刻（滚动窗口：周期起点 + 7 天）；周期未开始或已过期返回 false。
func (a *Account) quotaWeeklyPeriodEnd(now time.Time) (time.Time, bool) {
	start := a.getExtraTime("quota_weekly_start")
	if start.IsZero() {
		return time.Time{}, false
	}
	end := start.Add(7 * 24 * time.Hour)
	return end, now.Before(end)
}

// resolveAccountExtraBool 读 account.Extra 里的布尔值，容忍 JSON 反序列化可能给出的几种形状
// （真 bool、"true"/"false" 字符串、0/1 数字）。
func resolveAccountExtraBool(extra map[string]any, key string) bool {
	if len(extra) == 0 {
		return false
	}
	value, ok := extra[key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		return err == nil && parsed
	case float64:
		return v != 0
	case float32:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i != 0
		}
	}
	return false
}
