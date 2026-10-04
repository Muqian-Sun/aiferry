import { ref } from 'vue'

/**
 * 检测是否支持 Clipboard API（需要安全上下文：HTTPS/localhost）
 */
function isClipboardSupported(): boolean {
  return !!(navigator.clipboard && window.isSecureContext)
}

/**
 * 降级方案：使用 textarea + execCommand
 * 使用 textarea 而非 input，以正确处理多行文本
 */
function fallbackCopy(text: string): boolean {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.cssText = 'position:fixed;left:0;top:0;width:1px;height:1px;opacity:0;pointer-events:none'
  document.body.appendChild(textarea)
  textarea.focus({ preventScroll: true })
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)
  try {
    return document.execCommand('copy')
  } finally {
    document.body.removeChild(textarea)
  }
}

export function useClipboard() {
  const copied = ref(false)

  const copyToClipboard = async (text: string): Promise<boolean> => {
    if (!text) return false

    let success = false

    if (isClipboardSupported()) {
      try {
        await navigator.clipboard.writeText(text)
        success = true
      } catch {
        success = fallbackCopy(text)
      }
    } else {
      success = fallbackCopy(text)
    }

    if (success) {
      copied.value = true
      setTimeout(() => {
        copied.value = false
      }, 2000)
    } else {
      // 用户能看到的失败提示由调用方给（CopyButton 显示「复制失败」），这里只留排查用的日志
      console.error('copy to clipboard failed')
    }

    return success
  }

  return { copied, copyToClipboard }
}
