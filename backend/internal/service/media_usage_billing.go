package service

import (
	"context"
	"fmt"
)

// mediaUsage 是一次转发里可计费的非 token 用量，从 ForwardResult / OpenAIForwardResult 投影。
type mediaUsage struct {
	ImageCount           int
	ImageSizeTier        string // NormalizeImageBillingTierOrDefault(ImageSize)
	VideoCount           int
	VideoResolution      string
	VideoDurationSeconds int
	// Audio 是 Grok Voice（tts / stt / realtime）的用量，按内置单价。
	Audio *AudioUsage
}

func mediaUsageFromForwardResult(r *ForwardResult) mediaUsage {
	if r == nil {
		return mediaUsage{}
	}
	return mediaUsage{
		ImageCount:    r.ImageCount,
		ImageSizeTier: NormalizeImageBillingTierOrDefault(r.ImageSize),
		Audio:         r.AudioUsage,
	}
}

func mediaUsageFromOpenAIForwardResult(r *OpenAIForwardResult) mediaUsage {
	if r == nil {
		return mediaUsage{}
	}
	return mediaUsage{
		ImageCount:           r.ImageCount,
		ImageSizeTier:        NormalizeImageBillingTierOrDefault(r.ImageSize),
		VideoCount:           r.VideoCount,
		VideoResolution:      r.VideoResolution,
		VideoDurationSeconds: r.VideoDurationSeconds,
		Audio:                r.AudioUsage,
	}
}

// CalculateMediaCost 是两个网关共用的媒体计价：按目录条目的模式分发，图片 / 视频只认目录，
// 不再有分组价与默认价。handled=false 表示这次用量走 token 路径（没有媒体用量，或图片落在
// token 模式的条目上——gpt-image-* 按 image token 价）。
//
//	Audio != nil       → 没有价：语音（Grok tts / stt / realtime）不在目录里，接口已关（muqian 2026-10-06）
//	VideoCount > 0     → mode video：单价(分辨率档) × 条 × 秒；mode image/per_request：单价 × 条
//	ImageCount > 0 且 mode ∈ {image, per_request} → 单价(尺寸档) × 张
func (s *BillingService) CalculateMediaCost(ctx context.Context, resolver *ModelPricingResolver, model string, usage mediaUsage, multiplier float64) (cost *CostBreakdown, handled bool, err error) {
	if usage.Audio != nil {
		return nil, true, fmt.Errorf("audio usage for %q: %w", model, ErrModelPricingUnavailable)
	}
	if usage.VideoCount <= 0 && usage.ImageCount <= 0 {
		return nil, false, nil
	}
	resolved := resolveIfPossible(ctx, resolver, model)
	if usage.VideoCount > 0 {
		if resolved == nil || resolved.Source != PricingSourceCatalog {
			return nil, true, fmt.Errorf("video usage for %q: %w", model, ErrModelPricingUnavailable)
		}
		count := usage.VideoCount
		resolution := NormalizeVideoBillingResolutionOrDefault(usage.VideoResolution)
		seconds := NormalizeVideoBillingDurationSecondsOrDefault(usage.VideoDurationSeconds)
		switch resolved.Mode {
		case BillingModeVideo:
			cost, err = s.CalculateCostUnified(CostInput{
				Ctx: ctx, Model: model, RequestCount: count, UsageUnits: float64(count * seconds),
				SizeTier: resolution, RateMultiplier: multiplier, Resolver: resolver, Resolved: resolved,
			})
		case BillingModePerRequest, BillingModeImage:
			// 按次口径：价格由管理员按次配置，不乘时长。
			cost, err = s.CalculateCostUnified(CostInput{
				Ctx: ctx, Model: model, RequestCount: count, UsageUnits: float64(count),
				SizeTier: resolution, RateMultiplier: multiplier, Resolver: resolver, Resolved: resolved,
			})
		default:
			// 上架校验要求 video 条目带按次价；到这里说明校验被绕过，不按 token 价静默算。
			return nil, true, fmt.Errorf("video usage on token-priced catalog entry %q: %w", model, ErrModelPricingUnavailable)
		}
		return cost, true, err
	}
	// ImageCount > 0
	if resolved == nil || resolved.Source != PricingSourceCatalog {
		return nil, true, fmt.Errorf("image usage for %q: %w", model, ErrModelPricingUnavailable)
	}
	switch resolved.Mode {
	case BillingModeImage, BillingModePerRequest:
		cost, err = s.CalculateCostUnified(CostInput{
			Ctx: ctx, Model: model, RequestCount: usage.ImageCount, SizeTier: usage.ImageSizeTier,
			RateMultiplier: multiplier, Resolver: resolver, Resolved: resolved,
		})
		return cost, true, err
	default:
		// token 模式的生图条目（gpt-image-*）：按 image token 价走 token 路径。
		return nil, false, nil
	}
}

func resolveIfPossible(ctx context.Context, resolver *ModelPricingResolver, model string) *ResolvedPricing {
	if resolver == nil {
		return nil
	}
	return resolver.Resolve(ctx, PricingInput{Model: model})
}
