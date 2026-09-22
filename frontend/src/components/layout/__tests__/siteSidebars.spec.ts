/**
 * 管理后台侧边栏渲染测试与共享布局的注入守卫。用户站已改为顶部导航（见 user/shell/__tests__/SiteNav.spec.ts）。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { defineComponent, h } from 'vue'

const { appStore, authStore, adminSettingsStore, onboardingStore } = vi.hoisted(() => ({
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
    customMenuItems: [] as unknown[],
    opsMonitoringEnabled: true,
    paymentEnabled: true,
    fetch: vi.fn(),
  },
  onboardingStore: { isCurrentStep: vi.fn(() => false), nextStep: vi.fn(), setReplayCallback: vi.fn() },
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => adminSettingsStore }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => onboardingStore }))
vi.mock('@/stores/adminVersion', () => ({
  useAdminVersionStore: () => ({ fetchVersion: vi.fn(), clearVersionCache: vi.fn(), currentVersion: '', hasUpdate: false }),
}))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: { value: true }, refreshBatchImageAccess: vi.fn() }),
}))
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
import type { NavItem, NavSection } from '../sidebar/navTypes'

const mountOptions = {
  global: {
    stubs: { RouterLink: RouterLinkStub, VersionBadge: { template: '<span data-testid="version-badge" />' } },
  },
}

function linkPaths(wrapper: ReturnType<typeof mount>) {
  return wrapper.findAllComponents(RouterLinkStub).map((link) => link.props('to'))
}

/** 导航树（含未展开的子项）：折叠组的子链接不渲染，归属关系要看 SidebarFrame 收到的 sections。 */
function navItems(wrapper: ReturnType<typeof mount>): NavItem[] {
  const sections = wrapper.findComponent(SidebarFrame).props('sections') as NavSection[]
  return sections.flatMap((section) => section.items)
}

function childPaths(items: NavItem[], groupPath: string): string[] {
  const group = items.find((item) => item.path === groupPath)
  return (group?.children ?? []).map((child) => child.path)
}

function allPaths(items: NavItem[]): string[] {
  return items.flatMap((item) => [item.path, ...(item.children ?? []).map((child) => child.path)])
}

beforeEach(() => {
  appStore.backendModeEnabled = false
  authStore.isSimpleMode = false
  vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
})

describe('AdminSidebar', () => {
  it('renders admin navigation with tour anchors and account security, without user pages', () => {
    const wrapper = mount(AdminSidebar, mountOptions)
    const paths = linkPaths(wrapper)
    expect(paths).toContain('/settings')
    expect(paths).toContain('/profile')
    expect(paths).not.toContain('/keys')
    expect(paths).not.toContain('/purchase')
    expect(wrapper.find('[data-testid="version-badge"]').exists()).toBe(true)
    expect(adminSettingsStore.fetch).toHaveBeenCalled()
  })

  // 信息架构（PR-6a）：分组不再是导航概念；渠道页挂在「渠道管理」下；套餐属于「订阅」而不是「订单」。
  it('groups channels under channel management and plans under subscription, without a groups entry', () => {
    const wrapper = mount(AdminSidebar, mountOptions)
    const items = navItems(wrapper)
    expect(allPaths(items)).not.toContain('/groups')
    expect(childPaths(items, '/channels')).toEqual(['/accounts', '/model-catalog', '/channels/monitor'])
    expect(childPaths(items, '/subscriptions')).toEqual(['/subscriptions', '/orders/plans'])
    expect(childPaths(items, '/orders')).not.toContain('/orders/plans')
    expect(items.some((item) => item.path === '/accounts')).toBe(false)
    const channels = items.find((item) => item.path === '/channels')
    expect(channels?.children?.[0]?.elementId).toBe('sidebar-channel-manage')
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
