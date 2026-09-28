package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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

		// Ops monitoring defaults (vNext)
		SettingKeyOpsRealtimeMonitoringEnabled: "true",
		SettingKeyOpsQueryModeDefault:          "auto",
		SettingKeyOpsMetricsIntervalSeconds:    "60",

		// Channel monitor defaults (enabled, 60s)
		SettingKeyChannelMonitorMode:                   ChannelMonitorModeV2,
		SettingKeyChannelMonitorDefaultIntervalSeconds: "60",
		SettingKeyChannelMonitorHideThroughput:         "true",
		SettingKeyChannelMonitorShowQuota:              "false",
		SettingKeyChannelMonitorHideUserRanking:        "false",

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

		SettingPaymentVisibleMethodAlipaySource:  "",
		SettingPaymentVisibleMethodWxpaySource:   "",
		SettingPaymentVisibleMethodAlipayEnabled: "false",
		SettingPaymentVisibleMethodWxpayEnabled:  "false",

		SettingKeyProfitMinMargin: "0",
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

	// Ops monitoring settings (default: enabled, fail-open)
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

	// Web Search 模拟：有配了 Key 的服务商就算开（没有全局开关）
	result.WebSearchEmulationEnabled = webSearchEmulationActive(parseWebSearchConfigJSON(settings[SettingKeyWebSearchEmulationConfig]))

	result.ProfitMinMargin = parseProfitControlRatio(settings[SettingKeyProfitMinMargin])

	return result
}

// parseProfitControlRatio 解析利润门的最低毛利率：非法或越界回 0（= 不装门）。
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
