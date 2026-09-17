import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const read = (path: string) => readFileSync(resolve(here, path), 'utf8')
const frameSource = read('../sidebar/SidebarFrame.vue')
const userSidebarSource = read('../UserSidebar.vue')
const adminSidebarSource = read('../../admin/layout/AdminSidebar.vue')
const styleSource = read('../../../style.css')

describe('SidebarFrame custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(frameSource).toContain('.sidebar-svg-icon {')
    expect(frameSource).toContain('color: currentColor;')
    expect(frameSource).toContain('display: block;')
    expect(frameSource).not.toContain('stroke: currentColor;')
    expect(frameSource).not.toContain('fill: none;')
  })
})

describe('SidebarFrame scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(frameSource).toContain('ref="sidebarNavRef"')
    expect(frameSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(frameSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(frameSource).toContain('onBeforeUnmount')
    expect(frameSource).toContain('appStore.sidebarScrollTop')
    expect(frameSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(frameSource).toContain('onMounted')
    expect(frameSource).toContain('appStore.sidebarScrollTop')
    expect(frameSource).toContain('nextTick')
  })
})

describe('SidebarFrame collapsible groups', () => {
  it('lets the user collapse a group even while a child route is active', () => {
    // The expand state must come from the user's override first, falling back
    // to the active-route heuristic only when the user has not clicked yet.
    expect(frameSource).toContain('const groupExpandOverrides = ref<Map<string, boolean>>(new Map())')
    expect(frameSource).not.toContain('expandedGroups.value.has(item.path) || isGroupActive(item)')
  })
})

describe('SidebarFrame header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = frameSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('sidebar subscription feature flag', () => {
  it('gates the My Subscriptions entry behind the subscription public-settings flag', () => {
    expect(userSidebarSource).toContain('const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)')
    expect(userSidebarSource).toMatch(/path: '\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('also hides the admin Subscription Management entry on recharge-only sites', () => {
    expect(adminSidebarSource).toContain('const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)')
    expect(adminSidebarSource).toMatch(/path: '\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('derives the purchase entry label from the site billing mode', () => {
    expect(userSidebarSource).toContain("import { resolveSiteBillingMode } from '@/utils/siteBillingMode'")
    expect(userSidebarSource).toMatch(/case 'recharge_only':\s*return t\('nav\.recharge'\)/)
    expect(userSidebarSource).toMatch(/case 'subscription_only':\s*return t\('nav\.subscribe'\)/)
    expect(userSidebarSource).toMatch(/path: '\/purchase'[^\n]*label: purchaseNavLabel\.value/)
  })
})

describe('site sidebars stay separated', () => {
  it('user sidebar imports nothing from the admin console', () => {
    expect(userSidebarSource).not.toMatch(/from '@\/(stores\/admin|api\/admin|components\/admin)/)
  })
})
