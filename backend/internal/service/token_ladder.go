package service

// tokenLadder 价格来源里「整次请求的输入侧 token 数超过阈值后，整条按倍数加价」的官方阶梯：
// 价格文件的 *_above_XXXk_tokens 字段（解析成 LiteLLMModelPricing.LongContext*）与兜底价表（fallbackSeedLadders）。
// 计费只认模型目录的按 token 分段（muqian 2026-09-29「只做按token分段计价」），阶梯只在建目录条目时换算成分段。
type tokenLadder struct {
	threshold int
	// inclusive 达到阈值即进高段（xAI 口径）；否则严格大于。
	inclusive bool
	// inputMultiplier 作用于输入侧（输入、缓存写、缓存读——缓存本质是输入的复用）；outputMultiplier 作用于输出。≤0 视为 1。
	inputMultiplier  float64
	outputMultiplier float64
}

// applyTo 给按 token 计费的条目追加阶梯对应的分段：从阈值往上不封顶，各项价 = 条目基础价 × 倍数；
// 基础价没配的项分段也不配（计费时回落到基础价，与乘之前一样是 0 或由模型策略补）。
// 分段左开右闭 (min, max]，「达到即进高段」的下界取 threshold-1。阈值不为正或两个倍数都不大于 1 时不加。
func (l tokenLadder) applyTo(entry *ModelCatalogEntry) {
	in, out := multiplierOrOne(l.inputMultiplier), multiplierOrOne(l.outputMultiplier)
	if entry == nil || entry.BillingMode != BillingModeToken || l.threshold <= 0 || (in <= 1 && out <= 1) {
		return
	}
	lower := l.threshold
	if l.inclusive {
		lower--
	}
	entry.Intervals = append(entry.Intervals, PricingInterval{
		MinTokens:         lower,
		InputPrice:        scaledPrice(entry.InputPrice, in),
		OutputPrice:       scaledPrice(entry.OutputPrice, out),
		CacheWritePrice:   scaledPrice(entry.CacheWritePrice, in),
		CacheWrite1hPrice: scaledPrice(entry.CacheWrite1hPrice, in),
		CacheReadPrice:    scaledPrice(entry.CacheReadPrice, in),
	})
}

func multiplierOrOne(m float64) float64 {
	if m <= 0 {
		return 1
	}
	return m
}

func scaledPrice(price *float64, multiplier float64) *float64 {
	if price == nil {
		return nil
	}
	v := *price * multiplier
	return &v
}
