/**
 * 用户站导航集合（纯函数，便于单测）。
 *
 * 控制台是左侧分组栏（muqian 2026-09-23 定）：
 * - 主组：概览（落地页）· 密钥 · 用量明细 · 批量生图（功能开着且有可用密钥时）
 * - 账务组：充值 / 订阅，按功能开关出现（由调用方按 billingTabs 算好传入）；两个都关着时整组不出现
 * - 账户组：基本信息 · 安全（功能开着时）· 通知（开了余额提醒时）
 * - 更多：管理员配置的自定义页
 * simple mode 下去掉账务 / 批量生图；backend mode 下没有任何控制台导航。
 * 顶栏在控制台与公开站是同一组页签（产品 / 模型与价格 / 服务状态 / 文档）。
 * 模型与服务状态不分登录与否，入口只在顶栏（muqian 2026-09-30），侧栏不再重复。
 */
import type { HistoryState } from 'vue-router'
import type { CustomMenuItem } from '@/types'

/** 侧栏条目用到的图标（Icon 组件的名字；收起成图标栏时靠它认页面） */
export type NavIcon =
  | 'home'
  | 'key'
  | 'chartBar'
  | 'sparkles'
  | 'creditCard'
  | 'badge'
  | 'gift'
  | 'user'
  | 'shield'
  | 'bell'
  | 'link'

export interface NavTab {
  path: string
  label: string
  /** 外链（文档）用 <a target=_blank>，不走 router。 */
  external?: boolean
  /** 新手引导定位属性，落在可见链接元素上。 */
  dataTour?: string
  /** 管理员自定义菜单的图标 svg（已在渲染处净化）。 */
  iconSvg?: string
  /** 侧栏图标（顶栏页签不用） */
  icon?: NavIcon
  /** 跳转时带进浏览记录的状态（控制台里的顶栏页签带 CONSOLE_SHELL_STATE，见 consoleShell.ts） */
  state?: HistoryState
}

export interface ConsoleNavSection {
  key: 'main' | 'billing' | 'account' | 'more'
  /** 组标题；主组没有标题 */
  label?: string
  items: NavTab[]
}

export interface ConsoleNavContext {
  t: (key: string) => string
  simpleMode: boolean
  backendMode: boolean
  batchImageEnabled: boolean
  /** 账户「安全」页是否显示（SITE_FEATURES.accountSecurity） */
  accountSecurityEnabled: boolean
  /** 账务子页（已按功能开关过滤，见 views/user/billing/billingTabs.ts） */
  billingItems: NavTab[]
  /** 管理员开了余额不足提醒时，账户组才有「通知」 */
  balanceNotifyEnabled: boolean
  customItems: CustomMenuItem[]
}

export const CONSOLE_HOME_PATH = '/dashboard'

export function buildConsoleNav(ctx: ConsoleNavContext): ConsoleNavSection[] {
  if (ctx.backendMode) return []

  const main: NavTab[] = [
    { path: CONSOLE_HOME_PATH, label: ctx.t('userUi.nav.overview'), icon: 'home' },
    { path: '/keys', label: ctx.t('userUi.nav.keys'), dataTour: 'sidebar-my-keys', icon: 'key' },
    { path: '/usage', label: ctx.t('userUi.nav.usage'), icon: 'chartBar' }
  ]
  // 「仅充值」模式下控制台只留概览 / 密钥 / 用量 / 账户
  if (!ctx.simpleMode && ctx.batchImageEnabled) main.push({ path: '/batch-image', label: ctx.t('userUi.nav.batchImage'), icon: 'sparkles' })

  const sections: ConsoleNavSection[] = [{ key: 'main', items: main }]
  if (!ctx.simpleMode && ctx.billingItems.length) {
    sections.push({ key: 'billing', label: ctx.t('userUi.nav.billing'), items: ctx.billingItems })
  }
  const account: NavTab[] = [{ path: '/profile', label: ctx.t('userUi.account.sections.profile'), icon: 'user' }]
  if (ctx.accountSecurityEnabled) {
    account.push({ path: '/profile/security', label: ctx.t('userUi.account.sections.security'), icon: 'shield' })
  }
  if (ctx.balanceNotifyEnabled) {
    account.push({ path: '/profile/notifications', label: ctx.t('userUi.account.sections.notifications'), icon: 'bell' })
  }
  sections.push({ key: 'account', label: ctx.t('userUi.nav.account'), items: account })

  const more = ctx.customItems
    .filter((item) => item.visibility === 'user')
    .slice()
    .sort((a, b) => a.sort_order - b.sort_order)
    .map((item): NavTab => ({ path: `/custom/${item.id}`, label: item.label, iconSvg: item.icon_svg, icon: 'link' }))
  if (more.length) sections.push({ key: 'more', label: ctx.t('userUi.nav.more'), items: more })

  return sections
}

export interface PublicNavContext {
  t: (key: string) => string
  /** 管理站也会渲染公开壳（法律文档、404），那里没有产品页签。 */
  adminSite: boolean
  docUrl: string
  /** 服务状态页（各模型可用率与首字延迟）跟着「渠道健康」功能开关（后端写死开着） */
  serviceStatusEnabled: boolean
}

export function buildPublicNav(ctx: PublicNavContext): NavTab[] {
  if (ctx.adminSite) return []
  const tabs: NavTab[] = [{ path: '/home', label: ctx.t('userUi.nav.product') }]
  tabs.push({ path: '/model-plaza', label: ctx.t('userUi.nav.pricing') })
  // 服务状态紧跟模型：先看有哪些模型，再看它们现在能不能用；未登录也能看
  if (ctx.serviceStatusEnabled) tabs.push({ path: '/status', label: ctx.t('userUi.nav.status') })
  if (ctx.docUrl) tabs.push({ path: ctx.docUrl, label: ctx.t('userUi.nav.docs'), external: true })
  return tabs
}

/** 当前路由是否属于某个页签（子路由与查询参数都算）。 */
export function isTabActive(tab: NavTab, currentPath: string): boolean {
  if (tab.external) return false
  return currentPath === tab.path || currentPath.startsWith(`${tab.path}/`)
}

/**
 * 一组页签里当前该亮哪一个：取匹配的最长路径。
 * 同组里既有 /profile 又有 /profile/security 时，在 /profile/security 上只亮后者。
 */
export function pickActivePath(tabs: NavTab[], currentPath: string): string | null {
  let best: string | null = null
  for (const tab of tabs) {
    if (isTabActive(tab, currentPath) && (best === null || tab.path.length > best.length)) best = tab.path
  }
  return best
}
