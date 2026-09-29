/**
 * 按 Token 分段计价（muqian 2026-09-29 定，长上下文规则已并进来）：
 * 按单次请求的输入 Token 数（输入 + 缓存写 + 缓存读）落在哪一段，整条请求按那一段的价计费。
 *
 * 约定：条目的基础价就是第一段；intervals 只存「超过某个 Token 数之后」的各段，区间左开右闭 (min_tokens, max_tokens]，
 * 按 min_tokens 升序、每段 max_tokens = 下一段的 min_tokens、最后一段不封顶；Token 数 ≤ 第一个分段的 min_tokens 时按基础价。
 * 分段里没填的单价按基础价计：后端 intervalToModelPricing 以基础价为底、只覆盖分段里给了的项（只给倍率的按基础价 × 倍率）。
 * 按次 / 图片 / 视频模式的 intervals 是按 tier_label 分档，不是分段，这里一概不认。
 */

/** 一段的单价（与传入的基础价同单位：目录里是 USD / token） */
export interface TokenSegmentPrices {
  input: number | null
  output: number | null
  cacheWrite: number | null
  cacheWrite1h: number | null
  cacheRead: number | null
}

/** 管理端目录条目与用户站 /model-plaza 的区间字段交集 */
export interface TokenSegmentInterval {
  min_tokens: number
  max_tokens: number | null
  tier_label?: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price?: number | null
  cache_read_price: number | null
  input_multiplier?: number | null
  output_multiplier?: number | null
  cache_write_multiplier?: number | null
  cache_read_multiplier?: number | null
}

export interface TokenSegment {
  /** 下界（不含）；第一段（基础价）为 0 */
  min: number
  /** 上界（含）；null = 不封顶 */
  max: number | null
  prices: TokenSegmentPrices
}

/** 条目里的 Token 分段：不带 tier_label 的区间，按 min_tokens 升序 */
export function tokenIntervals<T extends TokenSegmentInterval>(intervals: readonly T[] | null | undefined): T[] {
  return (intervals ?? []).filter((iv) => !iv.tier_label).sort((a, b) => a.min_tokens - b.min_tokens)
}

function scaled(base: number | null, multiplier: number | null | undefined): number | null {
  if (base == null || multiplier == null) return base
  return base * multiplier
}

/** 这一段实际计费的单价：给了绝对价用绝对价，只给了倍率用基础价 × 倍率，都没给按基础价 */
export function segmentPrices(base: TokenSegmentPrices, iv: TokenSegmentInterval): TokenSegmentPrices {
  // 1 小时缓存写：分段单给了用它；分段只给了 5 分钟价时后端两档都按它算；基础价没分开定 1 小时价的就不列
  let cacheWrite1h: number | null = iv.cache_write_1h_price ?? null
  if (cacheWrite1h == null && base.cacheWrite1h != null) {
    cacheWrite1h = iv.cache_write_price ?? scaled(base.cacheWrite1h, iv.cache_write_multiplier)
  }
  return {
    input: iv.input_price ?? scaled(base.input, iv.input_multiplier),
    output: iv.output_price ?? scaled(base.output, iv.output_multiplier),
    cacheWrite: iv.cache_write_price ?? scaled(base.cacheWrite, iv.cache_write_multiplier),
    cacheWrite1h,
    cacheRead: iv.cache_read_price ?? scaled(base.cacheRead, iv.cache_read_multiplier)
  }
}

/** 全部分段（第一段 = 基础价）；没配分段时返回空数组 */
export function tokenSegments(base: TokenSegmentPrices, intervals: readonly TokenSegmentInterval[] | null | undefined): TokenSegment[] {
  const list = tokenIntervals(intervals)
  if (list.length === 0) return []
  return [
    { min: 0, max: list[0].min_tokens, prices: base },
    ...list.map((iv) => ({ min: iv.min_tokens, max: iv.max_tokens, prices: segmentPrices(base, iv) }))
  ]
}

/** Token 数：整百万写 M、整千写 K，否则千分位（272000 → 272K，1000000 → 1M，32768 → 32,768） */
export function formatTokenCount(tokens: number): string {
  if (tokens !== 0 && tokens % 1_000_000 === 0) return `${tokens / 1_000_000}M`
  if (tokens !== 0 && tokens % 1_000 === 0) return `${tokens / 1_000}K`
  return tokens.toLocaleString('en-US')
}

/** 段的范围：第一段「≤272K」、中间段「32K–128K」、最后一段「>128K」 */
export function formatSegmentRange(segment: Pick<TokenSegment, 'min' | 'max'>): string {
  if (segment.min === 0 && segment.max != null) return `≤${formatTokenCount(segment.max)}`
  if (segment.max == null) return `>${formatTokenCount(segment.min)}`
  return `${formatTokenCount(segment.min)}–${formatTokenCount(segment.max)}`
}
