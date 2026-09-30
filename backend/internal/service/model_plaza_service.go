package service

import (
	"context"
)

// PlazaCatalogModel 模型广场里的一个模型：目录上架条目的展示投影。
type PlazaCatalogModel struct {
	ModelID     string
	DisplayName string
	Vendor      string
	BillingMode BillingMode
	// Pricing 目录基准价卡（分档在 Intervals 里）；分时倍率单独给出。
	Pricing *PricingCard
	// TokenExtras 价卡（PricingCard）不带、但 token 计费会收的项。
	TokenExtras PlazaTokenExtras
	TimePricing *TimePricing
	Aliases     []string
}

// PlazaTokenExtras 目录条目上价卡没投影的 token 计费项（官方价，USD / token）：
// 图片缓存读、音频输入 / 输出。模型广场要把计费项列全（muqian 2026-09-30）。
type PlazaTokenExtras struct {
	ImageCacheReadPrice *float64
	AudioInputPrice     *float64
	AudioOutputPrice    *float64
	// WebSearchPricePerCall 联网搜索（/alpha/search，只走 OpenAI / Codex 账号）每次的实际计费价：
	// 条目配了用条目的，没配按内置单价；非 OpenAI 模型走不到这个入口，为 nil。
	WebSearchPricePerCall *float64
	// ToolSearchPricePerCall grok 渠道的搜索工具调用（web_search / x_search）每次的内置价；非 grok 模型为 nil。
	ToolSearchPricePerCall *float64
}

// plazaSearchPrices 这个模型实际会收的搜索费（官方价）：只给走得到的那一种，走不到的不给，免得广场列出永远不收的价。
func plazaSearchPrices(entry *ModelCatalogEntry) (webPerCall, toolPerCall *float64) {
	switch CatalogVendorPlatform(entry) {
	case PlatformOpenAI:
		price := defaultWebSearchPricePerCall
		if entry.SearchPricePerCall != nil && *entry.SearchPricePerCall >= 0 {
			price = *entry.SearchPricePerCall
		}
		return &price, nil
	case PlatformGrok:
		price := defaultSearchPricePer1k / 1000
		return nil, &price
	}
	return nil, nil
}

// ModelPlazaService 聚合模型广场数据：上架的目录条目及其基准价。
type ModelPlazaService struct {
	catalog CatalogListingSource
}

// NewModelPlazaService 创建模型广场服务；catalog 生产上是 *ModelCatalogService。
func NewModelPlazaService(catalog CatalogListingSource) *ModelPlazaService {
	return &ModelPlazaService{catalog: catalog}
}

// ListModels 返回全部上架条目（按模型标识排序，与 ListListedEntries 一致）。
// 价卡上补 fable 系列的默认最高推理档倍率，与「可用渠道」页口径一致。
func (s *ModelPlazaService) ListModels(ctx context.Context) []PlazaCatalogModel {
	entries := s.catalog.ListListedEntries(ctx)
	models := make([]PlazaCatalogModel, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		webSearch, toolSearch := plazaSearchPrices(entry)
		aliases := make([]string, 0, len(entry.Aliases))
		for _, alias := range entry.Aliases {
			aliases = append(aliases, alias.Alias)
		}
		models = append(models, PlazaCatalogModel{
			ModelID:     entry.ModelID,
			DisplayName: entry.DisplayName,
			Vendor:      entry.Vendor,
			BillingMode: entry.EffectiveBillingMode(),
			Pricing:     withDefaultMaxReasoningEffortMultiplier(entry.PricingCard(), entry.ModelID),
			TokenExtras: PlazaTokenExtras{
				ImageCacheReadPrice:    entry.ImageCacheReadPrice,
				AudioInputPrice:        entry.AudioInputPrice,
				AudioOutputPrice:       entry.AudioOutputPrice,
				WebSearchPricePerCall:  webSearch,
				ToolSearchPricePerCall: toolSearch,
			},
			TimePricing: entry.TimePricing,
			Aliases:     aliases,
		})
	}
	return models
}

func withDefaultMaxReasoningEffortMultiplier(pricing *PricingCard, model string) *PricingCard {
	if pricing == nil || pricing.MaxReasoningEffortMultiplier != nil {
		return pricing
	}
	multiplier := defaultMaxReasoningEffortMultiplier(model)
	if multiplier == nil {
		return pricing
	}
	cloned := pricing.Clone()
	cloned.MaxReasoningEffortMultiplier = multiplier
	return &cloned
}
