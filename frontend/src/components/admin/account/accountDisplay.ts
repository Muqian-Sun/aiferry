// 渠道的展示辅助（A5 从 AccountsView 挪出）：列表名称那行小字和详情抽屉共用。
import type { Account } from '@/types'
import { UPSTREAM_PROTOCOLS } from '@/components/account/protocolEndpoints'
import { sanitizeUrl } from '@/utils/url'

const GROK_QUOTA_SIGNAL_MAX_AGE_MS = 24 * 60 * 60 * 1000
const GROK_QUOTA_SIGNAL_MAX_FUTURE_SKEW_MS = 5 * 60 * 1000

function firstNonBlankString(...values: unknown[]): string | undefined {
  return values.find((value): value is string => (
    typeof value === 'string' && value.trim().length > 0
  ))
}

function normalizeGrokPlanKey(value: unknown): string {
  if (typeof value !== 'string') return ''
  return value
    .trim()
    .toLowerCase()
    .replace(/[\s_-]+/g, '')
}

function grokPersistedQuotaSnapshot(extra: Record<string, any>): Record<string, any> | undefined {
  const usage = extra.grok_usage_snapshot
  if (usage && typeof usage === 'object' && !Array.isArray(usage)) {
    return usage as Record<string, any>
  }
  const legacy = extra.grok_quota_snapshot
  if (legacy && typeof legacy === 'object' && !Array.isArray(legacy)) {
    return legacy as Record<string, any>
  }
  return undefined
}

function isGrokQuotaTimestampFresh(raw: unknown): boolean {
  const value = String(raw || '').trim()
  if (!value) return false
  const observedAt = Date.parse(value)
  if (!Number.isFinite(observedAt)) return false
  const age = Date.now() - observedAt
  return age <= GROK_QUOTA_SIGNAL_MAX_AGE_MS && age >= -GROK_QUOTA_SIGNAL_MAX_FUTURE_SKEW_MS
}

function isGrok45ResponsesQuotaModel(model: unknown): boolean {
  const value = String(model || '')
    .trim()
    .toLowerCase()
    .replace(/^(x-ai|xai)\//, '')
  return value === 'grok-4.5' || value.startsWith('grok-4.5-')
}

function grokQuotaLooksHeavy(snapshot: Record<string, any> | undefined): boolean {
  const req = Number(snapshot?.requests?.limit ?? 0)
  const tok = Number(snapshot?.tokens?.limit ?? 0)
  return req >= 8300 || tok >= 53_000_000
}

function grok45ResponsesPlanIsHeavy(snapshot: Record<string, any> | undefined): boolean {
  if (!snapshot) return false
  const hint = normalizeGrokPlanKey(snapshot.plan_from_45_responses)
  if (hint === 'supergrokheavy' && isGrokQuotaTimestampFresh(snapshot.plan_from_45_responses_at)) {
    return true
  }
  const observedAt = snapshot.last_headers_seen_at || snapshot.updated_at
  return (
    isGrok45ResponsesQuotaModel(snapshot.model) &&
    isGrokQuotaTimestampFresh(observedAt) &&
    grokQuotaLooksHeavy(snapshot)
  )
}

// JWT / unambiguous credentials outrank snapshots. SuperGrokPro is ambiguous
// (covers SuperGrok and Heavy). 8300/53M only upgrades when the window came
// from grok-4.5 Responses (or a carried 4.5 hint).
export function getAccountPlanType(row: any): string | undefined {
  if (!row) return undefined
  if (row.platform === 'grok') {
    const extra = (row.extra || {}) as Record<string, any>
    const billing = extra.grok_billing_snapshot as Record<string, any> | undefined
    const usage = extra.grok_usage_snapshot as Record<string, any> | undefined
    const legacyQuota = extra.grok_quota_snapshot as Record<string, any> | undefined
    const quota = grokPersistedQuotaSnapshot(extra)
    const cred = firstNonBlankString(row.credentials?.subscription_tier)
    const credKey = normalizeGrokPlanKey(cred)
    if (credKey && credKey !== 'supergrokpro') {
      return cred
    }
    if (
      grok45ResponsesPlanIsHeavy(quota) &&
      (credKey === 'supergrokpro' ||
        normalizeGrokPlanKey(billing?.plan) === 'supergrok' ||
        normalizeGrokPlanKey(billing?.plan) === 'supergrokpro')
    ) {
      return 'SuperGrok Heavy'
    }
    if (credKey === 'supergrokpro') {
      return firstNonBlankString(billing?.plan) || 'SuperGrok'
    }
    return firstNonBlankString(
      billing?.plan,
      usage?.subscription_tier,
      legacyQuota?.subscription_tier,
      extra.subscription_tier,
      row.credentials?.plan_type,
      row.parent_plan_type
    )
  }
  return firstNonBlankString(row.credentials?.plan_type, row.parent_plan_type)
}

export function getOpenAIAuthMode(row: any): string | undefined {
  if (!row || row.platform !== 'openai' || row.type !== 'oauth') return undefined
  const authMode = row.credentials?.auth_mode
  return typeof authMode === 'string' && authMode.trim() ? authMode : undefined
}


// 账号显示邮箱:优先账号自身(extra/credentials),影子账号回退母账号 parent_email。
// 供名称单元格 v-if/标题/文本三处共用,避免同一回退链在模板里重复三次。
export function accountDisplayEmail(row: any): string {
  return row.extra?.email_address || row.extra?.email || row.credentials?.email || row.parent_email || ''
}

// 第三方 key 配了哪些协议地址，按固定协议顺序；成品号没有协议地址（后端校验禁止），自然为空。
export function keyProtocolChips(row: Account): Array<{ protocol: string; host: string }> {
  return UPSTREAM_PROTOCOLS.flatMap((protocol) => {
    const url = row.protocol_endpoints?.[protocol]?.trim()
    if (!url) return []
    let host = url
    try {
      host = new URL(url).host
    } catch {
      // 非法地址原样展示，后端会在保存时拒绝
    }
    return [{ protocol, host }]
  })
}

// 第三方 key 名称链接到上游站点主页：地址只在协议映射里，按协议顺序取第一个已配置的。
export function accountHomepageUrl(row: Account): string {
  if (row.type !== 'apikey') return ''
  const endpoint = UPSTREAM_PROTOCOLS.map((protocol) => row.protocol_endpoints?.[protocol]).find((url) => !!url?.trim())
  const baseUrl = endpoint ? sanitizeUrl(endpoint) : ''
  return baseUrl ? new URL(baseUrl).origin : ''
}
