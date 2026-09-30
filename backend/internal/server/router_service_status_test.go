//go:build unit

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// serviceStatusSettingRepoStub 库里没有任何设置：backend 模式关、面板限流关、渠道监控按代码默认开。
type serviceStatusSettingRepoStub struct{}

func (serviceStatusSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}
func (serviceStatusSettingRepoStub) GetValue(context.Context, string) (string, error) { return "", nil }
func (serviceStatusSettingRepoStub) Set(context.Context, string, string) error      { return nil }
func (serviceStatusSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (serviceStatusSettingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (serviceStatusSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (serviceStatusSettingRepoStub) Delete(context.Context, string) error { return nil }

// serviceStatusMonitorRepoStub 只实现服务状态页用到的两个读数；其余方法被调到会直接 panic。
type serviceStatusMonitorRepoStub struct {
	service.ChannelMonitorV2Repository
}

func (serviceStatusMonitorRepoStub) GetConfig(context.Context) (*service.ChannelMonitorV2Config, error) {
	return &service.ChannelMonitorV2Config{Enabled: true, RefreshIntervalSeconds: 60, HealthThresholds: service.DefaultChannelMonitorV2HealthThresholds()}, nil
}

func serviceStatusStubCoverage() service.ChannelMonitorV2Coverage {
	end := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	return service.ChannelMonitorV2Coverage{RequestedStart: end.Add(-24 * time.Hour), RequestedEnd: end, DataThrough: end, BucketSeconds: 3600}
}

func (serviceStatusMonitorRepoStub) GetSnapshot(_ context.Context, _ service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config, _ bool) (*service.ChannelMonitorV2Snapshot, error) {
	return &service.ChannelMonitorV2Snapshot{Config: cfg, Coverage: serviceStatusStubCoverage()}, nil
}

func (serviceStatusMonitorRepoStub) GetMatrix(_ context.Context, _ service.ChannelMonitorV2Filter, _ service.ChannelMonitorV2Config, groupBy service.ChannelMonitorV2GroupBy, _ bool) (*service.ChannelMonitorV2Matrix, error) {
	return &service.ChannelMonitorV2Matrix{
		GroupBy: groupBy, Coverage: serviceStatusStubCoverage(),
		Items: []service.ChannelMonitorV2MatrixRow{{Platform: "stubplatform", Model: "stub-model-a"}},
	}, nil
}

// serviceStatusCatalogStub 上架目录只有 stub-model-a。
type serviceStatusCatalogStub struct{}

func (serviceStatusCatalogStub) ListListedEntries(context.Context) []service.ModelCatalogEntry {
	return []service.ModelCatalogEntry{{ID: 1, ModelID: "stub-model-a", Status: service.ModelCatalogStatusListed}}
}

func (serviceStatusCatalogStub) ResolveRoute(_ context.Context, model string) (service.CatalogRoute, bool) {
	if model != "stub-model-a" {
		return service.CatalogRoute{}, false
	}
	return service.CatalogRoute{EntryID: 1, CanonicalModel: model, RequestedModel: model}, true
}

// TestUserSiteServiceStatusIsPublic 服务状态页对未登录访客公开（muqian 2026-09-30）：
// 在用户站真实的路由表上，不带登录态请求 snapshot / matrix 拿到数据；渠道监控的其余读数仍要登录。
func TestUserSiteServiceStatusIsPublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.Handlers{}
	allocNilPointers(reflect.ValueOf(h).Elem())
	h.ChannelMonitorV2 = handler.NewChannelMonitorV2Handler(service.NewChannelMonitorV2Service(serviceStatusMonitorRepoStub{}, serviceStatusCatalogStub{}))

	next := func(c *gin.Context) { c.Next() }
	unauthorized := func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }
	deps := routeDeps{
		handlers:        h,
		jwtAuth:         middleware2.JWTAuthMiddleware(unauthorized),
		optionalJWTAuth: middleware2.OptionalJWTAuthMiddleware(next),
		adminAuth:       middleware2.AdminAuthMiddleware(unauthorized),
		apiKeyAuth:      middleware2.APIKeyAuthMiddleware(unauthorized),
		auditLog:        middleware2.AuditLogMiddleware(next),
		stepUpAuth:      middleware2.StepUpAuthMiddleware(unauthorized),
		settingService:  service.NewSettingService(serviceStatusSettingRepoStub{}, &config.Config{}),
		cfg:             &config.Config{},
	}
	engine := setupSiteRouter(gin.New(), service.SiteUser, deps, func() []string { return nil }, nil)

	// 回环地址：公开接口的按 IP 限流对它不计数（测试里没有 Redis）
	serve := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:40000"
		engine.ServeHTTP(rec, req)
		return rec
	}

	snapshot := serve("/api/v1/channel-monitor-v2/snapshot?range=24h")
	require.Equal(t, http.StatusOK, snapshot.Code, snapshot.Body.String())
	require.Contains(t, snapshot.Body.String(), `"refresh_interval_seconds":60`)

	matrix := serve("/api/v1/channel-monitor-v2/matrix?range=24h")
	require.Equal(t, http.StatusOK, matrix.Code, matrix.Body.String())
	require.Contains(t, matrix.Body.String(), "stub-model-a")
	require.NotContains(t, matrix.Body.String(), "stubplatform", "访客只拿白名单字段，没有上游平台")

	for _, path := range []string{
		"/api/v1/channel-monitor-v2/models?range=24h",
		"/api/v1/channel-monitor-v2/errors?range=24h",
		"/api/v1/channel-monitor-v2/users?range=24h",
	} {
		require.Equal(t, http.StatusUnauthorized, serve(path).Code, path)
	}
}
