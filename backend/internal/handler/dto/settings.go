package dto

import (
	"encoding/json"
	"strings"
)

// CustomMenuItem represents a user-configured custom menu entry.
type CustomMenuItem struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	IconSVG        string `json:"icon_svg"`
	URL            string `json:"url"`
	PageSlug       string `json:"page_slug,omitempty"`
	Visibility     string `json:"visibility"` // "user" or "admin"
	SortOrder      int    `json:"sort_order"`
	HideOpenButton bool   `json:"hide_open_button,omitempty"`
}

// CustomEndpoint represents an admin-configured API endpoint for quick copy.
type CustomEndpoint struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Description string `json:"description"`
}

// SystemSettings represents the admin settings API response payload.
type SystemSettings struct {
	AffiliateRebateRate          float64 `json:"affiliate_rebate_rate"`
	AffiliateRebateFreezeHours   int     `json:"affiliate_rebate_freeze_hours"`
	AffiliateRebateDurationDays  int     `json:"affiliate_rebate_duration_days"`
	AffiliateRebatePerInviteeCap float64 `json:"affiliate_rebate_per_invitee_cap"`
	AdminRechargeRebateEnabled   bool    `json:"affiliate_admin_recharge_enabled"`

	// Identity patch configuration (Claude -> Gemini)
	EnableIdentityPatch bool   `json:"enable_identity_patch"`
	IdentityPatchPrompt string `json:"identity_patch_prompt"`

	// Ops monitoring (vNext)
	OpsMonitoringEnabled         bool   `json:"ops_monitoring_enabled"` // 只读：部署配置 OPS_ENABLED
	OpsRealtimeMonitoringEnabled bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    int    `json:"ops_metrics_interval_seconds"`

	MinClaudeCodeVersion string `json:"min_claude_code_version"`
	MaxClaudeCodeVersion string `json:"max_claude_code_version"`

	// Gateway forwarding behavior
	OpenAITTFTMode                         string `json:"openai_ttft_mode"`
	EnableFingerprintUnification           bool   `json:"enable_fingerprint_unification"`
	EnableMetadataPassthrough              bool   `json:"enable_metadata_passthrough"`
	EnableCCHSigning                       bool   `json:"enable_cch_signing"`
	EnableClaudeOAuthSystemPromptInjection bool   `json:"enable_claude_oauth_system_prompt_injection"`
	ClaudeOAuthSystemPrompt                string `json:"claude_oauth_system_prompt"`
	ClaudeOAuthSystemPromptBlocks          string `json:"claude_oauth_system_prompt_blocks"`
	EnableAnthropicCacheTTL1hInjection     bool   `json:"enable_anthropic_cache_ttl_1h_injection"`
	RewriteMessageCacheControl             bool   `json:"rewrite_message_cache_control"`
	EnableClientDatelineNormalization      bool   `json:"enable_client_dateline_normalization"`
	AntigravityUserAgentVersion            string `json:"antigravity_user_agent_version"`
	OpenAICodexUserAgent                   string `json:"openai_codex_user_agent"`
	OpenAICodexClientVersion               string `json:"openai_codex_client_version"`
	OpenAICodexClientVersionSynced         string `json:"openai_codex_client_version_synced"`
	OpenAICodexVersionAutoSyncEnabled      bool   `json:"openai_codex_version_auto_sync_enabled"`

	// codex_cli_only 加固
	MinCodexVersion                      string `json:"min_codex_version"`
	MaxCodexVersion                      string `json:"max_codex_version"`
	CodexCLIOnlyBlacklist                string `json:"codex_cli_only_blacklist"`
	CodexCLIOnlyWhitelist                string `json:"codex_cli_only_whitelist"`
	CodexCLIOnlyAllowAppServerClients    bool   `json:"codex_cli_only_allow_app_server_clients"`
	CodexCLIOnlyEngineFingerprintSignals string `json:"codex_cli_only_engine_fingerprint_signals"`

	// Web Search Emulation
	WebSearchEmulationEnabled bool `json:"web_search_emulation_enabled"`

	// Force Alipay mobile clients to use QR code payment instead of mobile redirect
	// Use Alipay face-to-face precreate and an app deep link on mobile clients.

	// Channel Monitor feature switch
	ChannelMonitorMode                   string `json:"channel_monitor_mode"`
	ChannelMonitorDefaultIntervalSeconds int    `json:"channel_monitor_default_interval_seconds"`
	ChannelMonitorHideThroughput         bool   `json:"channel_monitor_hide_throughput"`
	ChannelMonitorShowQuota              bool   `json:"channel_monitor_show_quota"`
	ChannelMonitorHideUserRanking        bool   `json:"channel_monitor_hide_user_ranking"`

	// Grok model mapping policy (admin settings; empty account mapping falls back to these).
	GrokDefaultTextModel           string `json:"grok_default_text_model"`
	GrokCrossClientModelMapEnabled bool   `json:"grok_cross_client_model_map_enabled"`
	GrokDefaultBaseURLMode         string `json:"grok_default_base_url_mode"`

	// Model Plaza feature (public group/model pricing showcase)
	PluginManagementEnabled bool `json:"plugin_management_enabled"`

	// 风控中心功能开关
	RiskControlEnabled bool `json:"risk_control_enabled"`

	// cyber 会话屏蔽开关 + TTL
	CyberSessionBlockEnabled    bool `json:"cyber_session_block_enabled"`
	CyberSessionBlockTTLSeconds int  `json:"cyber_session_block_ttl_seconds"`

	// Affiliate (邀请返利) feature switch
	AffiliateEnabled bool `json:"affiliate_enabled"`

	// OpenAI fast/flex policy
	OpenAIFastPolicySettings *OpenAIFastPolicySettings `json:"openai_fast_policy_settings,omitempty"`

	// 系统全局账号自动停调阈值（key = platform，100 = disabled）
	AccountSchedulingThresholds map[string]int `json:"account_scheduling_thresholds,omitempty"`

	// 利润门（全站一档）
	ProfitControlEnabled bool    `json:"profit_control_enabled"`
	ProfitMinMargin      float64 `json:"profit_min_margin"`
	ProfitSafetyBuffer   float64 `json:"profit_safety_buffer"`
}

type PublicSettings struct {
	RegistrationEnabled                 bool                     `json:"registration_enabled"`
	EmailVerifyEnabled                  bool                     `json:"email_verify_enabled"`
	ForceEmailOnThirdPartySignup        bool                     `json:"force_email_on_third_party_signup"`
	RegistrationEmailSuffixWhitelist    []string                 `json:"registration_email_suffix_whitelist"`
	RegistrationEmailDomainQuotaEnabled bool                     `json:"registration_email_domain_quota_enabled"`
	PasswordResetEnabled                bool                     `json:"password_reset_enabled"`
	InvitationCodeEnabled               bool                     `json:"invitation_code_enabled"`
	PasskeyEnabled                      bool                     `json:"passkey_enabled"`
	LoginAgreementEnabled               bool                     `json:"login_agreement_enabled"`
	LoginAgreementMode                  string                   `json:"login_agreement_mode"`
	LoginAgreementUpdatedAt             string                   `json:"login_agreement_updated_at"`
	LoginAgreementRevision              string                   `json:"login_agreement_revision"`
	LoginAgreementDocuments             []LoginAgreementDocument `json:"login_agreement_documents"`
	TurnstileEnabled                    bool                     `json:"turnstile_enabled"`
	TurnstileSiteKey                    string                   `json:"turnstile_site_key"`
	TencentCaptchaEnabled               bool                     `json:"tencent_captcha_enabled"`
	TencentCaptchaAppID                 string                   `json:"tencent_captcha_app_id"`
	TencentCaptchaRegion                string                   `json:"tencent_captcha_region"`
	AliyunCaptchaEnabled                bool                     `json:"aliyun_captcha_enabled"`
	AliyunCaptchaSceneID                string                   `json:"aliyun_captcha_scene_id"`
	AliyunCaptchaPrefix                 string                   `json:"aliyun_captcha_prefix"`
	AliyunCaptchaRegion                 string                   `json:"aliyun_captcha_region"`
	SiteName                            string                   `json:"site_name"`
	SiteLogo                            string                   `json:"site_logo"`
	SiteSubtitle                        string                   `json:"site_subtitle"`
	APIBaseURL                          string                   `json:"api_base_url"`
	ContactInfo                         string                   `json:"contact_info"`
	DocURL                              string                   `json:"doc_url"`
	HomeContent                         string                   `json:"home_content"`
	CompactHomeEnabled                  bool                     `json:"compact_home_enabled"`
	HideCcsImportButton                 bool                     `json:"hide_ccs_import_button"`
	TableDefaultPageSize                int                      `json:"table_default_page_size"`
	TablePageSizeOptions                []int                    `json:"table_page_size_options"`
	CustomMenuItems                     []CustomMenuItem         `json:"custom_menu_items"`
	CustomEndpoints                     []CustomEndpoint         `json:"custom_endpoints"`
	WeChatOAuthEnabled                  bool                     `json:"wechat_oauth_enabled"`
	WeChatOAuthOpenEnabled              bool                     `json:"wechat_oauth_open_enabled"`
	WeChatOAuthMPEnabled                bool                     `json:"wechat_oauth_mp_enabled"`
	WeChatOAuthMobileEnabled            bool                     `json:"wechat_oauth_mobile_enabled"`
	GitHubOAuthEnabled                  bool                     `json:"github_oauth_enabled"`
	GoogleOAuthEnabled                  bool                     `json:"google_oauth_enabled"`
	BackendModeEnabled                  bool                     `json:"backend_mode_enabled"`
	PaymentEnabled                      bool                     `json:"payment_enabled"`
	Version                             string                   `json:"version"`
	// 服务器全局时区（IANA 名称与当前 UTC 偏移，如 "Asia/Shanghai" / "+08:00"）。
	// 高峰时段等按服务器本地时间判定的窗口，前端展示时据此标注，避免用户按浏览器本地时间误读。
	ServerTimezone              string  `json:"server_timezone"`
	ServerUTCOffset             string  `json:"server_utc_offset"`
	BalanceLowNotifyEnabled     bool    `json:"balance_low_notify_enabled"`
	AccountQuotaNotifyEnabled   bool    `json:"account_quota_notify_enabled"`
	BalanceLowNotifyThreshold   float64 `json:"balance_low_notify_threshold"`
	BalanceLowNotifyRechargeURL string  `json:"balance_low_notify_recharge_url"`

	ChannelMonitorEnabled                bool   `json:"channel_monitor_enabled"`
	ChannelMonitorMode                   string `json:"channel_monitor_mode"`
	ChannelMonitorDefaultIntervalSeconds int    `json:"channel_monitor_default_interval_seconds"`
	ChannelMonitorHideThroughput         bool   `json:"channel_monitor_hide_throughput"`
	ChannelMonitorShowQuota              bool   `json:"channel_monitor_show_quota"`
	ChannelMonitorHideUserRanking        bool   `json:"channel_monitor_hide_user_ranking"`

	PluginManagementEnabled bool `json:"plugin_management_enabled"`

	AffiliateEnabled bool `json:"affiliate_enabled"`

	RiskControlEnabled bool `json:"risk_control_enabled"`

	AllowUserViewErrorRequests bool `json:"allow_user_view_error_requests"`
}

type LoginAgreementDocument struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ContentMD string `json:"content_md"`
}

// OverloadCooldownSettings 529过载冷却配置 DTO
type OverloadCooldownSettings struct {
	Enabled         bool `json:"enabled"`
	CooldownMinutes int  `json:"cooldown_minutes"`
}

// RateLimit429CooldownSettings 429默认回避配置 DTO
type RateLimit429CooldownSettings struct {
	Enabled         bool `json:"enabled"`
	CooldownSeconds int  `json:"cooldown_seconds"`
}

type OpenAIImagesOAuthUnavailableCooldownSettings struct {
	CooldownMinutes int `json:"cooldown_minutes"`
}

// StreamTimeoutSettings 流超时处理配置 DTO
type StreamTimeoutSettings struct {
	Enabled                bool   `json:"enabled"`
	Action                 string `json:"action"`
	TempUnschedMinutes     int    `json:"temp_unsched_minutes"`
	ThresholdCount         int    `json:"threshold_count"`
	ThresholdWindowMinutes int    `json:"threshold_window_minutes"`
}

// RectifierSettings 请求整流器配置 DTO
type RectifierSettings struct {
	Enabled                  bool     `json:"enabled"`
	ThinkingSignatureEnabled bool     `json:"thinking_signature_enabled"`
	ThinkingBudgetEnabled    bool     `json:"thinking_budget_enabled"`
	APIKeySignatureEnabled   bool     `json:"apikey_signature_enabled"`
	APIKeySignaturePatterns  []string `json:"apikey_signature_patterns"`
}

// BetaPolicyRule Beta 策略规则 DTO
type BetaPolicyRule struct {
	BetaToken            string   `json:"beta_token"`
	Action               string   `json:"action"`
	Scope                string   `json:"scope"`
	ErrorMessage         string   `json:"error_message,omitempty"`
	ModelWhitelist       []string `json:"model_whitelist,omitempty"`
	FallbackAction       string   `json:"fallback_action,omitempty"`
	FallbackErrorMessage string   `json:"fallback_error_message,omitempty"`
}

// BetaPolicySettings Beta 策略配置 DTO
type BetaPolicySettings struct {
	Rules []BetaPolicyRule `json:"rules"`
}

// OpenAIFastPolicyRule OpenAI fast/flex 策略规则 DTO
type OpenAIFastPolicyRule struct {
	ServiceTier          string   `json:"service_tier"`
	Action               string   `json:"action"`
	Scope                string   `json:"scope"`
	UserIDs              []int64  `json:"user_ids,omitempty"`
	ErrorMessage         string   `json:"error_message,omitempty"`
	ModelWhitelist       []string `json:"model_whitelist,omitempty"`
	FallbackAction       string   `json:"fallback_action,omitempty"`
	FallbackErrorMessage string   `json:"fallback_error_message,omitempty"`
}

// OpenAIFastPolicySettings OpenAI fast 策略配置 DTO
type OpenAIFastPolicySettings struct {
	Rules []OpenAIFastPolicyRule `json:"rules"`
}

// EmailTemplateEventOption 描述可编辑的通知邮件事件。
type EmailTemplateEventOption struct {
	Value       string `json:"value"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Optional    bool   `json:"optional,omitempty"`
}

// EmailTemplateSummary is shown in the admin email template list.
type EmailTemplateSummary struct {
	Event     string `json:"event"`
	Locale    string `json:"locale"`
	Subject   string `json:"subject"`
	IsCustom  bool   `json:"is_custom,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// EmailTemplateListResponse is returned by GET /admin/settings/email-templates.
type EmailTemplateListResponse struct {
	Events       []EmailTemplateEventOption `json:"events"`
	Locales      []string                   `json:"locales"`
	Templates    []EmailTemplateSummary     `json:"templates,omitempty"`
	Placeholders []string                   `json:"placeholders,omitempty"`
}

// EmailTemplateDetail is returned for a specific event/locale template.
type EmailTemplateDetail struct {
	Event        string   `json:"event"`
	Locale       string   `json:"locale"`
	Subject      string   `json:"subject"`
	HTML         string   `json:"html"`
	IsCustom     bool     `json:"is_custom,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
	Placeholders []string `json:"placeholders,omitempty"`
}

// UpdateEmailTemplateRequest updates a template override.
type UpdateEmailTemplateRequest struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

// PreviewEmailTemplateRequest previews a template without saving it.
type PreviewEmailTemplateRequest struct {
	Event     string            `json:"event"`
	Locale    string            `json:"locale"`
	Subject   string            `json:"subject"`
	HTML      string            `json:"html"`
	Variables map[string]string `json:"variables,omitempty"`
}

// EmailTemplatePreviewResponse is the rendered preview payload.
type EmailTemplatePreviewResponse struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

// ParseCustomMenuItems parses a JSON string into a slice of CustomMenuItem.
// Returns empty slice on empty/invalid input.
func ParseCustomMenuItems(raw string) []CustomMenuItem {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return []CustomMenuItem{}
	}
	var items []CustomMenuItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []CustomMenuItem{}
	}
	return items
}

// ParseUserVisibleMenuItems parses custom menu items and filters out admin-only entries.
func ParseUserVisibleMenuItems(raw string) []CustomMenuItem {
	items := ParseCustomMenuItems(raw)
	filtered := make([]CustomMenuItem, 0, len(items))
	for _, item := range items {
		if item.Visibility != "admin" {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// ParseCustomEndpoints parses a JSON string into a slice of CustomEndpoint.
// Returns empty slice on empty/invalid input.
func ParseCustomEndpoints(raw string) []CustomEndpoint {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return []CustomEndpoint{}
	}
	var items []CustomEndpoint
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []CustomEndpoint{}
	}
	return items
}
