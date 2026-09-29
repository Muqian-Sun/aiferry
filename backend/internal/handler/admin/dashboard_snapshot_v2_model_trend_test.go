package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// snapshotModelTrendRepo 只实现按模型趋势，记下调用次数、粒度与过滤条件。
type snapshotModelTrendRepo struct {
	service.UsageLogRepository
	calls       int
	granularity string
	filters     usagestats.UsageLogFilters
}

func (r *snapshotModelTrendRepo) GetModelUsageTrendWithUsageFilters(
	_ context.Context,
	_, _ time.Time,
	granularity string,
	filters usagestats.UsageLogFilters,
) ([]usagestats.ModelTrendPoint, error) {
	r.calls++
	r.granularity = granularity
	r.filters = filters
	return []usagestats.ModelTrendPoint{{Date: "2026-09-28", Model: "claude-sonnet-4-6", Requests: 3, TotalTokens: 300}}, nil
}

// snapshot-v2 的 include_model_trend：默认不查不回；打开后按粒度与过滤条件查按模型趋势，且开关进缓存键。
func TestDashboardSnapshotV2_IncludeModelTrend(t *testing.T) {
	t.Cleanup(resetDashboardReadCachesForTest)
	resetDashboardReadCachesForTest()

	gin.SetMode(gin.TestMode)
	repo := &snapshotModelTrendRepo{}
	handler := NewDashboardHandler(service.NewDashboardService(repo, nil, nil, nil), nil)
	router := gin.New()
	router.GET("/admin/dashboard/snapshot-v2", handler.GetSnapshotV2)
	get := func(query string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/dashboard/snapshot-v2?"+query, nil))
		return rec
	}
	const base = "start_date=2026-09-22&end_date=2026-09-28&granularity=hour&include_stats=false&include_trend=false&include_model_stats=false&user_id=7&account_id=9"

	rec := get(base)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "model_trend")
	require.Zero(t, repo.calls, "不带开关不应查按模型趋势")

	rec = get(base + "&include_model_trend=true")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"model_trend":[{"date":"2026-09-28","model":"claude-sonnet-4-6","requests":3,"total_tokens":300}]`)
	require.Equal(t, 1, repo.calls, "开关应进缓存键，不能命中上一次不带开关的缓存")
	require.Equal(t, "hour", repo.granularity)
	require.Equal(t, int64(7), repo.filters.UserID)
	require.Equal(t, int64(9), repo.filters.AccountID)
}
