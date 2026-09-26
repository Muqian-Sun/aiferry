//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type settingPublicRepoStub struct {
	values map[string]string
	err    error
}

func (s *settingPublicRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingPublicRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *settingPublicRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingPublicRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *settingPublicRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingPublicRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingPublicRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingService_GetPublicSettings_ExposesRegistrationEmailSuffixWhitelist(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyEmailVerifyEnabled:               "true",
			SettingKeyRegistrationEmailSuffixWhitelist: `["@EXAMPLE.com"," @foo.bar ","*.EDU.CN","@invalid_domain",""]`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"@example.com", "@foo.bar", "*.edu.cn"}, settings.RegistrationEmailSuffixWhitelist)
}

func TestSettingService_ChannelMonitorHideThroughputDefaultsToPrivate(t *testing.T) {
	missing := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.True(t, missing.HideThroughput)
	public, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, public.ChannelMonitorHideThroughput)

	for _, value := range []string{"false", "0", "off", "disabled"} {
		runtime := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyChannelMonitorHideThroughput: value,
		}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
		require.False(t, runtime.HideThroughput, "value=%q", value)
	}
}

func TestSettingService_ChannelMonitorShowQuotaFailsClosed(t *testing.T) {
	// 缺省（迁移插入 'false' / 老库无行）一律不展示。
	missingRuntime := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.False(t, missingRuntime.ShowQuota)
	missingPublic, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).
		GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, missingPublic.ChannelMonitorShowQuota)

	// 仅字面 "true" 视为开启；其余值（含异常值）fail-closed。
	runtime := NewSettingService(&settingPublicRepoStub{values: map[string]string{
		SettingKeyChannelMonitorShowQuota: "true",
	}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.True(t, runtime.ShowQuota)

	for _, value := range []string{"false", "TRUE", "1", "yes", "on", "garbage"} {
		rt := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyChannelMonitorShowQuota: value,
		}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
		require.False(t, rt.ShowQuota, "value=%q", value)
	}
}

func TestSettingService_ChannelMonitorHideUserRankingDefaultsToVisible(t *testing.T) {
	missing := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
	require.False(t, missing.HideUserRanking)
	public, err := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, public.ChannelMonitorHideUserRanking)

	for _, value := range []string{"true", "1", "on", "enabled"} {
		runtime := NewSettingService(&settingPublicRepoStub{values: map[string]string{
			SettingKeyChannelMonitorHideUserRanking: value,
		}}, &config.Config{}).GetChannelMonitorRuntime(context.Background())
		require.True(t, runtime.HideUserRanking, "value=%q", value)
	}
}

func TestSettingService_GetPublicSettings_ExposesForceEmailOnThirdPartySignup(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyForceEmailOnThirdPartySignup: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.ForceEmailOnThirdPartySignup)
}

func TestSettingService_GetPublicSettings_ExposesAllowUserViewErrorRequests(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyAllowUserViewErrorRequests: "true",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.AllowUserViewErrorRequests)
}

func TestSettingService_GetPublicSettings_ExposesWeChatOAuthModeCapabilities(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeyWeChatConnectEnabled:             "true",
			SettingKeyWeChatConnectAppID:               "wx-mp-app",
			SettingKeyWeChatConnectAppSecret:           "wx-mp-secret",
			SettingKeyWeChatConnectMode:                "mp",
			SettingKeyWeChatConnectScopes:              "snsapi_base",
			SettingKeyWeChatConnectOpenEnabled:         "true",
			SettingKeyWeChatConnectMPEnabled:           "true",
			SettingKeyWeChatConnectRedirectURL:         "https://api.example.com/api/v1/auth/oauth/wechat/callback",
			SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
		},
	}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.WeChatOAuthEnabled)
	require.True(t, settings.WeChatOAuthOpenEnabled)
	require.True(t, settings.WeChatOAuthMPEnabled)
}

func TestSettingService_GetPublicSettings_DoesNotExposeMobileOnlyWeChatAsWebOAuthAvailable(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{
		values: map[string]string{
			SettingKeyWeChatConnectEnabled:             "true",
			SettingKeyWeChatConnectMobileEnabled:       "true",
			SettingKeyWeChatConnectMode:                "mobile",
			SettingKeyWeChatConnectMobileAppID:         "wx-mobile-app",
			SettingKeyWeChatConnectMobileAppSecret:     "wx-mobile-secret",
			SettingKeyWeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
		},
	}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.WeChatOAuthEnabled)
	require.False(t, settings.WeChatOAuthOpenEnabled)
	require.False(t, settings.WeChatOAuthMPEnabled)
	require.True(t, settings.WeChatOAuthMobileEnabled)
}

func TestSettingService_GetPublicSettings_FallsBackToConfigForWeChatOAuthCapabilities(t *testing.T) {
	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			OpenEnabled:         true,
			OpenAppID:           "wx-open-config",
			OpenAppSecret:       "wx-open-secret",
			FrontendRedirectURL: "/auth/wechat/config-callback",
		},
	})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.WeChatOAuthEnabled)
	require.True(t, settings.WeChatOAuthOpenEnabled)
	require.False(t, settings.WeChatOAuthMPEnabled)
	require.False(t, settings.WeChatOAuthMobileEnabled)
}

// 双因素认证只看 TOTP_ENCRYPTION_KEY 配没配：设置表里残留的 totp_enabled 不起作用。
// 以前关掉这个开关，已绑 TOTP 的账号（含管理员）登录就不再要验证码。
func TestSettingService_IsTotpEnabled_FollowsEncryptionKeyOnly(t *testing.T) {
	staleOff := &settingPublicRepoStub{values: map[string]string{"totp_enabled": "false"}}
	withKey := NewSettingService(staleOff, &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}})
	require.True(t, withKey.IsTotpEnabled(), "配了密钥：库里的旧开关关着也要验证码")

	staleOn := &settingPublicRepoStub{values: map[string]string{"totp_enabled": "true"}}
	withoutKey := NewSettingService(staleOn, &config.Config{})
	require.False(t, withoutKey.IsTotpEnabled(), "没配密钥：库里的旧开关开着也不可用")
}

// 站点相关由代码决定（site_features.go）：库里旧设置行不再影响公开设置。
func TestSettingService_GetPublicSettings_SiteFieldsComeFromCode(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			"site_name":               "Stale Name",
			"site_logo":               "https://stale.example/logo.png",
			"contact_info":            "stale@example.com",
			"doc_url":                 "https://stale.example/docs",
			"home_content":            "<h1>stale</h1>",
			"compact_home_enabled":    "true",
			"table_default_page_size": "50",
			"table_page_size_options": "[5]",
			"custom_menu_items":       `[{"id":"x","label":"x","url":"https://stale.example"}]`,
			"custom_endpoints":        `[{"name":"x","endpoint":"https://stale.example"}]`,
			"api_base_url":            "https://stale.example",
		},
	}
	svc := NewSettingService(repo, &config.Config{Server: config.ServerConfig{FrontendURL: "https://user.example"}})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, SiteName, settings.SiteName)
	require.Equal(t, SiteLogo, settings.SiteLogo)
	require.Equal(t, SiteContactInfo, settings.ContactInfo)
	require.Equal(t, SiteDocURL, settings.DocURL)
	require.Empty(t, settings.HomeContent)
	require.False(t, settings.CompactHomeEnabled)
	require.Equal(t, TableDefaultPageSize, settings.TableDefaultPageSize)
	require.Equal(t, TablePageSizeOptions(), settings.TablePageSizeOptions)
	require.Equal(t, "[]", settings.CustomMenuItems)
	require.Equal(t, "[]", settings.CustomEndpoints)
	require.Equal(t, "https://user.example", settings.APIBaseURL)
}

// 条款由代码决定：库里残留的旧开关 / 旧正文不生效，一律用 legal/*.md 与代码里的日期。
func TestSettingService_GetPublicSettings_LoginAgreementComesFromCode(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			"login_agreement_enabled":    "false",
			"login_agreement_mode":       "checkbox",
			"login_agreement_updated_at": "2020-01-01",
			"login_agreement_documents":  `[{"id":"stale","title":"旧条款","content_md":"stale"}]`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.LoginAgreementEnabled)
	require.Equal(t, LoginAgreementMode, settings.LoginAgreementMode)
	require.Equal(t, LoginAgreementUpdatedAt, settings.LoginAgreementUpdatedAt)
	require.Equal(t, LoginAgreementDocuments(), settings.LoginAgreementDocuments)
	require.Equal(t, buildLoginAgreementRevision(LoginAgreementUpdatedAt, LoginAgreementDocuments()), settings.LoginAgreementRevision)
}
