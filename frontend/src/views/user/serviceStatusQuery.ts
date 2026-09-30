/**
 * 服务状态（/channel-monitor-v2/snapshot 与 /matrix）：进页面前由路由预加载发出默认时间范围的两个请求，
 * 页面挂载时接手（router/routePreload.ts）——与模型页同一套「数据到了再换页」（muqian 2026-09-30）。
 */
import {
  getServiceStatusModels,
  getServiceStatusSnapshot,
  type ServiceStatusModels,
  type ServiceStatusRange,
  type ServiceStatusSnapshot
} from '@/api/serviceStatus'
import { adoptPreloaded, preloadKey, type Prefetch } from '@/router/routePreload'

export const SERVICE_STATUS_DEFAULT_RANGE: ServiceStatusRange = '24h'

const snapshotKey = (range: ServiceStatusRange) => preloadKey('service-status/snapshot', range)
const modelsKey = (range: ServiceStatusRange) => preloadKey('service-status/matrix', range)

export function preloadServiceStatus(prefetch: Prefetch): void {
  const range = SERVICE_STATUS_DEFAULT_RANGE
  prefetch(snapshotKey(range), () => getServiceStatusSnapshot(range))
  prefetch(modelsKey(range), () => getServiceStatusModels(range))
}

/** 有预加载好的就直接用（只接一次），否则现取；切换时间范围与定时刷新都走这里 */
export function loadServiceStatus(
  range: ServiceStatusRange,
  signal?: AbortSignal
): Promise<[ServiceStatusSnapshot, ServiceStatusModels]> {
  return Promise.all([
    adoptPreloaded(snapshotKey(range), () => getServiceStatusSnapshot(range, signal)),
    adoptPreloaded(modelsKey(range), () => getServiceStatusModels(range, signal))
  ])
}
