import { scaledTierPrices, type PriceRow } from './pricingDraft'

type Translate = (key: string, params?: Record<string, unknown>) => string

/** 按次价显示到 6 位有效数字（存的就是 $/张、$/秒、$/次，不换算） */
export function formatPerRequest(value: number | null | undefined): string {
  return value == null ? '' : String(Number(value.toPrecision(6)))
}

/** 售价 / 上游价那一格下面的说明：各档按官方同比例折算出的价（「各档按官方同比例：1K 0.03、2K 0.04」）；官方没有分档时为空 */
export function perRequestTierHint(t: Translate, base: number | null, official: PriceRow): string {
  const tiers = scaledTierPrices(base, official)
  if (tiers.length === 0) return ''
  const list = tiers.map((tier) => `${tier.label} ${formatPerRequest(tier.price)}`).join(t('admin.pricing.listSeparator'))
  return t('admin.pricing.media.tiersFollow', { tiers: list })
}
