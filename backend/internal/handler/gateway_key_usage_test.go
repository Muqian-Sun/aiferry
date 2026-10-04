//go:build unit

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// keyUsageRepoStub 只给按模型统计（带渠道成本），其余读数失败（handler 按 best-effort 跳过）。
type keyUsageRepoStub struct {
	service.UsageLogRepository
}

func (keyUsageRepoStub) GetModelStatsWithFilters(context.Context, time.Time, time.Time, int64, int64, int64, *int16, *bool, *int8) ([]usagestats.ModelStat, error) {
	return []usagestats.ModelStat{{Model: "gpt-5.5", Requests: 2, Cost: 0.002, ActualCost: 0.0001, AccountCost: 0.00005}}, nil
}

func (keyUsageRepoStub) GetAPIKeyDashboardStats(context.Context, int64) (*usagestats.UserDashboardStats, error) {
	return nil, errors.New("not in this test")
}

func (keyUsageRepoStub) GetUsageTrendWithFilters(context.Context, time.Time, time.Time, string, int64, int64, int64, string, *int16, *bool, *int8) ([]usagestats.TrendDataPoint, error) {
	return nil, errors.New("not in this test")
}

// 按 key 查用量（/v1/usage，持 key 的任何人都能调）：按模型统计不能带渠道成本，否则可以算出平台毛利
// （2026-10-04 UI E2E 发现原样返回了 usagestats.ModelStat）。
func TestGatewayHandlerUsage_ModelStatsHideAccountCost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &GatewayHandler{usageService: service.NewUsageService(keyUsageRepoStub{}, nil, nil, nil)}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)
	// 设了总额度：走 quota_limited 分支，不需要查用户
	apiKey := &service.APIKey{ID: 7, UserID: 3, Status: service.StatusActive, Quota: 10, User: &service.User{ID: 3}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 3})

	h.Usage(c)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"model_stats"`)
	require.Contains(t, rec.Body.String(), `"actual_cost":0.0001`)
	require.NotContains(t, rec.Body.String(), "account_cost")
}
