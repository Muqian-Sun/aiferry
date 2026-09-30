/**
 * 用户站「服务状态」：各模型最近能不能用、快不快。
 * 对应后端 dto.ServiceStatus*——普通用户路由只回可用率、首字延迟和状态档位，没有平台 / 渠道 / 用量规模。
 */
import { apiClient } from './client'

export type ServiceStatusRange = '90m' | '24h' | '7d' | '30d'
/** unknown = 这段时间请求太少，不下结论 */
export type ServiceHealth = 'healthy' | 'warning' | 'critical' | 'unknown'

export interface ServiceStatusMetric {
  /** 0–1；这段时间没有请求时为 null */
  availability: number | null
  ttft_p50_ms: number | null
  ttft_p90_ms: number | null
}

export interface ServiceStatusHealth {
  overall: ServiceHealth
  availability: ServiceHealth
  ttft: ServiceHealth
}

export interface ServiceStatusCoverage {
  requested_start: string
  /** 不含 */
  requested_end: string
  data_through: string
  bucket_seconds: number
  coverage_complete: boolean
  /** 首次启用后补历史数据的进度；补齐后为 null */
  backfill_percent: number | null
}

export interface ServiceStatusPoint {
  bucket_start: string
  metrics: ServiceStatusMetric
  health: ServiceStatusHealth
}

export interface ServiceStatusSnapshot {
  refresh_interval_seconds: number
  coverage: ServiceStatusCoverage
  metrics: ServiceStatusMetric
  health: ServiceStatusHealth
  trend: ServiceStatusPoint[]
}

export interface ServiceStatusModel {
  model: string
  metrics: ServiceStatusMetric
  health: ServiceStatusHealth
  buckets: ServiceStatusPoint[]
}

export interface ServiceStatusModels {
  coverage: ServiceStatusCoverage
  items: ServiceStatusModel[]
}

/** 功能被站长关掉时接口返回的原因码 */
export const SERVICE_STATUS_DISABLED_REASON = 'CHANNEL_MONITOR_DISABLED'

export async function getServiceStatusSnapshot(range: ServiceStatusRange, signal?: AbortSignal) {
  const { data } = await apiClient.get<ServiceStatusSnapshot>('/channel-monitor-v2/snapshot', { params: { range }, signal })
  return data
}

/** 每个模型一行，带逐段趋势 */
export async function getServiceStatusModels(range: ServiceStatusRange, signal?: AbortSignal) {
  const { data } = await apiClient.get<ServiceStatusModels>('/channel-monitor-v2/matrix', { params: { range }, signal })
  return data
}
