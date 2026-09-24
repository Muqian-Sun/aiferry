//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestCheckSiteRole(t *testing.T) {
	cases := []struct {
		site    Site
		role    string
		allowed bool
	}{
		{SiteUser, RoleUser, true},
		{SiteUser, RoleAdmin, false},
		{SiteAdmin, RoleAdmin, true},
		{SiteAdmin, RoleUser, false},
		// 未设置站点（单测直接构造的上下文、安装向导）不做限制
		{"", RoleAdmin, true},
		{"", RoleUser, true},
	}
	for _, tc := range cases {
		err := CheckSiteRole(WithSite(context.Background(), tc.site), tc.role)
		if tc.allowed {
			require.NoError(t, err, "site=%q role=%q", tc.site, tc.role)
			continue
		}
		require.Error(t, err, "site=%q role=%q", tc.site, tc.role)
		require.Equal(t, "SITE_ROLE_FORBIDDEN", infraerrors.Reason(err))
		require.Equal(t, 403, infraerrors.Code(err))
	}
}

// TestTokenIssuanceRespectsSite 签发 token 的两个入口都要拦：只拦一个，登录流程会从另一个拿到 token。
func TestTokenIssuanceRespectsSite(t *testing.T) {
	cfg := &config.Config{}
	cfg.JWT.Secret = "test-jwt-secret-32bytes-long!!!"
	cfg.JWT.AccessTokenExpireMinutes = 60
	svc := NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil)
	admin := &User{ID: 1, Role: RoleAdmin, Status: StatusActive}
	user := &User{ID: 2, Role: RoleUser, Status: StatusActive}
	userSite := WithSite(context.Background(), SiteUser)
	adminSite := WithSite(context.Background(), SiteAdmin)

	_, err := svc.GenerateToken(userSite, admin)
	require.Equal(t, "SITE_ROLE_FORBIDDEN", infraerrors.Reason(err))
	_, err = svc.GenerateToken(adminSite, user)
	require.Equal(t, "SITE_ROLE_FORBIDDEN", infraerrors.Reason(err))

	// refresh token 缓存未配置时本应报别的错，站点校验必须先于它生效。
	_, err = svc.GenerateTokenPair(userSite, admin, "")
	require.Equal(t, "SITE_ROLE_FORBIDDEN", infraerrors.Reason(err))
	_, err = svc.GenerateTokenPair(adminSite, user, "")
	require.Equal(t, "SITE_ROLE_FORBIDDEN", infraerrors.Reason(err))

	token, err := svc.GenerateToken(adminSite, admin)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	token, err = svc.GenerateToken(userSite, user)
	require.NoError(t, err)
	require.NotEmpty(t, token)
}
