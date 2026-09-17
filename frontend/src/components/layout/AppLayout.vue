<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar：由站点根组件提供（用户站 / 管理后台各自的导航） -->
    <component :is="siteLayout.sidebar" />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, inject, onMounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import { SITE_LAYOUT } from '@/app/siteLayout'
import AppHeader from './AppHeader.vue'

const siteLayout = inject(SITE_LAYOUT)
if (!siteLayout) {
  throw new Error('AppLayout requires a site layout; provide SITE_LAYOUT from the site root component')
}

const appStore = useAppStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)

const { replayTour } = useOnboardingTour({
  storageKey: siteLayout.onboardingStorageKey,
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>
