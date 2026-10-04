/**
 * 服务状态页的纯函数：状态配色、数字格式、把稀疏的逐段数据铺满时间窗。
 */
import type { ServiceHealth, ServiceStatusCoverage, ServiceStatusPoint } from '@/api/serviceStatus'

/** 红绿黄只用在状态上；灰 = 请求太少或没有请求，不下结论 */
export const HEALTH_DOT: Record<ServiceHealth, string> = {
  healthy: 'bg-af-success',
  warning: 'bg-af-warning',
  critical: 'bg-af-danger',
  unknown: 'bg-af-ink-4'
}

export const HEALTH_BAR: Record<ServiceHealth, string> = {
  healthy: 'bg-af-success',
  warning: 'bg-af-warning',
  critical: 'bg-af-danger',
  unknown: 'bg-af-hairline-strong'
}

/** 数字只在不正常时着色 */
export const HEALTH_TEXT: Record<ServiceHealth, string> = {
  healthy: 'text-af-ink',
  warning: 'text-af-warning',
  critical: 'text-af-danger',
  unknown: 'text-af-ink-3'
}

/** 0–1 的比率写成百分比（可用率、缓存命中率） */
export function formatPercent(value: number | null | undefined): string {
  if (value == null) return '—'
  const percent = value * 100
  // 99.95% 以上写 100%，其余一位小数：再多的精度读不出差别；整数不带 .0（0.0% → 0%）
  if (percent >= 99.95) return '100%'
  return `${Number(percent.toFixed(1))}%`
}

export function formatLatency(ms: number | null | undefined): string {
  if (ms == null) return '—'
  if (ms < 1000) return `${Math.round(ms)} ms`
  return `${(ms / 1000).toFixed(ms < 10_000 ? 1 : 0)} s`
}

/** 时间窗里的一段；没有请求的段 point 为 null */
export interface StatusSlot {
  start: Date
  point: ServiceStatusPoint | null
}

/**
 * 把逐段数据铺满 [requested_start, requested_end)：没有请求的段也占一格，色条才对得上时间。
 * 后端的段起点按 bucket_seconds 对齐 UTC 零点，与 requested_start 同一套刻度；
 * 对不上的点说明两边刻度变了，打一条警告（不静默丢）。
 */
export function fillSlots(coverage: ServiceStatusCoverage, points: ServiceStatusPoint[]): StatusSlot[] {
  const step = coverage.bucket_seconds * 1000
  const start = Date.parse(coverage.requested_start)
  const end = Date.parse(coverage.requested_end)
  if (!(step > 0) || !Number.isFinite(start) || !Number.isFinite(end) || end <= start) return []
  const byStart = new Map(points.map((point) => [Date.parse(point.bucket_start), point]))
  const slots: StatusSlot[] = []
  for (let at = start; at < end; at += step) {
    slots.push({ start: new Date(at), point: byStart.get(at) ?? null })
  }
  const placed = slots.filter((slot) => slot.point).length
  if (placed < points.length) {
    console.warn(`[service-status] ${points.length - placed} bucket(s) do not align with the requested window`)
  }
  return slots
}

/** 段的时间标签：按天的段只写日期；按小时起的段会跨天，带上日期；90 分钟内只写时分 */
export function formatSlotTime(date: Date, bucketSeconds: number, locale: string): string {
  const options: Intl.DateTimeFormatOptions =
    bucketSeconds >= 86_400
      ? { month: '2-digit', day: '2-digit' }
      : bucketSeconds >= 3_600
        ? { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }
        : { hour: '2-digit', minute: '2-digit' }
  return new Intl.DateTimeFormat(locale || undefined, options).format(date)
}
