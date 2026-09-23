/**
 * 图表配色：从 styles/tokens.css 的 --af-* 变量读色，随 useTheme 的 isDark 重算。
 *
 * chart.js 不认 CSS 变量，所以在这里把 "r g b" 通道解析成 rgba() 字符串；
 * 依赖 isDark 是为了在 html.dark 切换后重新读 computed style（applyTheme 先切类再改 isDark，
 * 所以 computed 重跑时 getComputedStyle 已经是新值）。
 * 分类色固定 8 槽、按顺序分配、不循环；第 9 个系列应折进「其他」而不是生成新色。
 */
import { computed } from 'vue'
import { useTheme } from './useTheme'

export const CHART_SERIES_SLOTS = 8

export function readTokenRgb(name: string, alpha = 1): string {
  if (typeof document === 'undefined') return `rgba(0, 0, 0, ${alpha})`
  const channels = getComputedStyle(document.documentElement).getPropertyValue(`--af-${name}`).trim()
  const [r, g, b] = channels.split(/\s+/)
  if (!r || !g || !b) return `rgba(0, 0, 0, ${alpha})`
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

export interface ChartTheme {
  /** 坐标轴刻度、图例文字 */
  text: string
  /** 网格线 */
  grid: string
  /** tooltip 底色 / 文字 */
  tooltipBg: string
  tooltipText: string
  tooltipBorder: string
  /** 单色图（用户控制台）的主线与面积：墨色与极淡墨色 */
  ink: string
  inkFill: string
  /** 分类系列色，固定顺序 */
  series: string[]
  /** 面积 / 柱体的淡填充 */
  fill: (index: number) => string
  /** 第 index 个系列色（超出 8 槽折回最后一槽，调用方应先合并「其他」） */
  color: (index: number) => string
}

export function useChartTheme() {
  const { isDark } = useTheme()
  return computed<ChartTheme>(() => {
    void isDark.value
    const series = Array.from({ length: CHART_SERIES_SLOTS }, (_, i) => readTokenRgb(`chart-${i + 1}`))
    const slot = (index: number) => Math.min(Math.max(index, 0), CHART_SERIES_SLOTS - 1) + 1
    return {
      text: readTokenRgb('ink-3'),
      grid: readTokenRgb('hairline'),
      tooltipBg: readTokenRgb('sheet'),
      tooltipText: readTokenRgb('ink'),
      tooltipBorder: readTokenRgb('hairline-strong'),
      ink: readTokenRgb('ink'),
      inkFill: readTokenRgb('ink', 0.06),
      series,
      fill: (index: number) => readTokenRgb(`chart-${slot(index)}`, 0.12),
      color: (index: number) => readTokenRgb(`chart-${slot(index)}`)
    }
  })
}
