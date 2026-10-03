/**
 * 模型页的展示契约：每个上架的目录条目一格，点格子看全部计费项。
 *
 * 标价来自 /model-plaza 每个条目的 pricing（目录售价）。计费模式决定收哪些钱（后端 billing_service 的实际口径）：
 * - token：各项都是 USD / token，这里换算成 USD / 百万 token；按 Token 分段的（pricing.intervals 非空）每段一行（utils/tokenSegments），
 *   另有图片与音频单价、联网搜索按次价、最高推理档倍率；
 * - per_request / image / video：收 per_request_price 一个单价（单位分别是 次 / 张 / 秒），带 tier_label 的区间是按档位的单价；
 *   token 价不参与计费、不展示。
 * 用户价 = 标价 × 用户倍率，倍率由页面按登录态另取，这里不算。
 */
import type { PlazaModel, PlazaTimePricing, UserSupportedModelPricing } from '@/api/modelPlaza'
import { tokenSegments, type TokenSegment, type TokenSegmentPrices } from '@/utils/tokenSegments'

/** token 模式的各项单价，USD / 1M tokens；目录没给的项为 null */
export interface CatalogPrice {
  input: number | null
  output: number | null
  cacheWrite: number | null
  /** 1 小时缓存写入（只有与 5 分钟价分开定价的模型才有） */
  cacheWrite1h: number | null
  cacheRead: number | null
  imageInput: number | null
  imageOutput: number | null
  imageCacheRead: number | null
  audioInput: number | null
  audioOutput: number | null
}

export type CatalogPriceKey = keyof CatalogPrice

/** 分段表里的项（随分段变价的文本 Token 价），也是表格的列序 */
export const TOKEN_ROW_KEYS = ['input', 'output', 'cacheWrite', 'cacheWrite1h', 'cacheRead'] as const satisfies ReadonlyArray<keyof TokenSegmentPrices>
export type TokenRowKey = (typeof TOKEN_ROW_KEYS)[number]

/** 按次 / 图片 / 视频模式的一个档位（pricing.intervals 里带 tier_label 的），单价 USD / 次、张、秒 */
export interface CatalogTier {
  label: string
  price: number | null
}

export interface CatalogModel {
  id: string
  displayName: string
  vendor: string
  /** token / per_request / image / video；缺省视为 token */
  billingMode: string
  /** token 模式的基础单价；非 token 模式或各项都缺时为 null */
  price: CatalogPrice | null
  /** 非 token 模式的单价（USD / 次、张、秒）；token 模式或没给时为 null */
  unitPrice: number | null
  /** 非 token 模式的分档单价；没分档或 token 模式为空 */
  tiers: CatalogTier[]
  /**
   * token 模式标准价的各行，USD / 1M tokens：按 Token 分段的一段一行（第一段 = 基础价），没分段只有一行（0, 不封顶）；
   * 非 token 模式或文本 Token 价全缺时为空。多于一行即「按 Token 分段」。
   */
  rows: TokenSegment[]
  /** 联网搜索（官方原价，不乘倍率），USD / 千次；没有官方搜索工具的模型为 null */
  searchPerThousand: number | null
  /** xAI X 搜索按取回条目收：USD / 千条帖子、USD / 千个主页；非 xAI 模型为 null */
  xPostPerThousand: number | null
  xUserPerThousand: number | null
  /** reasoning_effort=max 时整单倍率；没配为 null */
  maxReasoningMultiplier: number | null
  aliases: string[]
  /** 分时倍率（有时段才带） */
  timePricing: PlazaTimePricing | null
}

const PER_MILLION = 1_000_000
const PER_THOUSAND = 1_000

function perMillion(value: number | null | undefined): number | null {
  return value == null ? null : value * PER_MILLION
}

function priceOf(p: UserSupportedModelPricing): CatalogPrice | null {
  const price: CatalogPrice = {
    input: perMillion(p.input_price),
    output: perMillion(p.output_price),
    cacheWrite: perMillion(p.cache_write_price),
    cacheWrite1h: perMillion(p.cache_write_1h_price),
    cacheRead: perMillion(p.cache_read_price),
    imageInput: perMillion(p.image_input_price),
    imageOutput: perMillion(p.image_output_price),
    imageCacheRead: perMillion(p.image_cache_read_price),
    audioInput: perMillion(p.audio_input_price),
    audioOutput: perMillion(p.audio_output_price)
  }
  return Object.values(price).every((value) => value == null) ? null : price
}

/** 标准价各行：有分段按段，没分段就基础价一行；文本 Token 价全缺时没有行 */
function rowsOf(p: UserSupportedModelPricing): TokenSegment[] {
  const base: TokenSegmentPrices = {
    input: p.input_price,
    output: p.output_price,
    cacheWrite: p.cache_write_price,
    cacheWrite1h: p.cache_write_1h_price ?? null,
    cacheRead: p.cache_read_price
  }
  if (TOKEN_ROW_KEYS.every((key) => base[key] == null)) return []
  const segments = tokenSegments(base, p.intervals)
  const rows = segments.length ? segments : [{ min: 0, max: null, prices: base }]
  return rows.map((row) => ({ ...row, prices: scalePrices(row.prices, PER_MILLION) }))
}


function tiersOf(p: UserSupportedModelPricing): CatalogTier[] {
  return (p.intervals ?? [])
    .filter((iv) => iv.tier_label)
    .map((iv) => ({ label: iv.tier_label as string, price: iv.per_request_price ?? null }))
}

/** 分段单价逐项换算；缺项保持 null */
function mapPrices(prices: TokenSegmentPrices, fn: (key: TokenRowKey, value: number) => number): TokenSegmentPrices {
  const out = { ...prices }
  for (const key of TOKEN_ROW_KEYS) {
    const value = prices[key]
    out[key] = value == null ? null : fn(key, value)
  }
  return out
}

/** 分段单价整体乘一个系数（换算单位或乘账户倍率）；缺项保持 null */
export function scalePrices(prices: TokenSegmentPrices, factor: number): TokenSegmentPrices {
  return mapPrices(prices, (_key, value) => value * factor)
}

/** 各行整体乘账户倍率（倍率 = 1 原样返回） */
export function scaleRows(rows: TokenSegment[], factor: number): TokenSegment[] {
  if (factor === 1) return rows
  return rows.map((row) => ({ ...row, prices: scalePrices(row.prices, factor) }))
}

function catalogModel(model: PlazaModel): CatalogModel {
  const billingMode = model.billing_mode || model.pricing?.billing_mode || 'token'
  const p = model.pricing
  const token = billingMode === 'token'
  const rows = p && token ? rowsOf(p) : []
  return {
    id: model.model_id,
    displayName: model.display_name || model.model_id,
    vendor: model.vendor,
    billingMode,
    price: p && token ? priceOf(p) : null,
    unitPrice: token ? null : (p?.per_request_price ?? null),
    tiers: p && !token ? tiersOf(p) : [],
    rows,
    searchPerThousand: p?.search_price_per_call == null ? null : p.search_price_per_call * PER_THOUSAND,
    xPostPerThousand: p?.x_post_price == null ? null : p.x_post_price * PER_THOUSAND,
    xUserPerThousand: p?.x_user_price == null ? null : p.x_user_price * PER_THOUSAND,
    maxReasoningMultiplier: p?.max_reasoning_effort_multiplier ?? null,
    aliases: model.aliases ?? [],
    timePricing: model.time_pricing?.periods.length ? model.time_pricing : null
  }
}

export function buildCatalog(models: PlazaModel[]): CatalogModel[] {
  return models.map(catalogModel).sort((a, b) => a.vendor.localeCompare(b.vendor) || a.id.localeCompare(b.id))
}

/** 搜索匹配模型 id、展示名和别名；厂商 / 计费模式筛选精确匹配（'all' = 不筛） */
export function filterCatalog(entries: CatalogModel[], search: string, vendor: string, billingMode = 'all'): CatalogModel[] {
  const query = search.trim().toLowerCase()
  return entries.filter((entry) => {
    if (vendor !== 'all' && entry.vendor !== vendor) return false
    if (billingMode !== 'all' && entry.billingMode !== billingMode) return false
    if (!query) return true
    return (
      entry.id.toLowerCase().includes(query) ||
      entry.displayName.toLowerCase().includes(query) ||
      entry.aliases.some((alias) => alias.toLowerCase().includes(query))
    )
  })
}

/** 每个厂商的条目数（顺序 = catalogVendors） */
export function countByVendor(entries: CatalogModel[]): Map<string, number> {
  const counts = new Map<string, number>()
  for (const entry of entries) {
    if (!entry.vendor) continue
    counts.set(entry.vendor, (counts.get(entry.vendor) ?? 0) + 1)
  }
  return counts
}

/** 目录里出现过的计费模式，按名排序 */
export function catalogBillingModes(entries: CatalogModel[]): string[] {
  return [...new Set(entries.map((entry) => entry.billingMode))].sort()
}

/** 用户价 = 标价 × 账户倍率；标价缺项的位置保持 null */
export function applyMultiplier(price: CatalogPrice | null, multiplier: number): CatalogPrice | null {
  if (!price) return null
  return Object.fromEntries(
    Object.entries(price).map(([key, value]) => [key, value == null ? null : value * multiplier])
  ) as unknown as CatalogPrice
}

/** 分时倍率的一行说明：09:00–18:00 ×1.5 · 12:00–14:00 ×0.8（时区，仅工作日） */
export function formatTimePricing(timePricing: PlazaTimePricing, weekdaysOnlyLabel: string): string {
  const periods = timePricing.periods.map((p) => `${p.start_time}–${p.end_time} ×${p.multiplier}`).join(' · ')
  const scope = timePricing.weekdays_only ? `${timePricing.timezone}, ${weekdaysOnlyLabel}` : timePricing.timezone
  return `${periods} (${scope})`
}

/** 目录里出现过的厂商，按名排序 */
export function catalogVendors(entries: CatalogModel[]): string[] {
  return [...new Set(entries.map((entry) => entry.vendor).filter(Boolean))].sort()
}

// 厂商一律显示公司名，不写产品名（muqian 2026-09-29：「用公司名称」）
const VENDOR_LABELS: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  gemini: 'Google',
  google: 'Google',
  xai: 'xAI',
  deepseek: 'DeepSeek',
  moonshot: 'Moonshot AI',
  minimax: 'MiniMax',
  zhipu: 'Zhipu AI',
  volcengine: 'Volcengine',
  bedrock: 'AWS',
  // 目录播种带进来的 litellm 供应商名：展示按品牌，归一化留给播种
  'vertex_ai-language-models': 'Google',
  'vertex_ai-embedding-models': 'Google',
  'text-completion-openai': 'OpenAI'
}

/** 厂商标签的展示名：已知的按品牌写法，未知的原样，空的显示破折号 */
export function vendorLabel(vendor: string): string {
  if (!vendor) return '—'
  return VENDOR_LABELS[vendor] ?? vendor
}

/** 价格的展示格式：≥100 取整、≥1 两位小数、更小的保留到 4 位并去掉尾零；null 显示破折号 */
export function formatCatalogPrice(value: number | null | undefined): string {
  if (value == null) return '—'
  if (value >= 100) return `$${value.toFixed(0)}`
  if (value >= 1) return `$${value.toFixed(2)}`
  return `$${Number(value.toFixed(4))}`
}
