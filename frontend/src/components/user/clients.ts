/**
 * 「使用密钥」弹窗里带配置片段的客户端清单——弹窗页签与首页「客户端配置」计数共用同一份，
 * 首页不另写营销名单。
 */
export type UseKeyClientId = 'claude' | 'codex' | 'codex-ws' | 'gemini' | 'grok' | 'opencode'

export interface UseKeyClient {
  id: UseKeyClientId
  /** 页签显示名（keys.useKeyModal.cliTabs.*） */
  labelKey: string
  /** 只是另一个客户端的另一种传输（codex-ws 之于 codex）：首页计数不重复算 */
  variantOf?: UseKeyClientId
}

export const USE_KEY_CLIENTS: readonly UseKeyClient[] = [
  { id: 'claude', labelKey: 'keys.useKeyModal.cliTabs.claudeCode' },
  { id: 'codex', labelKey: 'keys.useKeyModal.cliTabs.codexCli' },
  { id: 'codex-ws', labelKey: 'keys.useKeyModal.cliTabs.codexCliWs', variantOf: 'codex' },
  { id: 'gemini', labelKey: 'keys.useKeyModal.cliTabs.geminiCli' },
  { id: 'grok', labelKey: 'keys.useKeyModal.cliTabs.grokCli' },
  { id: 'opencode', labelKey: 'keys.useKeyModal.cliTabs.opencode' }
]

/** 首页计数用：去掉传输变体后的客户端 */
export const HOME_CLIENTS = USE_KEY_CLIENTS.filter((client) => !client.variantOf)
