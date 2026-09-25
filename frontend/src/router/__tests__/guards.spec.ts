/**
 * 站点导航守卫测试：直接驱动生产代码 createSiteGuard，不在测试里重写守卫逻辑。
 * （旧版本在测试内复制了一份守卫逻辑再断言副本，删掉生产守卫也能通过。）
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { RouteLocationNormalized } from 'vue-router'
import type { AppSite } from '@/app/site'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  publicSettingsLoaded: true,
  cachedPublicSettings: {} as Record<string, unknown>,
  fetchPublicSettings: vi.fn(),
}))

const setup = vi.hoisted(() => ({ needsSetup: true }))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/api/setup', () => ({
  getSetupStatus: vi.fn(async () => ({ needs_setup: setup.needsSetup })),
}))

import { createSiteGuard, defaultAuthedPath } from '@/router/siteGuard'

type Outcome = { redirect: unknown; allowed: boolean }

async function navigate(site: AppSite, path: string, meta: Record<string, unknown> = {}): Promise<Outcome> {
  const guard = createSiteGuard({ site, getCustomMenuItems: () => [] })
  const next = vi.fn()
  const to = { path, fullPath: path, name: undefined, params: {}, query: {}, hash: '', matched: [], redirectedFrom: undefined, meta } as unknown as RouteLocationNormalized
  await guard(to, to, next)
  expect(next).toHaveBeenCalledOnce()
  const arg = next.mock.calls[0][0]
  return { redirect: arg, allowed: arg === undefined }
}

const publicMeta = { requiresAuth: false }
const protectedMeta = { requiresAuth: true }

function signIn(role: 'user' | 'admin') {
  authStore.isAuthenticated = true
  authStore.isAdmin = role === 'admin'
}

beforeEach(() => {
  authStore.isAuthenticated = false
  authStore.isAdmin = false
  authStore.isSimpleMode = false
  authStore.hasPendingAuthSession = false
  appStore.backendModeEnabled = false
  appStore.publicSettingsLoaded = true
  appStore.cachedPublicSettings = {}
  setup.needsSetup = true
})

describe.each<AppSite>(['user', 'admin'])('%s 站点通用守卫', (site) => {
  const role = site === 'admin' ? 'admin' : 'user'
  const home = defaultAuthedPath(site)

  it('未登录访问受保护页面去登录页并带回跳地址', async () => {
    expect((await navigate(site, '/dashboard', protectedMeta)).redirect).toEqual({ path: '/login', query: { redirect: '/dashboard' } })
  })

  it('已登录访问登录页去本站仪表盘', async () => {
    signIn(role)
    expect((await navigate(site, '/login', publicMeta)).redirect).toBe(home)
  })

  it('已登录访问受保护页面放行', async () => {
    signIn(role)
    expect((await navigate(site, '/dashboard', protectedMeta)).allowed).toBe(true)
  })

  it('本地残留另一站点的登录态：受保护页面回登录页，登录页本身放行（不能形成循环）', async () => {
    signIn(site === 'admin' ? 'user' : 'admin')
    expect((await navigate(site, '/dashboard', protectedMeta)).redirect).toBe('/login')
    expect((await navigate(site, '/login', publicMeta)).allowed).toBe(true)
  })

  it('简易模式隐藏订阅页面', async () => {
    signIn(role)
    authStore.isSimpleMode = true
    expect((await navigate(site, '/subscriptions', protectedMeta)).redirect).toBe(home)
    expect((await navigate(site, '/billing/orders', protectedMeta)).redirect).toBe(home)
    expect((await navigate(site, '/dashboard', protectedMeta)).allowed).toBe(true)
  })
})

describe('Backend mode 只作用于用户站', () => {
  beforeEach(() => {
    appStore.backendModeEnabled = true
  })

  it('用户站：未登录访问首页去登录页，登录页与 key 用量页放行', async () => {
    expect((await navigate('user', '/home', publicMeta)).redirect).toBe('/login')
    expect((await navigate('user', '/login', publicMeta)).allowed).toBe(true)
    expect((await navigate('user', '/key-usage', publicMeta)).allowed).toBe(true)
  })

  it('用户站：回调页放行；注册页仅在有待完成的第三方登录会话时放行', async () => {
    expect((await navigate('user', '/auth/wechat/payment/callback', publicMeta)).allowed).toBe(true)
    expect((await navigate('user', '/register', publicMeta)).redirect).toBe('/login')
    authStore.hasPendingAuthSession = true
    expect((await navigate('user', '/register', publicMeta)).allowed).toBe(true)
  })

  it('用户站：已登录普通用户被挡在受保护页面外，停留在登录页不循环', async () => {
    signIn('user')
    expect((await navigate('user', '/dashboard', protectedMeta)).redirect).toBe('/login')
    expect((await navigate('user', '/login', publicMeta)).allowed).toBe(true)
  })

  it('管理后台不受影响', async () => {
    signIn('admin')
    expect((await navigate('admin', '/dashboard', protectedMeta)).allowed).toBe(true)
    expect((await navigate('admin', '/login', publicMeta)).redirect).toBe('/dashboard')
  })
})

describe('安装向导', () => {
  it('尚未安装时放行', async () => {
    expect((await navigate('admin', '/setup', publicMeta)).allowed).toBe(true)
  })

  it('已安装时：未登录去登录页，已登录去仪表盘', async () => {
    setup.needsSetup = false
    expect((await navigate('admin', '/setup', publicMeta)).redirect).toBe('/login')
    signIn('admin')
    expect((await navigate('admin', '/setup', publicMeta)).redirect).toBe('/dashboard')
  })
})
