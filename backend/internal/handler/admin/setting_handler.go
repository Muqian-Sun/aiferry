package admin

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// semverPattern 预编译 semver 格式校验正则
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// SettingHandler 系统设置处理器
type SettingHandler struct {
	settingService           *service.SettingService
	emailService             *service.EmailService
	turnstileService         *service.TurnstileService
	aliyunCaptchaService     *service.AliyunCaptchaService
	opsService               *service.OpsService
	paymentConfigService     *service.PaymentConfigService
	paymentService           *service.PaymentService
	notificationEmailService *service.NotificationEmailService
	totpService              *service.TotpService
	userService              *service.UserService
}

// NewSettingHandler 创建系统设置处理器
func NewSettingHandler(settingService *service.SettingService, emailService *service.EmailService, turnstileService *service.TurnstileService, opsService *service.OpsService, paymentConfigService *service.PaymentConfigService, paymentService *service.PaymentService) *SettingHandler {
	return &SettingHandler{
		settingService:       settingService,
		emailService:         emailService,
		turnstileService:     turnstileService,
		opsService:           opsService,
		paymentConfigService: paymentConfigService,
		paymentService:       paymentService,
	}
}

// SetNotificationEmailService attaches the notification template service without changing
// the constructor signature used by existing unit tests.
func (h *SettingHandler) SetNotificationEmailService(notificationEmailService *service.NotificationEmailService) {
	h.notificationEmailService = notificationEmailService
}

// SetAliyunCaptchaService attaches the Aliyun captcha credential validator without
// changing the constructor signature used by existing unit tests.
func (h *SettingHandler) SetAliyunCaptchaService(aliyunCaptchaService *service.AliyunCaptchaService) {
	h.aliyunCaptchaService = aliyunCaptchaService
}

// SetStepUpDeps attaches the services backing the step-up switch preconditions
// (enable requires the acting admin to have TOTP enabled; disable is itself a
// step-up gated operation), without changing the constructor signature used by
// existing unit tests.
func (h *SettingHandler) SetStepUpDeps(totpService *service.TotpService, userService *service.UserService) {
	h.totpService = totpService
	h.userService = userService
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

	// Load payment config
	var paymentCfg *service.PaymentConfig
	if h.paymentConfigService != nil {
		paymentCfg, _ = h.paymentConfigService.GetPaymentConfig(c.Request.Context())
	}
	if paymentCfg == nil {
		paymentCfg = &service.PaymentConfig{}
	}

	payload := dto.SystemSettings{
		TurnstileEnabled:                       settings.TurnstileEnabled,
		TurnstileSiteKey:                       settings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:           settings.TurnstileSecretKeyConfigured,
		TencentCaptchaEnabled:                  settings.TencentCaptchaEnabled,
		TencentCaptchaAppID:                    settings.TencentCaptchaAppID,
		TencentCaptchaAppSecretKeyConfigured:   settings.TencentCaptchaAppSecretKeyConfigured,
		TencentCaptchaCloudSecretIDConfigured:  settings.TencentCaptchaCloudSecretIDConfigured,
		TencentCaptchaCloudSecretKeyConfigured: settings.TencentCaptchaCloudSecretKeyConfigured,
		TencentCaptchaRegion:                   settings.TencentCaptchaRegion,
		AliyunCaptchaEnabled:                   settings.AliyunCaptchaEnabled,
		AliyunCaptchaAccessKeyID:               settings.AliyunCaptchaAccessKeyID,
		AliyunCaptchaAccessKeySecretConfigured: settings.AliyunCaptchaAccessKeySecretConfigured,
		AliyunCaptchaSceneID:                   settings.AliyunCaptchaSceneID,
		AliyunCaptchaPrefix:                    settings.AliyunCaptchaPrefix,
		AliyunCaptchaRegion:                    settings.AliyunCaptchaRegion,
		RiskControlEnabled:                     settings.RiskControlEnabled,
		CyberSessionBlockEnabled:               settings.CyberSessionBlockEnabled,
		CyberSessionBlockTTLSeconds:            settings.CyberSessionBlockTTLSeconds,
		AffiliateRebateRate:                    settings.AffiliateRebateRate,
		AffiliateRebateFreezeHours:             settings.AffiliateRebateFreezeHours,
		AffiliateRebateDurationDays:            settings.AffiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap:           settings.AffiliateRebatePerInviteeCap,
		AdminRechargeRebateEnabled:             settings.AdminRechargeRebateEnabled,
		EnableIdentityPatch:                    settings.EnableIdentityPatch,
		IdentityPatchPrompt:                    settings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                   opsEnabled && settings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:           settings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                    settings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:              settings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                   settings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                   settings.MaxClaudeCodeVersion,
		OpenAITTFTMode:                         settings.OpenAITTFTMode,
		EnableFingerprintUnification:           settings.EnableFingerprintUnification,
		EnableMetadataPassthrough:              settings.EnableMetadataPassthrough,
		EnableCCHSigning:                       settings.EnableCCHSigning,
		EnableClaudeOAuthSystemPromptInjection: settings.EnableClaudeOAuthSystemPromptInjection,
		ClaudeOAuthSystemPrompt:                settings.ClaudeOAuthSystemPrompt,
		ClaudeOAuthSystemPromptBlocks:          settings.ClaudeOAuthSystemPromptBlocks,
		EnableAnthropicCacheTTL1hInjection:     settings.EnableAnthropicCacheTTL1hInjection,
		RewriteMessageCacheControl:             settings.RewriteMessageCacheControl,
		EnableClientDatelineNormalization:      settings.EnableClientDatelineNormalization,
		AntigravityUserAgentVersion:            settings.AntigravityUserAgentVersion,
		OpenAICodexUserAgent:                   settings.OpenAICodexUserAgent,
		OpenAICodexClientVersion:               settings.OpenAICodexClientVersion,
		OpenAICodexClientVersionSynced:         settings.OpenAICodexClientVersionSynced,
		OpenAICodexVersionAutoSyncEnabled:      settings.OpenAICodexVersionAutoSyncEnabled,
		MinCodexVersion:                        settings.MinCodexVersion,
		MaxCodexVersion:                        settings.MaxCodexVersion,
		CodexCLIOnlyBlacklist:                  settings.CodexCLIOnlyBlacklist,
		CodexCLIOnlyWhitelist:                  settings.CodexCLIOnlyWhitelist,
		CodexCLIOnlyAllowAppServerClients:      settings.CodexCLIOnlyAllowAppServerClients,
		CodexCLIOnlyEngineFingerprintSignals:   settings.CodexCLIOnlyEngineFingerprintSignals,
		WebSearchEmulationEnabled:              settings.WebSearchEmulationEnabled,
		PaymentVisibleMethodAlipaySource:       settings.PaymentVisibleMethodAlipaySource,
		PaymentVisibleMethodWxpaySource:        settings.PaymentVisibleMethodWxpaySource,
		PaymentVisibleMethodAlipayEnabled:      settings.PaymentVisibleMethodAlipayEnabled,
		PaymentVisibleMethodWxpayEnabled:       settings.PaymentVisibleMethodWxpayEnabled,
		PaymentEnabled:                         paymentCfg.Enabled,
		PaymentMinAmount:                       paymentCfg.MinAmount,
		PaymentMaxAmount:                       paymentCfg.MaxAmount,
		PaymentDailyLimit:                      paymentCfg.DailyLimit,
		PaymentOrderTimeoutMin:                 paymentCfg.OrderTimeoutMin,
		PaymentMaxPendingOrders:                paymentCfg.MaxPendingOrders,
		PaymentEnabledTypes:                    paymentCfg.EnabledTypes,
		PaymentUSDToCNYRate:                    paymentCfg.USDToCNYRate,
		PaymentRechargeFeeRate:                 paymentCfg.RechargeFeeRate,
		PaymentLoadBalanceStrat:                paymentCfg.LoadBalanceStrategy,
		PaymentProductNamePrefix:               paymentCfg.ProductNamePrefix,
		PaymentProductNameSuffix:               paymentCfg.ProductNameSuffix,
		PaymentHelpImageURL:                    paymentCfg.HelpImageURL,
		PaymentHelpText:                        paymentCfg.HelpText,
		PaymentCancelRateLimitEnabled:          paymentCfg.CancelRateLimitEnabled,
		PaymentCancelRateLimitMax:              paymentCfg.CancelRateLimitMax,
		PaymentCancelRateLimitWindow:           paymentCfg.CancelRateLimitWindow,
		PaymentCancelRateLimitUnit:             paymentCfg.CancelRateLimitUnit,
		PaymentCancelRateLimitMode:             paymentCfg.CancelRateLimitMode,
		PaymentAlipayForceQRCode:               paymentCfg.AlipayForceQRCode,
		PaymentAlipayMobilePrecreateDeepLink:   paymentCfg.AlipayMobilePrecreateDeepLink,

		ChannelMonitorEnabled:                settings.ChannelMonitorEnabled,
		ChannelMonitorMode:                   settings.ChannelMonitorMode,
		ChannelMonitorDefaultIntervalSeconds: settings.ChannelMonitorDefaultIntervalSeconds,
		ChannelMonitorHideThroughput:         settings.ChannelMonitorHideThroughput,
		ChannelMonitorShowQuota:              settings.ChannelMonitorShowQuota,
		ChannelMonitorHideUserRanking:        settings.ChannelMonitorHideUserRanking,

		GrokDefaultTextModel:           settings.GrokDefaultTextModel,
		GrokCrossClientModelMapEnabled: settings.GrokCrossClientModelMapEnabled,
		GrokDefaultBaseURLMode:         settings.GrokDefaultBaseURLMode,

		PluginManagementEnabled: settings.PluginManagementEnabled,

		AffiliateEnabled: settings.AffiliateEnabled,

		AccountSchedulingThresholds: settings.AccountSchedulingThresholds,
		ProfitControlEnabled:        settings.ProfitControlEnabled,
		ProfitMinMargin:             settings.ProfitMinMargin,
		ProfitSafetyBuffer:          settings.ProfitSafetyBuffer,
	}

	// OpenAI fast policy (stored under a dedicated setting key)
	if fastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context()); err != nil {
		slog.Error("openai_fast_policy_settings_get_failed", "error", err)
	} else if fastPolicy != nil {
		payload.OpenAIFastPolicySettings = openaiFastPolicySettingsToDTO(fastPolicy)
	}

	response.Success(c, payload)
}

// openaiFastPolicySettingsToDTO converts service -> dto for OpenAI fast policy.
func openaiFastPolicySettingsToDTO(s *service.OpenAIFastPolicySettings) *dto.OpenAIFastPolicySettings {
	if s == nil {
		return nil
	}
	rules := make([]dto.OpenAIFastPolicyRule, len(s.Rules))
	for i, r := range s.Rules {
		rules[i] = dto.OpenAIFastPolicyRule(r)
	}
	return &dto.OpenAIFastPolicySettings{Rules: rules}
}

// openaiFastPolicySettingsFromDTO converts dto -> service for OpenAI fast policy.
//
// 规范化 ServiceTier：在 DTO 进入 service 层之前统一把空字符串归一为
// service.OpenAIFastTierAny ("all")，避免管理员保存时空串与 "all" 同时
// 表达"匹配任意 tier"造成数据库取值的二义性。其它非空值原样透传，由
// service.SetOpenAIFastPolicySettings 负责合法值校验。
func openaiFastPolicySettingsFromDTO(s *dto.OpenAIFastPolicySettings) *service.OpenAIFastPolicySettings {
	if s == nil {
		return nil
	}
	rules := make([]service.OpenAIFastPolicyRule, len(s.Rules))
	for i, r := range s.Rules {
		rules[i] = service.OpenAIFastPolicyRule(r)
		tier := strings.ToLower(strings.TrimSpace(rules[i].ServiceTier))
		if tier == "" {
			tier = service.OpenAIFastTierAny
		}
		rules[i].ServiceTier = tier
	}
	return &service.OpenAIFastPolicySettings{Rules: rules}
}
