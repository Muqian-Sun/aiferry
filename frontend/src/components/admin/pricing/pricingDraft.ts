/**
 * 价格页的草稿：每一块（按模型 / 按渠道）各自一份，改动只在块内，点保存才整块提交。
 *
 * 一行价 = 五项 token 价（$/token）+ 按 Token 分段（与模型编辑页同一套分段行：基础价是第一段，
 * 这里只放「超过某个 Token 数之后」的各段）+ 联网搜索价（$/次、$/条）。上游价的必填规则与后端
 * ModelCatalogBinding.ValidateAgainst 一致：输入 / 输出必填；官方价有的缓存项（缓存读、缓存写 5 分钟 / 1 小时）
 * 与官方价显式设了的搜索价，上游价也必填。
 */

import type { PricingAccount, PricingBinding, PricingEntry, PricingPrices, PricingSalePrices, PricingSearchDefaults, TimePricing } from '@/api/admin/pricing'
import {
  numberOrNull,
  parseTokenThreshold,
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

/** 联网搜索价（$/次、$/条）：每次 web 搜索；xAI 另有 X 帖子、X 主页（按取回条目收） */
export const SEARCH_KEYS = ['search_price_per_call', 'x_post_price', 'x_user_price'] as const
export type SearchKey = (typeof SEARCH_KEYS)[number]
export type PriceField = PriceKey | SearchKey

export type PriceRow = Record<PriceKey, number | null> & Record<SearchKey, number | null> & { segments: TokenSegmentForm[] }

/** 这个模型能填的搜索价：厂商没有官方搜索工具（search_defaults 为 null）时一项都没有；X 帖子 / 主页只有 xAI 有 */
export function searchKeysOf(defaults: PricingSearchDefaults | null | undefined): SearchKey[] {
  if (!defaults) return []
  return defaults.x_post_price != null ? [...SEARCH_KEYS] : ['search_price_per_call']
}

export function priceRowFrom(prices: Pick<PricingPrices, PriceKey | 'intervals'> & Partial<Record<SearchKey, number | null>>): PriceRow {
  return {
    input_price: prices.input_price ?? null,
    output_price: prices.output_price ?? null,
    cache_read_price: prices.cache_read_price ?? null,
    cache_write_price: prices.cache_write_price ?? null,
    cache_write_1h_price: prices.cache_write_1h_price ?? null,
    search_price_per_call: prices.search_price_per_call ?? null,
    x_post_price: prices.x_post_price ?? null,
    x_user_price: prices.x_user_price ?? null,
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
    search_price_per_call: null,
    x_post_price: null,
    x_user_price: null,
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
    intervals: tokenSegmentsToIntervals(row.segments),
    search_price_per_call: numberOrNull(row.search_price_per_call),
    x_post_price: numberOrNull(row.x_post_price),
    x_user_price: numberOrNull(row.x_user_price)
  }
}

export interface RowIssues {
  /** 必填却没填的价 */
  missing: PriceField[]
  /** 填了但格式不对的价（负数、不是数字：输入框回写 NaN） */
  invalid: PriceField[]
  /** 每段的问题（null = 没问题），与 segments 一一对应 */
  segments: Array<TokenSegmentError | null>
  /** 上游模型名不是一个具体的名字（带通配或空白） */
  upstreamModelInvalid?: boolean
  /** 上游忙闲时每个时段的问题（null = 没问题），与 peak.periods 一一对应 */
  peak?: Array<PeakPeriodError | null>
}

export function hasRowIssues(issues: RowIssues): boolean {
  return (
    issues.missing.length > 0 ||
    issues.invalid.length > 0 ||
    issues.segments.some((error) => error != null) ||
    issues.upstreamModelInvalid === true ||
    (issues.peak ?? []).some((error) => error != null)
  )
}

/** 格式不对的价：输入框把负数、不是数字的输入回写成 NaN */
function invalidKeys(row: PriceRow): PriceField[] {
  return [...PRICE_KEYS, ...SEARCH_KEYS].filter((key) => Number.isNaN(row[key]))
}

/** 上游模型名只能是一个具体的名字：不带 *、不含空白（与后端 ValidateAgainst 同口径；首尾空白保存时去掉） */
export function upstreamModelInvalid(name: string): boolean {
  return /[*\s]/.test(name.trim())
}

/** 官方价：输入 / 输出必填 */
export function officialIssues(row: PriceRow): RowIssues {
  return {
    missing: (['input_price', 'output_price'] as PriceKey[]).filter((key) => row[key] == null),
    invalid: invalidKeys(row),
    segments: tokenSegmentErrors(row.segments)
  }
}

/** 上游价：输入 / 输出必填，官方价（official）有的缓存项、显式设了的搜索价也必填 */
export function upstreamIssues(row: PriceRow, official: OfficialRef): RowIssues {
  const required: PriceField[] = [
    'input_price',
    'output_price',
    ...CACHE_KEYS.filter((key) => official[key] != null),
    ...SEARCH_KEYS.filter((key) => official[key] != null)
  ]
  return {
    missing: required.filter((key) => row[key] == null),
    invalid: invalidKeys(row),
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

/** 两行价之间改了几处：每项价（含搜索价）算一处，分段有任何不同算一处 */
export function priceRowChanges(current: PriceRow, initial: PriceRow): number {
  const changed = [...PRICE_KEYS, ...SEARCH_KEYS].filter((key) => !sameNumber(current[key], initial[key])).length
  return changed + (sameSegments(current.segments, initial.segments) ? 0 : 1)
}

export interface KeyedRow {
  /** 按模型的块里是渠道 ID，按渠道的块里是模型条目 ID */
  id: number
  /** 这个渠道给这个模型用的上游模型名，空 = 与目录模型标识同名 */
  upstreamModel: string
  prices: PriceRow
  /** 上游忙闲时；null = 上游不分忙闲时 */
  peak: PeakForm | null
}

/** 上游价必填规则用的官方价：五项 token 价 + 显式设了的搜索价（没设 = null，上游可不填） */
export type OfficialRef = Record<PriceKey, number | null | undefined> & Partial<Record<SearchKey, number | null | undefined>>

/** 承接行的问题：上游价 + 上游模型名 + 上游忙闲时 */
export function bindingRowIssues(row: KeyedRow, official: OfficialRef): RowIssues {
  return {
    ...upstreamIssues(row.prices, official),
    upstreamModelInvalid: upstreamModelInvalid(row.upstreamModel),
    peak: peakErrors(row.peak)
  }
}

/** 一块的承接行改了几处：新加 / 移除一行各算一处，同一行按 priceRowChanges 计，上游模型名、忙闲时改了各算一处 */
export function keyedRowsChanges(current: KeyedRow[], initial: KeyedRow[]): number {
  const before = new Map(initial.map((row) => [row.id, row]))
  const after = new Set(current.map((row) => row.id))
  let count = initial.filter((row) => !after.has(row.id)).length
  for (const row of current) {
    const original = before.get(row.id)
    if (!original) {
      count += 1
      continue
    }
    count += priceRowChanges(row.prices, original.prices)
    if (row.upstreamModel.trim() !== original.upstreamModel.trim()) count += 1
    if (!samePeak(row.peak, original.peak)) count += 1
  }
  return count
}

/** 这一行的价与服务端保存的一致（毛利只对没改过的行显示，改过的要保存后由后端重算） */
export function keyedRowUnchanged(row: KeyedRow, initial: KeyedRow[]): boolean {
  const original = initial.find((item) => item.id === row.id)
  return original != null && priceRowChanges(row.prices, original.prices) === 0 && samePeak(row.peak, original.peak)
}

/** 毛利 = 1 − 上游成本比 ÷ 默认售价比例（上游成本比由后端按售价口径算）；算不出时为 null */
export function marginOf(costRatio: number | null | undefined, defaultSaleRatio: number): number | null {
  if (costRatio == null || !(defaultSaleRatio > 0)) return null
  return 1 - costRatio / defaultSaleRatio
}

/** 利润门会跳过这条承接：最低毛利率 > 0 且毛利低于它 */
export function belowMinMargin(margin: number | null, minMargin: number): boolean {
  return minMargin > 0 && margin != null && margin < minMargin
}

/** 模型的「问题」：官方价没填齐、上架了却没有渠道、有渠道毛利（平时或忙时）低于门槛 */
export function entryHasProblem(entry: PricingEntry, defaultSaleRatio: number, minMargin: number): boolean {
  if (entry.input_price == null || entry.output_price == null) return true
  if (entry.status === 'listed' && entry.bindings.length === 0) return true
  return entry.bindings.some(
    (binding) =>
      belowMinMargin(marginOf(binding.cost_ratio, defaultSaleRatio), minMargin) ||
      belowMinMargin(marginOf(binding.peak_cost_ratio, defaultSaleRatio), minMargin)
  )
}

// ---- 块的草稿：由页面统一保管（筛选、翻页、切视图不丢），块组件只读写它

/** 一块的草稿：initial = 服务端数据（或刚保存成功的内容），draft = 正在改的 */
export interface BlockState<D> {
  initial: D
  draft: D
}

/** 按模型的一块：官方价（含官方忙闲时）、售价 + 每个承接渠道一行（id = 渠道 ID），按渠道优先级排 */
export interface ModelDraft {
  official: PriceRow
  /** 官方忙闲时（目录条目的分时）；null = 不分忙闲时 */
  officialPeak: PeakForm | null
  sale: SaleRow
  rows: KeyedRow[]
}

/** 按渠道的一块：每个承接的模型一行（id = 模型条目 ID），按模型 ID 排 */
export interface ChannelDraft {
  rows: KeyedRow[]
}

export function cloneKeyedRows(rows: KeyedRow[]): KeyedRow[] {
  return rows.map((row) => ({ id: row.id, upstreamModel: row.upstreamModel, prices: clonePriceRow(row.prices), peak: clonePeakForm(row.peak) }))
}

export function modelDraftFrom(entry: PricingEntry, accountOrder: (accountId: number) => number): ModelDraft {
  return {
    official: priceRowFrom(entry),
    officialPeak: peakFormFrom(entry.time_pricing),
    sale: saleRowFrom(entry.sale_prices),
    rows: [...entry.bindings]
      .sort((a, b) => accountOrder(a.account_id) - accountOrder(b.account_id))
      .map((binding) => ({
        id: binding.account_id,
        upstreamModel: binding.upstream_model ?? '',
        prices: priceRowFrom(binding),
        peak: peakFormFrom(binding.time_pricing)
      }))
  }
}

export function cloneModelDraft(draft: ModelDraft): ModelDraft {
  return {
    official: clonePriceRow(draft.official),
    officialPeak: clonePeakForm(draft.officialPeak),
    sale: cloneSaleRow(draft.sale),
    rows: cloneKeyedRows(draft.rows)
  }
}

export function modelDraftChanges(state: BlockState<ModelDraft>): number {
  return (
    priceRowChanges(state.draft.official, state.initial.official) +
    (samePeak(state.draft.officialPeak, state.initial.officialPeak) ? 0 : 1) +
    saleRowChanges(state.draft.sale, state.initial.sale) +
    keyedRowsChanges(state.draft.rows, state.initial.rows)
  )
}

export function channelDraftFrom(accountId: number, entries: PricingEntry[]): ChannelDraft {
  const rows: Array<KeyedRow & { modelId: string }> = []
  for (const entry of entries) {
    const binding = entry.bindings.find((item) => item.account_id === accountId)
    if (binding) rows.push({ id: entry.id, modelId: entry.model_id, ...bindingRowFrom(binding) })
  }
  rows.sort((a, b) => a.modelId.localeCompare(b.modelId))
  return { rows: rows.map(({ id, upstreamModel, prices, peak }) => ({ id, upstreamModel, prices, peak })) }
}

export function cloneChannelDraft(draft: ChannelDraft): ChannelDraft {
  return { rows: cloneKeyedRows(draft.rows) }
}

export function channelDraftChanges(state: BlockState<ChannelDraft>): number {
  return keyedRowsChanges(state.draft.rows, state.initial.rows)
}

// ---- 按官方价 × 折扣快填上游价（muqian 2026-10-03：中转多按官方价打折标价，填一个数把空格一次填上）

/**
 * 折扣输入框的值 → 折扣；不是数字、负数为 null。allowZero 时 0 也认（上游价可以是 0：免费的上游，
 * muqian 2026-10-06），售价比例仍只认大于 0 的数。
 */
export function parseDiscount(text: string, allowZero = false): number | null {
  const value = Number(text.trim())
  if (text.trim() === '' || !Number.isFinite(value)) return null
  return value > 0 || (allowZero && value === 0) ? value : null
}

// 只为去掉浮点乘法尾巴（0.000005 × 0.03 = 1.4999999999999999e-7）：12 位有效数字远超 $/token 价格的精度
function scaled(price: number, ratio: number): number {
  return Number((price * ratio).toPrecision(12))
}

/**
 * 按官方价 × ratio 填一行上游价：只填空着的格子，已填的不动（搜索价只在官方显式设了时填）；
 * 这一行还没有分段、官方价有分段时，按官方的分段整份折算（切点相同，各段价 × ratio）。
 * 返回填了几处（每格算一处，整份分段算一处）。
 */
export function fillByDiscount(row: PriceRow, official: PriceRow, ratio: number): number {
  let filled = 0
  for (const key of [...PRICE_KEYS, ...SEARCH_KEYS]) {
    const base = official[key]
    if (row[key] == null && base != null) {
      row[key] = scaled(base, ratio)
      filled += 1
    }
  }
  if (row.segments.length === 0 && official.segments.length > 0) {
    row.segments = official.segments.map((segment) => {
      const next = { ...segment }
      for (const key of PRICE_KEYS) {
        const base = segment[key]
        next[key] = base == null ? null : scaled(base, ratio)
      }
      return next
    })
    filled += 1
  }
  return filled
}

/** 一条已保存的承接关系在草稿里的样子（不含 id：按模型的块里是渠道 ID，按渠道的块里是模型条目 ID） */
export function bindingRowFrom(binding: PricingBinding): Omit<KeyedRow, 'id'> {
  return { upstreamModel: binding.upstream_model ?? '', prices: priceRowFrom(binding), peak: peakFormFrom(binding.time_pricing) }
}

/**
 * 按渠道给模型新加的一行（id = 模型条目 ID）：同一上游的渠道承接过这个模型的，带上它的上游模型名、上游价与忙闲时；
 * 否则价格空着，上游忙闲时默认同官方（目录条目的分时，如 DeepSeek 高峰）。upstreamModel 给了就用它。
 */
export function newChannelRow(entry: PricingEntry, account: PricingAccount, accounts: PricingAccount[], upstreamModel?: string): KeyedRow {
  const sibling = siblingBindingOf(entry, account, accounts)
  const base = sibling ? bindingRowFrom(sibling) : { upstreamModel: '', prices: emptyPriceRow(), peak: peakFormFrom(entry.time_pricing) }
  return { id: entry.id, ...base, upstreamModel: upstreamModel ?? base.upstreamModel }
}

/** 同一上游（主机名相同）的另一个渠道承接这个模型时的那条承接关系（取第一个）；成品号没有主机名，返回 null */
export function siblingBindingOf(entry: PricingEntry, account: PricingAccount, accounts: PricingAccount[]): PricingBinding | null {
  const host = account.upstream_host
  if (!host) return null
  return (
    entry.bindings.find(
      (binding) => binding.account_id !== account.id && accounts.find((other) => other.id === binding.account_id)?.upstream_host === host
    ) ?? null
  )
}

// ---- 售价（muqian 2026-10-06：每项单独填，没填的按官方价 × 默认售价比例；用户倍率 = 在售价上再打折）

type SalePrices = Record<PriceKey, number | null>

/** 售价草稿：五项 + 各段（键 = 官方价分段的下界，分段跟着官方价走，不能单独加删）；null = 没单独定 */
export interface SaleRow {
  base: SalePrices
  segments: Record<number, SalePrices>
}

function emptySalePrices(): SalePrices {
  return { input_price: null, output_price: null, cache_read_price: null, cache_write_price: null, cache_write_1h_price: null }
}

function salePricesOf(source: Partial<Record<PriceKey, number | null>> | undefined): SalePrices {
  const out = emptySalePrices()
  for (const key of PRICE_KEYS) out[key] = source?.[key] ?? null
  return out
}

export function saleRowFrom(prices: PricingSalePrices | null | undefined): SaleRow {
  const segments: Record<number, SalePrices> = {}
  for (const segment of prices?.segments ?? []) segments[segment.min_tokens] = salePricesOf(segment)
  return { base: salePricesOf(prices ?? undefined), segments }
}

export function cloneSaleRow(row: SaleRow): SaleRow {
  const segments: Record<number, SalePrices> = {}
  for (const [min, prices] of Object.entries(row.segments)) segments[Number(min)] = { ...prices }
  return { base: { ...row.base }, segments }
}

/** 官方价这一行当前有效的分段下界（输入框里填对了的） */
export function officialSegmentMins(official: PriceRow): number[] {
  return official.segments.map((segment) => parseTokenThreshold(segment.above)).filter((min): min is number => min != null)
}

/** 售价改了几处：每格算一处（只数官方价现在还有的分段） */
export function saleRowChanges(current: SaleRow, initial: SaleRow): number {
  let changed = PRICE_KEYS.filter((key) => !sameNumber(current.base[key], initial.base[key])).length
  const mins = new Set([...Object.keys(current.segments), ...Object.keys(initial.segments)].map(Number))
  for (const min of mins) {
    const a = current.segments[min] ?? emptySalePrices()
    const b = initial.segments[min] ?? emptySalePrices()
    changed += PRICE_KEYS.filter((key) => !sameNumber(a[key], b[key])).length
  }
  return changed
}

/** 提交用：只带官方价现在还有的分段，一项都没填的段不带 */
export function saleRowToRequest(row: SaleRow, official: PriceRow): PricingSalePrices {
  const segments = officialSegmentMins(official)
    .map((min) => ({ min, prices: row.segments[min] }))
    .filter(({ prices }) => prices != null && PRICE_KEYS.some((key) => prices[key] != null))
    .map(({ min, prices }) => ({
      min_tokens: min,
      input_price: numberOrNull(prices.input_price),
      output_price: numberOrNull(prices.output_price),
      cache_write_price: numberOrNull(prices.cache_write_price),
      cache_write_1h_price: numberOrNull(prices.cache_write_1h_price),
      cache_read_price: numberOrNull(prices.cache_read_price)
    }))
  return {
    input_price: numberOrNull(row.base.input_price),
    output_price: numberOrNull(row.base.output_price),
    cache_write_price: numberOrNull(row.base.cache_write_price),
    cache_write_1h_price: numberOrNull(row.base.cache_write_1h_price),
    cache_read_price: numberOrNull(row.base.cache_read_price),
    segments
  }
}

/** 填了但格式不对的售价（负数、不是数字） */
export function saleRowInvalid(row: SaleRow, official: PriceRow): boolean {
  const cells = [row.base, ...officialSegmentMins(official).map((min) => row.segments[min]).filter((prices) => prices != null)]
  return cells.some((prices) => PRICE_KEYS.some((key) => Number.isNaN(prices[key])))
}

/** 官方价某一段五项的实际值：段内没填的输入 / 输出沿用基础价，缓存价按「本段输入价 ÷ 基础输入价」折算（与计费一致） */
function officialSegmentPrices(official: PriceRow, min: number): SalePrices {
  const segment = official.segments.find((item) => parseTokenThreshold(item.above) === min)
  const out = emptySalePrices()
  if (!segment) return out
  const input = segment.input_price ?? official.input_price
  const inputRatio = input != null && official.input_price ? input / official.input_price : 1
  out.input_price = input
  out.output_price = segment.output_price ?? official.output_price
  for (const key of ['cache_read_price', 'cache_write_price', 'cache_write_1h_price'] as const) {
    const base = official[key]
    out[key] = segment[key] ?? (base == null ? null : base * inputRatio)
  }
  return out
}

/**
 * 没单独定售价时实际按什么收（输入框的灰字）：基础价 = 官方价 × 默认售价比例；
 * 分段 = 基础售价定了的项按「基础售价 × 本段官方价 ÷ 基础官方价」，没定的按本段官方价 × 默认售价比例（与后端计费同一规则）。
 */
export function saleDefaults(official: PriceRow, sale: SaleRow, ratio: number): { base: SalePrices; segments: Record<number, SalePrices> } {
  const base = emptySalePrices()
  for (const key of PRICE_KEYS) base[key] = official[key] == null ? null : scaled(official[key] as number, ratio)
  const segments: Record<number, SalePrices> = {}
  for (const min of officialSegmentMins(official)) {
    const seg = officialSegmentPrices(official, min)
    const out = emptySalePrices()
    for (const key of PRICE_KEYS) {
      const officialSeg = seg[key]
      const officialBase = official[key]
      const saleBase = sale.base[key]
      if (officialSeg == null) continue
      out[key] = saleBase != null && officialBase ? scaled(saleBase, officialSeg / officialBase) : scaled(officialSeg, ratio)
    }
    segments[min] = out
  }
  return { base, segments }
}

/** 按官方价 × ratio 填售价：只填空着的格子（含分段），填过的不动；返回填了几格 */
export function fillSaleByRatio(sale: SaleRow, official: PriceRow, ratio: number): number {
  let filled = 0
  for (const key of PRICE_KEYS) {
    const base = official[key]
    if (sale.base[key] == null && base != null) {
      sale.base[key] = scaled(base, ratio)
      filled += 1
    }
  }
  for (const min of officialSegmentMins(official)) {
    const seg = officialSegmentPrices(official, min)
    const target = sale.segments[min] ?? (sale.segments[min] = emptySalePrices())
    for (const key of PRICE_KEYS) {
      const value = seg[key]
      if (target[key] == null && value != null) {
        target[key] = scaled(value, ratio)
        filled += 1
      }
    }
  }
  return filled
}

// ---- 忙闲时：官方的（目录条目的分时，向用户收钱整单乘倍数）与上游的（承接上的，渠道成本整单乘倍数）同一份表单
// （muqian 2026-10-06：所有模型的计费都从模型目录出发；上游有没有忙闲时在填承接时定）

/** 一个时段：开始 / 结束为 HH:mm（结束 00:00 = 到当天结束），倍数是输入框原文 */
export interface PeakPeriodForm {
  start: string
  end: string
  multiplier: string
}

export interface PeakForm {
  timezone: string
  weekdaysOnly: boolean
  periods: PeakPeriodForm[]
}

/** 时段的问题：时间格式不对、开始不早于结束、倍数不对（> 0、最多两位小数）、与前一个时段重叠 */
export type PeakPeriodError = 'time' | 'order' | 'multiplier' | 'overlap'

export function peakFormFrom(tp: TimePricing | null | undefined): PeakForm | null {
  if (!tp || !tp.periods?.length) return null
  return {
    timezone: tp.timezone,
    weekdaysOnly: tp.weekdays_only === true,
    periods: tp.periods.map((period) => ({ start: period.start_time, end: period.end_time, multiplier: String(period.multiplier) }))
  }
}

export function clonePeakForm(form: PeakForm | null): PeakForm | null {
  return form ? { ...form, periods: form.periods.map((period) => ({ ...period })) } : null
}

const TIME_RE = /^([01]\d|2[0-3]):[0-5]\d(:[0-5]\d)?$/

/** HH:mm(:ss) → 当天第几秒；结束 00:00 = 24 点（与后端 parseChannelTime 一致）；格式不对为 null */
function secondsOf(value: string, end: boolean): number | null {
  const text = value.trim()
  if (!TIME_RE.test(text)) return null
  const [h, m, sec] = text.split(':').map(Number)
  const seconds = h * 3600 + m * 60 + (sec ?? 0)
  return end && seconds === 0 ? 24 * 3600 : seconds
}

/** 倍数：> 0（至少 0.01）、最多两位小数（与后端 parseChannelTimePeriods 一致）；不对为 null */
export function parsePeakMultiplier(text: string): number | null {
  const value = Number(text.trim())
  if (text.trim() === '' || !Number.isFinite(value) || value < 0.01) return null
  return Math.abs(value * 100 - Math.round(value * 100)) > 1e-9 ? null : value
}

/** 每个时段的问题，与 periods 一一对应；不分忙闲时为空 */
export function peakErrors(form: PeakForm | null): Array<PeakPeriodError | null> {
  if (!form) return []
  const spans = form.periods.map((period) => ({ start: secondsOf(period.start, false), end: secondsOf(period.end, true) }))
  return form.periods.map((period, index) => {
    const { start, end } = spans[index]
    if (start == null || end == null) return 'time'
    if (start >= end) return 'order'
    if (parsePeakMultiplier(period.multiplier) == null) return 'multiplier'
    // 只和自身有效（开始早于结束）的时段比：无效的时段已经单独报错
    const overlaps = spans.some(
      (other, j) => j !== index && other.start != null && other.end != null && other.start < other.end && other.start < end && start < other.end
    )
    return overlaps ? 'overlap' : null
  })
}

/** 提交用（调用前先确认 peakErrors 没有问题）：没有时段 = 不分忙闲时 */
export function peakFormToRequest(form: PeakForm | null): TimePricing | null {
  if (!form || form.periods.length === 0) return null
  return {
    timezone: form.timezone,
    weekdays_only: form.weekdaysOnly,
    periods: form.periods.map((period) => ({
      start_time: period.start.trim(),
      end_time: period.end.trim(),
      multiplier: parsePeakMultiplier(period.multiplier) ?? 0
    }))
  }
}

export function samePeak(a: PeakForm | null, b: PeakForm | null): boolean {
  const left = a && a.periods.length > 0 ? a : null
  const right = b && b.periods.length > 0 ? b : null
  if (!left || !right) return left === right
  return (
    left.timezone === right.timezone &&
    left.weekdaysOnly === right.weekdaysOnly &&
    left.periods.length === right.periods.length &&
    left.periods.every(
      (period, i) =>
        period.start.trim() === right.periods[i].start.trim() &&
        period.end.trim() === right.periods[i].end.trim() &&
        period.multiplier.trim() === right.periods[i].multiplier.trim()
    )
  )
}

/** 最高的忙时倍数（开关上显示）；不分忙闲时为 null */
export function peakMaxMultiplier(form: PeakForm | null): number | null {
  if (!form || form.periods.length === 0) return null
  const values = form.periods.map((period) => parsePeakMultiplier(period.multiplier)).filter((value): value is number => value != null)
  return values.length ? Math.max(...values) : null
}
