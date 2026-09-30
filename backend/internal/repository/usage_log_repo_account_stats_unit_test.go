//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

// 渠道统计（/admin/accounts/:id/stats）的金额和同一响应里的 models[] 同名同义：
// actual_cost 是收入（Σ usage_logs.actual_cost），account_cost 是渠道成本（Σ usage_logs.account_cost）。
// 以前 history[].actual_cost / summary.total_cost / today.cost 实际是渠道成本，user_cost 才是收入。
func TestGetAccountUsageStats_ActualCostIsRevenueAndAccountCostIsCost(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 30)
	today := timezone.Now().Format("2006-01-02")

	selectList := strings.Join([]string{
		regexp.QuoteMeta("COALESCE(SUM(actual_cost), 0) as actual_cost,"),
		regexp.QuoteMeta("COALESCE(SUM(account_cost), 0) as account_cost"),
	}, `\s*`)
	mock.ExpectQuery(selectList).
		WithArgs(int64(7), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"date", "requests", "tokens", "actual_cost", "account_cost"}).
			// 收入最高的一天
			AddRow("2026-01-01", int64(10), int64(1000), 5.0, 2.0).
			// 请求最多、成本最高，但收入不是最高
			AddRow("2026-01-02", int64(20), int64(50), 3.0, 9.0).
			// 今天：亏本
			AddRow(today, int64(3), int64(300), 1.5, 4.0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(AVG(duration_ms), 0)")).
		WithArgs(int64(7), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"avg_duration_ms"}).AddRow(1234.0))
	// 模型 / 端点分布走各自的查询，这里不关心：让它们报错，函数会降级成空列表。

	resp, err := repo.GetAccountUsageStats(context.Background(), 7, start, end)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	require.Len(t, resp.History, 3)
	require.Equal(t, 5.0, resp.History[0].ActualCost)
	require.Equal(t, 2.0, resp.History[0].AccountCost)
	require.Equal(t, "01/01", resp.History[0].Label)

	s := resp.Summary
	require.Equal(t, 31, s.Days)
	require.Equal(t, 3, s.ActualDaysUsed)
	require.InDelta(t, 9.5, s.TotalActualCost, 1e-9)
	require.InDelta(t, 15.0, s.TotalAccountCost, 1e-9)
	require.Equal(t, int64(33), s.TotalRequests)
	require.Equal(t, int64(1350), s.TotalTokens)
	require.InDelta(t, 9.5/3, s.AvgDailyActualCost, 1e-9)
	require.InDelta(t, 5.0, s.AvgDailyAccountCost, 1e-9)
	require.Equal(t, 1234.0, s.AvgDurationMs)

	require.NotNil(t, s.Today)
	require.Equal(t, today, s.Today.Date)
	require.Equal(t, 1.5, s.Today.ActualCost)
	require.Equal(t, 4.0, s.Today.AccountCost)

	require.NotNil(t, s.HighestRevenueDay)
	require.Equal(t, "2026-01-01", s.HighestRevenueDay.Date, "收入最高日按收入挑，不按成本")
	require.NotNil(t, s.HighestRequestDay)
	require.Equal(t, "2026-01-02", s.HighestRequestDay.Date)
}

func TestSummarizeAccountUsageHistory_EmptyHasNoDays(t *testing.T) {
	s := summarizeAccountUsageHistory(nil, 30, 0, "2026-01-01")
	require.Equal(t, 1, s.ActualDaysUsed)
	require.Zero(t, s.TotalActualCost)
	require.Zero(t, s.TotalAccountCost)
	require.Nil(t, s.Today)
	require.Nil(t, s.HighestRevenueDay)
	require.Nil(t, s.HighestRequestDay)
}

// 前端按这些字段名读：收入 actual_cost / total_actual_cost，成本 account_cost / total_account_cost。
// 旧名（cost、user_cost、total_cost、total_user_cost、total_standard_cost、highest_cost_day）不能再出现。
func TestAccountUsageStatsJSONFieldNames(t *testing.T) {
	day := AccountUsageHistory{Date: "2026-01-01", Label: "01/01", Requests: 1, Tokens: 2, ActualCost: 3, AccountCost: 4}
	summary := summarizeAccountUsageHistory([]AccountUsageHistory{day}, 30, 0, "2026-01-01")
	raw, err := json.Marshal(AccountUsageStatsResponse{History: []AccountUsageHistory{day}, Summary: summary})
	require.NoError(t, err)

	var decoded struct {
		History []map[string]any `json:"history"`
		Summary map[string]any   `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	historyKeys := keysOf(decoded.History[0])
	require.ElementsMatch(t, []string{"date", "label", "requests", "tokens", "actual_cost", "account_cost"}, historyKeys)
	require.Equal(t, 3.0, decoded.History[0]["actual_cost"])
	require.Equal(t, 4.0, decoded.History[0]["account_cost"])

	for _, key := range []string{"total_actual_cost", "total_account_cost", "avg_daily_actual_cost", "avg_daily_account_cost", "today", "highest_revenue_day", "highest_request_day"} {
		require.Contains(t, decoded.Summary, key)
	}
	for _, key := range []string{"total_cost", "total_user_cost", "total_standard_cost", "avg_daily_cost", "avg_daily_user_cost", "highest_cost_day"} {
		require.NotContains(t, decoded.Summary, key)
	}
	todayKeys := keysOf(decoded.Summary["today"].(map[string]any))
	require.ElementsMatch(t, historyKeys, todayKeys)
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
