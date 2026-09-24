// 代理表单里有效期「选天数」⇄ 日历联动（创建 / 编辑对话框共用，原来写在 ProxiesView 里）。
// 天数自 base 起算（创建 = 今天；编辑 = 代理创建日），本地日历日 round-trip 稳定；canonical 仍是 expires_at 日期串。

export const EXPIRY_PRESETS = [7, 30, 90, 180]

const toLocalDateStr = (dt: Date): string => {
  const y = dt.getFullYear()
  const m = String(dt.getMonth() + 1).padStart(2, '0')
  const d = String(dt.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

// base 为空 → 今天本地 00:00；否则该日期本地 00:00
const baseDateOrToday = (baseDateStr: string): Date => {
  const base = baseDateStr ? new Date(`${baseDateStr}T00:00:00`) : new Date()
  base.setHours(0, 0, 0, 0)
  return base
}

// base + N 天 → 本地 YYYY-MM-DD；N≤0 / 空 → '' 表示永不过期
export const addDaysToBase = (baseDateStr: string, n: number | null): string => {
  const days = Number(n)
  if (!days || days <= 0) return ''
  const dt = baseDateOrToday(baseDateStr)
  dt.setDate(dt.getDate() + days)
  return toLocalDateStr(dt)
}

// target 相对 base 的整天数（本地日历差，避免时区 / 时刻抖动）
export const daysFromBase = (baseDateStr: string, targetDateStr: string): number | null => {
  if (!targetDateStr) return null
  const target = new Date(`${targetDateStr}T00:00:00`)
  return Math.round((target.getTime() - baseDateOrToday(baseDateStr).getTime()) / 86400000)
}
