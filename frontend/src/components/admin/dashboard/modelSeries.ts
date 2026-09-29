/**
 * 概览「模型用量」一节的系列划分：柱状图与模型表共用，保证同一个模型两处颜色一致。
 *
 * 按区间内 Token 从高到低排；前 CHART_SERIES_SLOTS 个模型各占一个分类色槽（--af-chart-1..8），
 * 其余合成「其他」用灰色（--af-ink-4）。模型数不超过槽数时没有「其他」。
 * 颜色按排名分配，所以换时间范围后同一个模型可能换色——每次都以当前区间的排名为准。
 */
import { CHART_SERIES_SLOTS } from '@/composables/useChartTheme'
import type { ModelStat } from '@/types'

export interface ModelSeries {
  /** 有独立颜色的模型，按 Token 降序 */
  colored: string[]
  /** 并进「其他」的模型 */
  others: string[]
}

const finite = (value: unknown): number => {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

export function modelsByTokens(stats: ModelStat[]): ModelStat[] {
  return [...stats].sort((a, b) => finite(b.total_tokens) - finite(a.total_tokens) || a.model.localeCompare(b.model))
}

export function splitModelSeries(stats: ModelStat[]): ModelSeries {
  const ordered = modelsByTokens(stats).map((m) => m.model)
  return { colored: ordered.slice(0, CHART_SERIES_SLOTS), others: ordered.slice(CHART_SERIES_SLOTS) }
}

/** 模板里用的色块颜色（随明暗主题的 CSS 变量走）；不在前 8 的模型返回「其他」的灰 */
export function modelSwatchColor(series: ModelSeries, model: string): string {
  const index = series.colored.indexOf(model)
  return index >= 0 ? `rgb(var(--af-chart-${index + 1}))` : 'rgb(var(--af-ink-4))'
}
