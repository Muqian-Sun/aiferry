package service

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// 渠道成本 = 用量 × 这个渠道给这个模型的上游价（承接关系上的上游价）。
// muqian 2026-09-30：「付给上游的钱就是上游的标价就行了，不要再引入渠道倍率这个概念」。
//
// 上游价与官方价是同一套字段、同一套算法：把承接关系上的上游价换进目录条目，得到一张「上游价卡」，
// 再用算用户价的同一个计算器算一遍（分段、缓存 5 分钟 / 1 小时、最高推理倍率都跟着走），倍率取 1。

// ErrUpstreamPriceNotFound 这次请求找不到承接关系（没有上游价），渠道成本无从算起。
var ErrUpstreamPriceNotFound = errors.New("upstream price not found for account and model")

// BindingFor 返回条目上这个渠道的承接关系；没有时为 nil。
func (e *ModelCatalogEntry) BindingFor(accountID int64) *ModelCatalogBinding {
	if e == nil {
		return nil
	}
	for i := range e.Bindings {
		if e.Bindings[i].AccountID == accountID {
			return &e.Bindings[i]
		}
	}
	return nil
}

// upstreamCatalogEntry 用承接关系上的上游价替换条目的 token 价与分段，得到「上游价卡」：
//   - 分时用承接上的上游忙闲时（muqian 2026-10-06：上游有没有忙闲时在填承接时定），不用条目给用户定的分时；
//   - 上游价没填的缓存项只会是官方价也没单独配的（ValidateAgainst 要求官方有的项必填），按 0 算；
//   - 图片 / 音频 token 价、最高推理倍率沿用条目（大语言模型用不到前两项）。
func upstreamCatalogEntry(entry *ModelCatalogEntry, b *ModelCatalogBinding) *ModelCatalogEntry {
	up := entry.Clone()
	input, output := b.InputPrice, b.OutputPrice
	up.InputPrice = &input
	up.OutputPrice = &output
	up.CacheWritePrice = clonePricePtr(b.CacheWritePrice)
	up.CacheWrite1hPrice = clonePricePtr(b.CacheWrite1hPrice)
	up.CacheReadPrice = clonePricePtr(b.CacheReadPrice)
	up.Intervals = append([]PricingInterval(nil), b.Intervals...)
	up.TimePricing = b.TimePricing
	up.SalePrices = CatalogSalePrices{}
	up.Bindings = nil
	return up
}

// LookupUpstreamPrice 找这次请求的承接关系：按计费模型查目录条目（别名归一由目录完成），
// 再在条目的承接关系里找这个渠道。找不到返回 nil, nil。
func (r *ModelPricingResolver) LookupUpstreamPrice(ctx context.Context, model string, accountID int64) (*ModelCatalogEntry, *ModelCatalogBinding) {
	entry := r.lookupCatalogEntry(ctx, model)
	if entry == nil {
		return nil, nil
	}
	binding := entry.BindingFor(accountID)
	if binding == nil {
		return nil, nil
	}
	return entry, binding
}

// CalculateUpstreamCost 按上游价卡算这次 token 用量的渠道成本（USD）。
func (s *BillingService) CalculateUpstreamCost(ctx context.Context, resolver *ModelPricingResolver, entry *ModelCatalogEntry, binding *ModelCatalogBinding, tokens UsageTokens, pricingAt time.Time, reasoningEffort string) (float64, error) {
	if s == nil || resolver == nil || entry == nil || binding == nil {
		return 0, ErrUpstreamPriceNotFound
	}
	upstream := upstreamCatalogEntry(entry, binding)
	cost, err := s.CalculateCostUnified(CostInput{
		Ctx:             ctx,
		Model:           entry.ModelID,
		Tokens:          tokens,
		RequestCount:    1,
		RateMultiplier:  1,
		PricingAt:       pricingAt,
		ReasoningEffort: reasoningEffort,
		Resolver:        resolver,
		Resolved:        resolver.resolveCatalogPricing(upstream),
	})
	if err != nil {
		return 0, err
	}
	return cost.TotalCost, nil
}

// recordUsageAccountCost 两个网关记账时的渠道成本：按候选计费模型依次找这个渠道的承接关系，
// 用第一个找得到的算。媒体用量（图片 / 视频 / 音频）不经这里，记 0。
// 找不到承接关系或算不出来时记 0 并告警：请求已经完成，不能再拒，只能把缺口暴露出来。
//
// 联网搜索的上游成本按承接关系上的搜索上游价算，没填的项不收（见 upstreamWebSearchPrices）。
func recordUsageAccountCost(ctx context.Context, billing *BillingService, resolver *ModelPricingResolver, accountID int64, models []string, tokens UsageTokens, search WebSearchUsage, pricingAt time.Time, reasoningEffort string) float64 {
	if billing == nil || resolver == nil {
		return 0
	}
	for _, model := range models {
		entry, binding := resolver.LookupUpstreamPrice(ctx, model, accountID)
		if binding == nil {
			continue
		}
		cost, err := billing.CalculateUpstreamCost(ctx, resolver, entry, binding, tokens, pricingAt, reasoningEffort)
		if err != nil {
			slog.Warn("usage_account_cost_failed", "account_id", accountID, "model", entry.ModelID, "error", err)
			return 0
		}
		return cost + upstreamWebSearchPrices(binding).cost(search)
	}
	slog.Warn("usage_account_cost_missing_upstream_price", "account_id", accountID, "models", models)
	return 0
}

// bindingCostRatio 上游价相对售价口径（官方口径：定了售价的项 = 售价 ÷ 默认售价比例，没定的 = 官方价）的最高比值：
// 逐项（输入、输出、缓存写 5 分钟 / 1 小时、缓存读）、逐段比，取最大的一个。两边分段切点不一样时，先把两边的切点
// 合在一起，再在每个切点之后比。售价口径没有（或为 0）的项不参与比较；一项都比不了时 ok=false。
// 利润门拿它和「计费倍率 × (1 − 最低毛利率)」比（D3）；价格页的毛利（1 − 它 ÷ 默认售价比例）也按它算。
func bindingCostRatio(entry *ModelCatalogEntry, b *ModelCatalogBinding) (ratio float64, ok bool) {
	if entry == nil || b == nil {
		return 0, false
	}
	// 和售价比（muqian 2026-10-06：单独定了售价的项按售价，没定的按官方价 × 默认售价比例）；都在官方口径上比，
	// 利润门的阈值 = 计费倍率 × (1 − 最低毛利率)，正好是「上游价 ≤ 售价 × 折扣 × (1 − 最低毛利率)」。
	official := entry.saleEquivalentPriceBase()
	in, out := b.InputPrice, b.OutputPrice
	upstream := segmentPriceBase{
		input: &in, output: &out,
		cacheWrite: b.CacheWritePrice, cacheWrite1h: b.CacheWrite1hPrice, cacheRead: b.CacheReadPrice,
		intervals: b.Intervals,
	}
	points := []int{0}
	for _, iv := range entry.Intervals {
		points = append(points, iv.MinTokens)
	}
	for _, iv := range b.Intervals {
		points = append(points, iv.MinTokens)
	}
	upstreamCharges := false
	for _, point := range points {
		// 分段区间左开右闭 (min, max]：切点之后的第一个 token 数落在下一段。
		tokens := point + 1
		off := official.pricesAt(tokens)
		up := upstream.pricesAt(tokens)
		for i := range off {
			if up[i] != nil && *up[i] > 0 {
				upstreamCharges = true
			}
			if off[i] == nil || *off[i] <= 0 || up[i] == nil {
				continue
			}
			r := *up[i] / *off[i]
			if !ok || r > ratio {
				ratio, ok = r, true
			}
		}
	}
	// 官网免费的模型（输入、输出价显式为 0，没有一项要钱）：上游也不收钱就不亏，成本比按 0；
	// 上游收钱时照旧比不出（利润门按缺价挡掉）。
	if !ok && isZeroPrice(official.input) && isZeroPrice(official.output) && !upstreamCharges {
		return 0, true
	}
	return ratio, ok
}

func isZeroPrice(p *float64) bool { return p != nil && *p == 0 }

// segmentPriceBase 一张价卡的五项 token 价与分段（官方价或上游价），用于按 token 数取某一段的价。
type segmentPriceBase struct {
	input, output, cacheWrite, cacheWrite1h, cacheRead *float64
	intervals                                          []PricingInterval
}

// pricesAt 返回上下文 token 数落在哪一段时的五项价（输入、输出、缓存写 5 分钟、缓存写 1 小时、缓存读）。
// 与计费的分段规则一致（intervalToModelPricing）：段内没填的输入 / 输出沿用基础价；没填的缓存价按
// 「本段输入价 ÷ 基础输入价」同比例折算；段内给的是倍率时按基础价 × 倍率。
func (p segmentPriceBase) pricesAt(tokens int) [5]*float64 {
	base := [5]*float64{p.input, p.output, p.cacheWrite, p.cacheWrite1h, p.cacheRead}
	iv := FindMatchingInterval(p.intervals, tokens)
	if iv == nil {
		return base
	}
	scaled := func(v *float64, factor float64) *float64 {
		if v == nil {
			return nil
		}
		x := *v * factor
		return &x
	}
	pick := func(abs *float64, mult *float64, baseValue *float64) *float64 {
		if abs != nil {
			return abs
		}
		if mult != nil {
			return scaled(baseValue, *mult)
		}
		return baseValue
	}
	input := pick(iv.InputPrice, iv.InputMultiplier, p.input)
	inputRatio := 1.0
	if input != nil && p.input != nil && *p.input > 0 {
		inputRatio = *input / *p.input
	}
	cacheFactor := func(mult *float64) *float64 {
		if mult != nil {
			return mult
		}
		return &inputRatio
	}
	cacheWrite := iv.CacheWritePrice
	if cacheWrite == nil {
		cacheWrite = scaled(p.cacheWrite, *cacheFactor(iv.CacheWriteMultiplier))
	}
	cacheWrite1h := iv.CacheWrite1hPrice
	if cacheWrite1h == nil {
		cacheWrite1h = scaled(p.cacheWrite1h, *cacheFactor(iv.CacheWriteMultiplier))
	}
	cacheRead := iv.CacheReadPrice
	if cacheRead == nil {
		cacheRead = scaled(p.cacheRead, *cacheFactor(iv.CacheReadMultiplier))
	}
	return [5]*float64{input, pick(iv.OutputPrice, iv.OutputMultiplier, p.output), cacheWrite, cacheWrite1h, cacheRead}
}
