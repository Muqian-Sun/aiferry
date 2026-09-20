import { describe, expect, it } from 'vitest'
import type { CustomMenuItem } from '@/types'
import { buildConsoleNav, buildPublicNav, isTabActive } from '../navItems'

const t = (key: string) => key
const base = {
  t,
  simpleMode: false,
  backendMode: false,
  modelPlazaEnabled: true as boolean | undefined,
  batchImageEnabled: false,
  customItems: [] as CustomMenuItem[]
}

function custom(id: string, sort_order: number, visibility: 'user' | 'admin' = 'user'): CustomMenuItem {
  return { id, label: `Page ${id}`, icon_svg: '<svg/>', url: '', visibility, sort_order }
}

describe('buildConsoleNav', () => {
  it('has the five primary tabs in order for a standard user', () => {
    const { tabs, more } = buildConsoleNav(base)
    expect(tabs.map((tab) => tab.path)).toEqual(['/usage', '/keys', '/model-plaza', '/billing', '/profile'])
    expect(more).toEqual([])
  })

  it('keeps the onboarding tour hook on the keys tab', () => {
    const { tabs } = buildConsoleNav(base)
    expect(tabs.find((tab) => tab.path === '/keys')?.dataTour).toBe('sidebar-my-keys')
  })

  it('simple mode hides billing, models and batch images', () => {
    const { tabs } = buildConsoleNav({ ...base, simpleMode: true, batchImageEnabled: true })
    expect(tabs.map((tab) => tab.path)).toEqual(['/usage', '/keys', '/profile'])
  })

  it('backend mode has no console tabs at all', () => {
    expect(buildConsoleNav({ ...base, backendMode: true, customItems: [custom('a', 1)] })).toEqual({ tabs: [], more: [] })
  })

  it('model plaza follows the feature flag with lenient undefined', () => {
    expect(buildConsoleNav({ ...base, modelPlazaEnabled: false }).tabs.map((tab) => tab.path)).not.toContain('/model-plaza')
    expect(buildConsoleNav({ ...base, modelPlazaEnabled: undefined }).tabs.map((tab) => tab.path)).toContain('/model-plaza')
  })

  it('appends batch images only when the user has access', () => {
    expect(buildConsoleNav({ ...base, batchImageEnabled: true }).tabs.at(-1)?.path).toBe('/batch-image')
  })

  it('puts user-visible custom pages into "more", sorted, admin-only ones dropped', () => {
    const { more } = buildConsoleNav({
      ...base,
      customItems: [custom('b', 2), custom('hidden', 0, 'admin'), custom('a', 1)]
    })
    expect(more.map((tab) => tab.path)).toEqual(['/custom/a', '/custom/b'])
    expect(more[0].iconSvg).toBe('<svg/>')
  })
})

describe('buildPublicNav', () => {
  it('lists product, pricing and docs for the user site', () => {
    const tabs = buildPublicNav({ t, adminSite: false, modelPlazaVisible: true, docUrl: 'https://docs.example' })
    expect(tabs.map((tab) => tab.path)).toEqual(['/home', '/model-plaza', 'https://docs.example'])
    expect(tabs[2].external).toBe(true)
  })

  it('drops pricing when the plaza is hidden and docs when unset', () => {
    expect(buildPublicNav({ t, adminSite: false, modelPlazaVisible: false, docUrl: '' }).map((tab) => tab.path)).toEqual(['/home'])
  })

  it('renders nothing on the admin site', () => {
    expect(buildPublicNav({ t, adminSite: true, modelPlazaVisible: true, docUrl: 'x' })).toEqual([])
  })
})

describe('isTabActive', () => {
  it('matches the path and its children, never external links', () => {
    expect(isTabActive({ path: '/billing', label: '' }, '/billing/orders')).toBe(true)
    expect(isTabActive({ path: '/billing', label: '' }, '/billingx')).toBe(false)
    expect(isTabActive({ path: 'https://docs', label: '', external: true }, 'https://docs')).toBe(false)
  })
})
