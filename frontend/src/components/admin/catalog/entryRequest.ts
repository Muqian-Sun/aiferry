/**
 * 目录条目 ↔ 保存请求的映射（编辑器与批量上下架共用）。
 * PUT /admin/model-catalog/entries/:id 是整条覆盖，所以任何只想改一个字段的地方都得先把整条投影成请求体。
 */
import type { ModelCatalogEntry, ModelCatalogEntryRequest, PricingInterval } from '@/api/admin/modelCatalog'

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
  'input_price_priority',
  'output_price_priority',
  'cache_write_price_priority',
  'cache_read_price_priority',
  'per_request_price',
  'search_price_per_call',
  'long_context_input_threshold',
  'long_context_input_multiplier',
  'long_context_output_multiplier',
  'fast_multiplier',
  'flex_multiplier',
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
    input_price_priority: entry.input_price_priority,
    output_price_priority: entry.output_price_priority,
    cache_write_price_priority: entry.cache_write_price_priority,
    cache_read_price_priority: entry.cache_read_price_priority,
    per_request_price: entry.per_request_price,
    search_price_per_call: entry.search_price_per_call,
    long_context_input_threshold: entry.long_context_input_threshold,
    long_context_threshold_inclusive: entry.long_context_threshold_inclusive,
    long_context_input_multiplier: entry.long_context_input_multiplier,
    long_context_output_multiplier: entry.long_context_output_multiplier,
    fast_multiplier: entry.fast_multiplier,
    flex_multiplier: entry.flex_multiplier,
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

/** 条目是否有可用于上架的价格：token 模式看输入 / 输出价，其它模式看按次价或任一分档价。 */
export function hasPrice(entry: ModelCatalogEntry): boolean {
  if (entry.billing_mode === 'token' || !entry.billing_mode) {
    return entry.input_price != null || entry.output_price != null
  }
  return entry.per_request_price != null || (entry.intervals ?? []).some((iv) => iv.per_request_price != null)
}
