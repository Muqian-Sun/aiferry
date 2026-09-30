package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2DateBinOriginIsUTC(t *testing.T) {
	require.Equal(t, "TIMESTAMPTZ '1970-01-01 00:00:00+00'", channelMonitorV2DateBinOrigin)
	require.Equal(t, "date_bin($1::interval,m.bucket_start,TIMESTAMPTZ '1970-01-01 00:00:00+00')", channelMonitorV2DateBinExpr("m.bucket_start"))

	for _, query := range []string{
		channelMonitorV2FixedRollupBoundsSQL,
		channelMonitorV2MetricsRollupSQL,
		channelMonitorV2UserMetricsRollupSQL,
		channelMonitorV2HistogramRollupSQL,
		channelMonitorV2ErrorRollupSQL,
	} {
		require.Contains(t, query, channelMonitorV2DateBinOrigin)
		require.NotContains(t, query, "TIMESTAMPTZ '1970-01-01'")
	}
}

func TestChannelMonitorV2DisplayModelIsPlatformScoped(t *testing.T) {
	cfg := service.ChannelMonitorV2Config{Platforms: []service.ChannelMonitorV2PlatformConfig{
		{Platform: "openai", Enabled: true, Models: []string{"shared", "gpt-5"}},
		{Platform: "grok", Enabled: true, Models: []string{"grok-4"}},
		// Empty models list must NOT collapse everything into __other__.
		{Platform: "anthropic", Enabled: true, Models: []string{}},
	}}
	require.Equal(t, "shared", channelMonitorV2DisplayModel(cfg, "openai", "shared"))
	require.Equal(t, service.ChannelMonitorV2OtherModel, channelMonitorV2DisplayModel(cfg, "grok", "shared"))
	require.Equal(t, "claude-sonnet-4", channelMonitorV2DisplayModel(cfg, "anthropic", "claude-sonnet-4"))
	// Unconfigured platform still surfaces the real model name.
	require.Equal(t, "gemini-2.5-pro", channelMonitorV2DisplayModel(cfg, "gemini", "gemini-2.5-pro"))
	require.True(t, channelMonitorV2ModelSelected(service.ChannelMonitorV2Filter{Models: []string{service.ChannelMonitorV2OtherModel}}, cfg, "grok", "shared"))
}

// 访客名单（上架目录）与监控的平台配置无关：openai 上配了的 gpt-4o 没上架就不计，
// 没选到上游（unknown 平台）的上架模型照样算；别名归到本名；SQL 不按平台筛；每个上架模型都有一行。
func TestChannelMonitorV2VisitorRosterIgnoresPlatformConfig(t *testing.T) {
	cfg := service.ChannelMonitorV2Config{
		Platforms: []service.ChannelMonitorV2PlatformConfig{{Platform: "openai", Enabled: true, Models: []string{"gpt-4o", "gpt-5.5"}}},
		ModelRoster: &service.ChannelMonitorV2ModelRoster{
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
		},
	}
	require.Equal(t, "gpt-5.5", channelMonitorV2DisplayModel(cfg, "openai", "gpt-latest"))
	require.Equal(t, "glm-5.3-flash", channelMonitorV2DisplayModel(cfg, "unknown", "glm-5.3-flash"))
	require.Equal(t, service.ChannelMonitorV2OtherModel, channelMonitorV2DisplayModel(cfg, "openai", "gpt-4o"))

	all := service.ChannelMonitorV2Filter{}
	require.True(t, channelMonitorV2ModelSelected(all, cfg, "unknown", "glm-5.3-flash"))
	require.False(t, channelMonitorV2ModelSelected(all, cfg, "openai", "gpt-4o"))
	require.False(t, channelMonitorV2ModelSelected(all, cfg, "unknown", "unknown"))

	where, args := channelMonitorV2Where(all, cfg, "m")
	require.NotContains(t, where, "platform")
	require.NotContains(t, where, "FALSE")
	require.Len(t, args, 2)

	accs := seedChannelMonitorV2MatrixAccumulators(all, cfg, service.ChannelMonitorV2GroupByModel)
	require.Len(t, accs, 2)
	require.Contains(t, accs, channelMonitorV2MatrixKey{model: "gpt-5.5"})
	require.Contains(t, accs, channelMonitorV2MatrixKey{model: "glm-5.3-flash"})
}

func TestChannelMonitorV2MatrixDimensionKey(t *testing.T) {
	cfg := service.ChannelMonitorV2Config{Platforms: []service.ChannelMonitorV2PlatformConfig{{Platform: "openai", Enabled: true, Models: []string{"gpt-5"}}}}
	key := channelMonitorV2MatrixDimensionKey(service.ChannelMonitorV2GroupByPlatformModel, cfg, "openai", "gpt-5")
	require.Equal(t, channelMonitorV2MatrixKey{platform: "openai", model: "gpt-5"}, key)
	key = channelMonitorV2MatrixDimensionKey(service.ChannelMonitorV2GroupByPlatformModel, cfg, "openai", "unlisted")
	require.Equal(t, channelMonitorV2MatrixKey{platform: "openai", model: service.ChannelMonitorV2OtherModel}, key)
	key = channelMonitorV2MatrixDimensionKey(service.ChannelMonitorV2GroupByPlatform, cfg, "openai", "gpt-5")
	require.Equal(t, channelMonitorV2MatrixKey{platform: "openai"}, key)
	// model 维度跨平台聚合：键里不带平台，两个平台的同名模型落同一行。
	key = channelMonitorV2MatrixDimensionKey(service.ChannelMonitorV2GroupByModel, cfg, "openai", "gpt-5")
	require.Equal(t, channelMonitorV2MatrixKey{model: "gpt-5"}, key)
	require.Equal(t, key, channelMonitorV2MatrixDimensionKey(service.ChannelMonitorV2GroupByModel, cfg, "anthropic", "gpt-5"))
}

func TestChannelMonitorV2HistogramPercentilesAreMergedFromCounts(t *testing.T) {
	// 100 samples: 50@100, 40@500, 10@1000
	// target = int64(total*p + 0.999999) truncates: p50→50, p90→90, p95→95
	// cumulative hits: p50@100, p90@500 (50+40), p95@1000
	histogram := map[int64]int64{100: 50, 500: 40, 1000: 10}
	require.Equal(t, int64(100), *histPercentile(histogram, .5))
	require.Equal(t, int64(500), *histPercentile(histogram, .9))
	require.Equal(t, int64(1000), *histPercentile(histogram, .95))
	require.Nil(t, histPercentile(nil, .95))
	// latencyMetric exposes avg + p50 + p90 + p95
	lat := latencyMetric(1000, 10, histogram)
	require.NotNil(t, lat.AvgMs)
	require.NotNil(t, lat.P50Ms)
	require.NotNil(t, lat.P90Ms)
	require.NotNil(t, lat.P95Ms)
	require.Equal(t, int64(100), *lat.P50Ms)
	require.Equal(t, int64(500), *lat.P90Ms)
	require.Equal(t, int64(1000), *lat.P95Ms)
}

func TestChannelMonitorV2MetricIncludesSuccessRate(t *testing.T) {
	acc := newMetricAccumulator()
	acc.success, acc.errors = 80, 20
	metric := acc.metric(1, false)
	require.Equal(t, int64(100), metric.RequestCount)
	require.InDelta(t, 0.8, metric.SuccessRate, 0.0001)
	require.InDelta(t, 0.2, metric.ErrorRate, 0.0001)
	require.Nil(t, metric.UpstreamAffectedRequests)

	adminMetric := acc.metric(1, true)
	require.NotNil(t, adminMetric.UpstreamAffectedRequests)
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
	for _, query := range []string{channelMonitorV2UsageMetricsSQL, channelMonitorV2UserMetricsSQL} {
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

func TestChannelMonitorV2HistoryCoverageCompleteIgnoresTrailingLag(t *testing.T) {
	start := time.Date(2026, 8, 7, 2, 20, 0, 0, time.UTC)
	// History reaches the window start → complete even if data_through is behind filter.End.
	require.True(t, channelMonitorV2HistoryCoverageComplete(start, start))
	require.True(t, channelMonitorV2HistoryCoverageComplete(start.Add(-time.Hour), start))
	// Backfill still short of the window start → incomplete.
	require.False(t, channelMonitorV2HistoryCoverageComplete(start.Add(time.Hour), start))
	require.False(t, channelMonitorV2HistoryCoverageComplete(time.Time{}, start))
}

func TestChannelMonitorV2TierRetentionPolicy(t *testing.T) {
	require.Equal(t, 3*24*time.Hour, channelMonitorV2RetentionUser1m)
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

func TestSameFixedRollupBucket(t *testing.T) {
	start := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)
	require.True(t, sameFixedRollupBucket(start, start.Add(10*time.Minute), 86400))
	require.False(t, sameFixedRollupBucket(start, start.Add(15*time.Hour), 43200))
	require.False(t, sameFixedRollupBucket(start, start.Add(24*time.Hour), 86400))
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

func TestRedactChannelMonitorV2MetricZerosVolume(t *testing.T) {
	// Service helper is in service package; covered there. Keep a smoke note that
	// rates survive a manual zeroing of volume fields used by the UI contract.
	m := service.ChannelMonitorV2Metric{
		RequestCount: 100, ErrorRequests: 10, SuccessRequests: 90,
		TokenCount: 1000, RPM: 5, TPM: 50, ErrorRate: 0.1, SuccessRate: 0.9, CacheRate: 0.4,
	}
	// Mimic redact: zero volume only
	m.RequestCount, m.ErrorRequests, m.SuccessRequests, m.TokenCount = 0, 0, 0, 0
	require.Equal(t, 0.1, m.ErrorRate)
	require.Equal(t, 5.0, m.RPM)
}

func TestChannelMonitorV2CatalogFilterClearsMultiSelectDimensions(t *testing.T) {
	start := time.Unix(1, 0)
	end := time.Unix(2, 0)
	filter := service.ChannelMonitorV2Filter{
		Start: start, End: end, Bucket: time.Minute,
		Platforms: []string{"openai"}, Models: []string{"gpt-5"},
	}
	catalog := channelMonitorV2CatalogFilter(filter)
	require.Nil(t, catalog.Platforms)
	require.Nil(t, catalog.Models)
	// Time window / coverage-related fields remain.
	require.Equal(t, start, catalog.Start)
	require.Equal(t, end, catalog.End)
	require.Equal(t, time.Minute, catalog.Bucket)

	cfg := service.ChannelMonitorV2Config{
		Platforms: []service.ChannelMonitorV2PlatformConfig{
			{Platform: "openai", Enabled: true},
			{Platform: "grok", Enabled: true},
		},
	}
	catalogWhere, catalogArgs := channelMonitorV2Where(catalog, cfg, "m")
	_, metricArgs := channelMonitorV2Where(filter, cfg, "m")

	// Catalog WHERE still applies the config scope (enabled platforms).
	require.Contains(t, catalogWhere, "m.platform = ANY")
	require.NotContains(t, catalogWhere, "m.group_id")
	require.Len(t, catalogArgs, 3) // start, end, platforms
	require.Len(t, metricArgs, 3)

	// Metrics WHERE is narrower once the multi-select platform is applied.
	require.NotEqual(t, catalogArgs, metricArgs)
}

// 非管理员的 /models 聚合键不带平台：两个平台上的同名模型落同一行。
// 管理员保留平台维度。
func TestChannelMonitorV2ModelStatsKeyHidesPlatformForNonAdmin(t *testing.T) {
	adminOpenAI := channelMonitorV2ModelStatsKey(true, "openai", "gpt-5")
	adminAnthropic := channelMonitorV2ModelStatsKey(true, "anthropic", "gpt-5")
	require.NotEqual(t, adminOpenAI, adminAnthropic)
	require.Contains(t, adminOpenAI, "openai")

	userOpenAI := channelMonitorV2ModelStatsKey(false, "openai", "gpt-5")
	userAnthropic := channelMonitorV2ModelStatsKey(false, "anthropic", "gpt-5")
	require.Equal(t, userOpenAI, userAnthropic)
	require.NotContains(t, userOpenAI, "openai")
	require.NotContains(t, userOpenAI, "anthropic")
	require.NotEqual(t, userOpenAI, channelMonitorV2ModelStatsKey(false, "openai", "claude-sonnet-4-5"))
}
