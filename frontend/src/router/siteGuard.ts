/**
 * 站点导航守卫。用户站与管理后台共用一套守卫，按站点区分差异：
 * - 两个站点各自只有一种角色（后端签发与鉴权都按站点拦截），已登录访问登录页一律去本站默认落点（defaultAuthedPath）
 * - Backend mode 只作用于用户站；管理后台不受影响
 * - 管理端专属的前置检查（合规确认）由管理后台入口通过 beforeProtectedRoute 注入
 */
import type { NavigationGuardNext, RouteLocationNormalized } from 'vue-router'
import type { AppSite } from '@/app/site'
import type { CustomMenuItem } from '@/types'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { getSetupStatus } from '@/api/setup'
import { resolveCompletedSetupRedirectPath } from './setupRedirect'
import { resolveRouteDocumentTitle } from './title'
import { defaultAuthedPath } from './defaultAuthedPath'

export { defaultAuthedPath }

export interface SiteGuardOptions {
  site: AppSite
  getCustomMenuItems: () => CustomMenuItem[]
  /** 进入需要登录的页面前执行，管理后台用于拉取合规确认状态。 */
  beforeProtectedRoute?: (to: RouteLocationNormalized) => Promise<void>
}

const BACKEND_MODE_ALLOWED_PATHS = ['/login', '/key-usage', '/payment/result', '/payment/airwallex', '/legal']
const BACKEND_MODE_CALLBACK_PATHS = [
  '/auth/callback',
  '/auth/linuxdo/callback',
  '/auth/dingtalk/callback',
  '/auth/dingtalk/email-completion',
  '/auth/oidc/callback',
  '/auth/wechat/callback',
  '/auth/wechat/payment/callback',
]
const BACKEND_MODE_PENDING_AUTH_PATHS = ['/register', '/email-verify']

// 简易模式下隐藏的页面：用户站整个账务区（含旧路径 /subscriptions /redeem 的 redirect 入口），
// 管理后台的订阅/兑换管理路径与旧用户站相同
const SIMPLE_MODE_RESTRICTED_PATHS = ['/billing', '/subscriptions', '/redeem']


export function isBackendModePublicRouteAllowed(path: string, hasPendingAuthSession: boolean): boolean {
  if (BACKEND_MODE_ALLOWED_PATHS.some((allowedPath) => path === allowedPath || path.startsWith(allowedPath))) {
    return true
  }
  if (BACKEND_MODE_CALLBACK_PATHS.some((callbackPath) => path === callbackPath)) {
    return true
  }
  return hasPendingAuthSession && BACKEND_MODE_PENDING_AUTH_PATHS.some((allowedPath) => path === allowedPath)
}

export function createSiteGuard(options: SiteGuardOptions) {
  const { site } = options
  const isUserSite = site === 'user'
  const homePath = defaultAuthedPath(site)
  let authInitialized = false

  return async function siteGuard(
    to: RouteLocationNormalized,
    _from: RouteLocationNormalized,
    next: NavigationGuardNext
  ): Promise<void> {
    const authStore = useAuthStore()
    const appStore = useAppStore()

    // Restore auth state from localStorage on first navigation (page refresh)
    if (!authInitialized) {
      authStore.checkAuth()
      authInitialized = true
    }

    document.title = resolveRouteDocumentTitle(to, appStore.siteName, options.getCustomMenuItems())

    const requiresAuth = to.meta.requiresAuth !== false // Default to true
    const backendModeBlocksUsers = isUserSite && appStore.backendModeEnabled
    // 本地可能残留另一站点的登录态（例如升级前在同一端口以管理员登录过），后端会拒绝其所有请求
    const roleMatchesSite = authStore.isAdmin === !isUserSite

    if (to.path === '/setup') {
      try {
        const status = await getSetupStatus()
        if (!status.needs_setup) {
          next(resolveCompletedSetupRedirectPath(authStore.isAuthenticated))
          return
        }
      } catch {
        // If setup status cannot be determined, keep the setup page reachable.
      }
    }

    if (!requiresAuth) {
      if (authStore.isAuthenticated && (to.path === '/login' || to.path === '/register')) {
        // 两种情况必须留在登录页，否则会与受保护页面的回跳形成循环：
        // 登录态与站点不符；Backend mode 下用户站的普通用户被挡在所有受保护页面之外
        if (!roleMatchesSite || backendModeBlocksUsers) {
          next()
          return
        }
        next(homePath)
        return
      }
      // Model Plaza:公开路由但受「启用开关 + 可选强制登录」双重控制(后端同口径 fail-closed)
      if (to.path === '/model-plaza') {
        if (!appStore.publicSettingsLoaded) {
          try {
            await appStore.fetchPublicSettings()
          } catch (error) {
            console.warn('Failed to load public settings in route guard', error)
          }
        }
        const plazaSettings = appStore.cachedPublicSettings
        // 仅在设置成功加载且明确为 false 时拦截(瞬时加载失败视为未知,由后端 404 兜底)
        if (appStore.publicSettingsLoaded && plazaSettings?.model_plaza_enabled === false) {
          next(authStore.isAuthenticated ? homePath : '/home')
          return
        }
        if (plazaSettings?.model_plaza_require_auth === true && !authStore.isAuthenticated) {
          next({ path: '/login', query: { redirect: to.fullPath } })
          return
        }
        if (backendModeBlocksUsers && authStore.isAuthenticated) {
          next('/login')
          return
        }
      }
      // Backend mode: block public pages for unauthenticated users (except login, key-usage, callbacks)
      if (backendModeBlocksUsers && !authStore.isAuthenticated) {
        if (!isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)) {
          next('/login')
          return
        }
      }
      next()
      return
    }

    if (!authStore.isAuthenticated) {
      next({ path: '/login', query: { redirect: to.fullPath } })
      return
    }

    if (!roleMatchesSite) {
      next('/login')
      return
    }

    if (options.beforeProtectedRoute) {
      await options.beforeProtectedRoute(to)
    }

    // 公共设置可能尚未加载（根组件 onMounted 异步拉取晚于首次导航，且纯静态部署
    // 无 __APP_CONFIG__ 注入）。此时 cachedPublicSettings 为空会把 payment/risk_control
    // 误判为“未启用”而错误拦截，故这里先确保设置加载完成。
    if ((to.meta.requiresPayment || to.meta.requiresRiskControl || to.meta.requiresSubscription || to.meta.requiresAffiliate) && !appStore.publicSettingsLoaded) {
      try {
        await appStore.fetchPublicSettings()
      } catch (error) {
        console.warn('Failed to load public settings in route guard', error)
      }
    }

    // Only an explicit value from successfully loaded settings can disable a route.
    // A transient settings failure is unknown state, not a confirmed feature toggle.
    if (to.meta.requiresPayment && appStore.publicSettingsLoaded && appStore.cachedPublicSettings?.payment_enabled === false) {
      next(homePath)
      return
    }
    if (to.meta.requiresRiskControl && appStore.publicSettingsLoaded && appStore.cachedPublicSettings?.risk_control_enabled === false) {
      next(isUserSite ? homePath : '/settings')
      return
    }
    // 订阅功能是 opt-out 开关：只有显式 false 才拦截「我的订阅」页直达。
    if (to.meta.requiresSubscription && appStore.publicSettingsLoaded && appStore.cachedPublicSettings?.subscription_enabled === false) {
      next(homePath)
      return
    }
    // 邀请返利与支付一样是 opt-in：账务页签不显示它时，直达 /billing/affiliate 也要弹走
    if (to.meta.requiresAffiliate && appStore.publicSettingsLoaded && appStore.cachedPublicSettings?.affiliate_enabled === false) {
      next(homePath)
      return
    }

    if (authStore.isSimpleMode && SIMPLE_MODE_RESTRICTED_PATHS.some((path) => to.path.startsWith(path))) {
      next(homePath)
      return
    }

    if (backendModeBlocksUsers && !isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)) {
      next('/login')
      return
    }

    next()
  }
}
