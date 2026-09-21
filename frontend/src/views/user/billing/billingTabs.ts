/**
 * 账务页签集合：按功能开关出现，与子路由的 requiresPayment / requiresSubscription 守卫一致。
 * 路由表的 /billing 索引 redirect 与 BillingView 的页签共用这一份，避免两处漂移。
 */
import type { SectionTab } from '@/components/user/shell/types'

export interface BillingFlags {
  payment: boolean
  subscription: boolean
  affiliate: boolean
}

interface BillingTabDef {
  key: string
  path: string
  labelKey: string
  visible: (flags: BillingFlags) => boolean
}

const BILLING_TABS: BillingTabDef[] = [
  { key: 'recharge', path: '/billing/recharge', labelKey: 'userUi.billing.tabs.recharge', visible: (f) => f.payment },
  { key: 'subscriptions', path: '/billing/subscriptions', labelKey: 'userUi.billing.tabs.subscriptions', visible: (f) => f.subscription },
  { key: 'orders', path: '/billing/orders', labelKey: 'userUi.billing.tabs.orders', visible: (f) => f.payment },
  { key: 'redeem', path: '/billing/redeem', labelKey: 'redeem.title', visible: () => true },
  { key: 'affiliate', path: '/billing/affiliate', labelKey: 'userUi.billing.tabs.affiliate', visible: (f) => f.affiliate }
]

export function buildBillingTabs(flags: BillingFlags, t: (key: string) => string): SectionTab[] {
  return BILLING_TABS.filter((tab) => tab.visible(flags)).map((tab) => ({ key: tab.key, label: t(tab.labelKey), to: tab.path }))
}

/** /billing 索引落点：第一个可见页签（兑换码永远在，所以总有落点）。 */
export function firstBillingPath(flags: BillingFlags): string {
  return BILLING_TABS.find((tab) => tab.visible(flags))!.path
}
