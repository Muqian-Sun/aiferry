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
	// 联网搜索的官方单价（不乘用户倍率）：每次 web 搜索、每条 X 帖子、每个 X 主页（后两项只有 xAI 有）；
	// 没有官方搜索工具的厂商为 nil。
	WebSearchPricePerCall *float64
	XPostPrice            *float64
	XUserPrice            *float64
}

// ModelPlazaService 聚合模型广场数据：上架的目录条目及其基准价。
type ModelPlazaService struct {
	catalog CatalogListingSource
	// pricing 查不上架的条目（代执行搜索的模型）；生产上与 catalog 是同一个 *ModelCatalogService。
	pricing ModelCatalogPricingSource
}

// NewModelPlazaService 创建模型广场服务；catalog / pricing 生产上都是 *ModelCatalogService。
func NewModelPlazaService(catalog CatalogListingSource, pricing ModelCatalogPricingSource) *ModelPlazaService {
	return &ModelPlazaService{catalog: catalog, pricing: pricing}
}

// PlazaWebSearchBilling Claude Code 配第三方模型时那次搜索请求的计费项（官方价，USD / token、/ 次）：
// token 按它 × 用户倍率，每次搜索按官方原价收。广场给非 Anthropic 模型列这一行，不提代执行的模型。
type PlazaWebSearchBilling struct {
	InputPrice         *float64
	OutputPrice        *float64
	CacheReadPrice     *float64
	CacheWritePrice    *float64
	SearchPricePerCall float64
}

// ClaudeCodeWebSearchBilling 「联网搜索」计费项 = 代执行模型这条目录的官方价（muqian 2026-10-03）；目录里没有它时为 nil。
func (s *ModelPlazaService) ClaudeCodeWebSearchBilling(ctx context.Context) *PlazaWebSearchBilling {
	if s == nil || s.pricing == nil {
		return nil
	}
	entry := s.pricing.LookupPricingEntry(ctx, WebSearchDelegateModel)
	if entry == nil {
		return nil
	}
	return &PlazaWebSearchBilling{
		InputPrice:         entry.InputPrice,
		OutputPrice:        entry.OutputPrice,
		CacheReadPrice:     entry.CacheReadPrice,
		CacheWritePrice:    entry.CacheWritePrice,
		SearchPricePerCall: officialWebSearchPrices(entry).PerCall,
	}
}

// ListModels 返回全部上架条目（按模型标识排序，与 ListListedEntries 一致）。
// 价卡上补 fable 系列的默认最高推理档倍率，与「可用渠道」页口径一致。
func (s *ModelPlazaService) ListModels(ctx context.Context) []PlazaCatalogModel {
	entries := s.catalog.ListListedEntries(ctx)
	models := make([]PlazaCatalogModel, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		webSearch, xPost, xUser := plazaWebSearchPrices(entry)
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
				ImageCacheReadPrice:   entry.ImageCacheReadPrice,
				AudioInputPrice:       entry.AudioInputPrice,
				AudioOutputPrice:      entry.AudioOutputPrice,
				WebSearchPricePerCall: webSearch,
				XPostPrice:            xPost,
				XUserPrice:            xUser,
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
