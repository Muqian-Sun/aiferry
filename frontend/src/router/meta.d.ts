/**
 * Type definitions for Vue Router meta fields
 * Extends the RouteMeta interface with custom properties
 */

import 'vue-router'
import type { RouteLocationNormalized } from 'vue-router'
import type { Prefetch } from './routePreload'
import type { SiteFeature } from '@/utils/siteFeatures'

declare module 'vue-router' {
  interface RouteMeta {
    /**
     * Whether this route requires authentication
     * @default true
     */
    requiresAuth?: boolean

    /**
     * Whether this route requires admin role
     * @default false
     */
    requiresAdmin?: boolean

    /**
     * Page title for this route
     */
    title?: string

    /**
     * Optional breadcrumb items for navigation
     */
    breadcrumbs?: Array<{
      label: string
      to?: string
    }>

    /**
     * Icon name for this route (for sidebar navigation)
     */
    icon?: string

    /**
     * Whether to hide this route from navigation menu
     * @default false
     */
    hideInMenu?: boolean

    /**
     * Whether this route requires internal payment system to be enabled
     * @default false
     */
    requiresPayment?: boolean

    /**
     * 找回 / 重置密码页：公开设置里 password_reset_enabled 为 false 时拦回登录页
     */
    requiresPasswordReset?: boolean

    /**
     * 是否要求风控中心功能开关已启用
     * @default false
     */
    requiresRiskControl?: boolean

    /**
     * 属于哪个由代码决定的功能（utils/siteFeatures.ts）；该功能关着时拦回首页
     */
    siteFeature?: SiteFeature

    /**
     * 进入该页面前预加载首屏数据（router/routePreload.ts）：用传进来的 prefetch 登记请求，导航等请求落地再切页
     */
    preload?: (to: RouteLocationNormalized, prefetch: Prefetch) => void | Promise<void>


    /**
     * i18n key for the page title
     */
    titleKey?: string

    /**
     * 页签标题把站名放在前面（「站名 - 标题」），首页用：「AiFerry - 一把 Key，摆渡全球大模型」
     * @default false
     */
    titleBrandFirst?: boolean

    /**
     * i18n key for the page description
     */
    descriptionKey?: string

    /**
     * 管理站：同组页面共用组标题 + 页签（见 components/admin/layout/adminPageGroups.ts）
     */
    pageGroup?: 'subscriptions' | 'orders' | 'review'

    /**
     * 管理站：页面自己画标题（运维监控有全屏模式，标题在它自己的工具条里），内容区不再放页头
     * @default false
     */
    hidePageHeader?: boolean
  }
}
