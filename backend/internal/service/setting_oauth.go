package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 第三方登录只认部署配置（环境变量 / config.yaml），后台不能开也不能改，数据库里旧的设置行一律不读。
//   - Google / GitHub：配了 Client ID 和 Secret 就开；回调地址没配时按用户站地址推出来。
//   - 微信：WECHAT_CONNECT_ENABLED 加凭证，启动时 config.Validate 已校验过一遍。

// emailOAuthCallbackPath 是 Google / GitHub 的后端回调路由（routes/auth.go），用户站与 API 同一个域名。
const emailOAuthCallbackPathFormat = "/api/v1/auth/oauth/%s/callback"

func normalizeWeChatConnectModeSetting(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mp":
		return "mp"
	case "mobile":
		return "mobile"
	default:
		return "open"
	}
}

func defaultWeChatConnectScopeForMode(mode string) string {
	switch normalizeWeChatConnectModeSetting(mode) {
	case "mp":
		return "snsapi_userinfo"
	case "mobile":
		return ""
	}
	return defaultWeChatConnectScopes
}

func normalizeWeChatConnectScopeSetting(raw, mode string) string {
	switch normalizeWeChatConnectModeSetting(mode) {
	case "mp":
		switch strings.TrimSpace(raw) {
		case "snsapi_base":
			return "snsapi_base"
		case "snsapi_userinfo":
			return "snsapi_userinfo"
		default:
			return defaultWeChatConnectScopeForMode(mode)
		}
	case "mobile":
		return ""
	default:
		return defaultWeChatConnectScopes
	}
}

func normalizeWeChatConnectStoredMode(openEnabled, mpEnabled, mobileEnabled bool, mode string) string {
	mode = normalizeWeChatConnectModeSetting(mode)
	switch mode {
	case "open":
		if openEnabled {
			return "open"
		}
	case "mp":
		if mpEnabled {
			return "mp"
		}
	case "mobile":
		if mobileEnabled {
			return "mobile"
		}
	}
	switch {
	case openEnabled:
		return "open"
	case mpEnabled:
		return "mp"
	case mobileEnabled:
		return "mobile"
	default:
		return mode
	}
}

// weChatConnectOAuthConfigFrom 把部署配置里的微信登录整理成最终生效的配置：
// 旧的单套 app_id / app_secret 兜底到三种形态；开了总开关但一种形态都没开时按 mode 开一种。
func weChatConnectOAuthConfigFrom(base config.WeChatConnectConfig) WeChatConnectOAuthConfig {
	legacyAppID := strings.TrimSpace(firstNonEmpty(base.AppID, base.OpenAppID, base.MPAppID, base.MobileAppID))
	legacyAppSecret := strings.TrimSpace(firstNonEmpty(base.AppSecret, base.OpenAppSecret, base.MPAppSecret, base.MobileAppSecret))

	openEnabled, mpEnabled, mobileEnabled := false, false, false
	if base.Enabled {
		openEnabled, mpEnabled, mobileEnabled = base.OpenEnabled, base.MPEnabled, base.MobileEnabled
		if !openEnabled && !mpEnabled && !mobileEnabled {
			switch normalizeWeChatConnectModeSetting(base.Mode) {
			case "mp":
				mpEnabled = true
			case "mobile":
				mobileEnabled = true
			default:
				openEnabled = true
			}
		}
	}
	mode := normalizeWeChatConnectStoredMode(openEnabled, mpEnabled, mobileEnabled, base.Mode)

	return WeChatConnectOAuthConfig{
		Enabled:             base.Enabled,
		LegacyAppID:         legacyAppID,
		LegacyAppSecret:     legacyAppSecret,
		OpenAppID:           strings.TrimSpace(firstNonEmpty(base.OpenAppID, legacyAppID)),
		OpenAppSecret:       strings.TrimSpace(firstNonEmpty(base.OpenAppSecret, legacyAppSecret)),
		MPAppID:             strings.TrimSpace(firstNonEmpty(base.MPAppID, legacyAppID)),
		MPAppSecret:         strings.TrimSpace(firstNonEmpty(base.MPAppSecret, legacyAppSecret)),
		MobileAppID:         strings.TrimSpace(firstNonEmpty(base.MobileAppID, legacyAppID)),
		MobileAppSecret:     strings.TrimSpace(firstNonEmpty(base.MobileAppSecret, legacyAppSecret)),
		OpenEnabled:         openEnabled,
		MPEnabled:           mpEnabled,
		MobileEnabled:       mobileEnabled,
		Mode:                mode,
		Scopes:              normalizeWeChatConnectScopeSetting(base.Scopes, mode),
		RedirectURL:         strings.TrimSpace(base.RedirectURL),
		FrontendRedirectURL: strings.TrimSpace(firstNonEmpty(base.FrontendRedirectURL, defaultWeChatConnectFrontend)),
	}
}

func validateWeChatConnectOAuthConfig(cfg WeChatConnectOAuthConfig) (WeChatConnectOAuthConfig, error) {
	if !cfg.Enabled || (!cfg.OpenEnabled && !cfg.MPEnabled) {
		return WeChatConnectOAuthConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "wechat oauth is disabled")
	}
	if cfg.OpenEnabled {
		if cfg.AppIDForMode("open") == "" {
			return WeChatConnectOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "wechat oauth pc app id not configured")
		}
		if cfg.AppSecretForMode("open") == "" {
			return WeChatConnectOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "wechat oauth pc app secret not configured")
		}
	}
	if cfg.MPEnabled {
		if cfg.AppIDForMode("mp") == "" {
			return WeChatConnectOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "wechat oauth official account app id not configured")
		}
		if cfg.AppSecretForMode("mp") == "" {
			return WeChatConnectOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "wechat oauth official account app secret not configured")
		}
	}
	if cfg.MobileEnabled {
		if cfg.AppIDForMode("mobile") == "" {
			return WeChatConnectOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "wechat oauth mobile app id not configured")
		}
		if cfg.AppSecretForMode("mobile") == "" {
			return WeChatConnectOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "wechat oauth mobile app secret not configured")
		}
	}
	if v := strings.TrimSpace(cfg.RedirectURL); v != "" {
		if err := config.ValidateAbsoluteHTTPURL(v); err != nil {
			return WeChatConnectOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "wechat oauth redirect url invalid")
		}
	}
	if err := config.ValidateFrontendRedirectURL(cfg.FrontendRedirectURL); err != nil {
		return WeChatConnectOAuthConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "wechat oauth frontend redirect url invalid")
	}
	return cfg, nil
}

func (s *SettingService) weChatConnectBase() config.WeChatConnectConfig {
	if s == nil || s.cfg == nil {
		return config.WeChatConnectConfig{}
	}
	return s.cfg.WeChat
}

// weChatOAuthCapabilities 返回（能登录、PC 扫码可用、公众号可用、移动应用可用），凭证不全的形态算不可用。
func (s *SettingService) weChatOAuthCapabilities() (bool, bool, bool, bool) {
	cfg := weChatConnectOAuthConfigFrom(s.weChatConnectBase())
	if !cfg.Enabled {
		return false, false, false, false
	}

	openReady := cfg.OpenEnabled && cfg.AppIDForMode("open") != "" && cfg.AppSecretForMode("open") != ""
	mpReady := cfg.MPEnabled && cfg.AppIDForMode("mp") != "" && cfg.AppSecretForMode("mp") != ""
	mobileReady := cfg.MobileEnabled && cfg.AppIDForMode("mobile") != "" && cfg.AppSecretForMode("mobile") != ""

	return openReady || mpReady, openReady, mpReady, mobileReady
}

// effectiveEmailOAuthConfig 取 Google / GitHub 的最终生效配置：部署配置覆盖内置默认值，
// 回调地址没配时按用户站地址拼后端回调路由。
func (s *SettingService) effectiveEmailOAuthConfig(provider string) config.EmailOAuthProviderConfig {
	var cfg, override config.EmailOAuthProviderConfig
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "github":
		cfg = config.EmailOAuthProviderConfig{
			AuthorizeURL:        defaultGitHubOAuthAuthorize,
			TokenURL:            defaultGitHubOAuthToken,
			UserInfoURL:         defaultGitHubOAuthUserInfo,
			EmailsURL:           defaultGitHubOAuthEmails,
			Scopes:              defaultGitHubOAuthScopes,
			FrontendRedirectURL: defaultGitHubOAuthFrontend,
		}
		if s != nil && s.cfg != nil {
			override = s.cfg.GitHubOAuth
		}
	case "google":
		cfg = config.EmailOAuthProviderConfig{
			AuthorizeURL:        defaultGoogleOAuthAuthorize,
			TokenURL:            defaultGoogleOAuthToken,
			UserInfoURL:         defaultGoogleOAuthUserInfo,
			Scopes:              defaultGoogleOAuthScopes,
			FrontendRedirectURL: defaultGoogleOAuthFrontend,
		}
		if s != nil && s.cfg != nil {
			override = s.cfg.GoogleOAuth
		}
	default:
		return config.EmailOAuthProviderConfig{}
	}

	cfg.ClientID = strings.TrimSpace(override.ClientID)
	cfg.ClientSecret = strings.TrimSpace(override.ClientSecret)
	cfg.AuthorizeURL = firstNonEmpty(override.AuthorizeURL, cfg.AuthorizeURL)
	cfg.TokenURL = firstNonEmpty(override.TokenURL, cfg.TokenURL)
	cfg.UserInfoURL = firstNonEmpty(override.UserInfoURL, cfg.UserInfoURL)
	cfg.EmailsURL = firstNonEmpty(override.EmailsURL, cfg.EmailsURL)
	cfg.Scopes = firstNonEmpty(override.Scopes, cfg.Scopes)
	cfg.FrontendRedirectURL = firstNonEmpty(override.FrontendRedirectURL, cfg.FrontendRedirectURL)
	cfg.RedirectURL = strings.TrimSpace(override.RedirectURL)
	if cfg.RedirectURL == "" {
		if base := strings.TrimRight(s.GetFrontendURL(context.Background()), "/"); base != "" {
			cfg.RedirectURL = base + fmt.Sprintf(emailOAuthCallbackPathFormat, strings.ToLower(strings.TrimSpace(provider)))
		}
	}
	return cfg
}

// emailOAuthEnabled Google / GitHub 配了 Client ID 和 Secret 就开。
func emailOAuthEnabled(cfg config.EmailOAuthProviderConfig) bool {
	return strings.TrimSpace(cfg.ClientID) != "" && strings.TrimSpace(cfg.ClientSecret) != ""
}

func (s *SettingService) GetEmailOAuthProviderConfig(ctx context.Context, provider string) (config.EmailOAuthProviderConfig, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "github" && provider != "google" {
		return config.EmailOAuthProviderConfig{}, infraerrors.NotFound("OAUTH_PROVIDER_NOT_FOUND", "oauth provider not found")
	}
	cfg := s.effectiveEmailOAuthConfig(provider)
	if !emailOAuthEnabled(cfg) {
		return config.EmailOAuthProviderConfig{}, infraerrors.NotFound("OAUTH_DISABLED", "oauth login is disabled")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return config.EmailOAuthProviderConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth client id not configured")
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return config.EmailOAuthProviderConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth client secret not configured")
	}
	for label, rawURL := range map[string]string{
		"authorize": cfg.AuthorizeURL,
		"token":     cfg.TokenURL,
		"userinfo":  cfg.UserInfoURL,
		"redirect":  cfg.RedirectURL,
	} {
		if strings.TrimSpace(rawURL) == "" {
			return config.EmailOAuthProviderConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth "+label+" url not configured")
		}
		if err := config.ValidateAbsoluteHTTPURL(rawURL); err != nil {
			return config.EmailOAuthProviderConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth "+label+" url invalid")
		}
	}
	if strings.TrimSpace(cfg.EmailsURL) != "" {
		if err := config.ValidateAbsoluteHTTPURL(cfg.EmailsURL); err != nil {
			return config.EmailOAuthProviderConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth emails url invalid")
		}
	}
	if err := config.ValidateFrontendRedirectURL(cfg.FrontendRedirectURL); err != nil {
		return config.EmailOAuthProviderConfig{}, infraerrors.InternalServer("OAUTH_CONFIG_INVALID", "oauth frontend redirect url invalid")
	}
	return cfg, nil
}

// GetWeChatConnectOAuthConfig 返回用于登录的微信配置（只认部署配置）。
func (s *SettingService) GetWeChatConnectOAuthConfig(ctx context.Context) (WeChatConnectOAuthConfig, error) {
	return validateWeChatConnectOAuthConfig(weChatConnectOAuthConfigFrom(s.weChatConnectBase()))
}
