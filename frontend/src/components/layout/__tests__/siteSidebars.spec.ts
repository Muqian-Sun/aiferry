/**
 * 管理后台侧边栏渲染测试与共享布局的注入守卫。用户站已改为顶部导航（见 user/shell/__tests__/SiteNav.spec.ts）。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

const { appStore, authStore, adminSettingsStore } = vi.hoisted(() => ({
  appStore: {
    siteName: 'Site',
    siteLogo: '',
    siteVersion: '1.2.3',
    publicSettingsLoaded: true,
    backendModeEnabled: false,
    cachedPublicSettings: {} as Record<string, unknown>,
    sidebarCollapsed: false,
    mobileOpen: false,
    sidebarScrollTop: 0,
    toggleSidebar: vi.fn(),
    setMobileOpen: vi.fn(),
  },
  authStore: { isSimpleMode: false, isAdmin: false },
  adminSettingsStore: {
    opsMonitoringEnabled: true,
    paymentEnabled: true,
    fetch: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => adminSettingsStore }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/dashboard' }),
  useRouter: () => ({ push: vi.fn() }),
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import AdminSidebar from '../../admin/layout/AdminSidebar.vue'
import SidebarFrame from '../sidebar/SidebarFrame.vue'
import type { NavSection } from '../sidebar/navTypes'

const mountOptions = {
  global: {
    stubs: { RouterLink: RouterLinkStub },
  },
}

function linkPaths(wrapper: ReturnType<typeof mount>) {
  return wrapper.findAllComponents(RouterLinkStub).map((link) => link.props('to'))
}

function sectionPaths(wrapper: ReturnType<typeof mount>): Record<string, string[]> {
  const sections = wrapper.findComponent(SidebarFrame).props('sections') as NavSection[]
  return Object.fromEntries(sections.map((section) => [section.key, section.items.map((item) => item.path)]))
}

beforeEach(() => {
  appStore.backendModeEnabled = false
  appStore.cachedPublicSettings = {}
  authStore.isSimpleMode = false
  vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
})

describe('AdminSidebar', () => {
  it('renders admin navigation without user pages; account security lives in the header menu', () => {
    const wrapper = mount(AdminSidebar, mountOptions)
    const paths = linkPaths(wrapper)
    expect(paths).toContain('/settings')
    expect(paths).not.toContain('/profile')
    expect(paths).not.toContain('/keys')
    expect(paths).not.toContain('/purchase')
    expect(adminSettingsStore.fetch).toHaveBeenCalled()
  })

  // A3 导航（渠道在前）：概览 / 供给 / 用户 / 运营 / 安全，最后是不带标题的设置；
  // 订阅、订单、审查各一个入口，同组页面走页头页签。
  it('groups the navigation into five titled sections plus settings', () => {
    appStore.cachedPublicSettings = { risk_control_enabled: true }
    const wrapper = mount(AdminSidebar, mountOptions)
    const sections = wrapper.findComponent(SidebarFrame).props('sections') as NavSection[]
    expect(sections.map((section) => section.key)).toEqual(['overview', 'supply', 'users', 'operations', 'security', 'settings'])
    expect(sections.filter((section) => section.key !== 'settings').every((section) => section.title)).toBe(true)
    expect(sections.find((section) => section.key === 'settings')?.title).toBeUndefined()
    expect(sectionPaths(wrapper)).toEqual({
      overview: ['/dashboard', '/ops'],
      supply: ['/accounts', '/model-catalog', '/proxies'],
      users: ['/users', '/orders'],
      operations: ['/usage', '/announcements'],
      security: ['/risk-control', '/audit-logs'],
      settings: ['/settings'],
    })
  })

  it('drops a whole section when every item in it is hidden', () => {
    authStore.isSimpleMode = true
    const wrapper = mount(AdminSidebar, mountOptions)
    const paths = sectionPaths(wrapper)
    // 简易模式：用户 / 订阅 / 订单 / 兑换码全部收起 → 「用户」整组消失
    expect(Object.keys(paths)).not.toContain('users')
    expect(paths.security).toEqual(['/risk-control'])
  })

  it('does not offer API keys to administrators in simple mode', () => {
    authStore.isSimpleMode = true
    expect(linkPaths(mount(AdminSidebar, mountOptions))).not.toContain('/keys')
  })
})

describe('AppLayout', () => {
  it('fails loudly when the site root did not provide a layout', async () => {
    const { default: AppLayout } = await import('../AppLayout.vue')
    const Host = defineComponent({ render: () => h(AppLayout) })
    expect(() => mount(Host, { global: { stubs: { AppHeader: true } } })).toThrow('AppLayout requires a site layout')
  })
})
