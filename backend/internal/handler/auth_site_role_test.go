//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRespondWithTokenPairRejectsWrongSite 签发失败会回退到只发 access token。
// 站点角色不符必须在回退之前拒绝，且以 403 返回而不是被吞成 500。
// refresh token 缓存留空，正是会触发回退的配置。
func TestRespondWithTokenPairRejectsWrongSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.JWT.Secret = "test-jwt-secret-32bytes-long!!!"
	cfg.JWT.AccessTokenExpireMinutes = 60
	authSvc := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil)

	cases := []struct {
		site service.Site
		user *service.User
	}{
		{service.SiteUser, &service.User{ID: 1, Role: service.RoleAdmin, Status: service.StatusActive}},
		{service.SiteAdmin, &service.User{ID: 2, Role: service.RoleUser, Status: service.StatusActive}},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		c.Request = req.WithContext(service.WithSite(req.Context(), tc.site))

		respondWithTokenPair(c, authSvc, tc.user)

		require.Equal(t, http.StatusForbidden, w.Code, "site=%s role=%s body=%s", tc.site, tc.user.Role, w.Body.String())
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, "SITE_ROLE_FORBIDDEN", body["reason"])
		require.NotContains(t, w.Body.String(), "access_token")
	}
}
