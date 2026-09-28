package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SettingHandler 系统设置处理器
type SettingHandler struct {
	settingService           *service.SettingService
	emailService             *service.EmailService
	opsService               *service.OpsService
	notificationEmailService *service.NotificationEmailService
}

// NewSettingHandler 创建系统设置处理器
func NewSettingHandler(settingService *service.SettingService, emailService *service.EmailService, opsService *service.OpsService) *SettingHandler {
	return &SettingHandler{
		settingService: settingService,
		emailService:   emailService,
		opsService:     opsService,
	}
}

// SetNotificationEmailService attaches the notification template service without changing
// the constructor signature used by existing unit tests.
func (h *SettingHandler) SetNotificationEmailService(notificationEmailService *service.NotificationEmailService) {
	h.notificationEmailService = notificationEmailService
}

// GetSettings 获取所有系统设置
// GET /api/v1/admin/settings
func (h *SettingHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Check if ops monitoring is enabled (respects config.ops.enabled)
	opsEnabled := h.opsService != nil && h.opsService.IsMonitoringEnabled(c.Request.Context())

	payload := dto.SystemSettings{
		RiskControlEnabled:           settings.RiskControlEnabled,
		CyberSessionBlockEnabled:     settings.CyberSessionBlockEnabled,
		CyberSessionBlockTTLSeconds:  settings.CyberSessionBlockTTLSeconds,
		AffiliateRebateRate:          settings.AffiliateRebateRate,
		AffiliateRebateFreezeHours:   settings.AffiliateRebateFreezeHours,
		AffiliateRebateDurationDays:  settings.AffiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap: settings.AffiliateRebatePerInviteeCap,
		AdminRechargeRebateEnabled:   settings.AdminRechargeRebateEnabled,
		OpsMonitoringEnabled:         opsEnabled,
		OpsRealtimeMonitoringEnabled: settings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:          settings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:    settings.OpsMetricsIntervalSeconds,
		WebSearchEmulationEnabled:    settings.WebSearchEmulationEnabled,

		ChannelMonitorMode:                   settings.ChannelMonitorMode,
		ChannelMonitorDefaultIntervalSeconds: settings.ChannelMonitorDefaultIntervalSeconds,
		ChannelMonitorHideThroughput:         settings.ChannelMonitorHideThroughput,
		ChannelMonitorShowQuota:              settings.ChannelMonitorShowQuota,
		ChannelMonitorHideUserRanking:        settings.ChannelMonitorHideUserRanking,

		AffiliateEnabled: settings.AffiliateEnabled,

		ProfitMinMargin: settings.ProfitMinMargin,
	}

	response.Success(c, payload)
}
