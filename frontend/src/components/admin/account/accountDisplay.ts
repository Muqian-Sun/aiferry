// 渠道的展示辅助（A5 从 AccountsView 挪出）：列表名称那行小字和详情抽屉共用。
import type { Account, AccountPlatform } from '@/types'
import { UPSTREAM_PROTOCOLS } from '@/components/account/protocolEndpoints'
import { sanitizeUrl } from '@/utils/url'
import { platformLabel, RELAY_PLATFORM } from '@/utils/platformLabel'
import { vendorLabel } from '@/components/modelPlaza/catalog'

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


// 渠道显示邮箱：优先渠道自身（extra / credentials），影子渠道回退母渠道的 parent_email。
export function accountDisplayEmail(row: any): string {
  return row.extra?.email_address || row.extra?.email || row.credentials?.email || row.parent_email || ''
}

// 列表名称下面那行「厂商 · 接入方式」（方案 2026-09-25）：厂商用图标 + 名称；第三方 key 看按地址识别的厂商，
// 写公司名（Moonshot AI，不写 Kimi），没识别出来的是中转。套餐、隐私、到期、协议地址都在详情抽屉里看。
export function accountVendor(
  row: Pick<Account, 'platform' | 'type' | 'vendor'>
): { icon: AccountPlatform | typeof RELAY_PLATFORM; labelKey?: string; label?: string } {
  if (row.type !== 'apikey') return { icon: row.platform, label: platformLabel(row.platform) }
  if (row.vendor) return { icon: row.vendor as AccountPlatform, label: vendorLabel(row.vendor) }
  return { icon: RELAY_PLATFORM, labelKey: 'admin.accounts.vendorRelay' }
}

/** Antigravity 订阅等级（load_code_assist 里优先 paidTier，否则 currentTier），对应 admin.accounts.tier.* */
export function antigravityTierKey(row: Pick<Account, 'platform' | 'extra'>): 'free' | 'pro' | 'ultra' | null {
  if (row.platform !== 'antigravity') return null
  const lca = row.extra?.load_code_assist as Record<string, any> | undefined
  const tier = [lca?.paidTier?.id, lca?.currentTier?.id].find((id): id is string => typeof id === 'string')
  switch (tier) {
    case 'free-tier':
      return 'free'
    case 'g1-pro-tier':
      return 'pro'
    case 'g1-ultra-tier':
      return 'ultra'
    default:
      return null
  }
}

/** OpenAI 的 Compact 支持：只看探测结果（Compact 模式写死 auto），没探测过是 auto。非 OpenAI 返回 null。 */
export function openAICompactState(row: Pick<Account, 'platform' | 'type' | 'extra'>): 'active' | 'blocked' | 'auto' | null {
  if (row.platform !== 'openai' || (row.type !== 'oauth' && row.type !== 'apikey')) return null
  const extra = row.extra as Record<string, unknown> | undefined
  if (typeof extra?.openai_compact_supported === 'boolean') return extra.openai_compact_supported ? 'active' : 'blocked'
  return 'auto'
}

// 第三方 key 名称链接到上游站点主页：地址只在协议映射里，按协议顺序取第一个已配置的。
export function accountHomepageUrl(row: Account): string {
  if (row.type !== 'apikey') return ''
  const endpoint = UPSTREAM_PROTOCOLS.map((protocol) => row.protocol_endpoints?.[protocol]).find((url) => !!url?.trim())
  const baseUrl = endpoint ? sanitizeUrl(endpoint) : ''
  return baseUrl ? new URL(baseUrl).origin : ''
}
