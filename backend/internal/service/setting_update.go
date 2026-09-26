package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// OmittedSettingKeys marks setting keys the caller's payload never carried.
// SystemSettings is a plain struct, so a field the caller omitted arrives as a
// zero value and is indistinguishable from a deliberate clear. Listing the key
// here drops it from the write, leaving the stored value in place.
//
// A nil or empty set keeps whole-document semantics: every key is written.
type OmittedSettingKeys map[string]struct{}

func (o OmittedSettingKeys) dropFrom(updates map[string]string) {
	for key := range o {
		delete(updates, key)
	}
}

// UpdateSettings 更新系统设置
func (s *SettingService) UpdateSettings(ctx context.Context, settings *SystemSettings) error {
	return s.UpdateSettingsOmitting(ctx, settings, nil)
}

// UpdateSettingsOmitting persists system settings, leaving the keys in omitted
// at their stored value.
func (s *SettingService) UpdateSettingsOmitting(ctx context.Context, settings *SystemSettings, omitted OmittedSettingKeys) error {
	updates, err := s.buildSystemSettingsUpdates(ctx, settings)
	if err != nil {
		return err
	}
	omitted.dropFrom(updates)

	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return err
	}
	s.refreshCachedSettingsAfterWrite(ctx, settings, omitted)
	return nil
}

// refreshCachedSettingsAfterWrite keeps the in-process caches in step with the
// write that just landed. A partial payload carries zero values for the fields
// it omitted, so in that case the caches are rebuilt from storage rather than
// from the request struct.
func (s *SettingService) refreshCachedSettingsAfterWrite(ctx context.Context, settings *SystemSettings, omitted OmittedSettingKeys) {
	if len(omitted) == 0 {
		s.refreshCachedSettings(settings)
		return
	}
	stored, err := s.GetAllSettings(ctx)
	if err != nil {
		slog.Warn("refresh cached settings after partial update failed", "error", err)
		return
	}
	s.refreshCachedSettings(stored)
}

func (s *SettingService) buildSystemSettingsUpdates(ctx context.Context, settings *SystemSettings) (map[string]string, error) {
	alipaySource, err := normalizeVisibleMethodSettingSource("alipay", settings.PaymentVisibleMethodAlipaySource, settings.PaymentVisibleMethodAlipayEnabled)
	if err != nil {
		return nil, err
	}
	wxpaySource, err := normalizeVisibleMethodSettingSource("wxpay", settings.PaymentVisibleMethodWxpaySource, settings.PaymentVisibleMethodWxpayEnabled)
	if err != nil {
		return nil, err
	}
	settings.PaymentVisibleMethodAlipaySource = alipaySource
	settings.PaymentVisibleMethodWxpaySource = wxpaySource
	updates := make(map[string]string)

	// Cloudflare Turnstile 设置（只有非空才更新密钥）
	updates[SettingKeyTurnstileEnabled] = strconv.FormatBool(settings.TurnstileEnabled)
	updates[SettingKeyTurnstileSiteKey] = settings.TurnstileSiteKey
	if settings.TurnstileSecretKey != "" {
		updates[SettingKeyTurnstileSecretKey] = settings.TurnstileSecretKey
	}

	updates[SettingKeyTencentCaptchaEnabled] = strconv.FormatBool(settings.TencentCaptchaEnabled)
	updates[SettingKeyTencentCaptchaAppID] = settings.TencentCaptchaAppID
	if settings.TencentCaptchaAppSecretKey != "" {
		updates[SettingKeyTencentCaptchaAppSecretKey] = settings.TencentCaptchaAppSecretKey
	}
	if settings.TencentCaptchaCloudSecretID != "" {
		updates[SettingKeyTencentCaptchaCloudSecretID] = settings.TencentCaptchaCloudSecretID
	}
	if settings.TencentCaptchaCloudSecretKey != "" {
		updates[SettingKeyTencentCaptchaCloudSecretKey] = settings.TencentCaptchaCloudSecretKey
	}
	updates[SettingKeyTencentCaptchaRegion] = normalizeTencentCaptchaRegion(settings.TencentCaptchaRegion)
	// 阿里云验证码 2.0 设置（只有非空才更新密钥）
	updates[SettingKeyAliyunCaptchaEnabled] = strconv.FormatBool(settings.AliyunCaptchaEnabled)
	updates[SettingKeyAliyunCaptchaAccessKeyID] = settings.AliyunCaptchaAccessKeyID
	if settings.AliyunCaptchaAccessKeySecret != "" {
		updates[SettingKeyAliyunCaptchaAccessKeySecret] = settings.AliyunCaptchaAccessKeySecret
	}
	updates[SettingKeyAliyunCaptchaSceneID] = settings.AliyunCaptchaSceneID
	updates[SettingKeyAliyunCaptchaPrefix] = settings.AliyunCaptchaPrefix
	updates[SettingKeyAliyunCaptchaRegion] = normalizeAliyunCaptchaRegion(settings.AliyunCaptchaRegion)

	// 默认配置
	settings.AffiliateRebateRate = clampAffiliateRebateRate(settings.AffiliateRebateRate)
	updates[SettingKeyAffiliateRebateRate] = strconv.FormatFloat(settings.AffiliateRebateRate, 'f', 8, 64)
	if settings.AffiliateRebateFreezeHours < 0 {
		settings.AffiliateRebateFreezeHours = AffiliateRebateFreezeHoursDefault
	}
	if settings.AffiliateRebateFreezeHours > AffiliateRebateFreezeHoursMax {
		settings.AffiliateRebateFreezeHours = AffiliateRebateFreezeHoursMax
	}
	updates[SettingKeyAffiliateRebateFreezeHours] = strconv.Itoa(settings.AffiliateRebateFreezeHours)
	if settings.AffiliateRebateDurationDays < 0 {
		settings.AffiliateRebateDurationDays = AffiliateRebateDurationDaysDefault
	}
	if settings.AffiliateRebateDurationDays > AffiliateRebateDurationDaysMax {
		settings.AffiliateRebateDurationDays = AffiliateRebateDurationDaysMax
	}
	updates[SettingKeyAffiliateRebateDurationDays] = strconv.Itoa(settings.AffiliateRebateDurationDays)
	if settings.AffiliateRebatePerInviteeCap < 0 {
		settings.AffiliateRebatePerInviteeCap = AffiliateRebatePerInviteeCapDefault
	}
	updates[SettingKeyAffiliateRebatePerInviteeCap] = strconv.FormatFloat(settings.AffiliateRebatePerInviteeCap, 'f', 8, 64)
	updates[SettingKeyAffiliateAdminRechargeEnabled] = strconv.FormatBool(settings.AdminRechargeRebateEnabled)

	// Identity patch configuration (Claude -> Gemini)
	updates[SettingKeyEnableIdentityPatch] = strconv.FormatBool(settings.EnableIdentityPatch)
	updates[SettingKeyIdentityPatchPrompt] = settings.IdentityPatchPrompt

	// Ops monitoring (vNext)
	updates[SettingKeyOpsMonitoringEnabled] = strconv.FormatBool(settings.OpsMonitoringEnabled)
	updates[SettingKeyOpsRealtimeMonitoringEnabled] = strconv.FormatBool(settings.OpsRealtimeMonitoringEnabled)
	updates[SettingKeyOpsQueryModeDefault] = string(ParseOpsQueryMode(settings.OpsQueryModeDefault))
	if settings.OpsMetricsIntervalSeconds > 0 {
		updates[SettingKeyOpsMetricsIntervalSeconds] = strconv.Itoa(settings.OpsMetricsIntervalSeconds)
	}

	// Channel monitor feature switch
	updates[SettingKeyChannelMonitorEnabled] = strconv.FormatBool(settings.ChannelMonitorEnabled)
	// V1 已下线：不管请求带什么都写 v2，库里存量的 v1 在下次保存时对齐
	updates[SettingKeyChannelMonitorMode] = ChannelMonitorModeV2
	if v := clampChannelMonitorInterval(settings.ChannelMonitorDefaultIntervalSeconds); v > 0 {
		updates[SettingKeyChannelMonitorDefaultIntervalSeconds] = strconv.Itoa(v)
	}
	updates[SettingKeyChannelMonitorHideThroughput] = strconv.FormatBool(settings.ChannelMonitorHideThroughput)
	updates[SettingKeyChannelMonitorShowQuota] = strconv.FormatBool(settings.ChannelMonitorShowQuota)
	updates[SettingKeyChannelMonitorHideUserRanking] = strconv.FormatBool(settings.ChannelMonitorHideUserRanking)

	// Grok model mapping policy
	if v := strings.TrimSpace(settings.GrokDefaultTextModel); v != "" {
		updates[SettingKeyGrokDefaultTextModel] = v
	} else {
		updates[SettingKeyGrokDefaultTextModel] = "grok-4.6"
	}
	updates[SettingKeyGrokCrossClientModelMapEnabled] = strconv.FormatBool(settings.GrokCrossClientModelMapEnabled)
	updates[SettingKeyGrokDefaultBaseURLMode] = normalizeGrokDefaultBaseURLMode(settings.GrokDefaultBaseURLMode)

	// Available channels feature switch

	// Model plaza feature switches + description
	updates[SettingKeyPluginManagementEnabled] = strconv.FormatBool(settings.PluginManagementEnabled)

	// Affiliate (邀请返利) feature switch
	updates[SettingKeyAffiliateEnabled] = strconv.FormatBool(settings.AffiliateEnabled)

	// 风控中心功能开关
	updates[SettingKeyRiskControlEnabled] = strconv.FormatBool(settings.RiskControlEnabled)

	// cyber 会话屏蔽开关 + TTL
	updates[SettingKeyCyberSessionBlockEnabled] = strconv.FormatBool(settings.CyberSessionBlockEnabled)
	if settings.CyberSessionBlockTTLSeconds > 0 {
		updates[SettingKeyCyberSessionBlockTTLSeconds] = strconv.Itoa(settings.CyberSessionBlockTTLSeconds)
	}

	// Claude Code version check
	updates[SettingKeyMinClaudeCodeVersion] = settings.MinClaudeCodeVersion
	updates[SettingKeyMaxClaudeCodeVersion] = settings.MaxClaudeCodeVersion

	// 分组隔离

	// Backend Mode

	// Gateway forwarding behavior
	mode := normalizeOpenAITTFTMode(settings.OpenAITTFTMode)
	if strings.TrimSpace(settings.OpenAITTFTMode) != "" && strings.ToLower(strings.TrimSpace(settings.OpenAITTFTMode)) != OpenAITTFTModeSemantic && strings.ToLower(strings.TrimSpace(settings.OpenAITTFTMode)) != OpenAITTFTModeVisible {
		return nil, fmt.Errorf("%s must be one of: %s/%s", SettingKeyOpenAITTFTMode, OpenAITTFTModeSemantic, OpenAITTFTModeVisible)
	}
	updates[SettingKeyOpenAITTFTMode] = mode
	updates[SettingKeyEnableFingerprintUnification] = strconv.FormatBool(settings.EnableFingerprintUnification)
	updates[SettingKeyEnableMetadataPassthrough] = strconv.FormatBool(settings.EnableMetadataPassthrough)
	updates[SettingKeyEnableCCHSigning] = strconv.FormatBool(settings.EnableCCHSigning)
	updates[SettingKeyEnableClaudeOAuthSystemPromptInjection] = strconv.FormatBool(settings.EnableClaudeOAuthSystemPromptInjection)
	updates[SettingKeyClaudeOAuthSystemPrompt] = settings.ClaudeOAuthSystemPrompt
	if err := ValidateClaudeOAuthSystemPromptBlocksConfig(settings.ClaudeOAuthSystemPromptBlocks); err != nil {
		return nil, err
	}
	updates[SettingKeyClaudeOAuthSystemPromptBlocks] = settings.ClaudeOAuthSystemPromptBlocks
	updates[SettingKeyEnableAnthropicCacheTTL1hInjection] = strconv.FormatBool(settings.EnableAnthropicCacheTTL1hInjection)
	updates[SettingKeyRewriteMessageCacheControl] = strconv.FormatBool(settings.RewriteMessageCacheControl)
	updates[SettingKeyEnableClientDatelineNormalization] = strconv.FormatBool(settings.EnableClientDatelineNormalization)
	updates[SettingKeyAntigravityUserAgentVersion] = antigravity.NormalizeUserAgentVersion(settings.AntigravityUserAgentVersion)
	updates[SettingKeyOpenAICodexUserAgent] = strings.TrimSpace(settings.OpenAICodexUserAgent)
	updates[SettingKeyOpenAICodexClientVersion] = NormalizeCodexClientVersion(settings.OpenAICodexClientVersion)
	updates[SettingKeyOpenAICodexVersionAutoSyncEnabled] = strconv.FormatBool(settings.OpenAICodexVersionAutoSyncEnabled)
	// SettingKeyOpenAICodexClientVersionSynced 由自动同步任务独占写入，此处不得覆盖，
	// 否则面板保存会把同步结果清空。
	// codex_cli_only 加固
	updates[SettingKeyMinCodexVersion] = strings.TrimSpace(settings.MinCodexVersion)
	updates[SettingKeyMaxCodexVersion] = strings.TrimSpace(settings.MaxCodexVersion)
	updates[SettingKeyCodexCLIOnlyBlacklist] = strings.TrimSpace(settings.CodexCLIOnlyBlacklist)
	updates[SettingKeyCodexCLIOnlyWhitelist] = strings.TrimSpace(settings.CodexCLIOnlyWhitelist)
	updates[SettingKeyCodexCLIOnlyAllowAppServerClients] = strconv.FormatBool(settings.CodexCLIOnlyAllowAppServerClients)
	updates[SettingKeyCodexCLIOnlyEngineFingerprintSignals] = strings.TrimSpace(settings.CodexCLIOnlyEngineFingerprintSignals)
	updates[SettingPaymentVisibleMethodAlipaySource] = settings.PaymentVisibleMethodAlipaySource
	updates[SettingPaymentVisibleMethodWxpaySource] = settings.PaymentVisibleMethodWxpaySource
	updates[SettingPaymentVisibleMethodAlipayEnabled] = strconv.FormatBool(settings.PaymentVisibleMethodAlipayEnabled)
	updates[SettingPaymentVisibleMethodWxpayEnabled] = strconv.FormatBool(settings.PaymentVisibleMethodWxpayEnabled)

	if settings.AccountSchedulingThresholds != nil {
		normalized, err := validateAndNormalizeAccountSchedulingThresholds(settings.AccountSchedulingThresholds)
		if err != nil {
			return nil, err
		}
		blob, err := json.Marshal(normalized)
		if err != nil {
			return nil, fmt.Errorf("marshal account scheduling thresholds: %w", err)
		}
		updates[SettingKeyAccountSchedulingThresholds] = string(blob)
	}

	if settings.ProfitMinMargin < 0 || settings.ProfitSafetyBuffer < 0 || settings.ProfitMinMargin+settings.ProfitSafetyBuffer > ProfitControlRatioMax {
		return nil, infraerrors.BadRequest("INVALID_PROFIT_CONTROL", fmt.Sprintf("profit control: min_margin + safety_buffer must be within [0, %g]", ProfitControlRatioMax))
	}
	updates[SettingKeyProfitControlEnabled] = strconv.FormatBool(settings.ProfitControlEnabled)
	updates[SettingKeyProfitMinMargin] = strconv.FormatFloat(settings.ProfitMinMargin, 'f', 8, 64)
	updates[SettingKeyProfitSafetyBuffer] = strconv.FormatFloat(settings.ProfitSafetyBuffer, 'f', 8, 64)

	return updates, nil
}

func defaultAccountSchedulingThresholds() map[string]int {
	return map[string]int{
		PlatformOpenAI:    100,
		PlatformAnthropic: 100,
		PlatformGrok:      100,
	}
}

func validateAndNormalizeAccountSchedulingThresholds(input map[string]int) (map[string]int, error) {
	normalized := defaultAccountSchedulingThresholds()
	for platform, value := range input {
		allowed := false
		for _, item := range AllowedSchedulingThresholdPlatforms {
			if item == platform {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, infraerrors.BadRequest("INVALID_ACCOUNT_SCHEDULING_THRESHOLDS", fmt.Sprintf("unknown platform %q", platform))
		}
		if value < 1 || value > 100 {
			return nil, infraerrors.BadRequest("INVALID_ACCOUNT_SCHEDULING_THRESHOLDS", "platform scheduling threshold must be between 1 and 100")
		}
		normalized[platform] = value
	}
	return normalized, nil
}

func parseAccountSchedulingThresholdsSetting(raw string) (map[string]int, error) {
	thresholds := defaultAccountSchedulingThresholds()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return thresholds, nil
	}
	parsed := map[string]int{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return thresholds, err
	}
	for _, platform := range AllowedSchedulingThresholdPlatforms {
		if value, ok := parsed[platform]; ok {
			thresholds[platform] = boundedIntOrDefault(value, 1, 100, 100)
		}
	}
	return thresholds, nil
}

func boundedIntOrDefault(value, minValue, maxValue, defaultValue int) int {
	if value < minValue || value > maxValue {
		return defaultValue
	}
	return value
}

func cloneAccountSchedulingThresholds(input map[string]int) map[string]int {
	if len(input) == 0 {
		return defaultAccountSchedulingThresholds()
	}
	cloned := make(map[string]int, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func (s *SettingService) refreshCachedSettings(settings *SystemSettings) {
	if settings == nil {
		return
	}

	// 先使 inflight singleflight 失效，再刷新缓存，缩小旧值覆盖新值的竞态窗口
	versionBoundsSF.Forget("version_bounds")
	versionBoundsCache.Store(&cachedVersionBounds{
		min:       settings.MinClaudeCodeVersion,
		max:       settings.MaxClaudeCodeVersion,
		expiresAt: time.Now().Add(versionBoundsCacheTTL).UnixNano(),
	})
	gatewayForwardingSF.Forget("gateway_forwarding")
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{
		openAITTFTMode:                   normalizeOpenAITTFTMode(settings.OpenAITTFTMode),
		fingerprintUnification:           settings.EnableFingerprintUnification,
		metadataPassthrough:              settings.EnableMetadataPassthrough,
		cchSigning:                       settings.EnableCCHSigning,
		claudeOAuthSystemPromptInjection: settings.EnableClaudeOAuthSystemPromptInjection,
		claudeOAuthSystemPrompt:          settings.ClaudeOAuthSystemPrompt,
		claudeOAuthSystemPromptBlocks:    settings.ClaudeOAuthSystemPromptBlocks,
		anthropicCacheTTL1hInjection:     settings.EnableAnthropicCacheTTL1hInjection,
		rewriteMessageCacheControl:       settings.RewriteMessageCacheControl,
		clientDatelineNormalization:      settings.EnableClientDatelineNormalization,
		expiresAt:                        time.Now().Add(gatewayForwardingCacheTTL).UnixNano(),
	})
	s.antigravityUAVersionSF.Forget("antigravity_user_agent_version")
	antigravityUserAgentVersion := antigravity.NormalizeUserAgentVersion(settings.AntigravityUserAgentVersion)
	if antigravityUserAgentVersion == "" {
		antigravityUserAgentVersion = antigravity.GetDefaultUserAgentVersion()
	}
	s.antigravityUAVersionCache.Store(&cachedAntigravityUserAgentVersion{
		version:   antigravityUserAgentVersion,
		expiresAt: time.Now().Add(antigravityUserAgentVersionCacheTTL).UnixNano(),
	})
	s.openAICodexUASF.Forget("openai_codex_user_agent")
	codexUA := strings.TrimSpace(settings.OpenAICodexUserAgent)
	if codexUA == "" {
		codexUA = DefaultOpenAICodexUserAgent
	}
	s.openAICodexUACache.Store(&cachedOpenAICodexUserAgent{
		value:     codexUA,
		expiresAt: time.Now().Add(openAICodexUserAgentCacheTTL).UnixNano(),
	})
	// 版本号缓存只做失效，不在此重算：生效值还取决于自动同步写入的 synced 键，
	// 这里没有它的最新值，重算会把同步结果覆盖成陈旧值。
	s.InvalidateOpenAICodexClientVersionCache()
	// Invalidate the quota auto-pause cache and let the next read trigger a fresh load.
	// We can't know from here whether ops_advanced_settings was also touched, so be
	// defensive: store an expired entry — GetOpenAIQuotaAutoPauseSettings will serve
	// stale and kick off an async refresh, never blocking the request that follows.
	s.openAIQuotaAutoPauseSettingsSF.Forget(openAIQuotaAutoPauseSettingsRefreshKey)
	if cached, _ := s.openAIQuotaAutoPauseSettingsCache.Load().(*cachedOpenAIQuotaAutoPauseSettings); cached != nil {
		s.openAIQuotaAutoPauseSettingsCache.Store(&cachedOpenAIQuotaAutoPauseSettings{
			settings:  cached.settings,
			expiresAt: 0,
		})
	}
	InvalidateProfitControlSettingsCache()
	accountSchedulingThresholdsSF.Forget(SettingKeyAccountSchedulingThresholds)
	if settings.AccountSchedulingThresholds != nil {
		normalizedThresholds, err := validateAndNormalizeAccountSchedulingThresholds(settings.AccountSchedulingThresholds)
		if err != nil {
			normalizedThresholds = defaultAccountSchedulingThresholds()
		}
		accountSchedulingThresholdsCache.Store(&cachedAccountSchedulingThresholds{
			thresholds: cloneAccountSchedulingThresholds(normalizedThresholds),
			expiresAt:  time.Now().Add(accountSchedulingThresholdsCacheTTL).UnixNano(),
		})
	} else {
		// Partial/omitted payload: clear cache so the next hot-path read reloads from DB.
		accountSchedulingThresholdsCache.Store(&cachedAccountSchedulingThresholds{})
	}
	// codex_cli_only 加固策略缓存：设置更新后强制下次重载（涉及 4 个键 + JSON 解析，直接置过期）。
	s.codexRestrictionPolicySF.Forget("codex_restriction_policy")
	s.codexRestrictionPolicyCache.Store(&cachedCodexRestrictionPolicy{expiresAt: 0})
	if s.onUpdate != nil {
		s.onUpdate() // Invalidate cache after settings update
	}
	s.notifyChannelMonitorRuntimeListeners()
}

func (s *SettingService) defaultRewriteMessageCacheControl() bool {
	return false
}
