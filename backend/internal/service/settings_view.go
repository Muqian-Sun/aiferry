package service

import "strings"

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type SystemSettings struct {
	RiskControlEnabled           bool
	CyberSessionBlockEnabled     bool
	CyberSessionBlockTTLSeconds  int
	AffiliateEnabled             bool
	AffiliateRebateRate          float64
	AffiliateRebateFreezeHours   int
	AffiliateRebateDurationDays  int
	AffiliateRebatePerInviteeCap float64
	AdminRechargeRebateEnabled   bool

	// Ops monitoring (vNext)
	OpsRealtimeMonitoringEnabled bool
	OpsQueryModeDefault          string
	OpsMetricsIntervalSeconds    int

	// Channel Monitor feature
	ChannelMonitorMode                   string `json:"channel_monitor_mode"`
	ChannelMonitorDefaultIntervalSeconds int    `json:"channel_monitor_default_interval_seconds"`
	ChannelMonitorHideThroughput         bool   `json:"channel_monitor_hide_throughput"`
	ChannelMonitorShowQuota              bool   `json:"channel_monitor_show_quota"`
	ChannelMonitorHideUserRanking        bool   `json:"channel_monitor_hide_user_ranking"`

	// Model Plaza feature (public group/model pricing showcase)
	PluginManagementEnabled bool `json:"plugin_management_enabled"`

	// Web Search Emulation
	WebSearchEmulationEnabled bool // 是否启用 web search 模拟

	// 利润门（全站一档）
	ProfitControlEnabled bool
	ProfitMinMargin      float64
	ProfitSafetyBuffer   float64
}

// DefaultSubscriptionSetting 新用户自动赠送的一条订阅（见 site_features.go 的 newUserDefaultSubscriptions）。
type DefaultSubscriptionSetting struct {
	PlanID       int64 `json:"plan_id"`
	ValidityDays int   `json:"validity_days"`
}

type PublicSettings struct {
	RegistrationEnabled                 bool
	EmailVerifyEnabled                  bool
	ForceEmailOnThirdPartySignup        bool
	RegistrationEmailSuffixWhitelist    []string
	RegistrationEmailDomainQuotaEnabled bool
	PasswordResetEnabled                bool
	InvitationCodeEnabled               bool
	PasskeyEnabled                      bool
	LoginAgreementEnabled               bool
	LoginAgreementMode                  string
	LoginAgreementUpdatedAt             string
	LoginAgreementRevision              string
	LoginAgreementDocuments             []LoginAgreementDocument
	TurnstileEnabled                    bool
	TurnstileSiteKey                    string
	TencentCaptchaEnabled               bool
	TencentCaptchaAppID                 string
	TencentCaptchaRegion                string
	AliyunCaptchaEnabled                bool
	AliyunCaptchaSceneID                string
	AliyunCaptchaPrefix                 string
	AliyunCaptchaRegion                 string
	SiteName                            string
	SiteLogo                            string
	SiteSubtitle                        string
	APIBaseURL                          string
	ContactInfo                         string
	DocURL                              string
	HomeContent                         string
	CompactHomeEnabled                  bool
	HideCcsImportButton                 bool

	TableDefaultPageSize int
	TablePageSizeOptions []int
	CustomMenuItems      string // JSON array of custom menu items
	CustomEndpoints      string // JSON array of custom endpoints

	WeChatOAuthEnabled       bool
	WeChatOAuthOpenEnabled   bool
	WeChatOAuthMPEnabled     bool
	WeChatOAuthMobileEnabled bool
	BackendModeEnabled       bool
	PaymentEnabled           bool
	GitHubOAuthEnabled       bool
	GoogleOAuthEnabled       bool
	Version                  string

	BalanceLowNotifyEnabled     bool
	AccountQuotaNotifyEnabled   bool
	BalanceLowNotifyThreshold   float64
	BalanceLowNotifyRechargeURL string

	// Channel Monitor feature
	ChannelMonitorEnabled                bool   `json:"channel_monitor_enabled"`
	ChannelMonitorMode                   string `json:"channel_monitor_mode"`
	ChannelMonitorDefaultIntervalSeconds int    `json:"channel_monitor_default_interval_seconds"`
	ChannelMonitorHideThroughput         bool   `json:"channel_monitor_hide_throughput"`
	ChannelMonitorShowQuota              bool   `json:"channel_monitor_show_quota"`
	ChannelMonitorHideUserRanking        bool   `json:"channel_monitor_hide_user_ranking"`

	// Grok model mapping policy (admin settings).
	GrokDefaultTextModel           string `json:"grok_default_text_model"`
	GrokCrossClientModelMapEnabled bool   `json:"grok_cross_client_model_map_enabled"`
	GrokDefaultBaseURLMode         string `json:"grok_default_base_url_mode"`

	// Model Plaza feature (public group/model pricing showcase)
	PluginManagementEnabled bool `json:"plugin_management_enabled"`

	// Affiliate (邀请返利) feature toggle
	AffiliateEnabled bool `json:"affiliate_enabled"`

	// 风控中心功能开关
	RiskControlEnabled bool `json:"risk_control_enabled"`

	// 允许终端用户在用量页查看自己的失败请求
	AllowUserViewErrorRequests bool `json:"allow_user_view_error_requests"`
}

type LoginAgreementDocument struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ContentMD string `json:"content_md"`
}

type WeChatConnectOAuthConfig struct {
	Enabled             bool
	LegacyAppID         string
	LegacyAppSecret     string
	OpenAppID           string
	OpenAppSecret       string
	MPAppID             string
	MPAppSecret         string
	MobileAppID         string
	MobileAppSecret     string
	OpenEnabled         bool
	MPEnabled           bool
	MobileEnabled       bool
	Mode                string
	Scopes              string
	RedirectURL         string
	FrontendRedirectURL string
}

func (cfg WeChatConnectOAuthConfig) SupportsMode(mode string) bool {
	switch normalizeWeChatConnectModeSetting(mode) {
	case "mp":
		return cfg.MPEnabled
	case "mobile":
		return cfg.MobileEnabled
	default:
		return cfg.OpenEnabled
	}
}

func (cfg WeChatConnectOAuthConfig) ScopeForMode(mode string) string {
	switch normalizeWeChatConnectModeSetting(mode) {
	case "mp":
		return normalizeWeChatConnectScopeSetting(cfg.Scopes, "mp")
	case "mobile":
		return ""
	}
	return defaultWeChatConnectScopeForMode("open")
}

func (cfg WeChatConnectOAuthConfig) AppIDForMode(mode string) string {
	switch normalizeWeChatConnectModeSetting(mode) {
	case "mp":
		return strings.TrimSpace(firstNonEmpty(cfg.MPAppID, cfg.LegacyAppID))
	case "mobile":
		return strings.TrimSpace(firstNonEmpty(cfg.MobileAppID, cfg.LegacyAppID))
	}
	return strings.TrimSpace(firstNonEmpty(cfg.OpenAppID, cfg.LegacyAppID))
}

func (cfg WeChatConnectOAuthConfig) AppSecretForMode(mode string) string {
	switch normalizeWeChatConnectModeSetting(mode) {
	case "mp":
		return strings.TrimSpace(firstNonEmpty(cfg.MPAppSecret, cfg.LegacyAppSecret))
	case "mobile":
		return strings.TrimSpace(firstNonEmpty(cfg.MobileAppSecret, cfg.LegacyAppSecret))
	}
	return strings.TrimSpace(firstNonEmpty(cfg.OpenAppSecret, cfg.LegacyAppSecret))
}

// StreamTimeoutSettings 流超时处理配置（仅控制超时后的处理方式，超时判定由网关配置控制）
type StreamTimeoutSettings struct {
	// Enabled 是否启用流超时处理
	Enabled bool `json:"enabled"`
	// Action 超时后的处理方式: "temp_unsched" | "error" | "none"
	Action string `json:"action"`
	// TempUnschedMinutes 临时不可调度持续时间（分钟）
	TempUnschedMinutes int `json:"temp_unsched_minutes"`
	// ThresholdCount 触发阈值次数（累计多少次超时才触发）
	ThresholdCount int `json:"threshold_count"`
	// ThresholdWindowMinutes 阈值窗口时间（分钟）
	ThresholdWindowMinutes int `json:"threshold_window_minutes"`
}

// StreamTimeoutAction 流超时处理方式常量
const (
	StreamTimeoutActionTempUnsched = "temp_unsched" // 临时不可调度
	StreamTimeoutActionError       = "error"        // 标记为错误状态
	StreamTimeoutActionNone        = "none"         // 不处理
)

// RectifierSettings 请求整流器配置
type RectifierSettings struct {
	Enabled                  bool     `json:"enabled"`                    // 总开关
	ThinkingSignatureEnabled bool     `json:"thinking_signature_enabled"` // Thinking 签名整流
	ThinkingBudgetEnabled    bool     `json:"thinking_budget_enabled"`    // Thinking Budget 整流
	APIKeySignatureEnabled   bool     `json:"apikey_signature_enabled"`   // API Key 签名整流开关
	APIKeySignaturePatterns  []string `json:"apikey_signature_patterns"`  // API Key 自定义匹配关键词
}

// Beta Policy 策略常量
const (
	BetaPolicyActionPass   = "pass"   // 透传，不做任何处理
	BetaPolicyActionFilter = "filter" // 过滤，从 beta header 中移除该 token
	BetaPolicyActionBlock  = "block"  // 拦截，直接返回错误

	BetaPolicyScopeAll     = "all"     // 所有账号类型
	BetaPolicyScopeOAuth   = "oauth"   // 仅 OAuth 账号
	BetaPolicyScopeAPIKey  = "apikey"  // 仅 API Key 账号
	BetaPolicyScopeBedrock = "bedrock" // 仅 AWS Bedrock 账号
)

// BetaPolicyRule 单条 Beta 策略规则
type BetaPolicyRule struct {
	BetaToken            string   `json:"beta_token"`                       // beta token 值
	Action               string   `json:"action"`                           // "pass" | "filter" | "block"
	Scope                string   `json:"scope"`                            // "all" | "oauth" | "apikey" | "bedrock"
	ErrorMessage         string   `json:"error_message,omitempty"`          // 自定义错误消息 (action=block 时生效)
	ModelWhitelist       []string `json:"model_whitelist,omitempty"`        // 模型匹配模式列表（为空=对所有模型生效）
	FallbackAction       string   `json:"fallback_action,omitempty"`        // 未匹配白名单的模型的处理方式
	FallbackErrorMessage string   `json:"fallback_error_message,omitempty"` // 未匹配白名单时的自定义错误消息 (fallback_action=block 时生效)
}

// BetaPolicySettings Beta 策略配置
type BetaPolicySettings struct {
	Rules []BetaPolicyRule `json:"rules"`
}

// OpenAIAPIKeyHealthBreakerSettings controls cross-instance failure counting for OpenAI pool API keys.
type OpenAIAPIKeyHealthBreakerSettings struct {
	Enabled          bool `json:"enabled"`
	WindowMinutes    int  `json:"window_minutes"`
	FailureThreshold int  `json:"failure_threshold"`
	CooldownMinutes  int  `json:"cooldown_minutes"`
}

func DefaultOpenAIAPIKeyHealthBreakerSettings() *OpenAIAPIKeyHealthBreakerSettings {
	return &OpenAIAPIKeyHealthBreakerSettings{
		Enabled:          false,
		WindowMinutes:    2,
		FailureThreshold: 10,
		CooldownMinutes:  5,
	}
}

// OpenAI Fast Policy 策略常量
// OpenAI 的 "fast 模式" 通过请求体中的 service_tier 字段识别：
//   - "priority"（客户端可传 "fast"，归一化为 "priority"）：fast 模式
//   - "ultrafast"：Codex/API 的 Ultrafast 档位
//   - "flex"：低优先级模式
//   - 省略：normal 默认；策略中可用专用 "missing" 条件显式匹配
//
// 本策略复用 BetaPolicyAction*/BetaPolicyScope* 常量语义，只是匹配键从
// anthropic-beta header 换成 body 的 service_tier 字段。
const (
	OpenAIFastTierAny       = "all"       // 匹配任意已识别的 service_tier
	OpenAIFastTierPriority  = "priority"  // 仅匹配 fast（priority）
	OpenAIFastTierUltrafast = "ultrafast" // 仅匹配 ultrafast
	OpenAIFastTierFlex      = "flex"      // 仅匹配 flex
	OpenAIFastTierMissing   = "missing"   // 仅匹配省略 service_tier 的请求

	// OpenAIFastPolicyActionForcePriority 会保留 service_tier 字段并强制写成
	// priority，用于把 flex/auto/default/scale 等已识别 tier 收敛为 fast。
	OpenAIFastPolicyActionForcePriority = "force_priority"
)

// OpenAIFastPolicyRule 单条 OpenAI fast/flex 策略规则
type OpenAIFastPolicyRule struct {
	ServiceTier          string   `json:"service_tier"`                     // "priority" | "ultrafast" | "flex" | "missing" | "all"
	Action               string   `json:"action"`                           // "pass" | "filter" | "block" | "force_priority"
	Scope                string   `json:"scope"`                            // "all" | "oauth" | "apikey" | "bedrock"
	UserIDs              []int64  `json:"user_ids,omitempty"`               // 空=所有 Sub2API 用户；非空=仅指定 API Key 所属用户
	ErrorMessage         string   `json:"error_message,omitempty"`          // 自定义错误消息 (action=block 时生效)
	ModelWhitelist       []string `json:"model_whitelist,omitempty"`        // 模型匹配模式列表（为空=对所有模型生效）
	FallbackAction       string   `json:"fallback_action,omitempty"`        // 未匹配白名单的模型的处理方式
	FallbackErrorMessage string   `json:"fallback_error_message,omitempty"` // 未匹配白名单时的自定义错误消息 (fallback_action=block 时生效)
}

// OpenAIFastPolicySettings OpenAI fast 策略配置
type OpenAIFastPolicySettings struct {
	Rules []OpenAIFastPolicyRule `json:"rules"`
}
