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
	Pricing     *ChannelModelPricing
	TimePricing *ChannelTimePricing
	Aliases     []string
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
			TimePricing: entry.TimePricing,
			Aliases:     aliases,
		})
	}
	return models
}

func withDefaultMaxReasoningEffortMultiplier(pricing *ChannelModelPricing, model string) *ChannelModelPricing {
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
