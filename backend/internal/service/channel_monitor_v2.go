package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	ChannelMonitorV2TaxonomyVersion = 1
	// ChannelMonitorV2RefreshIntervalSeconds 聚合任务的汇总频率，也是服务状态页的自动刷新间隔。
	ChannelMonitorV2RefreshIntervalSeconds = 60
)

var (
	ErrChannelMonitorV2InvalidRange = errors.New("invalid channel monitor v2 range")
	ErrChannelMonitorV2InvalidModel = errors.New("invalid channel monitor v2 model")
)

// ChannelMonitorV2Config 渠道健康（用户站「服务状态」）一次读数用的配置。
// 汇总与评分写在代码里（muqian 2026-09-30），管理站不再有配置页，见 channelMonitorV2Config；
// 模型名单每次读数由服务按上架目录填入。
type ChannelMonitorV2Config struct {
	HealthThresholds ChannelMonitorV2HealthThresholds
	// IgnoredErrorCategories 不计入错误率与健康评分的错误类别（多是调用方自己的问题）。
	IgnoredErrorCategories []string
	Roster                 ChannelMonitorV2ModelRoster
}

// channelMonitorV2Config 服务状态的评分配置。错误率与首字延迟取原配置表全新安装时的生效值（迁移 198 / 205）：
// 首字延迟目标 3 秒、8 秒不稳定、20 秒异常；错误率 5% 不稳定、20% 异常；满 50 个请求才评状态。
// 缓存命中率参与评分（muqian 2026-09-30）：85% 以下提醒、60% 以下异常，取迁移 203 当时的出厂值。
// 忽略的错误类别去掉了分类里已不存在的 group_access。
var channelMonitorV2Config = ChannelMonitorV2Config{
	HealthThresholds: ChannelMonitorV2HealthThresholds{
		MinimumSample:     50,
		WarningErrorRate:  0.05,
		CriticalErrorRate: 0.20,
		TargetTTFTMs:      3000,
		WarningTTFTMs:     8000,
		CriticalTTFTMs:    20000,
		WarningCacheRate:  0.85,
		CriticalCacheRate: 0.60,
		ErrorWeight:       0.60,
		TTFTWeight:        0.20,
		CacheWeight:       0.20,
	},
	IgnoredErrorCategories: []string{
		"authentication",
		"client_cancelled",
		"content_policy",
		"context_limit",
		"model_unsupported",
		"not_found",
		"quota_or_balance",
	},
}

// ChannelMonitorV2ModelRoster 服务状态的模型名单 = 上架目录（muqian 2026-09-30）。
// 只统计名单里的模型：请求名（含别名）经 Resolve 归到条目本名，名单外的流量不计；
// 也不按上游平台筛——访客看的是这个模型好不好用，与哪个上游在服务无关。
type ChannelMonitorV2ModelRoster struct {
	Models  []string
	Resolve func(model string) (string, bool)
}

// ChannelMonitorV2ErrorCategories is the ordered, versioned taxonomy used by the
// classifier. Unmatched errors become "other".
var ChannelMonitorV2ErrorCategories = []string{
	"content_policy",
	"authentication",
	"context_limit",
	"invalid_request",
	"model_unsupported",
	"quota_or_balance",
	"account_pool_unavailable",
	"rate_or_capacity",
	"timeout",
	"transport_or_stream",
	"upstream_forbidden",
	"not_found",
	"client_cancelled",
	"upstream_5xx",
	"internal",
	"other",
}

type ChannelMonitorV2Filter struct {
	Range  string
	Start  time.Time
	End    time.Time
	Bucket time.Duration
}

type ChannelMonitorV2Metric struct {
	SuccessRequests          int64                   `json:"success_requests"`
	ErrorRequests            int64                   `json:"error_requests"`
	RequestCount             int64                   `json:"request_count"`
	InputTokens              int64                   `json:"input_tokens"`
	OutputTokens             int64                   `json:"output_tokens"`
	CacheCreationTokens      int64                   `json:"cache_creation_tokens"`
	CacheReadTokens          int64                   `json:"cache_read_tokens"`
	TokenCount               int64                   `json:"token_count"`
	RPM                      float64                 `json:"rpm"`
	TPM                      float64                 `json:"tpm"`
	ErrorRate                float64                 `json:"error_rate"`
	SuccessRate              float64                 `json:"success_rate"`
	CacheRate                float64                 `json:"cache_rate"`
	CacheRateNumerator       int64                   `json:"cache_rate_numerator"`
	CacheRateDenominator     int64                   `json:"cache_rate_denominator"`
	TTFT                     ChannelMonitorV2Latency `json:"ttft"`
	Duration                 ChannelMonitorV2Latency `json:"duration"`
	UpstreamAffectedRequests *int64                  `json:"upstream_affected_requests,omitempty"`
	UpstreamAttemptCount     *int64                  `json:"upstream_attempt_count,omitempty"`
}

type ChannelMonitorV2Latency struct {
	SampleCount int64    `json:"sample_count"`
	P50Ms       *int64   `json:"p50_ms"`
	P90Ms       *int64   `json:"p90_ms"`
	P95Ms       *int64   `json:"p95_ms"`
	AvgMs       *float64 `json:"avg_ms"`
}

type ChannelMonitorV2Health struct {
	Overall   string `json:"overall"`
	ErrorRate string `json:"error_rate"`
	TTFT      string `json:"ttft"`
	Cache     string `json:"cache"`
	// Score is 0–100 when samples are sufficient; omitted/null when unknown.
	// Overall blends error-rate, TTFT p50, and cache rate (weights in Thresholds).
	Score          *float64                         `json:"score,omitempty"`
	ErrorRateScore *float64                         `json:"error_rate_score,omitempty"`
	TTFTScore      *float64                         `json:"ttft_score,omitempty"`
	CacheScore     *float64                         `json:"cache_score,omitempty"`
	MinimumSample  int64                            `json:"minimum_sample"`
	Thresholds     ChannelMonitorV2HealthThresholds `json:"thresholds"`
}

type ChannelMonitorV2HealthThresholds struct {
	// MinimumSample is required before scoring request/latency/cache signals.
	MinimumSample int64 `json:"minimum_sample"`
	// WarningErrorRate / CriticalErrorRate map to discrete bands for legacy UI.
	WarningErrorRate  float64 `json:"warning_error_rate"`
	CriticalErrorRate float64 `json:"critical_error_rate"`
	// TargetTTFTMs is the primary TTFT p50 budget (ms). At or below this is full TTFT score.
	TargetTTFTMs   int64 `json:"target_ttft_ms"`
	WarningTTFTMs  int64 `json:"warning_ttft_ms"`
	CriticalTTFTMs int64 `json:"critical_ttft_ms"`
	// WarningCacheRate / CriticalCacheRate: cache rate below these → warning/critical bands.
	// Higher cache rate is better.
	WarningCacheRate  float64 `json:"warning_cache_rate"`
	CriticalCacheRate float64 `json:"critical_cache_rate"`
	// ErrorWeight + TTFTWeight + CacheWeight should sum to 1.0.
	ErrorWeight float64 `json:"error_weight"`
	TTFTWeight  float64 `json:"ttft_weight"`
	CacheWeight float64 `json:"cache_weight"`
}

type ChannelMonitorV2Coverage struct {
	RequestedStart time.Time `json:"requested_start"`
	// RequestedEnd is the exclusive upper bound of the UI-selected window
	// (filter.End). Charts/matrices should plot [RequestedStart, RequestedEnd)
	// even when CoverageStart is later (partial backfill).
	RequestedEnd time.Time `json:"requested_end"`
	// CoverageStart / DataThrough 汇总实际覆盖到的起止：读数按它们与请求窗口的交集算比率
	CoverageStart time.Time `json:"coverage_start"`
	DataThrough   time.Time `json:"data_through"`
	BucketSeconds int       `json:"bucket_seconds"`
}

// ChannelMonitorV2AggregationWatermark is the durable aggregator cursor state.
type ChannelMonitorV2AggregationWatermark struct {
	UsageCoverageStart time.Time
	ErrorCoverageStart time.Time
	DataThrough        time.Time
	LastSuccessfulAt   time.Time
	// BackfillCursor is the earliest start already recomputed; zero when never set.
	BackfillCursor time.Time
	// HasData is true once any successful recompute wrote data_through.
	HasData bool
}

type ChannelMonitorV2TrendPoint struct {
	BucketStart time.Time              `json:"bucket_start"`
	Metrics     ChannelMonitorV2Metric `json:"metrics"`
	Health      ChannelMonitorV2Health `json:"health"`
}

type ChannelMonitorV2Snapshot struct {
	Coverage ChannelMonitorV2Coverage
	Metrics  ChannelMonitorV2Metric
	Health   ChannelMonitorV2Health
	Trend    []ChannelMonitorV2TrendPoint
}

type ChannelMonitorV2MatrixRow struct {
	Model   string
	Metrics ChannelMonitorV2Metric
	Health  ChannelMonitorV2Health
	Buckets []ChannelMonitorV2TrendPoint
}

type ChannelMonitorV2Matrix struct {
	Coverage ChannelMonitorV2Coverage
	Items    []ChannelMonitorV2MatrixRow
}

// ChannelMonitorV2ChannelRow 管理站渠道状态的一行：一个渠道。AccountID 0 = 没选到渠道就失败的请求。
type ChannelMonitorV2ChannelRow struct {
	AccountID                    int64
	Name, Platform, Type, Status string
	Deleted                      bool
	Metrics                      ChannelMonitorV2Metric
	Health                       ChannelMonitorV2Health
	Buckets                      []ChannelMonitorV2TrendPoint
}

// ChannelMonitorV2Channels 管理站渠道状态（muqian 2026-09-30）：全部渠道合计的数字与趋势 + 每个渠道一行；
// Models 是上架模型，给按模型筛选用。
type ChannelMonitorV2Channels struct {
	ChannelMonitorV2Snapshot
	Items  []ChannelMonitorV2ChannelRow
	Models []string
}

type ChannelMonitorV2Repository interface {
	GetSnapshot(ctx context.Context, filter ChannelMonitorV2Filter, config ChannelMonitorV2Config) (*ChannelMonitorV2Snapshot, error)
	// GetMatrix 逐模型（名单里每个模型一行）的总量与逐段数据。
	GetMatrix(ctx context.Context, filter ChannelMonitorV2Filter, config ChannelMonitorV2Config) (*ChannelMonitorV2Matrix, error)
	// GetChannels 全部流量按渠道聚合；model 非空时只算请求名解析到这个上架模型的。
	GetChannels(ctx context.Context, filter ChannelMonitorV2Filter, config ChannelMonitorV2Config, model string) (*ChannelMonitorV2Channels, error)
	// GetAggregationWatermark loads durable backfill / coverage cursors for the
	// passive aggregator (and bootstrap progress). Missing row → zero value, nil error.
	GetAggregationWatermark(ctx context.Context) (*ChannelMonitorV2AggregationWatermark, error)
	RecomputeRange(ctx context.Context, start, end time.Time) error
}

// ChannelMonitorV2Service 用户站「服务状态」的读数（全站快照与逐模型矩阵，对未登录访客公开）
// 与管理站「渠道状态」的读数（按渠道）。
type ChannelMonitorV2Service struct {
	repo    ChannelMonitorV2Repository
	catalog CatalogListingSource
	now     func() time.Time
}

// NewChannelMonitorV2Service catalog 生产上是 *ModelCatalogService：模型名单按它的上架条目出。
func NewChannelMonitorV2Service(repo ChannelMonitorV2Repository, catalog CatalogListingSource) *ChannelMonitorV2Service {
	return &ChannelMonitorV2Service{repo: repo, catalog: catalog, now: func() time.Time { return time.Now().UTC() }}
}

// readConfig 一次读数用的配置：代码里的评分配置 + 上架目录名单。
func (s *ChannelMonitorV2Service) readConfig(ctx context.Context) ChannelMonitorV2Config {
	entries := s.catalog.ListListedEntries(ctx)
	models := make([]string, 0, len(entries))
	for _, entry := range entries {
		models = append(models, entry.ModelID)
	}
	// 一次读数里同一个请求名会出现在很多段里，解析结果记下来（空串 = 不在名单里）
	resolved := map[string]string{}
	cfg := channelMonitorV2Config
	cfg.Roster = ChannelMonitorV2ModelRoster{
		Models: models,
		Resolve: func(model string) (string, bool) {
			id, seen := resolved[model]
			if !seen {
				if route, ok := s.catalog.ResolveRoute(ctx, model); ok {
					id = route.CanonicalModel
				}
				resolved[model] = id
			}
			return id, id != ""
		},
	}
	return cfg
}

func (s *ChannelMonitorV2Service) ParseFilter(rangeValue string) (ChannelMonitorV2Filter, error) {
	now := s.now().UTC()
	var window, bucket time.Duration
	switch strings.TrimSpace(rangeValue) {
	case "", "90m":
		rangeValue, window, bucket = "90m", 90*time.Minute, 5*time.Minute
	case "24h":
		window, bucket = 24*time.Hour, time.Hour
	case "7d":
		window, bucket = 7*24*time.Hour, 12*time.Hour
	case "30d":
		window, bucket = 30*24*time.Hour, 24*time.Hour
	default:
		return ChannelMonitorV2Filter{}, fmt.Errorf("%w: %s", ErrChannelMonitorV2InvalidRange, rangeValue)
	}
	start, end := now.Add(-window), now
	if bucket > time.Minute {
		// Align display windows to a fixed number of whole buckets. The trailing
		// bucket may point slightly into the future; SQL simply has no future rows,
		// while the current partial bucket can still be shown and recomputed often.
		end = now.Truncate(bucket).Add(bucket)
		start = end.Add(-window)
	}
	return ChannelMonitorV2Filter{Range: rangeValue, Start: start, End: end, Bucket: bucket}, nil
}

func (s *ChannelMonitorV2Service) Snapshot(ctx context.Context, filter ChannelMonitorV2Filter) (*ChannelMonitorV2Snapshot, error) {
	return s.repo.GetSnapshot(ctx, filter, s.readConfig(ctx))
}

func (s *ChannelMonitorV2Service) Matrix(ctx context.Context, filter ChannelMonitorV2Filter) (*ChannelMonitorV2Matrix, error) {
	return s.repo.GetMatrix(ctx, filter, s.readConfig(ctx))
}

// Channels 管理站渠道状态：全部流量按渠道聚合（不只上架模型），model 非空时只算请求名解析到这个上架模型的。
func (s *ChannelMonitorV2Service) Channels(ctx context.Context, filter ChannelMonitorV2Filter, model string) (*ChannelMonitorV2Channels, error) {
	cfg := s.readConfig(ctx)
	model = strings.TrimSpace(model)
	if model != "" && !slices.Contains(cfg.Roster.Models, model) {
		return nil, fmt.Errorf("%w: %s", ErrChannelMonitorV2InvalidModel, model)
	}
	result, err := s.repo.GetChannels(ctx, filter, cfg, model)
	if err != nil {
		return nil, err
	}
	result.Models = cfg.Roster.Models
	return result, nil
}

func ChannelMonitorV2HealthFor(metrics ChannelMonitorV2Metric) ChannelMonitorV2Health {
	return ChannelMonitorV2HealthForWithThresholds(metrics, channelMonitorV2Config.HealthThresholds)
}

func ChannelMonitorV2HealthForWithThresholds(metrics ChannelMonitorV2Metric, thresholds ChannelMonitorV2HealthThresholds) ChannelMonitorV2Health {
	result := ChannelMonitorV2Health{
		Overall: "unknown", ErrorRate: "unknown", TTFT: "unknown", Cache: "unknown",
		MinimumSample: thresholds.MinimumSample, Thresholds: thresholds,
	}

	type scored struct {
		score  float64
		weight float64
		band   string
	}
	parts := make([]scored, 0, 3)

	if metrics.RequestCount >= result.MinimumSample {
		s := errorRateScore(metrics.ErrorRate, thresholds.CriticalErrorRate)
		result.ErrorRateScore = &s
		result.ErrorRate = healthBand(metrics.ErrorRate, thresholds.WarningErrorRate, thresholds.CriticalErrorRate)
		parts = append(parts, scored{score: s, weight: thresholds.ErrorWeight, band: result.ErrorRate})
	}
	// Prefer p50 for TTFT scoring; fall back to p95 only if p50 is missing.
	if metrics.TTFT.SampleCount >= result.MinimumSample {
		var ttftMs *int64
		if metrics.TTFT.P50Ms != nil {
			ttftMs = metrics.TTFT.P50Ms
		} else if metrics.TTFT.P95Ms != nil {
			ttftMs = metrics.TTFT.P95Ms
		}
		if ttftMs != nil {
			s := ttftP50Score(float64(*ttftMs), float64(thresholds.TargetTTFTMs), float64(thresholds.CriticalTTFTMs))
			result.TTFTScore = &s
			result.TTFT = healthBand(float64(*ttftMs), float64(thresholds.WarningTTFTMs), float64(thresholds.CriticalTTFTMs))
			parts = append(parts, scored{score: s, weight: thresholds.TTFTWeight, band: result.TTFT})
		}
	}
	// Cache: same request-count gate as error rate (the token denominator alone would let a
	// handful of long requests rate a model on cache only); higher rate is better.
	if metrics.RequestCount >= result.MinimumSample && metrics.CacheRateDenominator > 0 {
		s := cacheRateScore(metrics.CacheRate)
		result.CacheScore = &s
		// Invert for healthBand (lower is worse): use (1 - rate) against warning/critical floors.
		result.Cache = cacheRateBand(metrics.CacheRate, thresholds.WarningCacheRate, thresholds.CriticalCacheRate)
		parts = append(parts, scored{score: s, weight: thresholds.CacheWeight, band: result.Cache})
	}

	if len(parts) == 0 {
		return result
	}
	var weightSum, scoreSum float64
	for _, p := range parts {
		weightSum += p.weight
		scoreSum += p.weight * p.score
	}
	if weightSum <= 0 {
		return result
	}
	overall := scoreSum / weightSum
	result.Score = &overall
	result.Overall = scoreBand(overall)
	return result
}

// errorRateScore maps error rate to 0–100. 0% → 100; at/above critical → 0 (linear).
func errorRateScore(errorRate, critical float64) float64 {
	if critical <= 0 {
		critical = 0.05
	}
	if errorRate <= 0 {
		return 100
	}
	if errorRate >= critical {
		return 0
	}
	return 100 * (1 - errorRate/critical)
}

// ttftP50Score maps TTFT p50 ms to 0–100.
// At/below target → 100; at/above critical → 0; linear in between.
func ttftP50Score(p50Ms, targetMs, criticalMs float64) float64 {
	if targetMs <= 0 {
		targetMs = 2500
	}
	if criticalMs <= targetMs {
		criticalMs = targetMs * 2.4
	}
	if p50Ms <= targetMs {
		return 100
	}
	if p50Ms >= criticalMs {
		return 0
	}
	return 100 * (1 - (p50Ms-targetMs)/(criticalMs-targetMs))
}

// cacheRateScore maps cache hit rate to 0–100 (higher is better, linear).
func cacheRateScore(cacheRate float64) float64 {
	if cacheRate <= 0 {
		return 0
	}
	if cacheRate >= 1 {
		return 100
	}
	return 100 * cacheRate
}

// cacheRateBand: below critical → critical; below warning → warning; else healthy.
func cacheRateBand(cacheRate, warning, critical float64) string {
	if cacheRate < critical {
		return "critical"
	}
	if cacheRate < warning {
		return "warning"
	}
	return "healthy"
}

// scoreBand maps continuous 0–100 scores to coarse labels for legacy consumers.
func scoreBand(score float64) string {
	switch {
	case score >= 80:
		return "healthy"
	case score >= 50:
		return "warning"
	default:
		return "critical"
	}
}

func healthBand(value, warning, critical float64) string {
	if value >= critical {
		return "critical"
	}
	if value >= warning {
		return "warning"
	}
	return "healthy"
}
