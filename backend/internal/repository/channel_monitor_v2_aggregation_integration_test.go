//go:build integration

package repository

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestChannelMonitorV2RecomputeRangeUpsertsMatchRebuiltPrimaryKeys 真实跑一遍 v2 聚合。
//
// 7c（迁移 252）把这 8 张表的主键从含 group_id 收窄了一列，同时把
// channel_monitor_v2_aggregation 里三处 ON CONFLICT 目标同步改窄。两份是手写的，
// 一旦不一致，UPSERT 会在运行时报 "no unique or exclusion constraint matching the
// ON CONFLICT specification" —— 而 RecomputeRange 此前只有 stub 测试，没有任何
// 集成用例真正执行过这些语句。
func TestChannelMonitorV2RecomputeRangeUpsertsMatchRebuiltPrimaryKeys(t *testing.T) {
	ctx := context.Background()
	repo := NewChannelMonitorV2Repository(integrationDB)

	// 落在保留窗口内（RecomputeRange 会把 start 夹到最长 rollup TTL）
	bucket := time.Now().UTC().Truncate(time.Minute).Add(-10 * time.Minute)
	start, end := bucket.Add(-time.Minute), bucket.Add(2*time.Minute)

	_, err := integrationDB.ExecContext(ctx,
		"DELETE FROM usage_logs WHERE created_at >= $1 AND created_at < $2", start, end)
	require.NoError(t, err)

	// usage_logs 的 user_id / api_key_id 都有外键：用原生 SQL 造，拿回自增 ID。
	// （ent 夹具走的是另一条连接，这里要的是 integrationDB 上可见的行）
	mkUser := func(email string) int64 {
		var id int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `
			INSERT INTO users (email, password_hash, role, status, concurrency)
			VALUES ($1, 'x', 'user', 'active', 5) RETURNING id`, email).Scan(&id))
		return id
	}
	mkKey := func(userID int64, key string) int64 {
		var id int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, `
			INSERT INTO api_keys (user_id, key, name, status)
			VALUES ($1, $2, $2, 'active') RETURNING id`, userID, key).Scan(&id))
		return id
	}
	// account_id 同样有外键，造一个账号
	var accountID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, status, concurrency, credentials, extra, protocol_endpoints)
		VALUES ('cmv2-acc', 'anthropic', 'api_key', 'active', 1, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb)
		RETURNING id`).Scan(&accountID))
	userAID, userBID := mkUser("cmv2-a@example.com"), mkUser("cmv2-b@example.com")
	keyAID, keyBID := mkKey(userAID, "sk-cmv2-a"), mkKey(userBID, "sk-cmv2-b")

	// 两条同 (分钟, 平台, 模型) 的成功请求 + 一条不同模型，覆盖聚合与分组。
	// 成功判据是 actual_cost > 0（usageLogSuccessFilterUL）。
	insert := func(requestID, model string, userID, apiKeyID int64, inputTokens int, firstTokenMs, durationMs int) {
		_, err := integrationDB.ExecContext(ctx, `
			INSERT INTO usage_logs (
				request_id, user_id, api_key_id, account_id, model, requested_model,
				input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
				total_cost, actual_cost, rate_multiplier, billing_type, request_type,
				first_token_ms, duration_ms, created_at
			) VALUES ($1, $2, $3, $4, $5, $5, $6, 1, 0, 0, 0.01, 0.01, 1, 0, 0, $7, $8, $9)`,
			requestID, userID, apiKeyID, accountID, model, inputTokens, firstTokenMs, durationMs, bucket)
		require.NoError(t, err)
	}
	insert("e2e-cmv2-1", "claude-fable-5", userAID, keyAID, 10, 100, 900)
	insert("e2e-cmv2-2", "claude-fable-5", userAID, keyAID, 20, 200, 1100)
	insert("e2e-cmv2-3", "gpt-5.5", userBID, keyBID, 5, 50, 400)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM usage_logs WHERE created_at >= $1 AND created_at < $2", start, end)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", accountID)
		for _, kid := range []int64{keyAID, keyBID} {
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE id = $1", kid)
		}
		for _, uid := range []int64{userAID, userBID} {
			_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", uid)
		}
		for _, table := range []string{
			"channel_monitor_v2_metrics_1m", "channel_monitor_v2_metrics_rollup",
			"channel_monitor_v2_error_metrics_1m", "channel_monitor_v2_error_metrics_rollup",
			"channel_monitor_v2_latency_histograms_1m", "channel_monitor_v2_latency_histograms_rollup",
		} {
			_, _ = integrationDB.ExecContext(context.Background(),
				"DELETE FROM "+table+" WHERE bucket_start >= $1 AND bucket_start < $2", start, end)
		}
	})

	countRows := func(table string) int {
		var n int
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			"SELECT count(*) FROM "+table+" WHERE bucket_start >= $1 AND bucket_start < $2",
			start, end).Scan(&n))
		return n
	}
	sumSuccess := func(table string) int {
		var n int
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			"SELECT COALESCE(SUM(success_requests), 0) FROM "+table+" WHERE bucket_start >= $1 AND bucket_start < $2",
			start, end).Scan(&n))
		return n
	}

	require.NoError(t, repo.RecomputeRange(ctx, start, end), "首次聚合")
	firstMetrics, firstHistograms := countRows("channel_monitor_v2_metrics_1m"), countRows("channel_monitor_v2_latency_histograms_1m")
	require.Equal(t, 2, firstMetrics, "两个模型各一行")
	// 延迟分布只有全站一份（迁移 258 去掉了按用户的副本）：
	// claude-fable-5 首字 100 / 200 落 100、250 两档，耗时 900 / 1100 落 1000、2000 两档；gpt-5.5 各一档
	require.Equal(t, 6, firstHistograms, "每个模型每个指标每档一行")
	require.Equal(t, 3, sumSuccess("channel_monitor_v2_metrics_1m"), "三条成功请求")
	// rollup 的 bucket_start 按 bucket_seconds 对齐，可能落在 [start,end) 之外，
	// 所以不按窗口断言行数；这三张 rollup 表的 UPSERT 能否匹配重建后的主键，
	// 已由上面 RecomputeRange 不返回 error 证明（不匹配会直接报
	// "no unique or exclusion constraint matching the ON CONFLICT specification"）。
	var rollupRows int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT count(*) FROM channel_monitor_v2_metrics_rollup").Scan(&rollupRows))
	require.Positive(t, rollupRows, "rollup 表应有行")

	// 再跑一次：窗口重写 + UPSERT 都要幂等，行数与数值不变。
	require.NoError(t, repo.RecomputeRange(ctx, start, end), "重复聚合必须幂等")
	require.Equal(t, firstMetrics, countRows("channel_monitor_v2_metrics_1m"))
	require.Equal(t, firstHistograms, countRows("channel_monitor_v2_latency_histograms_1m"))
	require.Equal(t, 3, sumSuccess("channel_monitor_v2_metrics_1m"), "成功数不得翻倍")

	// 读数：只统计名单里的模型（gpt-5.5 不在名单），百分位由全站那份延迟分布现算。
	// 段长 1 分钟不是固定粒度，走 1m 表 + date_bin 这条查询路径。
	cfg := service.ChannelMonitorV2Config{Roster: service.ChannelMonitorV2ModelRoster{
		Models:  []string{"claude-fable-5"},
		Resolve: func(model string) (string, bool) { return model, model == "claude-fable-5" },
	}}
	filter := service.ChannelMonitorV2Filter{Start: start, End: end, Bucket: time.Minute}
	matrix, err := repo.GetMatrix(ctx, filter, cfg)
	require.NoError(t, err)
	require.Len(t, matrix.Items, 1)
	row := matrix.Items[0]
	require.Equal(t, "claude-fable-5", row.Model)
	require.Equal(t, int64(2), row.Metrics.RequestCount)
	require.Equal(t, int64(2), row.Metrics.TTFT.SampleCount)
	require.NotNil(t, row.Metrics.TTFT.P50Ms)
	require.Equal(t, int64(100), *row.Metrics.TTFT.P50Ms, "首字 100 / 200 落 100、250 两档，P50 取 100 档")
	require.Len(t, row.Buckets, 1)

	snapshot, err := repo.GetSnapshot(ctx, filter, cfg)
	require.NoError(t, err)
	require.Equal(t, int64(2), snapshot.Metrics.RequestCount, "名单外的 gpt-5.5 不计入整体")
	require.Len(t, snapshot.Trend, 1)

	// 管理站渠道状态：按渠道（account_id）聚合全部流量（不只上架模型），带渠道身份；按模型筛选只算解析到它的请求
	channelRow := func(channels *service.ChannelMonitorV2Channels) service.ChannelMonitorV2ChannelRow {
		for _, row := range channels.Items {
			if row.AccountID == accountID {
				return row
			}
		}
		t.Fatalf("渠道 %d 不在渠道状态里", accountID)
		return service.ChannelMonitorV2ChannelRow{}
	}
	channels, err := repo.GetChannels(ctx, filter, cfg, "")
	require.NoError(t, err)
	all := channelRow(channels)
	require.Equal(t, "cmv2-acc", all.Name)
	require.Equal(t, "anthropic", all.Platform)
	require.Equal(t, int64(3), all.Metrics.RequestCount, "渠道状态统计全部流量，含没上架的 gpt-5.5")
	require.Equal(t, int64(3), channels.Metrics.RequestCount)
	require.Len(t, all.Buckets, 1)

	channels, err = repo.GetChannels(ctx, filter, cfg, "claude-fable-5")
	require.NoError(t, err)
	fable := channelRow(channels)
	require.Equal(t, int64(2), fable.Metrics.RequestCount)
	require.Equal(t, int64(2), fable.Metrics.TTFT.SampleCount)
	require.Equal(t, int64(2), channels.Metrics.RequestCount)
}

// newChannelMonitorV2UsageFixture 造一个用户、一把 key、一个渠道；返回的 insert 写一条成功用量（模型、时刻、首字延迟由调用方给）。
// 清理时删掉这些行，以及 models 在汇总表里的行。
func newChannelMonitorV2UsageFixture(t *testing.T, ctx context.Context, tag string, models ...string) func(requestID, model string, at time.Time, firstTokenMs int) {
	t.Helper()
	var userID, apiKeyID, accountID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, role, status, concurrency)
		VALUES ($1, 'x', 'user', 'active', 5) RETURNING id`, tag+"@example.com").Scan(&userID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO api_keys (user_id, key, name, status)
		VALUES ($1, $2, $2, 'active') RETURNING id`, userID, "sk-"+tag).Scan(&apiKeyID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, status, concurrency, credentials, extra, protocol_endpoints)
		VALUES ($1, 'openai', 'api_key', 'active', 1, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb)
		RETURNING id`, tag).Scan(&accountID))
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = integrationDB.ExecContext(bg, "DELETE FROM usage_logs WHERE account_id = $1", accountID)
		for _, table := range []string{
			"channel_monitor_v2_metrics_1m", "channel_monitor_v2_metrics_rollup",
			"channel_monitor_v2_error_metrics_1m", "channel_monitor_v2_error_metrics_rollup",
			"channel_monitor_v2_latency_histograms_1m", "channel_monitor_v2_latency_histograms_rollup",
		} {
			_, _ = integrationDB.ExecContext(bg, "DELETE FROM "+table+" WHERE model = ANY($1)", pq.Array(models))
		}
		_, _ = integrationDB.ExecContext(bg, "DELETE FROM accounts WHERE id = $1", accountID)
		_, _ = integrationDB.ExecContext(bg, "DELETE FROM api_keys WHERE id = $1", apiKeyID)
		_, _ = integrationDB.ExecContext(bg, "DELETE FROM users WHERE id = $1", userID)
	})
	return func(requestID, model string, at time.Time, firstTokenMs int) {
		_, err := integrationDB.ExecContext(ctx, `
			INSERT INTO usage_logs (
				request_id, user_id, api_key_id, account_id, model, requested_model,
				input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
				total_cost, actual_cost, rate_multiplier, billing_type, request_type,
				first_token_ms, duration_ms, created_at
			) VALUES ($1, $2, $3, $4, $5, $5, 10, 1, 0, 0, 0.01, 0.01, 1, 0, 2, $6, $6 + 100, $7)`,
			requestID, userID, apiKeyID, accountID, model, firstTokenMs, at)
		require.NoError(t, err)
	}
}

func channelMonitorV2TestRoster(models ...string) service.ChannelMonitorV2Config {
	return service.ChannelMonitorV2Config{Roster: service.ChannelMonitorV2ModelRoster{
		Models:  models,
		Resolve: func(model string) (string, bool) { return model, slices.Contains(models, model) },
	}}
}

// 聚合任务每分钟只重算最近 10 分钟，窗口几乎总落在同一个 12 小时段、同一天里。
// 12 小时、1 天两档（7 天 / 30 天读的就是它们）也必须把还没走完的当前段跟着重算，
// 否则这两个窗口停在上一个整 12 小时 / 整天，比 24 小时窗口还少请求（2026-10-04 走查）。
func TestChannelMonitorV2TrailingRefreshKeepsCoarseBucketsCurrent(t *testing.T) {
	ctx := context.Background()
	repo := NewChannelMonitorV2Repository(integrationDB)
	const model = "cmv2-coarse-model"
	insert := newChannelMonitorV2UsageFixture(t, ctx, "cmv2-coarse", model)
	// 昨天 UTC 零点：在保留窗口内，也和别的用例用的「现在附近」错开
	day := time.Now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)

	// 跨过 12 小时 / 天边界的一轮：00:03 的请求进了这两档的当天段
	insert("cmv2-coarse-1", model, day.Add(3*time.Minute), 1000)
	require.NoError(t, repo.RecomputeRange(ctx, day.Add(-5*time.Minute), day.Add(5*time.Minute)))
	// 当天中途的一轮：窗口 [05:00, 05:10) 落在同一个 12 小时段、同一天里
	insert("cmv2-coarse-2", model, day.Add(5*time.Hour+3*time.Minute), 2000)
	require.NoError(t, repo.RecomputeRange(ctx, day.Add(5*time.Hour), day.Add(5*time.Hour+10*time.Minute)))

	cfg := channelMonitorV2TestRoster(model)
	for _, bucket := range []time.Duration{time.Hour, 12 * time.Hour, 24 * time.Hour} {
		filter := service.ChannelMonitorV2Filter{Start: day, End: day.Add(24 * time.Hour), Bucket: bucket}
		snapshot, err := repo.GetSnapshot(ctx, filter, cfg)
		require.NoError(t, err)
		require.Equalf(t, int64(2), snapshot.Metrics.RequestCount, "段长 %s 的窗口要含到最近一轮", bucket)
		require.Equalf(t, int64(2), snapshot.Metrics.TTFT.SampleCount, "段长 %s 的首字延迟样本", bucket)
	}
}
