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

// serviceStatusSettingRepoStub 库里没有任何设置：backend 模式关。
type serviceStatusSettingRepoStub struct{}

func (serviceStatusSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}
func (serviceStatusSettingRepoStub) GetValue(context.Context, string) (string, error) { return "", nil }
func (serviceStatusSettingRepoStub) Set(context.Context, string, string) error        { return nil }
func (serviceStatusSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (serviceStatusSettingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (serviceStatusSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (serviceStatusSettingRepoStub) Delete(context.Context, string) error { return nil }

// serviceStatusMonitorRepoStub 服务状态的两个读数：矩阵里有一个上架模型。
type serviceStatusMonitorRepoStub struct{}

func serviceStatusStubCoverage() service.ChannelMonitorV2Coverage {
	end := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	return service.ChannelMonitorV2Coverage{RequestedStart: end.Add(-24 * time.Hour), RequestedEnd: end, DataThrough: end, BucketSeconds: 3600}
}

func (serviceStatusMonitorRepoStub) GetSnapshot(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Snapshot, error) {
	return &service.ChannelMonitorV2Snapshot{Coverage: serviceStatusStubCoverage()}, nil
}

func (serviceStatusMonitorRepoStub) GetMatrix(_ context.Context, _ service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Matrix, error) {
	items := make([]service.ChannelMonitorV2MatrixRow, 0, len(cfg.Roster.Models))
	for _, model := range cfg.Roster.Models {
		items = append(items, service.ChannelMonitorV2MatrixRow{Model: model})
	}
	return &service.ChannelMonitorV2Matrix{Coverage: serviceStatusStubCoverage(), Items: items}, nil
}

func (serviceStatusMonitorRepoStub) GetChannels(context.Context, service.ChannelMonitorV2Filter, service.ChannelMonitorV2Config, string) (*service.ChannelMonitorV2Channels, error) {
	return &service.ChannelMonitorV2Channels{}, nil
}

func (serviceStatusMonitorRepoStub) GetAggregationWatermark(context.Context) (*service.ChannelMonitorV2AggregationWatermark, error) {
	return &service.ChannelMonitorV2AggregationWatermark{}, nil
}

func (serviceStatusMonitorRepoStub) RecomputeRange(context.Context, time.Time, time.Time) error {
	return nil
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
// 在用户站真实的路由表上，不带登录态请求 snapshot / matrix 拿到数据；
// 渠道健康的其余读数（维度 / 按模型列表 / 错误分布 / 用户排行）已随管理站配置页一起删掉。
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

	for _, path := range []string{
		"/api/v1/channel-monitor-v2/dimensions?range=24h",
		"/api/v1/channel-monitor-v2/models?range=24h",
		"/api/v1/channel-monitor-v2/errors?range=24h",
		"/api/v1/channel-monitor-v2/users?range=24h",
	} {
		require.Equal(t, http.StatusNotFound, serve(path).Code, path)
	}
}
