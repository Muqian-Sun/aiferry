package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// 只按渠道统计时，actual_cost 仍是收入（向用户扣的钱），渠道成本是逐行落的 usage_logs.account_cost 之和。
// 原来这三处在仅按 account_id 聚合时把 actual_cost 换成渠道成本，管理站渠道抽屉里「实际」和「成本」因此一模一样。

func TestUsageLogRepositoryAccountOnlyModelStatsKeepRevenue(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery(`(?s)COALESCE\(SUM\(actual_cost\), 0\) as actual_cost,\s+COALESCE\(SUM\(account_cost\), 0\) as account_cost.*AND account_id = \$3`).
		WithArgs(start, end, int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"model", "requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens",
			"total_tokens", "cost", "actual_cost", "account_cost",
		}).AddRow("claude-sonnet-4-5", int64(3), int64(10), int64(20), int64(0), int64(0), int64(30), 1.0, 1.5, 0.6))

	stats, err := repo.GetModelStatsWithFilters(context.Background(), start, end, 0, 0, 9, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.InDelta(t, 1.5, stats[0].ActualCost, 1e-9)
	require.InDelta(t, 0.6, stats[0].AccountCost, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryAccountOnlyEndpointStatsCarryCost(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery(`(?s)COALESCE\(SUM\(actual_cost\), 0\) as actual_cost,\s+COALESCE\(SUM\(account_cost\), 0\) as account_cost\s+FROM usage_logs.*AND account_id = \$3`).
		WithArgs(start, end, int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"endpoint", "requests", "total_tokens", "cost", "actual_cost", "account_cost"}).
			AddRow("/v1/messages", int64(3), int64(30), 1.0, 1.5, 0.6))

	stats, err := repo.GetEndpointStatsWithFilters(context.Background(), start, end, 0, 0, 9, "", nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.InDelta(t, 1.5, stats[0].ActualCost, 1e-9)
	require.InDelta(t, 0.6, stats[0].AccountCost, 1e-9)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryAccountOnlyUsageStatsEndpointsKeepRevenue(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	mock.ExpectQuery(`(?s)actual_cost,\s+account_cost,\s+duration_ms\s+FROM usage_logs\s+WHERE account_id = \$1.*COALESCE\(SUM\(account_cost\), 0\) AS account_cost.*GROUP BY GROUPING SETS`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"inbound_grouped", "upstream_grouped", "inbound_endpoint", "upstream_endpoint",
			"requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens",
			"cost", "actual_cost", "account_cost", "avg_duration_ms",
		}).
			AddRow(1, 1, nil, nil, int64(1), int64(2), int64(3), int64(0), int64(0), 1.2, 1.0, 0.4, 20.0).
			AddRow(0, 1, "/v1/messages", nil, int64(1), int64(2), int64(3), int64(0), int64(0), 1.2, 1.0, 0.4, 20.0).
			AddRow(1, 0, nil, "/v1/messages", int64(1), int64(2), int64(3), int64(0), int64(0), 1.2, 1.0, 0.4, 20.0).
			AddRow(0, 0, "/v1/messages", "/v1/messages", int64(1), int64(2), int64(3), int64(0), int64(0), 1.2, 1.0, 0.4, 20.0))

	stats, err := repo.GetStatsWithFilters(context.Background(), usagestats.UsageLogFilters{AccountID: 9})
	require.NoError(t, err)
	for _, group := range [][]EndpointStat{stats.Endpoints, stats.UpstreamEndpoints, stats.EndpointPaths} {
		require.Len(t, group, 1)
		require.InDelta(t, 1.0, group[0].ActualCost, 1e-9)
		require.InDelta(t, 0.4, group[0].AccountCost, 1e-9)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
