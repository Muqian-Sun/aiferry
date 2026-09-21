package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// 本文件是「由我们自己的用量驱动」的额度评估：窗口费用（Anthropic 5h 窗口）、xAI 免费档本地用量、
// Gemini 本地 RPD/RPM、账号自己的配额计数。它们没有上游快照，只能在每次用量入账后评估。

const (
	windowCostSource       = "window_cost_limit"
	grokFreeQuotaSource    = "grok_free_quota"
	geminiLocalQuotaReason = "gemini_local_quota"
	// grokFreeQuotaPauseMin 免费档是滚动 24h 窗口，没有明确的解除点（拍的）：停这么久后靠下一次入账再评、再停。
	grokFreeQuotaPauseMin = 5 * time.Minute
	// geminiRPDScopePrefix / geminiRPMScopePrefix 是 Gemini 本地配额的模型级限流 scope：按模型档（flash / pro）写，
	// 选号时 modelRateLimitKeysForRequest 会为 gemini 厂商账号附加同一个 scope。
	geminiLocalQuotaScopePrefix = "gemini:"
)

// SetSessionLimitCache 注入窗口费用缓存：用量入账时累进，评估时读。
func (s *RateLimitService) SetSessionLimitCache(cache SessionLimitCache) {
	if s == nil {
		return
	}
	s.sessionLimitCache = cache
}

// ApplyAccountUsageState 一次用量入账后的额度评估：先把本次标准费用累进窗口费用缓存，再走
// ApplyAccountQuotaState（配额计数）、窗口费用、xAI 免费档、Gemini 本地配额。
// model 是入账的模型名（Gemini 按模型档写限流），standardCost 是不含倍率的标准费用。
// 只对设了相应额度的资源做，其余直接返回。
func (s *RateLimitService) ApplyAccountUsageState(ctx context.Context, account *Account, model string, standardCost float64) {
	if s == nil || s.accountRepo == nil || account == nil || account.ID <= 0 {
		return
	}
	if account.GetWindowCostLimit() > 0 {
		s.accumulateWindowCost(ctx, account, standardCost)
	}
	if account.GetQuotaLimit() > 0 || account.GetQuotaDailyLimit() > 0 || account.GetQuotaWeeklyLimit() > 0 {
		// 计数是 DB 原子递增，内存对象没更新：重读一遍再评。
		if latest, err := s.accountRepo.GetByID(ctx, account.ID); err == nil && latest != nil {
			account = latest
		}
	}
	if !account.IsActive() || !account.Schedulable {
		return
	}
	if account.Vendor() == PlatformGemini {
		s.applyGeminiLocalQuota(ctx, account, model)
	}
	if s.ApplyAccountQuotaState(ctx, account) {
		return
	}
	now := time.Now()
	if until, reason, paused := s.windowCostPauseDecision(ctx, account, now); paused {
		s.pauseAccountUntil(ctx, account, until, reason, windowCostSource)
		return
	}
	if until, reason, paused := s.grokFreeQuotaPauseDecision(ctx, account, now); paused {
		s.pauseAccountUntil(ctx, account, until, reason, grokFreeQuotaSource)
	}
}

// accumulateWindowCost 把本次标准费用累进窗口费用缓存；缓存未命中不累（下次评估从 DB 聚合）。
func (s *RateLimitService) accumulateWindowCost(ctx context.Context, account *Account, standardCost float64) {
	if s.sessionLimitCache == nil || standardCost <= 0 {
		return
	}
	cost, hit, err := s.sessionLimitCache.GetWindowCost(ctx, account.ID)
	if err != nil || !hit {
		return
	}
	if err := s.sessionLimitCache.SetWindowCost(ctx, account.ID, cost+standardCost); err != nil {
		slog.Debug("window_cost_accumulate_failed", "account_id", account.ID, "error", err)
	}
}

// windowCostPauseDecision 5h 窗口费用到阈值就停到窗口结束。任何设了 window_cost_limit 的资源都算。
// 原来的「黄区只允许粘性」不再有：状态只有停 / 不停。
func (s *RateLimitService) windowCostPauseDecision(ctx context.Context, account *Account, now time.Time) (time.Time, string, bool) {
	limit := account.GetWindowCostLimit()
	if limit <= 0 {
		return time.Time{}, "", false
	}
	cost, ok := s.currentWindowCost(ctx, account)
	if !ok || cost < limit {
		return time.Time{}, "", false
	}
	until := time.Time{}
	if account.SessionWindowEnd != nil && account.SessionWindowEnd.After(now) {
		until = *account.SessionWindowEnd
	} else {
		until = account.GetCurrentWindowStartTime().Add(5 * time.Hour)
	}
	message := fmt.Sprintf("window cost %.4f >= limit %.4f; paused until %s", cost, limit, until.UTC().Format(time.RFC3339))
	return until, BuildTempUnschedReasonPayload(windowCostSource, message), true
}

// currentWindowCost 当前窗口的标准费用：缓存命中用缓存，否则从用量日志聚合并回填缓存。查不到按不停处理。
func (s *RateLimitService) currentWindowCost(ctx context.Context, account *Account) (float64, bool) {
	if s.sessionLimitCache != nil {
		if cost, hit, err := s.sessionLimitCache.GetWindowCost(ctx, account.ID); err == nil && hit {
			return cost, true
		}
	}
	if s.usageRepo == nil {
		return 0, false
	}
	stats, err := s.usageRepo.GetAccountWindowStats(ctx, account.ID, account.GetCurrentWindowStartTime())
	if err != nil || stats == nil {
		return 0, false
	}
	if s.sessionLimitCache != nil {
		_ = s.sessionLimitCache.SetWindowCost(ctx, account.ID, stats.StandardCost)
	}
	return stats.StandardCost, true
}

// grokFreeQuotaPauseDecision xAI 免费档（明确 free 的 OAuth 成品号）本地滚动窗口用量到软门就停一小段。
func (s *RateLimitService) grokFreeQuotaPauseDecision(ctx context.Context, account *Account, now time.Time) (time.Time, string, bool) {
	if !isExplicitGrokFreeOAuthAccount(account) || s.usageRepo == nil {
		return time.Time{}, "", false
	}
	settings, enabled := resolveGrokFreeQuotaGateSettings(s.cfg)
	if !enabled {
		return time.Time{}, "", false
	}
	stats, err := s.usageRepo.GetAccountWindowStats(ctx, account.ID, now.Add(-settings.window))
	if err != nil || stats == nil || stats.Tokens < settings.gateTokens {
		return time.Time{}, "", false
	}
	pause := settings.cacheTTL
	if pause < grokFreeQuotaPauseMin {
		pause = grokFreeQuotaPauseMin
	}
	until := now.Add(pause)
	message := fmt.Sprintf("xai free tier local usage %d tokens >= soft gate %d (limit %d, window %.0fh); paused until %s",
		stats.Tokens, settings.gateTokens, settings.limitTokens, settings.window.Hours(), until.UTC().Format(time.RFC3339))
	return until, BuildTempUnschedReasonPayload(grokFreeQuotaSource, message), true
}

// applyGeminiLocalQuota Gemini 官方档位的本地 RPD / RPM：某一档（flash / pro）用满就给该档写模型级限流，
// RPD 到 PST 午夜、RPM 到下一分钟整。共享池（SharedRPD / SharedRPM）两档一起写。
func (s *RateLimitService) applyGeminiLocalQuota(ctx context.Context, account *Account, model string) {
	if s.usageRepo == nil || s.geminiQuotaService == nil || model == "" {
		return
	}
	quota, ok := s.geminiQuotaService.QuotaForAccount(ctx, account)
	if !ok {
		return
	}
	now := time.Now()
	modelClass := geminiModelClassFromName(model)

	block := func(resetAt time.Time, shared bool, window string, used, limit int64) {
		scopes := []string{geminiLocalQuotaScope(modelClass)}
		if shared {
			scopes = []string{geminiLocalQuotaScope(geminiModelFlash), geminiLocalQuotaScope(geminiModelPro)}
		}
		reason := fmt.Sprintf("%s: %s %d/%d used; blocked until %s", geminiLocalQuotaReason, window, used, limit, resetAt.UTC().Format(time.RFC3339))
		for _, scope := range scopes {
			if account.isRateLimitActiveForKey(scope) {
				continue
			}
			setAccountModelRateLimitSnapshot(account, scope, resetAt, reason, now)
			if err := s.accountRepo.SetModelRateLimit(ctx, account.ID, scope, resetAt, reason); err != nil {
				slog.Warn("gemini_local_quota_set_model_limit_failed", "account_id", account.ID, "scope", scope, "error", err)
			}
		}
	}

	if limit := geminiDailyLimit(quota, modelClass); limit > 0 {
		start := geminiDailyWindowStart(now)
		totals, cached := s.getGeminiUsageTotals(account.ID, start, now)
		if !cached {
			stats, err := s.usageRepo.GetModelStatsWithFilters(ctx, start, now, 0, 0, account.ID, 0, nil, nil, nil)
			if err == nil {
				totals = geminiAggregateUsage(stats)
				s.setGeminiUsageTotals(account.ID, start, now, totals)
				cached = true
			}
		}
		if used := geminiUsedRequests(quota, modelClass, totals, true); cached && used >= limit {
			block(geminiDailyResetTime(now), quota.SharedRPD > 0, "daily", used, limit)
		}
	}
	if limit := geminiMinuteLimit(quota, modelClass); limit > 0 {
		start := now.Truncate(time.Minute)
		stats, err := s.usageRepo.GetModelStatsWithFilters(ctx, start, now, 0, 0, account.ID, 0, nil, nil, nil)
		if err == nil {
			if used := geminiUsedRequests(quota, modelClass, geminiAggregateUsage(stats), false); used >= limit {
				block(start.Add(time.Minute), quota.SharedRPM > 0, "minute", used, limit)
			}
		}
	}
}

// geminiLocalQuotaScope 是 Gemini 本地配额的模型级限流 scope。
func geminiLocalQuotaScope(class geminiModelClass) string {
	if class == geminiModelFlash {
		return geminiLocalQuotaScopePrefix + "flash"
	}
	return geminiLocalQuotaScopePrefix + "pro"
}
