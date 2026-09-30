/**
 * 账号平台的显示名称。
 *
 * 原 utils/platformColors.ts 的按平台配色表已删（muqian 2026-09-24：管理站装饰色收成墨色，
 * 平台靠图标与名称区分，不靠颜色），这里只剩名称。
 */

/** 第三方 key 没识别出官方厂商时在徽章上占的位：中转 / 聚合平台。不是账号平台值。 */
export const RELAY_PLATFORM = 'relay' as const

export function platformLabel(p: string): string {
  switch (p) {
    case RELAY_PLATFORM: return 'Relay'
    case 'anthropic': return 'Anthropic'
    case 'openai': return 'OpenAI'
    case 'antigravity': return 'Antigravity'
    case 'gemini': return 'Gemini'
    case 'grok': return 'Grok'
    case 'kimi': return 'Kimi'
    case 'zhipu': return 'Zhipu AI'
    case 'deepseek': return 'DeepSeek'
    case 'minimax': return 'MiniMax'
    case 'opencode_go': return 'OpenCode'
    case 'composite': return 'Composite'
    default: return p || 'API'
  }
}
