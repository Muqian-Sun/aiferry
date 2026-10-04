package service

import (
	"context"
	"fmt"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// UpdateSettings 更新系统设置（整份写入；管理端对没发送的字段回填库里的旧值）
// UpdateSettings 校验 settings，只写 keys 列出的设置键（请求里带了的字段）。
// 设置页每一节单独保存：没带的键不写回，两个管理员同时改不同的节不会互相改回去（2026-10-04 D7）。
func (s *SettingService) UpdateSettings(ctx context.Context, settings *SystemSettings, keys []string) error {
	all, err := s.buildSystemSettingsUpdates(ctx, settings)
	if err != nil {
		return err
	}
	updates := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := all[key]; ok {
			updates[key] = value
		}
	}
	if len(updates) > 0 {
		if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
			return err
		}
	}
	s.refreshCachedSettings(settings)
	return nil
}

func (s *SettingService) buildSystemSettingsUpdates(ctx context.Context, settings *SystemSettings) (map[string]string, error) {
	updates := make(map[string]string)

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

	// Ops monitoring (vNext)
	updates[SettingKeyOpsRealtimeMonitoringEnabled] = strconv.FormatBool(settings.OpsRealtimeMonitoringEnabled)
	updates[SettingKeyOpsQueryModeDefault] = string(ParseOpsQueryMode(settings.OpsQueryModeDefault))
	if settings.OpsMetricsIntervalSeconds > 0 {
		updates[SettingKeyOpsMetricsIntervalSeconds] = strconv.Itoa(settings.OpsMetricsIntervalSeconds)
	}

	// Channel monitor feature switch
	// V1 已下线：不管请求带什么都写 v2，库里存量的 v1 在下次保存时对齐
	updates[SettingKeyChannelMonitorMode] = ChannelMonitorModeV2
	if v := clampChannelMonitorInterval(settings.ChannelMonitorDefaultIntervalSeconds); v > 0 {
		updates[SettingKeyChannelMonitorDefaultIntervalSeconds] = strconv.Itoa(v)
	}
	updates[SettingKeyChannelMonitorShowQuota] = strconv.FormatBool(settings.ChannelMonitorShowQuota)

	// Available channels feature switch

	// Affiliate (邀请返利) feature switch
	updates[SettingKeyAffiliateEnabled] = strconv.FormatBool(settings.AffiliateEnabled)

	// 风控中心功能开关
	updates[SettingKeyRiskControlEnabled] = strconv.FormatBool(settings.RiskControlEnabled)

	// 利润门只有最低毛利率一个数，0 = 关。
	if settings.ProfitMinMargin < 0 || settings.ProfitMinMargin > ProfitControlRatioMax {
		return nil, infraerrors.BadRequest("INVALID_PROFIT_CONTROL", fmt.Sprintf("profit control: min_margin must be within [0, %g]", ProfitControlRatioMax))
	}
	updates[SettingKeyProfitMinMargin] = strconv.FormatFloat(settings.ProfitMinMargin, 'f', 8, 64)

	return updates, nil
}

func (s *SettingService) refreshCachedSettings(settings *SystemSettings) {
	if settings == nil {
		return
	}

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
	if s.onUpdate != nil {
		s.onUpdate() // Invalidate cache after settings update
	}
	s.notifyChannelMonitorRuntimeListeners()
}
