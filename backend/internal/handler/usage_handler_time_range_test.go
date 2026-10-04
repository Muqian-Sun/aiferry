package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 用户站「近 24 小时」：明细与区间统计按精确时刻查，不展开成两个自然日
func TestUserUsageListAndStatsUsePreciseTimeRange(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)
	wantStart := time.Date(2026, 10, 3, 2, 45, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 10, 4, 2, 45, 0, 0, time.UTC)
	const rangeQuery = "start_time=2026-10-03T02:45:00Z&end_time=2026-10-04T02:45:00Z&timezone=Asia/Shanghai"

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage?"+rangeQuery, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.listFilters.StartTime)
	require.NotNil(t, repo.listFilters.EndTime)
	require.True(t, repo.listFilters.StartTime.Equal(wantStart), "list start=%s", repo.listFilters.StartTime)
	require.True(t, repo.listFilters.EndTime.Equal(wantEnd), "list end=%s", repo.listFilters.EndTime)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/usage/stats?"+rangeQuery, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.statsFilters.StartTime)
	require.NotNil(t, repo.statsFilters.EndTime)
	require.True(t, repo.statsFilters.StartTime.Equal(wantStart), "stats start=%s", repo.statsFilters.StartTime)
	require.True(t, repo.statsFilters.EndTime.Equal(wantEnd), "stats end=%s", repo.statsFilters.EndTime)
}

// 起止颠倒直接 400：原来回空列表，页面显示成「先创建一把密钥」的空状态
func TestUserUsageRejectsReversedRange(t *testing.T) {
	for _, path := range []string{
		"/usage?start_date=2026-10-05&end_date=2026-10-03",
		"/usage/stats?start_date=2026-10-05&end_date=2026-10-03",
		"/usage/dashboard/models?start_time=2026-10-04T02:45:00Z&end_time=2026-10-03T02:45:00Z",
	} {
		repo := &userUsageRepoCapture{}
		router := newUserUsageRequestTypeTestRouter(repo)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusBadRequest, rec.Code, path)
		require.Nil(t, repo.listFilters.StartTime, path)
		require.Nil(t, repo.statsFilters.StartTime, path)
	}
}
