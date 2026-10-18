/**
 * 管理站「调度」（2026-10-18）：每个模型、每条入站协议上，渠道此刻的调度顺序或不被选的原因，
 * 叠加并发 / 排队、粘性会话数和时间窗内实际接了多少请求。顺序与原因由后端按调度器同一套规则算出。
 */
import { apiClient } from '../client'

export type SchedulingWindow = '15m' | '1h' | '24h'
export type SchedulingProtocol = 'anthropic' | 'chat_completions' | 'responses' | 'gemini'
export type SchedulingItemState = 'ready' | 'queue' | 'blocked'
export type SchedulingWarning = 'load_unavailable' | 'sticky_unavailable'

/** 渠道在一条调度线上的位置；rank 从 1 起，只在 ready 时有：1 = 下一个没有粘性绑定的请求会落的渠道 */
export interface SchedulingItem {
  account_id: number
  state: SchedulingItemState
  rank?: number
  /** 能以入站协议直连（不用转换），调度时排在要转换的前面 */
  direct: boolean
  reason?: string
  until?: string
  detail?: string
}

export interface SchedulingLane {
  protocol: SchedulingProtocol
  items: SchedulingItem[]
}

/** 时间窗内这个模型落到该渠道的请求数，以及此刻绑在它上面的粘性会话数 */
export interface SchedulingModelChannel {
  account_id: number
  requests: number
  failures: number
  sticky_sessions: number
}

export interface SchedulingModel {
  entry_id: number
  model_id: string
  display_name: string
  vendor: string
  status: string
  requests: number
  failures: number
  /** 没选到渠道就失败的请求 */
  unrouted: number
  lanes: SchedulingLane[]
  channels: SchedulingModelChannel[]
}

/** 渠道此刻的整体状态与负载；in_use / waiting 为 null = 负载图不可用 */
export interface SchedulingChannel {
  id: number
  name: string
  platform: string
  type: string
  status: string
  priority: number
  concurrency: number
  in_use: number | null
  waiting: number | null
  blocked_reason?: string
  blocked_until?: string
  requests: number
  failures: number
}

export interface SchedulingOverview {
  generated_at: string
  window: SchedulingWindow
  /** 请求数汇总已覆盖到的时刻；还没汇总过时没有 */
  data_through?: string
  warnings: SchedulingWarning[]
  channels: SchedulingChannel[]
  models: SchedulingModel[]
}

export async function getSchedulingOverview(window: SchedulingWindow, signal?: AbortSignal) {
  const { data } = await apiClient.get<SchedulingOverview>('/admin/scheduling/overview', { params: { window }, signal })
  return data
}
