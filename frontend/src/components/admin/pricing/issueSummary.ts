import type { PriceField, RowIssues } from './pricingDraft'

type Translate = (key: string, params?: Record<string, unknown>) => string

/** 一行的问题写成一句话，显示在块底部：「fenno · Responses：输出、缓存读还没填；分段 2：……」；tierLabels 是分档的档位名（官方价那一行） */
export function issueSummary(t: Translate, name: string, issues: RowIssues, tierLabels?: string[]): string {
  const parts: string[] = []
  if (issues.missing.length > 0) {
    const fields = issues.missing.map((key: PriceField) => t(`admin.pricing.columns.${key}`)).join(t('admin.pricing.listSeparator'))
    parts.push(t('admin.pricing.issues.missing', { fields }))
  }
  if (issues.invalid.length > 0) {
    const fields = issues.invalid.map((key: PriceField) => t(`admin.pricing.columns.${key}`)).join(t('admin.pricing.listSeparator'))
    parts.push(t('admin.pricing.issues.invalid', { fields }))
  }
  if (issues.upstreamModelInvalid) parts.push(t('admin.pricing.issues.upstreamModel'))
  issues.segments.forEach((error, index) => {
    if (error) parts.push(t('admin.pricing.issues.segment', { index: index + 2, error: t(`admin.modelCatalog.segments.errors.${error}`) }))
  })
  issues.peak?.forEach((error, index) => {
    if (error) parts.push(t('admin.pricing.issues.peak', { index: index + 1, error: t(`admin.pricing.peak.errors.${error}`) }))
  })
  if (issues.peakDatesInvalid) parts.push(t('admin.pricing.issues.peakDates'))
  if (issues.reasoningInvalid) parts.push(t('admin.pricing.issues.reasoning'))
  issues.tiers?.forEach((error, index) => {
    if (error) parts.push(t('admin.pricing.issues.tier', { label: tierLabels?.[index] ?? index + 1, error: t(`admin.pricing.media.tierErrors.${error}`) }))
  })
  return `${name}${t('common.labelSeparator')}${parts.join(t('admin.pricing.issueSeparator'))}`
}
