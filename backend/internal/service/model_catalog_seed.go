package service

import (
	"context"
	"log/slog"
	"sort"
	"strings"
)

// ModelCatalogSeedInput 是播种的两个默认价来源：
//   - pricingService：LiteLLM 格式价格文件（基准价主来源）
//   - billingService：硬编码兜底价表 initFallbackPricing
type ModelCatalogSeedInput struct {
	PricingService *PricingService
	BillingService *BillingService
}

// Seed 用默认价来源补齐目录。
//
// 规则（幂等、可重复执行）：
//   - 模型标识在目录里不存在 → 插入，managed_by = seed；
//   - 已存在且 managed_by = seed → 用当前价格文件刷新，保证价格文件更新后目录不会冻在播种时刻；
//   - 已存在且 managed_by = admin → 整条跳过，管理员改动永不被覆盖。
//
// 播种只写「条目本身」：别名、分档、分时没有默认数据来源，不会被播种写入，
// 也不会被播种刷新掉。
func (s *ModelCatalogService) Seed(ctx context.Context) (ModelCatalogSeedResult, error) {
	if s == nil || s.repo == nil {
		return ModelCatalogSeedResult{}, nil
	}
	candidates := buildModelCatalogSeedEntries(s.seedInput)
	result := ModelCatalogSeedResult{CandidateModels: len(candidates)}

	valid := make([]ModelCatalogEntry, 0, len(candidates))
	for i := range candidates {
		entry := candidates[i]
		entry.Normalize()
		if err := entry.Validate(); err != nil {
			// fail-loud：默认价来源里出现不合法条目时打出来，而不是静默少播一条。
			slog.Warn("skipping invalid model catalog seed entry",
				"model_id", entry.ModelID, "error", err)
			result.SkippedInvalid++
			continue
		}
		valid = append(valid, entry)
	}

	written, err := s.repo.InsertOrRefreshSeedEntries(ctx, valid)
	if err != nil {
		return result, err
	}
	result.Inserted = written.Inserted
	result.Refreshed = written.Refreshed
	result.SkippedAdmin = written.SkippedAdmin
	s.invalidate(ctx)
	return result, nil
}

// buildModelCatalogSeedEntries 把两个默认价来源展开成目录条目。
// 同一个模型标识两边都有时以价格文件为准——今天的解析顺序就是
// 「价格文件 → 硬编码兜底价」，播种不能把这个优先级翻过来。
func buildModelCatalogSeedEntries(input ModelCatalogSeedInput) []ModelCatalogEntry {
	byKey := make(map[string]ModelCatalogEntry)

	if input.PricingService != nil {
		for name, pricing := range input.PricingService.SnapshotModelPricing() {
			if pricing == nil {
				continue
			}
			// 仅有图片价、没有 token 价的条目今天会被 getModelPricingAt 明确拒绝
			// （否则 token 流量按 $0 计费）。播种它们等于把这条拒绝绕过去。
			if pricing.TokenPricingAbsent {
				continue
			}
			entry := seedEntryFromLiteLLM(name, pricing)
			byKey[NormalizeModelCatalogKey(entry.ModelID)] = entry
		}
	}

	if input.BillingService != nil {
		for name, pricing := range input.BillingService.SnapshotFallbackPricing() {
			if pricing == nil {
				continue
			}
			key := NormalizeModelCatalogKey(name)
			if _, exists := byKey[key]; exists {
				continue
			}
			// 价格文件的查表阶梯（拼写变体、去日期后缀、系列匹配）也能定到价时，
			// 今天走的是价格文件那一份。把兜底价播进目录会让目录反过来压住它，
			// 所以这里跳过。
			if input.PricingService != nil && input.PricingService.GetModelPricing(name) != nil {
				continue
			}
			byKey[key] = seedEntryFromFallback(name, pricing)
		}
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	entries := make([]ModelCatalogEntry, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, byKey[key])
	}
	return entries
}

// positivePrice 把 0 / 负数视为「未配置」返回 nil。
// 价格文件用零值表示缺失，播种成 0 会让目录把「没配这一项」变成「显式收 0」，
// 进而关掉下游的回退（如图片输出价回退到文本输出价）。
func positivePrice(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	v := value
	return &v
}

func seedEntryFromLiteLLM(name string, pricing *LiteLLMModelPricing) ModelCatalogEntry {
	entry := ModelCatalogEntry{
		ModelID: name,
		Vendor:  strings.ToLower(strings.TrimSpace(pricing.LiteLLMProvider)),
		// 播种一律记 token：今天没有渠道/分组价卡的模型在解析器里就是 token 模式，
		// 图片 / 视频计费走的是另一条读 PricingService 的路径。播成 image/video
		// 会让 CalculateCostUnified 改走按次分支，属于行为改动。
		BillingMode: BillingModeToken,
		Status:      ModelCatalogStatusListed,
		ManagedBy:   ModelCatalogManagedBySeed,

		InputPrice:          positivePrice(pricing.InputCostPerToken),
		OutputPrice:         positivePrice(pricing.OutputCostPerToken),
		CacheWritePrice:     positivePrice(pricing.CacheCreationInputTokenCost),
		CacheReadPrice:      positivePrice(pricing.CacheReadInputTokenCost),
		ImageInputPrice:     positivePrice(pricing.InputCostPerImageToken),
		ImageOutputPrice:    positivePrice(pricing.OutputCostPerImageToken),
		ImageCacheReadPrice: positivePrice(pricing.CacheReadInputImageTokenCost),

		InputPricePriority:      positivePrice(pricing.InputCostPerTokenPriority),
		OutputPricePriority:     positivePrice(pricing.OutputCostPerTokenPriority),
		CacheWritePricePriority: positivePrice(pricing.CacheCreationInputTokenCostPriority),
		CacheReadPricePriority:  positivePrice(pricing.CacheReadInputTokenCostPriority),

		LongContextInputMultiplier:  positivePrice(pricing.LongContextInputCostMultiplier),
		LongContextOutputMultiplier: positivePrice(pricing.LongContextOutputCostMultiplier),
		// xAI 的长上下文阈值语义是「达到即进高档」，其余提供商严格大于，
		// 口径与 getModelPricingAt 一致。
		LongContextThresholdInclusive: strings.EqualFold(pricing.LiteLLMProvider, "xai"),
	}
	// 5m/1h 分档只在 1h 价严格高于 5m 价时成立，与 getModelPricingAt 同口径：
	// 价格文件写反时不分档，避免把 1h 缓存按更低的价算。
	if pricing.CacheCreationInputTokenCostAbove1hr > 0 &&
		pricing.CacheCreationInputTokenCostAbove1hr > pricing.CacheCreationInputTokenCost {
		entry.CacheWrite1hPrice = positivePrice(pricing.CacheCreationInputTokenCostAbove1hr)
	}
	if pricing.LongContextInputTokenThreshold > 0 {
		threshold := pricing.LongContextInputTokenThreshold
		entry.LongContextInputThreshold = &threshold
	}
	return entry
}

func seedEntryFromFallback(name string, pricing *ModelPricing) ModelCatalogEntry {
	entry := ModelCatalogEntry{
		ModelID:     name,
		BillingMode: BillingModeToken,
		Status:      ModelCatalogStatusListed,
		ManagedBy:   ModelCatalogManagedBySeed,

		InputPrice:          positivePrice(pricing.InputPricePerToken),
		OutputPrice:         positivePrice(pricing.OutputPricePerToken),
		CacheWritePrice:     positivePrice(pricing.CacheCreationPricePerToken),
		CacheReadPrice:      positivePrice(pricing.CacheReadPricePerToken),
		ImageInputPrice:     positivePrice(pricing.ImageInputPricePerToken),
		ImageOutputPrice:    positivePrice(pricing.ImageOutputPricePerToken),
		ImageCacheReadPrice: positivePrice(pricing.ImageCacheReadPricePerToken),

		InputPricePriority:      positivePrice(pricing.InputPricePerTokenPriority),
		OutputPricePriority:     positivePrice(pricing.OutputPricePerTokenPriority),
		CacheWritePricePriority: positivePrice(pricing.CacheCreationPricePerTokenPriority),
		CacheReadPricePriority:  positivePrice(pricing.CacheReadPricePerTokenPriority),

		LongContextInputMultiplier:    positivePrice(pricing.LongContextInputMultiplier),
		LongContextOutputMultiplier:   positivePrice(pricing.LongContextOutputMultiplier),
		LongContextThresholdInclusive: pricing.LongContextThresholdInclusive,

		FastMultiplier:               clonePricePtr(pricing.FastMultiplier),
		FlexMultiplier:               clonePricePtr(pricing.FlexMultiplier),
		MaxReasoningEffortMultiplier: clonePricePtr(pricing.MaxReasoningEffortMultiplier),
	}
	if pricing.SupportsCacheBreakdown &&
		pricing.CacheCreation1hPrice > 0 &&
		pricing.CacheCreation1hPrice > pricing.CacheCreation5mPrice {
		entry.CacheWrite1hPrice = positivePrice(pricing.CacheCreation1hPrice)
	}
	if pricing.LongContextInputThreshold > 0 {
		threshold := pricing.LongContextInputThreshold
		entry.LongContextInputThreshold = &threshold
	}
	return entry
}

func clonePricePtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}
