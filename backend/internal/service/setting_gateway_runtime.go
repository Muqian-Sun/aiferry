package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

// ProfitControlSettings 利润门（全站一档）：后台只有「最低毛利率」一个数，填 0 = 不装门。
type ProfitControlSettings struct {
	MinMargin float64
}

// Enabled 最低毛利率 > 0 才装门。
func (s ProfitControlSettings) Enabled() bool {
	return s.MinMargin > 0
}

// cachedProfitControlSettings 缓存利润门设置（进程内缓存，60s TTL；调度热路径每次选号都读）
type cachedProfitControlSettings struct {
	settings  ProfitControlSettings
	expiresAt int64 // unix nano
}

var profitControlSettingsCache atomic.Value // *cachedProfitControlSettings
var profitControlSettingsSF singleflight.Group

const profitControlSettingsRefreshKey = "profit_control_settings"
const profitControlSettingsCacheTTL = 60 * time.Second
const profitControlSettingsErrorTTL = 5 * time.Second
const profitControlSettingsDBTimeout = 5 * time.Second

// InvalidateProfitControlSettingsCache 让下一次热路径读取重载设置（设置更新后 / 测试换夹具）。
func InvalidateProfitControlSettingsCache() {
	profitControlSettingsSF.Forget(profitControlSettingsRefreshKey)
	profitControlSettingsCache.Store(&cachedProfitControlSettings{})
}

// DefaultOpenAICodexUserAgent 是 OpenAI Codex 默认 User-Agent，用于规避浏览器 UA 的质询。
// 默认采用 codex-tui 身份，版本段随 codexCLIVersion 一起更新。
const DefaultOpenAICodexUserAgent = codexCLIUserAgent

// cachedOpenAICodexClientVersion 缓存出站 Codex 客户端版本号（进程内缓存，60s TTL）
type cachedOpenAICodexClientVersion struct {
	version   string
	expiresAt int64 // unix nano
}

const openAICodexClientVersionCacheTTL = 60 * time.Second
const openAICodexClientVersionErrorTTL = 5 * time.Second
const openAICodexClientVersionDBTimeout = 5 * time.Second

// openAICodexClientVersionSFKey singleflight 键。
const openAICodexClientVersionSFKey = "openai_codex_client_version"

type cachedOpenAIQuotaAutoPauseSettings struct {
	settings  OpsOpenAIAccountQuotaAutoPauseSettings
	expiresAt int64
}

// cachedCyberSessionBlockRuntime cyber 会话屏蔽开关+TTL 进程内缓存（60s TTL）。
// GetCyberSessionBlockRuntime 在网关请求热路径上被调用，避免每次访问 DB。
type cachedCyberSessionBlockRuntime struct {
	enabled   bool
	ttl       time.Duration
	expiresAt int64 // unix nano
}

const cyberSessionBlockRuntimeCacheTTL = 60 * time.Second
const cyberSessionBlockRuntimeErrorTTL = 5 * time.Second
const cyberSessionBlockRuntimeDBTimeout = 5 * time.Second

const openAIQuotaAutoPauseSettingsCacheTTL = 60 * time.Second
const openAIQuotaAutoPauseSettingsErrorTTL = 5 * time.Second
const openAIQuotaAutoPauseSettingsDBTimeout = 5 * time.Second

const openAIQuotaAutoPauseSettingsRefreshKey = "openai_quota_auto_pause_settings"

// GetCyberSessionBlockRuntime 返回 (开关, TTL)，进程内缓存 ~60s，
// 供网关热路径读取时避免 DB 往返。
// 两个 setting key 在单次 singleflight 里一起读取，减少 DB 往返。
// 默认值：开关 false，TTL 1h（与粘性会话对齐）。
func (s *SettingService) GetCyberSessionBlockRuntime(ctx context.Context) (bool, time.Duration) {
	if cached, ok := s.cyberSessionBlockRuntimeCache.Load().(*cachedCyberSessionBlockRuntime); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.enabled, cached.ttl
		}
	}
	result, _, _ := s.cyberSessionBlockRuntimeSF.Do("cyber_session_block_runtime", func() (any, error) {
		if cached, ok := s.cyberSessionBlockRuntimeCache.Load().(*cachedCyberSessionBlockRuntime); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cyberSessionBlockRuntimeDBTimeout)
		defer cancel()

		enabledVal, enabledErr := s.settingRepo.GetValue(dbCtx, SettingKeyCyberSessionBlockEnabled)
		ttlVal, ttlErr := s.settingRepo.GetValue(dbCtx, SettingKeyCyberSessionBlockTTLSeconds)

		if enabledErr != nil && !errors.Is(enabledErr, ErrSettingNotFound) {
			slog.Warn("failed to get cyber_session_block_enabled setting", "error", enabledErr)
			entry := &cachedCyberSessionBlockRuntime{
				enabled:   false,
				ttl:       time.Hour,
				expiresAt: time.Now().Add(cyberSessionBlockRuntimeErrorTTL).UnixNano(),
			}
			s.cyberSessionBlockRuntimeCache.Store(entry)
			return entry, nil
		}

		enabled := enabledErr == nil && strings.TrimSpace(enabledVal) == "true"

		ttl := time.Hour
		if ttlErr == nil {
			if n, perr := strconv.Atoi(strings.TrimSpace(ttlVal)); perr == nil && n > 0 {
				ttl = time.Duration(n) * time.Second
			}
		}

		entry := &cachedCyberSessionBlockRuntime{
			enabled:   enabled,
			ttl:       ttl,
			expiresAt: time.Now().Add(cyberSessionBlockRuntimeCacheTTL).UnixNano(),
		}
		s.cyberSessionBlockRuntimeCache.Store(entry)
		return entry, nil
	})
	if entry, ok := result.(*cachedCyberSessionBlockRuntime); ok && entry != nil {
		return entry.enabled, entry.ttl
	}
	return false, time.Hour
}

// GetOpenAICodexClientVersion 返回出站声明的 Codex 客户端版本号。
// 优先级：自动同步到的官方最新稳定版 → 内置常量（后台不再能手工固定版本）。
// 上游在容量紧张时按客户端身份分优先级降载，陈旧版本会被优先丢弃，故该值需保持跟随官方发布；
// 自动同步让运维不必为了跟版本而发新版本。
func (s *SettingService) GetOpenAICodexClientVersion(ctx context.Context) string {
	fallback := codexCLIVersion
	if s == nil || s.settingRepo == nil {
		return fallback
	}
	if cached, ok := s.openAICodexVersionCache.Load().(*cachedOpenAICodexClientVersion); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.version
		}
	}

	result, _, _ := s.openAICodexVersionSF.Do(openAICodexClientVersionSFKey, func() (any, error) {
		if cached, ok := s.openAICodexVersionCache.Load().(*cachedOpenAICodexClientVersion); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.version, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAICodexClientVersionDBTimeout)
		defer cancel()
		synced, err := s.settingRepo.GetValue(dbCtx, SettingKeyOpenAICodexClientVersionSynced)
		if err != nil && !errors.Is(err, ErrSettingNotFound) {
			slog.Warn("failed to get openai codex client version setting", "error", err)
			s.openAICodexVersionCache.Store(&cachedOpenAICodexClientVersion{
				version:   fallback,
				expiresAt: time.Now().Add(openAICodexClientVersionErrorTTL).UnixNano(),
			})
			return fallback, nil
		}
		version := NormalizeCodexClientVersion(synced)
		if version == "" {
			version = fallback
		}
		s.openAICodexVersionCache.Store(&cachedOpenAICodexClientVersion{
			version:   version,
			expiresAt: time.Now().Add(openAICodexClientVersionCacheTTL).UnixNano(),
		})
		return version, nil
	})
	if version, ok := result.(string); ok && version != "" {
		return version
	}
	return fallback
}

// InvalidateOpenAICodexClientVersionCache 丢弃版本号缓存，下次读取回源。自动同步写入后调用。
func (s *SettingService) InvalidateOpenAICodexClientVersionCache() {
	if s == nil {
		return
	}
	s.openAICodexVersionSF.Forget(openAICodexClientVersionSFKey)
	s.openAICodexVersionCache.Store((*cachedOpenAICodexClientVersion)(nil))
}

// GetOpenAICodexCanonicalUserAgent 返回出站规范 Codex User-Agent：按当前生效的客户端版本号拼出标准 Codex TUI UA。
func (s *SettingService) GetOpenAICodexCanonicalUserAgent(ctx context.Context) string {
	if s == nil {
		return codexCLIUserAgent
	}
	return buildCodexCLIUserAgent(s.GetOpenAICodexClientVersion(ctx))
}

// IsBackendModeEnabled「只留管理站」模式：由代码决定（site_features.go），不再有后台开关。
func (s *SettingService) IsBackendModeEnabled(ctx context.Context) bool {
	return BackendModeEnabled
}

// GetOpenAIQuotaAutoPauseSettings returns the current global default quota auto-pause
// settings. It is invoked on the OpenAI scheduling hot path (once per request) and is
// therefore designed to never block on the DB:
//
//   - Fresh cached value → returned immediately.
//   - Stale or empty cache → the last known value is returned, and a background
//     goroutine refreshes the cache via singleflight (stale-while-revalidate).
//   - First call with no cache yet → zero defaults are returned and the same async
//     refresh is kicked off; the next call gets the freshly populated value.
//
// Callers that need the freshly persisted value synchronously (tests, post-update
// confirmation, optional startup warm-up) should call WarmOpenAIQuotaAutoPauseSettings.
func (s *SettingService) GetOpenAIQuotaAutoPauseSettings(ctx context.Context) OpsOpenAIAccountQuotaAutoPauseSettings {
	if s == nil {
		return OpsOpenAIAccountQuotaAutoPauseSettings{}
	}
	cached, _ := s.openAIQuotaAutoPauseSettingsCache.Load().(*cachedOpenAIQuotaAutoPauseSettings)
	now := time.Now().UnixNano()
	if cached != nil && now < cached.expiresAt {
		return cached.settings
	}
	// Stale or unset: trigger background refresh without blocking this request.
	// singleflight.DoChan dedupes concurrent refreshes; we deliberately ignore the
	// returned channel — the result is observable via the atomic cache.
	s.openAIQuotaAutoPauseSettingsSF.DoChan(openAIQuotaAutoPauseSettingsRefreshKey, func() (any, error) {
		s.refreshOpenAIQuotaAutoPauseSettings(context.Background())
		return nil, nil
	})
	if cached != nil {
		return cached.settings // serve stale value while revalidating
	}
	return OpsOpenAIAccountQuotaAutoPauseSettings{}
}

// WarmOpenAIQuotaAutoPauseSettings synchronously loads the quota auto-pause settings
// into the in-memory cache. Useful for application startup (so the first request hits
// a warm cache) and for tests that need deterministic reads immediately after
// constructing the service.
func (s *SettingService) WarmOpenAIQuotaAutoPauseSettings(ctx context.Context) OpsOpenAIAccountQuotaAutoPauseSettings {
	if s == nil {
		return OpsOpenAIAccountQuotaAutoPauseSettings{}
	}
	s.refreshOpenAIQuotaAutoPauseSettings(ctx)
	cached, _ := s.openAIQuotaAutoPauseSettingsCache.Load().(*cachedOpenAIQuotaAutoPauseSettings)
	if cached == nil {
		return OpsOpenAIAccountQuotaAutoPauseSettings{}
	}
	return cached.settings
}

// refreshOpenAIQuotaAutoPauseSettings reads the latest settings from the DB and stores
// them into the in-memory cache. On error it stores the prior value (or zero defaults
// if nothing is cached yet) with the shorter error TTL so the next refresh comes
// sooner. Always uses its own timeout-bounded context to keep refresh latency
// predictable regardless of the caller.
func (s *SettingService) refreshOpenAIQuotaAutoPauseSettings(ctx context.Context) {
	if s == nil || s.settingRepo == nil {
		return
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIQuotaAutoPauseSettingsDBTimeout)
	defer cancel()

	settings := OpsOpenAIAccountQuotaAutoPauseSettings{}
	ttl := openAIQuotaAutoPauseSettingsCacheTTL
	raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyOpsAdvancedSettings)
	if err == nil {
		cfg := defaultOpsAdvancedSettings()
		if strings.TrimSpace(raw) != "" {
			if jsonErr := json.Unmarshal([]byte(raw), cfg); jsonErr == nil {
				normalizeOpsAdvancedSettings(cfg)
			}
		}
		settings = cfg.OpenAIAccountQuotaAutoPause
	} else if !errors.Is(err, ErrSettingNotFound) {
		// Real error: keep serving prior value but refresh sooner.
		if prior, _ := s.openAIQuotaAutoPauseSettingsCache.Load().(*cachedOpenAIQuotaAutoPauseSettings); prior != nil {
			settings = prior.settings
		}
		ttl = openAIQuotaAutoPauseSettingsErrorTTL
	}

	s.openAIQuotaAutoPauseSettingsCache.Store(&cachedOpenAIQuotaAutoPauseSettings{
		settings:  settings,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

// SetOpenAIQuotaAutoPauseSettings writes the given settings directly into the in-memory
// cache. Called from settings-write code paths so that the next read reflects the new
// value immediately, without waiting for the background refresh.
func (s *SettingService) SetOpenAIQuotaAutoPauseSettings(settings OpsOpenAIAccountQuotaAutoPauseSettings) {
	if s == nil {
		return
	}
	s.openAIQuotaAutoPauseSettingsCache.Store(&cachedOpenAIQuotaAutoPauseSettings{
		settings:  settings,
		expiresAt: time.Now().Add(openAIQuotaAutoPauseSettingsCacheTTL).UnixNano(),
	})
}
