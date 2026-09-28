/**
 * 渠道健康（V2 被动监控）的汇总配置，只在管理站用。
 * 用户站的「服务状态」读数走 api/serviceStatus.ts（后端对普通用户只回白名单字段）。
 */
import { apiClient } from './client'

export interface MonitorConfig {
  version: number
  enabled: boolean
  refresh_interval_seconds: 60 | 300
  platforms: Array<{ platform: string; enabled: boolean; models: string[] }>
  health_thresholds: {
    minimum_sample: number
    warning_error_rate: number
    critical_error_rate: number
    target_ttft_ms: number
    warning_ttft_ms: number
    critical_ttft_ms: number
    warning_cache_rate: number
    critical_cache_rate: number
    error_weight: number
    ttft_weight: number
    cache_weight: number
  }
  /** Categories excluded from error_rate / health; still listed in error breakdown. */
  ignored_error_categories?: string[]
}

/** Ordered taxonomy mirrored from backend ChannelMonitorV2ErrorCategories. */
export const MONITOR_ERROR_CATEGORIES = [
  'content_policy',
  'authentication',
  'context_limit',
  'invalid_request',
  'model_unsupported',
  'quota_or_balance',
  'account_pool_unavailable',
  'rate_or_capacity',
  'timeout',
  'transport_or_stream',
  'upstream_forbidden',
  'not_found',
  'client_cancelled',
  'upstream_5xx',
  'internal',
  'other',
] as const

export async function getConfig() {
  const { data } = await apiClient.get<MonitorConfig>('/admin/channel-monitor-v2/config')
  return data
}
export async function updateConfig(config: MonitorConfig) {
  const { data } = await apiClient.put<MonitorConfig>('/admin/channel-monitor-v2/config', config)
  return data
}
