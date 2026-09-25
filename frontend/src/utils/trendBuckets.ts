/**
 * 趋势图的连续时间桶（管理站概览、用量页「分析」共用）。
 * 接口只返回有请求的桶，直接连线会把中间没请求的时段连过去，所以按区间补零。
 * 桶的写法与后端 TO_CHAR 一致：按天 YYYY-MM-DD；按小时 YYYY-MM-DD HH:00（截到当前小时）。
 */
import type { TrendDataPoint } from '@/types'

export type TrendGranularity = 'day' | 'hour'

const pad = (n: number) => String(n).padStart(2, '0')

export const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`

export function trendBucketKeys(startDate: string, endDate: string, granularity: TrendGranularity): string[] {
  const start = new Date(`${startDate}T00:00:00`)
  const end = new Date(`${endDate}T00:00:00`)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return []
  const keys: string[] = []
  if (granularity === 'day') {
    for (const d = new Date(start); d <= end; d.setDate(d.getDate() + 1)) keys.push(formatLocalDate(d))
    return keys
  }
  const last = Math.min(end.getTime() + 23 * 60 * 60 * 1000, Date.now())
  for (const d = new Date(start); d.getTime() <= last; d.setHours(d.getHours() + 1)) {
    keys.push(`${formatLocalDate(d)} ${pad(d.getHours())}:00`)
  }
  return keys
}

export function fillTrendBuckets(points: TrendDataPoint[], keys: string[]): TrendDataPoint[] {
  const byDate = new Map(points.map((point) => [point.date, point]))
  return keys.map(
    (date) =>
      byDate.get(date) ?? {
        date,
        requests: 0,
        input_tokens: 0,
        output_tokens: 0,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        total_tokens: 0,
        cost: 0,
        actual_cost: 0
      }
  )
}
