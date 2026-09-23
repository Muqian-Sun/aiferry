import type { AppSite } from '@/app/site'

/**
 * 已登录用户的默认落点：用户站是概览（/dashboard，muqian 2026-09-23 新增），管理后台是仪表盘。
 * 单独成文件是为了让认证页只依赖一个纯函数，不把守卫的 store / i18n 依赖链拖进去。
 */
const DEFAULT_AUTHED_PATHS: Record<AppSite, string> = {
  user: '/dashboard',
  admin: '/dashboard'
}

export function defaultAuthedPath(site: AppSite): string {
  return DEFAULT_AUTHED_PATHS[site]
}
