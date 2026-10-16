<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppShell from '@/app/AppShell.vue'
import AnnouncementNotice from '@/components/user/AnnouncementNotice.vue'
import { CONSOLE_SHELL_STATE, isConsoleShellPath } from '@/components/user/shell/consoleShell'
import { useAuthStore } from '@/stores/auth'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { useAnnouncementStore } from '@/stores/announcements'
import { SITE_FEATURES } from '@/utils/siteFeatures'

// 用户站的壳是 components/user/shell/SiteShell（顶部导航，无侧栏）；SITE_LAYOUT 注入只剩管理后台在用。
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()
const announcementStore = useAnnouncementStore()

function onVisibilityChange() {
  if (document.visibilityState === 'visible' && authStore.isAuthenticated) {
    announcementStore.fetchAnnouncements()
  }
}

// 订阅不显示时（SITE_FEATURES.subscription）不预加载 / 轮询订阅接口。
function startSubscriptionSync() {
  subscriptionStore.fetchActiveSubscriptions().catch((error) => {
    console.error('Failed to preload subscriptions:', error)
  })
  subscriptionStore.startPolling()
}

// Watch for authentication state and manage subscription data + announcements
watch(
  () => authStore.isAuthenticated,
  (isAuthenticated, oldValue) => {
    if (isAuthenticated) {
      // User logged in: preload subscriptions and start polling
      if (SITE_FEATURES.subscription) {
        startSubscriptionSync()
      }

      // 公告：登录 / 进站时登记一次弹窗（新登录总弹；刷新恢复登录态时本标签页弹过就不再弹；今日不再弹出的除外）。
      // 真正打开要等进入控制台，见下面的 setInConsole。
      const userId = authStore.user?.id
      if (userId !== undefined) {
        announcementStore.requestNotice(userId, oldValue === false)
      }
      if (oldValue === false) {
        // New login: delay 3s then force fetch
        setTimeout(() => announcementStore.fetchAnnouncements(true), 3000)
      } else {
        // Page refresh restore (oldValue was undefined)
        announcementStore.fetchAnnouncements()
      }

      document.addEventListener('visibilitychange', onVisibilityChange)
    } else {
      // User logged out: clear data and stop polling
      subscriptionStore.clear()
      announcementStore.reset()
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  },
  { immediate: true }
)

// Route change trigger (throttled by store)
router.afterEach(() => {
  if (authStore.isAuthenticated) {
    announcementStore.fetchAnnouncements()
  }
})

// 公告弹窗只允许在控制台壳出现。公开站、认证页、支付回跳和支付弹窗先压住。
watch(
  () => [route.path, route.query.method, authStore.isAuthenticated] as const,
  () => {
    const stripePopup = route.path === '/payment/stripe' && route.query.method != null && route.query.method !== ''
    const fromConsole = window.history.state?.shell === CONSOLE_SHELL_STATE.shell
    announcementStore.setInConsole(
      isConsoleShellPath(route.path, { authenticated: authStore.isAuthenticated, stripePopup, fromConsole })
    )
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<template>
  <AppShell>
    <AnnouncementNotice />
  </AppShell>
</template>
