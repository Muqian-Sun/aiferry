/**
 * 新建第三方 key 时按地址与协议建议渠道名（muqian 2026-10-03）：现有渠道都叫「上游 · 协议」（fenno · Chat、
 * agentrouter · Messages），按同一个写法填；管理员改过就不再跟着变。
 */
import type { UpstreamProtocol } from '@/types'

const PROTOCOL_SHORT: Record<UpstreamProtocol, string> = {
  anthropic: 'Messages',
  chat_completions: 'Chat',
  responses: 'Responses',
  gemini: 'Gemini'
}

// 两级公共后缀里常见的第二级（api.example.com.cn 取 example）
const SECOND_LEVEL = new Set(['com', 'net', 'org', 'co', 'ac', 'gov', 'edu'])

/** 地址里能代表上游的那一段：去掉 api. 之类的子域与顶级域；IP、localhost 原样返回；解析不了返回空串 */
export function upstreamHostLabel(url: string): string {
  let host = ''
  try {
    host = new URL(url.includes('://') ? url : `https://${url}`).hostname
  } catch {
    return ''
  }
  if (!host.includes('.') || /^\d+(\.\d+){3}$/.test(host)) return host
  const parts = host.split('.')
  let index = parts.length - 2
  if (parts.length >= 3 && SECOND_LEVEL.has(parts[index]) && parts[parts.length - 1].length === 2) index -= 1
  return parts[index]
}

export function suggestChannelName(label: string, protocol: UpstreamProtocol): string {
  return label ? `${label} · ${PROTOCOL_SHORT[protocol] ?? protocol}` : ''
}
