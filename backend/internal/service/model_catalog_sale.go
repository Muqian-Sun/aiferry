package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

// 售价（muqian 2026-10-06：每项单独填售价，没填的按官方价 × 默认售价比例；用户倍率 = 在售价上再打折）。
//
// 计费、利润门、模型广场内部一律用「官方价口径」：用户的计费倍率 = 售价折扣 × DefaultSalePriceRatio。
// 填了售价的项先换算成官方口径（售价 ÷ DefaultSalePriceRatio），再乘计费倍率，正好是「售价 × 折扣」；
// 没填的项就是官方价本身，乘计费倍率是「官方价 × 默认售价比例 × 折扣」。所以一项售价都没填的模型
// 与以前（统一 1/15）完全一样。

// CatalogSalePrices / CatalogSaleSegment 见 domain：整份存成 model_catalog_entries.sale_prices。
type (
	CatalogSalePrices = domain.CatalogSalePrices
	// TimePricingSpec 售价忙闲时的存储形状（与 TimePricing 同字段）
	TimePricingSpec       = domain.TimePricingSpec
	TimePricingSpecPeriod = domain.TimePricingSpecPeriod
	CatalogSaleSegment    = domain.CatalogSaleSegment
)

// saleItems 五项售价，顺序与 segmentPriceBase.pricesAt 相同：输入、输出、缓存写 5 分钟、缓存写 1 小时、缓存读。
func saleBaseItems(p CatalogSalePrices) [5]*float64 {
	return [5]*float64{p.InputPrice, p.OutputPrice, p.CacheWritePrice, p.CacheWrite1hPrice, p.CacheReadPrice}
}

func saleSegmentFor(p CatalogSalePrices, minTokens int) *CatalogSaleSegment {
	for i := range p.Segments {
		if p.Segments[i].MinTokens == minTokens {
			return &p.Segments[i]
		}
	}
	return nil
}

// saleExplicitAt 这一段五项里单独定了售价的项（售价本身，USD / token）；没定的为 nil。
// segmentMin 为 nil 表示落在基础价。段内没定、基础价定了的项按「基础售价 × 本段官方价 ÷ 基础官方价」推
// （与缓存价随段折算同一个思路，各段与官方价同比例）；基础官方价没有或为 0 时推不出，留空。
func saleExplicitAt(p CatalogSalePrices, segmentMin *int, officialBase, officialSeg [5]*float64) [5]*float64 {
	base := saleBaseItems(p)
	if segmentMin == nil {
		return base
	}
	var out [5]*float64
	var seg [5]*float64
	if s := saleSegmentFor(p, *segmentMin); s != nil {
		seg = [5]*float64{s.InputPrice, s.OutputPrice, s.CacheWritePrice, s.CacheWrite1hPrice, s.CacheReadPrice}
	}
	for i := range out {
		switch {
		case seg[i] != nil:
			out[i] = seg[i]
		case base[i] != nil && officialBase[i] != nil && *officialBase[i] > 0 && officialSeg[i] != nil:
			v := *base[i] * *officialSeg[i] / *officialBase[i]
			out[i] = &v
		}
	}
	return out
}

// toOfficialBasis 售价换算成官方口径：售价 ÷ 默认售价比例。
func toOfficialBasis(sale float64) float64 {
	return sale / DefaultSalePriceRatio
}

// modelPricingItems 价卡的五项（0 视为没有）。
func modelPricingItems(p *ModelPricing) [5]*float64 {
	if p == nil {
		return [5]*float64{}
	}
	pos := func(v float64) *float64 {
		if v <= 0 {
			return nil
		}
		return &v
	}
	cacheWrite := p.CacheCreation5mPrice
	if cacheWrite <= 0 {
		cacheWrite = p.CacheCreationPricePerToken
	}
	return [5]*float64{pos(p.InputPricePerToken), pos(p.OutputPricePerToken), pos(cacheWrite), pos(p.CacheCreation1hPrice), pos(p.CacheReadPricePerToken)}
}

// saleEquivalentPricing 计费用：把这次请求落到的那一段里单独定了售价的项换成官方口径，放进 officialSeg 的副本。
// officialBase / officialSeg 是目录官方价（基础价、本段）——没定售价的项沿用本段官方价。忙闲时不在这里乘：
// 计费最后整单乘条目的分时，售价与官方价一起加。一项都没定时返回 nil（实付就按 officialSeg 算）。
func saleEquivalentPricing(sale CatalogSalePrices, segment *PricingInterval, officialBase, officialSeg *ModelPricing) *ModelPricing {
	if sale.IsZero() || officialSeg == nil {
		return nil
	}
	var segmentMin *int
	if segment != nil {
		segmentMin = &segment.MinTokens
	}
	explicit := saleExplicitAt(sale, segmentMin, modelPricingItems(officialBase), modelPricingItems(officialSeg))
	if explicit == [5]*float64{} {
		return nil
	}
	out := *officialSeg
	if v := explicit[0]; v != nil {
		out.InputPricePerToken = toOfficialBasis(*v)
	}
	if v := explicit[1]; v != nil {
		out.OutputPricePerToken = toOfficialBasis(*v)
	}
	if v := explicit[2]; v != nil {
		out.CacheCreationPricePerToken = toOfficialBasis(*v)
		out.CacheCreation5mPrice = toOfficialBasis(*v)
		out.CacheCreationPriceExplicit = true
	}
	if v := explicit[3]; v != nil {
		out.CacheCreation1hPrice = toOfficialBasis(*v)
	}
	if v := explicit[4]; v != nil {
		out.CacheReadPricePerToken = toOfficialBasis(*v)
	}
	return &out
}

// saleEquivalentPriceBase 售价口径的五项价与分段（官方口径：定了售价的项 = 售价 ÷ 默认售价比例，没定的 = 官方价），
// 给利润门、上架提示与价格页拿来和上游价比。一项售价都没定时就是官方价本身。分段一律写成绝对价。
func (e *ModelCatalogEntry) saleEquivalentPriceBase() segmentPriceBase {
	official := segmentPriceBase{
		input: e.InputPrice, output: e.OutputPrice,
		cacheWrite: e.CacheWritePrice, cacheWrite1h: e.CacheWrite1hPrice, cacheRead: e.CacheReadPrice,
		intervals: e.Intervals,
	}
	if e.SalePrices.IsZero() {
		return official
	}
	officialBase := [5]*float64{official.input, official.output, official.cacheWrite, official.cacheWrite1h, official.cacheRead}
	pick := func(explicit, fallback *float64) *float64 {
		if explicit != nil {
			v := toOfficialBasis(*explicit)
			return &v
		}
		return fallback
	}
	baseExplicit := saleExplicitAt(e.SalePrices, nil, officialBase, officialBase)
	out := segmentPriceBase{
		input: pick(baseExplicit[0], official.input), output: pick(baseExplicit[1], official.output),
		cacheWrite: pick(baseExplicit[2], official.cacheWrite), cacheWrite1h: pick(baseExplicit[3], official.cacheWrite1h),
		cacheRead: pick(baseExplicit[4], official.cacheRead),
	}
	for _, iv := range e.Intervals {
		officialSeg := official.pricesAt(iv.MinTokens + 1)
		min := iv.MinTokens
		explicit := saleExplicitAt(e.SalePrices, &min, officialBase, officialSeg)
		seg := iv
		seg.InputPrice, seg.OutputPrice = pick(explicit[0], officialSeg[0]), pick(explicit[1], officialSeg[1])
		seg.CacheWritePrice, seg.CacheWrite1hPrice = pick(explicit[2], officialSeg[2]), pick(explicit[3], officialSeg[3])
		seg.CacheReadPrice = pick(explicit[4], officialSeg[4])
		seg.InputMultiplier, seg.OutputMultiplier, seg.CacheWriteMultiplier, seg.CacheReadMultiplier = nil, nil, nil, nil
		out.intervals = append(out.intervals, seg)
	}
	return out
}

// SaleEquivalentPricingCard 模型广场用的价卡：五项与分段换成售价口径（官方口径，展示时 × 访问者的计费倍率 = 售价 × 折扣）；
// 一项售价都没定时就是 PricingCard 本身。图片 / 音频 / 按次价不在这次售价范围里，照旧是官方价。
func (e *ModelCatalogEntry) SaleEquivalentPricingCard() *PricingCard {
	card := e.PricingCard()
	if card == nil || e.SalePrices.IsZero() || e.EffectiveBillingMode() != BillingModeToken {
		return card
	}
	base := e.saleEquivalentPriceBase()
	card.InputPrice, card.OutputPrice = base.input, base.output
	card.CacheWritePrice, card.CacheWrite1hPrice, card.CacheReadPrice = base.cacheWrite, base.cacheWrite1h, base.cacheRead
	card.Intervals = base.intervals
	return card
}

// SaleTimePricing 向用户收钱时整单乘的忙闲时：售价单独定了按售价的（没有时段 = 全天一个价），没定跟官方忙闲时。
// 计费、利润门、模型广场都按它。
func (e *ModelCatalogEntry) SaleTimePricing() *TimePricing {
	if e == nil {
		return nil
	}
	spec := e.SalePrices.TimePricing
	if spec == nil {
		return e.TimePricing
	}
	if len(spec.Periods) == 0 {
		return nil
	}
	tp := &TimePricing{Timezone: spec.Timezone, WeekdaysOnly: spec.WeekdaysOnly, ExcludeDates: append([]string(nil), spec.ExcludeDates...)}
	for _, period := range spec.Periods {
		tp.Periods = append(tp.Periods, TimePricingPeriod{StartTime: period.StartTime, EndTime: period.EndTime, Multiplier: period.Multiplier})
	}
	return tp
}

// SaleMaxReasoningMultiplier 向用户收钱时最高推理档整单乘的倍数：售价单独定了按售价的，没定跟官方；nil = 不加价。
// 计费与模型广场都按它；定成 1 也是不加价，返回 nil（广场不显示「整单 × 1」）。
func (e *ModelCatalogEntry) SaleMaxReasoningMultiplier() *float64 {
	if e == nil {
		return nil
	}
	m := e.SalePrices.MaxReasoningEffortMultiplier
	if m == nil {
		m = e.MaxReasoningEffortMultiplier
	}
	if m == nil || *m == 1 {
		return nil
	}
	return m
}

// validateSalePrices 售价不能为负；只有按 token 计费的模型能定售价；各段按下界对上官方价的分段，同一段不能写两次；
// 售价忙闲时与官方忙闲时同一套校验；最高推理倍率与官方的一样须 > 0。
func validateSalePrices(e *ModelCatalogEntry) error {
	p := e.SalePrices
	if p.IsZero() {
		return nil
	}
	if e.EffectiveBillingMode() != BillingModeToken {
		return catalogValidationError("sale prices are only for token-billed models")
	}
	if p.TimePricing != nil && len(p.TimePricing.Periods) > 0 {
		if err := validateCatalogLength("sale_prices.time_pricing.timezone", p.TimePricing.Timezone, 64); err != nil {
			return err
		}
		if err := validateTimePricing(e.SaleTimePricing()); err != nil {
			return catalogValidationError(fmt.Sprintf("sale_prices.time_pricing: %s", err.Error()))
		}
	}
	if m := p.MaxReasoningEffortMultiplier; m != nil && *m <= 0 {
		return catalogValidationError("sale max_reasoning_effort_multiplier must be > 0")
	}
	check := func(where string, items [5]*float64) error {
		names := [5]string{"input_price", "output_price", "cache_write_price", "cache_write_1h_price", "cache_read_price"}
		for i, v := range items {
			if v != nil && *v < 0 {
				return catalogValidationError(fmt.Sprintf("sale %s%s must be >= 0", where, names[i]))
			}
		}
		return nil
	}
	if err := check("", saleBaseItems(p)); err != nil {
		return err
	}
	officialMins := make(map[int]struct{}, len(e.Intervals))
	for _, iv := range e.Intervals {
		officialMins[iv.MinTokens] = struct{}{}
	}
	seen := make(map[int]struct{}, len(p.Segments))
	for _, s := range p.Segments {
		if _, ok := officialMins[s.MinTokens]; !ok {
			return catalogValidationError(fmt.Sprintf("sale segment above %d tokens has no matching official segment", s.MinTokens))
		}
		if _, dup := seen[s.MinTokens]; dup {
			return catalogValidationError(fmt.Sprintf("sale segment above %d tokens is set twice", s.MinTokens))
		}
		seen[s.MinTokens] = struct{}{}
		if err := check(fmt.Sprintf("segment %d ", s.MinTokens), [5]*float64{s.InputPrice, s.OutputPrice, s.CacheWritePrice, s.CacheWrite1hPrice, s.CacheReadPrice}); err != nil {
			return err
		}
	}
	return nil
}

// normalizeSalePrices 去掉一项都没填的段，各段按下界排好；一项都没填时是零值（存成 {}）。
// 售价每百万 Token 最多保留 4 位小数（muqian 2026-10-07）。
func normalizeSalePrices(p CatalogSalePrices) CatalogSalePrices {
	out := CatalogSalePrices{
		InputPrice: roundPricePtr(p.InputPrice), OutputPrice: roundPricePtr(p.OutputPrice),
		CacheWritePrice: roundPricePtr(p.CacheWritePrice), CacheWrite1hPrice: roundPricePtr(p.CacheWrite1hPrice),
		CacheReadPrice: roundPricePtr(p.CacheReadPrice),
	}
	for _, s := range p.Segments {
		if s.InputPrice == nil && s.OutputPrice == nil && s.CacheWritePrice == nil && s.CacheWrite1hPrice == nil && s.CacheReadPrice == nil {
			continue
		}
		out.Segments = append(out.Segments, CatalogSaleSegment{
			MinTokens: s.MinTokens, InputPrice: roundPricePtr(s.InputPrice), OutputPrice: roundPricePtr(s.OutputPrice),
			CacheWritePrice: roundPricePtr(s.CacheWritePrice), CacheWrite1hPrice: roundPricePtr(s.CacheWrite1hPrice),
			CacheReadPrice: roundPricePtr(s.CacheReadPrice),
		})
	}
	sort.Slice(out.Segments, func(i, j int) bool { return out.Segments[i].MinTokens < out.Segments[j].MinTokens })
	out.TimePricing = normalizeSaleTimePricing(p.TimePricing)
	out.MaxReasoningEffortMultiplier = clonePricePtr(p.MaxReasoningEffortMultiplier)
	return out
}

// normalizeSaleTimePricing nil = 跟官方；没有时段 = 全天一个价（只留空时段，其余字段清掉）。
func normalizeSaleTimePricing(spec *TimePricingSpec) *TimePricingSpec {
	if spec == nil {
		return nil
	}
	if len(spec.Periods) == 0 {
		return &TimePricingSpec{Periods: []TimePricingSpecPeriod{}}
	}
	return &TimePricingSpec{
		Timezone:     strings.TrimSpace(spec.Timezone),
		WeekdaysOnly: spec.WeekdaysOnly,
		Periods:      append([]TimePricingSpecPeriod(nil), spec.Periods...),
		ExcludeDates: normalizeExcludeDates(spec.ExcludeDates),
	}
}
