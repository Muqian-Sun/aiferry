//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 运维页趋势图（2026-10-04）：每个时间段的首字延迟 P50 / P99，以及失败按环节拆开、换渠道恢复单独数。
func TestOpsTrends_TTFTPercentilesAndFailureBreakdown(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE ops_error_logs RESTART IDENTITY CASCADE")
	repo, ok := NewOpsRepository(integrationDB).(*opsRepository)
	require.True(t, ok)

	suffix := time.Now().UnixNano()
	model := fmt.Sprintf("trend-model-%d", suffix)
	user := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("ops-trend-%d@example.com", suffix), Concurrency: 5})
	key := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: user.ID, Key: fmt.Sprintf("sk-ops-trend-%d", suffix)})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: fmt.Sprintf("ops-trend-%d", suffix)})

	hour := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)
	for i, ttft := range []int{1000, 2000, 9000} {
		_, err := integrationEntClient.UsageLog.Create().
			SetUserID(user.ID).SetAPIKeyID(key.ID).SetAccountID(account.ID).
			SetRequestID(fmt.Sprintf("ops-trend-%d-%d", suffix, i)).
			SetModel(model).SetFirstTokenMs(ttft).
			SetCreatedAt(hour.Add(time.Duration(10*(i+1)) * time.Minute)).
			Save(ctx)
		require.NoError(t, err)
	}
	accountID := account.ID
	errorAt := hour.Add(40 * time.Minute)
	for _, row := range []*service.OpsInsertErrorLogInput{
		{RequestID: "upstream-failed", ErrorPhase: "upstream", ErrorType: "upstream_error", Severity: "error", StatusCode: 502,
			Model: model, AccountID: &accountID, ErrorOwner: "provider", CreatedAt: errorAt},
		{RequestID: "routing-failed", ErrorPhase: "routing", ErrorType: "api_error", Severity: "error", StatusCode: 503,
			Model: model, ErrorOwner: "platform", CreatedAt: errorAt},
		{RequestID: "recovered", ErrorPhase: "upstream", ErrorType: "upstream_error", Severity: "error", StatusCode: 200,
			Model: model, AccountID: &accountID, ErrorOwner: "provider", CreatedAt: errorAt},
		// 用户自己的并发限制：业务限制，不算失败
		{RequestID: "user-limit", ErrorPhase: "concurrency", ErrorType: "rate_limit_error", Severity: "warn", StatusCode: 429,
			Model: model, ErrorOwner: "client", IsBusinessLimited: true, CreatedAt: errorAt},
	} {
		_, err := repo.InsertErrorLog(ctx, row)
		require.NoError(t, err)
	}

	filter := &service.OpsDashboardFilter{StartTime: hour, EndTime: hour.Add(2 * time.Hour), Model: model}

	throughput, err := repo.GetThroughputTrend(ctx, filter, 3600)
	require.NoError(t, err)
	require.Len(t, throughput.Points, 2)
	first := throughput.Points[0]
	require.Equal(t, int64(6), first.RequestCount, "3 次成功 + 3 次状态码 >= 400")
	require.NotNil(t, first.TTFTP50Ms)
	require.Equal(t, 2000, *first.TTFTP50Ms)
	require.NotNil(t, first.TTFTP99Ms)
	require.Equal(t, 8860, *first.TTFTP99Ms, "percentile_cont(0.99) 在 2000 与 9000 之间插值")
	require.Nil(t, throughput.Points[1].TTFTP50Ms, "没有样本的时间段留空")

	errTrend, err := repo.GetErrorTrend(ctx, filter, 3600)
	require.NoError(t, err)
	require.Len(t, errTrend.Points, 2)
	got := errTrend.Points[0]
	require.Equal(t, int64(2), got.ErrorCountSLA)
	require.Equal(t, int64(1), got.UpstreamFailedCount)
	require.Equal(t, int64(1), got.RoutingFailedCount)
	require.Equal(t, int64(1), got.RecoveredCount)
	require.Equal(t, int64(1), got.BusinessLimitedCount)
}
