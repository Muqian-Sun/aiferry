//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type bmSettingRepo struct {
	values map[string]string
}

func (r *bmSettingRepo) Get(_ context.Context, _ string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (r *bmSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	v, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return v, nil
}

func (r *bmSettingRepo) Set(_ context.Context, _, _ string) error {
	panic("unexpected Set call")
}

func (r *bmSettingRepo) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (r *bmSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	if r.values == nil {
		r.values = make(map[string]string, len(settings))
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *bmSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (r *bmSettingRepo) Delete(_ context.Context, _ string) error {
	panic("unexpected Delete call")
}

// 「只留管理站」模式由代码决定（service.BackendModeEnabled = false）：库里残留旧开关开着也不生效，守卫一律放行。
func newBackendModeSettingService(t *testing.T) *service.SettingService {
	t.Helper()

	repo := &bmSettingRepo{
		values: map[string]string{
			"backend_mode_enabled": "true", // 旧后台开关留下的行
		},
	}
	return service.NewSettingService(repo, &config.Config{})
}

func TestBackendModeGuards_StaleSettingRowDoesNotEnable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, withService := range []bool{true, false} {
		var svc *service.SettingService
		if withService {
			svc = newBackendModeSettingService(t)
		}

		userRouter := gin.New()
		userRouter.Use(func(c *gin.Context) {
			c.Set(string(ContextKeyUserRole), "user")
			c.Next()
		})
		userRouter.Use(BackendModeUserGuard(svc))
		userRouter.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		w := httptest.NewRecorder()
		userRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
		require.Equal(t, http.StatusOK, w.Code)

		authRouter := gin.New()
		authRouter.Use(BackendModeAuthGuard(svc))
		authRouter.Any("/*path", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		w = httptest.NewRecorder()
		authRouter.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/register", nil))
		require.Equal(t, http.StatusOK, w.Code)
	}
}

// 模式打开时哪些认证路径放行：模式现在由代码常量关着，这里直接测路径判断，改常量打开时规则仍有保障。
func TestBackendModeAllowsAuthPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		allowed bool
	}{
		{name: "allows_login", path: "/api/v1/auth/login", allowed: true},
		{name: "allows_login_2fa", path: "/api/v1/auth/login/2fa", allowed: true},
		{name: "allows_logout", path: "/api/v1/auth/logout", allowed: true},
		{name: "allows_refresh", path: "/api/v1/auth/refresh", allowed: true},
		{name: "blocks_linuxdo_oauth_start", path: "/api/v1/auth/oauth/linuxdo/start", allowed: false},
		{name: "allows_linuxdo_oauth_callback", path: "/api/v1/auth/oauth/linuxdo/callback", allowed: true},
		{name: "blocks_wechat_oauth_start", path: "/api/v1/auth/oauth/wechat/start", allowed: false},
		{name: "allows_wechat_oauth_callback", path: "/api/v1/auth/oauth/wechat/callback", allowed: true},
		{name: "blocks_wechat_payment_oauth_start", path: "/api/v1/auth/oauth/wechat/payment/start", allowed: false},
		{name: "allows_wechat_payment_oauth_callback", path: "/api/v1/auth/oauth/wechat/payment/callback", allowed: true},
		{name: "blocks_oidc_oauth_start", path: "/api/v1/auth/oauth/oidc/start", allowed: false},
		{name: "allows_oidc_oauth_callback", path: "/api/v1/auth/oauth/oidc/callback", allowed: true},
		{name: "blocks_github_oauth_start", path: "/api/v1/auth/oauth/github/start", allowed: false},
		{name: "allows_github_oauth_callback", path: "/api/v1/auth/oauth/github/callback", allowed: true},
		{name: "allows_github_complete_registration", path: "/api/v1/auth/oauth/github/complete-registration", allowed: true},
		{name: "blocks_google_oauth_start", path: "/api/v1/auth/oauth/google/start", allowed: false},
		{name: "allows_google_oauth_callback", path: "/api/v1/auth/oauth/google/callback", allowed: true},
		{name: "allows_google_complete_registration", path: "/api/v1/auth/oauth/google/complete-registration", allowed: true},
		{name: "blocks_dingtalk_oauth_start", path: "/api/v1/auth/oauth/dingtalk/start", allowed: false},
		{name: "allows_dingtalk_oauth_callback", path: "/api/v1/auth/oauth/dingtalk/callback", allowed: true},
		{name: "allows_dingtalk_complete_registration", path: "/api/v1/auth/oauth/dingtalk/complete-registration", allowed: true},
		{name: "allows_dingtalk_create_account", path: "/api/v1/auth/oauth/dingtalk/create-account", allowed: true},
		{name: "allows_dingtalk_bind_login", path: "/api/v1/auth/oauth/dingtalk/bind-login", allowed: true},
		{name: "allows_oauth_pending_exchange", path: "/api/v1/auth/oauth/pending/exchange", allowed: true},
		{name: "allows_oauth_pending_send_verify_code", path: "/api/v1/auth/oauth/pending/send-verify-code", allowed: true},
		{name: "allows_oauth_pending_create_account", path: "/api/v1/auth/oauth/pending/create-account", allowed: true},
		{name: "allows_oauth_pending_bind_login", path: "/api/v1/auth/oauth/pending/bind-login", allowed: true},
		{name: "allows_provider_bind_login", path: "/api/v1/auth/oauth/oidc/bind-login", allowed: true},
		{name: "allows_provider_create_account", path: "/api/v1/auth/oauth/wechat/create-account", allowed: true},
		{name: "allows_legacy_complete_registration", path: "/api/v1/auth/oauth/linuxdo/complete-registration", allowed: true},
		{name: "blocks_register", path: "/api/v1/auth/register", allowed: false},
		{name: "blocks_forgot_password", path: "/api/v1/auth/forgot-password", allowed: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.allowed, backendModeAllowsAuthPath(tc.path))
		})
	}
}
