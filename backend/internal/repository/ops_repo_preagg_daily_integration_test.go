//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestUpsertDailyMetricsAggregatesByDateAndPlatform 固定日汇总在 7c 之后的口径：
// 小时表按 (日期, 平台) 聚合成日表一行，唯一维度只有两列（迁移 252 把索引和
// ON CONFLICT 目标都从三列收窄到两列），重复执行只更新不新增。
//
// 原用例测的是「排除历史分组行」（group_id 非空的那一层），列删掉后那个场景不存在了；
// 但 UpsertDailyMetrics 的 WHERE 与 ON CONFLICT 在 7c 都改过，聚合口径本身要有用例盯着。
func TestUpsertDailyMetricsAggregatesByDateAndPlatform(t *testing.T) {
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, "TRUNCATE ops_metrics_hourly, ops_metrics_daily RESTART IDENTITY CASCADE")
	require.NoError(t, err)
	repo := NewOpsRepository(integrationDB).(*opsRepository)

	day := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	insert := func(hour int, platform any, success int64, tokens int64) {
		_, err := integrationDB.ExecContext(ctx, `
			INSERT INTO ops_metrics_hourly (bucket_start, platform, success_count, error_count_total, token_consumed)
			VALUES ($1, $2, $3, 0, $4)`, day.Add(time.Duration(hour)*time.Hour), platform, success, tokens)
		require.NoError(t, err)
	}
	// 同一天两个小时桶 × (总量行, anthropic 行)：每个维度都该被加起来。
	insert(3, nil, 3, 30)
	insert(5, nil, 4, 40)
	insert(3, "anthropic", 3, 30)
	insert(5, "anthropic", 4, 40)
	// 另一个平台只有一个小时桶。
	insert(3, "openai", 1, 10)

	type row struct {
		platform string
		success  int64
		tokens   int64
	}
	readDaily := func() []row {
		rows, err := integrationDB.QueryContext(ctx, `
			SELECT COALESCE(platform, ''), success_count, token_consumed
			FROM ops_metrics_daily ORDER BY 1`)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()
		out := make([]row, 0, 4)
		for rows.Next() {
			var r row
			require.NoError(t, rows.Scan(&r.platform, &r.success, &r.tokens))
			out = append(out, r)
		}
		require.NoError(t, rows.Err())
		return out
	}

	want := []row{
		{platform: "", success: 7, tokens: 70},
		{platform: "anthropic", success: 7, tokens: 70},
		{platform: "openai", success: 1, tokens: 10},
	}

	require.NoError(t, repo.UpsertDailyMetrics(ctx, day.Add(-24*time.Hour), day.Add(24*time.Hour)))
	require.Equal(t, want, readDaily(), "按 (日期, 平台) 汇总：两个小时桶要合成一行")

	// 再跑一次：ON CONFLICT 走 UPDATE 分支，行数不变、数值不翻倍。
	require.NoError(t, repo.UpsertDailyMetrics(ctx, day.Add(-24*time.Hour), day.Add(24*time.Hour)))
	require.Equal(t, want, readDaily(), "重复执行必须幂等（唯一维度 = (bucket_date, platform)）")
}
