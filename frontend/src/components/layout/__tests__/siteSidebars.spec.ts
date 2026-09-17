/**
 * 站点侧边栏渲染测试：两站各自只渲染本站导航，布局缺少站点注入时直接报错。
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

import UserSidebar from '../UserSidebar.vue'
import AdminSidebar from '../../admin/layout/AdminSidebar.vue'

const mountOptions = {
  global: {
    stubs: { RouterLink: RouterLinkStub, VersionBadge: { template: '<span data-testid="version-badge" />' } },
  },
}

function linkPaths(wrapper: ReturnType<typeof mount>) {
  return wrapper.findAllComponents(RouterLinkStub).map((link) => link.props('to'))
}

beforeEach(() => {
  appStore.backendModeEnabled = false
  authStore.isSimpleMode = false
  vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
})

describe('UserSidebar', () => {
  it('renders user navigation only, with the keys tour anchor and plain version text', () => {
    const wrapper = mount(UserSidebar, mountOptions)
    const paths = linkPaths(wrapper)
    expect(paths).toContain('/keys')
    expect(paths).toContain('/purchase')
    expect(paths).not.toContain('/accounts')
    expect(paths).not.toContain('/settings')
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('v1.2.3')
    expect(wrapper.find('[data-testid="version-badge"]').exists()).toBe(false)
  })

  it('shows no navigation in backend mode', () => {
    appStore.backendModeEnabled = true
    const paths = linkPaths(mount(UserSidebar, mountOptions)).filter((path) => path !== '/dashboard')
    expect(paths).toEqual([])
  })
})

describe('AdminSidebar', () => {
  it('renders admin navigation with tour anchors and account security, without user pages', () => {
    const wrapper = mount(AdminSidebar, mountOptions)
    const paths = linkPaths(wrapper)
    expect(paths).toContain('/accounts')
    expect(paths).toContain('/settings')
    expect(paths).toContain('/profile')
    expect(paths).not.toContain('/keys')
    expect(paths).not.toContain('/purchase')
    expect(wrapper.find('#sidebar-channel-manage').exists()).toBe(true)
    expect(wrapper.find('#sidebar-group-manage').exists()).toBe(true)
    expect(wrapper.find('[data-testid="version-badge"]').exists()).toBe(true)
    expect(adminSettingsStore.fetch).toHaveBeenCalled()
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
