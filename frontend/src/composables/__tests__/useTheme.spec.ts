import { beforeEach, describe, expect, it, vi } from 'vitest'

// 模块级单例：每个用例重新加载模块，避免 initialized 状态串味
async function loadTheme() {
  vi.resetModules()
  return await import('../useTheme')
}

function mockMatchMedia(prefersDark: boolean) {
  const listeners: Array<(e: { matches: boolean }) => void> = []
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: prefersDark,
    media: query,
    addEventListener: (_: string, cb: (e: { matches: boolean }) => void) => listeners.push(cb),
    removeEventListener: vi.fn()
  })) as unknown as typeof window.matchMedia
  return { fire: (matches: boolean) => listeners.forEach((cb) => cb({ matches })) }
}

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
  })

  it('applies the saved theme on init', async () => {
    localStorage.setItem('theme', 'dark')
    mockMatchMedia(false)
    const { initTheme, useTheme } = await loadTheme()
    initTheme()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(useTheme().isDark.value).toBe(true)
  })

  it('follows the system preference when nothing is saved and does not persist it', async () => {
    const media = mockMatchMedia(true)
    const { initTheme, useTheme } = await loadTheme()
    initTheme()
    expect(useTheme().isDark.value).toBe(true)
    expect(localStorage.getItem('theme')).toBeNull()

    media.fire(false)
    expect(useTheme().isDark.value).toBe(false)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('toggle persists the choice and stops following the system', async () => {
    const media = mockMatchMedia(false)
    const { useTheme } = await loadTheme()
    const theme = useTheme()
    theme.toggleTheme()
    expect(theme.isDark.value).toBe(true)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')

    media.fire(false)
    expect(theme.isDark.value).toBe(true)
  })

  it('shares one state between callers', async () => {
    mockMatchMedia(false)
    const { useTheme } = await loadTheme()
    const a = useTheme()
    const b = useTheme()
    a.setTheme(true)
    expect(b.isDark.value).toBe(true)
  })
})
