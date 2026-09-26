package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// InitializeDefaultSettings 初始化默认设置
func (s *SettingService) InitializeDefaultSettings(ctx context.Context) error {
	// 检查是否已有设置（最低毛利率一定在默认值里写入、后台也一直保留，用它当「已初始化」的标记）
	_, err := s.settingRepo.GetValue(ctx, SettingKeyProfitMinMargin)
	if err == nil {
		// 已有设置，不需要初始化
		return nil
	}
	if !errors.Is(err, ErrSettingNotFound) {
		return fmt.Errorf("check existing settings: %w", err)
	}

	// 初始化默认设置
	defaults := map[string]string{
		SettingKeyAffiliateRebateRate:          strconv.FormatFloat(AffiliateRebateRateDefault, 'f', 8, 64),
		SettingKeyAffiliateRebateFreezeHours:   strconv.Itoa(AffiliateRebateFreezeHoursDefault),
		SettingKeyAffiliateRebateDurationDays:  strconv.Itoa(AffiliateRebateDurationDaysDefault),
		SettingKeyAffiliateRebatePerInviteeCap: strconv.FormatFloat(AffiliateRebatePerInviteeCapDefault, 'f', 2, 64),
		// Identity patch defaults
		SettingKeyEnableIdentityPatch: "true",
		SettingKeyIdentityPatchPrompt: "",

		// Ops monitoring defaults (vNext)
		SettingKeyOpsMonitoringEnabled:         "true",
		SettingKeyOpsRealtimeMonitoringEnabled: "true",
		SettingKeyOpsQueryModeDefault:          "auto",
		SettingKeyOpsMetricsIntervalSeconds:    "60",

		// Channel monitor defaults (enabled, 60s)
		SettingKeyChannelMonitorEnabled:                "true",
		SettingKeyChannelMonitorMode:                   ChannelMonitorModeV2,
		SettingKeyChannelMonitorDefaultIntervalSeconds: "60",
		SettingKeyChannelMonitorHideThroughput:         "true",
		SettingKeyChannelMonitorShowQuota:              "false",
		SettingKeyChannelMonitorHideUserRanking:        "false",

		// Grok compatibility defaults: cross-client mapping stays enabled unless
		// operators explicitly disable it.
		SettingKeyGrokDefaultTextModel:           "grok-4.6",
		SettingKeyGrokCrossClientModelMapEnabled: "true",
		SettingKeyGrokDefaultBaseURLMode:         GrokDefaultBaseURLModeCLI,

		// Available channels feature (default disabled; opt-in)

		SettingKeyPluginManagementEnabled: "false",

		// Affiliate (邀请返利) feature (default disabled; opt-in)
		SettingKeyAffiliateEnabled:              "false",
		SettingKeyAffiliateAdminRechargeEnabled: strconv.FormatBool(AdminRechargeRebateEnabledDefault),

		// 风控中心功能（默认关闭，显式启用）
		SettingKeyRiskControlEnabled: "false",

		// cyber 会话屏蔽（默认关闭，TTL 默认 3600s）
		SettingKeyCyberSessionBlockEnabled:    "false",
		SettingKeyCyberSessionBlockTTLSeconds: "3600",

		// Claude Code version check (default: empty = disabled)
		SettingKeyMinClaudeCodeVersion: "",
		SettingKeyMaxClaudeCodeVersion: "",

		// codex_cli_only 加固（默认：版本不检查、名单空、默认种子指纹信号）
		SettingKeyMinCodexVersion:                      "",
		SettingKeyMaxCodexVersion:                      "",
		SettingKeyCodexCLIOnlyBlacklist:                "",
		SettingKeyCodexCLIOnlyWhitelist:                "",
		SettingKeyCodexCLIOnlyAllowAppServerClients:    "false",
		SettingKeyCodexCLIOnlyEngineFingerprintSignals: openai.DefaultEngineFingerprintSignalsJSON(),

		// 分组隔离（默认不允许未分组 Key 调度）
		SettingKeyEnableAnthropicCacheTTL1hInjection: "false",
		SettingKeyRewriteMessageCacheControl:         strconv.FormatBool(s.defaultRewriteMessageCacheControl()),
		SettingKeyEnableClientDatelineNormalization:  "true",
		SettingKeyAntigravityUserAgentVersion:        "",
		SettingKeyOpenAICodexUserAgent:               "",
		SettingKeyOpenAICodexClientVersion:           "",
		SettingKeyOpenAICodexClientVersionSynced:     "",
		SettingKeyOpenAICodexVersionAutoSyncEnabled:  "true",
		SettingPaymentVisibleMethodAlipaySource:      "",
		SettingPaymentVisibleMethodWxpaySource:       "",
		SettingPaymentVisibleMethodAlipayEnabled:     "false",
		SettingPaymentVisibleMethodWxpayEnabled:      "false",

		SettingKeyProfitControlEnabled: "false",
		SettingKeyProfitMinMargin:      "0",
		SettingKeyProfitSafetyBuffer:   "0",
	}

	return s.settingRepo.SetMultiple(ctx, defaults)
}

// parseSettings 解析设置到结构体
func (s *SettingService) parseSettings(settings map[string]string) *SystemSettings {
	result := &SystemSettings{}

	// 解析浮点数类型
	if rebateRate, err := strconv.ParseFloat(settings[SettingKeyAffiliateRebateRate], 64); err == nil {
		result.AffiliateRebateRate = clampAffiliateRebateRate(rebateRate)
	} else {
		result.AffiliateRebateRate = AffiliateRebateRateDefault
	}
	if freezeHours, err := strconv.Atoi(settings[SettingKeyAffiliateRebateFreezeHours]); err == nil && freezeHours >= 0 {
		if freezeHours > AffiliateRebateFreezeHoursMax {
			freezeHours = AffiliateRebateFreezeHoursMax
		}
		result.AffiliateRebateFreezeHours = freezeHours
	}
	if durationDays, err := strconv.Atoi(settings[SettingKeyAffiliateRebateDurationDays]); err == nil && durationDays >= 0 {
		if durationDays > AffiliateRebateDurationDaysMax {
			durationDays = AffiliateRebateDurationDaysMax
		}
		result.AffiliateRebateDurationDays = durationDays
	}
	if perInviteeCap, err := strconv.ParseFloat(settings[SettingKeyAffiliateRebatePerInviteeCap], 64); err == nil && perInviteeCap >= 0 {
		result.AffiliateRebatePerInviteeCap = perInviteeCap
	}
	result.AdminRechargeRebateEnabled = settings[SettingKeyAffiliateAdminRechargeEnabled] == "true"

	// 敏感信息直接返回，方便测试连接时使用

	// Identity patch settings (default: enabled, to preserve existing behavior)
	if v, ok := settings[SettingKeyEnableIdentityPatch]; ok && v != "" {
		result.EnableIdentityPatch = v == "true"
	} else {
		result.EnableIdentityPatch = true
	}
	result.IdentityPatchPrompt = settings[SettingKeyIdentityPatchPrompt]

	// Ops monitoring settings (default: enabled, fail-open)
	result.OpsMonitoringEnabled = !isFalseSettingValue(settings[SettingKeyOpsMonitoringEnabled])
	result.OpsRealtimeMonitoringEnabled = !isFalseSettingValue(settings[SettingKeyOpsRealtimeMonitoringEnabled])
	result.OpsQueryModeDefault = string(ParseOpsQueryMode(settings[SettingKeyOpsQueryModeDefault]))
	result.OpsMetricsIntervalSeconds = 60
	if raw := strings.TrimSpace(settings[SettingKeyOpsMetricsIntervalSeconds]); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			if v < 60 {
				v = 60
			}
			if v > 3600 {
				v = 3600
			}
			result.OpsMetricsIntervalSeconds = v
		}
	}

	// Channel monitor feature (default: enabled, 60s)
	result.ChannelMonitorEnabled = !isFalseSettingValue(settings[SettingKeyChannelMonitorEnabled])
	result.ChannelMonitorMode = ChannelMonitorModeV2 // V1 已下线，模式固定 v2
	result.ChannelMonitorDefaultIntervalSeconds = parseChannelMonitorInterval(
		settings[SettingKeyChannelMonitorDefaultIntervalSeconds],
	)
	// 默认隐藏吞吐（迁移 206 的隐私默认）：未配置时必须与 setting_public.go 的
	// 公开读取路径给出同一个值，否则管理端看到“未隐藏”而用户端实际已隐藏。
	result.ChannelMonitorHideThroughput = !isFalseSettingValue(settings[SettingKeyChannelMonitorHideThroughput])
	// 配额展示默认关闭且 fail-closed：仅字面 "true" 视为开启
	// （与 setting_public.go 公开读取路径保持一致）。
	result.ChannelMonitorShowQuota = settings[SettingKeyChannelMonitorShowQuota] == "true"
	result.ChannelMonitorHideUserRanking = isTrueSettingValue(settings[SettingKeyChannelMonitorHideUserRanking])

	// Grok default mapping policy
	result.GrokDefaultTextModel = strings.TrimSpace(settings[SettingKeyGrokDefaultTextModel])
	if result.GrokDefaultTextModel == "" {
		result.GrokDefaultTextModel = "grok-4.6"
	}
	// Default true (missing/empty → enabled) so Claude/Codex→Grok mapping keeps working.
	// Operators can set false to disable silent cross-client rewrite.
	result.GrokCrossClientModelMapEnabled = !isFalseSettingValue(settings[SettingKeyGrokCrossClientModelMapEnabled])
	result.GrokDefaultBaseURLMode = normalizeGrokDefaultBaseURLMode(settings[SettingKeyGrokDefaultBaseURLMode])

	// Available channels feature (default: disabled; strict true)

	result.PluginManagementEnabled = settings[SettingKeyPluginManagementEnabled] == "true"

	// Affiliate (邀请返利) feature (default: disabled; strict true)
	result.AffiliateEnabled = settings[SettingKeyAffiliateEnabled] == "true"

	// 风控中心功能（默认关闭，严格 true 才启用）
	result.RiskControlEnabled = settings[SettingKeyRiskControlEnabled] == "true"

	// cyber 会话屏蔽（默认关闭，TTL 默认 3600s）
	result.CyberSessionBlockEnabled = settings[SettingKeyCyberSessionBlockEnabled] == "true"
	if v, err := strconv.Atoi(strings.TrimSpace(settings[SettingKeyCyberSessionBlockTTLSeconds])); err == nil && v > 0 {
		result.CyberSessionBlockTTLSeconds = v
	} else {
		result.CyberSessionBlockTTLSeconds = 3600
	}

	// Claude Code version check
	result.MinClaudeCodeVersion = settings[SettingKeyMinClaudeCodeVersion]
	result.MaxClaudeCodeVersion = settings[SettingKeyMaxClaudeCodeVersion]

	// 分组隔离

	// Gateway forwarding behavior (defaults: fingerprint=true, metadata_passthrough=false,
	// cch_signing=false, claude_oauth_system_prompt_injection=true)
	result.OpenAITTFTMode = normalizeOpenAITTFTMode(settings[SettingKeyOpenAITTFTMode])
	if v, ok := settings[SettingKeyEnableFingerprintUnification]; ok && v != "" {
		result.EnableFingerprintUnification = v == "true"
	} else {
		result.EnableFingerprintUnification = true // default: enabled (current behavior)
	}
	result.EnableMetadataPassthrough = settings[SettingKeyEnableMetadataPassthrough] == "true"
	result.EnableCCHSigning = settings[SettingKeyEnableCCHSigning] == "true"
	if v, ok := settings[SettingKeyEnableClaudeOAuthSystemPromptInjection]; ok && v != "" {
		result.EnableClaudeOAuthSystemPromptInjection = v == "true"
	} else {
		result.EnableClaudeOAuthSystemPromptInjection = true
	}
	result.ClaudeOAuthSystemPrompt = settings[SettingKeyClaudeOAuthSystemPrompt]
	result.ClaudeOAuthSystemPromptBlocks = settings[SettingKeyClaudeOAuthSystemPromptBlocks]
	result.EnableAnthropicCacheTTL1hInjection = settings[SettingKeyEnableAnthropicCacheTTL1hInjection] == "true"
	if v, ok := settings[SettingKeyRewriteMessageCacheControl]; ok && v != "" {
		result.RewriteMessageCacheControl = v == "true"
	} else {
		result.RewriteMessageCacheControl = s.defaultRewriteMessageCacheControl()
	}
	if v, ok := settings[SettingKeyEnableClientDatelineNormalization]; ok && v != "" {
		result.EnableClientDatelineNormalization = v == "true"
	} else {
		result.EnableClientDatelineNormalization = true
	}
	result.AntigravityUserAgentVersion = antigravity.NormalizeUserAgentVersion(settings[SettingKeyAntigravityUserAgentVersion])
	result.OpenAICodexUserAgent = strings.TrimSpace(settings[SettingKeyOpenAICodexUserAgent])
	result.OpenAICodexClientVersion = NormalizeCodexClientVersion(settings[SettingKeyOpenAICodexClientVersion])
	result.OpenAICodexClientVersionSynced = NormalizeCodexClientVersion(settings[SettingKeyOpenAICodexClientVersionSynced])
	// 自动同步默认开启：缺失/空值一律视为开启，与 enable_client_dateline_normalization 同一惯例。
	if v, ok := settings[SettingKeyOpenAICodexVersionAutoSyncEnabled]; ok && v != "" {
		result.OpenAICodexVersionAutoSyncEnabled = v == "true"
	} else {
		result.OpenAICodexVersionAutoSyncEnabled = true
	}
	// codex_cli_only 加固
	result.MinCodexVersion = settings[SettingKeyMinCodexVersion]
	result.MaxCodexVersion = settings[SettingKeyMaxCodexVersion]
	result.CodexCLIOnlyBlacklist = settings[SettingKeyCodexCLIOnlyBlacklist]
	result.CodexCLIOnlyWhitelist = settings[SettingKeyCodexCLIOnlyWhitelist]
	result.CodexCLIOnlyAllowAppServerClients = settings[SettingKeyCodexCLIOnlyAllowAppServerClients] == "true"
	if raw := strings.TrimSpace(settings[SettingKeyCodexCLIOnlyEngineFingerprintSignals]); raw != "" {
		result.CodexCLIOnlyEngineFingerprintSignals = raw
	} else {
		result.CodexCLIOnlyEngineFingerprintSignals = openai.DefaultEngineFingerprintSignalsJSON() // 缺失/空 → 展示默认种子
	}

	// Web search emulation: quick enabled check from the JSON config
	if raw := settings[SettingKeyWebSearchEmulationConfig]; raw != "" {
		var wsCfg WebSearchEmulationConfig
		if err := json.Unmarshal([]byte(raw), &wsCfg); err == nil {
			result.WebSearchEmulationEnabled = wsCfg.Enabled && len(wsCfg.Providers) > 0
		}
	}

	result.AccountSchedulingThresholds = defaultAccountSchedulingThresholds()
	if raw := strings.TrimSpace(settings[SettingKeyAccountSchedulingThresholds]); raw != "" {
		if thresholds, err := parseAccountSchedulingThresholdsSetting(raw); err != nil {
			slog.Warn("[Setting] parseSettings: unmarshal account_scheduling_thresholds failed", "error", err)
		} else {
			result.AccountSchedulingThresholds = thresholds
		}
	}

	result.ProfitControlEnabled = settings[SettingKeyProfitControlEnabled] == "true"
	result.ProfitMinMargin = parseProfitControlRatio(settings[SettingKeyProfitMinMargin])
	result.ProfitSafetyBuffer = parseProfitControlRatio(settings[SettingKeyProfitSafetyBuffer])

	// Publish Grok default model_mapping options for accounts with empty mapping.
	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{
		DefaultText:          result.GrokDefaultTextModel,
		EnableCrossClientMap: result.GrokCrossClientModelMapEnabled,
	})

	return result
}

func normalizeOpenAITTFTMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), OpenAITTFTModeVisible) {
		return OpenAITTFTModeVisible
	}
	return OpenAITTFTModeSemantic
}

// parseProfitControlRatio 解析利润门的 margin / buffer：非法或越界回 0（= 不扣减）。
func parseProfitControlRatio(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > ProfitControlRatioMax {
		return 0
	}
	return value
}

func clampAffiliateRebateRate(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return AffiliateRebateRateDefault
	}
	if value < AffiliateRebateRateMin {
		return AffiliateRebateRateMin
	}
	if value > AffiliateRebateRateMax {
		return AffiliateRebateRateMax
	}
	return value
}

func isFalseSettingValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "disabled":
		return true
	default:
		return false
	}
}

func isTrueSettingValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "enabled":
		return true
	default:
		return false
	}
}

func normalizeVisibleMethodSettingSource(method, source string, enabled bool) (string, error) {
	_ = enabled
	source = strings.TrimSpace(source)
	if source == "" {
		return "", nil
	}

	normalized := NormalizeVisibleMethodSource(method, source)
	if normalized == "" {
		return "", infraerrors.BadRequest(
			"INVALID_PAYMENT_VISIBLE_METHOD_SOURCE",
			fmt.Sprintf("%s source must be one of the supported payment providers", method),
		)
	}
	return normalized, nil
}
