package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 「近 24 小时」：明细列表与区间统计都按精确时刻查，不再展开成两个自然日（2026-10-04 走查：列表多出前一天上午的记录）
func TestAdminUsageListAndStatsUsePreciseTimeRange(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)
	wantStart := time.Date(2026, 10, 3, 2, 45, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 10, 4, 2, 45, 0, 0, time.UTC)
	const rangeQuery = "start_time=2026-10-03T02:45:00Z&end_time=2026-10-04T02:45:00Z&timezone=Asia/Shanghai"

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/usage?"+rangeQuery, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.listFilters.StartTime)
	require.NotNil(t, repo.listFilters.EndTime)
	require.True(t, repo.listFilters.StartTime.Equal(wantStart), "list start=%s", repo.listFilters.StartTime)
	require.True(t, repo.listFilters.EndTime.Equal(wantEnd), "list end=%s", repo.listFilters.EndTime)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/usage/stats?nocache=1&"+rangeQuery, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.statsFilters.StartTime)
	require.NotNil(t, repo.statsFilters.EndTime)
	require.True(t, repo.statsFilters.StartTime.Equal(wantStart), "stats start=%s", repo.statsFilters.StartTime)
	require.True(t, repo.statsFilters.EndTime.Equal(wantEnd), "stats end=%s", repo.statsFilters.EndTime)
}

// 起止颠倒直接 400：原来列表回 0 条、统计回「平均响应 0ms」，看起来像这段时间没有请求
func TestAdminUsageRejectsReversedRange(t *testing.T) {
	for _, path := range []string{
		"/admin/usage?start_date=2026-10-05&end_date=2026-10-03",
		"/admin/usage/stats?start_date=2026-10-05&end_date=2026-10-03",
		"/admin/usage/stats?start_time=2026-10-04T02:45:00Z&end_time=2026-10-03T02:45:00Z",
	} {
		repo := &adminUsageRepoCapture{}
		router := newAdminUsageRequestTypeTestRouter(repo)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusBadRequest, rec.Code, path)
		require.Nil(t, repo.listFilters.StartTime, path)
		require.Nil(t, repo.statsFilters.StartTime, path)
	}
}
