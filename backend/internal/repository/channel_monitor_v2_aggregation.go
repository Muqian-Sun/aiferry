package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

// Platform is derived from group/account (usage_logs has no provider column on upstream schema).
const channelMonitorV2PlatformSQL = `lower(` + usageLogEffectivePlatformExpr + `)`
const channelMonitorV2ModelSQL = `COALESCE(NULLIF(TRIM(ul.requested_model), ''), NULLIF(TRIM(ul.model), ''), 'unknown')`

// Tiered retention balances UI windows against storage:
//
//	1m facts  → short (late writes + rebuild rollups)
//	5m/1h/12h/1d rollups → longer, aligned to 90m / 24h / 7d / 30d(+audit)
//
// Backfill may still write short-lived 1m rows for old windows so rollups can be
// built; prune at end of each recompute drops them past their TTL while rollups remain.
const (
	channelMonitorV2RetentionMetrics1m   = 7 * 24 * time.Hour
	channelMonitorV2RetentionError1m     = 7 * 24 * time.Hour
	channelMonitorV2RetentionHistogram1m = 7 * 24 * time.Hour
	channelMonitorV2RetentionRollup5m    = 7 * 24 * time.Hour  // bucket_seconds=300
	channelMonitorV2RetentionRollup1h    = 30 * 24 * time.Hour // 3600
	channelMonitorV2RetentionRollup12h   = 45 * 24 * time.Hour // 43200
	channelMonitorV2RetentionRollup1d    = 90 * 24 * time.Hour // 86400
	channelMonitorV2RetentionMax         = channelMonitorV2RetentionRollup1d
)

// channelMonitorV2MaxRetention is the longest stored window (1d rollup). Used to
// clamp recompute/backfill so we never scan older than product history needs.
func channelMonitorV2MaxRetention() time.Duration {
	return channelMonitorV2RetentionMax
}

func channelMonitorV2RetentionCutoff(now time.Time, retention time.Duration) time.Time {
	return now.UTC().Truncate(time.Minute).Add(-retention)
}

type channelMonitorV2RetentionRule struct {
	table         string
	retention     time.Duration
	bucketSeconds int // 0 = fact table (no bucket_seconds column)
}

// channelMonitorV2RetentionRules is ordered coarse→fine for predictable prune plans.
var channelMonitorV2RetentionRules = []channelMonitorV2RetentionRule{
	{table: "channel_monitor_v2_metrics_1m", retention: channelMonitorV2RetentionMetrics1m},
	{table: "channel_monitor_v2_error_metrics_1m", retention: channelMonitorV2RetentionError1m},
	{table: "channel_monitor_v2_latency_histograms_1m", retention: channelMonitorV2RetentionHistogram1m},
	{table: "channel_monitor_v2_metrics_rollup", retention: channelMonitorV2RetentionRollup5m, bucketSeconds: 300},
	{table: "channel_monitor_v2_error_metrics_rollup", retention: channelMonitorV2RetentionRollup5m, bucketSeconds: 300},
	{table: "channel_monitor_v2_latency_histograms_rollup", retention: channelMonitorV2RetentionRollup5m, bucketSeconds: 300},
	{table: "channel_monitor_v2_metrics_rollup", retention: channelMonitorV2RetentionRollup1h, bucketSeconds: 3600},
	{table: "channel_monitor_v2_error_metrics_rollup", retention: channelMonitorV2RetentionRollup1h, bucketSeconds: 3600},
	{table: "channel_monitor_v2_latency_histograms_rollup", retention: channelMonitorV2RetentionRollup1h, bucketSeconds: 3600},
	{table: "channel_monitor_v2_metrics_rollup", retention: channelMonitorV2RetentionRollup12h, bucketSeconds: 43200},
	{table: "channel_monitor_v2_error_metrics_rollup", retention: channelMonitorV2RetentionRollup12h, bucketSeconds: 43200},
	{table: "channel_monitor_v2_latency_histograms_rollup", retention: channelMonitorV2RetentionRollup12h, bucketSeconds: 43200},
	{table: "channel_monitor_v2_metrics_rollup", retention: channelMonitorV2RetentionRollup1d, bucketSeconds: 86400},
	{table: "channel_monitor_v2_error_metrics_rollup", retention: channelMonitorV2RetentionRollup1d, bucketSeconds: 86400},
	{table: "channel_monitor_v2_latency_histograms_rollup", retention: channelMonitorV2RetentionRollup1d, bucketSeconds: 86400},
}

func (r *channelMonitorV2Repository) pruneChannelMonitorV2Retention(ctx context.Context, tx *sql.Tx, now time.Time) error {
	// During historical bootstrap, retain all 1m facts until the cursor reaches
	// the oldest rollup boundary. Otherwise adjacent chunks would rebuild the
	// same daily bucket from source rows already pruned by the prior chunk.
	var backfillCursor time.Time
	if err := tx.QueryRowContext(ctx, `SELECT backfill_cursor FROM channel_monitor_v2_watermarks WHERE id = 1`).Scan(&backfillCursor); err == nil && backfillCursor.After(channelMonitorV2RetentionCutoff(now, channelMonitorV2RetentionMax)) {
		return nil
	}
	for _, rule := range channelMonitorV2RetentionRules {
		cutoff := channelMonitorV2RetentionCutoff(now, rule.retention)
		var err error
		if rule.bucketSeconds == 0 {
			_, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE bucket_start < $1`, rule.table), cutoff)
		} else {
			_, err = tx.ExecContext(ctx,
				fmt.Sprintf(`DELETE FROM %s WHERE bucket_seconds = $1 AND bucket_start < $2`, rule.table),
				rule.bucketSeconds, cutoff,
			)
		}
		if err != nil {
			return fmt.Errorf("prune %s (bucket_seconds=%d): %w", rule.table, rule.bucketSeconds, err)
		}
	}
	return nil
}

func (r *channelMonitorV2Repository) RecomputeRange(ctx context.Context, start, end time.Time) (err error) {
	start = start.UTC().Truncate(time.Minute)
	end = end.UTC().Truncate(time.Minute)
	now := time.Now().UTC().Truncate(time.Minute)
	// Clamp to longest rollup TTL so backfill does not scan beyond product history.
	maxCutoff := channelMonitorV2RetentionCutoff(now, channelMonitorV2MaxRetention())
	if start.Before(maxCutoff) {
		start = maxCutoff
	}
	if !start.Before(end) {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Idempotent window rewrite: drop existing facts/rollups in [start,end) then re-insert.
	for _, table := range []string{
		"channel_monitor_v2_latency_histograms_rollup",
		"channel_monitor_v2_error_metrics_rollup",
		"channel_monitor_v2_metrics_rollup",
		"channel_monitor_v2_latency_histograms_1m",
		"channel_monitor_v2_error_metrics_1m",
		"channel_monitor_v2_metrics_1m",
	} {
		if _, err = tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE bucket_start >= $1 AND bucket_start < $2", table), start, end); err != nil {
			return err
		}
	}

	if _, err = tx.ExecContext(ctx, fmt.Sprintf(channelMonitorV2UsageMetricsSQL, channelMonitorV2PlatformSQL, channelMonitorV2ModelSQL), start, end); err != nil {
		return fmt.Errorf("aggregate channel monitor v2 usage: %w", err)
	}
	if _, err = tx.ExecContext(ctx, fmt.Sprintf(channelMonitorV2HistogramSQL, channelMonitorV2PlatformSQL, channelMonitorV2ModelSQL, channelMonitorV2HistogramBoundSQL("latency.value_ms")), start, end); err != nil {
		return fmt.Errorf("aggregate channel monitor v2 histograms: %w", err)
	}
	if _, err = tx.ExecContext(ctx, channelMonitorV2ErrorAggregationSQL, start, end); err != nil {
		return fmt.Errorf("aggregate channel monitor v2 errors: %w", err)
	}
	if err = r.recomputeFixedRollups(ctx, tx, start, end); err != nil {
		return err
	}
	// Drop rows past per-tier TTL (1m short, coarse rollups long). Safe after rollup
	// so a backfill chunk can build 1d rollups from temporary 1m rows then discard 1m.
	if err = r.pruneChannelMonitorV2Retention(ctx, tx, now); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, channelMonitorV2WatermarkSQL, start, end); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

// ttft_min_ms / ttft_max_ms 与首字延迟分布同一口径（成功请求、非负值），读数用它们收紧分布的首末两档。
const channelMonitorV2UsageMetricsSQL = `
INSERT INTO channel_monitor_v2_metrics_1m (
  bucket_start, platform, model, account_id, success_requests,
  input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
  ttft_sum_ms, ttft_count, ttft_min_ms, ttft_max_ms, duration_sum_ms, duration_count, computed_at
)
SELECT date_trunc('minute', ul.created_at), %s, %s, COALESCE(ul.account_id, 0),
       COUNT(DISTINCT COALESCE(NULLIF(ul.request_id, ''), 'usage:' || ul.id::text))
         FILTER (WHERE COALESCE(ul.request_type, 0) NOT IN (4, 6) AND ` + usageLogSuccessFilterUL + `),
       COALESCE(SUM(ul.input_tokens) FILTER (WHERE ` + usageLogSuccessFilterUL + `), 0),
       COALESCE(SUM(ul.output_tokens) FILTER (WHERE ` + usageLogSuccessFilterUL + `), 0),
       COALESCE(SUM(ul.cache_creation_tokens) FILTER (WHERE ` + usageLogSuccessFilterUL + `), 0),
       COALESCE(SUM(ul.cache_read_tokens) FILTER (WHERE ` + usageLogSuccessFilterUL + `), 0),
       COALESCE(SUM(ul.first_token_ms) FILTER (WHERE ul.first_token_ms IS NOT NULL AND ` + usageLogSuccessFilterUL + `), 0),
       COUNT(ul.first_token_ms) FILTER (WHERE ` + usageLogSuccessFilterUL + `),
       MIN(ul.first_token_ms) FILTER (WHERE ul.first_token_ms >= 0 AND ` + usageLogSuccessFilterUL + `),
       MAX(ul.first_token_ms) FILTER (WHERE ul.first_token_ms >= 0 AND ` + usageLogSuccessFilterUL + `),
       COALESCE(SUM(ul.duration_ms) FILTER (WHERE ul.duration_ms IS NOT NULL AND ` + usageLogSuccessFilterUL + `), 0),
       COUNT(ul.duration_ms) FILTER (WHERE ` + usageLogSuccessFilterUL + `), NOW()
FROM usage_logs ul
LEFT JOIN accounts a ON a.id = ul.account_id
WHERE ul.created_at >= $1 AND ul.created_at < $2
GROUP BY 1, 2, 3, 4`

const channelMonitorV2HistogramSQL = `
INSERT INTO channel_monitor_v2_latency_histograms_1m (
  bucket_start, platform, model, account_id, metric, upper_bound_ms, sample_count
)
SELECT date_trunc('minute', ul.created_at), %s, %s, COALESCE(ul.account_id, 0),
       latency.metric, %s, COUNT(*)
FROM usage_logs ul
LEFT JOIN accounts a ON a.id = ul.account_id
CROSS JOIN LATERAL (VALUES ('ttft'::text, ul.first_token_ms), ('duration'::text, ul.duration_ms)) latency(metric, value_ms)
WHERE ul.created_at >= $1 AND ul.created_at < $2
  AND latency.value_ms IS NOT NULL AND latency.value_ms >= 0
  AND ` + usageLogSuccessFilterUL + `
GROUP BY 1, 2, 3, 4, 5, 6`

// channelMonitorV2LatencyBoundsMs 延迟分布各档的上沿（毫秒）：一档覆盖（上一档上沿, 本档上沿]，第一档从 0 起；
// 超过最后一档的样本记在 channelMonitorV2LatencyOverflowMs。汇总 SQL 分档与读数插值共用这一份。
var channelMonitorV2LatencyBoundsMs = []int64{50, 100, 250, 500, 1000, 2000, 3000, 5000, 8000, 10000, 15000, 30000, 60000, 120000, 300000, 600000}

// channelMonitorV2LatencyOverflowMs 超出最后一档的样本的档位值（upper_bound_ms 是 integer 列，取它的上限）。
const channelMonitorV2LatencyOverflowMs int64 = math.MaxInt32

func channelMonitorV2HistogramBoundSQL(column string) string {
	var b strings.Builder
	b.WriteString("CASE")
	for _, bound := range channelMonitorV2LatencyBoundsMs {
		fmt.Fprintf(&b, " WHEN %s <= %d THEN %d", column, bound, bound)
	}
	fmt.Fprintf(&b, " ELSE %d END", channelMonitorV2LatencyOverflowMs)
	return b.String()
}

// channelMonitorV2LatencyLowerBound 一档的下沿 = 档位表里上一档的上沿；第一档从 0 起。
func channelMonitorV2LatencyLowerBound(upper int64) int64 {
	var lower int64
	for _, bound := range channelMonitorV2LatencyBoundsMs {
		if bound >= upper {
			break
		}
		lower = bound
	}
	return lower
}

// Error dedup lookback: request_id branch is bounded by chunk start minus 90
// minutes so candidate_ids never forces a full-history scan of ops_error_logs.
const channelMonitorV2ErrorAggregationSQL = `
WITH dedup AS (
  WITH candidate_ids AS MATERIALIZED (
    SELECT DISTINCT request_id
    FROM ops_error_logs
    WHERE created_at >= $1 AND created_at < $2 AND NULLIF(request_id, '') IS NOT NULL
  )
  SELECT DISTINCT ON (COALESCE(NULLIF(current_error.request_id, ''), 'error:' || current_error.id::text))
    date_trunc('minute', current_error.created_at) AS bucket_start,
    -- Composite groups are a routing layer: resolve the concrete account
    -- platform (mirrors usageLogEffectivePlatformExpr on the usage side) so
    -- error facts share the usage facts' platform key. Without this, composite
    lower(COALESCE(NULLIF(TRIM(current_error.platform), ''), NULLIF(TRIM(a.platform), ''), 'unknown')) AS platform,
    COALESCE(NULLIF(TRIM(current_error.requested_model), ''), NULLIF(TRIM(current_error.model), ''), 'unknown') AS model,
    -- 0 = 没选到渠道就失败（如没有可用渠道）
    COALESCE(current_error.account_id, 0) AS account_id,
    current_error.error_type, current_error.error_owner, COALESCE(current_error.status_code, 0) AS status_code,
    COALESCE(current_error.upstream_status_code, 0) AS upstream_status_code,
    lower(CONCAT_WS(' ', current_error.error_type, current_error.error_source, current_error.error_message, current_error.upstream_error_message, current_error.upstream_error_detail, current_error.error_body)) AS text,
    (CASE WHEN jsonb_typeof(current_error.upstream_errors) = 'array' THEN jsonb_array_length(current_error.upstream_errors) > 0 ELSE FALSE END
      OR current_error.error_owner = 'provider' OR current_error.upstream_status_code IS NOT NULL) AS upstream_affected,
    CASE WHEN jsonb_typeof(current_error.upstream_errors) = 'array' THEN jsonb_array_length(current_error.upstream_errors) ELSE 0 END AS upstream_attempts
  FROM ops_error_logs current_error
  LEFT JOIN accounts a ON a.id = current_error.account_id
  WHERE (
      (NULLIF(current_error.request_id, '') IS NULL AND current_error.created_at >= $1 AND current_error.created_at < $2)
      OR (
        current_error.request_id IN (SELECT request_id FROM candidate_ids)
        AND current_error.created_at >= $1 - INTERVAL '90 minutes'
        AND current_error.created_at < $2
      )
    )
    AND NOT current_error.is_count_tokens
    AND (COALESCE(current_error.status_code, 0) >= 400 OR current_error.error_type = 'cyber_policy')
  ORDER BY COALESCE(NULLIF(current_error.request_id, ''), 'error:' || current_error.id::text), current_error.created_at DESC, current_error.id DESC
), classified AS (
  SELECT *, CASE
    -- Keep in lockstep with service.ClassifyChannelMonitorV2Error needles.
    WHEN error_type = 'cyber_policy' OR text LIKE ANY(ARRAY['%content policy%','%content_policy%','%safety policy%','%moderation%','%blocked keyword%']) THEN 'content_policy'
    WHEN status_code = 401 OR upstream_status_code = 401 OR text LIKE ANY(ARRAY['%unauthorized%','%invalid api key%','%invalid_api_key%','%authentication%','%api_key_disabled%']) THEN 'authentication'
    WHEN text LIKE ANY(ARRAY['%context window%','%context length%','%maximum prompt length%','%too many tokens%','%max_tokens%']) THEN 'context_limit'
    WHEN text LIKE ANY(ARRAY['%failed to deserialize%','%missing required parameter%','%invalid request%','%invalid_request%','%tool_choice%']) THEN 'invalid_request'
    WHEN text LIKE ANY(ARRAY['%does not support the requested model%','%not supported by any configured account%','%model not supported%','%unsupported model%']) THEN 'model_unsupported'
    WHEN text LIKE ANY(ARRAY['%run out of credits%','%insufficient balance%','%insufficient quota%','%subscription%','%quota exceeded%','%billing hard limit%']) THEN 'quota_or_balance'
    WHEN text LIKE ANY(ARRAY['%no available accounts%','%no healthy account%','%no healthy upstream account%','%failover budget exhausted%','%account pool%']) THEN 'account_pool_unavailable'
    WHEN status_code = 429 OR upstream_status_code = 429 OR text LIKE ANY(ARRAY['%rate limit%','%rate_limit%','%high demand%','%overloaded%','%concurrency limit%','%capacity%']) THEN 'rate_or_capacity'
    WHEN status_code IN (408,504) OR text LIKE ANY(ARRAY['%timeout%','%deadline exceeded%','%error code: 524%','%gateway time-out%','%gateway timeout%']) THEN 'timeout'
    WHEN text LIKE ANY(ARRAY['%transport%','%stream_read_error%','%connection reset%','%connection refused%','%tls%','%http2%','%missing terminal event%','%unexpected eof%']) THEN 'transport_or_stream'
    WHEN status_code = 403 OR upstream_status_code = 403 THEN 'upstream_forbidden'
    WHEN status_code = 404 OR upstream_status_code = 404 THEN 'not_found'
    WHEN status_code = 499 OR text LIKE ANY(ARRAY['%client cancelled%','%client canceled%','%context canceled%']) THEN 'client_cancelled'
    WHEN upstream_status_code >= 500 OR (error_owner = 'provider' AND status_code >= 500) THEN 'upstream_5xx'
    WHEN status_code >= 500 OR error_type = 'internal' OR error_owner = 'system' THEN 'internal'
    ELSE 'other' END AS category
  FROM dedup
  WHERE bucket_start >= $1 AND bucket_start < $2
), metric_rows AS (
  INSERT INTO channel_monitor_v2_metrics_1m (bucket_start, platform, model, account_id, error_requests, upstream_affected_requests, upstream_attempt_count, computed_at)
  SELECT bucket_start, platform, model, account_id, COUNT(*), COUNT(*) FILTER (WHERE upstream_affected), SUM(upstream_attempts), NOW()
  FROM classified GROUP BY 1,2,3,4
  ON CONFLICT (bucket_start, platform, model, account_id) DO UPDATE SET
    error_requests = EXCLUDED.error_requests, upstream_affected_requests = EXCLUDED.upstream_affected_requests,
    upstream_attempt_count = EXCLUDED.upstream_attempt_count, computed_at = NOW()
)
INSERT INTO channel_monitor_v2_error_metrics_1m (bucket_start, platform, model, account_id, error_category, taxonomy_version, error_requests)
SELECT bucket_start, platform, model, account_id, category, 1, COUNT(*) FROM classified GROUP BY 1,2,3,4,5
ON CONFLICT (bucket_start, platform, model, account_id, error_category, taxonomy_version)
DO UPDATE SET error_requests = EXCLUDED.error_requests`

// Floor matches channelMonitorV2RetentionMax (90d). Keep the INTERVAL literal in
// sync when changing channelMonitorV2RetentionRollup1d.
//
// Coverage starts track how far back recompute has walked ($1 = chunk start), not
// "min(source_log.created_at)". Using global min(ops_error_logs) pins
// error_coverage_start to the first real error forever and collapses UI windows
// when errors only exist in a recent slice (common on first upgrade).
const channelMonitorV2WatermarkSQL = `
INSERT INTO channel_monitor_v2_watermarks (id, usage_coverage_start, error_coverage_start, data_through, last_successful_at, backfill_cursor, updated_at)
VALUES (
  1,
  $1,
  $1,
  $2, NOW(), $1, NOW()
)
ON CONFLICT (id) DO UPDATE SET
  usage_coverage_start = GREATEST(
    date_trunc('minute', NOW()) - INTERVAL '90 days',
    LEAST(COALESCE(channel_monitor_v2_watermarks.usage_coverage_start, EXCLUDED.usage_coverage_start), EXCLUDED.usage_coverage_start)
  ),
  error_coverage_start = GREATEST(
    date_trunc('minute', NOW()) - INTERVAL '90 days',
    LEAST(COALESCE(channel_monitor_v2_watermarks.error_coverage_start, EXCLUDED.error_coverage_start), EXCLUDED.error_coverage_start)
  ),
  data_through = GREATEST(COALESCE(channel_monitor_v2_watermarks.data_through, EXCLUDED.data_through), EXCLUDED.data_through),
  last_successful_at = NOW(),
  backfill_cursor = LEAST(COALESCE(channel_monitor_v2_watermarks.backfill_cursor, EXCLUDED.backfill_cursor), EXCLUDED.backfill_cursor),
  updated_at = NOW()`

// channelMonitorV2RollupTier 一档固定粒度汇总：段长与来源（sourceSeconds 为 0 = 1m 事实表，否则 = 同表里更细的一档）。
type channelMonitorV2RollupTier struct{ seconds, sourceSeconds int }

// channelMonitorV2RollupTiers 各档按顺序在同一个事务里重算，来源档必须排在前面。
//
// 每次重算都要重建窗口碰到的各档段，包括还没走完的当前段：7 天 / 30 天读的就是 12 小时 / 1 天这两档，
// 原来只在窗口跨过档边界时才重建，这两个窗口就一直停在上一个 UTC 整 12 小时 / 整天（2026-10-04 走查：
// 24 小时有 3 个请求、7 天只有 2 个）。12 小时、1 天改从 1 小时档合并：当前这一天每轮只合并几十行，
// 不用每分钟把一整天的分钟行重扫一遍；1 小时档保留 30 天，覆盖得到任何会被重建的段。
var channelMonitorV2RollupTiers = []channelMonitorV2RollupTier{
	{seconds: 300},
	{seconds: 3600},
	{seconds: 43200, sourceSeconds: 3600},
	{seconds: 86400, sourceSeconds: 3600},
}

func (r *channelMonitorV2Repository) recomputeFixedRollups(ctx context.Context, tx *sql.Tx, start, end time.Time) error {
	for _, tier := range channelMonitorV2RollupTiers {
		interval := fmt.Sprintf("%d seconds", tier.seconds)
		for _, table := range []string{
			"channel_monitor_v2_latency_histograms_rollup",
			"channel_monitor_v2_error_metrics_rollup",
			"channel_monitor_v2_metrics_rollup",
		} {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(channelMonitorV2FixedRollupDeleteSQL, table), interval, tier.seconds, start, end); err != nil {
				return err
			}
		}
		for _, step := range []struct{ name, query string }{
			{"metrics", channelMonitorV2MetricsRollupSQL(tier.sourceSeconds)},
			{"histograms", channelMonitorV2HistogramRollupSQL(tier.sourceSeconds)},
			{"errors", channelMonitorV2ErrorRollupSQL(tier.sourceSeconds)},
		} {
			if _, err := tx.ExecContext(ctx, step.query, interval, tier.seconds, start, end); err != nil {
				return fmt.Errorf("roll up channel monitor v2 %s %ds: %w", step.name, tier.seconds, err)
			}
		}
	}
	return nil
}

// channelMonitorV2RollupFrom 一档汇总的 FROM … WHERE 前缀：1m 事实表，或同一张 rollup 表里 sourceSeconds 那一档。
func channelMonitorV2RollupFrom(factTable, rollupTable, alias string, sourceSeconds int) string {
	if sourceSeconds == 0 {
		return factTable + " " + alias + ", bounds WHERE "
	}
	return fmt.Sprintf("%s %s, bounds WHERE %s.bucket_seconds = %d AND ", rollupTable, alias, alias, sourceSeconds)
}

// PostgreSQL interprets a TIMESTAMPTZ literal without an explicit offset in
// the current session timezone. Keep date_bin's origin fixed in UTC so bucket
// boundaries do not shift when the database session runs in Asia/Shanghai (or
// any other non-UTC timezone).
const channelMonitorV2DateBinOrigin = "TIMESTAMPTZ '1970-01-01 00:00:00+00'"

func channelMonitorV2DateBinExpr(column string) string {
	return "date_bin($1::interval," + column + "," + channelMonitorV2DateBinOrigin + ")"
}

const channelMonitorV2FixedRollupBoundsSQL = `
WITH bounds AS (
  SELECT
    date_bin($1::interval, $3::timestamptz, ` + channelMonitorV2DateBinOrigin + `) AS start_at,
    date_bin($1::interval, $4::timestamptz - INTERVAL '1 microsecond', ` + channelMonitorV2DateBinOrigin + `) + $1::interval AS end_at
)`

const channelMonitorV2FixedRollupDeleteSQL = channelMonitorV2FixedRollupBoundsSQL + `
DELETE FROM %s
USING bounds
WHERE bucket_seconds = $2::integer
  AND bucket_start >= bounds.start_at
  AND bucket_start < bounds.end_at`

func channelMonitorV2MetricsRollupSQL(sourceSeconds int) string {
	return `
INSERT INTO channel_monitor_v2_metrics_rollup (
  bucket_start, bucket_seconds, platform, model, account_id, success_requests, error_requests,
  upstream_affected_requests, upstream_attempt_count, input_tokens, output_tokens,
  cache_creation_tokens, cache_read_tokens, ttft_sum_ms, ttft_count, ttft_min_ms, ttft_max_ms,
  duration_sum_ms, duration_count, computed_at
)
` + channelMonitorV2FixedRollupBoundsSQL + `
SELECT date_bin($1::interval, m.bucket_start, ` + channelMonitorV2DateBinOrigin + `), $2::integer,
       platform, model, account_id, SUM(success_requests), SUM(error_requests),
       SUM(upstream_affected_requests), SUM(upstream_attempt_count), SUM(input_tokens),
       SUM(output_tokens), SUM(cache_creation_tokens), SUM(cache_read_tokens),
       SUM(ttft_sum_ms), SUM(ttft_count), MIN(ttft_min_ms), MAX(ttft_max_ms),
       SUM(duration_sum_ms), SUM(duration_count), NOW()
FROM ` + channelMonitorV2RollupFrom("channel_monitor_v2_metrics_1m", "channel_monitor_v2_metrics_rollup", "m", sourceSeconds) + `m.bucket_start >= bounds.start_at AND m.bucket_start < bounds.end_at
GROUP BY 1, 2, 3, 4, 5`
}

func channelMonitorV2HistogramRollupSQL(sourceSeconds int) string {
	return `
INSERT INTO channel_monitor_v2_latency_histograms_rollup (
  bucket_start, bucket_seconds, platform, model, account_id, metric, upper_bound_ms, sample_count
)
` + channelMonitorV2FixedRollupBoundsSQL + `
SELECT date_bin($1::interval, h.bucket_start, ` + channelMonitorV2DateBinOrigin + `), $2::integer,
       platform, model, account_id, metric, upper_bound_ms, SUM(sample_count)
FROM ` + channelMonitorV2RollupFrom("channel_monitor_v2_latency_histograms_1m", "channel_monitor_v2_latency_histograms_rollup", "h", sourceSeconds) + `h.bucket_start >= bounds.start_at AND h.bucket_start < bounds.end_at
GROUP BY 1, 2, 3, 4, 5, 6, 7`
}

func channelMonitorV2ErrorRollupSQL(sourceSeconds int) string {
	return `
INSERT INTO channel_monitor_v2_error_metrics_rollup (
  bucket_start, bucket_seconds, platform, model, account_id, error_category, taxonomy_version, error_requests
)
` + channelMonitorV2FixedRollupBoundsSQL + `
SELECT date_bin($1::interval, e.bucket_start, ` + channelMonitorV2DateBinOrigin + `), $2::integer,
       platform, model, account_id, error_category, taxonomy_version, SUM(error_requests)
FROM ` + channelMonitorV2RollupFrom("channel_monitor_v2_error_metrics_1m", "channel_monitor_v2_error_metrics_rollup", "e", sourceSeconds) + `e.bucket_start >= bounds.start_at AND e.bucket_start < bounds.end_at
GROUP BY 1, 2, 3, 4, 5, 6, 7`
}
