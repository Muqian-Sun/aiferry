/**
 * 顶部导航渲染测试：控制台五页签与权限 / 模式过滤，公开壳的登录 / 控制台入口，管理站不渲染产品页签。
 * 断言的是路径、data-tour 与文案，不断言样式类。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

const { appStore, authStore, siteFlags } = vi.hoisted(() => ({
  appStore: {
    siteName: 'AiFerry',
    siteLogo: '',
    backendModeEnabled: false,
    docUrl: '',
    cachedPublicSettings: {} as Record<string, unknown>,
    publicSettingsLoaded: true
  },
  authStore: { user: null as null | { id: number }, isAuthenticated: false, isSimpleMode: false },
  siteFlags: { adminSite: false }
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/app/site', () => ({
  get IS_ADMIN_SITE() {
    return siteFlags.adminSite
  },
  APP_SITE: 'user'
}))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: { value: false }, refreshBatchImageAccess: vi.fn() })
}))
vi.mock('@/composables/useTheme', () => ({
  useTheme: () => ({ isDark: { value: false }, toggleTheme: vi.fn() })
}))
vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/usage', fullPath: '/usage' }),
  useRouter: () => ({ push: vi.fn() })
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import SiteNav from '../SiteNav.vue'

const stubs = {
  RouterLink: RouterLinkStub,
  AnnouncementBell: { template: '<span data-testid="bell" />' },
  LocaleSwitcher: { template: '<span data-testid="locale" />' },
  UserMenu: { template: '<span data-testid="user-menu" />' },
  BalanceLink: { template: '<span data-testid="balance" />' },
  Icon: { template: '<i />' }
}

function mountNav(variant: 'public' | 'console') {
  return mount(SiteNav, { props: { variant }, global: { stubs } })
}

function linkPaths(wrapper: ReturnType<typeof mountNav>) {
  return wrapper.findAllComponents(RouterLinkStub).map((link) => link.props('to') as string)
}

beforeEach(() => {
  appStore.backendModeEnabled = false
  appStore.docUrl = ''
  appStore.cachedPublicSettings = {}
  authStore.user = null
  authStore.isAuthenticated = false
  authStore.isSimpleMode = false
  siteFlags.adminSite = false
})

describe('SiteNav console', () => {
  beforeEach(() => {
    authStore.user = { id: 1 }
    authStore.isAuthenticated = true
  })

  it('renders the five primary tabs (desktop + mobile rows) with the keys tour anchor and no version text', () => {
    const wrapper = mountNav('console')
    const paths = new Set(linkPaths(wrapper))
    expect([...paths]).toEqual(expect.arrayContaining(['/usage', '/keys', '/model-plaza', '/billing', '/profile']))
    expect(paths.has('/purchase')).toBe(false)
    expect(paths.has('/accounts')).toBe(false)
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('v1.2.3')
    expect(wrapper.find('[data-testid="balance"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="user-menu"]').exists()).toBe(true)
  })

  it('simple mode drops billing and models and hides the balance', () => {
    authStore.isSimpleMode = true
    const wrapper = mountNav('console')
    const paths = new Set(linkPaths(wrapper))
    expect(paths.has('/billing')).toBe(false)
    expect(paths.has('/model-plaza')).toBe(false)
    expect(paths.has('/keys')).toBe(true)
    expect(wrapper.find('[data-testid="balance"]').exists()).toBe(false)
  })

  it('backend mode renders no console tabs', () => {
    appStore.backendModeEnabled = true
    const wrapper = mountNav('console')
    expect(linkPaths(wrapper).filter((path) => path !== '/usage')).toEqual([])
  })

  it('puts admin-configured custom pages behind the more menu', async () => {
    appStore.cachedPublicSettings = {
      custom_menu_items: [{ id: 'docs', label: 'Docs page', icon_svg: '', url: '', visibility: 'user', sort_order: 1 }]
    }
    const wrapper = mountNav('console')
    expect(linkPaths(wrapper)).not.toContain('/custom/docs')
    await wrapper.find('[data-testid="nav-more"]').trigger('click')
    expect(linkPaths(wrapper)).toContain('/custom/docs')
  })
})

describe('SiteNav public', () => {
  it('offers sign in when logged out and the console when logged in', () => {
    expect(mountNav('public').find('[data-testid="nav-login"]').exists()).toBe(true)
    authStore.isAuthenticated = true
    authStore.user = { id: 1 }
    const wrapper = mountNav('public')
    expect(wrapper.find('[data-testid="nav-console"]').exists()).toBe(true)
    expect(wrapper.findComponent('[data-testid="nav-console"]').props('to')).toBe('/usage')
  })

  it('always shows the pricing tab to anonymous visitors (the plaza has no switch)', () => {
    appStore.cachedPublicSettings = {}
    expect(linkPaths(mountNav('public'))).toContain('/model-plaza')
  })

  it('renders only brand and utilities on the admin site', () => {
    siteFlags.adminSite = true
    const wrapper = mountNav('public')
    expect(linkPaths(wrapper)).toEqual(['/home'])
    expect(wrapper.find('[data-testid="nav-login"]').exists()).toBe(false)
  })
})
