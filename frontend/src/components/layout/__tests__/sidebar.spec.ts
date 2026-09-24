import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const read = (path: string) => readFileSync(resolve(here, path), 'utf8')
const frameSource = read('../sidebar/SidebarFrame.vue')
// 用户站导航已改为顶部页签（components/user/shell），其行为测试见 user/shell/__tests__/navItems.spec.ts 与 SiteNav.spec.ts
const siteNavSource = read('../../user/shell/SiteNav.vue')

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

describe('site sidebars stay separated', () => {
  it('user-site top navigation imports nothing from the admin console', () => {
    expect(siteNavSource).not.toMatch(/from '@\/(stores\/admin|api\/admin|components\/admin)/)
  })
})
