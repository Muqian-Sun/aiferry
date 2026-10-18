//go:build unit

package service

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 生图 / 视频 / 按次的模型上价格页（PR-A）：官方按次价与分档、按次售价、承接上的上游按次价，
// 以及媒体用量的渠道成本（上游按次价 × 张数 / 条 × 秒，分档与官方同比例）。

// mediaTestImageEntry 图片条目：标价 0.06 / 张，1K 0.06、2K 0.08；token 价照抄（对话入口不出图时按 token 算），价格页不动它。
func mediaTestImageEntry() ModelCatalogEntry {
	return ModelCatalogEntry{
		ID: 1, ModelID: "media-image-test", BillingMode: BillingModeImage,
		Status: ModelCatalogStatusListed, ManagedBy: ModelCatalogManagedBySeed,
		InputPrice: upstreamCostPtr(2e-6), OutputPrice: upstreamCostPtr(10e-6),
		PerRequestPrice: upstreamCostPtr(0.06),
		Intervals: []PricingInterval{
			{TierLabel: ImageBillingSize1K, PerRequestPrice: upstreamCostPtr(0.06), SortOrder: 0},
			{TierLabel: ImageBillingSize2K, PerRequestPrice: upstreamCostPtr(0.08), SortOrder: 1},
		},
	}
}

// mediaTestVideoEntry 视频条目：标价 0.08 / 秒，480p 0.08、720p 0.14。
func mediaTestVideoEntry() ModelCatalogEntry {
	return ModelCatalogEntry{
		ID: 2, ModelID: "media-video-test", BillingMode: BillingModeVideo,
		Status: ModelCatalogStatusListed, ManagedBy: ModelCatalogManagedByAdmin,
		PerRequestPrice: upstreamCostPtr(0.08),
		Intervals: []PricingInterval{
			{TierLabel: VideoBillingResolution480P, PerRequestPrice: upstreamCostPtr(0.08), SortOrder: 0},
			{TierLabel: VideoBillingResolution720P, PerRequestPrice: upstreamCostPtr(0.14), SortOrder: 1},
		},
	}
}

func mediaTestBinding(entryID, accountID int64, perRequest float64) ModelCatalogBinding {
	return ModelCatalogBinding{EntryID: entryID, AccountID: accountID, PerRequestPrice: upstreamCostPtr(perRequest)}
}

func TestModelCatalogBinding_ValidateAgainstPerRequest(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(b *ModelCatalogBinding)
		wantErr string
	}{
		{name: "upstream per-request price", mutate: func(*ModelCatalogBinding) {}},
		{name: "zero upstream price (subscription account)", mutate: func(b *ModelCatalogBinding) { b.PerRequestPrice = upstreamCostPtr(0) }},
		{name: "missing upstream per-request price", mutate: func(b *ModelCatalogBinding) { b.PerRequestPrice = nil },
			wantErr: "upstream per_request_price is required"},
		{name: "negative upstream per-request price", mutate: func(b *ModelCatalogBinding) { b.PerRequestPrice = upstreamCostPtr(-0.01) },
			wantErr: "upstream per_request_price must be >= 0"},
		{name: "token price on a per-request model", mutate: func(b *ModelCatalogBinding) { b.InputPrice = 1e-6 },
			wantErr: "upstream token prices, segments, search prices, peak hours and max reasoning do not apply"},
		{name: "segments on a per-request model", mutate: func(b *ModelCatalogBinding) {
			b.Intervals = []PricingInterval{{MinTokens: 1000, InputPrice: upstreamCostPtr(1e-6)}}
		}, wantErr: "do not apply"},
		{name: "peak hours on a per-request model", mutate: func(b *ModelCatalogBinding) {
			b.TimePricing = &TimePricing{Timezone: "Asia/Shanghai", Periods: []TimePricingPeriod{{StartTime: "09:00", EndTime: "12:00", Multiplier: 2}}}
		}, wantErr: "do not apply"},
		{name: "upstream model still validated", mutate: func(b *ModelCatalogBinding) { b.UpstreamModel = "img-*" },
			wantErr: "upstream_model must be a single model name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := mediaTestImageEntry()
			b := mediaTestBinding(1, 1, 0.03)
			tc.mutate(&b)
			err := b.ValidateAgainst(&entry)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestModelCatalogService_SaveEntryPricingPerRequestModel(t *testing.T) {
	ctx := context.Background()
	accounts := newPricingTestAccounts()
	official := func() OfficialPrices {
		return OfficialPrices{
			PerRequestPrice: upstreamCostPtr(0.05),
			Intervals: []PricingInterval{
				{ID: 9, TierLabel: ImageBillingSize1K, PerRequestPrice: upstreamCostPtr(0.05), SortOrder: 7},
				{TierLabel: ImageBillingSize2K, PerRequestPrice: upstreamCostPtr(0.07)},
			},
		}
	}

	t.Run("writes per-request official, sale and upstream prices", func(t *testing.T) {
		svc, repo := newTestModelCatalogService(mediaTestImageEntry())
		got, err := svc.SaveEntryPricing(ctx, 1, official(), CatalogSalePrices{PerRequestPrice: upstreamCostPtr(0.004)},
			[]ModelCatalogBinding{mediaTestBinding(0, 1, 0.025), mediaTestBinding(0, 3, 0)}, accounts)
		require.NoError(t, err)
		require.Equal(t, 0.05, *got.PerRequestPrice)
		require.Len(t, got.Intervals, 2)
		require.Equal(t, ImageBillingSize1K, got.Intervals[0].TierLabel)
		require.Zero(t, got.Intervals[0].ID, "request ids are dropped")
		require.Equal(t, 0, got.Intervals[0].SortOrder)
		require.Equal(t, 1, got.Intervals[1].SortOrder)
		require.Equal(t, 0.07, *got.Intervals[1].PerRequestPrice)
		require.Equal(t, 2e-6, *got.InputPrice, "token prices copied onto the image entry are not on the pricing page: kept")
		require.Equal(t, 0.004, *got.SalePrices.PerRequestPrice)
		require.Equal(t, ModelCatalogManagedByAdmin, got.ManagedBy, "a changed per-request price is operator pricing")

		saved := repo.bindings[1]
		require.Len(t, saved, 2)
		require.Equal(t, 0.025, *saved[0].PerRequestPrice)
		require.Equal(t, 0.0, *saved[1].PerRequestPrice, "0 is a valid upstream price")
		require.Zero(t, saved[0].InputPrice)
	})

	t.Run("unchanged per-request prices keep the seed ownership", func(t *testing.T) {
		seed := mediaTestImageEntry()
		svc, _ := newTestModelCatalogService(seed)
		same := OfficialPrices{PerRequestPrice: seed.PerRequestPrice, Intervals: seed.Intervals}
		got, err := svc.SaveEntryPricing(ctx, 1, same, CatalogSalePrices{}, []ModelCatalogBinding{mediaTestBinding(0, 1, 0.03)}, accounts)
		require.NoError(t, err)
		require.Equal(t, ModelCatalogManagedBySeed, got.ManagedBy)
	})

	reject := []struct {
		name     string
		official func() OfficialPrices
		sale     CatalogSalePrices
		bindings []ModelCatalogBinding
		wantErr  string
	}{
		{name: "official per-request price missing",
			official: func() OfficialPrices { o := official(); o.PerRequestPrice = nil; return o },
			wantErr:  "official per_request_price is required"},
		{name: "tier without a price",
			official: func() OfficialPrices { o := official(); o.Intervals[1].PerRequestPrice = nil; return o },
			wantErr:  "official tier 2 has no per_request_price"},
		{name: "tier with token prices",
			official: func() OfficialPrices { o := official(); o.Intervals[0].InputPrice = upstreamCostPtr(1e-6); return o },
			wantErr:  "official tier 1 must only have a per_request_price"},
		{name: "unknown image tier label",
			official: func() OfficialPrices { o := official(); o.Intervals[1].TierLabel = "8K"; return o },
			wantErr:  "unknown image tier_label"},
		{name: "binding without upstream per-request price",
			official: official,
			bindings: []ModelCatalogBinding{mediaTestBinding(0, 1, 0.02), {AccountID: 3, InputPrice: 1e-6, OutputPrice: 2e-6}},
			wantErr:  "upstream per_request_price is required"},
		{name: "token sale prices on a per-request model",
			official: official,
			sale:     CatalogSalePrices{InputPrice: upstreamCostPtr(1e-6)},
			wantErr:  "only have per_request_price"},
		{name: "negative sale per-request price",
			official: official,
			sale:     CatalogSalePrices{PerRequestPrice: upstreamCostPtr(-1)},
			wantErr:  "sale per_request_price must be >= 0"},
	}
	for _, tc := range reject {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			svc, repo := newTestModelCatalogService(mediaTestImageEntry())
			before := []ModelCatalogBinding{mediaTestBinding(1, 3, 0.03)}
			repo.bindings = map[int64][]ModelCatalogBinding{1: before}
			_, err := svc.SaveEntryPricing(ctx, 1, tc.official(), tc.sale, tc.bindings, accounts)
			require.ErrorContains(t, err, tc.wantErr)
			stored, _ := repo.GetEntryByID(ctx, 1)
			require.Equal(t, 0.06, *stored.PerRequestPrice, "nothing written")
			require.Equal(t, before, repo.bindings[1], "nothing written")
		})
	}
}

// 按 Token 计费的模型不用上游按次价：请求里带了也丢掉，其余照旧（行为不变）。
func TestModelCatalogService_SavePricingTokenModelIgnoresPerRequestPrice(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestModelCatalogService(upstreamCostTestEntry())
	b := upstreamCostTestBinding(1)
	b.PerRequestPrice = upstreamCostPtr(0.5)
	official := OfficialPrices{InputPrice: upstreamCostPtr(5e-6), OutputPrice: upstreamCostPtr(30e-6), CacheReadPrice: upstreamCostPtr(0.5e-6)}
	_, err := svc.SaveEntryPricing(ctx, 1, official, CatalogSalePrices{}, []ModelCatalogBinding{b}, newPricingTestAccounts())
	require.NoError(t, err)
	require.Nil(t, repo.bindings[1][0].PerRequestPrice)
	require.Equal(t, 0.15e-6, repo.bindings[1][0].InputPrice)

	saved, err := svc.SaveAccountPricing(ctx, 3, []ModelCatalogBinding{b}, newPricingTestAccounts())
	require.NoError(t, err)
	require.Nil(t, saved[0].PerRequestPrice)

	_, err = svc.SaveEntryPricing(ctx, 1, official, CatalogSalePrices{PerRequestPrice: upstreamCostPtr(0.1)}, nil, newPricingTestAccounts())
	require.ErrorContains(t, err, "sale per_request_price is only for image / video / per-request models")
}

func TestModelCatalogService_SaveAccountPricingPerRequestModel(t *testing.T) {
	ctx := context.Background()
	svc, repo := newTestModelCatalogService(upstreamCostTestEntry(), mediaTestVideoEntry())
	video := mediaTestBinding(2, 0, 0.04)
	saved, err := svc.SaveAccountPricing(ctx, 1, []ModelCatalogBinding{video}, newPricingTestAccounts())
	require.NoError(t, err)
	require.Len(t, saved, 1)
	require.Equal(t, 0.04, *saved[0].PerRequestPrice)
	require.Equal(t, int64(1), repo.bindings[2][0].AccountID)

	missing := ModelCatalogBinding{EntryID: 2}
	_, err = svc.SaveAccountPricing(ctx, 1, []ModelCatalogBinding{missing}, newPricingTestAccounts())
	require.ErrorContains(t, err, "upstream per_request_price is required")
	require.Equal(t, 0.04, *repo.bindings[2][0].PerRequestPrice, "nothing written")
}

// 媒体用量的渠道成本与用户价同一套分发；单价换成上游按次价，各档 × 本档官方价 ÷ 官方按次价。
func TestCalculateUpstreamMediaCost(t *testing.T) {
	ctx := context.Background()
	bs := NewBillingService()

	imageEntry := mediaTestImageEntry()
	imageResolver := newUpstreamCostTestResolver(t, bs, imageEntry, mediaTestBinding(1, 7, 0.03))
	image, imageBinding := imageResolver.LookupUpstreamPrice(ctx, imageEntry.ModelID, 7)
	require.NotNil(t, imageBinding)

	videoEntry := mediaTestVideoEntry()
	videoEntry.ID = 1
	videoResolver := newUpstreamCostTestResolver(t, bs, videoEntry, mediaTestBinding(1, 7, 0.04))
	video, videoBinding := videoResolver.LookupUpstreamPrice(ctx, videoEntry.ModelID, 7)
	require.NotNil(t, videoBinding)

	perRequest := ModelCatalogEntry{ID: 1, ModelID: "media-per-request-test", BillingMode: BillingModePerRequest, Status: ModelCatalogStatusListed,
		ManagedBy: ModelCatalogManagedByAdmin, PerRequestPrice: upstreamCostPtr(2)}
	perRequestResolver := newUpstreamCostTestResolver(t, bs, perRequest, mediaTestBinding(1, 7, 0.5))
	perRequestFound, perRequestBinding := perRequestResolver.LookupUpstreamPrice(ctx, perRequest.ModelID, 7)
	require.NotNil(t, perRequestBinding)

	cases := []struct {
		name     string
		resolver *ModelPricingResolver
		entry    *ModelCatalogEntry
		binding  *ModelCatalogBinding
		usage    mediaUsage
		want     float64
	}{
		{"image base tier x2", imageResolver, image, imageBinding, mediaUsage{ImageCount: 2, ImageSizeTier: ImageBillingSize1K}, 0.06},
		{"image 2K scales with the official tiers", imageResolver, image, imageBinding, mediaUsage{ImageCount: 1, ImageSizeTier: ImageBillingSize2K}, 0.04},
		{"image tier without a price uses the base", imageResolver, image, imageBinding, mediaUsage{ImageCount: 1, ImageSizeTier: ImageBillingSize4K}, 0.03},
		// 0.04 × 0.14 / 0.08 = 0.07 每秒，× 8 秒
		{"video 720p per second", videoResolver, video, videoBinding, mediaUsage{VideoCount: 1, VideoResolution: VideoBillingResolution720P, VideoDurationSeconds: 8}, 0.56},
		{"video on a per-request entry bills per clip", perRequestResolver, perRequestFound, perRequestBinding, mediaUsage{VideoCount: 2, VideoDurationSeconds: 10}, 1.0},
		{"no media usage", imageResolver, image, imageBinding, mediaUsage{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bs.CalculateUpstreamMediaCost(ctx, tc.resolver, tc.entry, tc.binding, tc.usage)
			require.NoError(t, err)
			require.InDelta(t, tc.want, got, 1e-12)
		})
	}

	t.Run("binding without upstream per-request price", func(t *testing.T) {
		bare := ModelCatalogBinding{EntryID: 1, AccountID: 7}
		_, err := bs.CalculateUpstreamMediaCost(ctx, imageResolver, image, &bare, mediaUsage{ImageCount: 1})
		require.ErrorIs(t, err, ErrUpstreamPriceNotFound)
		_, err = bs.CalculateUpstreamMediaCost(ctx, imageResolver, image, nil, mediaUsage{ImageCount: 1})
		require.ErrorIs(t, err, ErrUpstreamPriceNotFound)
	})

	t.Run("official base price missing: every tier uses the upstream price", func(t *testing.T) {
		noBase := mediaTestImageEntry()
		noBase.PerRequestPrice = nil
		resolver := newUpstreamCostTestResolver(t, bs, noBase, mediaTestBinding(1, 7, 0.03))
		found, b := resolver.LookupUpstreamPrice(ctx, noBase.ModelID, 7)
		got, err := bs.CalculateUpstreamMediaCost(ctx, resolver, found, b, mediaUsage{ImageCount: 1, ImageSizeTier: ImageBillingSize2K})
		require.NoError(t, err)
		require.InDelta(t, 0.03, got, 1e-12)
	})
}

func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logs
}

// 找不到承接、或承接没填上游按次价：渠道成本记 0 并告警，不让请求失败。
func TestRecordUsageMediaAccountCost(t *testing.T) {
	ctx := context.Background()
	bs := NewBillingService()
	entry := mediaTestImageEntry()
	resolver := newUpstreamCostTestResolver(t, bs, entry, mediaTestBinding(1, 7, 0.03), ModelCatalogBinding{EntryID: 1, AccountID: 8})
	usage := mediaUsage{ImageCount: 2, ImageSizeTier: ImageBillingSize2K}

	require.InDelta(t, 0.08, recordUsageMediaAccountCost(ctx, bs, resolver, 7, entry.ModelID, usage), 1e-12)

	for name, accountID := range map[string]int64{"no binding": 9, "binding without per-request price": 8} {
		t.Run(name, func(t *testing.T) {
			logs := captureSlog(t)
			require.Zero(t, recordUsageMediaAccountCost(ctx, bs, resolver, accountID, entry.ModelID, usage))
			require.Contains(t, logs.String(), "usage_account_cost_missing_upstream_price")
		})
	}
}

// 按次售价：实付 = 售价（命中的档与官方同比例）× 折扣；TotalCost 仍是官方价。模型广场按同一口径展示。
func TestPerRequestSalePrice(t *testing.T) {
	ctx := context.Background()
	bs := NewBillingService()
	entry := mediaTestImageEntry()
	entry.SalePrices = CatalogSalePrices{PerRequestPrice: upstreamCostPtr(0.0045)}
	catalog, _ := newTestModelCatalogService(entry)
	resolver := NewModelPricingResolver(catalog)

	// 计费倍率 = 折扣 × 默认售价比例：不打折的用户就是 1/15
	cost, handled, err := bs.CalculateMediaCost(ctx, resolver, entry.ModelID, mediaUsage{ImageCount: 1, ImageSizeTier: ImageBillingSize2K}, DefaultSalePriceRatio)
	require.NoError(t, err)
	require.True(t, handled)
	require.InDelta(t, 0.08, cost.TotalCost, 1e-12, "official total")
	require.InDelta(t, 0.006, cost.ActualCost, 1e-12, "sale 0.0045 × 0.08 / 0.06")

	plain := mediaTestImageEntry()
	plainCatalog, _ := newTestModelCatalogService(plain)
	cost, _, err = bs.CalculateMediaCost(ctx, NewModelPricingResolver(plainCatalog), plain.ModelID, mediaUsage{ImageCount: 1, ImageSizeTier: ImageBillingSize2K}, DefaultSalePriceRatio)
	require.NoError(t, err)
	require.InDelta(t, 0.08*DefaultSalePriceRatio, cost.ActualCost, 1e-12, "no sale price: official × default ratio (unchanged)")

	card := entry.SaleEquivalentPricingCard()
	require.InDelta(t, 0.0045, *card.PerRequestPrice*DefaultSalePriceRatio, 1e-12)
	require.InDelta(t, 0.006, *card.Intervals[1].PerRequestPrice*DefaultSalePriceRatio, 1e-12)
	require.Equal(t, plain.PricingCard(), plain.SaleEquivalentPricingCard(), "no sale price: the official card")
}

// 利润门 / 价格页毛利：按次计价的模型比上游按次价与售价口径的按次价；不分忙闲时、没有推理档。
func TestPerRequestBindingCostRatio(t *testing.T) {
	entry := mediaTestImageEntry()
	b := mediaTestBinding(1, 7, 0.003)

	ratio, ok := entry.UpstreamCostRatio(&b)
	require.True(t, ok)
	require.InDelta(t, 0.05, ratio, 1e-12, "0.003 / 0.06")

	withSale := entry
	withSale.SalePrices = CatalogSalePrices{PerRequestPrice: upstreamCostPtr(0.0045)}
	ratio, ok = withSale.UpstreamCostRatio(&b)
	require.True(t, ok)
	require.InDelta(t, 0.003/(0.0045*15), ratio, 1e-12, "compared with the sale price on the official basis")

	peaky := entry
	peaky.TimePricing = &TimePricing{Timezone: "Asia/Shanghai", Periods: []TimePricingPeriod{{StartTime: "09:00", EndTime: "12:00", Multiplier: 2}}}
	peaky.MaxReasoningEffortMultiplier = upstreamCostPtr(3)
	_, worse := peaky.UpstreamPeakCostRatio(&b)
	require.False(t, worse, "media billing has no peak hours")
	_, worse = peaky.UpstreamMaxReasoningCostRatio(&b)
	require.False(t, worse, "media billing has no reasoning effort")
	at := time.Date(2026, 10, 19, 2, 0, 0, 0, time.UTC) // 北京 10:00
	ratio, ok = bindingCostRatioFor(&peaky, &b, at, true)
	require.True(t, ok)
	require.InDelta(t, 0.05, ratio, 1e-12)

	_, ok = entry.UpstreamCostRatio(&ModelCatalogBinding{EntryID: 1, AccountID: 7})
	require.False(t, ok, "no upstream per-request price")
	free := mediaTestImageEntry()
	free.PerRequestPrice = upstreamCostPtr(0)
	ratio, ok = free.UpstreamCostRatio(ptrBinding(mediaTestBinding(1, 7, 0)))
	require.True(t, ok)
	require.Zero(t, ratio)
	_, ok = free.UpstreamCostRatio(&b)
	require.False(t, ok, "free officially but the upstream charges")

	threshold := clampProfitControlThreshold(DefaultSalePriceRatio)
	rejected, _ := profitGateRejectsBinding(&entry, &b, threshold, time.Time{}, false)
	require.False(t, rejected, "0.003 ≤ 0.06 / 15")
	pricey := mediaTestBinding(1, 7, 0.005)
	rejected, reason := profitGateRejectsBinding(&entry, &pricey, threshold, time.Time{}, false)
	require.True(t, rejected)
	require.Equal(t, openAIProfitFilterReasonThreshold, reason)
}

func ptrBinding(b ModelCatalogBinding) *ModelCatalogBinding { return &b }

// 两个网关的媒体记账路径都按上游按次价落渠道成本（原来记 0）。
func TestGatewayServiceRecordUsage_MediaAccountCost(t *testing.T) {
	record := func(t *testing.T, accountID int64) *UsageLog {
		usageRepo := &openAIRecordUsageBestEffortLogRepoStub{}
		svc, apiKey := newGatewayRecordUsageServiceWithResolverForTest(usageRepo)
		svc.resolver = newUpstreamCostTestResolver(t, svc.billingService, mediaTestImageEntry(), mediaTestBinding(1, 7, 0.03))
		err := svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result: &ForwardResult{
				RequestID:  "media_account_cost_test",
				Model:      "media-image-test",
				ImageCount: 2,
				ImageSize:  ImageBillingSize1K,
				Duration:   time.Second,
			},
			APIKey:  apiKey,
			User:    &User{ID: 1, RateMultiplier: officialRate(0.1)},
			Account: &Account{ID: accountID, Platform: PlatformGemini},
		})
		require.NoError(t, err)
		require.NotNil(t, usageRepo.lastLog)
		return usageRepo.lastLog
	}

	log := record(t, 7)
	require.Equal(t, string(BillingModeImage), *log.BillingMode)
	require.InDelta(t, 0.12, log.TotalCost, 1e-12)
	require.InDelta(t, 0.06, log.AccountCost, 1e-12, "2 × 0.03")

	require.Zero(t, record(t, 8).AccountCost, "unbound channel records zero")
}

func TestOpenAIGatewayServiceRecordUsage_MediaAccountCost(t *testing.T) {
	record := func(t *testing.T, entry ModelCatalogEntry, binding ModelCatalogBinding, result *OpenAIForwardResult) *UsageLog {
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		svc.resolver = newUpstreamCostTestResolver(t, svc.billingService, entry, binding)
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result:  result,
			APIKey:  &APIKey{ID: 10},
			User:    &User{ID: 20, RateMultiplier: officialRate(0.25)},
			Account: &Account{ID: 7},
		})
		require.NoError(t, err)
		require.NotNil(t, usageRepo.lastLog)
		return usageRepo.lastLog
	}

	t.Run("image", func(t *testing.T) {
		log := record(t, mediaTestImageEntry(), mediaTestBinding(1, 7, 0.03), &OpenAIForwardResult{
			RequestID: "openai-media-image", Model: "media-image-test", BillingModel: "media-image-test",
			ImageCount: 2, ImageSize: ImageBillingSize1K, Duration: time.Second,
		})
		require.InDelta(t, 0.12, log.TotalCost, 1e-12)
		require.InDelta(t, 0.06, log.AccountCost, 1e-12)
	})

	t.Run("video", func(t *testing.T) {
		entry := mediaTestVideoEntry()
		entry.ID = 1
		log := record(t, entry, mediaTestBinding(1, 7, 0.04), &OpenAIForwardResult{
			RequestID: "openai-media-video", ResponseID: "openai-media-video", Model: "media-video-test", BillingModel: "media-video-test",
			VideoCount: 1, VideoResolution: VideoBillingResolution720P, VideoDurationSeconds: 8, Duration: time.Second,
		})
		require.InDelta(t, 0.14*8, log.TotalCost, 1e-12)
		require.InDelta(t, 0.56, log.AccountCost, 1e-12, "0.04 × 0.14 / 0.08 × 8 s")
	})
}
