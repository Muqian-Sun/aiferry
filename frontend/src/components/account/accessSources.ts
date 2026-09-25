import type { AccountPlatform } from '@/types'

// 新建渠道的第一步（muqian 2026-09-25）：先选「第三方 key / 成品号」，再选来源。
//
// - 第三方 key 不选平台：来源只决定预填哪家的官方地址和厂商专属选项（国产套餐、Gemini 档位等），
//   提交时不带平台，后端按地址认厂商（认不出的中转按协议归族）。「自定义中转」不预填地址。
// - 成品号（订阅账号与云账号）只走官方地址，来源就是它的平台。

export type AccessKind = 'key' | 'subscription'

/** 表单内部的账号类别，与 CreateAccountModal 的 accountCategory 同一套取值。 */
export type AccessCategory = 'oauth-based' | 'apikey' | 'bedrock' | 'service_account'

export interface AccessSource {
  id: string
  kind: AccessKind
  /** 表单内部用的平台：决定预填地址与厂商专属选项；第三方 key 提交时不带它。 */
  platform: AccountPlatform
  category: AccessCategory
  /** 图标用的平台标识；自定义中转用通用图标。 */
  icon: AccountPlatform | 'relay'
  /** 专名（厂商 / 产品名）直接写，不翻译。 */
  name?: string
  /** 需要翻译的名称。 */
  nameKey?: string
  /** 一句话说明的 i18n key。 */
  hintKey: string
}

export const CUSTOM_RELAY_SOURCE_ID = 'relay'

const hint = (id: string) => `admin.accounts.accessSource.hints.${id}`

export const ACCESS_SOURCES: readonly AccessSource[] = [
  { id: 'anthropic-key', kind: 'key', platform: 'anthropic', category: 'apikey', icon: 'anthropic', name: 'Anthropic', hintKey: hint('anthropicKey') },
  { id: 'openai-key', kind: 'key', platform: 'openai', category: 'apikey', icon: 'openai', name: 'OpenAI', hintKey: hint('openaiKey') },
  { id: 'gemini-key', kind: 'key', platform: 'gemini', category: 'apikey', icon: 'gemini', name: 'Gemini', hintKey: hint('geminiKey') },
  { id: 'grok-key', kind: 'key', platform: 'grok', category: 'apikey', icon: 'grok', name: 'xAI', hintKey: hint('grokKey') },
  { id: 'kimi', kind: 'key', platform: 'kimi', category: 'apikey', icon: 'kimi', name: 'Kimi', hintKey: hint('kimi') },
  { id: 'zhipu', kind: 'key', platform: 'zhipu', category: 'apikey', icon: 'zhipu', name: 'Zhipu GLM', hintKey: hint('zhipu') },
  { id: 'deepseek', kind: 'key', platform: 'deepseek', category: 'apikey', icon: 'deepseek', name: 'DeepSeek', hintKey: hint('deepseek') },
  { id: 'minimax', kind: 'key', platform: 'minimax', category: 'apikey', icon: 'minimax', name: 'MiniMax', hintKey: hint('minimax') },
  { id: 'opencode', kind: 'key', platform: 'opencode_go', category: 'apikey', icon: 'opencode_go', name: 'OpenCode', hintKey: hint('opencode') },
  // 中转站的 key：平台只影响表单里的提示，地址与协议由管理员自己填。
  { id: CUSTOM_RELAY_SOURCE_ID, kind: 'key', platform: 'openai', category: 'apikey', icon: 'relay', nameKey: 'admin.accounts.accessSource.relay', hintKey: hint('relay') },

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
