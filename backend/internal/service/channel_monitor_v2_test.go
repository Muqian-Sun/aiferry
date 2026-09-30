package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// channelMonitorV2RepoStub 记下最近一次读数收到的配置（看上架名单带没带上）。
type channelMonitorV2RepoStub struct {
	cfg ChannelMonitorV2Config
}

func (s *channelMonitorV2RepoStub) GetSnapshot(_ context.Context, _ ChannelMonitorV2Filter, cfg ChannelMonitorV2Config) (*ChannelMonitorV2Snapshot, error) {
	s.cfg = cfg
	return &ChannelMonitorV2Snapshot{}, nil
}
func (s *channelMonitorV2RepoStub) GetMatrix(_ context.Context, _ ChannelMonitorV2Filter, cfg ChannelMonitorV2Config) (*ChannelMonitorV2Matrix, error) {
	s.cfg = cfg
	return &ChannelMonitorV2Matrix{}, nil
}
func (s *channelMonitorV2RepoStub) GetAggregationWatermark(context.Context) (*ChannelMonitorV2AggregationWatermark, error) {
	return &ChannelMonitorV2AggregationWatermark{}, nil
}
func (s *channelMonitorV2RepoStub) RecomputeRange(context.Context, time.Time, time.Time) error {
	return nil
}

// channelMonitorV2CatalogStub 上架目录：ids 是上架条目本名，aliases 把别名指到本名。
type channelMonitorV2CatalogStub struct {
	ids     []string
	aliases map[string]string
}

func (s channelMonitorV2CatalogStub) ListListedEntries(context.Context) []ModelCatalogEntry {
	entries := make([]ModelCatalogEntry, 0, len(s.ids))
	for i, id := range s.ids {
		entries = append(entries, ModelCatalogEntry{ID: int64(i + 1), ModelID: id, Status: ModelCatalogStatusListed})
	}
	return entries
}

func (s channelMonitorV2CatalogStub) ResolveRoute(_ context.Context, model string) (CatalogRoute, bool) {
	if canonical, ok := s.aliases[model]; ok {
		model = canonical
	}
	for i, id := range s.ids {
		if id == model {
			return CatalogRoute{EntryID: int64(i + 1), CanonicalModel: id, RequestedModel: model}, true
		}
	}
	return CatalogRoute{}, false
}

func TestChannelMonitorV2BootstrapProgress(t *testing.T) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

	t.Run("no data shows active zero progress", func(t *testing.T) {
		b := ChannelMonitorV2BootstrapProgress(now, time.Time{}, false)
		require.NotNil(t, b)
		require.True(t, b.Active)
		require.Equal(t, 0, b.ProgressPercent)
		require.Equal(t, now.Add(-ChannelMonitorV2BootstrapProductWindow), b.TargetStart)
	})

	t.Run("2h seed is small percent of 30d", func(t *testing.T) {
		covered := now.Add(-2 * time.Hour)
		b := ChannelMonitorV2BootstrapProgress(now, covered, true)
		require.NotNil(t, b)
		require.True(t, b.Active)
		require.Greater(t, b.ProgressPercent, 0)
		require.Less(t, b.ProgressPercent, 5)
	})

	t.Run("30d covered hides bootstrap", func(t *testing.T) {
		covered := now.Add(-ChannelMonitorV2BootstrapProductWindow)
		b := ChannelMonitorV2BootstrapProgress(now, covered, true)
		require.Nil(t, b)
	})

	t.Run("beyond 30d also hides", func(t *testing.T) {
		covered := now.Add(-60 * 24 * time.Hour)
		b := ChannelMonitorV2BootstrapProgress(now, covered, true)
		require.Nil(t, b)
	})
}

func TestChannelMonitorV2ParseFilterDefaultsAndBuckets(t *testing.T) {
	svc := &ChannelMonitorV2Service{now: func() time.Time { return time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC) }}

	filter, err := svc.ParseFilter("")
	require.NoError(t, err)
	require.Equal(t, "90m", filter.Range)
	require.Equal(t, 5*time.Minute, filter.Bucket)
	require.Equal(t, 90*time.Minute, filter.End.Sub(filter.Start))

	filter, err = svc.ParseFilter("30d")
	require.NoError(t, err)
	require.Equal(t, 24*time.Hour, filter.Bucket)
	_, err = svc.ParseFilter("15d")
	require.ErrorIs(t, err, ErrChannelMonitorV2InvalidRange)
}

func TestChannelMonitorV2ErrorTaxonomyPriority(t *testing.T) {
	tests := []struct {
		name string
		in   ChannelMonitorV2ErrorInput
		want string
	}{
		{"cyber", ChannelMonitorV2ErrorInput{ErrorType: "cyber_policy", StatusCode: 200}, "content_policy"},
		{"auth before forbidden", ChannelMonitorV2ErrorInput{StatusCode: 403, Message: "invalid API key"}, "authentication"},
		{"context", ChannelMonitorV2ErrorInput{Message: "maximum prompt length exceeded"}, "context_limit"},
		{"unsupported", ChannelMonitorV2ErrorInput{Message: "not supported by any configured account"}, "model_unsupported"},
		{"pool", ChannelMonitorV2ErrorInput{Message: "No available accounts"}, "account_pool_unavailable"},
		{"timeout", ChannelMonitorV2ErrorInput{Message: "error code: 524"}, "timeout"},
		{"upstream", ChannelMonitorV2ErrorInput{ErrorOwner: "provider", StatusCode: 502, UpstreamStatusCode: 502}, "upstream_5xx"},
		{"other", ChannelMonitorV2ErrorInput{StatusCode: 400, Message: "unknown"}, "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, ClassifyChannelMonitorV2Error(tt.in)) })
	}
}

func TestChannelMonitorV2HealthBlendsErrorTTFTAndCache(t *testing.T) {
	// error 3%/5% → 40; ttft p50 2s → 100; cache 50% → 50
	// overall = (0.6*40 + 0.2*100 + 0.2*50) / 1.0 = 54 → warning
	p50 := int64(2000)
	p95 := int64(9000)
	thresholds := ChannelMonitorV2HealthThresholds{
		MinimumSample:     20,
		WarningErrorRate:  0.02,
		CriticalErrorRate: 0.05,
		TargetTTFTMs:      2500,
		WarningTTFTMs:     2501,
		CriticalTTFTMs:    6000,
		WarningCacheRate:  0.20,
		CriticalCacheRate: 0.05,
		ErrorWeight:       0.60,
		TTFTWeight:        0.20,
		CacheWeight:       0.20,
	}
	metrics := ChannelMonitorV2Metric{
		RequestCount:         100,
		ErrorRate:            0.03,
		CacheRate:            0.50,
		CacheRateDenominator: 100,
		TTFT:                 ChannelMonitorV2Latency{SampleCount: 100, P50Ms: &p50, P95Ms: &p95},
	}
	health := ChannelMonitorV2HealthForWithThresholds(metrics, thresholds)
	require.Equal(t, "warning", health.ErrorRate)
	require.Equal(t, "healthy", health.TTFT)
	require.Equal(t, "healthy", health.Cache)
	require.NotNil(t, health.Score)
	require.NotNil(t, health.CacheScore)
	require.InDelta(t, 50.0, *health.CacheScore, 0.01)
	require.InDelta(t, 54.0, *health.Score, 0.01)
	require.Equal(t, "warning", health.Overall)

	// Perfect signals → 100
	p50OK := int64(1000)
	metrics.ErrorRate = 0
	metrics.CacheRate = 1
	metrics.TTFT.P50Ms = &p50OK
	health = ChannelMonitorV2HealthForWithThresholds(metrics, thresholds)
	require.Equal(t, "healthy", health.Overall)
	require.NotNil(t, health.Score)
	require.InDelta(t, 100.0, *health.Score, 0.01)

	// Small samples stay unknown
	health = ChannelMonitorV2HealthFor(ChannelMonitorV2Metric{RequestCount: 2})
	require.Equal(t, "unknown", health.Overall)
	require.Nil(t, health.Score)
}

func TestChannelMonitorV2HealthLeavesMissingTTFTUnknown(t *testing.T) {
	health := ChannelMonitorV2HealthFor(ChannelMonitorV2Metric{
		RequestCount:         200,
		ErrorRate:            0,
		CacheRate:            1,
		CacheRateDenominator: 200,
		TTFT:                 ChannelMonitorV2Latency{SampleCount: 0},
	})
	require.Equal(t, "unknown", health.TTFT)
	require.Nil(t, health.TTFTScore)
	require.NotEqual(t, "critical", health.Overall)
	require.Equal(t, "healthy", health.Overall)
}

// 缓存命中率参与评分（muqian 2026-09-30）：命中率高时不拖分；低于 60% 时缓存档是异常，并按 20% 权重拉低总分。
func TestChannelMonitorV2HealthScoresCacheHitRate(t *testing.T) {
	p50 := int64(2500)
	metrics := ChannelMonitorV2Metric{
		RequestCount:         100,
		ErrorRate:            0.03,
		CacheRate:            0.9,
		CacheRateDenominator: 100,
		TTFT:                 ChannelMonitorV2Latency{SampleCount: 100, P50Ms: &p50},
	}
	health := ChannelMonitorV2HealthFor(metrics)
	require.Equal(t, "healthy", health.ErrorRate)
	require.Equal(t, "healthy", health.TTFT)
	require.Equal(t, "healthy", health.Cache)
	require.Equal(t, "healthy", health.Overall)

	// 错误率 3%（20% 封顶 → 85 分）、首字 100 分、缓存 50% → 50 分：0.6×85 + 0.2×100 + 0.2×50 = 81
	metrics.CacheRate = 0.5
	health = ChannelMonitorV2HealthFor(metrics)
	require.Equal(t, "critical", health.Cache)
	require.NotNil(t, health.Score)
	require.InDelta(t, 81.0, *health.Score, 0.01)
}

// 请求不满 50 个时三项都不评：以前缓存只看 token 数，几个长请求就能单凭缓存给出「正常」。
func TestChannelMonitorV2HealthNeedsMinimumRequestsForCache(t *testing.T) {
	health := ChannelMonitorV2HealthFor(ChannelMonitorV2Metric{
		RequestCount:         4,
		CacheRate:            0.9,
		CacheRateDenominator: 5000,
	})
	require.Nil(t, health.CacheScore)
	require.Equal(t, "unknown", health.Cache)
	require.Equal(t, "unknown", health.Overall)
}

func TestErrorRateTTFTAndCacheScoreHelpers(t *testing.T) {
	require.InDelta(t, 100.0, errorRateScore(0, 0.05), 0.001)
	require.InDelta(t, 0.0, errorRateScore(0.05, 0.05), 0.001)
	require.InDelta(t, 40.0, errorRateScore(0.03, 0.05), 0.001)

	require.InDelta(t, 100.0, ttftP50Score(2500, 2500, 6000), 0.001)
	require.InDelta(t, 100.0, ttftP50Score(1000, 2500, 6000), 0.001)
	require.InDelta(t, 0.0, ttftP50Score(6000, 2500, 6000), 0.001)
	require.InDelta(t, 50.0, ttftP50Score(4250, 2500, 6000), 0.001)

	require.InDelta(t, 0.0, cacheRateScore(0), 0.001)
	require.InDelta(t, 100.0, cacheRateScore(1), 0.001)
	require.InDelta(t, 50.0, cacheRateScore(0.5), 0.001)
	require.Equal(t, "critical", cacheRateBand(0.02, 0.20, 0.05))
	require.Equal(t, "warning", cacheRateBand(0.10, 0.20, 0.05))
	require.Equal(t, "healthy", cacheRateBand(0.50, 0.20, 0.05))
}

func TestChannelMonitorV2HealthTTFTAtTargetIsHealthy(t *testing.T) {
	p50 := int64(2500)
	m := ChannelMonitorV2Metric{
		RequestCount: 100, ErrorRate: 0,
		CacheRate: 1, CacheRateDenominator: 100,
		TTFT: ChannelMonitorV2Latency{SampleCount: 100, P50Ms: &p50},
	}
	h := ChannelMonitorV2HealthFor(m)
	require.Equal(t, "healthy", h.TTFT)
	require.NotNil(t, h.TTFTScore)
	require.InDelta(t, 100.0, *h.TTFTScore, 0.01)
}

// 服务状态按上架目录出模型（muqian 2026-09-30）：快照与各模型两个读数都带上架名单，
// 别名归到本名，名单外解析不到；评分配置用代码里的那份。
func TestChannelMonitorV2ReadsUseListedCatalog(t *testing.T) {
	catalog := channelMonitorV2CatalogStub{ids: []string{"gpt-5.5", "grok-4.6"}, aliases: map[string]string{"grok-latest": "grok-4.6"}}
	repo := &channelMonitorV2RepoStub{}
	svc := NewChannelMonitorV2Service(repo, catalog)
	reads := map[string]func() error{
		"snapshot": func() error {
			_, err := svc.Snapshot(context.Background(), ChannelMonitorV2Filter{})
			return err
		},
		"matrix": func() error {
			_, err := svc.Matrix(context.Background(), ChannelMonitorV2Filter{})
			return err
		},
	}
	for name, read := range reads {
		repo.cfg = ChannelMonitorV2Config{}
		require.NoError(t, read(), name)
		require.Equal(t, []string{"gpt-5.5", "grok-4.6"}, repo.cfg.Roster.Models, name)
		id, ok := repo.cfg.Roster.Resolve("grok-latest")
		require.True(t, ok, name)
		require.Equal(t, "grok-4.6", id, name)
		_, ok = repo.cfg.Roster.Resolve("claude-opus-4")
		require.False(t, ok, name)
		require.Equal(t, channelMonitorV2Config.HealthThresholds, repo.cfg.HealthThresholds, name)
		require.Equal(t, channelMonitorV2Config.IgnoredErrorCategories, repo.cfg.IgnoredErrorCategories, name)
	}
}

// 写进代码的配置：错误率与首字取原配置表全新安装时的生效值（迁移 198 / 205），缓存取迁移 203 的出厂值；
// 忽略的类别都在分类里。
func TestChannelMonitorV2ConfigValues(t *testing.T) {
	require.Equal(t, ChannelMonitorV2HealthThresholds{
		MinimumSample: 50, WarningErrorRate: 0.05, CriticalErrorRate: 0.20,
		TargetTTFTMs: 3000, WarningTTFTMs: 8000, CriticalTTFTMs: 20000,
		WarningCacheRate: 0.85, CriticalCacheRate: 0.60,
		ErrorWeight: 0.60, TTFTWeight: 0.20, CacheWeight: 0.20,
	}, channelMonitorV2Config.HealthThresholds)
	require.Equal(t, 60, ChannelMonitorV2RefreshIntervalSeconds)
	for _, category := range channelMonitorV2Config.IgnoredErrorCategories {
		require.Contains(t, ChannelMonitorV2ErrorCategories, category)
	}
}
