package service

import (
	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// Status constants
const (
	StatusActive   = domain.StatusActive
	StatusDisabled = domain.StatusDisabled
	StatusError    = domain.StatusError
	StatusUnused   = domain.StatusUnused
	StatusUsed     = domain.StatusUsed
	StatusExpired  = domain.StatusExpired
)

// Role constants
const (
	RoleAdmin = domain.RoleAdmin
	RoleUser  = domain.RoleUser
)

// Affiliate rebate settings
const (
	AffiliateRebateRateDefault          = 20.0
	AffiliateRebateRateMin              = 0.0
	AffiliateRebateRateMax              = 100.0
	AffiliateEnabledDefault             = false // 邀请返利总开关默认关闭
	AffiliateRebateFreezeHoursDefault   = 0     // 0 = 不冻结（向后兼容）
	AffiliateRebateFreezeHoursMax       = 720   // 最大 30 天
	AffiliateRebateDurationDaysDefault  = 0     // 0 = 永久有效
	AffiliateRebateDurationDaysMax      = 3650  // ~10 年
	AffiliateRebatePerInviteeCapDefault = 0.0   // 0 = 无上限
	AdminRechargeRebateEnabledDefault   = false // 管理员充值默认不产生返利
)

// Platform constants
const (
	PlatformAnthropic   = domain.PlatformAnthropic
	PlatformOpenAI      = domain.PlatformOpenAI
	PlatformGemini      = domain.PlatformGemini
	PlatformAntigravity = domain.PlatformAntigravity
	PlatformGrok        = domain.PlatformGrok
	// 国产 OpenAI 兼容供应商（与 grok 一样经 OpenAI 网关转发）。
	PlatformKimi       = domain.PlatformKimi
	PlatformZhipu      = domain.PlatformZhipu
	PlatformDeepseek   = domain.PlatformDeepseek
	PlatformMiniMax    = domain.PlatformMiniMax
	PlatformOpenCodeGo = domain.PlatformOpenCodeGo
	PlatformComposite  = domain.PlatformComposite
	// PlatformKiro is retained for unsupported-platform threshold tests and legacy
	// account rows. Scheduling-threshold evaluation never pauses kiro accounts.
	PlatformKiro = "kiro"
)

// 账号接入模式（国产供应商）：按量付费 vs Coding Plan。
const (
	AccountModePayG   = domain.AccountModePayG
	AccountModeCoding = domain.AccountModeCoding
	AccountModeZen    = domain.AccountModeZen
	AccountModeGo     = domain.AccountModeGo
)

// 上游 API 协议（国产供应商）：决定转发端点与格式，与接入模式正交。
const (
	APIProtocolChatCompletions = domain.APIProtocolChatCompletions
	APIProtocolAnthropic       = domain.APIProtocolAnthropic
	APIProtocolResponses       = domain.APIProtocolResponses
)

// 国产 OpenAI 兼容供应商各模式的默认 base_url。
// 与前端 credentialsBuilder.ts 中的预设保持一致。
const (
	DefaultKimiPayGBaseURL    = "https://api.moonshot.cn/v1"
	DefaultKimiCodingBaseURL  = "https://api.kimi.com/coding/v1"
	DefaultZhipuPayGBaseURL   = "https://open.bigmodel.cn/api/paas/v4"
	DefaultZhipuCodingBaseURL = "https://open.bigmodel.cn/api/coding/paas/v4"
	DefaultDeepseekBaseURL    = "https://api.deepseek.com"
	// MiniMax 按量付费与 Coding/Token Plan 共用推理域名，靠 API Key 区分套餐。
	DefaultMiniMaxBaseURL = "https://api.minimaxi.com/v1"
	// OpenCode Go：Chat Completions / Responses / models 共用 /v1 基址。
	DefaultOpenCodeGoBaseURL = "https://opencode.ai/zen/go/v1"
	// OpenCode Zen：按量付费网关，模型列表为 /zen/v1/models。
	DefaultOpenCodeZenBaseURL = "https://opencode.ai/zen/v1"
)

// 国产供应商 Anthropic 协议端点的默认 base_url（上游路径为 {base}/v1/messages）。
// 与前端 credentialsBuilder.ts 中的预设保持一致。
const (
	DefaultKimiPayGAnthropicBaseURL   = "https://api.moonshot.cn/anthropic"
	DefaultKimiCodingAnthropicBaseURL = "https://api.kimi.com/coding"
	DefaultZhipuAnthropicBaseURL      = "https://open.bigmodel.cn/api/anthropic"
	DefaultDeepseekAnthropicBaseURL   = "https://api.deepseek.com/anthropic"
	DefaultMiniMaxAnthropicBaseURL    = "https://api.minimaxi.com/anthropic"
	// OpenCode Go Anthropic 基址不含 /v1：nativeAnthropicTargetURL 会再拼 /v1/messages。
	DefaultOpenCodeGoAnthropicBaseURL  = "https://opencode.ai/zen/go"
	DefaultOpenCodeZenAnthropicBaseURL = "https://opencode.ai/zen"
)

// IsCNProvider 报告 platform 是否为国产 OpenAI 兼容供应商（kimi/zhipu/deepseek/minimax）。
func IsCNProvider(platform string) bool {
	switch platform {
	case PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax:
		return true
	default:
		return false
	}
}

// IsOpenCodeGo 报告 platform 是否为 OpenCode Go 订阅网关。
func IsOpenCodeGo(platform string) bool {
	return platform == PlatformOpenCodeGo
}

// IsMultiProtocolAPIKeyProvider 报告 platform 是否为多协议 API Key 网关
// （国产供应商 + OpenCode）：走 OpenAI 网关、支持 adaptive 协议分流。
func IsMultiProtocolAPIKeyProvider(platform string) bool {
	return IsCNProvider(platform) || platform == PlatformOpenCodeGo
}

// AllowedSchedulingThresholdPlatforms 是允许设置账号自动停调阈值的平台列表。
// openai/anthropic/grok 有原生用量窗口；kimi/zhipu/minimax 的 Coding Plan 同样暴露
// 5h/weekly 滚动窗口，纳入阈值评估。deepseek 为余额型，走余额检测而非阈值。
var AllowedSchedulingThresholdPlatforms = []string{
	PlatformOpenAI,
	PlatformAnthropic,
	PlatformGrok,
	PlatformKimi,
	PlatformZhipu,
	PlatformMiniMax,
	PlatformOpenCodeGo,
}

// Account type constants
const (
	AccountTypeOAuth          = domain.AccountTypeOAuth          // OAuth类型账号（full scope: profile + inference）
	AccountTypeSetupToken     = domain.AccountTypeSetupToken     // Setup Token类型账号（inference only scope）
	AccountTypeAPIKey         = domain.AccountTypeAPIKey         // API Key类型账号
	AccountTypeBedrock        = domain.AccountTypeBedrock        // AWS Bedrock 类型账号（通过 SigV4 签名或 API Key 连接 Bedrock，由 credentials.auth_mode 区分）
	AccountTypeServiceAccount = domain.AccountTypeServiceAccount // Google Service Account 类型账号（用于 Vertex AI）
)

// Redeem type constants
const (
	RedeemTypeBalance          = domain.RedeemTypeBalance
	RedeemTypeConcurrency      = domain.RedeemTypeConcurrency
	RedeemTypeSubscription     = domain.RedeemTypeSubscription
	RedeemTypeInvitation       = domain.RedeemTypeInvitation
	RedeemTypeAffiliateBalance = "affiliate_balance"
)

// Admin adjustment type constants
const (
	AdjustmentTypeAdminBalance     = domain.AdjustmentTypeAdminBalance     // 管理员调整余额
	AdjustmentTypeAdminConcurrency = domain.AdjustmentTypeAdminConcurrency // 管理员调整并发数
)

// Group subscription type constants
const ()

// Subscription status constants
const (
	SubscriptionStatusActive    = domain.SubscriptionStatusActive
	SubscriptionStatusExpired   = domain.SubscriptionStatusExpired
	SubscriptionStatusSuspended = domain.SubscriptionStatusSuspended
	// SubscriptionStatusRevoked 是 soft-deleted 订阅的 API 展示态，不写入 status 字段。
	SubscriptionStatusRevoked = "revoked"
)

// WeChatConnectSyntheticEmailDomain 是 WeChat Connect 用户的合成邮箱后缀（RFC 保留域名）。
const WeChatConnectSyntheticEmailDomain = "@wechat-connect.invalid"

// Setting keys
const (
	// 白名单非空时，是否放行非白名单域名按主域名限量注册（每域名 1 个账户）。
	// 默认 false：非白名单域名直接拒绝（白名单严格模式）。
	SettingKeyAffiliateEnabled              = "affiliate_enabled"                // 邀请返利功能总开关
	SettingKeyAffiliateRebateRate           = "affiliate_rebate_rate"            // 邀请返利比例（百分比，0-100）
	SettingKeyAffiliateRebateFreezeHours    = "affiliate_rebate_freeze_hours"    // 返利冻结期（小时，0=不冻结）
	SettingKeyAffiliateRebateDurationDays   = "affiliate_rebate_duration_days"   // 返利有效期（天，0=永久）
	SettingKeyAffiliateRebatePerInviteeCap  = "affiliate_rebate_per_invitee_cap" // 单人返利上限（0=无上限）
	SettingKeyAffiliateAdminRechargeEnabled = "affiliate_admin_recharge_enabled" // 管理员充值是否产生返利
	SettingKeyRiskControlEnabled            = "risk_control_enabled"             // 是否启用风控中心入口与审计链路
	SettingKeyContentModerationConfig       = "content_moderation_config"        // 内容审计配置（JSON）

	// Cloudflare Turnstile 设置

	// 腾讯天御验证码设置

	// 阿里云验证码 2.0 设置（与 Turnstile、腾讯天御互斥，同一时间仅可启用一家）

	// 默认配置

	// 第三方认证来源默认授予配置

	// 管理员 API Key
	SettingKeyAdminAPIKey = "admin_api_key" // 全局管理员 API Key（用于外部系统集成）

	// Gemini 配额策略（JSON）
	SettingKeyGeminiQuotaPolicy = "gemini_quota_policy"

	// =========================
	// Ops Monitoring (vNext)
	// =========================

	// SettingKeyOpsRealtimeMonitoringEnabled controls realtime features (e.g. WS/QPS push).
	SettingKeyOpsRealtimeMonitoringEnabled = "ops_realtime_monitoring_enabled"

	// SettingKeyOpsQueryModeDefault controls the default query mode for ops dashboard (auto/raw/preagg).
	SettingKeyOpsQueryModeDefault = "ops_query_mode_default"

	// SettingKeyOpsEmailNotificationConfig stores JSON config for ops email notifications.
	SettingKeyOpsEmailNotificationConfig = "ops_email_notification_config"

	// SettingKeyOpsAlertRuntimeSettings stores JSON config for ops alert evaluator runtime settings.
	SettingKeyOpsAlertRuntimeSettings = "ops_alert_runtime_settings"

	// SettingKeyOpsMetricsIntervalSeconds controls the ops metrics collector interval (>=60).
	SettingKeyOpsMetricsIntervalSeconds = "ops_metrics_interval_seconds"

	// SettingKeyOpsAdvancedSettings stores JSON config for ops advanced settings (data retention, aggregation).
	SettingKeyOpsAdvancedSettings = "ops_advanced_settings"

	// SettingKeyOpsRuntimeLogConfig stores JSON config for runtime log settings.
	SettingKeyOpsRuntimeLogConfig = "ops_runtime_log_config"

	// =========================
	// Channel Monitor (渠道监控)
	// =========================

	// SettingKeyChannelMonitorMode used to select "v1" active probes or "v2" passive aggregation.
	// V1 was retired from the admin console (2026-09-24): the mode is fixed to "v2" and the stored
	// value is no longer read; V1 code and tables remain but are unreachable at runtime.
	SettingKeyChannelMonitorMode = "channel_monitor_mode"

	// ChannelMonitorModeV1/V2 are the only accepted mode values.
	ChannelMonitorModeV1 = "v1"
	ChannelMonitorModeV2 = "v2"

	// SettingKeyChannelMonitorDefaultIntervalSeconds controls the default interval (seconds)
	// pre-filled when creating a new channel monitor from the admin UI. Range: [15, 3600].
	SettingKeyChannelMonitorDefaultIntervalSeconds = "channel_monitor_default_interval_seconds"

	// SettingKeyChannelMonitorShowQuota controls whether quota/balance snapshots
	// attached to channel monitors (check_mode=quota/quota_probe) are exposed on
	// the user-facing monitor APIs and UI. Default false (hidden); parsed
	// fail-closed (only the literal "true" enables it). Admin endpoints always
	// keep the full snapshots regardless of this flag.
	SettingKeyChannelMonitorShowQuota = "channel_monitor_show_quota"

	// SettingKeyModelPlazaDescription stores the Markdown blurb rendered at the top of
	// the Model Plaza page (global pricing notes, exchange rate, promotions, ...).

	// SettingKeyOpenAIAPIKeyHealthBreakerSettings stores the opt-in OpenAI pool API-key breaker config.
	SettingKeyOpenAIAPIKeyHealthBreakerSettings = "openai_apikey_health_breaker_settings"

	// OpenAI Responses first_token_ms 的统计口径（取值见 gateway_features.go 的 OpenAITTFTMode）。
	OpenAITTFTModeSemantic = "semantic"
	OpenAITTFTModeVisible  = "visible"

	// SettingKeyOpenAICodexClientVersionSynced 自动同步任务写入的官方 Codex 最新稳定版版本号。
	// 由 OpenAICodexVersionSyncService 独占写入；出站身份按它拼（没有时用内置常量）。
	SettingKeyOpenAICodexClientVersionSynced = "openai_codex_client_version_synced"
)

// 利润门（全站一档）：上游成本比（上游价 ÷ 官方价）> 用户倍率 × (1 − min_margin) 的渠道不派；min_margin 填 0 = 不能亏本（门一直开着）。
const (
	SettingKeyProfitMinMargin = "profit_min_margin" // 最低毛利率，小数（0.30 = 30%）
	ProfitControlRatioMax     = 0.99                // min_margin 上限：到 1 阈值就 ≤ 0，全池不可派
)

// QuotaDimension constants for spark shadow accounts.
const (
	QuotaDimensionGlobal = "global"
	QuotaDimensionSpark  = "spark"
)

// AdminAPIKeyPrefix is the prefix for admin API keys (distinct from user "sk-" keys).
const AdminAPIKeyPrefix = "admin-"
