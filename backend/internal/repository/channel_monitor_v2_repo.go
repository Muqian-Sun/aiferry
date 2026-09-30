package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// channelMonitorV2Repository 用户站「服务状态」的读数：从被动汇总表（metrics / latency histograms /
// error metrics）按上架目录名单算全站快照与逐模型矩阵。汇总表里的平台列不参与读数。
type channelMonitorV2Repository struct{ db *sql.DB }

func NewChannelMonitorV2Repository(db *sql.DB) service.ChannelMonitorV2Repository {
	return &channelMonitorV2Repository{db: db}
}

type channelMonitorV2Fact struct {
	BucketStart, Model                             string
	Success, Errors                                int64
	Input, Output, CacheCreation, CacheRead        int64
	TTFTSum, TTFTCount, DurationSum, DurationCount int64
}

type channelMonitorV2Histogram struct {
	BucketStart, Model, Metric string
	UpperBound                 int64
	Count                      int64
}

// channelMonitorV2RosterModel 请求名归到名单里的哪个模型；名单外（没上架的名字、没解析出模型的失败请求）不计。
func channelMonitorV2RosterModel(cfg service.ChannelMonitorV2Config, model string) (string, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", false
	}
	return cfg.Roster.Resolve(model)
}

func (r *channelMonitorV2Repository) GetSnapshot(ctx context.Context, filter service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Snapshot, error) {
	coverage, err := r.loadCoverage(ctx, filter)
	if err != nil {
		return nil, err
	}
	effectiveFilter := channelMonitorV2CommonCoverageFilter(filter, *coverage)
	facts, err := r.loadFacts(ctx, effectiveFilter)
	if err != nil {
		return nil, err
	}
	histograms, err := r.loadHistograms(ctx, effectiveFilter)
	if err != nil {
		return nil, err
	}
	byBucket := map[string]*metricAccumulator{}
	total := newMetricAccumulator()
	for _, fact := range facts {
		if _, ok := channelMonitorV2RosterModel(cfg, fact.Model); !ok {
			continue
		}
		acc := byBucket[fact.BucketStart]
		if acc == nil {
			acc = newMetricAccumulator()
			byBucket[fact.BucketStart] = acc
		}
		acc.addFact(fact)
		total.addFact(fact)
	}
	for _, hist := range histograms {
		if _, ok := channelMonitorV2RosterModel(cfg, hist.Model); !ok {
			continue
		}
		if acc := byBucket[hist.BucketStart]; acc != nil {
			acc.addHistogram(hist)
		}
		total.addHistogram(hist)
	}
	// Category-level ignored errors adjust error_rate without dropping volume.
	ignored, err := r.loadIgnoredErrorCounts(ctx, effectiveFilter, cfg)
	if err != nil {
		return nil, fmt.Errorf("load ignored error counts: %w", err)
	}
	ignoredByBucket := map[string]int64{}
	var ignoredTotal int64
	for _, buckets := range ignored {
		for bucket, count := range buckets {
			ignoredByBucket[bucket] += count
			ignoredTotal += count
		}
	}
	metrics := total.metric(channelMonitorV2CoveredMinutes(filter, *coverage))
	applyIgnoredErrors(&metrics, ignoredTotal)
	result := &service.ChannelMonitorV2Snapshot{Coverage: *coverage, Metrics: metrics, Health: service.ChannelMonitorV2HealthForWithThresholds(metrics, cfg.HealthThresholds), Trend: []service.ChannelMonitorV2TrendPoint{}}
	keys := make([]string, 0, len(byBucket))
	for key := range byBucket {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		bucket, _ := time.Parse(time.RFC3339Nano, key)
		m := byBucket[key].metric(filter.Bucket.Minutes())
		applyIgnoredErrors(&m, ignoredByBucket[key])
		result.Trend = append(result.Trend, service.ChannelMonitorV2TrendPoint{BucketStart: bucket, Metrics: m, Health: service.ChannelMonitorV2HealthForWithThresholds(m, cfg.HealthThresholds)})
	}
	return result, nil
}

type channelMonitorV2MatrixAccumulator struct {
	total   *metricAccumulator
	buckets map[string]*metricAccumulator
}

// GetMatrix 名单里每个模型一行（没有请求也在）：总量 + 逐段。
// 百分位由原始直方图现算（同一模型跨平台、跨别名的样本进同一个累加器），不是把已算好的分位数再平均。
func (r *channelMonitorV2Repository) GetMatrix(ctx context.Context, filter service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Matrix, error) {
	coverage, err := r.loadCoverage(ctx, filter)
	if err != nil {
		return nil, err
	}
	effectiveFilter := channelMonitorV2CommonCoverageFilter(filter, *coverage)
	facts, err := r.loadFacts(ctx, effectiveFilter)
	if err != nil {
		return nil, err
	}
	histograms, err := r.loadHistograms(ctx, effectiveFilter)
	if err != nil {
		return nil, err
	}

	accs := make(map[string]*channelMonitorV2MatrixAccumulator, len(cfg.Roster.Models))
	for _, model := range cfg.Roster.Models {
		accs[model] = &channelMonitorV2MatrixAccumulator{total: newMetricAccumulator(), buckets: make(map[string]*metricAccumulator)}
	}
	for _, fact := range facts {
		model, ok := channelMonitorV2RosterModel(cfg, fact.Model)
		acc := accs[model]
		if !ok || acc == nil {
			continue
		}
		bucket := acc.buckets[fact.BucketStart]
		if bucket == nil {
			bucket = newMetricAccumulator()
			acc.buckets[fact.BucketStart] = bucket
		}
		acc.total.addFact(fact)
		bucket.addFact(fact)
	}
	for _, histogram := range histograms {
		model, ok := channelMonitorV2RosterModel(cfg, histogram.Model)
		acc := accs[model]
		if !ok || acc == nil {
			continue
		}
		acc.total.addHistogram(histogram)
		if bucket := acc.buckets[histogram.BucketStart]; bucket != nil {
			bucket.addHistogram(histogram)
		}
	}

	ignored, err := r.loadIgnoredErrorCounts(ctx, effectiveFilter, cfg)
	if err != nil {
		return nil, fmt.Errorf("load ignored error counts by model: %w", err)
	}
	result := &service.ChannelMonitorV2Matrix{Coverage: *coverage, Items: make([]service.ChannelMonitorV2MatrixRow, 0, len(accs))}
	minutes := channelMonitorV2CoveredMinutes(filter, *coverage)
	for model, acc := range accs {
		var ignoredTotal int64
		for _, count := range ignored[model] {
			ignoredTotal += count
		}
		metrics := acc.total.metric(minutes)
		applyIgnoredErrors(&metrics, ignoredTotal)
		row := service.ChannelMonitorV2MatrixRow{Model: model, Metrics: metrics, Health: service.ChannelMonitorV2HealthForWithThresholds(metrics, cfg.HealthThresholds), Buckets: []service.ChannelMonitorV2TrendPoint{}}
		bucketKeys := make([]string, 0, len(acc.buckets))
		for bucket := range acc.buckets {
			bucketKeys = append(bucketKeys, bucket)
		}
		sort.Strings(bucketKeys)
		for _, bucketKey := range bucketKeys {
			bucketStart, parseErr := time.Parse(time.RFC3339Nano, bucketKey)
			if parseErr != nil {
				continue
			}
			bucketMetrics := acc.buckets[bucketKey].metric(filter.Bucket.Minutes())
			applyIgnoredErrors(&bucketMetrics, ignored[model][bucketKey])
			row.Buckets = append(row.Buckets, service.ChannelMonitorV2TrendPoint{BucketStart: bucketStart, Metrics: bucketMetrics, Health: service.ChannelMonitorV2HealthForWithThresholds(bucketMetrics, cfg.HealthThresholds)})
		}
		result.Items = append(result.Items, row)
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].Model < result.Items[j].Model })
	return result, nil
}

// channelMonitorV2BucketQuery 按段汇总的 SELECT 前缀与参数：固定粒度走 rollup 表的 bucket_start，
// 其他粒度用 date_bin 现场对齐（参数 $1 是段长，窗口参数顺延）。
func channelMonitorV2BucketQuery(filter service.ChannelMonitorV2Filter, alias string) (bucketExpr, where string, args []any) {
	where, args, bucketSeconds := channelMonitorV2WhereWithRollup(filter, alias)
	bucketExpr = alias + ".bucket_start"
	if bucketSeconds == 0 {
		args = append([]any{fmt.Sprintf("%d seconds", int(filter.Bucket.Seconds()))}, args...)
		where = shiftSQLPlaceholders(where, 1)
		bucketExpr = channelMonitorV2DateBinExpr(alias + ".bucket_start")
	}
	return bucketExpr, where, args
}

func (r *channelMonitorV2Repository) loadFacts(ctx context.Context, filter service.ChannelMonitorV2Filter) ([]channelMonitorV2Fact, error) {
	bucketExpr, where, args := channelMonitorV2BucketQuery(filter, "m")
	query := `SELECT ` + bucketExpr + `,m.model,SUM(m.success_requests),SUM(m.error_requests),SUM(m.input_tokens),SUM(m.output_tokens),SUM(m.cache_creation_tokens),SUM(m.cache_read_tokens),SUM(m.ttft_sum_ms),SUM(m.ttft_count),SUM(m.duration_sum_ms),SUM(m.duration_count) FROM ` + channelMonitorV2MetricsTable(filter) + ` m ` + where + ` GROUP BY 1,2`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	facts := []channelMonitorV2Fact{}
	for rows.Next() {
		var bucket time.Time
		var f channelMonitorV2Fact
		if err := rows.Scan(&bucket, &f.Model, &f.Success, &f.Errors, &f.Input, &f.Output, &f.CacheCreation, &f.CacheRead, &f.TTFTSum, &f.TTFTCount, &f.DurationSum, &f.DurationCount); err != nil {
			return nil, err
		}
		f.BucketStart = bucket.UTC().Format(time.RFC3339Nano)
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

func (r *channelMonitorV2Repository) loadHistograms(ctx context.Context, filter service.ChannelMonitorV2Filter) ([]channelMonitorV2Histogram, error) {
	bucketExpr, where, args := channelMonitorV2BucketQuery(filter, "h")
	rows, err := r.db.QueryContext(ctx, `SELECT `+bucketExpr+`,h.model,h.metric,h.upper_bound_ms,SUM(h.sample_count) FROM `+channelMonitorV2HistogramTable(filter)+` h `+where+` GROUP BY 1,2,3,4`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []channelMonitorV2Histogram{}
	for rows.Next() {
		var bucket time.Time
		var h channelMonitorV2Histogram
		if err := rows.Scan(&bucket, &h.Model, &h.Metric, &h.UpperBound, &h.Count); err != nil {
			return nil, err
		}
		h.BucketStart = bucket.UTC().Format(time.RFC3339Nano)
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *channelMonitorV2Repository) GetAggregationWatermark(ctx context.Context) (*service.ChannelMonitorV2AggregationWatermark, error) {
	var usageStart, errorStart, dataThrough, computed, backfill sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT usage_coverage_start, error_coverage_start, data_through, last_successful_at, backfill_cursor
		FROM channel_monitor_v2_watermarks WHERE id = 1`).Scan(&usageStart, &errorStart, &dataThrough, &computed, &backfill)
	if err == sql.ErrNoRows {
		return &service.ChannelMonitorV2AggregationWatermark{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := &service.ChannelMonitorV2AggregationWatermark{HasData: dataThrough.Valid}
	if usageStart.Valid {
		out.UsageCoverageStart = usageStart.Time.UTC()
	}
	if errorStart.Valid {
		out.ErrorCoverageStart = errorStart.Time.UTC()
	}
	if dataThrough.Valid {
		out.DataThrough = dataThrough.Time.UTC()
	}
	if computed.Valid {
		out.LastSuccessfulAt = computed.Time.UTC()
	}
	if backfill.Valid {
		out.BackfillCursor = backfill.Time.UTC()
	}
	return out, nil
}

func (r *channelMonitorV2Repository) loadCoverage(ctx context.Context, filter service.ChannelMonitorV2Filter) (*service.ChannelMonitorV2Coverage, error) {
	wm, err := r.GetAggregationWatermark(ctx)
	if err != nil {
		return nil, err
	}
	if wm == nil {
		wm = &service.ChannelMonitorV2AggregationWatermark{}
	}
	bootstrap := service.ChannelMonitorV2BootstrapProgress(time.Now().UTC(), wm.BackfillCursor, wm.HasData)
	if !wm.HasData || wm.DataThrough.IsZero() || wm.LastSuccessfulAt.IsZero() {
		return &service.ChannelMonitorV2Coverage{
			RequestedStart:        filter.Start,
			RequestedEnd:          filter.End,
			CoverageStart:         filter.End,
			DataThrough:           filter.Start,
			ComputedAt:            time.Time{},
			AggregationLagSeconds: 0,
			CoverageComplete:      false,
			BucketSeconds:         int(filter.Bucket.Seconds()),
			Bootstrap:             bootstrap,
		}, nil
	}
	coverageStart := filter.Start
	if !wm.UsageCoverageStart.IsZero() && wm.UsageCoverageStart.After(coverageStart) {
		coverageStart = wm.UsageCoverageStart
	}
	if !wm.ErrorCoverageStart.IsZero() && wm.ErrorCoverageStart.After(coverageStart) {
		coverageStart = wm.ErrorCoverageStart
	}
	through := filter.End
	if !wm.DataThrough.IsZero() && wm.DataThrough.Before(through) {
		through = wm.DataThrough
	}
	computedAt := wm.LastSuccessfulAt
	lag := int64(0)
	if !through.IsZero() {
		lag = int64(time.Since(through).Seconds())
		if lag < 0 {
			lag = 0
		}
	}
	// Complete = history depth for this range is filled (backfill reached
	// filter.Start). Do not require data_through >= filter.End: ParseFilter
	// aligns End to the next whole bucket (often in the future), so a healthy
	// minute-level lag would otherwise always show "partial historical coverage".
	return &service.ChannelMonitorV2Coverage{
		RequestedStart:        filter.Start,
		RequestedEnd:          filter.End,
		CoverageStart:         coverageStart,
		DataThrough:           through,
		ComputedAt:            computedAt,
		AggregationLagSeconds: lag,
		CoverageComplete:      channelMonitorV2HistoryCoverageComplete(coverageStart, filter.Start),
		BucketSeconds:         int(filter.Bucket.Seconds()),
		Bootstrap:             bootstrap,
	}, nil
}

// channelMonitorV2HistoryCoverageComplete is true when aggregated history
// reaches the requested window start. Trailing freshness is reported via
// data_through / aggregation_lag_seconds, not coverage_complete.
func channelMonitorV2HistoryCoverageComplete(coverageStart, filterStart time.Time) bool {
	if coverageStart.IsZero() || filterStart.IsZero() {
		return false
	}
	return !coverageStart.After(filterStart)
}

func channelMonitorV2FixedBucketSeconds(filter service.ChannelMonitorV2Filter) int {
	seconds := int(filter.Bucket.Seconds())
	switch seconds {
	case 300, 3600, 43200, 86400:
		return seconds
	default:
		return 0
	}
}

func channelMonitorV2MetricsTable(filter service.ChannelMonitorV2Filter) string {
	if channelMonitorV2FixedBucketSeconds(filter) > 0 {
		return "channel_monitor_v2_metrics_rollup"
	}
	return "channel_monitor_v2_metrics_1m"
}

func channelMonitorV2ErrorMetricsTable(filter service.ChannelMonitorV2Filter) string {
	if channelMonitorV2FixedBucketSeconds(filter) > 0 {
		return "channel_monitor_v2_error_metrics_rollup"
	}
	return "channel_monitor_v2_error_metrics_1m"
}

func channelMonitorV2HistogramTable(filter service.ChannelMonitorV2Filter) string {
	if channelMonitorV2FixedBucketSeconds(filter) > 0 {
		return "channel_monitor_v2_latency_histograms_rollup"
	}
	return "channel_monitor_v2_latency_histograms_1m"
}

func channelMonitorV2WhereWithRollup(filter service.ChannelMonitorV2Filter, alias string) (string, []any, int) {
	where := "WHERE " + alias + ".bucket_start >= $1 AND " + alias + ".bucket_start < $2"
	args := []any{filter.Start, filter.End}
	bucketSeconds := channelMonitorV2FixedBucketSeconds(filter)
	if bucketSeconds > 0 {
		args = append(args, bucketSeconds)
		where += fmt.Sprintf(" AND %s.bucket_seconds = $%d", alias, len(args))
	}
	return where, args, bucketSeconds
}

func channelMonitorV2CoveredMinutes(filter service.ChannelMonitorV2Filter, coverage service.ChannelMonitorV2Coverage) float64 {
	start := filter.Start
	if coverage.CoverageStart.After(start) {
		start = coverage.CoverageStart
	}
	end := filter.End
	if !coverage.DataThrough.IsZero() && coverage.DataThrough.Before(end) {
		end = coverage.DataThrough
	}
	if !start.Before(end) {
		return 1
	}
	return end.Sub(start).Minutes()
}

// Usage and error sources can have different retention windows. Composite
// health metrics must use their common interval; otherwise error rates mix
// unlike periods and RPM/TPM use a denominator shorter than their numerator.
func channelMonitorV2CommonCoverageFilter(filter service.ChannelMonitorV2Filter, coverage service.ChannelMonitorV2Coverage) service.ChannelMonitorV2Filter {
	if coverage.CoverageStart.After(filter.Start) {
		filter.Start = coverage.CoverageStart
	}
	if !coverage.DataThrough.IsZero() && coverage.DataThrough.Before(filter.End) {
		filter.End = coverage.DataThrough
	}
	return filter
}

// shiftSQLPlaceholders shifts existing positional placeholders by offset.
func shiftSQLPlaceholders(query string, offset int) string {
	for i := 64; i >= 1; i-- {
		query = strings.ReplaceAll(query, fmt.Sprintf("$%d", i), fmt.Sprintf("$%d", i+offset))
	}
	return query
}

type metricAccumulator struct {
	success, errors, input, output, cacheCreation, cacheRead, ttftSum, ttftCount, durationSum, durationCount int64
	hist                                                                                                     map[string]map[int64]int64
}

func newMetricAccumulator() *metricAccumulator {
	return &metricAccumulator{hist: map[string]map[int64]int64{"ttft": {}, "duration": {}}}
}
func (a *metricAccumulator) addFact(f channelMonitorV2Fact) {
	a.success += f.Success
	a.errors += f.Errors
	a.input += f.Input
	a.output += f.Output
	a.cacheCreation += f.CacheCreation
	a.cacheRead += f.CacheRead
	a.ttftSum += f.TTFTSum
	a.ttftCount += f.TTFTCount
	a.durationSum += f.DurationSum
	a.durationCount += f.DurationCount
}
func (a *metricAccumulator) addHistogram(h channelMonitorV2Histogram) {
	if a.hist[h.Metric] == nil {
		a.hist[h.Metric] = map[int64]int64{}
	}
	a.hist[h.Metric][h.UpperBound] += h.Count
}
func (a *metricAccumulator) metric(minutes float64) service.ChannelMonitorV2Metric {
	requests := a.success + a.errors
	tokens := a.input + a.output + a.cacheCreation + a.cacheRead
	denom := a.input + a.cacheCreation + a.cacheRead
	if minutes <= 0 {
		minutes = 1
	}
	m := service.ChannelMonitorV2Metric{SuccessRequests: a.success, ErrorRequests: a.errors, RequestCount: requests, InputTokens: a.input, OutputTokens: a.output, CacheCreationTokens: a.cacheCreation, CacheReadTokens: a.cacheRead, TokenCount: tokens, RPM: float64(requests) / minutes, TPM: float64(tokens) / minutes, CacheRateNumerator: a.cacheRead, CacheRateDenominator: denom, TTFT: latencyMetric(a.ttftSum, a.ttftCount, a.hist["ttft"]), Duration: latencyMetric(a.durationSum, a.durationCount, a.hist["duration"])}
	if requests > 0 {
		m.ErrorRate = float64(a.errors) / float64(requests)
		m.SuccessRate = float64(a.success) / float64(requests)
	}
	if denom > 0 {
		m.CacheRate = float64(a.cacheRead) / float64(denom)
	}
	return m
}
func latencyMetric(sum, count int64, hist map[int64]int64) service.ChannelMonitorV2Latency {
	result := service.ChannelMonitorV2Latency{SampleCount: count}
	if count > 0 {
		avg := float64(sum) / float64(count)
		result.AvgMs = &avg
	}
	result.P50Ms = histPercentile(hist, 0.50)
	result.P90Ms = histPercentile(hist, 0.90)
	result.P95Ms = histPercentile(hist, 0.95)
	return result
}
func histPercentile(hist map[int64]int64, p float64) *int64 {
	var total int64
	keys := make([]int64, 0, len(hist))
	for bound, count := range hist {
		keys = append(keys, bound)
		total += count
	}
	if total == 0 {
		return nil
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	target := int64(float64(total)*p + 0.999999)
	var cumulative int64
	for _, bound := range keys {
		cumulative += hist[bound]
		if cumulative >= target {
			v := bound
			return &v
		}
	}
	v := keys[len(keys)-1]
	return &v
}

// applyIgnoredErrors rewrites ErrorRate so ignored categories do not inflate the
// scored error rate used by health. Absolute ErrorRequests / RequestCount stay
// for volume/RPM. SuccessRate remains true success/request (ignored errors are
// not treated as successes).
func applyIgnoredErrors(m *service.ChannelMonitorV2Metric, ignoredCount int64) {
	if m == nil || ignoredCount <= 0 || m.RequestCount <= 0 {
		return
	}
	if ignoredCount > m.ErrorRequests {
		ignoredCount = m.ErrorRequests
	}
	countedErrors := m.ErrorRequests - ignoredCount
	if countedErrors < 0 {
		countedErrors = 0
	}
	m.ErrorRate = float64(countedErrors) / float64(m.RequestCount)
	// Keep SuccessRate = SuccessRequests / RequestCount when absolute success is
	// known; fall back to residual only if SuccessRequests was never populated.
	if m.SuccessRequests > 0 || m.ErrorRequests >= m.RequestCount {
		m.SuccessRate = float64(m.SuccessRequests) / float64(m.RequestCount)
	}
	if m.SuccessRate < 0 {
		m.SuccessRate = 0
	}
	if m.SuccessRate > 1 {
		m.SuccessRate = 1
	}
}

// loadIgnoredErrorCounts 名单里各模型各段被忽略类别的错误数（模型 → 段 → 数），
// 段与 loadFacts 同一套对齐。
func (r *channelMonitorV2Repository) loadIgnoredErrorCounts(
	ctx context.Context,
	filter service.ChannelMonitorV2Filter,
	cfg service.ChannelMonitorV2Config,
) (map[string]map[string]int64, error) {
	out := map[string]map[string]int64{}
	if len(cfg.IgnoredErrorCategories) == 0 {
		return out, nil
	}
	bucketExpr, where, args := channelMonitorV2BucketQuery(filter, "e")
	args = append(args, pq.Array(cfg.IgnoredErrorCategories), service.ChannelMonitorV2TaxonomyVersion)
	catIdx := len(args) - 1
	taxIdx := len(args)
	query := fmt.Sprintf(
		`SELECT %s, e.model, SUM(e.error_requests)
		 FROM `+channelMonitorV2ErrorMetricsTable(filter)+` e %s
		 AND e.error_category = ANY($%d) AND e.taxonomy_version = $%d
		 GROUP BY 1, 2`,
		bucketExpr, where, catIdx, taxIdx,
	)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket time.Time
		var raw string
		var count int64
		if err := rows.Scan(&bucket, &raw, &count); err != nil {
			return out, err
		}
		model, ok := channelMonitorV2RosterModel(cfg, raw)
		if !ok {
			continue
		}
		if out[model] == nil {
			out[model] = map[string]int64{}
		}
		out[model][bucket.UTC().Format(time.RFC3339Nano)] += count
	}
	return out, rows.Err()
}
