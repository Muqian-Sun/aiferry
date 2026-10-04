package admin

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// opsDashboardScopeRepo 记下看板查询收到的筛选条件。
type opsDashboardScopeRepo struct {
	service.OpsRepository
	overviewFilters []service.OpsDashboardFilter
}

func (r *opsDashboardScopeRepo) GetDashboardOverview(_ context.Context, filter *service.OpsDashboardFilter) (*service.OpsDashboardOverview, error) {
	r.overviewFilters = append(r.overviewFilters, *filter)
	return &service.OpsDashboardOverview{StartTime: filter.StartTime, EndTime: filter.EndTime}, nil
}

func (r *opsDashboardScopeRepo) GetThroughputTrend(_ context.Context, _ *service.OpsDashboardFilter, bucketSeconds int) (*service.OpsThroughputTrendResponse, error) {
	return &service.OpsThroughputTrendResponse{Bucket: "1m"}, nil
}

func (r *opsDashboardScopeRepo) GetErrorTrend(_ context.Context, _ *service.OpsDashboardFilter, bucketSeconds int) (*service.OpsErrorTrendResponse, error) {
	return &service.OpsErrorTrendResponse{Bucket: "1m"}, nil
}

func (r *opsDashboardScopeRepo) GetLatestSystemMetrics(context.Context, int) (*service.OpsSystemMetricsSnapshot, error) {
	return nil, sql.ErrNoRows
}

func (r *opsDashboardScopeRepo) ListJobHeartbeats(context.Context) ([]*service.OpsJobHeartbeat, error) {
	return nil, nil
}

// 运维看板按模型、渠道筛选（2026-10-04）：参数传到查询；快照缓存按筛选条件分开，换了模型 / 渠道不会拿到别的筛选的结果。
func TestOpsDashboardSnapshotV2_ModelAndAccountScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &opsDashboardScopeRepo{}
	h := NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	r := gin.New()
	r.GET("/snapshot", h.GetDashboardSnapshotV2)
	// 固定时间范围：缓存键只差筛选条件
	const window = "start_time=2026-10-04T01:00:00Z&end_time=2026-10-04T02:00:00Z"
	get := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/snapshot?"+window+query, nil))
		return w
	}

	first := get("&model=gpt-5.5&account_id=7")
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Len(t, repo.overviewFilters, 1)
	require.Equal(t, "gpt-5.5", repo.overviewFilters[0].Model)
	require.Equal(t, int64(7), repo.overviewFilters[0].AccountID)

	other := get("&model=claude-sonnet-4-6&account_id=7")
	require.Equal(t, http.StatusOK, other.Code)
	require.Equal(t, "miss", other.Header().Get("X-Snapshot-Cache"), "换了模型不能命中别的模型的缓存")
	require.Len(t, repo.overviewFilters, 2)
	require.Equal(t, "claude-sonnet-4-6", repo.overviewFilters[1].Model)

	otherAccount := get("&model=gpt-5.5&account_id=8")
	require.Equal(t, "miss", otherAccount.Header().Get("X-Snapshot-Cache"), "换了渠道不能命中别的渠道的缓存")

	again := get("&model=gpt-5.5&account_id=7")
	require.Equal(t, "hit", again.Header().Get("X-Snapshot-Cache"), "同样的筛选照常走缓存")

	bad := get("&account_id=abc")
	require.Equal(t, http.StatusBadRequest, bad.Code)
}
