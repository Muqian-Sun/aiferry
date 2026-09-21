import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

async function load() {
  vi.resetModules()
  const theme = await import('../useTheme')
  const chart = await import('../useChartTheme')
  return { ...theme, ...chart }
}

function setToken(name: string, channels: string) {
  document.documentElement.style.setProperty(`--af-${name}`, channels)
}

describe('useChartTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    document.documentElement.removeAttribute('style')
    window.matchMedia = vi.fn().mockReturnValue({ matches: false, addEventListener: vi.fn() }) as unknown as typeof window.matchMedia
  })

  it('reads token channels into rgba strings with the requested alpha', async () => {
    setToken('chart-1', '10 143 163')
    const { readTokenRgb } = await load()
    expect(readTokenRgb('chart-1')).toBe('rgba(10, 143, 163, 1)')
    expect(readTokenRgb('chart-1', 0.12)).toBe('rgba(10, 143, 163, 0.12)')
  })

  it('exposes 8 fixed series slots and clamps overflow to the last slot', async () => {
    for (let i = 1; i <= 8; i++) setToken(`chart-${i}`, `${i} ${i} ${i}`)
    const { useChartTheme, CHART_SERIES_SLOTS } = await load()
    const theme = useChartTheme()
    expect(CHART_SERIES_SLOTS).toBe(8)
    expect(theme.value.series).toHaveLength(8)
    expect(theme.value.color(0)).toBe('rgba(1, 1, 1, 1)')
    expect(theme.value.color(7)).toBe('rgba(8, 8, 8, 1)')
    expect(theme.value.color(11)).toBe('rgba(8, 8, 8, 1)')
  })

  it('re-reads tokens when the theme flips', async () => {
    setToken('ink-3', '1 1 1')
    const { useChartTheme, useTheme } = await load()
    const theme = useChartTheme()
    expect(theme.value.text).toBe('rgba(1, 1, 1, 1)')

    // 模拟 .dark 下变量变化：applyTheme 切类后 computed 依赖 isDark 重跑
    setToken('ink-3', '2 2 2')
    useTheme().setTheme(true)
    await nextTick()
    expect(theme.value.text).toBe('rgba(2, 2, 2, 1)')
  })
})
