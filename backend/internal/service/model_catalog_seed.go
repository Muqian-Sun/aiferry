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
// 播种写条目本身；条目带的分档（Intervals）也随之写入 / 刷新，分时没有默认数据来源。
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
			modelID := name
			if pricing.ModelID != "" {
				modelID = pricing.ModelID
			}
			entry := seedEntryFromLiteLLM(modelID, pricing)
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
		entry := byKey[key]
		if catalogModelExcluded(entry.ModelID) {
			continue
		}
		if !CatalogVendorAllowed(entry.Vendor) {
			// fail-loud：价格文件或兜底价表里混进了白名单外的厂商，打出来而不是静默播进目录
			slog.Warn("skipping catalog seed from a vendor outside the catalog allowlist", "model_id", entry.ModelID, "vendor", entry.Vendor)
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// catalogVendorAllowlist 模型目录只收这 11 家的模型（muqian 2026-10-06）：OpenAI、Anthropic、Google、xAI、DeepSeek、
// 智谱、月之暗面（Kimi）、MiniMax、阿里（通义）、字节（豆包）、小米（MiMo）。键是条目的厂商串（价格文件 provider 口径，
// Google 有 gemini 与 vertex_ai-* 两种写法、OpenAI 有 openai 与 text-completion-openai）。
// 只管播种（价格文件 / 兜底价表 → 目录）；管理员在后台手动建的条目不受限。
var catalogVendorAllowlist = map[string]bool{
	"openai":                     true,
	"text-completion-openai":     true,
	"anthropic":                  true,
	"gemini":                     true,
	"vertex_ai-language-models":  true,
	"vertex_ai-embedding-models": true,
	"xai":                        true,
	"deepseek":                   true,
	"zhipu":                      true,
	"moonshot":                   true,
	"minimax":                    true,
	"dashscope":                  true,
	"volcengine":                 true,
	"xiaomi":                     true,
}

// catalogExcludedModels 目录不收的模型 ID（2026-10-06 按各家官网核对，muqian 定：目录只留官网在售、没宣布停服、
// ID 固定的）。价格文件与兜底价表里还留着它们（计费、成品号映射、测试连接仍会用到），只是不播进目录。
// 依据：OpenAI developers.openai.com/api/docs/deprecations、Anthropic platform.claude.com/docs/en/about-claude/model-deprecations、
// Google ai.google.dev/gemini-api/docs/deprecations 与 changelog、xAI docs.x.ai/developers、Kimi platform.kimi.ai/docs/models、
// 火山方舟 volcengine.com/docs/82379/1330310、阿里云百炼 alibabacloud.com/help/en/model-studio/model-depreciation。
// 官网后续恢复或改名时从这里删掉即可。
var catalogExcludedModels = map[string]string{
	// 官网已宣布停服（现在还能调，muqian 定现在就从目录删）
	// 通义：阿里云百炼 2026-10-10 下线（公告 alibabacloud.com/notice/detail?id=1841 主线、1949 快照）
	"qvq-max":                                 "deprecated",
	"qwen-vl-max":                             "deprecated",
	"qwen-vl-plus":                            "deprecated",
	"qwq-plus":                                "deprecated",
	"qwen3-235b-a22b-instruct-2507":           "deprecated",
	"qwen3-235b-a22b-thinking-2507":           "deprecated",
	"qwen3-30b-a3b-instruct-2507":             "deprecated",
	"qwen3-30b-a3b-thinking-2507":             "deprecated",
	"qwen3-32b":                               "deprecated",
	"qwen3-next-80b-a3b-instruct":             "deprecated",
	"qwen3-next-80b-a3b-thinking":             "deprecated",
	"qwen3-vl-235b-a22b-instruct":             "deprecated",
	"qwen3-vl-235b-a22b-thinking":             "deprecated",
	"qwen3-vl-30b-a3b-instruct":               "deprecated",
	"qwen3-vl-30b-a3b-thinking":               "deprecated",
	"qwen3-vl-32b-instruct":                   "deprecated",
	"qwen3-vl-32b-thinking":                   "deprecated",
	"qwen3-vl-8b-instruct":                    "deprecated",
	"qwen3-vl-8b-thinking":                    "deprecated",
	"claude-sonnet-4-5":                       "deprecated",
	"claude-sonnet-4-5-20250929":              "deprecated",
	"gemini-3.1-flash-lite":                   "deprecated",
	"gemini-embedding-001":                    "deprecated",
	"gpt-3.5-turbo":                           "deprecated",
	"gpt-3.5-turbo-0125":                      "deprecated",
	"gpt-4":                                   "deprecated",
	"gpt-4-0613":                              "deprecated",
	"gpt-4-turbo":                             "deprecated",
	"gpt-4-turbo-2024-04-09":                  "deprecated",
	"gpt-4.1-nano":                            "deprecated",
	"gpt-4.1-nano-2025-04-14":                 "deprecated",
	"gpt-4o-2024-05-13":                       "deprecated",
	"gpt-4o-audio-preview-2024-12-17":         "deprecated",
	"gpt-4o-audio-preview-2025-06-03":         "deprecated",
	"gpt-4o-mini-audio-preview-2024-12-17":    "deprecated",
	"gpt-4o-mini-realtime-preview":            "deprecated",
	"gpt-4o-mini-realtime-preview-2024-12-17": "deprecated",
	"gpt-4o-mini-transcribe":                  "deprecated",
	"gpt-4o-mini-transcribe-2025-03-20":       "deprecated",
	"gpt-4o-mini-transcribe-2025-12-15":       "deprecated",
	"gpt-4o-mini-tts-2025-03-20":              "deprecated",
	"gpt-4o-mini-tts-2025-12-15":              "deprecated",
	"gpt-4o-realtime-preview":                 "deprecated",
	"gpt-4o-realtime-preview-2024-12-17":      "deprecated",
	"gpt-4o-realtime-preview-2025-06-03":      "deprecated",
	"gpt-4o-transcribe":                       "deprecated",
	"gpt-4o-transcribe-diarize":               "deprecated",
	"gpt-5":                                   "deprecated",
	"gpt-5-2025-08-07":                        "deprecated",
	"gpt-5-mini":                              "deprecated",
	"gpt-5-mini-2025-08-07":                   "deprecated",
	"gpt-5-nano":                              "deprecated",
	"gpt-5-nano-2025-08-07":                   "deprecated",
	"gpt-5-pro":                               "deprecated",
	"gpt-5-pro-2025-10-06":                    "deprecated",
	"gpt-5.1":                                 "deprecated",
	"gpt-5.3-codex":                           "deprecated",
	"gpt-5.4-nano":                            "deprecated",
	"gpt-audio":                               "deprecated",
	"gpt-audio-2025-08-28":                    "deprecated",
	"gpt-audio-mini":                          "deprecated",
	"gpt-audio-mini-2025-12-15":               "deprecated",
	"gpt-image-1":                             "deprecated",
	"gpt-image-1-mini":                        "deprecated",
	"gpt-image-1.5":                           "deprecated",
	"gpt-image-1.5-2025-12-16":                "deprecated",
	"gpt-realtime":                            "deprecated",
	"gpt-realtime-2025-08-28":                 "deprecated",
	"gpt-realtime-mini":                       "deprecated",
	"gpt-realtime-mini-2025-12-15":            "deprecated",
	"grok-imagine-image-quality":              "deprecated",
	"o1-2024-12-17":                           "deprecated",
	"o1-pro":                                  "deprecated",
	"o1-pro-2025-03-19":                       "deprecated",
	"o3":                                      "deprecated",
	"o3-2025-04-16":                           "deprecated",
	"o3-mini":                                 "deprecated",
	"o3-mini-2025-01-31":                      "deprecated",
	"o3-pro":                                  "deprecated",
	"o3-pro-2025-06-10":                       "deprecated",
	"o4-mini":                                 "deprecated",
	"o4-mini-2025-04-16":                      "deprecated",
	// 官网已停服
	"gemini-2.0-flash":                        "shutdown",
	"gemini-2.0-flash-001":                    "shutdown",
	"gemini-2.0-flash-exp-image-generation":   "shutdown",
	"gemini-2.0-flash-lite":                   "shutdown",
	"gemini-2.0-flash-lite-001":               "shutdown",
	"gemini-2.5-computer-use-preview-10-2025": "shutdown",
	"gemini-2.5-flash-image":                  "shutdown",
	"gemini-2.5-flash-lite-preview-06-17":     "shutdown",
	"gemini-2.5-flash-lite-preview-09-2025":   "shutdown",
	"gemini-2.5-flash-preview-09-2025":        "shutdown",
	"gemini-3-pro-image-preview":              "shutdown",
	"gemini-3-pro-preview":                    "shutdown",
	"gemini-3.1-flash-image-preview":          "shutdown",
	"gemini-3.1-flash-lite-preview":           "shutdown",
	"gemini-embedding-2-preview":              "shutdown",
	"gemini-robotics-er-1.5-preview":          "shutdown",
	"gpt-3.5-turbo-1106":                      "shutdown",
	"gpt-3.5-turbo-instruct":                  "shutdown",
	"gpt-4-0125-preview":                      "shutdown",
	"gpt-4-0314":                              "shutdown",
	"gpt-4-1106-preview":                      "shutdown",
	"gpt-4-turbo-preview":                     "shutdown",
	"gpt-4o-audio-preview":                    "shutdown",
	"gpt-4o-mini-audio-preview":               "shutdown",
	"gpt-4o-mini-search-preview":              "shutdown",
	"gpt-4o-mini-search-preview-2025-03-11":   "shutdown",
	"gpt-4o-search-preview":                   "shutdown",
	"gpt-4o-search-preview-2025-03-11":        "shutdown",
	"gpt-5-codex":                             "shutdown",
	"gpt-5.1-codex":                           "shutdown",
	"gpt-5.1-codex-max":                       "shutdown",
	"gpt-5.1-codex-mini":                      "shutdown",
	"gpt-5.2-codex":                           "shutdown",
	"gpt-audio-mini-2025-10-06":               "shutdown",
	"gpt-realtime-mini-2025-10-06":            "shutdown",
	"o3-deep-research":                        "shutdown",
	"o3-deep-research-2025-06-26":             "shutdown",
	"o4-mini-deep-research":                   "shutdown",
	"o4-mini-deep-research-2025-06-26":        "shutdown",
	// 官网已退役
	"claude-3-5-haiku":           "retired",
	"claude-3-7-sonnet-20250219": "retired",
	"claude-3-haiku-20240307":    "retired",
	"claude-3-opus-20240229":     "retired",
	"claude-opus-4-1":            "retired",
	"claude-opus-4-1-20250805":   "retired",
	"claude-opus-4-20250514":     "retired",
	"claude-sonnet-4-20250514":   "retired",
	"kimi-k2":                    "retired",
	"kimi-k2-thinking":           "retired",
	"kimi-k2.5":                  "retired",
	// 官网不再列出（模型页 / 定价页 / 停服表都没有）
	"gemini-exp-1206": "not_listed",
	"gemini-live-2.5-flash-preview-native-audio-09-2025": "not_listed",
	"grok-3-mini":      "not_listed",
	"grok-3-mini-fast": "not_listed",
	// 官网只在更新日志里出现过、没有价格
	"gemini-2.5-flash-native-audio-preview-09-2025": "not_priced",
	// 不是官方 API 模型 ID（中转 / 第三方工具起的名字，或厂商名写反）
	"claude-4-opus-20250514":      "not_official",
	"claude-4-sonnet-20250514":    "not_official",
	"claude-opus-4-6-20260205":    "not_official",
	"claude-opus-4-6-thinking":    "not_official",
	"claude-opus-4-7-20260416":    "not_official",
	"codex-auto-review":           "not_official",
	"doubao-embedding-vision":     "not_official",
	"gemini-3-flash":              "not_official",
	"gemini-3.1-pro":              "not_official",
	"gemini-3.1-pro-high":         "not_official",
	"gemini-3.1-pro-low":          "not_official",
	"gemini-flash-experimental":   "not_official",
	"gpt-3.5-turbo-16k":           "not_official",
	"gpt-3.5-turbo-instruct-0914": "not_official",
	"gpt-5-chat":                  "not_official",
	"gpt-5-search-api-2025-10-14": "not_official",
	// 会自动换指向的别名（-latest、kimi-for-coding），目录只留固定 ID
	"chat-latest":                          "moving_alias",
	"gemini-2.5-flash-native-audio-latest": "moving_alias",
	"gemini-flash-latest":                  "moving_alias",
	"gemini-flash-lite-latest":             "moving_alias",
	"gemini-pro-latest":                    "moving_alias",
	"gpt-5-chat-latest":                    "moving_alias",
	"gpt-5.1-chat-latest":                  "moving_alias",
	"gpt-5.2-chat-latest":                  "moving_alias",
	"gpt-5.3-chat-latest":                  "moving_alias",
	"grok-4.20-non-reasoning-latest":       "moving_alias",
	"grok-4.20-reasoning-latest":           "moving_alias",
	"grok-4.3-latest":                      "moving_alias",
	"grok-4.5-latest":                      "moving_alias",
	"grok-build-latest":                    "moving_alias",
	"kimi-for-coding":                      "moving_alias",
}

// catalogModelExcluded 这个模型 ID 是否被目录排除（不分大小写）。
func catalogModelExcluded(modelID string) bool {
	_, ok := catalogExcludedModels[strings.ToLower(strings.TrimSpace(modelID))]
	return ok
}

// CatalogVendorAllowed 厂商串在不在目录白名单里（不分大小写）。
func CatalogVendorAllowed(vendor string) bool {
	return catalogVendorAllowlist[strings.ToLower(strings.TrimSpace(vendor))]
}

// LookupPriceFileEntry 按模型标识构造一条建议条目，给「添加模型」自动带出厂商 / 计费方式 / 价格。
// 与播种同一来源、同一优先级：价格文件（确定性识别，不按子串猜）→ 硬编码兜底价 → xAI Imagine 官方媒体价。
// 带出的条目模型标识用管理员输入的写法、状态为下架；找不到时 ok=false。
func (s *ModelCatalogService) LookupPriceFileEntry(modelID string) (entry ModelCatalogEntry, ok bool) {
	modelID = strings.TrimSpace(modelID)
	if s == nil || modelID == "" {
		return ModelCatalogEntry{}, false
	}
	if s.seedInput.PricingService != nil {
		if pricing := s.seedInput.PricingService.GetIdentifiedModelPricing(modelID); pricing != nil {
			entry := seedEntryFromLiteLLM(modelID, pricing)
			// 与播种同一口径：按 token 计费却没有 token 价的不带出（会按 $0 计费）
			if !(entry.BillingMode == BillingModeToken && pricing.TokenPricingAbsent) {
				return entry, true
			}
		}
	}
	key := NormalizeModelCatalogKey(modelID)
	if s.seedInput.BillingService != nil {
		for name, pricing := range s.seedInput.BillingService.SnapshotFallbackPricing() {
			if pricing != nil && NormalizeModelCatalogKey(name) == key {
				entry := seedEntryFromFallback(name, pricing)
				entry.ModelID = modelID
				return entry, true
			}
		}
	}
	for _, seed := range xaiImagineSeeds() {
		if NormalizeModelCatalogKey(seed.ModelID) == key {
			seed.ModelID = modelID
			return seed, true
		}
	}
	return ModelCatalogEntry{}, false
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
		AudioInputPrice:     positivePrice(pricing.InputCostPerAudioToken),
		AudioOutputPrice:    positivePrice(pricing.OutputCostPerAudioToken),
	}
	// 5m/1h 分档只在 1h 价严格高于 5m 价时成立，与 getModelPricingAt 同口径：
	// 价格文件写反时不分档，避免把 1h 缓存按更低的价算。
	if pricing.CacheCreationInputTokenCostAbove1hr > 0 &&
		pricing.CacheCreationInputTokenCostAbove1hr > pricing.CacheCreationInputTokenCost {
		entry.CacheWrite1hPrice = positivePrice(pricing.CacheCreationInputTokenCostAbove1hr)
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
	// 官网逐段写明的多段价直接换算成按 token 分段；没有多段价时，价格文件的长上下文阶梯
	// 换算成按 token 分段（xAI 的阈值是「达到即进高段」，其余提供商严格大于）。
	if len(pricing.InputTokenTiers) > 0 {
		if entry.BillingMode == BillingModeToken {
			entry.Intervals = inputTokenTierIntervals(pricing.InputTokenTiers)
		}
		return entry
	}
	tokenLadder{
		threshold:        pricing.LongContextInputTokenThreshold,
		inclusive:        strings.EqualFold(pricing.LiteLLMProvider, "xai"),
		inputMultiplier:  pricing.LongContextInputCostMultiplier,
		outputMultiplier: pricing.LongContextOutputCostMultiplier,
	}.applyTo(&entry)
	return entry
}

// inputTokenTierIntervals 把多段价换算成目录分段：第一段就是基础价，不另建分段；其余每段是
// (上一段上限, 本段上限]，最后一段不封顶（超出官方上下文长度的请求上游本来就会拒）。各段的价都是
// 官网逐段写明的绝对价；某段没写缓存价时，计费按基础缓存价 × 本段输入价 / 基础输入价推算。
func inputTokenTierIntervals(tiers []LiteLLMInputTokenTier) []PricingInterval {
	intervals := make([]PricingInterval, 0, len(tiers)-1)
	for i := 1; i < len(tiers); i++ {
		tier := tiers[i]
		interval := PricingInterval{
			MinTokens:       tiers[i-1].MaxInputTokens,
			InputPrice:      positivePrice(tier.InputCostPerToken),
			OutputPrice:     positivePrice(tier.OutputCostPerToken),
			CacheReadPrice:  positivePrice(tier.CacheReadInputTokenCost),
			CacheWritePrice: positivePrice(tier.CacheCreationInputTokenCost),
			SortOrder:       i - 1,
		}
		if i < len(tiers)-1 {
			upper := tier.MaxInputTokens
			interval.MaxTokens = &upper
		}
		intervals = append(intervals, interval)
	}
	return intervals
}

// liteLLMModeImageGeneration 是价格文件里生图模型的 mode 值。
const liteLLMModeImageGeneration = "image_generation"

// modelFamilyVendors 按模型族给新建条目标厂商：兜底价表（BillingService.fallbackPrices，我们自己维护的已知模型）
// 播种时、以及从上游导入价格文件查不到厂商的模型时用。价格文件里的条目厂商来自 litellm_provider；这两处没有这一项，
// 建出来厂商为空，用户站模型页就归不到任何厂商、也没有图标（2026-09-29 E2E：播种的 grok-4.5 / 4.6、glm、claude-fable、
// gemini-3.x 等 42 条；导入的 deepseek-v4.1-flash、glm-5.3-flash）。只在建条目时写进厂商字段、管理员可改，
// 不是运行时按模型名猜厂商（CatalogVendorPlatform 仍只看条目厂商）。值与价格文件同口径（provider 串）。
var modelFamilyVendors = []struct{ prefix, vendor string }{
	{"claude-", "anthropic"},
	{"gpt-", "openai"},
	{"codex-", "openai"},
	{"gemini-", "gemini"},
	{"grok-", "xai"},
	{"glm-", "zhipu"},
	{"deepseek-", "deepseek"},
	{"kimi-", "moonshot"},
	{"minimax-", "minimax"},
	{"doubao-", "volcengine"},
}

// modelFamilyVendor 上游模型名可能带组织前缀（deepseek-ai/deepseek-v4），按最后一段认模型族。
func modelFamilyVendor(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if i := strings.LastIndex(lower, "/"); i >= 0 {
		lower = lower[i+1:]
	}
	for _, v := range modelFamilyVendors {
		if strings.HasPrefix(lower, v.prefix) {
			return v.vendor
		}
	}
	return ""
}

func seedEntryFromFallback(name string, pricing *ModelPricing) ModelCatalogEntry {
	entry := ModelCatalogEntry{
		ModelID:     name,
		Vendor:      modelFamilyVendor(name),
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
		AudioInputPrice:     positivePrice(pricing.AudioInputPricePerToken),
		AudioOutputPrice:    positivePrice(pricing.AudioOutputPricePerToken),

		MaxReasoningEffortMultiplier: clonePricePtr(pricing.MaxReasoningEffortMultiplier),
	}
	if pricing.SupportsCacheBreakdown &&
		pricing.CacheCreation1hPrice > 0 &&
		pricing.CacheCreation1hPrice > pricing.CacheCreation5mPrice {
		entry.CacheWrite1hPrice = positivePrice(pricing.CacheCreation1hPrice)
	}
	fallbackSeedLadders[name].applyTo(&entry)
	return entry
}

// fallbackSeedLadders 兜底价表（BillingService.fallbackPrices）里有官方长上下文加价的模型，播种时换算成按 token 分段。
// 值照搬原来写在兜底表 LongContext* 字段里的阶梯（2026-09-29 引擎改为只认分段时搬到这里）。
var fallbackSeedLadders = map[string]tokenLadder{
	"gpt-6-astra":    {threshold: 272_000, inputMultiplier: 2, outputMultiplier: 1.5},
	"grok-4.5":       {threshold: 200_000, inclusive: true, inputMultiplier: 2, outputMultiplier: 2},
	"grok-4.6":       {threshold: 200_000, inclusive: true, inputMultiplier: 2, outputMultiplier: 2},
	"grok-4.3":       {threshold: 200_000, inclusive: true, inputMultiplier: 2, outputMultiplier: 2},
	"grok-4.20":      {threshold: 200_000, inclusive: true, inputMultiplier: 2, outputMultiplier: 2},
	"grok-build-0.1": {threshold: 200_000, inclusive: true, inputMultiplier: 2, outputMultiplier: 2},
}

func clonePricePtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}
