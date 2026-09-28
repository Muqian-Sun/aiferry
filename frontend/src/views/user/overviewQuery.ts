/**
 * 概览：时间范围与各请求的参数，以及进入页面前的预加载。页面（OverviewView）和预加载（preloadOverview）共用这一份，
 * 两边发出的请求同名同参，预加载的结果才接得上（router/routePreload.ts）。
 */
import type { RouteLocationNormalized } from 'vue-router'
import { usageAPI } from '@/api'
import subscriptionsAPI from '@/api/subscriptions'
import { loadAllKeys } from '@/components/user/keys/keyAttention'
import { preloadKey, type Prefetch } from '@/router/routePreload'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { SITE_FEATURES } from '@/utils/siteFeatures'
import { formatLocalDate } from '@/utils/trendBuckets'
import { preloadModelPlaza } from '../modelPlazaQuery'
import type { UserErrorListParams } from '@/types'

/** 右上角 7 / 30 天：只作用于区间数与趋势 */
export const OVERVIEW_RANGE_DAYS = { '7d': 7, '30d': 30 } as const
export type OverviewRangeKey = keyof typeof OVERVIEW_RANGE_DAYS
export const OVERVIEW_DEFAULT_RANGE: OverviewRangeKey = '7d'

/** 区间起止（最后一天是今天）；用量明细的地址栏参数同名 */
export function overviewRange(days: number): { start: string; end: string } {
  const start = new Date()
  start.setDate(start.getDate() - (days - 1))
  return { start: formatLocalDate(start), end: formatLocalDate(new Date()) }
}

export function overviewSnapshotParams(start: string, end: string) {
  return {
    start_date: start,
    end_date: end,
    granularity: 'day' as const,
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    include_trend: true,
    include_model_stats: false,
    include_model_trend: false
  }
}

/** 今天的失败请求数（只要 total） */
export function overviewFailuresParams(today: string): UserErrorListParams {
  return { page: 1, page_size: 1, start_date: today, end_date: today }
}

/** 订阅额度块显不显示：由代码决定，「仅充值」模式下没有 */
export function overviewSubscriptionEnabled(simpleMode: boolean): boolean {
  return !simpleMode && SITE_FEATURES.subscription
}

/** 各请求的预加载键 */
export const overviewRequestKey = {
  stats: preloadKey('overview/stats'),
  snapshot: (params: ReturnType<typeof overviewSnapshotParams>) => preloadKey('overview/snapshot', params),
  keys: preloadKey('keys/all'),
  subscriptions: preloadKey('subscriptions/active'),
  failures: (params: UserErrorListParams) => preloadKey('usage/errors', params)
}

/** 进入概览前：三个数、默认区间的趋势、全部密钥（需要处理）、订阅与今日失败数（开着时）；新用户再加调用示例要的模型目录 */
export async function preloadOverview(_to: RouteLocationNormalized, prefetch: Prefetch): Promise<void> {
  const stats = prefetch(overviewRequestKey.stats, () => usageAPI.getDashboardStats())
  const { start, end } = overviewRange(OVERVIEW_RANGE_DAYS[OVERVIEW_DEFAULT_RANGE])
  const snapshotParams = overviewSnapshotParams(start, end)
  prefetch(overviewRequestKey.snapshot(snapshotParams), () => usageAPI.getDashboardSnapshotV2(snapshotParams))
  prefetch(overviewRequestKey.keys, () => loadAllKeys())
  if (overviewSubscriptionEnabled(useAuthStore().isSimpleMode)) {
    prefetch(overviewRequestKey.subscriptions, () => subscriptionsAPI.getActiveSubscriptions())
  }
  if (useAppStore().cachedPublicSettings?.allow_user_view_error_requests ?? false) {
    const failuresParams = overviewFailuresParams(formatLocalDate(new Date()))
    prefetch(overviewRequestKey.failures(failuresParams), () => usageAPI.listMyErrorRequests(failuresParams))
  }
  // 还没有任何请求的新用户看到的是「开始使用」，调用示例要从模型目录里挑模型
  const loaded = await stats.catch(() => null)
  if (loaded && loaded.total_requests === 0) preloadModelPlaza(prefetch)
}
