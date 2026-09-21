/**
 * 请求延迟健康度分档（用于用量明细"延迟"列的纵向健康扫视）。
 *
 * 首 Token（TTFT）：10s 内正常，10-30s 偏慢，30-60s 缓慢，60s 及以上严重。
 * 总耗时：流式请求整体时长天然更长，阈值放宽为 1min / 3min / 5min。
 */
export type LatencySeverity = 'good' | 'warn' | 'slow' | 'critical'

export const FIRST_TOKEN_THRESHOLDS_MS = {
  warn: 10_000,
  slow: 30_000,
  critical: 60_000,
} as const

export const DURATION_THRESHOLDS_MS = {
  warn: 60_000,
  slow: 180_000,
  critical: 300_000,
} as const

interface Thresholds {
  warn: number
  slow: number
  critical: number
}

const classify = (ms: number, thresholds: Thresholds): LatencySeverity => {
  if (ms >= thresholds.critical) return 'critical'
  if (ms >= thresholds.slow) return 'slow'
  if (ms >= thresholds.warn) return 'warn'
  return 'good'
}

export const firstTokenSeverity = (ms: number): LatencySeverity =>
  classify(ms, FIRST_TOKEN_THRESHOLDS_MS)

export const durationSeverity = (ms: number): LatencySeverity =>
  classify(ms, DURATION_THRESHOLDS_MS)

export const LATENCY_TEXT_CLASSES: Record<LatencySeverity, string> = {
  good: 'text-af-success',
  warn: 'text-af-warning',
  slow: 'text-af-danger/80',
  critical: 'text-af-danger',
}

/**
 * 健康度色条：无首字数据时整条按总耗时档着色；有首字数据时上半按首字档、下半按总耗时档各取一色（两段硬切，不做渐变）。
 * 四档只用三种语义色：slow 是 danger 的 60% 不透明度，与 critical 保持同一色相、不同强度。
 */
export const LATENCY_BAR_CLASSES: Record<LatencySeverity, string> = {
  good: 'bg-af-success',
  warn: 'bg-af-warning',
  slow: 'bg-af-danger/60',
  critical: 'bg-af-danger',
}
