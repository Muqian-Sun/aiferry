package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {

	// 默认配置
	AffiliateRebateRate          *float64 `json:"affiliate_rebate_rate"`
	AffiliateRebateFreezeHours   *int     `json:"affiliate_rebate_freeze_hours"`
	AffiliateRebateDurationDays  *int     `json:"affiliate_rebate_duration_days"`
	AffiliateRebatePerInviteeCap *float64 `json:"affiliate_rebate_per_invitee_cap"`
	AdminRechargeRebateEnabled   *bool    `json:"affiliate_admin_recharge_enabled"`

	// Identity patch configuration (Claude -> Gemini)

	// Ops monitoring (vNext)
	OpsRealtimeMonitoringEnabled *bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          *string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    *int    `json:"ops_metrics_interval_seconds"`

	// Gateway forwarding behavior

	// Channel Monitor feature switch
	ChannelMonitorMode                   *string `json:"channel_monitor_mode"`
	ChannelMonitorDefaultIntervalSeconds *int    `json:"channel_monitor_default_interval_seconds"`
	ChannelMonitorShowQuota              *bool   `json:"channel_monitor_show_quota"`

	// Grok model mapping policy

	// Affiliate (邀请返利) feature switch
	AffiliateEnabled *bool `json:"affiliate_enabled"`

	// 风控中心功能开关
	RiskControlEnabled *bool `json:"risk_control_enabled"`

	// cyber 会话屏蔽开关 + TTL
	CyberSessionBlockEnabled    *bool `json:"cyber_session_block_enabled"`
	CyberSessionBlockTTLSeconds *int  `json:"cyber_session_block_ttl_seconds"`

	// OpenAI fast/flex policy (optional, only updated when provided)

	// 利润门：最低毛利率（全站一档；0 = 关；nil = 不修改）
	ProfitMinMargin *float64 `json:"profit_min_margin"`
}

// settingKeys 请求里带了的字段对应的设置键：只写这些，没带的键保持库里的值（D7）。
func (r UpdateSettingsRequest) settingKeys() []string {
	var keys []string
	add := func(sent bool, key string) {
		if sent {
			keys = append(keys, key)
		}
	}
	add(r.AffiliateRebateRate != nil, service.SettingKeyAffiliateRebateRate)
	add(r.AffiliateRebateFreezeHours != nil, service.SettingKeyAffiliateRebateFreezeHours)
	add(r.AffiliateRebateDurationDays != nil, service.SettingKeyAffiliateRebateDurationDays)
	add(r.AffiliateRebatePerInviteeCap != nil, service.SettingKeyAffiliateRebatePerInviteeCap)
	add(r.AdminRechargeRebateEnabled != nil, service.SettingKeyAffiliateAdminRechargeEnabled)
	add(r.OpsRealtimeMonitoringEnabled != nil, service.SettingKeyOpsRealtimeMonitoringEnabled)
	add(r.OpsQueryModeDefault != nil, service.SettingKeyOpsQueryModeDefault)
	add(r.OpsMetricsIntervalSeconds != nil, service.SettingKeyOpsMetricsIntervalSeconds)
	add(r.ChannelMonitorMode != nil, service.SettingKeyChannelMonitorMode)
	add(r.ChannelMonitorDefaultIntervalSeconds != nil, service.SettingKeyChannelMonitorDefaultIntervalSeconds)
	add(r.ChannelMonitorShowQuota != nil, service.SettingKeyChannelMonitorShowQuota)
	add(r.AffiliateEnabled != nil, service.SettingKeyAffiliateEnabled)
	add(r.RiskControlEnabled != nil, service.SettingKeyRiskControlEnabled)
	add(r.CyberSessionBlockEnabled != nil, service.SettingKeyCyberSessionBlockEnabled)
	add(r.CyberSessionBlockTTLSeconds != nil, service.SettingKeyCyberSessionBlockTTLSeconds)
	add(r.ProfitMinMargin != nil, service.SettingKeyProfitMinMargin)
	return keys
}

func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	previousSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 验证参数
	affiliateRebateRate := previousSettings.AffiliateRebateRate
	if req.AffiliateRebateRate != nil {
		affiliateRebateRate = *req.AffiliateRebateRate
	}
	if affiliateRebateRate < service.AffiliateRebateRateMin {
		affiliateRebateRate = service.AffiliateRebateRateMin
	}
	if affiliateRebateRate > service.AffiliateRebateRateMax {
		affiliateRebateRate = service.AffiliateRebateRateMax
	}
	affiliateRebateFreezeHours := previousSettings.AffiliateRebateFreezeHours
	if req.AffiliateRebateFreezeHours != nil {
		affiliateRebateFreezeHours = *req.AffiliateRebateFreezeHours
	}
	if affiliateRebateFreezeHours < 0 {
		affiliateRebateFreezeHours = service.AffiliateRebateFreezeHoursDefault
	}
	if affiliateRebateFreezeHours > service.AffiliateRebateFreezeHoursMax {
		affiliateRebateFreezeHours = service.AffiliateRebateFreezeHoursMax
	}
	affiliateRebateDurationDays := previousSettings.AffiliateRebateDurationDays
	if req.AffiliateRebateDurationDays != nil {
		affiliateRebateDurationDays = *req.AffiliateRebateDurationDays
	}
	if affiliateRebateDurationDays < 0 {
		affiliateRebateDurationDays = service.AffiliateRebateDurationDaysDefault
	}
	if affiliateRebateDurationDays > service.AffiliateRebateDurationDaysMax {
		affiliateRebateDurationDays = service.AffiliateRebateDurationDaysMax
	}
	affiliateRebatePerInviteeCap := previousSettings.AffiliateRebatePerInviteeCap
	if req.AffiliateRebatePerInviteeCap != nil {
		affiliateRebatePerInviteeCap = *req.AffiliateRebatePerInviteeCap
	}
	if affiliateRebatePerInviteeCap < 0 {
		affiliateRebatePerInviteeCap = service.AffiliateRebatePerInviteeCapDefault
	}
	adminRechargeRebateEnabled := previousSettings.AdminRechargeRebateEnabled
	if req.AdminRechargeRebateEnabled != nil {
		adminRechargeRebateEnabled = *req.AdminRechargeRebateEnabled
	}
	// Ops metrics collector interval validation (seconds).
	if req.OpsMetricsIntervalSeconds != nil {
		v := *req.OpsMetricsIntervalSeconds
		if v < 60 {
			v = 60
		}
		if v > 3600 {
			v = 3600
		}
		req.OpsMetricsIntervalSeconds = &v
	}

	// cyber 会话屏蔽 TTL 校验：提供时必须 > 0
	if req.CyberSessionBlockTTLSeconds != nil && *req.CyberSessionBlockTTLSeconds <= 0 {
		response.BadRequest(c, "cyber_session_block_ttl_seconds must be > 0")
		return
	}

	settings := &service.SystemSettings{

		AffiliateRebateRate:          affiliateRebateRate,
		AffiliateRebateFreezeHours:   affiliateRebateFreezeHours,
		AffiliateRebateDurationDays:  affiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap: affiliateRebatePerInviteeCap,
		AdminRechargeRebateEnabled:   adminRechargeRebateEnabled,
		ProfitMinMargin: func() float64 {
			if req.ProfitMinMargin != nil {
				return *req.ProfitMinMargin
			}
			return previousSettings.ProfitMinMargin
		}(),
		OpsRealtimeMonitoringEnabled: func() bool {
			if req.OpsRealtimeMonitoringEnabled != nil {
				return *req.OpsRealtimeMonitoringEnabled
			}
			return previousSettings.OpsRealtimeMonitoringEnabled
		}(),
		OpsQueryModeDefault: func() string {
			if req.OpsQueryModeDefault != nil {
				return *req.OpsQueryModeDefault
			}
			return previousSettings.OpsQueryModeDefault
		}(),
		OpsMetricsIntervalSeconds: func() int {
			if req.OpsMetricsIntervalSeconds != nil {
				return *req.OpsMetricsIntervalSeconds
			}
			return previousSettings.OpsMetricsIntervalSeconds
		}(),
		ChannelMonitorMode: func() string {
			if req.ChannelMonitorMode != nil {
				return *req.ChannelMonitorMode
			}
			return previousSettings.ChannelMonitorMode
		}(),
		ChannelMonitorDefaultIntervalSeconds: func() int {
			if req.ChannelMonitorDefaultIntervalSeconds != nil {
				return *req.ChannelMonitorDefaultIntervalSeconds
			}
			return previousSettings.ChannelMonitorDefaultIntervalSeconds
		}(),
		ChannelMonitorShowQuota: func() bool {
			if req.ChannelMonitorShowQuota != nil {
				return *req.ChannelMonitorShowQuota
			}
			return previousSettings.ChannelMonitorShowQuota
		}(),
		AffiliateEnabled: func() bool {
			if req.AffiliateEnabled != nil {
				return *req.AffiliateEnabled
			}
			return previousSettings.AffiliateEnabled
		}(),
		RiskControlEnabled: func() bool {
			if req.RiskControlEnabled != nil {
				return *req.RiskControlEnabled
			}
			return previousSettings.RiskControlEnabled
		}(),
		CyberSessionBlockEnabled: func() bool {
			if req.CyberSessionBlockEnabled != nil {
				return *req.CyberSessionBlockEnabled
			}
			return previousSettings.CyberSessionBlockEnabled
		}(),
		CyberSessionBlockTTLSeconds: func() int {
			if req.CyberSessionBlockTTLSeconds != nil {
				return *req.CyberSessionBlockTTLSeconds
			}
			return previousSettings.CyberSessionBlockTTLSeconds
		}(),
	}

	if err := h.settingService.UpdateSettings(c.Request.Context(), settings, req.settingKeys()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	h.auditSettingsUpdate(c, previousSettings, settings, req)

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	payload := dto.SystemSettings{
		AffiliateRebateRate:          updatedSettings.AffiliateRebateRate,
		AffiliateRebateFreezeHours:   updatedSettings.AffiliateRebateFreezeHours,
		AffiliateRebateDurationDays:  updatedSettings.AffiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap: updatedSettings.AffiliateRebatePerInviteeCap,
		AdminRechargeRebateEnabled:   updatedSettings.AdminRechargeRebateEnabled,
		OpsMonitoringEnabled:         h.opsService != nil && h.opsService.IsMonitoringEnabled(c.Request.Context()),
		OpsRealtimeMonitoringEnabled: updatedSettings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:          updatedSettings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:    updatedSettings.OpsMetricsIntervalSeconds,

		ChannelMonitorMode:                   updatedSettings.ChannelMonitorMode,
		ChannelMonitorDefaultIntervalSeconds: updatedSettings.ChannelMonitorDefaultIntervalSeconds,
		ChannelMonitorShowQuota:              updatedSettings.ChannelMonitorShowQuota,

		AffiliateEnabled: updatedSettings.AffiliateEnabled,

		RiskControlEnabled:          updatedSettings.RiskControlEnabled,
		CyberSessionBlockEnabled:    updatedSettings.CyberSessionBlockEnabled,
		CyberSessionBlockTTLSeconds: updatedSettings.CyberSessionBlockTTLSeconds,
		ProfitMinMargin:             updatedSettings.ProfitMinMargin,
	}
	response.Success(c, payload)
}
