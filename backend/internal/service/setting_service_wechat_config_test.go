//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type settingWeChatRepoStub struct {
	values map[string]string
}

func (s *settingWeChatRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingWeChatRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *settingWeChatRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *settingWeChatRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *settingWeChatRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingWeChatRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingWeChatRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

// 数据库里旧的微信设置行（后台曾经能改）一律不读，只认部署配置。
func TestSettingService_GetWeChatConnectOAuthConfig_IgnoresDatabaseRows(t *testing.T) {
	repo := &settingWeChatRepoStub{
		values: map[string]string{
			"wechat_connect_enabled":      "false",
			"wechat_connect_open_app_id":  "wx-db-app",
			"wechat_connect_mode":         "mp",
			"wechat_connect_redirect_url": "https://db.example.com/callback",
		},
	}
	svc := NewSettingService(repo, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:       true,
			OpenEnabled:   true,
			Mode:          "open",
			OpenAppID:     "wx-open-config",
			OpenAppSecret: "wx-open-secret",
		},
	})

	got, err := svc.GetWeChatConnectOAuthConfig(context.Background())
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, "open", got.Mode)
	require.Equal(t, "wx-open-config", got.AppIDForMode("open"))
	require.Empty(t, got.RedirectURL)
	require.Equal(t, "/auth/wechat/callback", got.FrontendRedirectURL)
}

func TestSettingService_GetWeChatConnectOAuthConfig_ReadsDeploymentConfig(t *testing.T) {
	svc := NewSettingService(&settingWeChatRepoStub{values: map[string]string{}}, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:             true,
			OpenEnabled:         true,
			MPEnabled:           true,
			Mode:                "open",
			OpenAppID:           "wx-open-config",
			OpenAppSecret:       "wx-open-secret",
			MPAppID:             "wx-mp-config",
			MPAppSecret:         "wx-mp-secret",
			FrontendRedirectURL: "/auth/wechat/config-callback",
		},
	})

	got, err := svc.GetWeChatConnectOAuthConfig(context.Background())
	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.True(t, got.OpenEnabled)
	require.True(t, got.MPEnabled)
	require.Equal(t, "wx-open-config", got.AppIDForMode("open"))
	require.Equal(t, "wx-open-secret", got.AppSecretForMode("open"))
	require.Equal(t, "wx-mp-config", got.AppIDForMode("mp"))
	require.Equal(t, "wx-mp-secret", got.AppSecretForMode("mp"))
	require.Equal(t, "/auth/wechat/config-callback", got.FrontendRedirectURL)
	require.Empty(t, got.RedirectURL)
}

// 只配了总开关和旧的单套 app_id / app_secret：按 mode 开一种形态，凭证兜底到该形态。
func TestSettingService_GetWeChatConnectOAuthConfig_LegacySingleAppFollowsMode(t *testing.T) {
	svc := NewSettingService(&settingWeChatRepoStub{values: map[string]string{}}, &config.Config{
		WeChat: config.WeChatConnectConfig{
			Enabled:   true,
			Mode:      "mp",
			AppID:     "wx-legacy",
			AppSecret: "wx-legacy-secret",
		},
	})

	got, err := svc.GetWeChatConnectOAuthConfig(context.Background())
	require.NoError(t, err)
	require.False(t, got.OpenEnabled)
	require.True(t, got.MPEnabled)
	require.Equal(t, "mp", got.Mode)
	require.Equal(t, "snsapi_userinfo", got.Scopes)
	require.Equal(t, "wx-legacy", got.AppIDForMode("mp"))
	require.Equal(t, "wx-legacy-secret", got.AppSecretForMode("mp"))
}

func TestSettingService_GetWeChatConnectOAuthConfig_DisabledWhenNotEnabledInConfig(t *testing.T) {
	svc := NewSettingService(&settingWeChatRepoStub{values: map[string]string{"wechat_connect_enabled": "true"}}, &config.Config{
		WeChat: config.WeChatConnectConfig{OpenEnabled: true, OpenAppID: "wx", OpenAppSecret: "secret"},
	})

	_, err := svc.GetWeChatConnectOAuthConfig(context.Background())
	require.Error(t, err)
	require.Equal(t, "OAUTH_DISABLED", infraerrors.Reason(err))
}

// Google / GitHub：配了 Client ID 和 Secret 就开；回调地址没配时按用户站地址推出来；数据库行不读。
func TestSettingService_GetEmailOAuthProviderConfig_EnabledByCredentialsWithDerivedRedirect(t *testing.T) {
	svc := NewSettingService(&settingWeChatRepoStub{values: map[string]string{"github_oauth_enabled": "false"}}, &config.Config{
		Server:      config.ServerConfig{FrontendURL: "https://ai.example.com/"},
		GitHubOAuth: config.EmailOAuthProviderConfig{ClientID: "gh-id", ClientSecret: "gh-secret"},
	})

	got, err := svc.GetEmailOAuthProviderConfig(context.Background(), "github")
	require.NoError(t, err)
	require.Equal(t, "gh-id", got.ClientID)
	require.Equal(t, "https://ai.example.com/api/v1/auth/oauth/github/callback", got.RedirectURL)
	require.Equal(t, "https://github.com/login/oauth/authorize", got.AuthorizeURL)
	require.Equal(t, "/auth/oauth/callback", got.FrontendRedirectURL)

	public, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, public.GitHubOAuthEnabled)
	require.False(t, public.GoogleOAuthEnabled)
}

func TestSettingService_GetEmailOAuthProviderConfig_ExplicitRedirectWins(t *testing.T) {
	svc := NewSettingService(&settingWeChatRepoStub{values: map[string]string{}}, &config.Config{
		Server:      config.ServerConfig{FrontendURL: "https://ai.example.com"},
		GoogleOAuth: config.EmailOAuthProviderConfig{ClientID: "g-id", ClientSecret: "g-secret", RedirectURL: "https://api.example.com/cb"},
	})

	got, err := svc.GetEmailOAuthProviderConfig(context.Background(), "google")
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/cb", got.RedirectURL)
}

func TestSettingService_GetEmailOAuthProviderConfig_DisabledWithoutCredentials(t *testing.T) {
	svc := NewSettingService(&settingWeChatRepoStub{values: map[string]string{
		"google_oauth_enabled":       "true",
		"google_oauth_client_id":     "db-id",
		"google_oauth_client_secret": "db-secret",
	}}, &config.Config{Server: config.ServerConfig{FrontendURL: "https://ai.example.com"}})

	_, err := svc.GetEmailOAuthProviderConfig(context.Background(), "google")
	require.Error(t, err)
	require.Equal(t, "OAUTH_DISABLED", infraerrors.Reason(err))

	public, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.False(t, public.GoogleOAuthEnabled)
}
