/**
 * 「使用密钥」弹窗里带配置片段的客户端清单——首页「开箱即用的客户端」一栏与弹窗页签共用同一份，
 * 首页只列弹窗里真有配置的，不另写营销名单。
 */
export type UseKeyClientId = 'claude' | 'codex' | 'codex-ws' | 'gemini' | 'grok' | 'opencode'

export interface UseKeyClient {
  id: UseKeyClientId
  /** 页签 / 首页显示名（keys.useKeyModal.cliTabs.*） */
  labelKey: string
  /** 首页一句话说明（userUi.home.clients.items.*）；codex-ws 只是 Codex 的另一种传输，首页不单列 */
  homeKey: string | null
}

export const USE_KEY_CLIENTS: readonly UseKeyClient[] = [
  { id: 'claude', labelKey: 'keys.useKeyModal.cliTabs.claudeCode', homeKey: 'userUi.home.clients.items.claude' },
  { id: 'codex', labelKey: 'keys.useKeyModal.cliTabs.codexCli', homeKey: 'userUi.home.clients.items.codex' },
  { id: 'codex-ws', labelKey: 'keys.useKeyModal.cliTabs.codexCliWs', homeKey: null },
  { id: 'gemini', labelKey: 'keys.useKeyModal.cliTabs.geminiCli', homeKey: 'userUi.home.clients.items.gemini' },
  { id: 'grok', labelKey: 'keys.useKeyModal.cliTabs.grokCli', homeKey: 'userUi.home.clients.items.grok' },
  { id: 'opencode', labelKey: 'keys.useKeyModal.cliTabs.opencode', homeKey: 'userUi.home.clients.items.opencode' }
]

/** 首页展示的子集 */
export const HOME_CLIENTS = USE_KEY_CLIENTS.filter((client): client is UseKeyClient & { homeKey: string } => client.homeKey !== null)
