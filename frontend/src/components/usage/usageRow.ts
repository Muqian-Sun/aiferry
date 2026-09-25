import { reasoningEffortValuesEqual } from '@/utils/format'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import type { AdminUsageLog, UsageLog } from '@/types'

/**
 * 用量明细一行的派生值：明细表（两站共用）、管理站详情抽屉、管理站导出都要用，放一处免得各抄一份。
 * 这里只放纯函数，不引用管理端模块（用户站会打包进来）。
 */

type Translate = (key: string) => string

/** 请求类型的显示名：同步 / 流式 / WS / Live / 安全策略。 */
export function requestTypeLabel(row: UsageLog, t: Translate): string {
  const requestType = resolveUsageRequestType(row)
  if (requestType === 'cyber') return t('usage.cyber')
  if (requestType === 'live') return t('usage.live')
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

/** 一次请求的 Token 合计：输入 + 输出 + 缓存创建 + 缓存读取（输入 / 输出已含图片 Token）。 */
export function totalTokens(row: UsageLog | null | undefined): number {
  return (row?.input_tokens || 0) + (row?.output_tokens || 0) + (row?.cache_creation_tokens || 0) + (row?.cache_read_tokens || 0)
}

/** 请求推理强度与转发给上游的不一样（被映射过）。 */
export function hasReasoningEffortMapping(row: AdminUsageLog): boolean {
  const requested = row.reasoning_effort?.trim() || ''
  const forwarded = row.upstream_reasoning_effort?.trim() || ''
  return requested !== '' && forwarded !== '' && !reasoningEffortValuesEqual(requested, forwarded)
}

/** 实际发往上游的模型：映射过就是映射后的名字，否则就是请求模型。 */
export function sentUpstreamModel(row: AdminUsageLog): string {
  return row.upstream_model?.trim() || row.model?.trim() || ''
}

const normalizeModelVariant = (model: string): string => model
  .trim()
  .toLowerCase()
  .replace(/-latest$/, '')
  .replace(/-\d{4}-\d{2}-\d{2}$/, '')
  .replace(/-\d{8}$/, '')

/** 上游响应的模型与发出去的只差日期 / latest 后缀：疑似同一模型的版本变体，而不是换了模型。 */
export function isLikelyModelVariant(row: AdminUsageLog): boolean {
  const sent = sentUpstreamModel(row)
  const response = row.upstream_response_model?.trim() || ''
  return sent !== '' && response !== '' && normalizeModelVariant(sent) === normalizeModelVariant(response)
}

/** 耗时：1 秒内写毫秒，1 分钟内两位小数的秒，再长写「Xm Ys」，1 小时以上进位为「Xh Ym」，免去人工换算。 */
export function formatDurationMs(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)}s`
  const totalSec = Math.round(ms / 1000)
  if (totalSec < 3600) return `${Math.floor(totalSec / 60)}m ${totalSec % 60}s`
  return `${Math.floor(totalSec / 3600)}h ${Math.floor((totalSec % 3600) / 60)}m`
}

/** 这一笔的成本：标价 × 渠道成本倍率（没记倍率按 1，与统计 SQL 的 COALESCE 同口径）。 */
export function rowAccountCost(row: AdminUsageLog): number {
  const result = (row.total_cost ?? 0) * (row.account_rate_multiplier ?? 1)
  return Number.isFinite(result) ? result : 0
}
