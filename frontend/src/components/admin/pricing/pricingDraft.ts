/**
 * 价格页的草稿：每一块（按模型 / 按渠道）各自一份，改动只在块内，点保存才整块提交。
 *
 * 一行价 = 五项 token 价（$/token）+ 按 Token 分段（与模型编辑页同一套分段行：基础价是第一段，
 * 这里只放「超过某个 Token 数之后」的各段）。上游价的必填规则与后端 ModelCatalogBinding.ValidateAgainst 一致：
 * 输入 / 输出必填；官方价有的缓存项（缓存读、缓存写 5 分钟 / 1 小时）上游价也必填。
 */

import type { PricingEntry, PricingPrices } from '@/api/admin/pricing'
import {
  numberOrNull,
  tokenSegmentErrors,
  tokenSegmentsFromIntervals,
  tokenSegmentsToIntervals,
  type TokenSegmentError,
  type TokenSegmentForm
} from '@/components/admin/catalog/entryRequest'

/** 表格里的列顺序 */
export const PRICE_KEYS = ['input_price', 'output_price', 'cache_read_price', 'cache_write_price', 'cache_write_1h_price'] as const
export type PriceKey = (typeof PRICE_KEYS)[number]

/** 官方价有这几项时上游价也必须填 */
const CACHE_KEYS: PriceKey[] = ['cache_read_price', 'cache_write_price', 'cache_write_1h_price']

export type PriceRow = Record<PriceKey, number | null> & { segments: TokenSegmentForm[] }

export function priceRowFrom(prices: Pick<PricingPrices, PriceKey | 'intervals'>): PriceRow {
  return {
    input_price: prices.input_price ?? null,
    output_price: prices.output_price ?? null,
    cache_read_price: prices.cache_read_price ?? null,
    cache_write_price: prices.cache_write_price ?? null,
    cache_write_1h_price: prices.cache_write_1h_price ?? null,
    segments: tokenSegmentsFromIntervals(prices.intervals, prices)
  }
}

export function emptyPriceRow(): PriceRow {
  return {
    input_price: null,
    output_price: null,
    cache_read_price: null,
    cache_write_price: null,
    cache_write_1h_price: null,
    segments: []
  }
}

export function clonePriceRow(row: PriceRow): PriceRow {
  return { ...row, segments: row.segments.map((segment) => ({ ...segment })) }
}

/** 提交用（调用前先确认 rowIssues 没有问题） */
export function priceRowToRequest(row: PriceRow): PricingPrices {
  return {
    input_price: numberOrNull(row.input_price),
    output_price: numberOrNull(row.output_price),
    cache_read_price: numberOrNull(row.cache_read_price),
    cache_write_price: numberOrNull(row.cache_write_price),
    cache_write_1h_price: numberOrNull(row.cache_write_1h_price),
    intervals: tokenSegmentsToIntervals(row.segments)
  }
}

export interface RowIssues {
  /** 必填却没填的价 */
  missing: PriceKey[]
  /** 每段的问题（null = 没问题），与 segments 一一对应 */
  segments: Array<TokenSegmentError | null>
}

export function hasRowIssues(issues: RowIssues): boolean {
  return issues.missing.length > 0 || issues.segments.some((error) => error != null)
}

/** 官方价：输入 / 输出必填 */
export function officialIssues(row: PriceRow): RowIssues {
  return {
    missing: (['input_price', 'output_price'] as PriceKey[]).filter((key) => row[key] == null),
    segments: tokenSegmentErrors(row.segments)
  }
}

/** 上游价：输入 / 输出必填，官方价（official）有的缓存项也必填 */
export function upstreamIssues(row: PriceRow, official: Record<PriceKey, number | null | undefined>): RowIssues {
  const required: PriceKey[] = ['input_price', 'output_price', ...CACHE_KEYS.filter((key) => official[key] != null)]
  return {
    missing: required.filter((key) => row[key] == null),
    segments: tokenSegmentErrors(row.segments)
  }
}

function sameNumber(a: number | null, b: number | null): boolean {
  if (a == null || b == null) return a == null && b == null
  return Math.abs(a - b) <= Math.abs(b) * 1e-9
}

function sameSegments(a: TokenSegmentForm[], b: TokenSegmentForm[]): boolean {
  return (
    a.length === b.length &&
    a.every((row, i) => row.above.trim() === b[i].above.trim() && PRICE_KEYS.every((key) => sameNumber(row[key], b[i][key])))
  )
}

/** 两行价之间改了几处：每项价算一处，分段有任何不同算一处 */
export function priceRowChanges(current: PriceRow, initial: PriceRow): number {
  const changed = PRICE_KEYS.filter((key) => !sameNumber(current[key], initial[key])).length
  return changed + (sameSegments(current.segments, initial.segments) ? 0 : 1)
}

export interface KeyedRow {
  /** 按模型的块里是渠道 ID，按渠道的块里是模型条目 ID */
  id: number
  prices: PriceRow
}

/** 一块的承接行改了几处：新加 / 移除一行各算一处，同一行按 priceRowChanges 计 */
export function keyedRowsChanges(current: KeyedRow[], initial: KeyedRow[]): number {
  const before = new Map(initial.map((row) => [row.id, row.prices]))
  const after = new Set(current.map((row) => row.id))
  let count = initial.filter((row) => !after.has(row.id)).length
  for (const row of current) {
    const original = before.get(row.id)
    count += original ? priceRowChanges(row.prices, original) : 1
  }
  return count
}

/** 这一行的价与服务端保存的一致（毛利只对没改过的行显示，改过的要保存后由后端重算） */
export function keyedRowUnchanged(row: KeyedRow, initial: KeyedRow[]): boolean {
  const original = initial.find((item) => item.id === row.id)
  return original != null && priceRowChanges(row.prices, original.prices) === 0
}

/** 毛利 = 1 − 上游成本比 ÷ 默认售价倍率；算不出时为 null */
export function marginOf(costRatio: number | null | undefined, defaultUserRate: number): number | null {
  if (costRatio == null || !(defaultUserRate > 0)) return null
  return 1 - costRatio / defaultUserRate
}

/** 利润门会跳过这条承接：最低毛利率 > 0 且毛利低于它 */
export function belowMinMargin(margin: number | null, minMargin: number): boolean {
  return minMargin > 0 && margin != null && margin < minMargin
}

/** 模型的「问题」：官方价没填齐、上架了却没有渠道、有渠道毛利低于门槛 */
export function entryHasProblem(entry: PricingEntry, defaultUserRate: number, minMargin: number): boolean {
  if (entry.input_price == null || entry.output_price == null) return true
  if (entry.status === 'listed' && entry.bindings.length === 0) return true
  return entry.bindings.some((binding) => belowMinMargin(marginOf(binding.cost_ratio, defaultUserRate), minMargin))
}

// ---- 块的草稿：由页面统一保管（筛选、翻页、切视图不丢），块组件只读写它

/** 一块的草稿：initial = 服务端数据（或刚保存成功的内容），draft = 正在改的 */
export interface BlockState<D> {
  initial: D
  draft: D
}

/** 按模型的一块：官方价 + 每个承接渠道一行（id = 渠道 ID），按渠道优先级排 */
export interface ModelDraft {
  official: PriceRow
  rows: KeyedRow[]
}

/** 按渠道的一块：每个承接的模型一行（id = 模型条目 ID），按模型 ID 排 */
export interface ChannelDraft {
  rows: KeyedRow[]
}

export function cloneKeyedRows(rows: KeyedRow[]): KeyedRow[] {
  return rows.map((row) => ({ id: row.id, prices: clonePriceRow(row.prices) }))
}

export function modelDraftFrom(entry: PricingEntry, accountOrder: (accountId: number) => number): ModelDraft {
  return {
    official: priceRowFrom(entry),
    rows: [...entry.bindings]
      .sort((a, b) => accountOrder(a.account_id) - accountOrder(b.account_id))
      .map((binding) => ({ id: binding.account_id, prices: priceRowFrom(binding) }))
  }
}

export function cloneModelDraft(draft: ModelDraft): ModelDraft {
  return { official: clonePriceRow(draft.official), rows: cloneKeyedRows(draft.rows) }
}

export function modelDraftChanges(state: BlockState<ModelDraft>): number {
  return priceRowChanges(state.draft.official, state.initial.official) + keyedRowsChanges(state.draft.rows, state.initial.rows)
}

export function channelDraftFrom(accountId: number, entries: PricingEntry[]): ChannelDraft {
  const rows: Array<KeyedRow & { modelId: string }> = []
  for (const entry of entries) {
    const binding = entry.bindings.find((item) => item.account_id === accountId)
    if (binding) rows.push({ id: entry.id, modelId: entry.model_id, prices: priceRowFrom(binding) })
  }
  rows.sort((a, b) => a.modelId.localeCompare(b.modelId))
  return { rows: rows.map(({ id, prices }) => ({ id, prices })) }
}

export function cloneChannelDraft(draft: ChannelDraft): ChannelDraft {
  return { rows: cloneKeyedRows(draft.rows) }
}

export function channelDraftChanges(state: BlockState<ChannelDraft>): number {
  return keyedRowsChanges(state.draft.rows, state.initial.rows)
}
