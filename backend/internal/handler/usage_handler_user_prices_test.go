//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// userDashboardRepoStub 在 userUsageRepoCapture 上补概览数字（带今日 / 累计的标准计费）
type userDashboardRepoStub struct {
	*userUsageRepoCapture
}

func (userDashboardRepoStub) GetUserDashboardStats(context.Context, int64) (*usagestats.UserDashboardStats, error) {
	return &usagestats.UserDashboardStats{TodayRequests: 1, TodayCost: 0.3, TodayActualCost: 0.02, TotalCost: 3, TotalActualCost: 0.2}, nil
}

// 用户站接口只给实付：按官方价算的标价（cost / total_cost / today_cost）、用户倍率、渠道成本都不出接口
// （2026-10-04 D1：标价 ÷ 实付就是倍率；渠道成本可算毛利）。
func TestUserUsageEndpointsOnlyExposeActualCost(t *testing.T) {
	inputCost := 0.1
	repo := &userUsageRepoCapture{
		stats: &usagestats.UsageStats{
			TotalRequests: 1, TotalCost: 0.3, TotalActualCost: 0.02, TotalAccountCost: &inputCost,
			Endpoints:         []usagestats.EndpointStat{{Endpoint: "/v1/messages", Requests: 1, Cost: 0.3, ActualCost: 0.02, AccountCost: 0.01}},
			UpstreamEndpoints: []usagestats.EndpointStat{{Endpoint: "/v1/messages", Requests: 1, Cost: 0.3, ActualCost: 0.02, AccountCost: 0.01}},
		},
		trend:      []usagestats.TrendDataPoint{{Date: "2026-03-01", Requests: 1, Cost: 0.3, ActualCost: 0.02, AccountCost: 0.01}},
		modelStats: []usagestats.ModelStat{{Model: "gpt-5.5", Requests: 1, Cost: 0.3, ActualCost: 0.02, AccountCost: 0.01}},
		listRows: []service.UsageLog{{
			ID: 1, UserID: 42, Model: "gpt-5.5", InputCost: inputCost, OutputCost: 0.2, TotalCost: 0.3, ActualCost: 0.02, RateMultiplier: 1.0 / 15,
			ImageInputCost: 0.01, ImageOutputCost: 0.01,
		}},
	}
	router := newUserUsageRequestTypeTestRouter(repo)
	usageSvc := service.NewUsageService(userDashboardRepoStub{repo}, nil, nil, nil)
	router.GET("/usage/dashboard/stats", NewUsageHandler(usageSvc, nil, nil, nil).DashboardStats)

	for _, path := range []string{
		"/usage/stats?start_date=2026-03-01&end_date=2026-03-02",
		"/usage/dashboard/snapshot-v2?start_date=2026-03-01&end_date=2026-03-02",
		"/usage/dashboard/trend?start_date=2026-03-01&end_date=2026-03-02",
		"/usage/dashboard/models?start_date=2026-03-01&end_date=2026-03-02",
		"/usage",
		"/usage/dashboard/stats",
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, path+": "+rec.Body.String())
		body := rec.Body.String()
		require.Contains(t, body, `actual_cost":0.02`, path)
		// 明细行的分项费用（input_cost 等）是实付口径，可以给；见 dto 的 TestUsageLogDTOBilledItemsForUsersOfficialForAdmin
		for _, hidden := range []string{`"cost":`, `"total_cost"`, `"today_cost"`, `"rate_multiplier"`, `account_cost`, `"upstream_endpoints"`} {
			require.NotContains(t, body, hidden, path)
		}
	}
}
