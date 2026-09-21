package service

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// xAI 免费档本地软门的设置与判定。评估在用量入账后由状态服务做（grokFreeQuotaPauseDecision），
// 不在选号路径上。
//
// Config keys (gateway.grok.*):
//   - free_quota_soft_gate_enabled     (bool, default true)
//   - free_quota_token_limit           (int64, default 500_000)
//   - free_quota_soft_gate_percent     (int, default 95) — stop scheduling before the nominal limit
//   - free_quota_window_hours          (int, default 24) — local usage rolling window
//   - free_quota_stats_cache_seconds   (int, default 60) — 停调时长的下限来源（见 grokFreeQuotaPauseMin）
//
// Soft-gate applies only to *explicit* free OAuth (subscription_tier/plan_type ==
// "free"). Media/cache free detection uses isKnownGrokFreeAccount instead.
// Defaults live on config.Gateway.Grok (see config load defaults / tests).

type GrokFreeQuotaPolicy struct {
	Enabled         bool  `json:"enabled"`
	TokenLimit      int64 `json:"token_limit"`
	SoftGatePercent int   `json:"soft_gate_percent"`
	SoftGateTokens  int64 `json:"soft_gate_tokens"`
	WindowHours     int   `json:"window_hours"`
}

type grokFreeQuotaGateSettings struct {
	limitTokens int64
	gateTokens  int64
	window      time.Duration
	cacheTTL    time.Duration
}

func resolveGrokFreeQuotaGateSettings(cfg *config.Config) (grokFreeQuotaGateSettings, bool) {
	if cfg == nil || !cfg.Gateway.Grok.FreeQuotaSoftGateEnabled {
		return grokFreeQuotaGateSettings{}, false
	}
	limit := cfg.Gateway.Grok.FreeQuotaTokenLimit
	percent := cfg.Gateway.Grok.FreeQuotaSoftGatePercent
	windowHours := cfg.Gateway.Grok.FreeQuotaWindowHours
	cacheSeconds := cfg.Gateway.Grok.FreeQuotaStatsCacheSeconds
	if limit <= 0 || percent < 1 || percent > 100 || windowHours <= 0 || cacheSeconds < 0 {
		return grokFreeQuotaGateSettings{}, false
	}
	gate := calculateGrokFreeQuotaSoftGateTokens(limit, percent)
	if gate <= 0 {
		return grokFreeQuotaGateSettings{}, false
	}
	return grokFreeQuotaGateSettings{
		limitTokens: limit,
		gateTokens:  gate,
		window:      time.Duration(windowHours) * time.Hour,
		cacheTTL:    time.Duration(cacheSeconds) * time.Second,
	}, true
}

func calculateGrokFreeQuotaSoftGateTokens(limit int64, percent int) int64 {
	if limit <= 0 || percent <= 0 {
		return 0
	}
	return (limit/100)*int64(percent) + (limit%100)*int64(percent)/100
}

// isExplicitGrokFreeOAuthAccount decides whether the free soft-gate applies.
// Contract: only OAuth accounts with credentials/extra
// subscription_tier or plan_type exactly "free" (case-insensitive). Inferred
// free / basic / blank plan do not soft-gate.
func isExplicitGrokFreeOAuthAccount(account *Account) bool {
	if account == nil || !account.IsGrokOAuth() {
		return false
	}
	for _, tier := range []string{
		account.GetCredential("subscription_tier"),
		account.GetCredential("plan_type"),
		account.GetExtraString("subscription_tier"),
		account.GetExtraString("plan_type"),
	} {
		if strings.EqualFold(strings.TrimSpace(tier), "free") {
			return true
		}
	}
	return false
}
