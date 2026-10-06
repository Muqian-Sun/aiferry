/**
 * 「检测上游」查到的模型名单对照模型目录（muqian 2026-10-03）。
 *
 * 上游的 /models 只当参考：实测 agentrouter 承接的两个模型、qiniu 承接的 gpt-5.6-luna 都不在它们的列表里；
 * 名字也会差一点（qiniu 的 deepseek-v4-1-flash ↔ 目录 deepseek-v4.1-flash）。所以先认逐字相同的标识，
 * 再认「去掉大小写和标点后相同」的相近名——相近名承接时把上游的名字填进「上游模型名」。
 * 聚合平台写法「anthropic/claude-…」去掉斜杠前的厂商段再认一次（与后端 lookupCatalogEntryForUpstreamModel 同口径）。
 *
 * 目录里没有的（muqian 2026-10-06）：先看是不是官方模型 ID（联网的 LiteLLM 公开价格表）——是就加进目录，
 * 不是就映射到目录里的某个模型（这个渠道的上游模型名），映射给一个建议、由管理员确认。
 */
import type { ModelCatalogEntry, OfficialModelLookupResult } from '@/api/admin/modelCatalog'

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

/** 去掉聚合平台写法的厂商段：「anthropic/claude-sonnet-4.5」→「claude-sonnet-4.5」 */
function withoutVendorPrefix(name: string): string {
  const slash = name.lastIndexOf('/')
  return slash >= 0 ? name.slice(slash + 1) : name
}

interface CatalogIndex {
  byId: Map<string, ModelCatalogEntry>
  byLower: Map<string, ModelCatalogEntry>
  // 两个条目的比较键相同时记成 null：不认相近名，免得猜错
  byLoose: Map<string, ModelCatalogEntry | null>
}

function indexCatalog(catalog: ModelCatalogEntry[]): CatalogIndex {
  const byLoose = new Map<string, ModelCatalogEntry | null>()
  for (const entry of catalog) {
    const key = looseKey(entry.model_id)
    byLoose.set(key, byLoose.has(key) ? null : entry)
  }
  return {
    byId: new Map(catalog.map((entry) => [entry.model_id, entry])),
    byLower: new Map(catalog.map((entry) => [entry.model_id.toLowerCase(), entry])),
    byLoose
  }
}

function findInCatalog(index: CatalogIndex, name: string): ModelCatalogEntry | null {
  for (const candidate of new Set([name, withoutVendorPrefix(name)])) {
    const entry = index.byId.get(candidate) ?? index.byLower.get(candidate.toLowerCase()) ?? index.byLoose.get(looseKey(candidate)) ?? null
    if (entry) return entry
  }
  return null
}

export function classifyUpstreamModels(names: string[], catalog: ModelCatalogEntry[]): DetectedModels {
  const index = indexCatalog(catalog)
  const result: DetectedModels = { listed: [], unlisted: [], missing: [] }
  const seen = new Set<number>()
  for (const raw of names) {
    const name = raw.trim()
    if (!name) continue
    const entry = findInCatalog(index, name)
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

/**
 * 非官方名字映射到哪个目录模型的建议：比较键是这个名字前缀的目录模型里取最长的
 * （gpt-5.3-codex-spark → gpt-5.3-codex，claude-sonnet-4-5-thinking → claude-sonnet-4-5）。
 * 只是建议，由管理员确认；一个都不是前缀就不给。
 */
export function suggestCatalogEntry(name: string, catalog: ModelCatalogEntry[]): ModelCatalogEntry | null {
  const key = looseKey(withoutVendorPrefix(name))
  let best: ModelCatalogEntry | null = null
  let bestLength = 0
  for (const entry of catalog) {
    const entryKey = looseKey(entry.model_id)
    if (entryKey.length > bestLength && key.startsWith(entryKey)) {
      best = entry
      bestLength = entryKey.length
    }
  }
  return best
}

export interface OfficialCandidate {
  name: string
  /** 读得出官方价：可以直接加进目录；读不出（分档价等）要在新建弹窗里填价 */
  priced: boolean
  /** 带官方价的条目（priced 为 false 时只有厂商） */
  entry: ModelCatalogEntry | null
}

export interface UnofficialCandidate {
  name: string
  /** 映射建议 */
  suggestion: ModelCatalogEntry | null
}

/** 目录里没有的模型按「是不是官方 ID」分开；联网名单拉不到时都归到 undetermined */
export interface MissingModelPlan {
  official: OfficialCandidate[]
  unofficial: UnofficialCandidate[]
  undetermined: UnofficialCandidate[]
}

export function planMissingModels(
  missing: string[],
  lookup: OfficialModelLookupResult | null,
  catalog: ModelCatalogEntry[]
): MissingModelPlan {
  const plan: MissingModelPlan = { official: [], unofficial: [], undetermined: [] }
  const byName = new Map((lookup?.models ?? []).map((match) => [match.model_id.toLowerCase(), match]))
  for (const name of missing) {
    const match = byName.get(name.toLowerCase())
    if (lookup?.available && match?.official) {
      plan.official.push({ name, priced: match.priced, entry: match.entry ?? null })
      continue
    }
    const candidate = { name, suggestion: suggestCatalogEntry(name, catalog) }
    if (lookup?.available) plan.unofficial.push(candidate)
    else plan.undetermined.push(candidate)
  }
  return plan
}
