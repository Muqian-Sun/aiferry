/**
 * 用户站顶部导航的页签集合（纯函数，便于单测）。
 *
 * 控制台只有 5 个主页签：用量 · 密钥 · 模型 · 账务 · 账户；批量生图按权限追加；
 * 管理员配置的自定义页收进「更多」。simple mode 下去掉账务 / 模型 / 批量生图，
 * backend mode 下没有任何控制台页签（同旧侧栏的行为）。
 */
import type { CustomMenuItem } from '@/types'

export interface NavTab {
  path: string
  label: string
  /** 外链（文档）用 <a target=_blank>，不走 router。 */
  external?: boolean
  /** 新手引导定位属性，落在可见链接元素上。 */
  dataTour?: string
  /** 管理员自定义菜单的图标 svg（已在渲染处净化）。 */
  iconSvg?: string
}

export interface ConsoleNav {
  tabs: NavTab[]
  /** 「更多 ▾」下拉里的条目；为空则不渲染下拉。 */
  more: NavTab[]
}

export interface ConsoleNavContext {
  t: (key: string) => string
  simpleMode: boolean
  backendMode: boolean
  /** FeatureFlags.modelPlaza 的宽容语义：undefined（设置未加载）视为显示。 */
  modelPlazaEnabled: boolean | undefined
  batchImageEnabled: boolean
  customItems: CustomMenuItem[]
}

export const CONSOLE_HOME_PATH = '/usage'

export function buildConsoleNav(ctx: ConsoleNavContext): ConsoleNav {
  if (ctx.backendMode) return { tabs: [], more: [] }

  const tabs: NavTab[] = [
    { path: '/usage', label: ctx.t('userUi.nav.usage') },
    { path: '/keys', label: ctx.t('userUi.nav.keys'), dataTour: 'sidebar-my-keys' }
  ]
  if (!ctx.simpleMode && ctx.modelPlazaEnabled !== false) {
    tabs.push({ path: '/model-plaza', label: ctx.t('userUi.nav.models') })
  }
  if (!ctx.simpleMode) {
    tabs.push({ path: '/billing', label: ctx.t('userUi.nav.billing') })
  }
  tabs.push({ path: '/profile', label: ctx.t('userUi.nav.account') })
  if (!ctx.simpleMode && ctx.batchImageEnabled) {
    tabs.push({ path: '/batch-image', label: ctx.t('userUi.nav.batchImage') })
  }

  const more = ctx.customItems
    .filter((item) => item.visibility === 'user')
    .slice()
    .sort((a, b) => a.sort_order - b.sort_order)
    .map((item): NavTab => ({ path: `/custom/${item.id}`, label: item.label, iconSvg: item.icon_svg }))

  return { tabs, more }
}

export interface PublicNavContext {
  t: (key: string) => string
  /** 管理站也会渲染公开壳（法律文档、404），那里没有产品页签。 */
  adminSite: boolean
  modelPlazaVisible: boolean
  docUrl: string
}

export function buildPublicNav(ctx: PublicNavContext): NavTab[] {
  if (ctx.adminSite) return []
  const tabs: NavTab[] = [{ path: '/home', label: ctx.t('userUi.nav.product') }]
  if (ctx.modelPlazaVisible) tabs.push({ path: '/model-plaza', label: ctx.t('userUi.nav.pricing') })
  if (ctx.docUrl) tabs.push({ path: ctx.docUrl, label: ctx.t('userUi.nav.docs'), external: true })
  return tabs
}

/** 当前路由是否属于某个页签（子路由与查询参数都算）。 */
export function isTabActive(tab: NavTab, currentPath: string): boolean {
  if (tab.external) return false
  return currentPath === tab.path || currentPath.startsWith(`${tab.path}/`)
}
