package service

import (
	"context"
	"log/slog"
	"strings"
)

// PricingSource 定价来源标识
const (
	PricingSourceGroup = "group"
	// PricingSourceCatalog 表示价格来自 model_catalog_entries 里的条目。
	PricingSourceCatalog  = "catalog"
	PricingSourceLiteLLM  = "litellm"
	PricingSourceFallback = "fallback"
)

// ResolvedPricing 统一定价解析结果
type ResolvedPricing struct {
	// Mode 计费模式
	Mode BillingMode

	// Token 模式：基础定价（来自模型目录 / LiteLLM / fallback）
	BasePricing *ModelPricing

	// Token 模式：区间定价列表（如有，覆盖 BasePricing 中的对应字段）
	Intervals []PricingInterval

	// 按次/图片模式：分层定价
	RequestTiers []PricingInterval

	// 按次/图片模式：默认价格（未命中层级时使用）
	DefaultPerRequestPrice float64

	// 来源标识
	Source string // "group", "catalog", "litellm", "fallback"

	// CanonicalModel 是厂商价格政策（DeepSeek 官方价、GPT-5.6 缓存写价、Fast 档比例、
	// max 推理倍率）所依据的模型标识：目录命中时是条目的 model_id（请求可能用的是
	// 别名），否则就是请求的计费模型。
	CanonicalModel string

	// 是否支持缓存细分
	SupportsCacheBreakdown bool

	// configuredPricing 是命中的价卡原始配置（分组价卡或目录条目投影），
	// 用于区间模式取图片价、判定哪些字段被显式配置、以及分时倍率。
	configuredPricing *ChannelModelPricing

	// operatorPricing 表示胜出的价格是运营者写的（分组价卡，或被管理员改过的
	// 目录条目），而不是平台默认价卡（播种出来的目录条目 / 价格文件 / 硬编码兜底价）。
	//
	// 这条区分承接了原先「Source == channel」与「Source == litellm」两个判断：
	// 运营者定价保留运营者语义（不强制官方价、不叠加官方峰谷），平台默认价卡
	// 则继续套用官方价政策。
	operatorPricing bool

	longContextPricingEnabled bool
}

// ModelPricingResolver 统一模型定价解析器。
// 解析链：Group 价卡 → 模型目录 → LiteLLM 价格文件 → 硬编码兜底价。
type ModelPricingResolver struct {
	catalog        ModelCatalogPricingSource
	billingService *BillingService
}

// NewModelPricingResolver 创建定价解析器实例
func NewModelPricingResolver(catalog ModelCatalogPricingSource, billingService *BillingService) *ModelPricingResolver {
	return &ModelPricingResolver{
		catalog:        catalog,
		billingService: billingService,
	}
}

// PricingInput 定价解析输入
type PricingInput struct {
	Model string
	Group *Group
}

// Resolve 解析模型定价。
//  1. 分组价卡（groups.model_pricing）命中即返回，是运营者的分组级覆盖层，
//     未显式配置的项叠在目录基准价上（目录未命中时叠在价格文件上）；
//  2. 模型目录命中则作为基准价卡（叠加在价格文件之上，只写显式配置的项）；
//  3. 都没有则回到价格文件 / 硬编码兜底价。
func (r *ModelPricingResolver) Resolve(ctx context.Context, input PricingInput) *ResolvedPricing {
	longContextPricingEnabled := input.Group == nil || input.Group.LongContextPricingEnabled
	entry := r.lookupCatalogEntry(ctx, input.Model)
	if groupPricing := matchGroupModelPricing(input.Group, input.Model); groupPricing != nil {
		// Group token cards only override the first-tier / flat rates.
		// Long-context ladders come from official presets, gated by the checkbox.
		if groupPricing.BillingMode == "" || groupPricing.BillingMode == BillingModeToken {
			stripped := groupPricing.Clone()
			stripped.Intervals = nil
			groupPricing = &stripped
		}
		resolved := r.resolveConfiguredPricing(groupPricing, entry, input.Model, PricingSourceGroup)
		resolved.operatorPricing = true
		resolved.longContextPricingEnabled = longContextPricingEnabled
		return resolved
	}

	if entry != nil {
		resolved := r.resolveCatalogPricing(entry)
		resolved.longContextPricingEnabled = longContextPricingEnabled
		return resolved
	}

	basePricing, source := r.resolveBasePricing(input.Model)
	resolved := &ResolvedPricing{
		Mode:                   BillingModeToken,
		BasePricing:            basePricing,
		Source:                 source,
		CanonicalModel:         input.Model,
		SupportsCacheBreakdown: basePricing != nil && basePricing.SupportsCacheBreakdown,
	}
	resolved.longContextPricingEnabled = longContextPricingEnabled
	return resolved
}

// hasUsablePricing 判断解析结果能否用来计费：按次/图片/视频模式由价卡自带层级价，
// token 模式则必须解析出基准价。
func (p *ResolvedPricing) hasUsablePricing() bool {
	if p == nil {
		return false
	}
	return p.Mode != BillingModeToken || p.BasePricing != nil
}

func (r *ModelPricingResolver) lookupCatalogEntry(ctx context.Context, model string) *ModelCatalogEntry {
	if r == nil || r.catalog == nil {
		return nil
	}
	return r.catalog.LookupPricingEntry(ctx, model)
}

// resolveCatalogPricing 用目录条目构造解析结果。
//
// 与分组/渠道价卡的关键差别：目录是**基准价**，只写条目里显式配置的项，不做
// 「未配置即归零」。否则没配图片输出价的条目会把图片 token 静默算成免费，
// 而今天它会回退到文本输出价。
func (r *ModelPricingResolver) resolveCatalogPricing(entry *ModelCatalogEntry) *ResolvedPricing {
	card := entry.PricingCard()
	resolved := &ResolvedPricing{
		Mode:              entry.EffectiveBillingMode(),
		Source:            PricingSourceCatalog,
		CanonicalModel:    entry.ModelID,
		configuredPricing: card,
		operatorPricing:   entry.IsOperatorAuthored(),
	}

	switch resolved.Mode {
	case BillingModePerRequest, BillingModeImage, BillingModeVideo:
		r.applyRequestTierOverrides(card, resolved)
		return resolved
	}

	merged := r.catalogTokenBasePricing(entry)
	if merged == nil {
		// 目录条目没有任何 token 价，底下的价格文件也定不到价：保持「无价」，
		// 让计费链路走既有的 ErrModelPricingUnavailable 路径，而不是悄悄变成全 0 价卡。
		return resolved
	}
	resolved.BasePricing = merged
	resolved.SupportsCacheBreakdown = merged.SupportsCacheBreakdown
	resolved.Intervals = filterValidIntervals(card.Intervals)
	if len(resolved.Intervals) > 0 {
		for i := range resolved.Intervals {
			if resolved.Intervals[i].CacheWrite1hPrice != nil {
				resolved.SupportsCacheBreakdown = true
				resolved.BasePricing.SupportsCacheBreakdown = true
				break
			}
		}
	}
	return resolved
}

// catalogTokenBasePricing 把目录条目叠在价格文件的基准价上，得到 token 模式的基准价卡。
// 基准价按条目的 model_id 查（请求名可能是别名，价格文件按别名查不到）。
// 条目没有任何 token 价、价格文件也定不到价时返回 nil。
func (r *ModelPricingResolver) catalogTokenBasePricing(entry *ModelCatalogEntry) *ModelPricing {
	base, _ := r.resolveBasePricing(entry.ModelID)
	if base == nil && !entry.HasAnyTokenPrice() {
		return nil
	}
	merged := &ModelPricing{}
	if base != nil {
		*merged = *base
	}
	entry.ApplyToModelPricing(merged)
	return merged
}

// resolveConfiguredPricing 用运营者价卡构造解析结果。token 模式的基准价取目录条目
// （命中时）或价格文件，价卡里显式配置的项再覆盖上去。
func (r *ModelPricingResolver) resolveConfiguredPricing(config *ChannelModelPricing, entry *ModelCatalogEntry, model, source string) *ResolvedPricing {
	mode := config.BillingMode
	if mode == "" {
		mode = BillingModeToken
	}
	resolved := &ResolvedPricing{Mode: mode, Source: source, CanonicalModel: model, configuredPricing: config}
	if entry != nil {
		resolved.CanonicalModel = entry.ModelID
	}
	if mode == BillingModePerRequest || mode == BillingModeImage || mode == BillingModeVideo {
		r.applyRequestTierOverrides(config, resolved)
		return resolved
	}
	if entry != nil {
		resolved.BasePricing = r.catalogTokenBasePricing(entry)
	} else {
		resolved.BasePricing, _ = r.resolveBasePricing(model)
	}
	resolved.SupportsCacheBreakdown = resolved.BasePricing != nil && resolved.BasePricing.SupportsCacheBreakdown
	r.applyTokenOverrides(config, resolved)
	return resolved
}

func matchGroupModelPricing(group *Group, model string) *ChannelModelPricing {
	if group == nil {
		return nil
	}
	model = normalizeChannelPricingModelName(model)
	var wildcard *ChannelModelPricing
	for i := range group.ModelPricing {
		entry := &group.ModelPricing[i]
		for _, pattern := range entry.Models {
			normalized := normalizeChannelPricingModelName(pattern)
			if normalized == model {
				cp := entry.Clone()
				return &cp
			}
			if strings.HasSuffix(normalized, "*") && strings.HasPrefix(model, strings.TrimSuffix(normalized, "*")) && wildcard == nil {
				cp := entry.Clone()
				wildcard = &cp
			}
		}
	}
	return wildcard
}

// resolveBasePricing 从 LiteLLM 或 Fallback 获取基础定价
func (r *ModelPricingResolver) resolveBasePricing(model string) (*ModelPricing, string) {
	pricing, err := r.billingService.GetModelPricing(model)
	if err != nil {
		slog.Debug("failed to get model pricing from LiteLLM, using fallback",
			"model", model, "error", err)
		return nil, PricingSourceFallback
	}
	return pricing, PricingSourceLiteLLM
}

// applyTokenOverrides 应用 token 模式的分组价卡覆盖。
// 分组价卡是运营者定价，语义是「覆盖一切」：显式配置则用配置值，未配置则归零。
func (r *ModelPricingResolver) applyTokenOverrides(chPricing *ChannelModelPricing, resolved *ResolvedPricing) {
	if resolved.BasePricing == nil {
		resolved.BasePricing = &ModelPricing{}
	} else {
		// 防止修改 fallbackPrices 中的共享指针
		cloned := *resolved.BasePricing
		resolved.BasePricing = &cloned
	}

	applyChannelTokenPriceOverrides(resolved.BasePricing, chPricing)
	if chPricing.CacheWrite1hPrice != nil {
		resolved.SupportsCacheBreakdown = true
		resolved.BasePricing.SupportsCacheBreakdown = true
	}
	for i := range chPricing.Intervals {
		if chPricing.Intervals[i].CacheWrite1hPrice != nil {
			resolved.SupportsCacheBreakdown = true
			resolved.BasePricing.SupportsCacheBreakdown = true
			break
		}
	}
	resolved.BasePricing.FastMultiplier = chPricing.FastMultiplier
	resolved.BasePricing.FlexMultiplier = chPricing.FlexMultiplier
	if chPricing.MaxReasoningEffortMultiplier != nil {
		resolved.BasePricing.MaxReasoningEffortMultiplier = chPricing.MaxReasoningEffortMultiplier
	}
	// 运营者价卡覆盖一切：显式配置则用配置值，未配置则归零（不回退到 LiteLLM）
	if chPricing.ImageOutputPrice != nil {
		resolved.BasePricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
	} else {
		resolved.BasePricing.ImageOutputPricePerToken = 0
	}
	resolved.BasePricing.ImageOutputPriceExplicit = true
	applyConfiguredImageInputPrice(chPricing, resolved.BasePricing)

	// 区间未命中时回退到上面已经应用覆盖的基础价。
	resolved.Intervals = filterValidIntervals(chPricing.Intervals)
}

// applyConfiguredImageInputPrice 应用运营者价卡的图片输入价：显式配置则用配置值；
// 未配置时归零，使 computeTokenBreakdown 回退到文本输入价（向后兼容，
// 避免 LiteLLM 图片输入价泄漏进自定义定价）。
// 与 image_output 不同，此处不设 Explicit 标志——图片输入未配置应回退文本价，
// 而非硬置 0。
func applyConfiguredImageInputPrice(chPricing *ChannelModelPricing, pricing *ModelPricing) {
	if chPricing != nil && chPricing.ImageInputPrice != nil {
		pricing.ImageInputPricePerToken = *chPricing.ImageInputPrice
	} else {
		pricing.ImageInputPricePerToken = 0
	}
}

// applyRequestTierOverrides 应用按次/图片模式的价卡覆盖
func (r *ModelPricingResolver) applyRequestTierOverrides(chPricing *ChannelModelPricing, resolved *ResolvedPricing) {
	resolved.RequestTiers = filterValidIntervals(chPricing.Intervals)
	if chPricing.PerRequestPrice != nil {
		resolved.DefaultPerRequestPrice = *chPricing.PerRequestPrice
	}
}

// filterValidIntervals 过滤掉所有价格字段都为空的无效 interval。
// 前端可能创建了只有 min/max 但无价格的空 interval。
func filterValidIntervals(intervals []PricingInterval) []PricingInterval {
	var valid []PricingInterval
	for _, iv := range intervals {
		if iv.InputPrice != nil || iv.OutputPrice != nil ||
			iv.CacheWritePrice != nil || iv.CacheWrite1hPrice != nil || iv.CacheReadPrice != nil ||
			iv.PerRequestPrice != nil || iv.InputMultiplier != nil ||
			iv.OutputMultiplier != nil || iv.CacheWriteMultiplier != nil ||
			iv.CacheReadMultiplier != nil {
			valid = append(valid, iv)
		}
	}
	return valid
}

// GetIntervalPricing 根据 context token 数获取区间定价。
// 如果有区间列表，找到匹配区间并构造 ModelPricing；否则直接返回 BasePricing。
func (r *ModelPricingResolver) GetIntervalPricing(resolved *ResolvedPricing, totalContextTokens int) *ModelPricing {
	if len(resolved.Intervals) == 0 {
		return resolved.BasePricing
	}

	iv := FindMatchingInterval(resolved.Intervals, totalContextTokens)
	if iv == nil {
		return resolved.BasePricing
	}

	// 目录条目是基准价，不做「未配置即归零」；分组价卡是运营者定价，保持覆盖一切。
	pricing := intervalToModelPricing(iv, resolved.BasePricing, resolved.configuredPricing,
		resolved.Source != PricingSourceCatalog)
	// BasePricing 为 nil（仅配置区间）时拷贝不到该标志，从 resolved 回填，
	// 保证 computeCacheCreationCost 的 5m/1h 分档判断不被区间路径吞掉。
	pricing.SupportsCacheBreakdown = resolved.SupportsCacheBreakdown
	return pricing
}

// intervalToModelPricing 将区间定价转换为 ModelPricing。
// overrideImagePrices 为 true 时按运营者价卡语义处理图片价（未配置即归零）。
func intervalToModelPricing(iv *PricingInterval, base *ModelPricing, chPricing *ChannelModelPricing, overrideImagePrices bool) *ModelPricing {
	pricing := &ModelPricing{}
	if base != nil {
		*pricing = *base
	}
	applyMultiplier := func(value float64, multiplier *float64) float64 {
		if multiplier == nil {
			return value
		}
		return value * *multiplier
	}
	if iv.InputPrice != nil {
		pricing.InputPricePerTokenPriority = channelTierOverridePrice(pricing.InputPricePerToken, pricing.InputPricePerTokenPriority, *iv.InputPrice)
		pricing.InputPricePerToken = *iv.InputPrice
	} else if iv.InputMultiplier != nil {
		pricing.InputPricePerToken = applyMultiplier(pricing.InputPricePerToken, iv.InputMultiplier)
		pricing.InputPricePerTokenPriority = applyMultiplier(pricing.InputPricePerTokenPriority, iv.InputMultiplier)
	}
	if iv.OutputPrice != nil {
		pricing.OutputPricePerTokenPriority = channelTierOverridePrice(pricing.OutputPricePerToken, pricing.OutputPricePerTokenPriority, *iv.OutputPrice)
		pricing.OutputPricePerToken = *iv.OutputPrice
	} else if iv.OutputMultiplier != nil {
		pricing.OutputPricePerToken = applyMultiplier(pricing.OutputPricePerToken, iv.OutputMultiplier)
		pricing.OutputPricePerTokenPriority = applyMultiplier(pricing.OutputPricePerTokenPriority, iv.OutputMultiplier)
	}
	if iv.CacheWritePrice != nil {
		pricing.CacheCreationPricePerTokenPriority = channelTierOverridePrice(pricing.CacheCreationPricePerToken, pricing.CacheCreationPricePerTokenPriority, *iv.CacheWritePrice)
		pricing.CacheCreationPricePerToken = *iv.CacheWritePrice
		pricing.CacheCreationPriceExplicit = true
		pricing.CacheCreation5mPrice = *iv.CacheWritePrice
		if iv.CacheWrite1hPrice == nil {
			pricing.CacheCreation1hPrice = *iv.CacheWritePrice
		}
	} else if iv.CacheWriteMultiplier != nil {
		pricing.CacheCreationPricePerToken = applyMultiplier(pricing.CacheCreationPricePerToken, iv.CacheWriteMultiplier)
		pricing.CacheCreationPricePerTokenPriority = applyMultiplier(pricing.CacheCreationPricePerTokenPriority, iv.CacheWriteMultiplier)
		pricing.CacheCreation5mPrice = applyMultiplier(pricing.CacheCreation5mPrice, iv.CacheWriteMultiplier)
		pricing.CacheCreation1hPrice = applyMultiplier(pricing.CacheCreation1hPrice, iv.CacheWriteMultiplier)
	}
	if iv.CacheWrite1hPrice != nil {
		pricing.CacheCreation1hPrice = *iv.CacheWrite1hPrice
		pricing.SupportsCacheBreakdown = true
	}
	if iv.CacheReadPrice != nil {
		pricing.CacheReadPricePerTokenPriority = channelTierOverridePrice(pricing.CacheReadPricePerToken, pricing.CacheReadPricePerTokenPriority, *iv.CacheReadPrice)
		pricing.CacheReadPricePerToken = *iv.CacheReadPrice
	} else if iv.CacheReadMultiplier != nil {
		pricing.CacheReadPricePerToken = applyMultiplier(pricing.CacheReadPricePerToken, iv.CacheReadMultiplier)
		pricing.CacheReadPricePerTokenPriority = applyMultiplier(pricing.CacheReadPricePerTokenPriority, iv.CacheReadMultiplier)
	}
	// 运营者价卡存在时，ImageOutputPrice 显式覆盖；图片输入价用价卡级配置
	// （区间不携带图片输入价，与 image_output 一致）。
	if chPricing != nil && overrideImagePrices {
		pricing.ImageOutputPriceExplicit = true
		if chPricing.ImageOutputPrice != nil {
			pricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
		}
		applyConfiguredImageInputPrice(chPricing, pricing)
	}
	return pricing
}

// GetRequestTierPrice 根据层级标签获取按次价格
func (r *ModelPricingResolver) GetRequestTierPrice(resolved *ResolvedPricing, tierLabel string) float64 {
	for _, tier := range resolved.RequestTiers {
		if tier.TierLabel == tierLabel && tier.PerRequestPrice != nil {
			return *tier.PerRequestPrice
		}
	}
	return 0
}

// GetRequestTierPriceByContext 根据 context token 数获取按次价格
func (r *ModelPricingResolver) GetRequestTierPriceByContext(resolved *ResolvedPricing, totalContextTokens int) float64 {
	iv := FindMatchingInterval(resolved.RequestTiers, totalContextTokens)
	if iv != nil && iv.PerRequestPrice != nil {
		return *iv.PerRequestPrice
	}
	return 0
}
