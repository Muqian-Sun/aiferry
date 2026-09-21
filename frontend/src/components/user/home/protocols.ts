/**
 * 首页「四条官方协议」的事实数据：协议名 / 端点 / 能抵达的厂商 走 i18n，路径是产品契约，写死在这里。
 */
export interface ProtocolRoute {
  key: 'messages' | 'responses' | 'chat' | 'gemini'
  method: 'POST'
  path: string
}

export const PROTOCOL_ROUTES: readonly ProtocolRoute[] = [
  { key: 'messages', method: 'POST', path: '/v1/messages' },
  { key: 'responses', method: 'POST', path: '/v1/responses' },
  { key: 'chat', method: 'POST', path: '/v1/chat/completions' },
  { key: 'gemini', method: 'POST', path: '/v1beta/models/{model}:generateContent' }
] as const
