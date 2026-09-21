<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useRouter } from 'vue-router'
import AppShell from '@/app/AppShell.vue'
import AnnouncementPopup from '@/components/common/AnnouncementPopup.vue'
import { useAuthStore } from '@/stores/auth'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { useAnnouncementStore } from '@/stores/announcements'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

// 用户站的壳是 components/user/shell/SiteShell（顶部导航，无侧栏）；SITE_LAYOUT 注入只剩管理后台在用。
const router = useRouter()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()
const announcementStore = useAnnouncementStore()

function onVisibilityChange() {
  if (document.visibilityState === 'visible' && authStore.isAuthenticated) {
    announcementStore.fetchAnnouncements()
  }
}

// 订阅功能开关（opt-out）。关闭后不再预加载/轮询订阅接口；开关在登录后才到达时补启动，反向则清空。
const subscriptionFeatureEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.subscription))

function startSubscriptionSync() {
  subscriptionStore.fetchActiveSubscriptions().catch((error) => {
    console.error('Failed to preload subscriptions:', error)
  })
  subscriptionStore.startPolling()
}

watch(subscriptionFeatureEnabled, (enabled) => {
  if (!authStore.isAuthenticated) return
  if (enabled) {
    startSubscriptionSync()
  } else {
    subscriptionStore.clear()
  }
})

// Watch for authentication state and manage subscription data + announcements
watch(
  () => authStore.isAuthenticated,
  (isAuthenticated, oldValue) => {
    if (isAuthenticated) {
      // User logged in: preload subscriptions and start polling (skipped when the
      // subscription feature is switched off; see the flag watcher above)
      if (subscriptionFeatureEnabled.value) {
        startSubscriptionSync()
      }

      // Announcements: new login vs page refresh restore
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

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<template>
  <AppShell>
    <AnnouncementPopup />
  </AppShell>
</template>
