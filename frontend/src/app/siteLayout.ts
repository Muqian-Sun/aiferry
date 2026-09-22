import type { Component, InjectionKey } from 'vue'

/**
 * 站点布局：侧边栏与新手引导存储键由站点根组件提供，共享的 AppLayout 只负责摆放。
 * 共享布局因此不引用任何站点专属组件，用户站产物不会带上管理端侧边栏及其依赖。
 */
export interface SiteLayout {
  sidebar: Component
}

export const SITE_LAYOUT: InjectionKey<SiteLayout> = Symbol('site-layout')
