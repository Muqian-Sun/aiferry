import type { SiteFeature } from '@/utils/siteFeatures'

/**
 * 同组页面（A3）：侧栏只留一个入口，页头显示组标题 + 页签。路由 meta.pageGroup 指向这里的键。
 * 页签挂了 siteFeature 的，该功能关着时不显示（与路由守卫同一个开关，utils/siteFeatures.ts）。
 */
export type AdminPageGroupKey = 'subscriptions' | 'orders' | 'review'

export interface AdminPageGroup {
  titleKey: string
  tabs: Array<{ path: string; labelKey: string; siteFeature?: SiteFeature }>
}

export const ADMIN_PAGE_GROUPS: Record<AdminPageGroupKey, AdminPageGroup> = {
  subscriptions: {
    titleKey: 'nav.subscriptions',
    tabs: [
      { path: '/subscriptions', labelKey: 'nav.tabs.subscriptions' },
      { path: '/orders/plans', labelKey: 'nav.tabs.plans' }
    ]
  },
  orders: {
    titleKey: 'nav.orders',
    tabs: [
      { path: '/orders', labelKey: 'nav.tabs.orders' },
      { path: '/orders/dashboard', labelKey: 'nav.tabs.collections' }
    ]
  },
  review: {
    titleKey: 'nav.review',
    tabs: [
      { path: '/risk-control', labelKey: 'nav.tabs.moderation' },
      { path: '/prompt-audit', labelKey: 'nav.tabs.prompts', siteFeature: 'promptAudit' }
    ]
  }
}
