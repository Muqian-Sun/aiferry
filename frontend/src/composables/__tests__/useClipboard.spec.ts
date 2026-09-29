import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Mock i18n
vi.mock('@/i18n', () => ({
  i18n: {
    global: {
      t: (key: string) => key,
    },
  },
}))

// Mock app store

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({}),
}))

import { useClipboard } from '@/composables/useClipboard'

describe('useClipboard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.clearAllMocks()

    // 默认模拟安全上下文 + Clipboard API
    Object.defineProperty(window, 'isSecureContext', { value: true, writable: true })
    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
      writable: true,
      configurable: true,
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    // 恢复 execCommand
    if ('execCommand' in document) {
      delete (document as any).execCommand
    }
  })

  it('复制成功后 copied 变为 true', async () => {
    const { copied, copyToClipboard } = useClipboard()

    expect(copied.value).toBe(false)

    await copyToClipboard('hello')

    expect(copied.value).toBe(true)
  })

  it('copied 在 2 秒后自动恢复为 false', async () => {
    const { copied, copyToClipboard } = useClipboard()

    await copyToClipboard('hello')
    expect(copied.value).toBe(true)

    vi.advanceTimersByTime(2000)

    expect(copied.value).toBe(false)
  })



  it('空文本返回 false 且不复制', async () => {
    const { copyToClipboard, copied } = useClipboard()

    const result = await copyToClipboard('')

    expect(result).toBe(false)
    expect(copied.value).toBe(false)
    expect(navigator.clipboard.writeText).not.toHaveBeenCalled()
  })

  it('Clipboard API 失败时降级到 fallback', async () => {
    const writeTextMock = navigator.clipboard.writeText as any
    writeTextMock.mockRejectedValue(new Error('API failed'))

    // jsdom 没有 execCommand，手动定义
    const documentAny = document as any
    documentAny.execCommand = vi.fn().mockReturnValue(true)

    const { copyToClipboard, copied } = useClipboard()
    const result = await copyToClipboard('fallback text')

    expect(result).toBe(true)
    expect(copied.value).toBe(true)
    expect(document.execCommand).toHaveBeenCalledWith('copy')
  })

  it('非安全上下文使用 fallback', async () => {
    Object.defineProperty(window, 'isSecureContext', { value: false, writable: true })

    const documentAny = document as any
    documentAny.execCommand = vi.fn().mockReturnValue(true)

    const { copyToClipboard, copied } = useClipboard()
    const result = await copyToClipboard('insecure context text')

    expect(result).toBe(true)
    expect(copied.value).toBe(true)
    expect(navigator.clipboard.writeText).not.toHaveBeenCalled()
    expect(document.execCommand).toHaveBeenCalledWith('copy')
  })

  it('所有复制方式均失败时返回 false', async () => {
    const writeTextMock = navigator.clipboard.writeText as any
    writeTextMock.mockRejectedValue(new Error('fail'))

    const documentAny = document as any
    documentAny.execCommand = vi.fn().mockReturnValue(false)

    const { copyToClipboard, copied } = useClipboard()
    const result = await copyToClipboard('text')

    expect(result).toBe(false)
    expect(copied.value).toBe(false)
  })
})
