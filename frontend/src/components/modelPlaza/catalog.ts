/**
 * 模型页的展示契约：每个精确模型 ID 一行，与分组无关。
 *
 * 官方价来自 /model-plaza 每个模型的 official_pricing（与计费目录同源，USD / token），
 * 这里换算成 USD / 百万 token。不用分组倍率推导"本站价"——统一计价接口上线前该列不渲染。
 */
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'

export interface CatalogOfficialPrice {
  /** USD / 1M tokens；官方目录未覆盖时为 null */
  input: number | null
  output: number | null
  cacheRead: number | null
}

export interface CatalogModel {
  id: string
  /** 出现过该模型的平台（去重、按名排序） */
  platforms: string[]
  /** token / per_request / image / video；缺省视为 token */
  billingMode: string
  official: CatalogOfficialPrice | null
}

const PER_MILLION = 1_000_000

function perMillion(value: number | null | undefined): number | null {
  return value == null ? null : value * PER_MILLION
}

function officialOf(model: PlazaModel): CatalogOfficialPrice | null {
  const p = model.official_pricing
  if (!p) return null
  const price = { input: perMillion(p.input_price), output: perMillion(p.output_price), cacheRead: perMillion(p.cache_read_price) }
  return price.input == null && price.output == null && price.cacheRead == null ? null : price
}

export function buildCatalog(groups: ModelPlazaGroup[]): CatalogModel[] {
  const entries = new Map<string, CatalogModel>()
  for (const group of groups) {
    for (const model of group.models) {
      let entry = entries.get(model.name)
      if (!entry) {
        entry = {
          id: model.name,
          platforms: [],
          billingMode: model.pricing?.billing_mode || 'token',
          official: officialOf(model)
        }
        entries.set(model.name, entry)
      }
      if (model.platform && !entry.platforms.includes(model.platform)) entry.platforms.push(model.platform)
      // 同一模型在不同分组里官方价一致；先出现的分组没带价时用后面的补上
      if (!entry.official) entry.official = officialOf(model)
    }
  }
  return [...entries.values()]
    .map((entry) => ({ ...entry, platforms: [...entry.platforms].sort() }))
    .sort((a, b) => a.id.localeCompare(b.id))
}

export function filterCatalog(entries: CatalogModel[], search: string, platform: string): CatalogModel[] {
  const query = search.trim().toLowerCase()
  return entries.filter(
    (entry) => (!query || entry.id.toLowerCase().includes(query)) && (platform === 'all' || entry.platforms.includes(platform))
  )
}

/** 目录里出现过的平台，按名排序 */
export function catalogPlatforms(entries: CatalogModel[]): string[] {
  return [...new Set(entries.flatMap((entry) => entry.platforms))].sort()
}

/** 官方参考价的展示格式：≥100 取整、≥1 两位小数、更小的保留到 4 位并去掉尾零；null 显示破折号 */
export function formatCatalogPrice(value: number | null | undefined): string {
  if (value == null) return '—'
  if (value >= 100) return `$${value.toFixed(0)}`
  if (value >= 1) return `$${value.toFixed(2)}`
  return `$${Number(value.toFixed(4))}`
}
