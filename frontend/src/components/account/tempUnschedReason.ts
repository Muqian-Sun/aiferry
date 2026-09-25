// 临时停调的原因（渠道状态一行、详情抽屉横幅）：后端 temp_unschedulable_reason 多数是 JSON
// （service.TempUnschedState / tempUnschedReasonPayload，原因在 error_message 里），少数来源（OpenAI 403 冷却、
// OAuth 401 等）存的是纯文本。界面只写原因本身：能取到 error_message 就写它，否则原样写。

export function tempUnschedReasonText(raw: string | null | undefined): string {
  const text = (raw ?? '').trim()
  if (!text.startsWith('{')) return text
  try {
    const parsed: unknown = JSON.parse(text)
    const message = (parsed as { error_message?: unknown } | null)?.error_message
    if (typeof message === 'string' && message.trim()) return message.trim()
  } catch {
    // 以 { 开头的纯文本（如截断的上游响应体）：原样写
  }
  return text
}
