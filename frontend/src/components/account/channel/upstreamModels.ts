/**
 * 「检测上游」查到的模型名单对照模型目录（muqian 2026-10-03）。
 *
 * 上游的 /models 只当参考：实测 agentrouter 承接的两个模型、qiniu 承接的 gpt-5.6-luna 都不在它们的列表里；
 * 名字也会差一点（qiniu 的 deepseek-v4-1-flash ↔ 目录 deepseek-v4.1-flash）。所以先认逐字相同的标识，
 * 再认「去掉大小写和标点后相同」的相近名——相近名承接时把上游的名字填进「上游模型名」。
 */
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

export interface DetectedMatch {
  entry: ModelCatalogEntry
  /** 上游列表里的名字；与 entry.model_id 不同时就是这条承接的上游模型名 */
  upstreamName: string
}

export interface DetectedModels {
  /** 目录里已上架 */
  listed: DetectedMatch[]
  /** 目录里有、没上架 */
  unlisted: DetectedMatch[]
  /** 目录里没有 */
  missing: string[]
}

/** 相近名的比较键：只留小写字母和数字 */
function looseKey(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]/g, '')
}

export function classifyUpstreamModels(names: string[], catalog: ModelCatalogEntry[]): DetectedModels {
  const byId = new Map(catalog.map((entry) => [entry.model_id, entry]))
  const byLower = new Map(catalog.map((entry) => [entry.model_id.toLowerCase(), entry]))
  // 两个条目的比较键相同时记成 null：不认相近名，免得猜错
  const byLoose = new Map<string, ModelCatalogEntry | null>()
  for (const entry of catalog) {
    const key = looseKey(entry.model_id)
    byLoose.set(key, byLoose.has(key) ? null : entry)
  }

  const result: DetectedModels = { listed: [], unlisted: [], missing: [] }
  const seen = new Set<number>()
  for (const raw of names) {
    const name = raw.trim()
    if (!name) continue
    const entry = byId.get(name) ?? byLower.get(name.toLowerCase()) ?? byLoose.get(looseKey(name)) ?? null
    if (!entry) {
      if (!result.missing.includes(name)) result.missing.push(name)
      continue
    }
    if (seen.has(entry.id)) continue
    seen.add(entry.id)
    const match = { entry, upstreamName: name }
    if (entry.status === 'listed') result.listed.push(match)
    else result.unlisted.push(match)
  }
  return result
}

/** 承接行的「上游模型名」：与目录标识逐字相同就留空（= 同名） */
export function upstreamModelFor(match: DetectedMatch): string {
  return match.upstreamName === match.entry.model_id ? '' : match.upstreamName
}
