package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/xai"

// mediaCatalogSeeds 是价格文件里没有的媒体模型官方价：图片按张、视频按秒，分档在 Intervals。
// 与 SnapshotFallbackPricing 同规则：价格文件已有该模型时不播（buildModelCatalogSeedEntries）。
func mediaCatalogSeeds() []ModelCatalogEntry {
	return append(xaiImagineSeeds(), officialVideoSeeds()...)
}

// xaiImagineSeeds 是 xAI Imagine 官方价（原 billing_service.go 的 defaultGrokImagine* 常量）。
func xaiImagineSeeds() []ModelCatalogEntry {
	image := func(modelID, notes string, defaultPrice, price1K, price2K float64) ModelCatalogEntry {
		entry := mediaSeedEntry(modelID, "xai", BillingModeImage, defaultPrice, notes)
		entry.Intervals = []PricingInterval{
			{TierLabel: ImageBillingSize1K, PerRequestPrice: float64Ptr(price1K), SortOrder: 0},
			{TierLabel: ImageBillingSize2K, PerRequestPrice: float64Ptr(price2K), SortOrder: 1},
		}
		return entry
	}
	video := func(modelID, notes string, defaultPrice float64, perSecond map[string]float64) ModelCatalogEntry {
		return videoSeedEntry(modelID, "xai", notes, defaultPrice, perSecond)
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

// officialVideoSeeds 官方视频厂商的每秒价（USD），按次价 = 最低档的价。
//
// 可灵、MiniMax 是官网的每秒美元价。Seedance 官方按「元 / 百万 token」计，每秒价是换算的：
// 每秒 token = 宽 × 高 × 24 ÷ 1024（按 16:9 取像素：480p 9600、720p 21600、1080p 48600、4k 194400），
// 元 / 百万 token：2.5 为 70（480p / 720p）与 77（1080p）；2.0 为 46（480p / 720p）、51（1080p）、26（4k）；
// 2.0 fast 37；2.0 mini 23；美元 = 元 ÷ 6.8（产品定的汇率）。结果四舍五入到 4 位小数。
func officialVideoSeeds() []ModelCatalogEntry {
	video := func(modelID, vendor, notes string, perSecond map[string]float64) ModelCatalogEntry {
		base := 0.0
		for _, resolution := range VideoBillingResolutions() {
			if price, ok := perSecond[resolution]; ok {
				base = price
				break
			}
		}
		return videoSeedEntry(modelID, vendor, notes, base, perSecond)
	}
	return []ModelCatalogEntry{
		video("kling-3.0-turbo", PlatformKling, "可灵 3.0 Turbo 视频，每秒；分档按分辨率",
			map[string]float64{VideoBillingResolution720P: 0.112, VideoBillingResolution1080P: 0.14}),
		video("kling-3.0", PlatformKling, "可灵 3.0 视频，每秒；分档按分辨率",
			map[string]float64{VideoBillingResolution720P: 0.084, VideoBillingResolution1080P: 0.112, VideoBillingResolution4K: 0.42}),
		video("kling-2.6", PlatformKling, "可灵 2.6 视频，每秒；分档按分辨率",
			map[string]float64{VideoBillingResolution720P: 0.042, VideoBillingResolution1080P: 0.07}),
		video("kling-2.5-turbo", PlatformKling, "可灵 2.5 Turbo 视频，每秒；分档按分辨率",
			map[string]float64{VideoBillingResolution720P: 0.042, VideoBillingResolution1080P: 0.07}),
		video("MiniMax-H3", PlatformMiniMax, "MiniMax H3 视频，每秒；分档按分辨率",
			map[string]float64{VideoBillingResolution768P: 0.08, VideoBillingResolution2K: 0.13}),
		video("MiniMax-H3-Max", PlatformMiniMax, "MiniMax H3 Max 视频，每秒；分档按分辨率",
			map[string]float64{VideoBillingResolution480P: 0.05, VideoBillingResolution768P: 0.08}),
		video("seedance-2.5", PlatformSeedance, "Seedance 2.5 视频，每秒（由官方 token 价换算）；分档按分辨率",
			map[string]float64{VideoBillingResolution480P: 0.0988, VideoBillingResolution720P: 0.2224, VideoBillingResolution1080P: 0.5503}),
		video("seedance-2.0", PlatformSeedance, "Seedance 2.0 视频，每秒（由官方 token 价换算）；分档按分辨率",
			map[string]float64{VideoBillingResolution480P: 0.0649, VideoBillingResolution720P: 0.1461, VideoBillingResolution1080P: 0.3645, VideoBillingResolution4K: 0.7433}),
		video("seedance-2.0-fast", PlatformSeedance, "Seedance 2.0 Fast 视频，每秒（由官方 token 价换算）；分档按分辨率",
			map[string]float64{VideoBillingResolution480P: 0.0522, VideoBillingResolution720P: 0.1175}),
		video("seedance-2.0-mini", PlatformSeedance, "Seedance 2.0 Mini 视频，每秒（由官方 token 价换算）；分档按分辨率",
			map[string]float64{VideoBillingResolution480P: 0.0325, VideoBillingResolution720P: 0.0731}),
	}
}

// videoSeedEntry 视频条目：perSecond 按计费档从低到高排进 Intervals。
func videoSeedEntry(modelID, vendor, notes string, defaultPrice float64, perSecond map[string]float64) ModelCatalogEntry {
	entry := mediaSeedEntry(modelID, vendor, BillingModeVideo, defaultPrice, notes)
	for _, resolution := range VideoBillingResolutions() {
		price, ok := perSecond[resolution]
		if !ok {
			continue
		}
		entry.Intervals = append(entry.Intervals, PricingInterval{TierLabel: resolution, PerRequestPrice: float64Ptr(price), SortOrder: len(entry.Intervals)})
	}
	return entry
}

func mediaSeedEntry(modelID, vendor string, mode BillingMode, defaultPrice float64, notes string) ModelCatalogEntry {
	return ModelCatalogEntry{
		ModelID:         modelID,
		Vendor:          vendor,
		BillingMode:     mode,
		Status:          ModelCatalogStatusUnlisted,
		ManagedBy:       ModelCatalogManagedBySeed,
		PerRequestPrice: float64Ptr(defaultPrice),
		Notes:           &notes,
	}
}
