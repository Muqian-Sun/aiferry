//go:build unit

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestJWTAuthRejectsTokenFromOtherSite token 本身不绑定站点，
// 管理员在管理站拿到的 token 不能拿去访问用户站，反之亦然。
func TestJWTAuthRejectsTokenFromOtherSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.JWT.Secret = "test-jwt-secret-32bytes-long!!!"
	cfg.JWT.AccessTokenExpireMinutes = 60
	admin := &service.User{ID: 1, Role: service.RoleAdmin, Status: service.StatusActive, TokenVersion: 1}
	user := &service.User{ID: 2, Role: service.RoleUser, Status: service.StatusActive, TokenVersion: 1}
	repo := &stubJWTUserRepo{users: map[int64]*service.User{1: admin, 2: user}}
	authSvc := service.NewAuthService(nil, repo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil)
	mw := NewJWTAuthMiddleware(authSvc, service.NewUserService(repo, nil, nil, nil), nil, nil)

	engine := func(site service.Site) *gin.Engine {
		r := gin.New()
		r.Use(SiteContext(site))
		r.Use(gin.HandlerFunc(mw))
		r.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })
		return r
	}
	// 签发时不带站点，模拟「在另一个站点合法拿到的 token」。
	tokenOf := func(u *service.User) string {
		token, err := authSvc.GenerateToken(context.Background(), u)
		require.NoError(t, err)
		return token
	}
	call := func(site service.Site, u *service.User) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tokenOf(u))
		engine(site).ServeHTTP(w, req)
		return w
	}

	require.Equal(t, http.StatusOK, call(service.SiteUser, user).Code)
	require.Equal(t, http.StatusOK, call(service.SiteAdmin, admin).Code)
	for _, tc := range []struct {
		site service.Site
		u    *service.User
	}{{service.SiteUser, admin}, {service.SiteAdmin, user}} {
		w := call(tc.site, tc.u)
		require.Equal(t, http.StatusForbidden, w.Code, "site=%s role=%s", tc.site, tc.u.Role)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, "SITE_ROLE_FORBIDDEN", body["code"])
	}
}
