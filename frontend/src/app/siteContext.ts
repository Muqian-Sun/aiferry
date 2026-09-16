import type { Router } from 'vue-router'
import type { CustomMenuItem } from '@/types'

/**
 * 站点上下文：由各站点入口在挂载前设置，供共享模块（i18n、根组件标题、自定义页面）
 * 读取当前站点的路由与自定义菜单。共享模块经由这里拿站点专属数据，
 * 自身不引用任何站点专属模块，用户站产物因此不会带上管理端代码。
 */
export interface SiteContext {
  router: Router
  /** 当前站点可见的自定义菜单项（用户站为公开项，管理后台另含管理员项）。 */
  getCustomMenuItems: () => CustomMenuItem[]
}

let current: SiteContext | null = null

export function setSiteContext(context: SiteContext): void {
  current = context
}

export function getSiteContext(): SiteContext {
  if (!current) {
    throw new Error('site context is not initialized; call setSiteContext in the app entry before mounting')
  }
  return current
}
