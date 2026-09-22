/**
 * 模型页的展示契约：每个上架的目录条目一行。
 *
 * 标价来自 /model-plaza 每个条目的 pricing（目录基准价，USD / token），这里换算成 USD / 百万 token。
 * 用户价 = 标价 × 用户倍率，倍率由页面按登录态另取，这里不算。
 */
import type { PlazaModel, PlazaTimePricing } from '@/api/modelPlaza'

export interface CatalogPrice {
  /** USD / 1M tokens；目录没给时为 null */
  input: number | null
  output: number | null
  cacheRead: number | null
}

export interface CatalogModel {
  id: string
  displayName: string
  vendor: string
  /** token / per_request / image / video；缺省视为 token */
  billingMode: string
  /** 三项都缺（如纯按次模型）时为 null */
  price: CatalogPrice | null
  aliases: string[]
  /** 分时倍率（有时段才带） */
  timePricing: PlazaTimePricing | null
}

const PER_MILLION = 1_000_000

function perMillion(value: number | null | undefined): number | null {
  return value == null ? null : value * PER_MILLION
}

function priceOf(model: PlazaModel): CatalogPrice | null {
  const p = model.pricing
  if (!p) return null
  const price = { input: perMillion(p.input_price), output: perMillion(p.output_price), cacheRead: perMillion(p.cache_read_price) }
  return price.input == null && price.output == null && price.cacheRead == null ? null : price
}

export function buildCatalog(models: PlazaModel[]): CatalogModel[] {
  return models
    .map(
      (model): CatalogModel => ({
        id: model.model_id,
        displayName: model.display_name || model.model_id,
        vendor: model.vendor,
        billingMode: model.billing_mode || model.pricing?.billing_mode || 'token',
        price: priceOf(model),
        aliases: model.aliases ?? [],
        timePricing: model.time_pricing?.periods.length ? model.time_pricing : null
      })
    )
    .sort((a, b) => a.vendor.localeCompare(b.vendor) || a.id.localeCompare(b.id))
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
  const scale = (value: number | null) => (value == null ? null : value * multiplier)
  return { input: scale(price.input), output: scale(price.output), cacheRead: scale(price.cacheRead) }
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

const VENDOR_LABELS: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  gemini: 'Gemini',
  google: 'Google',
  xai: 'xAI',
  deepseek: 'DeepSeek',
  moonshot: 'Moonshot',
  minimax: 'MiniMax',
  zhipu: 'Zhipu GLM',
  volcengine: 'Volcengine',
  bedrock: 'Bedrock',
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
