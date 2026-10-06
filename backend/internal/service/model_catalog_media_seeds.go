package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/xai"

// xaiImagineSeeds 是价格文件里没有的 xAI Imagine 官方价（原 billing_service.go 的
// defaultGrokImagine* 常量）：图片按张、视频按秒，分档在 Intervals。
// 与 SnapshotFallbackPricing 同规则：价格文件已有该模型时不播（buildModelCatalogSeedEntries）。
func xaiImagineSeeds() []ModelCatalogEntry {
	image := func(modelID, notes string, defaultPrice, price1K, price2K float64) ModelCatalogEntry {
		entry := mediaSeedEntry(modelID, BillingModeImage, defaultPrice, notes)
		entry.Intervals = []PricingInterval{
			{TierLabel: ImageBillingSize1K, PerRequestPrice: float64Ptr(price1K), SortOrder: 0},
			{TierLabel: ImageBillingSize2K, PerRequestPrice: float64Ptr(price2K), SortOrder: 1},
		}
		return entry
	}
	video := func(modelID, notes string, defaultPrice float64, perSecond map[string]float64) ModelCatalogEntry {
		entry := mediaSeedEntry(modelID, BillingModeVideo, defaultPrice, notes)
		for i, resolution := range []string{VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution1080P} {
			price, ok := perSecond[resolution]
			if !ok {
				continue
			}
			entry.Intervals = append(entry.Intervals, PricingInterval{TierLabel: resolution, PerRequestPrice: float64Ptr(price), SortOrder: i})
		}
		return entry
	}
	return []ModelCatalogEntry{
		image(xai.DefaultImagineImageFastModel, "xAI Imagine 图片，每张；分档按输出尺寸", 0.02, 0.02, 0.02),
		image(xai.DefaultImagineImageQualityModel, "xAI Imagine 图片（quality），每张；分档按输出尺寸", 0.05, 0.05, 0.07),
		image(xai.DefaultImagineImage20Model, "xAI Imagine 2.0 图片，每张；分档按输出尺寸", 0.06, 0.06, 0.08),
		video(xai.DefaultImagineVideoModel, "xAI Imagine 视频，每秒；分档按分辨率", 0.05,
			map[string]float64{VideoBillingResolution480P: 0.05, VideoBillingResolution720P: 0.07}),
		video(xai.DefaultImagineVideo15Model, "xAI Imagine 1.5 视频，每秒；分档按分辨率", 0.08,
			map[string]float64{VideoBillingResolution480P: 0.08, VideoBillingResolution720P: 0.14, VideoBillingResolution1080P: 0.25}),
	}
}

func mediaSeedEntry(modelID string, mode BillingMode, defaultPrice float64, notes string) ModelCatalogEntry {
	return ModelCatalogEntry{
		ModelID:         modelID,
		Vendor:          "xai",
		BillingMode:     mode,
		Status:          ModelCatalogStatusUnlisted,
		ManagedBy:       ModelCatalogManagedBySeed,
		PerRequestPrice: float64Ptr(defaultPrice),
		Notes:           &notes,
	}
}
