package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
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
	AccountID                                      int64
	Success, Errors                                int64
	Input, Output, CacheCreation, CacheRead        int64
	TTFTSum, TTFTCount, DurationSum, DurationCount int64
}

type channelMonitorV2Histogram struct {
	BucketStart, Model, Metric string
	AccountID                  int64
	UpperBound                 int64
	Count                      int64
}

// channelMonitorV2Ignored 被忽略类别的错误数（不计入错误率），与事实同一套段对齐。
type channelMonitorV2Ignored struct {
	BucketStart, Model string
	AccountID          int64
	Count              int64
}

// channelMonitorV2RosterModel 请求名归到名单里的哪个模型；名单外（没上架的名字、没解析出模型的失败请求）不计。
func channelMonitorV2RosterModel(cfg service.ChannelMonitorV2Config, model string) (string, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", false
	}
	return cfg.Roster.Resolve(model)
}

// channelMonitorV2Group 一组（一个模型 / 一个渠道 / 整体）的累加：总量 + 逐段 + 被忽略的错误数。
// 百分位由原始直方图现算（跨平台、跨别名、跨渠道的样本进同一个累加器），不是把已算好的分位数再平均。
type channelMonitorV2Group struct {
	total          *metricAccumulator
	buckets        map[string]*metricAccumulator
	ignoredTotal   int64
	ignoredBuckets map[string]int64
}

func newChannelMonitorV2Group() *channelMonitorV2Group {
	return &channelMonitorV2Group{total: newMetricAccumulator(), buckets: map[string]*metricAccumulator{}, ignoredBuckets: map[string]int64{}}
}

func (g *channelMonitorV2Group) bucket(start string) *metricAccumulator {
	acc := g.buckets[start]
	if acc == nil {
		acc = newMetricAccumulator()
		g.buckets[start] = acc
	}
	return acc
}

// result 总量与逐段（按段起点排序）的指标和档位；被忽略的错误只改错误率、不减量。
func (g *channelMonitorV2Group) result(minutes, bucketMinutes float64, thresholds service.ChannelMonitorV2HealthThresholds) (service.ChannelMonitorV2Metric, service.ChannelMonitorV2Health, []service.ChannelMonitorV2TrendPoint) {
	metrics := g.total.metric(minutes)
	applyIgnoredErrors(&metrics, g.ignoredTotal)
	keys := make([]string, 0, len(g.buckets))
	for key := range g.buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	points := make([]service.ChannelMonitorV2TrendPoint, 0, len(keys))
	for _, key := range keys {
		start, err := time.Parse(time.RFC3339Nano, key)
		if err != nil {
			continue
		}
		m := g.buckets[key].metric(bucketMinutes)
		applyIgnoredErrors(&m, g.ignoredBuckets[key])
		points = append(points, service.ChannelMonitorV2TrendPoint{BucketStart: start, Metrics: m, Health: service.ChannelMonitorV2HealthForWithThresholds(m, thresholds)})
	}
	return metrics, service.ChannelMonitorV2HealthForWithThresholds(metrics, thresholds), points
}

// channelMonitorV2Reading 一次读数：时间窗覆盖、整体、按键分组。
type channelMonitorV2Reading struct {
	coverage service.ChannelMonitorV2Coverage
	minutes  float64
	overall  *channelMonitorV2Group
	groups   map[string]*channelMonitorV2Group
}

// read 读一个时间窗的事实、延迟分布与被忽略的错误，按 keyOf（请求名、渠道）分组；
// keyOf 返回 false 的行整体也不计。seed 里的键没有请求也有一组。
func (r *channelMonitorV2Repository) read(
	ctx context.Context,
	filter service.ChannelMonitorV2Filter,
	cfg service.ChannelMonitorV2Config,
	seed []string,
	keyOf func(model string, accountID int64) (string, bool),
) (*channelMonitorV2Reading, error) {
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
	ignored, err := r.loadIgnoredErrors(ctx, effectiveFilter, cfg)
	if err != nil {
		return nil, fmt.Errorf("load ignored error counts: %w", err)
	}

	reading := &channelMonitorV2Reading{coverage: *coverage, minutes: channelMonitorV2CoveredMinutes(filter, *coverage), overall: newChannelMonitorV2Group(), groups: map[string]*channelMonitorV2Group{}}
	for _, key := range seed {
		reading.groups[key] = newChannelMonitorV2Group()
	}
	groupOf := func(model string, accountID int64) *channelMonitorV2Group {
		key, ok := keyOf(model, accountID)
		if !ok {
			return nil
		}
		g := reading.groups[key]
		if g == nil {
			g = newChannelMonitorV2Group()
			reading.groups[key] = g
		}
		return g
	}
	for _, fact := range facts {
		g := groupOf(fact.Model, fact.AccountID)
		if g == nil {
			continue
		}
		for _, target := range []*channelMonitorV2Group{g, reading.overall} {
			target.total.addFact(fact)
			target.bucket(fact.BucketStart).addFact(fact)
		}
	}
	for _, histogram := range histograms {
		g := groupOf(histogram.Model, histogram.AccountID)
		if g == nil {
			continue
		}
		for _, target := range []*channelMonitorV2Group{g, reading.overall} {
			target.total.addHistogram(histogram)
			// 只进已有请求的段：没有事实的段不凭延迟分布单独出一段
			if acc := target.buckets[histogram.BucketStart]; acc != nil {
				acc.addHistogram(histogram)
			}
		}
	}
	for _, row := range ignored {
		g := groupOf(row.Model, row.AccountID)
		if g == nil {
			continue
		}
		for _, target := range []*channelMonitorV2Group{g, reading.overall} {
			target.ignoredTotal += row.Count
			target.ignoredBuckets[row.BucketStart] += row.Count
		}
	}
	return reading, nil
}

func (r *channelMonitorV2Repository) GetSnapshot(ctx context.Context, filter service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Snapshot, error) {
	reading, err := r.read(ctx, filter, cfg, nil, func(model string, _ int64) (string, bool) {
		return channelMonitorV2RosterModel(cfg, model)
	})
	if err != nil {
		return nil, err
	}
	return channelMonitorV2Snapshot(reading, filter, cfg), nil
}

func channelMonitorV2Snapshot(reading *channelMonitorV2Reading, filter service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config) *service.ChannelMonitorV2Snapshot {
	metrics, health, trend := reading.overall.result(reading.minutes, filter.Bucket.Minutes(), cfg.HealthThresholds)
	return &service.ChannelMonitorV2Snapshot{Coverage: reading.coverage, Metrics: metrics, Health: health, Trend: trend}
}

// GetMatrix 名单里每个模型一行（没有请求也在）：总量 + 逐段。
func (r *channelMonitorV2Repository) GetMatrix(ctx context.Context, filter service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config) (*service.ChannelMonitorV2Matrix, error) {
	reading, err := r.read(ctx, filter, cfg, cfg.Roster.Models, func(model string, _ int64) (string, bool) {
		return channelMonitorV2RosterModel(cfg, model)
	})
	if err != nil {
		return nil, err
	}
	result := &service.ChannelMonitorV2Matrix{Coverage: reading.coverage, Items: make([]service.ChannelMonitorV2MatrixRow, 0, len(reading.groups))}
	for model, g := range reading.groups {
		metrics, health, buckets := g.result(reading.minutes, filter.Bucket.Minutes(), cfg.HealthThresholds)
		result.Items = append(result.Items, service.ChannelMonitorV2MatrixRow{Model: model, Metrics: metrics, Health: health, Buckets: buckets})
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].Model < result.Items[j].Model })
	return result, nil
}

// GetChannels 管理站渠道状态：全部流量按渠道聚合（0 = 没选到渠道就失败的请求），model 非空时只算请求名
// 解析到这个上架模型的。没删的渠道没有请求也有一行；已删的渠道只在有请求时出现。
func (r *channelMonitorV2Repository) GetChannels(ctx context.Context, filter service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config, model string) (*service.ChannelMonitorV2Channels, error) {
	accounts, err := r.loadChannelAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("load channel accounts: %w", err)
	}
	seed := make([]string, 0, len(accounts))
	for id, account := range accounts {
		if !account.Deleted {
			seed = append(seed, strconv.FormatInt(id, 10))
		}
	}
	reading, err := r.read(ctx, filter, cfg, seed, func(raw string, accountID int64) (string, bool) {
		if model != "" {
			if resolved, ok := channelMonitorV2RosterModel(cfg, raw); !ok || resolved != model {
				return "", false
			}
		}
		return strconv.FormatInt(accountID, 10), true
	})
	if err != nil {
		return nil, err
	}
	result := &service.ChannelMonitorV2Channels{ChannelMonitorV2Snapshot: *channelMonitorV2Snapshot(reading, filter, cfg), Items: make([]service.ChannelMonitorV2ChannelRow, 0, len(reading.groups))}
	for key, g := range reading.groups {
		id, _ := strconv.ParseInt(key, 10, 64)
		metrics, health, buckets := g.result(reading.minutes, filter.Bucket.Minutes(), cfg.HealthThresholds)
		row := service.ChannelMonitorV2ChannelRow{AccountID: id, Metrics: metrics, Health: health, Buckets: buckets}
		if account, ok := accounts[id]; ok {
			row.Name, row.Platform, row.Type, row.Status, row.Deleted = account.Name, account.Platform, account.Type, account.Status, account.Deleted
		}
		result.Items = append(result.Items, row)
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].AccountID < result.Items[j].AccountID })
	return result, nil
}

type channelMonitorV2Account struct {
	Name, Platform, Type, Status string
	Deleted                      bool
}

// loadChannelAccounts 全部渠道（含已删的：历史请求还挂在它们名下）的名字、平台、类型与状态。
func (r *channelMonitorV2Repository) loadChannelAccounts(ctx context.Context) (map[int64]channelMonitorV2Account, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, platform, type, status, deleted_at IS NOT NULL FROM accounts`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]channelMonitorV2Account{}
	for rows.Next() {
		var id int64
		var account channelMonitorV2Account
		if err := rows.Scan(&id, &account.Name, &account.Platform, &account.Type, &account.Status, &account.Deleted); err != nil {
			return nil, err
		}
		out[id] = account
	}
	return out, rows.Err()
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
	query := `SELECT ` + bucketExpr + `,m.model,m.account_id,SUM(m.success_requests),SUM(m.error_requests),SUM(m.input_tokens),SUM(m.output_tokens),SUM(m.cache_creation_tokens),SUM(m.cache_read_tokens),SUM(m.ttft_sum_ms),SUM(m.ttft_count),SUM(m.duration_sum_ms),SUM(m.duration_count) FROM ` + channelMonitorV2MetricsTable(filter) + ` m ` + where + ` GROUP BY 1,2,3`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	facts := []channelMonitorV2Fact{}
	for rows.Next() {
		var bucket time.Time
		var f channelMonitorV2Fact
		if err := rows.Scan(&bucket, &f.Model, &f.AccountID, &f.Success, &f.Errors, &f.Input, &f.Output, &f.CacheCreation, &f.CacheRead, &f.TTFTSum, &f.TTFTCount, &f.DurationSum, &f.DurationCount); err != nil {
			return nil, err
		}
		f.BucketStart = bucket.UTC().Format(time.RFC3339Nano)
		facts = append(facts, f)
	}
	return facts, rows.Err()
}

func (r *channelMonitorV2Repository) loadHistograms(ctx context.Context, filter service.ChannelMonitorV2Filter) ([]channelMonitorV2Histogram, error) {
	bucketExpr, where, args := channelMonitorV2BucketQuery(filter, "h")
	rows, err := r.db.QueryContext(ctx, `SELECT `+bucketExpr+`,h.model,h.account_id,h.metric,h.upper_bound_ms,SUM(h.sample_count) FROM `+channelMonitorV2HistogramTable(filter)+` h `+where+` GROUP BY 1,2,3,4,5`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []channelMonitorV2Histogram{}
	for rows.Next() {
		var bucket time.Time
		var h channelMonitorV2Histogram
		if err := rows.Scan(&bucket, &h.Model, &h.AccountID, &h.Metric, &h.UpperBound, &h.Count); err != nil {
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
	if !wm.HasData || wm.DataThrough.IsZero() || wm.LastSuccessfulAt.IsZero() {
		return &service.ChannelMonitorV2Coverage{
			RequestedStart: filter.Start,
			RequestedEnd:   filter.End,
			CoverageStart:  filter.End,
			DataThrough:    filter.Start,
			BucketSeconds:  int(filter.Bucket.Seconds()),
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
	return &service.ChannelMonitorV2Coverage{
		RequestedStart: filter.Start,
		RequestedEnd:   filter.End,
		CoverageStart:  coverageStart,
		DataThrough:    through,
		BucketSeconds:  int(filter.Bucket.Seconds()),
	}, nil
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

// loadIgnoredErrors 各段、各请求名、各渠道被忽略类别的错误数，段与 loadFacts 同一套对齐。
func (r *channelMonitorV2Repository) loadIgnoredErrors(
	ctx context.Context,
	filter service.ChannelMonitorV2Filter,
	cfg service.ChannelMonitorV2Config,
) ([]channelMonitorV2Ignored, error) {
	out := []channelMonitorV2Ignored{}
	if len(cfg.IgnoredErrorCategories) == 0 {
		return out, nil
	}
	bucketExpr, where, args := channelMonitorV2BucketQuery(filter, "e")
	args = append(args, pq.Array(cfg.IgnoredErrorCategories), service.ChannelMonitorV2TaxonomyVersion)
	catIdx := len(args) - 1
	taxIdx := len(args)
	query := fmt.Sprintf(
		`SELECT %s, e.model, e.account_id, SUM(e.error_requests)
		 FROM `+channelMonitorV2ErrorMetricsTable(filter)+` e %s
		 AND e.error_category = ANY($%d) AND e.taxonomy_version = $%d
		 GROUP BY 1, 2, 3`,
		bucketExpr, where, catIdx, taxIdx,
	)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket time.Time
		var row channelMonitorV2Ignored
		if err := rows.Scan(&bucket, &row.Model, &row.AccountID, &row.Count); err != nil {
			return out, err
		}
		row.BucketStart = bucket.UTC().Format(time.RFC3339Nano)
		out = append(out, row)
	}
	return out, rows.Err()
}
