/**
 * 页头时间范围（DateRangePicker）→ 接口参数。
 *
 * 「近 24 小时」是相对此刻的一段时间，要按精确时刻查（start_time / end_time）；其余预设和自定义日期按自然日查
 * （start_date / end_date，后端按请求带的 timezone 解释）。原来近 24 小时也只传两个日期，查成了昨天零点到今天结束的两个自然日
 * （2026-10-04 走查：用量页近 24 小时多出 2 条、列出了前一天上午的记录）。
 */

/** 日期选择器里唯一一个按「此刻往前推」算的预设 */
export const LAST_24_HOURS_PRESET = 'last24Hours'

/** 精确时间窗 [start_time, end_time)，ISO 8601（UTC） */
export interface TimeWindow {
  start_time: string
  end_time: string
}

/** 区间查询参数：两种写法只出现一种，另一种显式写成 undefined——铺在带着旧日期的筛选对象后面时能把它盖掉 */
export interface RangeParams {
  start_date?: string
  end_date?: string
  start_time?: string
  end_time?: string
}

const DAY_MS = 24 * 60 * 60 * 1000

/**
 * 近 24 小时的时间窗：终点取下一个整分钟，起点往前推 24 小时。
 * 取整分钟是为了同一分钟里算出的窗口完全相同：用户站进入用量页前的预加载和页面挂载各算一次，参数一致才接得上预加载
 * （router/routePreload.ts）；后端概览快照的缓存也按参数命中。代价是两端最多偏 1 分钟（粒度是拍的，够看「近 24 小时」）。
 */
export function last24HoursWindow(now: Date = new Date()): TimeWindow {
  const end = new Date(now)
  end.setSeconds(0, 0)
  end.setMinutes(end.getMinutes() + 1)
  return { start_time: new Date(end.getTime() - DAY_MS).toISOString(), end_time: end.toISOString() }
}

/** 应用或刷新时间范围时调用：近 24 小时按此刻算一个窗口并冻结（翻页、排序沿用同一个窗口，总数才不跳）；按天的范围没有窗口 */
export function windowForPreset(preset: string | null): TimeWindow | null {
  return preset === LAST_24_HOURS_PRESET ? last24HoursWindow() : null
}

export function rangeParams(startDate: string, endDate: string, window: TimeWindow | null): RangeParams {
  return window
    ? { start_date: undefined, end_date: undefined, start_time: window.start_time, end_time: window.end_time }
    : { start_date: startDate, end_date: endDate, start_time: undefined, end_time: undefined }
}

/** 起始日期晚于结束日期（YYYY-MM-DD 按字典序比较就是按日期比较）；有一端没填不算颠倒 */
export function isReversedDateRange(startDate: string, endDate: string): boolean {
  return Boolean(startDate && endDate && startDate > endDate)
}
