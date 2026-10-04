import { onBeforeUnmount, ref } from 'vue'

/**
 * 一条过几秒自动清掉的提示（如「已保存」）。再次 show 会重新计时；组件卸载时清掉定时器。
 */
export function useTransientMessage(durationMs = 2500) {
  const message = ref('')
  let timer: ReturnType<typeof setTimeout> | undefined

  function show(text: string) {
    message.value = text
    clearTimeout(timer)
    timer = setTimeout(() => (message.value = ''), durationMs)
  }

  function clear() {
    clearTimeout(timer)
    message.value = ''
  }

  onBeforeUnmount(() => clearTimeout(timer))

  return { message, show, clear }
}
