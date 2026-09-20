import { createApp } from 'vue'
import { createPinia } from 'pinia'
import type { Component } from 'vue'
import type { Router } from 'vue-router'
import i18n, { initI18n } from '@/i18n'
import { useAppStore } from '@/stores/app'
import { initTheme } from '@/composables/useTheme'
import { DEFAULT_SITE_NAME, DEFAULT_SITE_SUBTITLE, updateFavicon } from '@/utils/branding'
import { isIOSDevice } from '@/utils/device'
import '@/style.css'

function initIOSViewportZoomFix() {
  // iOS Safari 在输入框字号小于 16px 时聚焦会自动放大页面，且失焦后不会恢复。
  // 限制 maximum-scale 可阻止该行为；iOS 10+ 用户仍可双指手动缩放，不影响可访问性。
  // 仅在 iOS 设备上注入，避免影响 Android Chrome 的手动缩放能力。
  if (!isIOSDevice()) return

  const viewport = document.querySelector('meta[name="viewport"]')
  if (!viewport) return

  const content = viewport.getAttribute('content') || ''
  if (/maximum-scale/i.test(content)) return
  viewport.setAttribute('content', `${content}, maximum-scale=1.0`)
}

/**
 * 两个站点共用的启动流程：主题、站点配置、i18n、路由就绪后挂载。
 * 根组件与路由由各自入口传入，本模块不引用任何站点专属代码。
 */
export async function bootstrapApp(App: Component, router: Router): Promise<void> {
  // Apply theme class globally before app mount to keep all routes consistent.
  initTheme()
  initIOSViewportZoomFix()

  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  // Initialize settings from injected config BEFORE mounting (prevents flash)
  // This must happen after pinia is installed but before router and i18n
  const appStore = useAppStore()
  appStore.initFromInjectedConfig()

  // Set document title immediately after config is loaded
  if (appStore.siteName && appStore.siteName !== DEFAULT_SITE_NAME) {
    document.title = `${appStore.siteName} - ${DEFAULT_SITE_SUBTITLE}`
  }
  updateFavicon(appStore.siteLogo)

  await initI18n()

  app.use(router)
  app.use(i18n)

  // 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
  await router.isReady()
  app.mount('#app')
}

