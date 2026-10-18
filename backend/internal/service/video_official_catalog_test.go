//go:build unit

package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// 官方视频条目：下架、播种管理、按秒计费，按次价 = 最低档，各档每秒价与官网（Seedance 为换算价）一致。
func TestOfficialVideoSeeds(t *testing.T) {
	want := map[string]struct {
		vendor string
		tiers  map[string]float64
	}{
		"kling-3.0-turbo":   {PlatformKling, map[string]float64{"720p": 0.112, "1080p": 0.14}},
		"kling-3.0":         {PlatformKling, map[string]float64{"720p": 0.084, "1080p": 0.112, "4k": 0.42}},
		"kling-2.6":         {PlatformKling, map[string]float64{"720p": 0.042, "1080p": 0.07}},
		"kling-2.5-turbo":   {PlatformKling, map[string]float64{"720p": 0.042, "1080p": 0.07}},
		"MiniMax-H3":        {PlatformMiniMax, map[string]float64{"768p": 0.08, "2k": 0.13}},
		"MiniMax-H3-Max":    {PlatformMiniMax, map[string]float64{"480p": 0.05, "768p": 0.08}},
		"seedance-2.5":      {PlatformSeedance, map[string]float64{"480p": 0.0988, "720p": 0.2224, "1080p": 0.5503}},
		"seedance-2.0":      {PlatformSeedance, map[string]float64{"480p": 0.0649, "720p": 0.1461, "1080p": 0.3645, "4k": 0.7433}},
		"seedance-2.0-fast": {PlatformSeedance, map[string]float64{"480p": 0.0522, "720p": 0.1175}},
		"seedance-2.0-mini": {PlatformSeedance, map[string]float64{"480p": 0.0325, "720p": 0.0731}},
	}
	seeds := officialVideoSeeds()
	require.Len(t, seeds, len(want))
	order := map[string]int{}
	for i, resolution := range VideoBillingResolutions() {
		order[resolution] = i
	}
	for _, entry := range seeds {
		w, ok := want[entry.ModelID]
		require.True(t, ok, entry.ModelID)
		require.Equal(t, w.vendor, entry.Vendor, entry.ModelID)
		require.Equal(t, w.vendor, CatalogVendorPlatform(&entry), "条目厂商映射到平台 %s", entry.ModelID)
		require.Equal(t, BillingModeVideo, entry.BillingMode, entry.ModelID)
		require.Equal(t, ModelCatalogStatusUnlisted, entry.Status, entry.ModelID)
		require.Equal(t, ModelCatalogManagedBySeed, entry.ManagedBy, entry.ModelID)
		require.True(t, CatalogVendorAllowed(entry.Vendor), entry.ModelID)
		got := map[string]float64{}
		lowest := ""
		for i, interval := range entry.Intervals {
			got[interval.TierLabel] = *interval.PerRequestPrice
			require.Equal(t, i, interval.SortOrder, entry.ModelID)
			if i > 0 {
				require.Greater(t, order[interval.TierLabel], order[entry.Intervals[i-1].TierLabel], "分档按清晰度从低到高 %s", entry.ModelID)
			} else {
				lowest = interval.TierLabel
			}
		}
		require.Equal(t, w.tiers, got, entry.ModelID)
		require.Equal(t, w.tiers[lowest], *entry.PerRequestPrice, "按次价是最低档的价 %s", entry.ModelID)
		clone := entry
		clone.Normalize()
		require.NoError(t, clone.Validate(), entry.ModelID)
	}
}

// Seedance 每秒价由官方「元 / 百万 token」换算（换算口径见 officialVideoSeeds 的注释），这里按原始数据独立算一遍核对播种的字面值。
func TestSeedanceSeedPricesMatchTokenDerivation(t *testing.T) {
	tokensPerSecond := map[string]float64{"480p": 9600, "720p": 21600, "1080p": 48600, "4k": 194400}
	rmbPerMillion := map[string]map[string]float64{
		"seedance-2.5":      {"480p": 70, "720p": 70, "1080p": 77},
		"seedance-2.0":      {"480p": 46, "720p": 46, "1080p": 51, "4k": 26},
		"seedance-2.0-fast": {"480p": 37, "720p": 37},
		"seedance-2.0-mini": {"480p": 23, "720p": 23},
	}
	const usdPerRMB = 1 / 6.8
	for model, rates := range rmbPerMillion {
		entry := officialVideoSeed(t, model)
		require.Len(t, entry.Intervals, len(rates), model)
		for _, interval := range entry.Intervals {
			rate, ok := rates[interval.TierLabel]
			require.True(t, ok, "%s %s", model, interval.TierLabel)
			derived := math.Round(tokensPerSecond[interval.TierLabel]*rate/1e6*usdPerRMB*1e4) / 1e4
			require.InDelta(t, derived, *interval.PerRequestPrice, 1e-12, "%s %s", model, interval.TierLabel)
		}
	}
}

// 视频分档扩到 768p / 2k / 4k：价格页能存，计费按实际档位计（不再把 768p / 2k / 4k 记成 1080p），渠道成本同口径。
func TestVideoTierBillingForNewResolutions(t *testing.T) {
	bs := NewBillingService()
	ctx := context.Background()
	entries := make([]ModelCatalogEntry, 0, 3)
	for i, model := range []string{"MiniMax-H3", "seedance-2.0", "MiniMax-H3-Max"} {
		entry := *officialVideoSeed(t, model)
		entry.ID = int64(i + 1)
		entry.Status = ModelCatalogStatusListed
		entries = append(entries, entry)
	}
	catalog, _ := newTestModelCatalogService(entries...)
	resolver := NewModelPricingResolver(catalog)
	for _, tc := range []struct {
		model, resolution string
		seconds           int
		perSecond         float64
	}{
		{"MiniMax-H3", "768p", 6, 0.08},
		{"MiniMax-H3", "2k", 10, 0.13},
		{"MiniMax-H3", "2K", 10, 0.13},
		{"seedance-2.0", "4k", 5, 0.7433},
		{"seedance-2.0", "1080p", 5, 0.3645},
		{"seedance-2.0", "480p", 4, 0.0649},
		{"MiniMax-H3-Max", "768P", 15, 0.08},
	} {
		usage := mediaUsage{VideoCount: 1, VideoResolution: NormalizeVideoBillingResolutionOrDefault(tc.resolution), VideoDurationSeconds: tc.seconds}
		cost, handled, err := bs.CalculateMediaCost(ctx, resolver, tc.model, usage, 1)
		require.NoError(t, err)
		require.True(t, handled)
		require.InDelta(t, tc.perSecond*float64(tc.seconds), cost.TotalCost, 1e-9, "%s %s", tc.model, tc.resolution)
	}

	// 渠道成本：承接的上游按次价是基础档的价，其余档按官方同比例（H3：768p 0.04 → 2k 0.04 × 0.13 / 0.08）。
	h3 := officialVideoSeed(t, "MiniMax-H3")
	upstream, err := bs.CalculateUpstreamMediaCost(ctx, resolver, h3, &ModelCatalogBinding{PerRequestPrice: float64Ptr(0.04)},
		mediaUsage{VideoCount: 1, VideoResolution: VideoBillingResolution2K, VideoDurationSeconds: 10})
	require.NoError(t, err)
	require.InDelta(t, 0.04*0.13/0.08*10, upstream, 1e-9)

	// Seedance 2.5 的 30 秒按 30 秒计（框架上限 30）。
	task := &VideoTask{Request: VideoTaskRequest{DurationSeconds: 30, Resolution: VideoResolution1080P}}
	require.Equal(t, 30, task.BillingDurationSeconds())
	task.DurationSeconds = 29.6
	require.Equal(t, 30, task.BillingDurationSeconds(), "上游报的实际时长优先（四舍五入）")
	task.Resolution = VideoResolution2K
	require.Equal(t, VideoBillingResolution2K, task.BillingResolution())
	task.Resolution = ""
	require.Equal(t, VideoBillingResolution1080P, task.BillingResolution())
	require.Equal(t, 30, NormalizeVideoBillingDurationSecondsOrDefault(45))
	require.Equal(t, GrokVideoMaxDurationSeconds, NormalizeGrokVideoDurationSecondsOrDefault(45), "旧 Grok 媒体路径仍按 xAI 的 15 秒封顶")
}

func TestVideoTierLabelsValidation(t *testing.T) {
	tiers := func(labels ...string) []PricingInterval {
		out := make([]PricingInterval, 0, len(labels))
		for _, label := range labels {
			out = append(out, PricingInterval{TierLabel: label, PerRequestPrice: float64Ptr(0.1)})
		}
		return out
	}
	require.NoError(t, ValidateIntervals(tiers("480p", "720p", "768p", "1080p", "2k", "4k"), BillingModeVideo))
	for _, bad := range []string{"8k", "2K", "1440p", "4K"} {
		require.Error(t, ValidateIntervals(tiers(bad), BillingModeVideo), "档位只认规范写法（计费查档区分大小写）：%s", bad)
	}
	for raw, want := range map[string]string{"768P": "768p", "2K": "2k", "1440p": "2k", "4K": "4k", "2160p": "4k", "uhd": "4k"} {
		got, ok := LookupVideoBillingResolution(raw)
		require.True(t, ok, raw)
		require.Equal(t, want, got, raw)
	}
	_, ok := LookupVideoBillingResolution("1080i")
	require.False(t, ok)
}

// validateVideoEntryTier：条目分了档、没有这一档才拒绝。
func TestValidateVideoEntryTier(t *testing.T) {
	entry := officialVideoSeed(t, "kling-3.0")
	require.NoError(t, validateVideoEntryTier(entry, VideoResolution4K))
	require.NoError(t, validateVideoEntryTier(entry, ""), "交给上游默认的不拦")
	requireVideoClientError(t, validateVideoEntryTier(entry, VideoResolution2K), "resolution 2k is not available for model kling-3.0")
	requireVideoClientError(t, validateVideoEntryTier(entry, VideoResolution480P), "resolution 480p")
	require.NoError(t, validateVideoEntryTier(&ModelCatalogEntry{ModelID: "flat", PerRequestPrice: float64Ptr(0.1)}, VideoResolution4K))
}

// video key 测连接直接说明原因（每次探测都是一条要计费的视频），不报「缺协议地址」。
func TestAccountTestServiceVideoKeyHasNoConnectionTest(t *testing.T) {
	key := videoTestOfficialKey(410, PlatformSeedance, DefaultSeedanceCNBaseURL)
	svc, upstream := adaptiveCNAccountTestService(key, adaptiveCNChatTestResponse())
	c, recorder := newTestContext()
	require.Error(t, svc.TestAccountConnection(c, key.ID, "", "", AccountTestModeDefault))
	require.Empty(t, upstream.requests)
	require.Contains(t, recorder.Body.String(), "video keys have no connection test")
}
