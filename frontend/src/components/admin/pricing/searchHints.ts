import type { PricingSearchDefaults } from '@/api/admin/pricing'
import type { SearchKey } from './pricingDraft'

type Translate = (key: string, params?: Record<string, unknown>) => string

/** 搜索价按「每千次 / 千条 / 千个」显示（存的是 $/次、$/条） */
function perThousand(value: number): string {
  return String(Number((value * 1000).toPrecision(6)))
}

/** 官方价那一行的占位：写厂商公开价作参考（播种时已写进官方价；空着不收搜索费） */
export function officialSearchPlaceholders(t: Translate, keys: SearchKey[], defaults: PricingSearchDefaults | null | undefined): Partial<Record<SearchKey, string>> {
  const out: Partial<Record<SearchKey, string>> = {}
  for (const key of keys) {
    const value = defaults?.[key]
    if (value != null) out[key] = t('admin.pricing.search.defaultPlaceholder', { price: perThousand(value) })
  }
  return out
}

/** 承接行搜索价下面的参考：官方设了的写官方价，没设的写「官方不收」（计费只认目录） */
export function upstreamSearchHints(t: Translate, keys: SearchKey[], official: Partial<Record<SearchKey, number | null | undefined>>): Partial<Record<SearchKey, string>> {
  const out: Partial<Record<SearchKey, string>> = {}
  for (const key of keys) {
    const value = official[key]
    out[key] = value != null ? t('admin.pricing.search.officialRef', { price: perThousand(value) }) : t('admin.pricing.search.officialNone')
  }
  return out
}

/** 成本价搜索价的占位：空着 = 上游不收（muqian 2026-10-06） */
export function upstreamSearchPlaceholders(t: Translate, keys: SearchKey[]): Partial<Record<SearchKey, string>> {
  return Object.fromEntries(keys.map((key) => [key, t('admin.pricing.search.free')]))
}
