//go:build unit

package server

import (
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

// 数据库备份与数据管理已下线：生图对象存储的三条接口从 /admin/backups/image-storage*
// 挪到 /admin/image-storage，/admin/backups* 与 /admin/data-management* 不再注册。
func TestAdminSiteImageStorageRoutesMovedOutOfBackups(t *testing.T) {
	routes := siteRouteSet(t, service.SiteAdmin)
	for _, must := range []string{
		"GET /api/v1/admin/image-storage",
		"PUT /api/v1/admin/image-storage",
		"POST /api/v1/admin/image-storage/test",
	} {
		require.Contains(t, routes, must)
	}
	for _, route := range sortedKeys(routes) {
		require.NotRegexp(t, `^[A-Z]+ /api/v1/admin/(backups|data-management)(/|$)`, route)
	}
}

// 改写对象存储目标可把生成内容导向外部账号，PUT 仍要求 step-up；读取不要求。
func TestAdminImageStorageUpdateRequiresStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.Handlers{}
	allocNilPointers(reflect.ValueOf(h).Elem())
	h.Admin.ImageStorage = admin.NewImageStorageHandler(
		service.NewImageStorageSettingService(emptySettingRepo{}, nil, true, nil, config.ImageStorageConfig{}),
	)

	next := func(c *gin.Context) { c.Next() }
	requireStepUp := func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "STEP_UP_REQUIRED"})
	}
	cfg := &config.Config{}
	deps := routeDeps{
		handlers:        h,
		jwtAuth:         middleware2.JWTAuthMiddleware(next),
		optionalJWTAuth: middleware2.OptionalJWTAuthMiddleware(next),
		adminAuth:       middleware2.AdminAuthMiddleware(next),
		apiKeyAuth:      middleware2.APIKeyAuthMiddleware(next),
		auditLog:        middleware2.AuditLogMiddleware(next),
		stepUpAuth:      middleware2.StepUpAuthMiddleware(requireStepUp),
		cfg:             cfg,
	}
	engine := setupSiteRouter(gin.New(), service.SiteAdmin, deps, func() []string { return nil }, nil)

	put := httptest.NewRequest(http.MethodPut, "/api/v1/admin/image-storage", strings.NewReader(`{"enabled":false}`))
	put.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, put)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "STEP_UP_REQUIRED")

	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/image-storage", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"secret_configured":false`)
}
