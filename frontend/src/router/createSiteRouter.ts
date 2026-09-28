import { createRouter, createWebHistory, type RouteRecordRaw, type Router } from 'vue-router'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'
import { useRoutePrefetch } from '@/composables/useRoutePrefetch'
import { createSiteGuard, type SiteGuardOptions } from './siteGuard'
import { runRoutePreload } from './routePreload'

/** 按站点创建路由器：路由表与守卫差异由站点入口传入，其余行为两站一致。 */
export function createSiteRouter(routes: RouteRecordRaw[], guardOptions: SiteGuardOptions): Router {
  const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes,
    scrollBehavior(_to, _from, savedPosition) {
      // Scroll to saved position when using browser back/forward
      if (savedPosition) {
        return savedPosition
      }
      // Scroll to top for new routes
      return { top: 0 }
    }
  })

  const navigationLoading = useNavigationLoadingState()
  // 延迟初始化预加载，传入 router 实例
  let routePrefetch: ReturnType<typeof useRoutePrefetch> | null = null
  const guard = createSiteGuard(guardOptions)

  router.beforeEach(async (to, from, next) => {
    navigationLoading.startNavigation()
    await guard(to, from, next)
  })

  // 守卫都放行后再预加载首屏数据：被拦走的导航不发请求
  router.beforeResolve((to, from) => runRoutePreload(to, from))

  router.afterEach((to) => {
    navigationLoading.endNavigation()
    if (!routePrefetch) {
      routePrefetch = useRoutePrefetch(router)
    }
    // 触发路由预加载（在浏览器空闲时执行）
    routePrefetch.triggerPrefetch(to)
  })

  /**
   * Handles dynamic import failures caused by deployment updates
   */
  router.onError((error) => {
    console.error('Router error:', error)

    const isChunkLoadError =
      error.message?.includes('Failed to fetch dynamically imported module') ||
      error.message?.includes('Loading chunk') ||
      error.message?.includes('Loading CSS chunk') ||
      error.name === 'ChunkLoadError'

    if (isChunkLoadError) {
      // Avoid infinite reload loop by checking sessionStorage
      const reloadKey = 'chunk_reload_attempted'
      const lastReload = sessionStorage.getItem(reloadKey)
      const now = Date.now()

      // Allow reload if never attempted or more than 10 seconds ago
      if (!lastReload || now - parseInt(lastReload) > 10000) {
        sessionStorage.setItem(reloadKey, now.toString())
        console.warn('Chunk load error detected, reloading page to fetch latest version...')
        window.location.reload()
      } else {
        console.error('Chunk load error persists after reload. Please clear browser cache.')
      }
    }
  })

  return router
}
