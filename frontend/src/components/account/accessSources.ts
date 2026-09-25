import type { AccountPlatform } from '@/types'

// 新建渠道的第一步（muqian 2026-09-25）：先选「第三方 key / 成品号」。
//
// - 第三方 key 不选平台、也不选来源：只填协议 + 地址 + Key，常用官方地址做快捷填入，厂商按地址识别
//   （keyAddress.ts）；提交时不带平台，后端按地址推导。
// - 成品号（订阅账号与云账号）只走官方地址，授权流程各家不同，要选是哪家的账号。

export type AccessKind = 'key' | 'subscription'

/** 表单内部的账号类别，与 CreateAccountModal 的 accountCategory 同一套取值。 */
export type AccessCategory = 'oauth-based' | 'apikey' | 'bedrock' | 'service_account'

export interface AccessSource {
  id: string
  kind: AccessKind
  /** 表单内部用的平台：成品号即它的平台；第三方 key 只是占位（提交时不带，厂商看地址）。 */
  platform: AccountPlatform
  category: AccessCategory
  /** 图标用的平台标识。 */
  icon: AccountPlatform | 'relay'
  /** 专名（厂商 / 产品名），不翻译。 */
  name: string
  /** 一句话说明的 i18n key。 */
  hintKey: string
}

/** 第三方 key 只有这一个入口。 */
export const KEY_SOURCE_ID = 'key'

const hint = (id: string) => `admin.accounts.accessSource.hints.${id}`

export const ACCESS_SOURCES: readonly AccessSource[] = [
  { id: KEY_SOURCE_ID, kind: 'key', platform: 'openai', category: 'apikey', icon: 'relay', name: 'API Key', hintKey: hint('key') },

  { id: 'claude', kind: 'subscription', platform: 'anthropic', category: 'oauth-based', icon: 'anthropic', name: 'Claude', hintKey: hint('claude') },
  { id: 'chatgpt', kind: 'subscription', platform: 'openai', category: 'oauth-based', icon: 'openai', name: 'ChatGPT', hintKey: hint('chatgpt') },
  { id: 'gemini', kind: 'subscription', platform: 'gemini', category: 'oauth-based', icon: 'gemini', name: 'Gemini', hintKey: hint('gemini') },
  { id: 'antigravity', kind: 'subscription', platform: 'antigravity', category: 'oauth-based', icon: 'antigravity', name: 'Antigravity', hintKey: hint('antigravity') },
  { id: 'grok', kind: 'subscription', platform: 'grok', category: 'oauth-based', icon: 'grok', name: 'Grok', hintKey: hint('grok') },
  { id: 'bedrock', kind: 'subscription', platform: 'anthropic', category: 'bedrock', icon: 'anthropic', name: 'AWS Bedrock', hintKey: hint('bedrock') },
  { id: 'vertex-claude', kind: 'subscription', platform: 'anthropic', category: 'service_account', icon: 'anthropic', name: 'Vertex · Claude', hintKey: hint('vertexClaude') },
  { id: 'vertex-gemini', kind: 'subscription', platform: 'gemini', category: 'service_account', icon: 'gemini', name: 'Vertex · Gemini', hintKey: hint('vertexGemini') }
]

export function accessSourcesOf(kind: AccessKind): AccessSource[] {
  return ACCESS_SOURCES.filter((source) => source.kind === kind)
}

export function findAccessSource(id: string): AccessSource {
  const source = ACCESS_SOURCES.find((candidate) => candidate.id === id)
  if (!source) {
    throw new Error(`unknown access source: ${id}`)
  }
  return source
}

/** 新建表单的默认来源：与改版前的默认一致（Anthropic 成品号）。 */
export const DEFAULT_ACCESS_SOURCE_ID = 'claude'
