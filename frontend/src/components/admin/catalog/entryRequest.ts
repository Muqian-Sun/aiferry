/**
 * 目录条目 ↔ 保存请求的映射（编辑器与批量上下架共用）。
 * PUT /admin/model-catalog/entries/:id 是整条覆盖，所以任何只想改一个字段的地方都得先把整条投影成请求体。
 */
import type { ModelCatalogEntry, ModelCatalogEntryRequest, PricingInterval } from '@/api/admin/modelCatalog'
import { tokenIntervals } from '@/utils/tokenSegments'

// 数字输入清空后 v-model.number 得到 ''，后端按 *float64 解析会报 400：清空即「未配置」，发 null。
export const NUMERIC_FIELDS = [
  'input_price',
  'output_price',
  'cache_write_price',
  'cache_write_1h_price',
  'cache_read_price',
  'image_input_price',
  'image_output_price',
  'image_cache_read_price',
  'per_request_price',
  'search_price_per_call',
  'max_reasoning_effort_multiplier'
] as const satisfies readonly (keyof ModelCatalogEntryRequest)[]

/**
 * 音频单价：新后端才有的字段。只在有值时放进请求体——旧后端忽略未知字段，
 * 新后端整条覆盖时「没传」与「传 null」一样都是未配置，所以省掉 null 不会丢值。
 */
export const OPTIONAL_NUMERIC_FIELDS = ['audio_input_price', 'audio_output_price'] as const satisfies readonly (keyof ModelCatalogEntryRequest)[]

/** 就地把可选单价规整成数字；空值直接删掉这个键 */
export function applyOptionalPrices(body: ModelCatalogEntryRequest, source: Partial<Record<(typeof OPTIONAL_NUMERIC_FIELDS)[number], unknown>>): ModelCatalogEntryRequest {
  for (const field of OPTIONAL_NUMERIC_FIELDS) {
    const value = numberOrNull(source[field])
    if (value == null) delete body[field]
    else body[field] = value
  }
  return body
}

export function numberOrNull(value: unknown): number | null {
  if (value === '' || value === null || value === undefined) return null
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

/** 把一条目录条目原样投影成保存请求（区间分档、分时定价、隐藏价一并带上）。 */
export function entryToRequest(entry: ModelCatalogEntry): ModelCatalogEntryRequest {
  const body: ModelCatalogEntryRequest = {
    model_id: entry.model_id,
    display_name: entry.display_name,
    vendor: entry.vendor,
    protocols: entry.protocols,
    billing_mode: entry.billing_mode,
    status: entry.status,
    input_price: entry.input_price,
    output_price: entry.output_price,
    cache_write_price: entry.cache_write_price,
    cache_write_1h_price: entry.cache_write_1h_price,
    cache_read_price: entry.cache_read_price,
    image_input_price: entry.image_input_price,
    image_output_price: entry.image_output_price,
    image_cache_read_price: entry.image_cache_read_price,
    per_request_price: entry.per_request_price,
    search_price_per_call: entry.search_price_per_call,
    max_reasoning_effort_multiplier: entry.max_reasoning_effort_multiplier,
    notes: entry.notes,
    intervals: entry.intervals,
    time_pricing: entry.time_pricing
  }
  return applyOptionalPrices(body, entry)
}

/** 图片 / 视频分档：档位只能是后端认的这几个（计费查档区分大小写），每档一个按次价。 */
export const IMAGE_TIER_LABELS = ['1K', '2K', '4K'] as const
export const VIDEO_TIER_LABELS = ['480p', '720p', '1080p'] as const

export interface MediaTierForm {
  tier_label: string
  per_request_price: number | null
}

/** 编辑器里的分档从条目 intervals 投影；只认带 tier_label 的（token 区间分档不进这张表）。 */
export function mediaTiersFromIntervals(intervals: PricingInterval[] | undefined): MediaTierForm[] {
  return (intervals ?? [])
    .filter((iv) => iv.tier_label)
    .map((iv) => ({ tier_label: iv.tier_label, per_request_price: iv.per_request_price }))
}

export function mediaTiersToIntervals(tiers: MediaTierForm[]): PricingInterval[] {
  return tiers.map((tier, index) => ({
    min_tokens: 0,
    max_tokens: null,
    tier_label: tier.tier_label,
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    input_multiplier: null,
    output_multiplier: null,
    cache_write_multiplier: null,
    cache_read_multiplier: null,
    per_request_price: numberOrNull(tier.per_request_price),
    sort_order: index
  }))
}

/**
 * 按 Token 分段的一行（muqian 2026-09-29：阶梯只做按 Token 分段，长上下文并进来删掉）。
 * 条目的基础价就是第一段，这里只放「超过某个 Token 数之后」的各段；单价按 $/token 存，空 = 按第一段的价计。
 */
export interface TokenSegmentForm {
  /** 超过多少 Token（不含）进入这一段；存输入框里的原文，校验与保存时再解析 */
  above: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_write_1h_price: number | null
  cache_read_price: number | null
}

export const TOKEN_SEGMENT_PRICE_KEYS = [
  'input_price',
  'output_price',
  'cache_read_price',
  'cache_write_price',
  'cache_write_1h_price'
] as const satisfies readonly (keyof TokenSegmentForm)[]

type BasePrices = Pick<ModelCatalogEntryRequest, 'input_price' | 'output_price' | 'cache_write_price' | 'cache_write_1h_price' | 'cache_read_price'>

/** 只给了倍率的分段价换成绝对价（后端按基础价 × 倍率计）；本页只写绝对价，不换就会在保存时丢掉倍率、退回第一段的价 */
function absolutePrice(price: number | null | undefined, multiplier: number | null | undefined, base: number | null | undefined): number | null {
  if (price != null) return price
  if (multiplier == null || base == null) return null
  return base * multiplier
}

/** 条目 intervals → 分段行：只认不带 tier_label 的区间，按 min_tokens 升序 */
export function tokenSegmentsFromIntervals(intervals: PricingInterval[] | undefined, base: BasePrices): TokenSegmentForm[] {
  return tokenIntervals(intervals).map((iv) => ({
    above: String(iv.min_tokens),
    input_price: absolutePrice(iv.input_price, iv.input_multiplier, base.input_price),
    output_price: absolutePrice(iv.output_price, iv.output_multiplier, base.output_price),
    cache_write_price: absolutePrice(iv.cache_write_price, iv.cache_write_multiplier, base.cache_write_price),
    cache_write_1h_price: absolutePrice(iv.cache_write_1h_price, iv.cache_write_multiplier, base.cache_write_1h_price),
    cache_read_price: absolutePrice(iv.cache_read_price, iv.cache_read_multiplier, base.cache_read_price)
  }))
}

/** 「超过多少 Token」：正整数才算数，其余（空、小数、负数、非数字）为 null */
export function parseTokenThreshold(text: string): number | null {
  const trimmed = text.trim()
  if (!/^\d+$/.test(trimmed)) return null
  const value = Number(trimmed)
  return Number.isSafeInteger(value) && value > 0 ? value : null
}

export type TokenSegmentError = 'required' | 'integer' | 'notAscending' | 'noPrice'

/** 每行的校验结果（null = 没问题）：Token 数是正整数、严格大于上一行，且至少填了一个价 */
export function tokenSegmentErrors(rows: TokenSegmentForm[]): Array<TokenSegmentError | null> {
  return rows.map((row, index) => {
    if (row.above.trim() === '') return 'required'
    const above = parseTokenThreshold(row.above)
    if (above == null) return 'integer'
    const previous = index > 0 ? parseTokenThreshold(rows[index - 1].above) : null
    if (previous != null && above <= previous) return 'notAscending'
    if (TOKEN_SEGMENT_PRICE_KEYS.every((key) => numberOrNull(row[key]) == null)) return 'noPrice'
    return null
  })
}

/**
 * 分段行 → 条目 intervals（调用前先过 tokenSegmentErrors）：每段 (above, 下一段的 above]，最后一段不封顶；
 * 不带 tier_label、只写绝对价、不写倍率。
 */
export function tokenSegmentsToIntervals(rows: TokenSegmentForm[]): PricingInterval[] {
  const thresholds = rows.map((row) => parseTokenThreshold(row.above))
  return rows.map((row, index) => {
    const min = thresholds[index]
    if (min == null) throw new Error(`token segment #${index + 1} has no valid threshold`)
    return {
      min_tokens: min,
      max_tokens: index < rows.length - 1 ? thresholds[index + 1] : null,
      tier_label: '',
      input_price: numberOrNull(row.input_price),
      output_price: numberOrNull(row.output_price),
      cache_write_price: numberOrNull(row.cache_write_price),
      cache_write_1h_price: numberOrNull(row.cache_write_1h_price),
      cache_read_price: numberOrNull(row.cache_read_price),
      input_multiplier: null,
      output_multiplier: null,
      cache_write_multiplier: null,
      cache_read_multiplier: null,
      per_request_price: null,
      sort_order: index
    }
  })
}

/** 条目是否有可用于上架的价格：token 模式看输入 / 输出价，其它模式看按次价或任一分档价。 */
export function hasPrice(entry: ModelCatalogEntry): boolean {
  if (entry.billing_mode === 'token' || !entry.billing_mode) {
    return entry.input_price != null || entry.output_price != null
  }
  return entry.per_request_price != null || (entry.intervals ?? []).some((iv) => iv.per_request_price != null)
}
