import { describe, expect, it } from 'vitest'
import type { CustomMenuItem } from '@/types'
import { buildConsoleNav, buildPublicNav, isTabActive, type ConsoleNavSection, type NavTab } from '../navItems'

const t = (key: string) => key
const billing: NavTab[] = [
  { path: '/billing/recharge', label: 'recharge' },
  { path: '/billing/redeem', label: 'redeem' }
]
const base = {
  t,
  simpleMode: false,
  backendMode: false,
  batchImageEnabled: false,
  accountSecurityEnabled: true,
  billingItems: billing,
  balanceNotifyEnabled: false,
  customItems: [] as CustomMenuItem[]
}

function custom(id: string, sort_order: number, visibility: 'user' | 'admin' = 'user'): CustomMenuItem {
  return { id, label: `Page ${id}`, icon_svg: '<svg/>', url: '', visibility, sort_order }
}

const shape = (sections: ConsoleNavSection[]) => sections.map((section) => [section.key, section.items.map((item) => item.path)])

describe('buildConsoleNav', () => {
  it('groups the sidebar into main / billing / account for a standard user, landing on the overview', () => {
    expect(shape(buildConsoleNav(base))).toEqual([
      ['main', ['/dashboard', '/keys', '/usage', '/model-plaza']],
      ['billing', ['/billing/recharge', '/billing/redeem']],
      ['account', ['/profile', '/profile/security']]
    ])
  })

  it('keeps the onboarding tour hook on the keys item', () => {
    const [main] = buildConsoleNav(base)
    expect(main.items.find((item) => item.path === '/keys')?.dataTour).toBe('sidebar-my-keys')
  })

  it('simple mode hides billing, models and batch images', () => {
    expect(shape(buildConsoleNav({ ...base, simpleMode: true, batchImageEnabled: true }))).toEqual([
      ['main', ['/dashboard', '/keys', '/usage']],
      ['account', ['/profile', '/profile/security']]
    ])
  })

  it('drops the billing group when no billing page is enabled', () => {
    expect(buildConsoleNav({ ...base, billingItems: [] }).map((section) => section.key)).toEqual(['main', 'account'])
  })

  it('backend mode has no console navigation at all', () => {
    expect(buildConsoleNav({ ...base, backendMode: true, customItems: [custom('a', 1)] })).toEqual([])
  })

  it('appends batch images to the main group only when the user has access', () => {
    expect(buildConsoleNav({ ...base, batchImageEnabled: true })[0].items.at(-1)?.path).toBe('/batch-image')
    expect(buildConsoleNav(base)[0].items.some((item) => item.path === '/batch-image')).toBe(false)
  })

  it('puts user-visible custom pages into a trailing "more" group, sorted, admin-only ones dropped', () => {
    const sections = buildConsoleNav({
      ...base,
      customItems: [custom('b', 2), custom('hidden', 0, 'admin'), custom('a', 1)]
    })
    const more = sections.at(-1)!
    expect(more.key).toBe('more')
    expect(more.items.map((item) => item.path)).toEqual(['/custom/a', '/custom/b'])
    expect(more.items[0].iconSvg).toBe('<svg/>')
  })
})

describe('buildPublicNav', () => {
  it('lists product, pricing and docs for the user site', () => {
    const tabs = buildPublicNav({ t, adminSite: false, docUrl: 'https://docs.example' })
    expect(tabs.map((tab) => tab.path)).toEqual(['/home', '/model-plaza', 'https://docs.example'])
    expect(tabs[2].external).toBe(true)
  })

  it('always lists pricing and drops docs only when unset', () => {
    expect(buildPublicNav({ t, adminSite: false, docUrl: '' }).map((tab) => tab.path)).toEqual(['/home', '/model-plaza'])
  })

  it('renders nothing on the admin site', () => {
    expect(buildPublicNav({ t, adminSite: true, docUrl: 'x' })).toEqual([])
  })
})

describe('isTabActive', () => {
  it('matches the path and its children, never external links', () => {
    expect(isTabActive({ path: '/billing', label: '' }, '/billing/orders')).toBe(true)
    expect(isTabActive({ path: '/billing', label: '' }, '/billingx')).toBe(false)
    expect(isTabActive({ path: 'https://docs', label: '', external: true }, 'https://docs')).toBe(false)
  })
})
