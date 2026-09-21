import type { AppSite } from '@/app/site'

/**
 * 已登录用户的默认落点：用户站是用量页（概览已并入），管理后台是仪表盘。
 * 单独成文件是为了让认证页只依赖一个纯函数，不把守卫的 store / i18n 依赖链拖进去。
 */
export function defaultAuthedPath(site: AppSite): string {
  return site === 'user' ? '/usage' : '/dashboard'
}
