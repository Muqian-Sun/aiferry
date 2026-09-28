//go:build unit

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// emptySettingRepo 模拟一个从未写过任何设置的库：所有键都不存在。
type emptySettingRepo struct{}

func (emptySettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (emptySettingRepo) GetValue(context.Context, string) (string, error) {
	return "", service.ErrSettingNotFound
}

func (emptySettingRepo) Set(context.Context, string, string) error { return nil }

func (emptySettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (emptySettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }

func (emptySettingRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (emptySettingRepo) Delete(context.Context, string) error { return nil }

// 管理员合规承诺已下线：库里没有任何确认记录的管理员，经完整管理站路由访问管理接口
// 直接放行（以前会被合规守卫以 423 拦下），确认接口本身也不再注册。
func TestAdminSiteDoesNotRequireComplianceAcknowledgement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.Handlers{}
	allocNilPointers(reflect.ValueOf(h).Elem())
	h.Admin.System = admin.NewSystemHandler("1.0.0")

	next := func(c *gin.Context) { c.Next() }
	asAdmin := func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Set(string(middleware2.ContextKeyUserRole), service.RoleAdmin)
		c.Next()
	}
	cfg := &config.Config{}
	deps := routeDeps{
		handlers:        h,
		jwtAuth:         middleware2.JWTAuthMiddleware(next),
		optionalJWTAuth: middleware2.OptionalJWTAuthMiddleware(next),
		adminAuth:       middleware2.AdminAuthMiddleware(asAdmin),
		apiKeyAuth:      middleware2.APIKeyAuthMiddleware(next),
		auditLog:        middleware2.AuditLogMiddleware(next),
		stepUpAuth:      middleware2.StepUpAuthMiddleware(next),
		settingService:  service.NewSettingService(emptySettingRepo{}, cfg),
		cfg:             cfg,
	}
	engine := setupSiteRouter(gin.New(), service.SiteAdmin, deps, func() []string { return nil }, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/version", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"version":"1.0.0"`)

	for _, r := range engine.Routes() {
		require.False(t, strings.HasPrefix(r.Path, "/api/v1/admin/compliance"), "unexpected route %s %s", r.Method, r.Path)
	}
}
