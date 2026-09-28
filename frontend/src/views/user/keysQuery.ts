/**
 * 密钥页：地址栏参数 → 列表请求参数，以及进入页面前的预加载。页面（KeysView）和预加载（preloadKeys）共用这一份，
 * 两边发出的请求同名同参，预加载的结果才接得上（router/routePreload.ts）。
 */
import type { LocationQuery, RouteLocationNormalized } from 'vue-router'
import { authAPI, keysAPI, usageAPI } from '@/api'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { preloadKey, type Prefetch } from '@/router/routePreload'

/** 后端能筛的状态（「限额将满 / 即将到期」不在这里：要在全部密钥里挑，见 KeysView） */
export const KEY_STATUSES = ['active', 'inactive', 'quota_exhausted', 'expired'] as const

export interface KeysSort {
  sort_by: string
  sort_order: 'asc' | 'desc'
}

export const KEYS_DEFAULT_SORT: KeysSort = { sort_by: 'created_at', sort_order: 'desc' }

export interface KeysListFilters {
  search?: string
  status?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

/** 概览「需要处理」带过来的 ?status=：只认后端能筛的状态 */
export function keysStatusFromQuery(query: LocationQuery): string {
  const status = query.status
  return typeof status === 'string' && (KEY_STATUSES as readonly string[]).includes(status) ? status : ''
}

export function keysListFilters(search: string, status: string, sort: KeysSort): KeysListFilters {
  const filters: KeysListFilters = {}
  if (search) filters.search = search
  if (status) filters.status = status
  filters.sort_by = sort.sort_by
  filters.sort_order = sort.sort_order
  return filters
}

/** 用量一次最多查 100 把 */
export function keysUsageIds(ids: number[]): number[] {
  return ids.slice(0, 100)
}

/** 各请求的预加载键 */
export const keysRequestKey = {
  list: (page: number, pageSize: number, filters: KeysListFilters) => preloadKey('keys/list', { page, pageSize, filters }),
  usage: (ids: number[]) => preloadKey('keys/usage', ids),
  publicSettings: preloadKey('settings/public')
}

/** 进入密钥页前：第一页列表、这一页的用量、公开设置（接口地址） */
export async function preloadKeys(to: RouteLocationNormalized, prefetch: Prefetch): Promise<void> {
  prefetch(keysRequestKey.publicSettings, () => authAPI.getPublicSettings())
  const pageSize = getPersistedPageSize()
  const filters = keysListFilters('', keysStatusFromQuery(to.query), KEYS_DEFAULT_SORT)
  const list = await prefetch(keysRequestKey.list(1, pageSize, filters), () => keysAPI.list(1, pageSize, filters))
  const ids = keysUsageIds(list.items.map((key) => key.id))
  if (ids.length > 0) prefetch(keysRequestKey.usage(ids), () => usageAPI.getDashboardApiKeysUsage(ids))
}
