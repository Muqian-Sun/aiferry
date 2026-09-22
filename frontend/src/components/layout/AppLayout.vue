<template>
  <!-- 管理站的壳：左侧栏（站点根组件提供）+ 顶栏 + 内容区。一张面，没有背景渐变。 -->
  <div class="min-h-screen bg-af-sheet text-af-ink">
    <component :is="siteLayout.sidebar" />

    <div class="relative min-h-screen transition-[margin] duration-300" :class="[sidebarCollapsed ? 'lg:ml-16' : 'lg:ml-60']">
      <AppHeader />

      <main class="px-4 py-5 md:px-6 md:py-6 lg:px-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject } from 'vue'
import { useAppStore } from '@/stores/app'
import { SITE_LAYOUT } from '@/app/siteLayout'
import AppHeader from './AppHeader.vue'

const siteLayout = inject(SITE_LAYOUT)
if (!siteLayout) {
  throw new Error('AppLayout requires a site layout; provide SITE_LAYOUT from the site root component')
}

const appStore = useAppStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
</script>
