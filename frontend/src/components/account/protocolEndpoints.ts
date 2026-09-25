import { accountsAPI, type ProtocolDefaultsResponse } from '@/api/admin/accounts'
import type { ProtocolEndpoints, UpstreamProtocol } from '@/types'

// 第三方 key 的上游地址只认协议映射。官方地址表只在后端维护
// （GET /admin/accounts/protocol-defaults），这里只负责取用和校验，不保留副本，
// 也不在任何地方补默认地址。
//
// 一个 key 只承接一个上游协议（后端 NormalizeProtocolEndpoints 拒绝多协议）：官方地址表里一个平台
// 可能列了好几个协议的地址，预填、切换、预设都只取其中一个（2026-09-25 修：以前全填上导致建 key 被拒）。

/** 可配置的上游协议，顺序即编辑器展示顺序，与后端 service.UpstreamProtocols() 一致。 */
export const UPSTREAM_PROTOCOLS: readonly UpstreamProtocol[] = ['anthropic', 'chat_completions', 'responses', 'gemini']

let pendingDefaults: Promise<ProtocolDefaultsResponse> | null = null

/** 加载各平台官方地址。成功结果在页面生命周期内复用，失败不缓存，下次调用重试。 */
export function loadProtocolDefaults(): Promise<ProtocolDefaultsResponse> {
  if (!pendingDefaults) {
    pendingDefaults = accountsAPI.getProtocolDefaults().catch((error: unknown) => {
      pendingDefaults = null
      throw error
    })
  }
  return pendingDefaults
}

/** 仅供测试重置模块级缓存。 */
export function resetProtocolDefaultsCacheForTest(): void {
  pendingDefaults = null
}

/**
 * 前端账号模式到后端地址表模式键的映射：
 * 国产供应商的按量付费（payg）与不区分模式的平台对应后端的 default。
 */
export function protocolDefaultsModeKey(mode?: string): string {
  return !mode || mode === 'payg' ? 'default' : mode
}

/** 取某平台某模式的官方地址；后端没有该组合时返回空映射，由管理员手填。 */
export function protocolDefaultsFor(
  defaults: ProtocolDefaultsResponse | null,
  platform: string,
  mode?: string
): ProtocolEndpoints {
  const endpoints = defaults?.defaults[platform]?.[protocolDefaultsModeKey(mode)]
  return endpoints ? { ...endpoints } : {}
}

function configuredProtocols(endpoints: ProtocolEndpoints): UpstreamProtocol[] {
  return Object.keys(endpoints) as UpstreamProtocol[]
}

/** 各平台默认承接的协议：厂商原生协议；没列出的（国产供应商、中转）默认 Chat Completions。 */
const PREFERRED_PROTOCOL_BY_PLATFORM: Record<string, UpstreamProtocol> = {
  anthropic: 'anthropic',
  openai: 'responses',
  gemini: 'gemini',
  grok: 'responses',
}

export function preferredProtocolFor(platform: string): UpstreamProtocol {
  return PREFERRED_PROTOCOL_BY_PLATFORM[platform] ?? 'chat_completions'
}

/** 从一份多协议的官方地址里挑一个：优先 preferred，没有就按 UPSTREAM_PROTOCOLS 顺序取第一个。 */
export function pickSingleEndpoint(endpoints: ProtocolEndpoints, preferred: UpstreamProtocol): ProtocolEndpoints {
  if (endpoints[preferred] !== undefined) return { [preferred]: endpoints[preferred] }
  const protocol = UPSTREAM_PROTOCOLS.find((candidate) => endpoints[candidate] !== undefined)
  return protocol ? { [protocol]: endpoints[protocol] } : {}
}

/**
 * 第三方 key 配了 Anthropic 协议地址：Anthropic 协议上的 key 设置（自动透传、上游认证方式、
 * web search 模拟）只在按该协议转发时生效，账号弹窗据此展示，不看平台标签。
 * 看的是编辑中的地址行，刚添加、还没填地址的行也算（提交前另有非空校验）。
 */
export function hasAnthropicEndpoint(endpoints: ProtocolEndpoints): boolean {
  return 'anthropic' in endpoints
}

/**
 * 第三方 key 配了 OpenAI 系协议地址（Responses 或 Chat Completions）：OpenAI Responses 协议设置
 * （自动透传、WS mode、Compact）可配置，账号弹窗据此展示，不看平台标签。后端只对地址指向
 * OpenAI 官方或通用中转的 key 生效、其他厂商官方地址忽略——前端不识别厂商，只给提示。
 * 与 hasAnthropicEndpoint 一样，未填地址的新行也算。
 */
export function hasOpenAIEndpoint(endpoints: ProtocolEndpoints): boolean {
  return 'responses' in endpoints || 'chat_completions' in endpoints
}

export function sameEndpoints(a: ProtocolEndpoints, b: ProtocolEndpoints): boolean {
  const keysA = configuredProtocols(a)
  const keysB = configuredProtocols(b)
  return keysA.length === keysB.length && keysA.every((protocol) => a[protocol] === b[protocol])
}

/**
 * 平台或模式切换后是否换成新的官方地址：当前为空、或当前协议的地址仍等于切换前的官方地址
 * （管理员没改过）时替换成新官方地址里 preferred 那个协议（没有就按协议顺序取第一个）；
 * 管理员手填过的地址保留，避免切换一下就被吞掉。结果只含一个协议。
 * preferred 由调用方定：换了平台用新平台的默认协议，同一平台换模式（按量 / 套餐）传当前协议。
 */
export function endpointsAfterDefaultsChange(
  current: ProtocolEndpoints,
  previousDefaults: ProtocolEndpoints,
  nextDefaults: ProtocolEndpoints,
  preferred: UpstreamProtocol
): ProtocolEndpoints {
  const [protocol] = configuredProtocols(current)
  if (protocol) {
    const unchanged = !current[protocol]?.trim() || current[protocol] === previousDefaults[protocol]
    if (!unchanged) {
      return current
    }
  }
  return pickSingleEndpoint(nextDefaults, preferred)
}

/** 当前配置的协议；没配返回 null。 */
export function currentProtocolOf(endpoints: ProtocolEndpoints): UpstreamProtocol | null {
  return configuredProtocols(endpoints)[0] ?? null
}

export type ProtocolEndpointsIssue = { kind: 'empty' } | { kind: 'multiple' } | { kind: 'blank'; protocol: UpstreamProtocol }

/**
 * 提交前校验。与后端同一口径：恰好一个协议地址、地址不能为空。
 * 转发协议由已配置的地址决定，不存在「必须配某个协议」的约束。
 */
export function validateProtocolEndpoints(endpoints: ProtocolEndpoints): ProtocolEndpointsIssue | null {
  const protocols = configuredProtocols(endpoints)
  if (protocols.length === 0) {
    return { kind: 'empty' }
  }
  if (protocols.length > 1) {
    return { kind: 'multiple' }
  }
  const blank = protocols.find((protocol) => !endpoints[protocol]?.trim())
  if (blank) {
    return { kind: 'blank', protocol: blank }
  }
  return null
}

type Translate = (key: string, params?: Record<string, unknown>) => string

/** 校验问题转成提示文案。 */
export function describeProtocolEndpointsIssue(issue: ProtocolEndpointsIssue, t: Translate): string {
  if (issue.kind === 'empty') {
    return t('admin.accounts.protocolEndpoints.errors.empty')
  }
  if (issue.kind === 'multiple') {
    return t('admin.accounts.protocolEndpoints.errors.multiple')
  }
  const protocol = t(`admin.accounts.protocolEndpoints.protocols.${issue.protocol}`)
  return t('admin.accounts.protocolEndpoints.errors.blank', { protocol })
}

/**
 * 把一个预设地址填进当前协议：当前协议在预设支持的协议里就填它，否则换成预设的第一个协议。
 * 用于同一地址服务多个协议的预设（如 Grok）。结果只含一个协议。
 */
export function applyPresetUrl(
  endpoints: ProtocolEndpoints,
  protocols: readonly UpstreamProtocol[],
  url: string
): ProtocolEndpoints {
  const [current] = configuredProtocols(endpoints)
  const protocol = current && protocols.includes(current) ? current : protocols[0]
  return protocol ? { [protocol]: url } : { ...endpoints }
}

/** 提交用的映射：地址去首尾空白。须先通过 validateProtocolEndpoints。 */
export function trimProtocolEndpoints(endpoints: ProtocolEndpoints): ProtocolEndpoints {
  const out: ProtocolEndpoints = {}
  for (const protocol of configuredProtocols(endpoints)) {
    out[protocol] = (endpoints[protocol] ?? '').trim()
  }
  return out
}
