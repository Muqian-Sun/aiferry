import { APP_SITE, type AppSite } from '@/app/site'

export const ADMIN_UI_REQUEST_HEADER = 'X-Admin-UI-Request'
export const USER_UI_REQUEST_HEADER = 'X-User-UI-Request'

/** 管理 API 路径（相对 /api/v1 或绝对形式）。页面路径不再参与判断：管理后台是独立站点。 */
function isAdminAPIPath(path: string): boolean {
  return (
    path === '/admin' ||
    path.startsWith('/admin/') ||
    path === '/api/v1/admin' ||
    path.startsWith('/api/v1/admin/')
  )
}

function requestPath(rawURL: string): string {
  const value = rawURL.trim()
  if (!value) return ''
  try {
    const origin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
    return new URL(value, origin).pathname
  } catch {
    return value.split(/[?#]/, 1)[0]
  }
}

/** Normalize Axios relative paths and absolute API paths to a comparable form. */
function normalizeAPIPath(path: string): string {
  const raw = requestPath(path)
  if (!raw) return ''
  if (raw === '/api/v1' || raw.startsWith('/api/v1/')) {
    return raw.slice('/api/v1'.length) || '/'
  }
  if (raw.startsWith('/')) {
    return raw
  }
  return `/${raw}`
}

/**
 * User-facing web APIs that may emit Server-Timing when ENABLE_SERVER_TIMING is on.
 * Mirrors backend isUserTimingPath allowlist (excluding public payment surfaces).
 */
export function isUserTimingAPIPath(requestURL: string): boolean {
  const path = normalizeAPIPath(requestURL)
  if (!path) return false

  if (
    path === '/auth/me' ||
    path === '/auth/revoke-all-sessions' ||
    path === '/auth/oauth/bind-token'
  ) {
    return true
  }
  if (path === '/user' || path.startsWith('/user/')) return true
  if (path === '/keys' || path.startsWith('/keys/')) return true
  if (path === '/groups/available' || path === '/groups/rates') return true
  if (path === '/channels/available') return true
  if (path === '/usage' || path.startsWith('/usage/')) return true
  if (path === '/announcements' || path.startsWith('/announcements/')) return true
  if (path === '/redeem' || path.startsWith('/redeem/')) return true
  if (path === '/subscriptions' || path.startsWith('/subscriptions/')) return true
  if (path === '/channel-monitors' || path.startsWith('/channel-monitors/')) return true
  if (path.startsWith('/payment/')) {
    if (path.startsWith('/payment/public') || path.startsWith('/payment/webhook')) {
      return false
    }
    return true
  }
  return false
}

/**
 * 管理界面发出的请求打上管理 UI 标记（Server-Timing 范围信号，不参与鉴权）。
 * 管理后台是独立站点，其页面发出的所有请求都属于管理界面；用户站只标记直接访问管理 API 的请求。
 */
export function shouldMarkAdminUIRequest(requestURL: string, site: AppSite = APP_SITE): boolean {
  return site === 'admin' || isAdminAPIPath(requestPath(requestURL))
}

export function shouldMarkUserUIRequest(requestURL: string): boolean {
  return isUserTimingAPIPath(requestURL)
}
