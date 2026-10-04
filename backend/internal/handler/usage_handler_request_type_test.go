package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userUsageRepoCapture struct {
	service.UsageLogRepository
	listParams   pagination.PaginationParams
	listFilters  usagestats.UsageLogFilters
	statsFilters usagestats.UsageLogFilters
	trendFilters usagestats.UsageLogFilters
	listRows     []service.UsageLog
	stats        *usagestats.UsageStats
	modelStats   []usagestats.ModelStat
	// 按模型趋势：记录是否调用与入参
	modelTrendCalled      bool
	modelTrendGranularity string
	modelTrendFilters     usagestats.UsageLogFilters
	modelTrend            []usagestats.ModelTrendPoint
	trend                 []usagestats.TrendDataPoint
}

func (s *userUsageRepoCapture) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	s.listParams = params
	s.listFilters = filters
	return s.listRows, &pagination.PaginationResult{
		Total:    int64(len(s.listRows)),
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    0,
	}, nil
}

func (s *userUsageRepoCapture) GetStatsWithFilters(ctx context.Context, filters usagestats.UsageLogFilters) (*usagestats.UsageStats, error) {
	s.statsFilters = filters
	if s.stats != nil {
		return s.stats, nil
	}
	return &usagestats.UsageStats{}, nil
}

func (s *userUsageRepoCapture) GetUsageTrendWithFilters(ctx context.Context, startTime, endTime time.Time, granularity string, userID, apiKeyID, accountID int64, model string, requestType *int16, stream *bool, billingType *int8) ([]usagestats.TrendDataPoint, error) {
	s.trendFilters = usagestats.UsageLogFilters{
		UserID:      userID,
		APIKeyID:    apiKeyID,
		AccountID:   accountID,
		Model:       model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: billingType,
	}
	if s.trend != nil {
		return s.trend, nil
	}
	return []usagestats.TrendDataPoint{}, nil
}

func (s *userUsageRepoCapture) GetModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID int64, requestType *int16, stream *bool, billingType *int8) ([]usagestats.ModelStat, error) {
	return s.modelStats, nil
}

func (s *userUsageRepoCapture) GetModelUsageTrendWithUsageFilters(ctx context.Context, startTime, endTime time.Time, granularity string, filters usagestats.UsageLogFilters) ([]usagestats.ModelTrendPoint, error) {
	s.modelTrendCalled = true
	s.modelTrendGranularity = granularity
	s.modelTrendFilters = filters
	return s.modelTrend, nil
}

func newUserUsageRequestTypeTestRouter(repo *userUsageRepoCapture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage", handler.List)
	router.GET("/usage/stats", handler.Stats)
	router.GET("/usage/dashboard/trend", handler.DashboardTrend)
	router.GET("/usage/dashboard/models", handler.DashboardModels)
	router.GET("/usage/dashboard/snapshot-v2", handler.DashboardSnapshotV2)
	return router
}

func TestUserUsageListRequestTypePriority(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?request_type=ws_v2&stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.listFilters.UserID)
	require.NotNil(t, repo.listFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeWSV2), *repo.listFilters.RequestType)
	require.Nil(t, repo.listFilters.Stream)
}

func TestUserUsageListInvalidRequestType(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?request_type=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageListInvalidStream(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?stream=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageListNativeCompactionFilter(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?request_type=stream&native_compaction_v2=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.listFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeStream), *repo.listFilters.RequestType)
	require.NotNil(t, repo.listFilters.NativeCompactionV2)
	require.True(t, *repo.listFilters.NativeCompactionV2)
}

func TestUserUsageListInvalidNativeCompactionFilter(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?native_compaction_v2=unknown", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageListAdvancedFilters(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?model=gpt-5&billing_type=1&billing_mode=image&start_date=2026-03-01&end_date=2026-03-02", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.listFilters.UserID)
	require.Equal(t, "gpt-5", repo.listFilters.Model)
	require.Equal(t, usagestats.ModelSourceRequested, repo.listFilters.ModelFilterSource)
	require.NotNil(t, repo.listFilters.BillingType)
	require.Equal(t, int8(1), *repo.listFilters.BillingType)
	require.Equal(t, "image", repo.listFilters.BillingMode)
	require.NotNil(t, repo.listFilters.StartTime)
	require.NotNil(t, repo.listFilters.EndTime)
}

func TestUserUsageListInvalidBillingMode(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?billing_mode=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageListAllowsVideoBillingMode(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?billing_mode=video", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "video", repo.listFilters.BillingMode)
}

func TestUserUsageListKeepsUserBillingAndIPWithoutAdminCostFields(t *testing.T) {
	ipAddress := "203.0.113.10"
	upstreamModel := "upstream-private-model"
	repo := &userUsageRepoCapture{
		listRows: []service.UsageLog{{
			ID:                 1,
			UserID:             42,
			APIKeyID:           7,
			AccountID:          5,
			RequestID:          "req_user_billing",
			Model:              "gpt-5",
			NativeCompactionV2: true,
			InputCost:          0.01,
			OutputCost:         0.02,
			CacheCreationCost:  0.03,
			CacheReadCost:      0.04,
			TotalCost:          0.10,
			ActualCost:         0.08,
			RateMultiplier:     0.8,
			IPAddress:          &ipAddress,
			UpstreamModel:      &upstreamModel,
			AccountCost:        0.05,
		}},
	}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// 用户站只给实付：按官方价的分项费用、标准计费与倍率不出接口（2026-10-04 D1）
	require.Contains(t, body, `"actual_cost":0.08`)
	require.NotContains(t, body, "input_cost")
	require.NotContains(t, body, "total_cost")
	require.NotContains(t, body, "rate_multiplier")
	require.Contains(t, body, `"native_compaction_v2":true`)
	require.Contains(t, body, `"ip_address":"203.0.113.10"`)
	require.NotContains(t, body, "upstream_endpoint")
	require.NotContains(t, body, "account_cost")
	require.NotContains(t, body, "upstream_model")
	require.NotContains(t, body, "upstream_response_model")
	require.NotContains(t, body, "upstream_model_mismatch")
	require.NotContains(t, body, `"account":`)
}

func TestUserUsageStatsUsesScopedFilters(t *testing.T) {
	accountCost := 0.12
	repo := &userUsageRepoCapture{
		stats: &usagestats.UsageStats{
			TotalCost:        0.10,
			TotalActualCost:  0.08,
			TotalAccountCost: &accountCost,
			UpstreamEndpoints: []usagestats.EndpointStat{{
				Endpoint: "/v1/responses",
			}},
			EndpointPaths: []usagestats.EndpointStat{{
				Endpoint: "/v1/chat/completions -> /v1/responses",
			}},
		},
	}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage/stats?request_type=sync&billing_mode=token&start_date=2026-03-01&end_date=2026-03-02", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.statsFilters.UserID)
	require.Equal(t, usagestats.ModelSourceRequested, repo.statsFilters.ModelFilterSource)
	require.NotNil(t, repo.statsFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeSync), *repo.statsFilters.RequestType)
	require.Equal(t, "token", repo.statsFilters.BillingMode)
	require.NotContains(t, rec.Body.String(), `"total_cost"`, "标准计费按官方价算，用户站不给（D1）")
	require.Contains(t, rec.Body.String(), `"total_actual_cost":0.08`)
	require.NotContains(t, rec.Body.String(), "total_account_cost")
	require.NotContains(t, rec.Body.String(), "upstream_endpoints")
	require.NotContains(t, rec.Body.String(), "endpoint_paths")
}

func TestUserUsageDashboardModelsOmitsAccountCost(t *testing.T) {
	repo := &userUsageRepoCapture{
		modelStats: []usagestats.ModelStat{{
			Model:       "gpt-5",
			Requests:    2,
			TotalTokens: 30,
			Cost:        0.10,
			ActualCost:  0.08,
			AccountCost: 0.07,
		}},
	}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/models?start_date=2026-03-01&end_date=2026-03-02", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.NotContains(t, body, `"cost":`, "标价按官方价算，用户站不给（D1）")
	require.Contains(t, body, `"actual_cost":0.08`)
	require.NotContains(t, body, "account_cost")
}

// 趋势点在仓储层带了渠道成本（管理站概览算利润用），用户站的两个趋势接口都不能带出去。
func TestUserUsageTrendOmitsAccountCost(t *testing.T) {
	repo := &userUsageRepoCapture{
		trend: []usagestats.TrendDataPoint{{
			Date:        "2026-03-01",
			Requests:    2,
			TotalTokens: 30,
			Cost:        0.10,
			ActualCost:  0.08,
			AccountCost: 0.07,
		}},
	}
	router := newUserUsageRequestTypeTestRouter(repo)

	for _, path := range []string{
		"/usage/dashboard/trend?start_date=2026-03-01&end_date=2026-03-02",
		"/usage/dashboard/snapshot-v2?include_model_stats=false&start_date=2026-03-01&end_date=2026-03-02",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, path)
		body := rec.Body.String()
		require.Contains(t, body, `"actual_cost":0.08`, path)
		require.NotContains(t, body, "account_cost", path)
	}
}

func TestUserUsageDashboardModelsRejectsAdminModelSources(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/models?model_source=upstream&start_date=2026-03-01&end_date=2026-03-02", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageSnapshotUsesScopedFilters(t *testing.T) {
	repo := &userUsageRepoCapture{
		modelStats: []usagestats.ModelStat{{Model: "gpt-5", AccountCost: 0.07}},
	}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/snapshot-v2?include_trend=true&include_model_stats=true&request_type=stream&start_date=2026-03-01&end_date=2026-03-02", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.trendFilters.UserID)
	require.NotNil(t, repo.trendFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeStream), *repo.trendFilters.RequestType)
	require.NotContains(t, rec.Body.String(), "account_cost")
}

func TestUserUsageSnapshotRejectsInvalidIncludeFlags(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	for _, query := range []string{
		"include_trend=bad",
		"include_model_stats=bad",
		"include_model_trend=bad",
	} {
		req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/snapshot-v2?start_date=2026-03-01&end_date=2026-03-02&"+query, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code, query)
	}
}

func TestUserUsageSnapshotModelTrendIsOptInAndScoped(t *testing.T) {
	repo := &userUsageRepoCapture{
		modelTrend: []usagestats.ModelTrendPoint{{Date: "2026-03-01", Model: "gpt-5.6", Requests: 3, TotalTokens: 120}},
	}
	router := newUserUsageRequestTypeTestRouter(repo)

	// 不传 include_model_trend：不查、不返回
	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/snapshot-v2?include_trend=false&include_model_stats=false&start_date=2026-03-01&end_date=2026-03-02", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, repo.modelTrendCalled)
	require.NotContains(t, rec.Body.String(), "model_trend")

	req = httptest.NewRequest(http.MethodGet, "/usage/dashboard/snapshot-v2?include_trend=false&include_model_stats=false&include_model_trend=true&granularity=day&start_date=2026-03-01&end_date=2026-03-02", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.modelTrendCalled)
	require.Equal(t, "day", repo.modelTrendGranularity)
	require.Equal(t, int64(42), repo.modelTrendFilters.UserID)

	var body struct {
		Data struct {
			ModelTrend json.RawMessage `json:"model_trend"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.JSONEq(t, `[{"date":"2026-03-01","model":"gpt-5.6","requests":3,"total_tokens":120}]`, string(body.Data.ModelTrend))
}
