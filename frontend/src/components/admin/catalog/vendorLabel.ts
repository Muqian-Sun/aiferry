/**
 * 目录条目厂商的展示名。原始厂商串是价格文件带进来的 LiteLLM 标识（anthropic、vertex_ai-language-models…），不直接上屏：
 * 认得出厂商族的（列表接口的 vendor_platform，与渠道平台同一套标识）按厂商族归成一个名字；
 * 都写公司名（Google、xAI、Moonshot AI），不写产品名（Gemini、Grok、Kimi）；认不出的原样；没有厂商为空串。
 */
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import { vendorLabel } from '@/components/modelPlaza/catalog'

export function catalogVendorLabel(entry: Pick<ModelCatalogEntry, 'vendor' | 'vendor_platform'>): string {
  const vendor = entry.vendor_platform || entry.vendor
  return vendor ? vendorLabel(vendor) : ''
}

/** 新建 / 编辑弹窗厂商下拉的一项：显示名 + 写进条目的厂商串 + 这一族在目录里出现过的全部厂商串 */
export interface CatalogVendorChoice {
  label: string
  value: string
  values: string[]
}

/**
 * 厂商下拉的选项：按列表同一个展示名分组，同一厂商族的几个原始串（openai / text-completion-openai、
 * gemini / vertex_ai-*…）只出一项，不把原始串露给人看；写进条目的值取这一族里条目最多的那个串。
 */
export function catalogVendorChoices(entries: Array<Pick<ModelCatalogEntry, 'vendor' | 'vendor_platform'>>): CatalogVendorChoice[] {
  const byLabel = new Map<string, Map<string, number>>()
  for (const entry of entries) {
    if (!entry.vendor) continue
    const label = catalogVendorLabel(entry)
    const counts = byLabel.get(label) ?? new Map<string, number>()
    counts.set(entry.vendor, (counts.get(entry.vendor) ?? 0) + 1)
    byLabel.set(label, counts)
  }
  return [...byLabel]
    .map(([label, counts]) => {
      const ranked = [...counts].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
      return { label, value: ranked[0][0], values: ranked.map(([vendor]) => vendor) }
    })
    .sort((a, b) => a.label.localeCompare(b.label))
}
