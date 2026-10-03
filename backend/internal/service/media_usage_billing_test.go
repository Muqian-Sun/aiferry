//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// CalculateMediaCost 是两个网关共用的媒体计价：按目录条目模式分发，图片 / 视频只认目录，
// seed 与 admin 条目一视同仁；音频 / grok 搜索用内置常量；alpha search 用条目 search_price_per_call。
func TestCalculateMediaCost(t *testing.T) {
	bs := NewBillingService(&config.Config{}, nil)
	ctx := context.Background()
	p := func(v float64) *float64 { return &v }

	imageCard := PricingCard{Models: []string{"img"}, BillingMode: BillingModeImage, PerRequestPrice: p(0.10),
		Intervals: []PricingInterval{{TierLabel: ImageBillingSize1K, PerRequestPrice: p(0.10)}, {TierLabel: ImageBillingSize2K, PerRequestPrice: p(0.19)}}}
	videoCard := PricingCard{Models: []string{"vid"}, BillingMode: BillingModeVideo, PerRequestPrice: p(0.05),
		Intervals: []PricingInterval{{TierLabel: VideoBillingResolution720P, PerRequestPrice: p(0.14)}}}
	perRequestVideoCard := PricingCard{Models: []string{"vid-per-request"}, BillingMode: BillingModePerRequest, PerRequestPrice: p(2.0)}
	tokenCard := PricingCard{Models: []string{"tok"}, BillingMode: BillingModeToken, InputPrice: p(1e-6), OutputPrice: p(2e-6), ImageOutputPrice: p(4e-5)}
	adminResolver := newResolverWithCatalogCards(bs, imageCard, videoCard, perRequestVideoCard, tokenCard)

	// seed 条目：与 admin 同一份价卡，只是 managed_by=seed
	seedEntry := catalogEntryFromCard("img", ModelCatalogManagedBySeed, imageCard)
	seedEntry.ID = 1
	seedCatalog, _ := newTestModelCatalogService(seedEntry)
	seedResolver := NewModelPricingResolver(seedCatalog, bs)

	cases := []struct {
		name       string
		resolver   *ModelPricingResolver
		model      string
		usage      mediaUsage
		multiplier float64
		handled    bool
		wantErr    bool
		total      float64
		actual     float64
		mode       string
	}{
		{"image 1K x2 (admin)", adminResolver, "img", mediaUsage{ImageCount: 2, ImageSizeTier: ImageBillingSize1K}, 1.5, true, false, 0.20, 0.30, "image"},
		{"image 1K x2 (seed entry honored)", seedResolver, "img", mediaUsage{ImageCount: 2, ImageSizeTier: ImageBillingSize1K}, 1.5, true, false, 0.20, 0.30, "image"},
		{"image 4K falls back to default price", adminResolver, "img", mediaUsage{ImageCount: 1, ImageSizeTier: ImageBillingSize4K}, 1, true, false, 0.10, 0.10, "image"},
		{"video 720p x1 8s", adminResolver, "vid", mediaUsage{VideoCount: 1, VideoResolution: VideoBillingResolution720P, VideoDurationSeconds: 8}, 1, true, false, 1.12, 1.12, "video"},
		{"video 480p (no tier) default per second x 5s", adminResolver, "vid", mediaUsage{VideoCount: 1, VideoResolution: VideoBillingResolution480P, VideoDurationSeconds: 5}, 2, true, false, 0.25, 0.50, "video"},
		{"video on per_request entry bills per clip not per second", adminResolver, "vid-per-request", mediaUsage{VideoCount: 2, VideoDurationSeconds: 10}, 1, true, false, 4.0, 4.0, "per_request"},
		{"image on token entry goes to token path", adminResolver, "tok", mediaUsage{ImageCount: 2, ImageSizeTier: ImageBillingSize1K}, 1, false, false, 0, 0, ""},
		{"video on token entry is an error, not zero", adminResolver, "tok", mediaUsage{VideoCount: 1, VideoDurationSeconds: 5}, 1, true, true, 0, 0, ""},
		{"image on unknown model is an error", adminResolver, "nope", mediaUsage{ImageCount: 1, ImageSizeTier: ImageBillingSize1K}, 1, true, true, 0, 0, ""},
		{"audio tts 0.5M chars at built-in 15/M", adminResolver, "tok", mediaUsage{Audio: &AudioUsage{Mode: "tts", DurationOrUnits: 0.5}}, 1, true, false, 7.5, 7.5, "per_request"},
		{"no media usage is not handled", adminResolver, "tok", mediaUsage{}, 1, false, false, 0, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cost, handled, err := bs.CalculateMediaCost(ctx, tc.resolver, tc.model, tc.usage, tc.multiplier)
			require.Equal(t, tc.handled, handled, "handled")
			if !tc.handled {
				require.Nil(t, cost)
				require.NoError(t, err)
				return
			}
			if tc.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrModelPricingUnavailable)
				require.Nil(t, cost)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cost)
			require.InDelta(t, tc.total, cost.TotalCost, 1e-9, "total")
			require.InDelta(t, tc.actual, cost.ActualCost, 1e-9, "actual")
			require.Equal(t, tc.mode, cost.BillingMode)
		})
	}
}

// 用户价 = 目录价 × 用户倍率：seed 与 admin 条目在图片路径上给出同一个价（原来只认 admin 条目）。
func TestCalculateMediaCost_SeedAndAdminEntriesPriceTheSame(t *testing.T) {
	bs := NewBillingService(&config.Config{}, nil)
	price := 0.25
	card := PricingCard{Models: []string{"gpt-image-x"}, BillingMode: BillingModeImage, PerRequestPrice: &price}
	for _, managedBy := range []string{ModelCatalogManagedBySeed, ModelCatalogManagedByAdmin} {
		entry := catalogEntryFromCard("gpt-image-x", managedBy, card)
		entry.ID = 1
		catalog, _ := newTestModelCatalogService(entry)
		resolver := NewModelPricingResolver(catalog, bs)
		cost, handled, err := bs.CalculateMediaCost(context.Background(), resolver, "gpt-image-x", mediaUsage{ImageCount: 3, ImageSizeTier: ImageBillingSize1K}, 2)
		require.NoError(t, err, managedBy)
		require.True(t, handled, managedBy)
		require.InDelta(t, 0.75, cost.TotalCost, 1e-9, managedBy)
		require.InDelta(t, 1.5, cost.ActualCost, 1e-9, managedBy)
	}
}
