//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 运维看板按模型、渠道筛选，以及「换渠道恢复」次数（muqian 2026-10-04）：真库跑一遍 SQL。
func TestGetDashboardOverview_ModelAccountScopeAndRecovered(t *testing.T) {
	ctx := context.Background()
	_, _ = integrationDB.ExecContext(ctx, "TRUNCATE ops_error_logs RESTART IDENTITY CASCADE")
	repo, ok := NewOpsRepository(integrationDB).(*opsRepository)
	require.True(t, ok)

	now := time.Now()
	accountA, accountB, accountC := int64(7), int64(8), int64(9)
	upstream503 := 503
	for _, row := range []*service.OpsInsertErrorLogInput{
		// 没选到渠道：用户收到 503，算失败
		{RequestID: "routing", ErrorPhase: "routing", ErrorType: "api_error", Severity: "error", StatusCode: 503,
			Model: "gpt-5.5", ErrorOwner: "platform", CreatedAt: now.Add(-5 * time.Minute)},
		// 上游 503、换渠道后成功：状态码是最后给用户的 200
		{RequestID: "recovered", ErrorPhase: "upstream", ErrorType: "upstream_error", Severity: "error", StatusCode: 200,
			Model: "gpt-5.5", AccountID: &accountA, UpstreamStatusCode: &upstream503, ErrorOwner: "provider", CreatedAt: now.Add(-4 * time.Minute)},
		// 别的模型、别的渠道的上游失败
		{RequestID: "upstream", ErrorPhase: "upstream", ErrorType: "upstream_error", Severity: "error", StatusCode: 502,
			Model: "claude-sonnet-4-6", AccountID: &accountB, ErrorOwner: "provider", CreatedAt: now.Add(-3 * time.Minute)},
		// 请求的是别名：按模型筛选先看请求的模型（与错误列表同一口径）
		{RequestID: "alias", ErrorPhase: "upstream", ErrorType: "upstream_error", Severity: "error", StatusCode: 502,
			Model: "gpt-5.5", RequestedModel: "gpt-latest", AccountID: &accountA, ErrorOwner: "provider", CreatedAt: now.Add(-2 * time.Minute)},
		// 落在整点小时里（预聚合负责的那一段）的一行，只用于下面「强制查原始表」那条
		{RequestID: "full-hour", ErrorPhase: "upstream", ErrorType: "upstream_error", Severity: "error", StatusCode: 502,
			Model: "gpt-5.5", AccountID: &accountC, ErrorOwner: "provider", CreatedAt: now.Add(-150 * time.Minute)},
	} {
		_, err := repo.InsertErrorLog(ctx, row)
		require.NoError(t, err)
	}

	overview := func(f service.OpsDashboardFilter) *service.OpsDashboardOverview {
		t.Helper()
		if f.StartTime.IsZero() {
			f.StartTime, f.EndTime = now.Add(-time.Hour), now.Add(time.Minute)
		}
		if f.QueryMode == "" {
			f.QueryMode = service.OpsQueryModeRaw
		}
		out, err := repo.GetDashboardOverview(ctx, &f)
		require.NoError(t, err)
		return out
	}

	all := overview(service.OpsDashboardFilter{})
	require.Equal(t, int64(3), all.ErrorCountTotal, "状态码 >= 400 的三行")
	require.Equal(t, int64(1), all.UpstreamRecoveredCount)

	byModel := overview(service.OpsDashboardFilter{Model: "gpt-5.5"})
	require.Equal(t, int64(1), byModel.ErrorCountTotal, "没选到渠道那一行；别名请求按请求的模型算，不在 gpt-5.5 里")
	require.Equal(t, int64(1), byModel.UpstreamRecoveredCount)

	byAlias := overview(service.OpsDashboardFilter{Model: "gpt-latest"})
	require.Equal(t, int64(1), byAlias.ErrorCountTotal)

	byAccount := overview(service.OpsDashboardFilter{AccountID: accountB})
	require.Equal(t, int64(1), byAccount.ErrorCountTotal)
	require.Zero(t, byAccount.UpstreamRecoveredCount)

	// 预聚合表没有渠道维度：要求走预聚合也改查原始表。时间窗跨好几个整点小时，整点那段本该由预聚合表负责；
	// 预聚合表是空的而原始表有这个渠道的数据，真走了预聚合会报「未就绪」（原来还会把没按渠道过滤的整点汇总加进来）。
	preagg := overview(service.OpsDashboardFilter{AccountID: accountC, QueryMode: service.OpsQueryModePreagg,
		StartTime: now.Add(-4 * time.Hour), EndTime: now})
	require.Equal(t, int64(1), preagg.ErrorCountTotal, "整点小时里那一行")
	require.Zero(t, preagg.UpstreamRecoveredCount)
}
