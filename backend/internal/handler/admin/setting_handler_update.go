package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {

	// Cloudflare Turnstile 设置
	TurnstileEnabled   bool   `json:"turnstile_enabled"`
	TurnstileSiteKey   string `json:"turnstile_site_key"`
	TurnstileSecretKey string `json:"turnstile_secret_key"`

	// 腾讯天御验证码设置
	TencentCaptchaEnabled        bool   `json:"tencent_captcha_enabled"`
	TencentCaptchaAppID          string `json:"tencent_captcha_app_id"`
	TencentCaptchaAppSecretKey   string `json:"tencent_captcha_app_secret_key"`
	TencentCaptchaCloudSecretID  string `json:"tencent_captcha_cloud_secret_id"`
	TencentCaptchaCloudSecretKey string `json:"tencent_captcha_cloud_secret_key"`
	TencentCaptchaRegion         string `json:"tencent_captcha_region"`

	// 阿里云验证码 2.0 设置
	AliyunCaptchaEnabled         bool   `json:"aliyun_captcha_enabled"`
	AliyunCaptchaAccessKeyID     string `json:"aliyun_captcha_access_key_id"`
	AliyunCaptchaAccessKeySecret string `json:"aliyun_captcha_access_key_secret"`
	AliyunCaptchaSceneID         string `json:"aliyun_captcha_scene_id"`
	AliyunCaptchaPrefix          string `json:"aliyun_captcha_prefix"`
	AliyunCaptchaRegion          string `json:"aliyun_captcha_region"`

	// 默认配置
	AffiliateRebateRate          *float64 `json:"affiliate_rebate_rate"`
	AffiliateRebateFreezeHours   *int     `json:"affiliate_rebate_freeze_hours"`
	AffiliateRebateDurationDays  *int     `json:"affiliate_rebate_duration_days"`
	AffiliateRebatePerInviteeCap *float64 `json:"affiliate_rebate_per_invitee_cap"`
	AdminRechargeRebateEnabled   *bool    `json:"affiliate_admin_recharge_enabled"`

	// Identity patch configuration (Claude -> Gemini)
	EnableIdentityPatch bool   `json:"enable_identity_patch"`
	IdentityPatchPrompt string `json:"identity_patch_prompt"`

	// Ops monitoring (vNext)
	OpsMonitoringEnabled         *bool   `json:"ops_monitoring_enabled"`
	OpsRealtimeMonitoringEnabled *bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          *string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    *int    `json:"ops_metrics_interval_seconds"`

	MinClaudeCodeVersion string `json:"min_claude_code_version"`
	MaxClaudeCodeVersion string `json:"max_claude_code_version"`

	// Gateway forwarding behavior
	OpenAITTFTMode                         *string `json:"openai_ttft_mode"`
	EnableFingerprintUnification           *bool   `json:"enable_fingerprint_unification"`
	EnableMetadataPassthrough              *bool   `json:"enable_metadata_passthrough"`
	EnableCCHSigning                       *bool   `json:"enable_cch_signing"`
	EnableClaudeOAuthSystemPromptInjection *bool   `json:"enable_claude_oauth_system_prompt_injection"`
	ClaudeOAuthSystemPrompt                *string `json:"claude_oauth_system_prompt"`
	ClaudeOAuthSystemPromptBlocks          *string `json:"claude_oauth_system_prompt_blocks"`
	EnableAnthropicCacheTTL1hInjection     *bool   `json:"enable_anthropic_cache_ttl_1h_injection"`
	RewriteMessageCacheControl             *bool   `json:"rewrite_message_cache_control"`
	EnableClientDatelineNormalization      *bool   `json:"enable_client_dateline_normalization"`
	AntigravityUserAgentVersion            *string `json:"antigravity_user_agent_version"`
	OpenAICodexUserAgent                   *string `json:"openai_codex_user_agent"`
	OpenAICodexClientVersion               *string `json:"openai_codex_client_version"`
	OpenAICodexVersionAutoSyncEnabled      *bool   `json:"openai_codex_version_auto_sync_enabled"`

	// codex_cli_only 加固（global-only）
	MinCodexVersion                      string `json:"min_codex_version"`
	MaxCodexVersion                      string `json:"max_codex_version"`
	CodexCLIOnlyBlacklist                string `json:"codex_cli_only_blacklist"`
	CodexCLIOnlyWhitelist                string `json:"codex_cli_only_whitelist"`
	CodexCLIOnlyAllowAppServerClients    *bool  `json:"codex_cli_only_allow_app_server_clients"`
	CodexCLIOnlyEngineFingerprintSignals string `json:"codex_cli_only_engine_fingerprint_signals"`

	// Payment visible method routing
	PaymentVisibleMethodAlipaySource  *string `json:"payment_visible_method_alipay_source"`
	PaymentVisibleMethodWxpaySource   *string `json:"payment_visible_method_wxpay_source"`
	PaymentVisibleMethodAlipayEnabled *bool   `json:"payment_visible_method_alipay_enabled"`
	PaymentVisibleMethodWxpayEnabled  *bool   `json:"payment_visible_method_wxpay_enabled"`

	// Payment configuration (integrated into settings, full replace)
	PaymentEnabled           *bool    `json:"payment_enabled"`
	PaymentMinAmount         *float64 `json:"payment_min_amount"`
	PaymentMaxAmount         *float64 `json:"payment_max_amount"`
	PaymentDailyLimit        *float64 `json:"payment_daily_limit"`
	PaymentOrderTimeoutMin   *int     `json:"payment_order_timeout_minutes"`
	PaymentMaxPendingOrders  *int     `json:"payment_max_pending_orders"`
	PaymentEnabledTypes      []string `json:"payment_enabled_types"`
	PaymentUSDToCNYRate      *float64 `json:"payment_usd_to_cny_rate"`
	PaymentRechargeFeeRate   *float64 `json:"payment_recharge_fee_rate"`
	PaymentLoadBalanceStrat  *string  `json:"payment_load_balance_strategy"`
	PaymentProductNamePrefix *string  `json:"payment_product_name_prefix"`
	PaymentProductNameSuffix *string  `json:"payment_product_name_suffix"`
	PaymentHelpImageURL      *string  `json:"payment_help_image_url"`
	PaymentHelpText          *string  `json:"payment_help_text"`

	// Cancel rate limit
	PaymentCancelRateLimitEnabled *bool   `json:"payment_cancel_rate_limit_enabled"`
	PaymentCancelRateLimitMax     *int    `json:"payment_cancel_rate_limit_max"`
	PaymentCancelRateLimitWindow  *int    `json:"payment_cancel_rate_limit_window"`
	PaymentCancelRateLimitUnit    *string `json:"payment_cancel_rate_limit_unit"`
	PaymentCancelRateLimitMode    *string `json:"payment_cancel_rate_limit_window_mode"`

	// Force Alipay mobile clients to use QR code payment instead of mobile redirect
	PaymentAlipayForceQRCode *bool `json:"payment_alipay_force_qrcode"`
	// Use Alipay face-to-face precreate and an app deep link on mobile clients.
	PaymentAlipayMobilePrecreateDeepLink *bool `json:"payment_alipay_mobile_precreate_deep_link"`

	// Channel Monitor feature switch
	ChannelMonitorEnabled                *bool   `json:"channel_monitor_enabled"`
	ChannelMonitorMode                   *string `json:"channel_monitor_mode"`
	ChannelMonitorDefaultIntervalSeconds *int    `json:"channel_monitor_default_interval_seconds"`
	ChannelMonitorHideThroughput         *bool   `json:"channel_monitor_hide_throughput"`
	ChannelMonitorShowQuota              *bool   `json:"channel_monitor_show_quota"`
	ChannelMonitorHideUserRanking        *bool   `json:"channel_monitor_hide_user_ranking"`

	// Grok model mapping policy
	GrokDefaultTextModel           *string `json:"grok_default_text_model"`
	GrokCrossClientModelMapEnabled *bool   `json:"grok_cross_client_model_map_enabled"`
	GrokDefaultBaseURLMode         *string `json:"grok_default_base_url_mode"`

	// Plugin management menu visibility switch; plugin runtime is unaffected.
	PluginManagementEnabled *bool `json:"plugin_management_enabled"`

	// Affiliate (邀请返利) feature switch
	AffiliateEnabled *bool `json:"affiliate_enabled"`

	// 风控中心功能开关
	RiskControlEnabled *bool `json:"risk_control_enabled"`

	// cyber 会话屏蔽开关 + TTL
	CyberSessionBlockEnabled    *bool `json:"cyber_session_block_enabled"`
	CyberSessionBlockTTLSeconds *int  `json:"cyber_session_block_ttl_seconds"`

	// OpenAI fast/flex policy (optional, only updated when provided)
	OpenAIFastPolicySettings *dto.OpenAIFastPolicySettings `json:"openai_fast_policy_settings,omitempty"`

	// 各平台账号自动停调阈值（整体替换语义：nil = 不修改，non-nil = 整体覆盖）。
	AccountSchedulingThresholds map[string]int `json:"account_scheduling_thresholds"`

	// 利润门（全站一档；nil = 不修改）
	ProfitControlEnabled *bool    `json:"profit_control_enabled"`
	ProfitMinMargin      *float64 `json:"profit_min_margin"`
	ProfitSafetyBuffer   *float64 `json:"profit_safety_buffer"`
}

// settingKeyByJSONName maps the value-typed top-level JSON fields of
// UpdateSettingsRequest to the setting key each one writes. Resolved once from
// the struct tags so new fields are covered without touching this file.
//
// Pointer-typed fields are deliberately excluded: they already carry their own
// "omitted = keep the stored value" merge in UpdateSettings, and some of them
// rely on being rewritten on every save to re-normalize fail-closed security
// state (see TestUpdateSettingsMalformedForwardedClientIPHeadersRemainFailClosedWhenOmitted).
// Only the value-typed fields are indistinguishable from a deliberate clear.
var settingKeyByJSONName = buildSettingKeyByJSONName()

func buildSettingKeyByJSONName() map[string]string {
	t := reflect.TypeOf(UpdateSettingsRequest{})
	out := make(map[string]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() == reflect.Pointer {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		out[name] = name
	}
	return out
}

// omittedSettingKeys reports the setting keys this payload never mentioned.
// Saving settings is a whole-document PUT, so without this a client that sends
// only the one field it cares about resets every other field to a zero value.
func omittedSettingKeys(sentFields map[string]json.RawMessage) service.OmittedSettingKeys {
	omitted := make(service.OmittedSettingKeys, len(settingKeyByJSONName))
	for jsonName, settingKey := range settingKeyByJSONName {
		if _, sent := sentFields[jsonName]; !sent {
			omitted[settingKey] = struct{}{}
		}
	}
	return omitted
}

func settingsAuditRequest(req UpdateSettingsRequest) UpdateSettingsRequest {
	req.TencentCaptchaAppSecretKey = strings.TrimSpace(req.TencentCaptchaAppSecretKey)
	req.TencentCaptchaCloudSecretID = strings.TrimSpace(req.TencentCaptchaCloudSecretID)
	req.TencentCaptchaCloudSecretKey = strings.TrimSpace(req.TencentCaptchaCloudSecretKey)
	req.AliyunCaptchaAccessKeySecret = strings.TrimSpace(req.AliyunCaptchaAccessKeySecret)
	return req
}

func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var sentFields map[string]json.RawMessage
	if err := c.ShouldBindBodyWith(&sentFields, binding.JSON); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var req UpdateSettingsRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	auditReq := settingsAuditRequest(req)
	omitted := omittedSettingKeys(sentFields)

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
	req.TencentCaptchaAppID = strings.TrimSpace(req.TencentCaptchaAppID)
	req.TencentCaptchaAppSecretKey = strings.TrimSpace(req.TencentCaptchaAppSecretKey)
	req.TencentCaptchaCloudSecretID = strings.TrimSpace(req.TencentCaptchaCloudSecretID)
	req.TencentCaptchaCloudSecretKey = strings.TrimSpace(req.TencentCaptchaCloudSecretKey)

	turnstileEnabled := req.TurnstileEnabled
	if _, sent := sentFields["turnstile_enabled"]; !sent {
		turnstileEnabled = previousSettings.TurnstileEnabled
	}
	tencentCaptchaEnabled := req.TencentCaptchaEnabled
	if _, sent := sentFields["tencent_captcha_enabled"]; !sent {
		tencentCaptchaEnabled = previousSettings.TencentCaptchaEnabled
	}
	aliyunCaptchaEnabled := req.AliyunCaptchaEnabled
	if _, sent := sentFields["aliyun_captcha_enabled"]; !sent {
		aliyunCaptchaEnabled = previousSettings.AliyunCaptchaEnabled
	}
	enabledCaptchaProviders := 0
	for _, enabled := range []bool{turnstileEnabled, tencentCaptchaEnabled, aliyunCaptchaEnabled} {
		if enabled {
			enabledCaptchaProviders++
		}
	}
	if enabledCaptchaProviders > 1 {
		response.BadRequest(c, "Multiple captcha providers (Cloudflare Turnstile / Tencent Captcha / Aliyun Captcha) cannot be enabled at the same time")
		return
	}
	// 阿里云地域 normalize：未发送保留已存值，非法值一律按中国内地落库
	if _, sent := sentFields["aliyun_captcha_region"]; !sent {
		req.AliyunCaptchaRegion = previousSettings.AliyunCaptchaRegion
	}
	if req.AliyunCaptchaRegion != service.AliyunCaptchaRegionSGP {
		req.AliyunCaptchaRegion = service.AliyunCaptchaRegionCN
	}
	// 天御站点 normalize：未发送保留已存值，非法值一律按中国站落库
	if _, sent := sentFields["tencent_captcha_region"]; !sent {
		req.TencentCaptchaRegion = previousSettings.TencentCaptchaRegion
	}
	if req.TencentCaptchaRegion != service.TencentCaptchaRegionINTL {
		req.TencentCaptchaRegion = service.TencentCaptchaRegionCN
	}

	// Turnstile 参数验证
	if req.TurnstileEnabled {
		// 检查必填字段
		if req.TurnstileSiteKey == "" {
			response.BadRequest(c, "Turnstile Site Key is required when enabled")
			return
		}
		// 如果未提供 secret key，使用已保存的值（留空保留当前值）
		if req.TurnstileSecretKey == "" {
			if previousSettings.TurnstileSecretKey == "" {
				response.BadRequest(c, "Turnstile Secret Key is required when enabled")
				return
			}
			req.TurnstileSecretKey = previousSettings.TurnstileSecretKey
		}

		// 当 site_key 或 secret_key 任一变化时验证（避免配置错误导致无法登录）
		siteKeyChanged := previousSettings.TurnstileSiteKey != req.TurnstileSiteKey
		secretKeyChanged := previousSettings.TurnstileSecretKey != req.TurnstileSecretKey
		if siteKeyChanged || secretKeyChanged {
			if err := h.turnstileService.ValidateSecretKey(c.Request.Context(), req.TurnstileSecretKey); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
	}

	if tencentCaptchaEnabled {
		if _, sent := sentFields["tencent_captcha_app_id"]; !sent {
			req.TencentCaptchaAppID = previousSettings.TencentCaptchaAppID
		}
		appID, err := strconv.ParseUint(req.TencentCaptchaAppID, 10, 64)
		if err != nil || appID == 0 {
			response.BadRequest(c, "Tencent Captcha CaptchaAppId must be a positive integer when enabled")
			return
		}
		if req.TencentCaptchaAppSecretKey == "" {
			req.TencentCaptchaAppSecretKey = previousSettings.TencentCaptchaAppSecretKey
		}
		if req.TencentCaptchaCloudSecretID == "" {
			req.TencentCaptchaCloudSecretID = previousSettings.TencentCaptchaCloudSecretID
		}
		if req.TencentCaptchaCloudSecretKey == "" {
			req.TencentCaptchaCloudSecretKey = previousSettings.TencentCaptchaCloudSecretKey
		}
		if req.TencentCaptchaAppSecretKey == "" {
			response.BadRequest(c, "Tencent Captcha AppSecretKey is required when enabled")
			return
		}
		if req.TencentCaptchaCloudSecretID == "" {
			response.BadRequest(c, "Tencent Cloud SecretId is required when Tencent Captcha is enabled")
			return
		}
		if req.TencentCaptchaCloudSecretKey == "" {
			response.BadRequest(c, "Tencent Cloud SecretKey is required when Tencent Captcha is enabled")
			return
		}
	}

	// 阿里云验证码 2.0 参数验证
	if aliyunCaptchaEnabled {
		if _, sent := sentFields["aliyun_captcha_scene_id"]; !sent {
			req.AliyunCaptchaSceneID = previousSettings.AliyunCaptchaSceneID
		}
		if _, sent := sentFields["aliyun_captcha_prefix"]; !sent {
			req.AliyunCaptchaPrefix = previousSettings.AliyunCaptchaPrefix
		}
		if _, sent := sentFields["aliyun_captcha_access_key_id"]; !sent {
			req.AliyunCaptchaAccessKeyID = previousSettings.AliyunCaptchaAccessKeyID
		}
		if req.AliyunCaptchaSceneID == "" {
			response.BadRequest(c, "Aliyun Captcha Scene ID is required when enabled")
			return
		}
		if req.AliyunCaptchaPrefix == "" {
			response.BadRequest(c, "Aliyun Captcha Prefix is required when enabled")
			return
		}
		if req.AliyunCaptchaAccessKeyID == "" {
			response.BadRequest(c, "Aliyun Captcha AccessKey ID is required when enabled")
			return
		}
		// 如果未提供 AccessKey Secret，使用已保存的值（留空保留当前值）
		if req.AliyunCaptchaAccessKeySecret == "" {
			if previousSettings.AliyunCaptchaAccessKeySecret == "" {
				response.BadRequest(c, "Aliyun Captcha AccessKey Secret is required when enabled")
				return
			}
			req.AliyunCaptchaAccessKeySecret = previousSettings.AliyunCaptchaAccessKeySecret
		}

		// 凭证任一变化时真实调用一次阿里云校验（避免配置错误导致无法登录）
		credentialsChanged := previousSettings.AliyunCaptchaAccessKeyID != req.AliyunCaptchaAccessKeyID ||
			previousSettings.AliyunCaptchaAccessKeySecret != req.AliyunCaptchaAccessKeySecret ||
			previousSettings.AliyunCaptchaSceneID != req.AliyunCaptchaSceneID ||
			previousSettings.AliyunCaptchaRegion != req.AliyunCaptchaRegion
		if credentialsChanged {
			if err := h.aliyunCaptchaService.ValidateCredentials(c.Request.Context(), req.AliyunCaptchaAccessKeyID, req.AliyunCaptchaAccessKeySecret, req.AliyunCaptchaSceneID, req.AliyunCaptchaRegion); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
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

	// 验证最低版本号格式（空字符串=禁用，或合法 semver）
	if req.MinClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MinClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "min_claude_code_version must be empty or a valid semver (e.g. 2.1.63)")
			return
		}
	}

	// 验证最高版本号格式（空字符串=禁用，或合法 semver）
	if req.MaxClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MaxClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be empty or a valid semver (e.g. 3.0.0)")
			return
		}
	}
	if req.AntigravityUserAgentVersion != nil {
		normalized := strings.TrimSpace(*req.AntigravityUserAgentVersion)
		req.AntigravityUserAgentVersion = &normalized
		if normalized != "" && !semverPattern.MatchString(normalized) {
			response.Error(c, http.StatusBadRequest, "antigravity_user_agent_version must be empty or a valid semver (e.g. 1.23.2)")
			return
		}
	}
	if req.OpenAICodexUserAgent != nil {
		normalized := strings.TrimSpace(*req.OpenAICodexUserAgent)
		req.OpenAICodexUserAgent = &normalized
		// 仅做长度上限保护，不限制具体格式（运维需要可自由调整 codex 版本号）
		if len(normalized) > 512 {
			response.Error(c, http.StatusBadRequest, "openai_codex_user_agent must be at most 512 characters")
			return
		}
	}
	if req.OpenAICodexClientVersion != nil {
		// 该值会被拼进出站 User-Agent 与 version 头，必须是合法版本号；空串表示跟随自动同步。
		normalized := strings.TrimSpace(*req.OpenAICodexClientVersion)
		if normalized != "" && service.NormalizeCodexClientVersion(normalized) == "" {
			response.Error(c, http.StatusBadRequest, "openai_codex_client_version must be empty or a valid version (e.g. 0.146.0)")
			return
		}
		req.OpenAICodexClientVersion = &normalized
	}

	// codex_cli_only 加固：最低/最高 Codex 版本（空=禁用，或合法 semver；max>=min）
	if req.MinCodexVersion != "" && !semverPattern.MatchString(req.MinCodexVersion) {
		response.Error(c, http.StatusBadRequest, "min_codex_version must be empty or a valid semver (e.g. 0.141.0)")
		return
	}
	if req.MaxCodexVersion != "" && !semverPattern.MatchString(req.MaxCodexVersion) {
		response.Error(c, http.StatusBadRequest, "max_codex_version must be empty or a valid semver (e.g. 0.200.0)")
		return
	}
	if req.MinCodexVersion != "" && req.MaxCodexVersion != "" && service.CompareVersions(req.MaxCodexVersion, req.MinCodexVersion) < 0 {
		response.Error(c, http.StatusBadRequest, "max_codex_version must be greater than or equal to min_codex_version")
		return
	}
	// codex_cli_only 黑/白名单：非空须为合法 []AllowedClientEntry JSON。
	// 黑名单 OR 宽 deny（允许 originator-only）；白名单双因子 AND，额外要求每条可命中（非空 originator + ua_contains）。
	if err := service.ValidateCodexClientEntriesJSON(req.CodexCLIOnlyBlacklist); err != nil {
		response.Error(c, http.StatusBadRequest, "codex_cli_only_blacklist "+err.Error())
		return
	}
	if err := service.ValidateCodexWhitelistEntriesJSON(req.CodexCLIOnlyWhitelist); err != nil {
		response.Error(c, http.StatusBadRequest, "codex_cli_only_whitelist "+err.Error())
		return
	}
	if err := service.ValidateEngineFingerprintSignalsJSON(req.CodexCLIOnlyEngineFingerprintSignals); err != nil {
		response.Error(c, http.StatusBadRequest, "codex_cli_only_engine_fingerprint_signals "+err.Error())
		return
	}

	// 交叉验证：如果同时设置了最低和最高版本号，最高版本号必须 >= 最低版本号
	if req.MinClaudeCodeVersion != "" && req.MaxClaudeCodeVersion != "" {
		if service.CompareVersions(req.MaxClaudeCodeVersion, req.MinClaudeCodeVersion) < 0 {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be greater than or equal to min_claude_code_version")
			return
		}
	}

	// cyber 会话屏蔽 TTL 校验：提供时必须 > 0
	if req.CyberSessionBlockTTLSeconds != nil && *req.CyberSessionBlockTTLSeconds <= 0 {
		response.BadRequest(c, "cyber_session_block_ttl_seconds must be > 0")
		return
	}

	settings := &service.SystemSettings{
		AccountSchedulingThresholds: req.AccountSchedulingThresholds,

		TurnstileEnabled:             req.TurnstileEnabled,
		TurnstileSiteKey:             req.TurnstileSiteKey,
		TurnstileSecretKey:           req.TurnstileSecretKey,
		TencentCaptchaEnabled:        req.TencentCaptchaEnabled,
		TencentCaptchaAppID:          req.TencentCaptchaAppID,
		TencentCaptchaAppSecretKey:   req.TencentCaptchaAppSecretKey,
		TencentCaptchaCloudSecretID:  req.TencentCaptchaCloudSecretID,
		TencentCaptchaCloudSecretKey: req.TencentCaptchaCloudSecretKey,
		TencentCaptchaRegion:         req.TencentCaptchaRegion,
		AliyunCaptchaEnabled:         req.AliyunCaptchaEnabled,
		AliyunCaptchaAccessKeyID:     req.AliyunCaptchaAccessKeyID,
		AliyunCaptchaAccessKeySecret: req.AliyunCaptchaAccessKeySecret,
		AliyunCaptchaSceneID:         req.AliyunCaptchaSceneID,
		AliyunCaptchaPrefix:          req.AliyunCaptchaPrefix,
		AliyunCaptchaRegion:          req.AliyunCaptchaRegion,
		AffiliateRebateRate:          affiliateRebateRate,
		AffiliateRebateFreezeHours:   affiliateRebateFreezeHours,
		AffiliateRebateDurationDays:  affiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap: affiliateRebatePerInviteeCap,
		AdminRechargeRebateEnabled:   adminRechargeRebateEnabled,
		EnableIdentityPatch:          req.EnableIdentityPatch,
		IdentityPatchPrompt:          req.IdentityPatchPrompt,
		MinClaudeCodeVersion:         req.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:         req.MaxClaudeCodeVersion,
		ProfitControlEnabled: func() bool {
			if req.ProfitControlEnabled != nil {
				return *req.ProfitControlEnabled
			}
			return previousSettings.ProfitControlEnabled
		}(),
		ProfitMinMargin: func() float64 {
			if req.ProfitMinMargin != nil {
				return *req.ProfitMinMargin
			}
			return previousSettings.ProfitMinMargin
		}(),
		ProfitSafetyBuffer: func() float64 {
			if req.ProfitSafetyBuffer != nil {
				return *req.ProfitSafetyBuffer
			}
			return previousSettings.ProfitSafetyBuffer
		}(),
		OpsMonitoringEnabled: func() bool {
			if req.OpsMonitoringEnabled != nil {
				return *req.OpsMonitoringEnabled
			}
			return previousSettings.OpsMonitoringEnabled
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
		EnableFingerprintUnification: func() bool {
			if req.EnableFingerprintUnification != nil {
				return *req.EnableFingerprintUnification
			}
			return previousSettings.EnableFingerprintUnification
		}(),
		OpenAITTFTMode: func() string {
			if req.OpenAITTFTMode != nil {
				return *req.OpenAITTFTMode
			}
			return previousSettings.OpenAITTFTMode
		}(),
		EnableMetadataPassthrough: func() bool {
			if req.EnableMetadataPassthrough != nil {
				return *req.EnableMetadataPassthrough
			}
			return previousSettings.EnableMetadataPassthrough
		}(),
		EnableCCHSigning: func() bool {
			if req.EnableCCHSigning != nil {
				return *req.EnableCCHSigning
			}
			return previousSettings.EnableCCHSigning
		}(),
		EnableClaudeOAuthSystemPromptInjection: func() bool {
			if req.EnableClaudeOAuthSystemPromptInjection != nil {
				return *req.EnableClaudeOAuthSystemPromptInjection
			}
			return previousSettings.EnableClaudeOAuthSystemPromptInjection
		}(),
		ClaudeOAuthSystemPrompt: func() string {
			if req.ClaudeOAuthSystemPrompt != nil {
				return *req.ClaudeOAuthSystemPrompt
			}
			return previousSettings.ClaudeOAuthSystemPrompt
		}(),
		ClaudeOAuthSystemPromptBlocks: func() string {
			if req.ClaudeOAuthSystemPromptBlocks != nil {
				return *req.ClaudeOAuthSystemPromptBlocks
			}
			return previousSettings.ClaudeOAuthSystemPromptBlocks
		}(),
		EnableAnthropicCacheTTL1hInjection: func() bool {
			if req.EnableAnthropicCacheTTL1hInjection != nil {
				return *req.EnableAnthropicCacheTTL1hInjection
			}
			return previousSettings.EnableAnthropicCacheTTL1hInjection
		}(),
		RewriteMessageCacheControl: func() bool {
			if req.RewriteMessageCacheControl != nil {
				return *req.RewriteMessageCacheControl
			}
			return previousSettings.RewriteMessageCacheControl
		}(),
		EnableClientDatelineNormalization: func() bool {
			if req.EnableClientDatelineNormalization != nil {
				return *req.EnableClientDatelineNormalization
			}
			return previousSettings.EnableClientDatelineNormalization
		}(),
		AntigravityUserAgentVersion: func() string {
			if req.AntigravityUserAgentVersion != nil {
				return *req.AntigravityUserAgentVersion
			}
			return previousSettings.AntigravityUserAgentVersion
		}(),
		OpenAICodexUserAgent: func() string {
			if req.OpenAICodexUserAgent != nil {
				return *req.OpenAICodexUserAgent
			}
			return previousSettings.OpenAICodexUserAgent
		}(),
		OpenAICodexClientVersion: func() string {
			if req.OpenAICodexClientVersion != nil {
				return *req.OpenAICodexClientVersion
			}
			return previousSettings.OpenAICodexClientVersion
		}(),
		// 同步值由自动同步任务独占写入，面板保存时原样带回，避免被清空。
		OpenAICodexClientVersionSynced: previousSettings.OpenAICodexClientVersionSynced,
		OpenAICodexVersionAutoSyncEnabled: func() bool {
			if req.OpenAICodexVersionAutoSyncEnabled != nil {
				return *req.OpenAICodexVersionAutoSyncEnabled
			}
			return previousSettings.OpenAICodexVersionAutoSyncEnabled
		}(),
		MinCodexVersion:       strings.TrimSpace(req.MinCodexVersion),
		MaxCodexVersion:       strings.TrimSpace(req.MaxCodexVersion),
		CodexCLIOnlyBlacklist: strings.TrimSpace(req.CodexCLIOnlyBlacklist),
		CodexCLIOnlyWhitelist: strings.TrimSpace(req.CodexCLIOnlyWhitelist),
		CodexCLIOnlyAllowAppServerClients: func() bool {
			if req.CodexCLIOnlyAllowAppServerClients != nil {
				return *req.CodexCLIOnlyAllowAppServerClients
			}
			return previousSettings.CodexCLIOnlyAllowAppServerClients
		}(),
		CodexCLIOnlyEngineFingerprintSignals: strings.TrimSpace(req.CodexCLIOnlyEngineFingerprintSignals),
		PaymentVisibleMethodAlipaySource: func() string {
			if req.PaymentVisibleMethodAlipaySource != nil {
				return strings.TrimSpace(*req.PaymentVisibleMethodAlipaySource)
			}
			return previousSettings.PaymentVisibleMethodAlipaySource
		}(),
		PaymentVisibleMethodWxpaySource: func() string {
			if req.PaymentVisibleMethodWxpaySource != nil {
				return strings.TrimSpace(*req.PaymentVisibleMethodWxpaySource)
			}
			return previousSettings.PaymentVisibleMethodWxpaySource
		}(),
		PaymentVisibleMethodAlipayEnabled: func() bool {
			if req.PaymentVisibleMethodAlipayEnabled != nil {
				return *req.PaymentVisibleMethodAlipayEnabled
			}
			return previousSettings.PaymentVisibleMethodAlipayEnabled
		}(),
		PaymentVisibleMethodWxpayEnabled: func() bool {
			if req.PaymentVisibleMethodWxpayEnabled != nil {
				return *req.PaymentVisibleMethodWxpayEnabled
			}
			return previousSettings.PaymentVisibleMethodWxpayEnabled
		}(),
		ChannelMonitorEnabled: func() bool {
			if req.ChannelMonitorEnabled != nil {
				return *req.ChannelMonitorEnabled
			}
			return previousSettings.ChannelMonitorEnabled
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
		ChannelMonitorHideThroughput: func() bool {
			if req.ChannelMonitorHideThroughput != nil {
				return *req.ChannelMonitorHideThroughput
			}
			return previousSettings.ChannelMonitorHideThroughput
		}(),
		ChannelMonitorShowQuota: func() bool {
			if req.ChannelMonitorShowQuota != nil {
				return *req.ChannelMonitorShowQuota
			}
			return previousSettings.ChannelMonitorShowQuota
		}(),
		ChannelMonitorHideUserRanking: func() bool {
			if req.ChannelMonitorHideUserRanking != nil {
				return *req.ChannelMonitorHideUserRanking
			}
			return previousSettings.ChannelMonitorHideUserRanking
		}(),
		GrokDefaultTextModel: func() string {
			if req.GrokDefaultTextModel != nil {
				return *req.GrokDefaultTextModel
			}
			return previousSettings.GrokDefaultTextModel
		}(),
		GrokCrossClientModelMapEnabled: func() bool {
			if req.GrokCrossClientModelMapEnabled != nil {
				return *req.GrokCrossClientModelMapEnabled
			}
			return previousSettings.GrokCrossClientModelMapEnabled
		}(),
		GrokDefaultBaseURLMode: func() string {
			if req.GrokDefaultBaseURLMode != nil {
				return strings.TrimSpace(*req.GrokDefaultBaseURLMode)
			}
			return previousSettings.GrokDefaultBaseURLMode
		}(),
		PluginManagementEnabled: func() bool {
			if req.PluginManagementEnabled != nil {
				return *req.PluginManagementEnabled
			}
			return previousSettings.PluginManagementEnabled
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

	if err := h.settingService.UpdateSettingsOmitting(c.Request.Context(), settings, omitted); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.opsService != nil {
		h.opsService.SetMonitoringEnabled(settings.OpsMonitoringEnabled)
	}

	// Update OpenAI fast policy (stored under dedicated key, only when provided).
	if req.OpenAIFastPolicySettings != nil {
		if err := h.settingService.SetOpenAIFastPolicySettings(c.Request.Context(), openaiFastPolicySettingsFromDTO(req.OpenAIFastPolicySettings)); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}

	// Update payment configuration (integrated into system settings).
	// Skip if no payment fields were provided (prevents accidental wipe).
	if h.paymentConfigService != nil && hasPaymentFields(req) {
		paymentReq := service.UpdatePaymentConfigRequest{
			Enabled:                       req.PaymentEnabled,
			MinAmount:                     req.PaymentMinAmount,
			MaxAmount:                     req.PaymentMaxAmount,
			DailyLimit:                    req.PaymentDailyLimit,
			OrderTimeoutMin:               req.PaymentOrderTimeoutMin,
			MaxPendingOrders:              req.PaymentMaxPendingOrders,
			EnabledTypes:                  req.PaymentEnabledTypes,
			USDToCNYRate:                  req.PaymentUSDToCNYRate,
			RechargeFeeRate:               req.PaymentRechargeFeeRate,
			LoadBalanceStrategy:           req.PaymentLoadBalanceStrat,
			ProductNamePrefix:             req.PaymentProductNamePrefix,
			ProductNameSuffix:             req.PaymentProductNameSuffix,
			HelpImageURL:                  req.PaymentHelpImageURL,
			HelpText:                      req.PaymentHelpText,
			CancelRateLimitEnabled:        req.PaymentCancelRateLimitEnabled,
			CancelRateLimitMax:            req.PaymentCancelRateLimitMax,
			CancelRateLimitWindow:         req.PaymentCancelRateLimitWindow,
			CancelRateLimitUnit:           req.PaymentCancelRateLimitUnit,
			CancelRateLimitMode:           req.PaymentCancelRateLimitMode,
			AlipayForceQRCode:             req.PaymentAlipayForceQRCode,
			AlipayMobilePrecreateDeepLink: req.PaymentAlipayMobilePrecreateDeepLink,
		}
		if err := h.paymentConfigService.UpdatePaymentConfig(c.Request.Context(), paymentReq); err != nil {
			response.ErrorFrom(c, err)
			return
		}
		// Refresh in-memory provider registry so config changes take effect immediately
		if h.paymentService != nil {
			h.paymentService.RefreshProviders(c.Request.Context())
		}
	}

	h.auditSettingsUpdate(c, previousSettings, settings, auditReq)

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Reload payment config for response
	var updatedPaymentCfg *service.PaymentConfig
	if h.paymentConfigService != nil {
		updatedPaymentCfg, _ = h.paymentConfigService.GetPaymentConfig(c.Request.Context())
	}
	if updatedPaymentCfg == nil {
		updatedPaymentCfg = &service.PaymentConfig{}
	}

	payload := dto.SystemSettings{
		TurnstileEnabled:                       updatedSettings.TurnstileEnabled,
		TurnstileSiteKey:                       updatedSettings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:           updatedSettings.TurnstileSecretKeyConfigured,
		TencentCaptchaEnabled:                  updatedSettings.TencentCaptchaEnabled,
		TencentCaptchaAppID:                    updatedSettings.TencentCaptchaAppID,
		TencentCaptchaAppSecretKeyConfigured:   updatedSettings.TencentCaptchaAppSecretKeyConfigured,
		TencentCaptchaCloudSecretIDConfigured:  updatedSettings.TencentCaptchaCloudSecretIDConfigured,
		TencentCaptchaCloudSecretKeyConfigured: updatedSettings.TencentCaptchaCloudSecretKeyConfigured,
		TencentCaptchaRegion:                   updatedSettings.TencentCaptchaRegion,
		AliyunCaptchaEnabled:                   updatedSettings.AliyunCaptchaEnabled,
		AliyunCaptchaAccessKeyID:               updatedSettings.AliyunCaptchaAccessKeyID,
		AliyunCaptchaAccessKeySecretConfigured: updatedSettings.AliyunCaptchaAccessKeySecretConfigured,
		AliyunCaptchaSceneID:                   updatedSettings.AliyunCaptchaSceneID,
		AliyunCaptchaPrefix:                    updatedSettings.AliyunCaptchaPrefix,
		AliyunCaptchaRegion:                    updatedSettings.AliyunCaptchaRegion,
		AffiliateRebateRate:                    updatedSettings.AffiliateRebateRate,
		AffiliateRebateFreezeHours:             updatedSettings.AffiliateRebateFreezeHours,
		AffiliateRebateDurationDays:            updatedSettings.AffiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap:           updatedSettings.AffiliateRebatePerInviteeCap,
		AdminRechargeRebateEnabled:             updatedSettings.AdminRechargeRebateEnabled,
		EnableIdentityPatch:                    updatedSettings.EnableIdentityPatch,
		IdentityPatchPrompt:                    updatedSettings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                   updatedSettings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:           updatedSettings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                    updatedSettings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:              updatedSettings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                   updatedSettings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                   updatedSettings.MaxClaudeCodeVersion,
		EnableFingerprintUnification:           updatedSettings.EnableFingerprintUnification,
		EnableMetadataPassthrough:              updatedSettings.EnableMetadataPassthrough,
		EnableCCHSigning:                       updatedSettings.EnableCCHSigning,
		EnableClaudeOAuthSystemPromptInjection: updatedSettings.EnableClaudeOAuthSystemPromptInjection,
		ClaudeOAuthSystemPrompt:                updatedSettings.ClaudeOAuthSystemPrompt,
		ClaudeOAuthSystemPromptBlocks:          updatedSettings.ClaudeOAuthSystemPromptBlocks,
		EnableAnthropicCacheTTL1hInjection:     updatedSettings.EnableAnthropicCacheTTL1hInjection,
		RewriteMessageCacheControl:             updatedSettings.RewriteMessageCacheControl,
		EnableClientDatelineNormalization:      updatedSettings.EnableClientDatelineNormalization,
		AntigravityUserAgentVersion:            updatedSettings.AntigravityUserAgentVersion,
		OpenAICodexUserAgent:                   updatedSettings.OpenAICodexUserAgent,
		OpenAICodexClientVersion:               updatedSettings.OpenAICodexClientVersion,
		OpenAICodexClientVersionSynced:         updatedSettings.OpenAICodexClientVersionSynced,
		OpenAICodexVersionAutoSyncEnabled:      updatedSettings.OpenAICodexVersionAutoSyncEnabled,
		MinCodexVersion:                        updatedSettings.MinCodexVersion,
		MaxCodexVersion:                        updatedSettings.MaxCodexVersion,
		CodexCLIOnlyBlacklist:                  updatedSettings.CodexCLIOnlyBlacklist,
		CodexCLIOnlyWhitelist:                  updatedSettings.CodexCLIOnlyWhitelist,
		CodexCLIOnlyAllowAppServerClients:      updatedSettings.CodexCLIOnlyAllowAppServerClients,
		CodexCLIOnlyEngineFingerprintSignals:   updatedSettings.CodexCLIOnlyEngineFingerprintSignals,
		PaymentVisibleMethodAlipaySource:       updatedSettings.PaymentVisibleMethodAlipaySource,
		PaymentVisibleMethodWxpaySource:        updatedSettings.PaymentVisibleMethodWxpaySource,
		PaymentVisibleMethodAlipayEnabled:      updatedSettings.PaymentVisibleMethodAlipayEnabled,
		PaymentVisibleMethodWxpayEnabled:       updatedSettings.PaymentVisibleMethodWxpayEnabled,
		PaymentEnabled:                         updatedPaymentCfg.Enabled,
		PaymentMinAmount:                       updatedPaymentCfg.MinAmount,
		PaymentMaxAmount:                       updatedPaymentCfg.MaxAmount,
		PaymentDailyLimit:                      updatedPaymentCfg.DailyLimit,
		PaymentOrderTimeoutMin:                 updatedPaymentCfg.OrderTimeoutMin,
		PaymentMaxPendingOrders:                updatedPaymentCfg.MaxPendingOrders,
		PaymentEnabledTypes:                    updatedPaymentCfg.EnabledTypes,
		PaymentUSDToCNYRate:                    updatedPaymentCfg.USDToCNYRate,
		PaymentRechargeFeeRate:                 updatedPaymentCfg.RechargeFeeRate,
		PaymentLoadBalanceStrat:                updatedPaymentCfg.LoadBalanceStrategy,
		PaymentProductNamePrefix:               updatedPaymentCfg.ProductNamePrefix,
		PaymentProductNameSuffix:               updatedPaymentCfg.ProductNameSuffix,
		PaymentHelpImageURL:                    updatedPaymentCfg.HelpImageURL,
		PaymentHelpText:                        updatedPaymentCfg.HelpText,
		PaymentCancelRateLimitEnabled:          updatedPaymentCfg.CancelRateLimitEnabled,
		PaymentCancelRateLimitMax:              updatedPaymentCfg.CancelRateLimitMax,
		PaymentCancelRateLimitWindow:           updatedPaymentCfg.CancelRateLimitWindow,
		PaymentCancelRateLimitUnit:             updatedPaymentCfg.CancelRateLimitUnit,
		PaymentCancelRateLimitMode:             updatedPaymentCfg.CancelRateLimitMode,
		PaymentAlipayForceQRCode:               updatedPaymentCfg.AlipayForceQRCode,
		PaymentAlipayMobilePrecreateDeepLink:   updatedPaymentCfg.AlipayMobilePrecreateDeepLink,

		ChannelMonitorEnabled:                updatedSettings.ChannelMonitorEnabled,
		ChannelMonitorMode:                   updatedSettings.ChannelMonitorMode,
		ChannelMonitorDefaultIntervalSeconds: updatedSettings.ChannelMonitorDefaultIntervalSeconds,
		ChannelMonitorHideThroughput:         updatedSettings.ChannelMonitorHideThroughput,
		ChannelMonitorShowQuota:              updatedSettings.ChannelMonitorShowQuota,
		ChannelMonitorHideUserRanking:        updatedSettings.ChannelMonitorHideUserRanking,

		GrokDefaultTextModel:           updatedSettings.GrokDefaultTextModel,
		GrokCrossClientModelMapEnabled: updatedSettings.GrokCrossClientModelMapEnabled,
		GrokDefaultBaseURLMode:         updatedSettings.GrokDefaultBaseURLMode,

		PluginManagementEnabled: updatedSettings.PluginManagementEnabled,

		AffiliateEnabled: updatedSettings.AffiliateEnabled,

		RiskControlEnabled:          updatedSettings.RiskControlEnabled,
		CyberSessionBlockEnabled:    updatedSettings.CyberSessionBlockEnabled,
		CyberSessionBlockTTLSeconds: updatedSettings.CyberSessionBlockTTLSeconds,
		AccountSchedulingThresholds: updatedSettings.AccountSchedulingThresholds,
		ProfitControlEnabled:        updatedSettings.ProfitControlEnabled,
		ProfitMinMargin:             updatedSettings.ProfitMinMargin,
		ProfitSafetyBuffer:          updatedSettings.ProfitSafetyBuffer,
	}
	if fastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context()); err != nil {
		slog.Error("openai_fast_policy_settings_get_failed", "error", err)
	} else if fastPolicy != nil {
		payload.OpenAIFastPolicySettings = openaiFastPolicySettingsToDTO(fastPolicy)
	}
	response.Success(c, payload)
}

func hasPaymentFields(req UpdateSettingsRequest) bool {
	return req.PaymentEnabled != nil || req.PaymentMinAmount != nil ||
		req.PaymentMaxAmount != nil || req.PaymentDailyLimit != nil ||
		req.PaymentOrderTimeoutMin != nil || req.PaymentMaxPendingOrders != nil ||
		req.PaymentEnabledTypes != nil ||
		req.PaymentUSDToCNYRate != nil ||
		req.PaymentRechargeFeeRate != nil ||
		req.PaymentLoadBalanceStrat != nil || req.PaymentProductNamePrefix != nil ||
		req.PaymentProductNameSuffix != nil || req.PaymentHelpImageURL != nil ||
		req.PaymentHelpText != nil || req.PaymentCancelRateLimitEnabled != nil ||
		req.PaymentCancelRateLimitMax != nil || req.PaymentCancelRateLimitWindow != nil ||
		req.PaymentCancelRateLimitUnit != nil || req.PaymentCancelRateLimitMode != nil ||
		req.PaymentAlipayForceQRCode != nil || req.PaymentAlipayMobilePrecreateDeepLink != nil
}
