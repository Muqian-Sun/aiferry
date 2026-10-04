package repository

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

// 管理站概览的「利润」趋势要每个时间桶的渠道成本：三条趋势查询（明细表、小时预聚合、天预聚合）
// 和按用户的趋势共用 scanTrendRows，都得把 account_cost 带出来。明细表直接累加逐行落的 usage_logs.account_cost。
func TestUsageLogRepositoryTrendCarriesAccountCost(t *testing.T) {
	// 起止取服务器时区的零点：预聚合表按服务器时区分桶，落在桶边界上才走预聚合（见 trendRangeAlignedToBuckets）
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, timezone.Location())
	end := start.Add(24 * time.Hour)
	columns := []string{
		"date", "requests", "input_tokens", "output_tokens", "cache_creation_tokens",
		"cache_read_tokens", "total_tokens", "cost", "actual_cost", "account_cost",
	}

	cases := []struct {
		name  string
		query string
		args  []driver.Value
		load  func(repo *usageLogRepository) ([]TrendDataPoint, error)
	}{
		{
			name:  "usage_logs with filter",
			query: `(?s)COALESCE\(SUM\(account_cost\), 0\) as account_cost\s+FROM usage_logs\s+WHERE created_at`,
			args:  []driver.Value{start, end, int64(7)},
			load: func(repo *usageLogRepository) ([]TrendDataPoint, error) {
				return repo.GetUsageTrendWithUsageFilters(context.Background(), start, end, "day", usagestats.UsageLogFilters{UserID: 7})
			},
		},
		{
			name:  "hourly aggregate",
			query: `(?s)actual_cost,\s+account_cost\s+FROM usage_dashboard_hourly`,
			args:  []driver.Value{start, end},
			load: func(repo *usageLogRepository) ([]TrendDataPoint, error) {
				return repo.GetUsageTrendWithUsageFilters(context.Background(), start, end, "hour", usagestats.UsageLogFilters{})
			},
		},
		{
			name:  "daily aggregate",
			query: `(?s)actual_cost,\s+account_cost\s+FROM usage_dashboard_daily`,
			args:  []driver.Value{start, end},
			load: func(repo *usageLogRepository) ([]TrendDataPoint, error) {
				return repo.GetUsageTrendWithUsageFilters(context.Background(), start, end, "day", usagestats.UsageLogFilters{})
			},
		},
		{
			name:  "by user id",
			query: `(?s)COALESCE\(SUM\(account_cost\), 0\) as account_cost\s+FROM usage_logs\s+WHERE user_id = \$1`,
			args:  []driver.Value{int64(7), start, end},
			load: func(repo *usageLogRepository) ([]TrendDataPoint, error) {
				return repo.GetUserUsageTrendByUserID(context.Background(), 7, start, end, "day")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := &usageLogRepository{sql: db}
			mock.ExpectQuery(tc.query).
				WithArgs(tc.args...).
				WillReturnRows(sqlmock.NewRows(columns).AddRow("2025-01-01", int64(3), int64(10), int64(20), int64(0), int64(0), int64(30), 1.5, 1.8, 1.2))

			trend, err := tc.load(repo)
			require.NoError(t, err)
			require.Len(t, trend, 1)
			require.InDelta(t, 1.5, trend[0].Cost, 1e-9)
			require.InDelta(t, 1.8, trend[0].ActualCost, 1e-9)
			require.InDelta(t, 1.2, trend[0].AccountCost, 1e-9)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 「近 24 小时」这类精确时刻的区间落在桶中间：不能截取预聚合表（小时表漏掉起点那一小时，天表按 ::date 多算起点那天、丢掉终点那天），
// 要走明细表。两张表的查询都备好、不限顺序，看最后用的是哪份数据。
func TestUsageLogRepositoryTrendUnalignedRangeSkipsAggregates(t *testing.T) {
	start := time.Date(2026, 10, 3, 10, 45, 0, 0, timezone.Location())
	end := start.Add(24 * time.Hour)
	columns := []string{
		"date", "requests", "input_tokens", "output_tokens", "cache_creation_tokens",
		"cache_read_tokens", "total_tokens", "cost", "actual_cost", "account_cost",
	}

	for granularity, aggregateTable := range map[string]string{"hour": "usage_dashboard_hourly", "day": "usage_dashboard_daily"} {
		t.Run(granularity, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := &usageLogRepository{sql: db}
			mock.MatchExpectationsInOrder(false)
			mock.ExpectQuery(`FROM ` + aggregateTable).
				WillReturnRows(sqlmock.NewRows(columns).AddRow("from-aggregate", int64(1), int64(0), int64(0), int64(0), int64(0), int64(0), 0.0, 0.0, 0.0))
			mock.ExpectQuery(`(?s)FROM usage_logs\s+WHERE created_at >= \$1 AND created_at < \$2`).
				WithArgs(start, end).
				WillReturnRows(sqlmock.NewRows(columns).AddRow("from-usage-logs", int64(1), int64(0), int64(0), int64(0), int64(0), int64(0), 0.0, 0.0, 0.0))

			trend, err := repo.GetUsageTrendWithUsageFilters(context.Background(), start, end, granularity, usagestats.UsageLogFilters{})
			require.NoError(t, err)
			require.Len(t, trend, 1)
			require.Equal(t, "from-usage-logs", trend[0].Date)
		})
	}
}

func TestTrendRangeAlignedToBuckets(t *testing.T) {
	loc := timezone.Location()
	midnight := time.Date(2026, 10, 3, 0, 0, 0, 0, loc)
	onHour := time.Date(2026, 10, 3, 10, 0, 0, 0, loc)
	offHour := time.Date(2026, 10, 3, 10, 45, 0, 0, loc)

	require.True(t, trendRangeAlignedToBuckets("day", midnight, midnight.AddDate(0, 0, 7)))
	require.False(t, trendRangeAlignedToBuckets("day", onHour, onHour.Add(24*time.Hour)))
	require.True(t, trendRangeAlignedToBuckets("hour", onHour, onHour.Add(24*time.Hour)))
	require.False(t, trendRangeAlignedToBuckets("hour", offHour, offHour.Add(24*time.Hour)))
}
