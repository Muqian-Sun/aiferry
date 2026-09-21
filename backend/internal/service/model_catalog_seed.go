package service

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"
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
// 播种写条目本身；种子自带的分档（Intervals）与别名（SeedAliases）也随之写入 / 刷新
// （目前只有 xAI Imagine 种子带），分时没有默认数据来源。
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
	result.Inserted = written.Inserted
	result.Refreshed = written.Refreshed
	result.SkippedAdmin = written.SkippedAdmin
	result.Failed = written.Failed
	result.Errors = written.Errors
	if result.Inserted+result.Refreshed > 0 {
		// 整体中止（ctx 到期）前可能已经写进去一部分，照样失效缓存让它们生效。
		s.invalidate(ctx)
	}
	return result, err
}

// modelCatalogReseedTimeout 是价格文件更新后自动重播的时长上限（同启动播种，拍的）。
const modelCatalogReseedTimeout = 60 * time.Second

// ReseedAfterPricingUpdate 在价格文件更新后重播一次，让 seed 条目跟上新价；
// admin 条目照旧不动。跑在价格服务的调度 goroutine 里，用独立 ctx。
func (s *ModelCatalogService) ReseedAfterPricingUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), modelCatalogReseedTimeout)
	defer cancel()
	result, err := s.Seed(ctx)
	if err != nil {
		slog.Warn("model catalog reseed after pricing update aborted", "error", err,
			"inserted", result.Inserted, "refreshed", result.Refreshed)
		return
	}
	slog.Info("model catalog reseeded after pricing update",
		"inserted", result.Inserted, "refreshed", result.Refreshed,
		"skipped_admin", result.SkippedAdmin, "skipped_invalid", result.SkippedInvalid, "failed", result.Failed)
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
			entry := seedEntryFromLiteLLM(name, pricing)
			// token 模式却没有 token 价（理论不可达：解析期已丢掉无价条目）——播进去会让
			// token 流量按 $0 计费，跳过并打出来。
			if entry.BillingMode == BillingModeToken && pricing.TokenPricingAbsent {
				slog.Warn("skipping catalog seed without token or per-image price", "model", name, "mode", pricing.Mode)
				continue
			}
			byKey[NormalizeModelCatalogKey(entry.ModelID)] = entry
		}
	}

	if input.BillingService != nil {
		for name, pricing := range input.BillingService.SnapshotFallbackPricing() {
			if pricing == nil {
				continue
			}
			key := NormalizeModelCatalogKey(name)
			// 价格文件已经覆盖这个标识（精确键或查表阶梯命中）时跳过：
			// 今天的解析顺序是「价格文件 → 硬编码兜底价」，播种不能把这个优先级翻过来。
			if _, exists := byKey[key]; exists {
				continue
			}
			if input.PricingService != nil && input.PricingService.GetModelPricing(name) != nil {
				continue
			}
			byKey[key] = seedEntryFromFallback(name, pricing)
		}
	}

	// xAI Imagine 官方媒体价：价格文件没有这些模型时才播（同硬编码兜底价的优先级）。
	for _, entry := range xaiImagineSeeds() {
		key := NormalizeModelCatalogKey(entry.ModelID)
		if _, exists := byKey[key]; exists {
			continue
		}
		if input.PricingService != nil && input.PricingService.GetModelPricing(entry.ModelID) != nil {
			continue
		}
		byKey[key] = entry
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
		// 默认 token；按张计价的生图模型下面改成 image。
		BillingMode: BillingModeToken,
		// 播种条目没有绑定资源，默认下架；管理员绑好资源再上架。
		Status:    ModelCatalogStatusUnlisted,
		ManagedBy: ModelCatalogManagedBySeed,

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
	if pricing.Mode == liteLLMModeImageGeneration && pricing.OutputCostPerImage > 0 {
		// 按张计价的生图模型：默认价 = 每张价。token 价照抄——同一模型经对话入口调用、
		// 没有图片输出时（ImageCount == 0）仍按 token 算。
		entry.BillingMode = BillingModeImage
		entry.PerRequestPrice = positivePrice(pricing.OutputCostPerImage)
	}
	// 模型内置搜索价只存 medium 档（拍的：OpenAI 按请求的 search_context_size 三档计，
	// 我们只存一档；对账发现偏差再决定是否三档都存）。
	entry.SearchPricePerCall = positivePrice(pricing.SearchContextCostPerQuery["search_context_size_medium"])
	return entry
}

// liteLLMModeImageGeneration 是价格文件里生图模型的 mode 值。
const liteLLMModeImageGeneration = "image_generation"

func seedEntryFromFallback(name string, pricing *ModelPricing) ModelCatalogEntry {
	entry := ModelCatalogEntry{
		ModelID:     name,
		BillingMode: BillingModeToken,
		Status:      ModelCatalogStatusUnlisted,
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
