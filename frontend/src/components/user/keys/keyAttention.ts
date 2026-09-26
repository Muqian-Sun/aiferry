/**
 * 密钥的「限额」与「需要处理」（muqian 2026-09-25：密钥页摘要只在有事时出现，概览的「需要处理」也用它）。
 * 纯函数，便于单测；只看列表接口已经返回的字段，不另外请求。
 *
 * - 限额：额度（总额）+ 5h / 1d / 7d 三档速率，各自「已用 / 上限」。窗口的重置时间已过就当已用归零（接口里的已用要等下一次请求才刷新）。
 * - 需要处理：已过期、额度用尽（后端状态，密钥页可以按状态筛）；限额将满（仍可用、某一项 ≥ 80%）、7 天内到期（仍可用）。
 */
import { keysAPI } from '@/api'
import type { ApiKey } from '@/types'

export const LIMIT_WARN_RATIO = 0.8
export const EXPIRING_WITHIN_DAYS = 7
const DAY_MS = 24 * 60 * 60 * 1000

export type KeyLimitKind = 'quota' | '5h' | '1d' | '7d'

export interface KeyLimitMeter {
  kind: KeyLimitKind
  used: number
  limit: number
  /** 已用 ÷ 上限，可能超过 1 */
  ratio: number
  resetAt: string | null
}

function windowUsed(used: number | null | undefined, resetAt: string | null, now: Date): number {
  if (resetAt && new Date(resetAt).getTime() <= now.getTime()) return 0
  return used ?? 0
}

/** 设了上限的各项限额，顺序固定：额度、5h、1d、7d */
export function keyLimitMeters(key: ApiKey, now: Date = new Date()): KeyLimitMeter[] {
  const meters: KeyLimitMeter[] = []
  const push = (kind: KeyLimitKind, used: number, limit: number, resetAt: string | null) => {
    if (limit > 0) meters.push({ kind, used, limit, ratio: used / limit, resetAt })
  }
  push('quota', key.quota_used ?? 0, key.quota ?? 0, null)
  push('5h', windowUsed(key.usage_5h, key.reset_5h_at, now), key.rate_limit_5h ?? 0, key.reset_5h_at)
  push('1d', windowUsed(key.usage_1d, key.reset_1d_at, now), key.rate_limit_1d ?? 0, key.reset_1d_at)
  push('7d', windowUsed(key.usage_7d, key.reset_7d_at, now), key.rate_limit_7d ?? 0, key.reset_7d_at)
  return meters
}

/** 用得最满的那一项；没设任何限额时为 null */
export function tightestLimit(key: ApiKey, now: Date = new Date()): KeyLimitMeter | null {
  return keyLimitMeters(key, now).reduce<KeyLimitMeter | null>((best, meter) => (!best || meter.ratio > best.ratio ? meter : best), null)
}

/** 限额条 / 数字的颜色档：正常墨色，≥ 80% 橙，用尽红 */
export function limitLevel(ratio: number): 'normal' | 'warning' | 'danger' {
  if (ratio >= 1) return 'danger'
  return ratio >= LIMIT_WARN_RATIO ? 'warning' : 'normal'
}

export function isNearLimit(key: ApiKey, now: Date = new Date()): boolean {
  if (key.status !== 'active') return false
  return keyLimitMeters(key, now).some((meter) => meter.ratio >= LIMIT_WARN_RATIO)
}

export function isExpiringSoon(key: ApiKey, now: Date = new Date()): boolean {
  if (key.status !== 'active' || !key.expires_at) return false
  const left = new Date(key.expires_at).getTime() - now.getTime()
  return left > 0 && left <= EXPIRING_WITHIN_DAYS * DAY_MS
}

/** 距到期还有几天（向上取整，至少 1）；已过期或永不过期为 null */
export function daysUntilExpiry(key: Pick<ApiKey, 'expires_at'>, now: Date = new Date()): number | null {
  if (!key.expires_at) return null
  const left = new Date(key.expires_at).getTime() - now.getTime()
  return left > 0 ? Math.max(1, Math.ceil(left / DAY_MS)) : null
}

export interface KeyAttention {
  expired: ApiKey[]
  quotaExhausted: ApiKey[]
  nearLimit: ApiKey[]
  expiringSoon: ApiKey[]
}

export function keyAttention(keys: ApiKey[], now: Date = new Date()): KeyAttention {
  return {
    expired: keys.filter((key) => key.status === 'expired'),
    quotaExhausted: keys.filter((key) => key.status === 'quota_exhausted'),
    nearLimit: keys.filter((key) => isNearLimit(key, now)),
    expiringSoon: keys.filter((key) => isExpiringSoon(key, now))
  }
}

export function hasKeyAttention(attention: KeyAttention): boolean {
  return Object.values(attention).some((list) => list.length > 0)
}

/** 最多拉几页（每页 100）；再多就不算「需要处理」——数不全宁可不显示 */
const MAX_PAGES = 5

/**
 * 拉当前用户的全部密钥（每页 100，最多 5 页）。超过 500 把返回 complete=false，调用方不据此出数字。
 */
export async function loadAllKeys(): Promise<{ keys: ApiKey[]; complete: boolean }> {
  const first = await keysAPI.list(1, 100)
  const keys = [...first.items]
  let page = 2
  for (; page <= first.pages && page <= MAX_PAGES; page++) {
    const response = await keysAPI.list(page, 100)
    if (response.items.length === 0) break
    keys.push(...response.items)
  }
  return { keys, complete: first.pages <= MAX_PAGES }
}
