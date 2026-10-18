/**
 * 管理站「调度」（2026-10-18）：
 * - 实时：每个模型、每条入站协议上，渠道此刻的调度顺序或不被选的原因，叠加并发 / 排队、粘性会话数和
 *   时间窗内按协议分的请求数。顺序与原因由后端按调度器同一套规则算出。
 * - 请求轨迹：每个请求每次选渠道考虑过哪些候选、各自为什么被跳过、选中了谁，以及每次上游尝试的结果。
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

/** 时间窗内的请求数；unrouted = 没选到渠道（只在模型上有） */
export interface SchedulingCounts {
  requests: number
  failures: number
  unrouted: number
}

/** 时间窗内这个模型落到该渠道的请求数（合计与按协议），以及此刻绑在它上面的粘性会话数 */
export interface SchedulingModelChannel {
  account_id: number
  requests: number
  failures: number
  by_protocol: Partial<Record<SchedulingProtocol, SchedulingCounts>>
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
  unrouted: number
  by_protocol: Partial<Record<SchedulingProtocol, SchedulingCounts>>
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
  warnings: SchedulingWarning[]
  channels: SchedulingChannel[]
  models: SchedulingModel[]
}

export async function getSchedulingOverview(window: SchedulingWindow, signal?: AbortSignal) {
  const { data } = await apiClient.get<SchedulingOverview>('/admin/scheduling/overview', { params: { window }, signal })
  return data
}

// ---------- 请求轨迹 ----------

export type SchedulingTraceOutcome = 'success' | 'failed' | 'no_channel' | 'canceled'
/** 列表的结果筛选；switched = 换过渠道 */
export type SchedulingTraceOutcomeFilter = '' | SchedulingTraceOutcome | 'switched'

export interface SchedulingTraceCandidate {
  account_id: number
  /** excluded / skipped / eligible / full / slot_busy / session_full / picked / queued */
  decision: string
  reason?: string
  load_rate?: number
}

export interface SchedulingTraceSticky {
  account_id: number
  /** hit / wait / miss */
  outcome: string
  reason?: string
}

/** 调度器的一次选渠道；at_ms 是距请求开始的毫秒数 */
export interface SchedulingTracePick {
  at_ms: number
  /** sticky / load / legacy / queue；选不到时为空、error 有值 */
  layer?: string
  account_id?: number
  error?: string
  sticky?: SchedulingTraceSticky
  candidates: SchedulingTraceCandidate[]
}

/** 一次失败的上游尝试 */
export interface SchedulingTraceAttempt {
  at_ms: number
  account_id: number
  status_code?: number
  kind?: string
  message?: string
}

export interface SchedulingTrace {
  id: number
  created_at: string
  request_id: string
  user_id?: number
  api_key_id?: number
  model: string
  catalog_entry_id?: number
  inbound_protocol: string
  outcome: SchedulingTraceOutcome
  status_code: number
  final_account_id?: number
  switch_count: number
  sticky_hit: boolean
  duration_ms: number
  picks: SchedulingTracePick[]
  attempts: SchedulingTraceAttempt[]
}

export interface SchedulingTracePage {
  items: SchedulingTrace[]
  total: number
  page: number
  page_size: number
}

export interface SchedulingTraceQuery {
  window: SchedulingWindow
  entry_id?: number
  protocol?: SchedulingProtocol | ''
  outcome?: SchedulingTraceOutcomeFilter
  page: number
  page_size: number
}

export async function getSchedulingTraces(query: SchedulingTraceQuery, signal?: AbortSignal) {
  const params: Record<string, string | number> = { window: query.window, page: query.page, page_size: query.page_size }
  if (query.entry_id) params.entry_id = query.entry_id
  if (query.protocol) params.protocol = query.protocol
  if (query.outcome) params.outcome = query.outcome
  const { data } = await apiClient.get<SchedulingTracePage>('/admin/scheduling/traces', { params, signal })
  return data
}
