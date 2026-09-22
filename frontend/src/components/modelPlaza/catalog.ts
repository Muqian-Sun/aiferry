/**
 * 模型页的展示契约：每个上架的目录条目一行。
 *
 * 标价来自 /model-plaza 每个条目的 pricing（目录基准价，USD / token），这里换算成 USD / 百万 token。
 * 用户价 = 标价 × 用户倍率，倍率由页面按登录态另取，这里不算。
 */
import type { PlazaModel } from '@/api/modelPlaza'

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
  /** 是否配置了分时倍率 */
  hasTimePricing: boolean
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
        hasTimePricing: (model.time_pricing?.periods.length ?? 0) > 0
      })
    )
    .sort((a, b) => a.vendor.localeCompare(b.vendor) || a.id.localeCompare(b.id))
}

/** 搜索匹配模型 id、展示名和别名；厂商筛选精确匹配 */
export function filterCatalog(entries: CatalogModel[], search: string, vendor: string): CatalogModel[] {
  const query = search.trim().toLowerCase()
  return entries.filter((entry) => {
    if (vendor !== 'all' && entry.vendor !== vendor) return false
    if (!query) return true
    return (
      entry.id.toLowerCase().includes(query) ||
      entry.displayName.toLowerCase().includes(query) ||
      entry.aliases.some((alias) => alias.toLowerCase().includes(query))
    )
  })
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
