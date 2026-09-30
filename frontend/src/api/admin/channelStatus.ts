/**
 * 管理站「渠道状态」（muqian 2026-09-30）：按渠道看可用率、首字延迟、缓存命中率与请求数。
 * 与用户站服务状态同一份被动统计（渠道健康汇总），指标结构沿用 api/serviceStatus.ts，另加请求数与渠道身份。
 */
import { apiClient } from '../client'
import type {
  ServiceStatusCoverage,
  ServiceStatusHealth,
  ServiceStatusMetric,
  ServiceStatusPoint,
  ServiceStatusRange
} from '../serviceStatus'

export interface ChannelStatusMetric extends ServiceStatusMetric {
  request_count: number
}

/** 一个渠道；account_id 为 0 = 没选到渠道就失败的请求（没有名字） */
export interface ChannelStatusRow {
  account_id: number
  name: string
  platform: string
  type: string
  status: string
  deleted: boolean
  metrics: ChannelStatusMetric
  health: ServiceStatusHealth
  buckets: ServiceStatusPoint[]
}

export interface ChannelStatus {
  refresh_interval_seconds: number
  coverage: ServiceStatusCoverage
  metrics: ChannelStatusMetric
  health: ServiceStatusHealth
  trend: ServiceStatusPoint[]
  items: ChannelStatusRow[]
  /** 上架模型，给按模型筛选用 */
  models: string[]
}

/** model 为空 = 全部流量；否则只算请求名解析到这个上架模型的 */
export async function getChannelStatus(range: ServiceStatusRange, model: string, signal?: AbortSignal) {
  const { data } = await apiClient.get<ChannelStatus>('/admin/channel-status', {
    params: model ? { range, model } : { range },
    signal
  })
  return data
}
