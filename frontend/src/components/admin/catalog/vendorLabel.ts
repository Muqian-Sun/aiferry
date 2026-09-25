/**
 * 目录条目厂商的展示名。原始厂商串是价格文件带进来的 LiteLLM 标识（anthropic、vertex_ai-language-models…），不直接上屏：
 * 认得出厂商族的（列表接口的 vendor_platform，与渠道平台同一套标识）用渠道页同一套平台名，同一族归成一个名字；
 * 认不出的按品牌写法（未知的原样）；没有厂商为空串。
 */
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import { vendorLabel } from '@/components/modelPlaza/catalog'
import { platformLabel } from '@/utils/platformLabel'

export function catalogVendorLabel(entry: Pick<ModelCatalogEntry, 'vendor' | 'vendor_platform'>): string {
  if (entry.vendor_platform) return platformLabel(entry.vendor_platform)
  return entry.vendor ? vendorLabel(entry.vendor) : ''
}
