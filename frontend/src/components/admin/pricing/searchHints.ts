import type { PricingSearchDefaults } from '@/api/admin/pricing'
import { effectiveOfficialSearch, type SearchKey } from './pricingDraft'

type Translate = (key: string, params?: Record<string, unknown>) => string

/** 搜索价按「每千次 / 千条 / 千个」显示（存的是 $/次、$/条） */
function perThousand(value: number): string {
  return String(Number((value * 1000).toPrecision(6)))
}

/** 官方价那一行的占位：空着按厂商公开价收 */
export function officialSearchPlaceholders(t: Translate, keys: SearchKey[], defaults: PricingSearchDefaults | null | undefined): Partial<Record<SearchKey, string>> {
  const out: Partial<Record<SearchKey, string>> = {}
  for (const key of keys) {
    const value = defaults?.[key]
    if (value != null) out[key] = t('admin.pricing.search.defaultPlaceholder', { price: perThousand(value) })
  }
  return out
}

/** 承接行搜索价下面的参考：官方设了的写官方价，没设的写厂商公开价 */
export function upstreamSearchHints(
  t: Translate,
  keys: SearchKey[],
  official: Partial<Record<SearchKey, number | null | undefined>>,
  defaults: PricingSearchDefaults | null | undefined
): Partial<Record<SearchKey, string>> {
  const effective = effectiveOfficialSearch({ search_price_per_call: official.search_price_per_call, x_post_price: official.x_post_price, x_user_price: official.x_user_price }, defaults)
  const out: Partial<Record<SearchKey, string>> = {}
  for (const key of keys) {
    const value = effective[key]
    if (value == null) continue
    out[key] = official[key] != null
      ? t('admin.pricing.search.officialRef', { price: perThousand(value) })
      : t('admin.pricing.search.officialDefaultRef', { price: perThousand(value) })
  }
  return out
}

/** 承接行搜索价的占位：官方没设的项可不填（按官方价记成本） */
export function upstreamSearchPlaceholders(t: Translate, keys: SearchKey[], official: Partial<Record<SearchKey, number | null | undefined>>): Partial<Record<SearchKey, string>> {
  const out: Partial<Record<SearchKey, string>> = {}
  for (const key of keys) {
    if (official[key] == null) out[key] = t('admin.pricing.search.optional')
  }
  return out
}
