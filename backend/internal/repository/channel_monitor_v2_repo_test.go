package repository

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2DateBinOriginIsUTC(t *testing.T) {
	require.Equal(t, "TIMESTAMPTZ '1970-01-01 00:00:00+00'", channelMonitorV2DateBinOrigin)
	require.Equal(t, "date_bin($1::interval,m.bucket_start,TIMESTAMPTZ '1970-01-01 00:00:00+00')", channelMonitorV2DateBinExpr("m.bucket_start"))

	queries := []string{channelMonitorV2FixedRollupBoundsSQL}
	for _, tier := range channelMonitorV2RollupTiers {
		queries = append(queries,
			channelMonitorV2MetricsRollupSQL(tier.sourceSeconds),
			channelMonitorV2HistogramRollupSQL(tier.sourceSeconds),
			channelMonitorV2ErrorRollupSQL(tier.sourceSeconds),
		)
	}
	for _, query := range queries {
		require.Contains(t, query, channelMonitorV2DateBinOrigin)
		require.NotContains(t, query, "TIMESTAMPTZ '1970-01-01'")
	}
}

// 服务状态按上架名单统计：请求名（含别名）归到名单里的本名，平台不参与；
// 名单外（没上架的名字、没解析出模型的失败请求）不计；SQL 不按平台筛。
func TestChannelMonitorV2RosterModelIgnoresPlatform(t *testing.T) {
	cfg := service.ChannelMonitorV2Config{Roster: service.ChannelMonitorV2ModelRoster{
		Models: []string{"gpt-5.5", "glm-5.3-flash"},
		Resolve: func(model string) (string, bool) {
			switch model {
			case "gpt-5.5", "glm-5.3-flash":
				return model, true
			case "gpt-latest":
				return "gpt-5.5", true
			}
			return "", false
		},
	}}
	got, ok := channelMonitorV2RosterModel(cfg, " gpt-latest ")
	require.True(t, ok)
	require.Equal(t, "gpt-5.5", got)
	_, ok = channelMonitorV2RosterModel(cfg, "gpt-4o")
	require.False(t, ok)
	_, ok = channelMonitorV2RosterModel(cfg, "unknown")
	require.False(t, ok)
	_, ok = channelMonitorV2RosterModel(cfg, "  ")
	require.False(t, ok)

	where, args, _ := channelMonitorV2WhereWithRollup(service.ChannelMonitorV2Filter{Bucket: time.Minute}, "m")
	require.NotContains(t, where, "platform")
	require.Len(t, args, 2)
}

// 首字延迟分位数在档内线性插值，档的范围用实测最小 / 最大值收紧（2026-10-04 走查：只有一个 146 秒的请求显示「300 s」，
// 中位数显示档位上沿「3.0 s」）。
func TestChannelMonitorV2HistogramPercentilesInterpolateWithinObservedRange(t *testing.T) {
	// dev 库 gpt-5.5 近 24 小时的 16 个首字延迟（毫秒）：5, 689, 796, 1120, 1270, 1747, 1908, 2042, 2383, 2766,
	// 3297, 3355, 4061, 4499, 17183, 24685，按档计数如下
	hist := map[int64]int64{50: 1, 1000: 2, 2000: 4, 3000: 3, 5000: 4, 30000: 2}
	observed := latencyRange{min: 5, max: 24685, ok: true}
	// P50：第 8 个样本落在 (2000, 3000] 档、前面已有 7 个 → 2000 + 1000 × 1/3（实际中位数约 2.2 秒）
	require.Equal(t, int64(2333), *histPercentile(hist, .5, observed))
	// P90：第 14.4 个落在 (15000, 30000] 档、前面已有 14 个，档上沿收紧到最大值 24685 → 15000 + 9685 × 0.4/2
	require.Equal(t, int64(16937), *histPercentile(hist, .9, observed))

	// 只有一个样本：结果就是它本身，不是所在档 (120000, 300000] 的上沿
	single := latencyRange{min: 146278, max: 146278, ok: true}
	one := map[int64]int64{300000: 1}
	require.Equal(t, int64(146278), *histPercentile(one, .5, single))
	require.Equal(t, int64(146278), *histPercentile(one, .95, single))
	// 不知道实测范围（耗时没有记）时只插值
	require.Equal(t, int64(210000), *histPercentile(one, .5, latencyRange{}))
	// 超出最后一档：知道最大值就收紧到它，不知道取最后一档的上沿
	overflow := map[int64]int64{channelMonitorV2LatencyOverflowMs: 1}
	require.Equal(t, int64(700000), *histPercentile(overflow, .5, latencyRange{min: 700000, max: 700000, ok: true}))
	require.Equal(t, int64(600000), *histPercentile(overflow, .5, latencyRange{}))
	require.Nil(t, histPercentile(nil, .95, latencyRange{}))

	// latencyMetric 给出平均与 P50 / P90 / P95，分位数用的是传进来的实测范围
	lat := latencyMetric(146278, 1, one, single)
	require.Equal(t, 146278.0, *lat.AvgMs)
	require.Equal(t, int64(146278), *lat.P50Ms)
	require.Equal(t, int64(146278), *lat.P90Ms)
	require.Equal(t, int64(146278), *lat.P95Ms)
}

// 实测范围按 SQL 的 MIN / MAX 合并；没有样本的行（两列为空）不参与。
func TestChannelMonitorV2LatencyRangeMergesAcrossRows(t *testing.T) {
	var r latencyRange
	r.merge(sql.NullInt64{}, sql.NullInt64{})
	require.False(t, r.ok)
	r.merge(sql.NullInt64{Int64: 3000, Valid: true}, sql.NullInt64{Int64: 9000, Valid: true})
	r.merge(sql.NullInt64{Int64: 1200, Valid: true}, sql.NullInt64{Int64: 4000, Valid: true})
	r.merge(sql.NullInt64{}, sql.NullInt64{})
	require.Equal(t, latencyRange{min: 1200, max: 9000, ok: true}, r)
}

func TestChannelMonitorV2MetricIncludesSuccessRate(t *testing.T) {
	acc := newMetricAccumulator()
	acc.success, acc.errors = 80, 20
	metric := acc.metric(1)
	require.Equal(t, int64(100), metric.RequestCount)
	require.InDelta(t, 0.8, metric.SuccessRate, 0.0001)
	require.InDelta(t, 0.2, metric.ErrorRate, 0.0001)
}

func TestChannelMonitorV2ErrorAggregationCountsFinalUserErrorsOnly(t *testing.T) {
	query := strings.ToLower(channelMonitorV2ErrorAggregationSQL)
	require.Contains(t, query, "not current_error.is_count_tokens")
	require.Contains(t, query, "error_type = 'cyber_policy'")
	require.Contains(t, query, "distinct on")
	require.Contains(t, query, "candidate_ids")
	require.Contains(t, query, "where bucket_start >= $1 and bucket_start < $2")
	require.Contains(t, query, "upstream_affected_requests")
	require.Contains(t, query, "jsonb_array_length(current_error.upstream_errors) > 0")
	// request_id dedup must be time-bounded (no full-history scan).
	require.Contains(t, query, "interval '90 minutes'")
	require.Contains(t, query, "current_error.created_at >= $1 - interval '90 minutes'")
}

func TestChannelMonitorV2ErrorAggregationResolvesAccountPlatform(t *testing.T) {
	query := strings.ToLower(channelMonitorV2ErrorAggregationSQL)
	// 错误事实的平台与用量事实同一口径：先用错误行自带的 platform，缺失时回落到
	// 承接该请求的账号平台（分组已删，不再有路由层的平台覆写）。
	require.Contains(t, query, "left join accounts a on a.id = current_error.account_id")
	require.NotContains(t, query, "left join groups")
	require.NotContains(t, query, "'composite'")
	require.Contains(t, query, "nullif(trim(a.platform), '')")
	require.NotContains(t, query, "nullif(trim(a.platform))")
}

func TestChannelMonitorV2UsageSuccessExcludesCyberBillingRows(t *testing.T) {
	for _, query := range []string{channelMonitorV2UsageMetricsSQL} {
		require.Contains(t, query, "COALESCE(ul.request_type, 0) NOT IN (4, 6)")
		require.Contains(t, query, "ul.actual_cost > 0")
	}
	// 用量行的平台只看承接它的账号（分组已删）。
	require.Equal(t, "lower(a.platform)", channelMonitorV2PlatformSQL)
	require.Contains(t, channelMonitorV2HistogramSQL, "ul.actual_cost > 0")
}

func TestChannelMonitorV2RatesUseCoveredWindow(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	filter := service.ChannelMonitorV2Filter{Start: start, End: start.Add(24 * time.Hour)}
	coverage := service.ChannelMonitorV2Coverage{CoverageStart: start.Add(6 * time.Hour), DataThrough: start.Add(18 * time.Hour)}
	require.Equal(t, 12*60.0, channelMonitorV2CoveredMinutes(filter, coverage))
	effective := channelMonitorV2CommonCoverageFilter(filter, coverage)
	require.Equal(t, coverage.CoverageStart, effective.Start)
	require.Equal(t, coverage.DataThrough, effective.End)
}

func TestChannelMonitorV2TierRetentionPolicy(t *testing.T) {
	require.Equal(t, 7*24*time.Hour, channelMonitorV2RetentionMetrics1m)
	require.Equal(t, 7*24*time.Hour, channelMonitorV2RetentionError1m)
	require.Equal(t, 7*24*time.Hour, channelMonitorV2RetentionHistogram1m)
	require.Equal(t, 7*24*time.Hour, channelMonitorV2RetentionRollup5m)
	require.Equal(t, 30*24*time.Hour, channelMonitorV2RetentionRollup1h)
	require.Equal(t, 45*24*time.Hour, channelMonitorV2RetentionRollup12h)
	require.Equal(t, 90*24*time.Hour, channelMonitorV2RetentionRollup1d)
	require.Equal(t, channelMonitorV2RetentionRollup1d, channelMonitorV2MaxRetention())
	require.Contains(t, channelMonitorV2WatermarkSQL, "INTERVAL '90 days'")

	// Every fixed rollup second must appear with a retention rule.
	wantSeconds := map[int]time.Duration{
		300:   channelMonitorV2RetentionRollup5m,
		3600:  channelMonitorV2RetentionRollup1h,
		43200: channelMonitorV2RetentionRollup12h,
		86400: channelMonitorV2RetentionRollup1d,
	}
	seen := map[int]time.Duration{}
	for _, rule := range channelMonitorV2RetentionRules {
		if rule.bucketSeconds == 0 {
			require.True(t, rule.retention > 0)
			continue
		}
		if prev, ok := seen[rule.bucketSeconds]; ok {
			require.Equal(t, prev, rule.retention)
		}
		seen[rule.bucketSeconds] = rule.retention
	}
	for seconds, want := range wantSeconds {
		got, ok := seen[seconds]
		require.Truef(t, ok, "missing retention rule for bucket_seconds=%d", seconds)
		require.Equal(t, want, got)
	}

	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	require.Equal(t, now.Add(-7*24*time.Hour), channelMonitorV2RetentionCutoff(now, channelMonitorV2RetentionMetrics1m))
	require.Equal(t, now.Add(-90*24*time.Hour), channelMonitorV2RetentionCutoff(now, channelMonitorV2MaxRetention()))
}

// Needles present in service.ClassifyChannelMonitorV2Error must appear in the
// aggregation SQL CASE so rollup categories match drilldown classification.
func TestChannelMonitorV2SQLTaxonomyContainsGoNeedles(t *testing.T) {
	sql := channelMonitorV2ErrorAggregationSQL
	needles := []string{
		"blocked keyword",
		"invalid_api_key",
		"max_tokens",
		"invalid_request",
		"model not supported",
		"billing hard limit",
		"no healthy upstream account",
		"rate_limit",
		"gateway timeout",
		"connection refused",
		"unexpected eof",
	}
	for _, needle := range needles {
		require.Containsf(t, strings.ToLower(sql), strings.ToLower(needle), "SQL taxonomy missing Go needle %q", needle)
	}
}

func TestApplyIgnoredErrorsAdjustsRatesKeepsAbsoluteVolume(t *testing.T) {
	m := service.ChannelMonitorV2Metric{
		RequestCount:  100,
		ErrorRequests: 20,
		ErrorRate:     0.20,
		SuccessRate:   0.80,
	}
	// Success absolute still 80 → success rate stays 0.80 even after ignoring 5 errors.
	m.SuccessRequests = 80
	applyIgnoredErrors(&m, 5)
	require.Equal(t, int64(100), m.RequestCount)
	require.Equal(t, int64(20), m.ErrorRequests)
	require.InDelta(t, 0.15, m.ErrorRate, 0.0001)
	require.InDelta(t, 0.80, m.SuccessRate, 0.0001)

	// Clamp ignored > errors: scored error_rate → 0; success stays true ratio.
	m2 := service.ChannelMonitorV2Metric{RequestCount: 10, ErrorRequests: 2, SuccessRequests: 8, ErrorRate: 0.2, SuccessRate: 0.8}
	applyIgnoredErrors(&m2, 99)
	require.InDelta(t, 0.0, m2.ErrorRate, 0.0001)
	require.InDelta(t, 0.8, m2.SuccessRate, 0.0001)

	// No-op when ignored is zero
	m3 := service.ChannelMonitorV2Metric{RequestCount: 10, ErrorRequests: 2, SuccessRequests: 8, ErrorRate: 0.2, SuccessRate: 0.8}
	applyIgnoredErrors(&m3, 0)
	require.InDelta(t, 0.2, m3.ErrorRate, 0.0001)
	require.InDelta(t, 0.8, m3.SuccessRate, 0.0001)
}
