/**
 * 用量明细：地址栏参数 → 首屏状态 → 请求参数。页面（UsageView）和进入页面前的预加载（preloadUsage）共用这一份，
 * 两边发出的请求同名同参，预加载的结果才接得上（router/routePreload.ts）。
 */
import type { LocationQuery, RouteLocationNormalized } from 'vue-router'
import { usageAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { preloadKey, type Prefetch } from '@/router/routePreload'
import { formatLocalDate } from '@/utils/trendBuckets'
import { LAST_24_HOURS_PRESET, rangeParams, windowForPreset, type RangeParams, type TimeWindow } from '@/utils/dateRange'
import { requestTypeToLegacyStream } from '@/utils/usageRequestType'
import type { UsageQueryParams, UserErrorListParams } from '@/types'

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/

const queryString = (query: LocationQuery, key: string): string => {
  const value = query[key]
  return typeof value === 'string' ? value : ''
}

/** 默认时间范围近 24 小时在日期选择器里显示的两个日期（查询按精确时刻，见 windowForPreset） */
export function last24HoursRange(): { start: string; end: string } {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

export interface UsageRouteState {
  startDate: string
  endDate: string
  /** 地址栏没带范围 = 默认的近 24 小时；带了就是按天的范围 */
  preset: string | null
  /** 近 24 小时的精确窗口（按天的范围为 null）；预加载和页面挂载各读一次地址栏，同一分钟里算出的窗口相同 */
  timeWindow: TimeWindow | null
  apiKeyId?: number
  model: string
  /** 地址栏要错误页签（还要看管理员是否允许用户看错误） */
  wantsErrorTab: boolean
}

/** 读地址栏：时间范围（start / end）、密钥（key）、模型（model）、页签（tab） */
export function readUsageRoute(query: LocationQuery): UsageRouteState {
  const defaultRange = last24HoursRange()
  const start = queryString(query, 'start')
  const end = queryString(query, 'end')
  const hasRange = DATE_RE.test(start) && DATE_RE.test(end) && start <= end
  const keyId = Number(queryString(query, 'key'))
  const preset = hasRange ? null : LAST_24_HOURS_PRESET
  return {
    startDate: hasRange ? start : defaultRange.start,
    endDate: hasRange ? end : defaultRange.end,
    preset,
    timeWindow: windowForPreset(preset),
    apiKeyId: Number.isInteger(keyId) && keyId > 0 ? keyId : undefined,
    model: queryString(query, 'model').trim(),
    wantsErrorTab: queryString(query, 'tab') === 'errors'
  }
}

/** 维度筛选（密钥 / 模型 / 类型……）；时间范围不放这里，发请求时由 usageRangeParams 给 */
export function initialUsageFilters(state: UsageRouteState): UsageQueryParams {
  return {
    api_key_id: state.apiKeyId,
    model: state.model || undefined,
    request_type: undefined,
    native_compaction_v2: null,
    billing_type: null,
    billing_mode: null
  }
}

/** 页面状态里的时间范围 → 接口参数（近 24 小时给精确时刻，其余给日期） */
export function usageRangeParams(state: Pick<UsageRouteState, 'startDate' | 'endDate' | 'timeWindow'>): RangeParams {
  return rangeParams(state.startDate, state.endDate, state.timeWindow)
}

/** 统计 / 分布 / 列表共用的筛选：时间范围取当前值，请求类型折成旧的 stream 参数 */
export function normalizeUsageFilters(filters: UsageQueryParams, range: RangeParams): UsageQueryParams {
  const requestType = filters.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.stream
  return {
    ...filters,
    ...range,
    stream: legacyStream === null ? undefined : legacyStream
  }
}

export interface UsageSort {
  sort_by: string
  sort_order: 'asc' | 'desc'
}

export const USAGE_DEFAULT_SORT: UsageSort = { sort_by: 'created_at', sort_order: 'desc' }

export function usageListParams(normalized: UsageQueryParams, page: number, pageSize: number, sort: UsageSort): UsageQueryParams {
  return { page, page_size: pageSize, ...normalized, sort_by: sort.sort_by, sort_order: sort.sort_order }
}

export function usageModelStatsParams(normalized: UsageQueryParams) {
  return { ...normalized, model_source: 'requested' as const }
}

/** 摘要里的失败请求数：同一时间范围、同一密钥 / 模型筛选，只要 total */
export function usageErrorCountParams(range: RangeParams, filters: UsageQueryParams): UserErrorListParams {
  return {
    page: 1,
    page_size: 1,
    ...range,
    api_key_id: filters.api_key_id ?? undefined,
    model: filters.model || undefined
  }
}

export interface UsageErrorFilter {
  model: string
  category: string
  api_key_id: number | null
  status_code: number | null
}

export const EMPTY_ERROR_FILTER: UsageErrorFilter = { model: '', category: '', api_key_id: null, status_code: null }
export const ERROR_PAGE_SIZE = 20

export function usageErrorListParams(opts: {
  page: number
  pageSize: number
  range: RangeParams
  filter: UsageErrorFilter
  sort: UsageSort
}): UserErrorListParams {
  return {
    page: opts.page,
    page_size: opts.pageSize,
    ...opts.range,
    model: opts.filter.model.trim() || undefined,
    category: opts.filter.category || undefined,
    api_key_id: opts.filter.api_key_id ?? undefined,
    status_code: opts.filter.status_code ?? undefined,
    sort_by: opts.sort.sort_by,
    sort_order: opts.sort.sort_order
  }
}

/** 各请求的预加载键 */
export const usageRequestKey = {
  logs: (params: UsageQueryParams) => preloadKey('usage/logs', params),
  stats: (params: UsageQueryParams) => preloadKey('usage/stats', params),
  modelStats: (params: ReturnType<typeof usageModelStatsParams>) => preloadKey('usage/model-stats', params),
  errors: (params: UserErrorListParams) => preloadKey('usage/errors', params)
}

/** 进入用量明细前：按地址栏发出首屏的列表、统计、费用分布、失败数（和错误页签的列表） */
export function preloadUsage(to: RouteLocationNormalized, prefetch: Prefetch): void {
  const state = readUsageRoute(to.query)
  const filters = initialUsageFilters(state)
  const range = usageRangeParams(state)
  const normalized = normalizeUsageFilters(filters, range)

  const listParams = usageListParams(normalized, 1, getPersistedPageSize(), USAGE_DEFAULT_SORT)
  prefetch(usageRequestKey.logs(listParams), () => usageAPI.query(listParams))
  prefetch(usageRequestKey.stats(normalized), () => usageAPI.getStats(normalized))
  const modelParams = usageModelStatsParams(normalized)
  prefetch(usageRequestKey.modelStats(modelParams), () => usageAPI.getDashboardModels(modelParams))

  if (!(useAppStore().cachedPublicSettings?.allow_user_view_error_requests ?? false)) return
  const countParams = usageErrorCountParams(range, filters)
  prefetch(usageRequestKey.errors(countParams), () => usageAPI.listMyErrorRequests(countParams))
  if (state.wantsErrorTab) {
    const errorParams = usageErrorListParams({
      page: 1,
      pageSize: ERROR_PAGE_SIZE,
      range,
      filter: EMPTY_ERROR_FILTER,
      sort: USAGE_DEFAULT_SORT
    })
    prefetch(usageRequestKey.errors(errorParams), () => usageAPI.listMyErrorRequests(errorParams))
  }
}
