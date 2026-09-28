/**
 * 功能开关类路由（支付 / 风控 / 订阅）的守卫行为，直接驱动 createSiteGuard。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { RouteLocationNormalized } from 'vue-router'
import type { AppSite } from '@/app/site'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: true,
  isAdmin: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  publicSettingsLoaded: false,
  cachedPublicSettings: null as null | {
    payment_enabled?: boolean
    risk_control_enabled?: boolean
  },
  fetchPublicSettings: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/api/setup', () => ({ getSetupStatus: vi.fn() }))

import { createSiteGuard } from '@/router/siteGuard'

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function runGuard(site: AppSite, meta: Record<string, unknown>, path: string) {
  // 两个站点各自只有一种角色，登录态按站点给出
  authStore.isAdmin = site === 'admin'
  const guard = createSiteGuard({ site, getCustomMenuItems: () => [] })
  const next = vi.fn()
  const to = { path, fullPath: path, name: 'FeatureRoute', params: {}, meta: { requiresAuth: true, ...meta } } as unknown as RouteLocationNormalized
  const navigation = guard(to, to, next)
  return { navigation, next }
}

describe('feature route guard', () => {
  beforeEach(() => {
    authStore.isAuthenticated = true
    authStore.isSimpleMode = false
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockReset()
  })

  it('waits for the first public-settings request before deciding payment access', async () => {
    const deferred = createDeferred<{ payment_enabled: boolean }>()
    appStore.fetchPublicSettings.mockImplementation(async () => {
      const settings = await deferred.promise
      appStore.cachedPublicSettings = settings
      appStore.publicSettingsLoaded = true
      return settings
    })

    const { navigation, next } = runGuard('user', { requiresPayment: true }, '/purchase')

    await vi.waitFor(() => expect(appStore.fetchPublicSettings).toHaveBeenCalledTimes(1))
    expect(next).not.toHaveBeenCalled()

    deferred.resolve({ payment_enabled: true })
    await navigation
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each<[string, AppSite, Record<string, unknown>, string]>([
    ['payment', 'user', { requiresPayment: true }, '/purchase'],
    ['risk control', 'admin', { requiresRiskControl: true }, '/risk-control'],
  ])('does not treat a failed %s settings load as explicitly disabled', async (_name, site, meta, path) => {
    appStore.fetchPublicSettings.mockResolvedValue(null)

    const { navigation, next } = runGuard(site, meta, path)
    await navigation

    expect(appStore.publicSettingsLoaded).toBe(false)
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each<[string, AppSite, Record<string, unknown>, Record<string, boolean>, string]>([
    ['payment on the user site', 'user', { requiresPayment: true }, { payment_enabled: false }, '/dashboard'],
    ['payment on the admin console', 'admin', { requiresPayment: true }, { payment_enabled: false }, '/dashboard'],
    ['risk control on the admin console', 'admin', { requiresRiskControl: true }, { risk_control_enabled: false }, '/settings'],
  ])('redirects when loaded settings explicitly disable %s', async (_name, site, meta, settings, target) => {
    appStore.cachedPublicSettings = settings
    appStore.publicSettingsLoaded = true

    const { navigation, next } = runGuard(site, meta, '/feature')
    await navigation

    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith(target)
  })
})

// 订阅显不显示由代码决定（SITE_FEATURES.subscription，现在是 false），与公开设置无关
describe('subscription route guard (hidden in code)', () => {
  beforeEach(() => {
    authStore.isSimpleMode = false
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = {}
    appStore.fetchPublicSettings.mockReset()
  })

  it.each<[AppSite]>([['user'], ['admin']])('sends %s-site subscription pages home', async (site) => {
    const { navigation, next } = runGuard(site, { siteFeature: 'subscription' }, '/subscriptions')
    await navigation

    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith('/dashboard')
  })
})
