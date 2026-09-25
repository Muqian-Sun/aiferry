// 渠道页的时长一律写成字（方案 2026-09-25「缩写换成字」）：「3 天 2 小时」「4 小时 12 分」「15 分钟」，
// 不再是 2d 3h / 4h 12m。t 由调用方传入（组件里的 useI18n），这样单测替换 useI18n 时也走同一条路。

type Translate = (key: string, params?: Record<string, unknown>) => string

/** 距 target 还有多久；已过去或无效时返回 null。不足 1 分钟按 1 分钟算。 */
export function durationUntilWords(target: string | Date | null | undefined, t: Translate, now: Date = new Date()): string | null {
  if (!target) return null
  const diffMs = new Date(target).getTime() - now.getTime()
  if (!Number.isFinite(diffMs) || diffMs <= 0) return null
  const totalMinutes = Math.max(1, Math.floor(diffMs / 60_000))
  const days = Math.floor(totalMinutes / (60 * 24))
  const hours = Math.floor(totalMinutes / 60) % 24
  const minutes = totalMinutes % 60
  if (days > 0) return t('admin.accounts.duration.daysHours', { d: days, h: hours })
  if (hours > 0) return t('admin.accounts.duration.hoursMinutes', { h: hours, m: minutes })
  return t('admin.accounts.duration.minutes', { m: minutes })
}
