/**
 * 模型目录标价的显示格式。
 *
 * 标价是单价（按 Token 的换成 $ / 百万 Token），不是汇总金额，所以不走 utils/money 的「两位小数、不足一分写 <$0.01」：
 * $0.075 / 百万 Token 写成 <$0.01 或 $0.08 都是错的。但同一列的位数要一致——以前同一列里既有 $3.00 又有 $0.3，
 * 读的人分不清哪个是整数价、哪个被截了。做法：一列（一个抽屉）里的价共用一个小数位数，
 * 取「能把每个价都写全的最少位数」，至少 2 位、至多 maxDecimals 位（再多的四舍五入）。
 */
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

const MIN_DECIMALS = 2

/** 目录里按 Token 的价是 $/token；展示一律换成 $/百万 Token */
export function perMillion(value: number | null | undefined): number | null {
  return value == null ? null : value * 1_000_000
}

/** 这个值写 decimals 位小数是否不丢精度（按 10 位比较，吃掉 1.25e-6 × 1e6 = 1.2499999999999998 这类浮点误差） */
function exactAt(value: number, decimals: number): boolean {
  return Number(value.toFixed(decimals)) === Number(value.toFixed(10))
}

/** 一组价共用的小数位数：把每个值都写全所需的最少位数，夹在 [2, maxDecimals] 之间；空值不参与 */
export function sharedPriceDecimals(values: Array<number | null | undefined>, maxDecimals: number): number {
  let decimals = MIN_DECIMALS
  for (const value of values) {
    if (value == null || !Number.isFinite(value)) continue
    while (decimals < maxDecimals && !exactAt(value, decimals)) decimals++
  }
  return decimals
}

const formatters = new Map<number, Intl.NumberFormat>()

/** 按给定位数写标价：`$1,234.50`；空值写「—」 */
export function formatListPrice(value: number | null | undefined, decimals: number): string {
  if (value == null || !Number.isFinite(value)) return '—'
  let formatter = formatters.get(decimals)
  if (!formatter) {
    formatter = new Intl.NumberFormat('en-US', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })
    formatters.set(decimals, formatter)
  }
  return `$${formatter.format(value)}`
}

/** 列表「标价」格里显示的价：按 Token 是输入 / 输出（$ / 百万 Token），其余计费是默认按次价 */
export function listPriceValues(entry: ModelCatalogEntry): Array<number | null> {
  if (!entry.billing_mode || entry.billing_mode === 'token') {
    return [perMillion(entry.input_price), perMillion(entry.output_price)]
  }
  return [entry.per_request_price]
}

/** 列表标价列最多 4 位小数（完整的价在详情抽屉里） */
export const LIST_PRICE_MAX_DECIMALS = 4
/** 详情抽屉里写全：目录单价存 12 位小数，换成每百万 Token 后 6 位足够 */
export const DETAIL_PRICE_MAX_DECIMALS = 6
