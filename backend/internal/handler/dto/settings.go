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

	// Ops monitoring (vNext)
	OpsMonitoringEnabled         bool   `json:"ops_monitoring_enabled"` // 只读：部署配置 OPS_ENABLED
	OpsRealtimeMonitoringEnabled bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    int    `json:"ops_metrics_interval_seconds"`

	// Web Search Emulation
	WebSearchEmulationEnabled bool `json:"web_search_emulation_enabled"`

	// Channel Monitor feature switch
	ChannelMonitorMode                   string `json:"channel_monitor_mode"`
	ChannelMonitorDefaultIntervalSeconds int    `json:"channel_monitor_default_interval_seconds"`
	ChannelMonitorShowQuota              bool   `json:"channel_monitor_show_quota"`

	// 风控中心功能开关
	RiskControlEnabled bool `json:"risk_control_enabled"`

	// cyber 会话屏蔽开关 + TTL
	CyberSessionBlockEnabled    bool `json:"cyber_session_block_enabled"`
	CyberSessionBlockTTLSeconds int  `json:"cyber_session_block_ttl_seconds"`

	// Affiliate (邀请返利) feature switch
	AffiliateEnabled bool `json:"affiliate_enabled"`

	// 利润门：最低毛利率（全站一档；0 = 关）
	ProfitMinMargin float64 `json:"profit_min_margin"`
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
	LoginAgreementUpdatedAt             string                   `json:"login_agreement_updated_at"`
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
	ChannelMonitorShowQuota              bool   `json:"channel_monitor_show_quota"`

	AffiliateEnabled bool `json:"affiliate_enabled"`

	RiskControlEnabled bool `json:"risk_control_enabled"`

	AllowUserViewErrorRequests bool `json:"allow_user_view_error_requests"`
}

type LoginAgreementDocument struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ContentMD string `json:"content_md"`
}

type OpenAIImagesOAuthUnavailableCooldownSettings struct {
	CooldownMinutes int `json:"cooldown_minutes"`
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
