//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 日汇总读的是小时表，而小时表里还留着上线前写入的分组行（group_id 非空），
// 它们与同一小时的平台行统计的是同一批请求。按 (日期, 平台) 汇总必须排除，
// 否则任何有历史分组行的平台都会被算两遍。7c 删列后此用例随之删除。
func TestUpsertDailyMetricsIgnoresLegacyGroupRows(t *testing.T) {
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, "TRUNCATE ops_metrics_hourly, ops_metrics_daily RESTART IDENTITY CASCADE")
	require.NoError(t, err)
	repo := NewOpsRepository(integrationDB).(*opsRepository)

	bucket := time.Date(2026, 9, 22, 3, 0, 0, 0, time.UTC)
	insert := func(platform any, groupID any, success int64, tokens int64) {
		_, err := integrationDB.ExecContext(ctx, `
			INSERT INTO ops_metrics_hourly (bucket_start, platform, group_id, success_count, error_count_total, token_consumed)
			VALUES ($1, $2, $3, $4, 0, $5)`, bucket, platform, groupID, success, tokens)
		require.NoError(t, err)
	}
	// 一小时的三行：总量行、平台行，以及上线前才会出现的分组行（统计的是同一批请求）。
	insert(nil, nil, 3, 30)
	insert("anthropic", nil, 3, 30)
	insert("anthropic", int64(7), 3, 30)

	require.NoError(t, repo.UpsertDailyMetrics(ctx, bucket.Add(-24*time.Hour), bucket.Add(24*time.Hour)))

	rows, err := integrationDB.QueryContext(ctx, `
		SELECT COALESCE(platform, ''), COALESCE(group_id, 0), success_count, token_consumed
		FROM ops_metrics_daily ORDER BY 1, 2`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	type row struct {
		platform string
		success  int64
		tokens   int64
	}
	got := make([]row, 0, 4)
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.platform, &r.groupID, &r.success, &r.tokens))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())

	require.Equal(t, []row{
		{platform: "", groupID: 0, success: 3, tokens: 30},
		{platform: "anthropic", groupID: 0, success: 3, tokens: 30},
	}, got, "分组行必须被排除：anthropic 只能有一行、且成功数不能翻倍")
}
