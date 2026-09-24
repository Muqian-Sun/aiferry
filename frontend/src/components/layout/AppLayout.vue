<template>
  <!-- 管理站的壳：左侧栏（站点根组件提供）+ 顶栏 + 内容区。一张面，没有背景渐变。页面标题在内容区顶部（A3），顶栏只放工具。 -->
  <div class="min-h-screen bg-af-sheet text-af-ink">
    <component :is="siteLayout.sidebar" />

    <div class="relative min-h-screen transition-[margin] duration-300" :class="[sidebarCollapsed ? 'lg:ml-16' : 'lg:ml-60']">
      <AppHeader />

      <!-- 页头高度交给列表页：TablePageLayout 按视口定高，要把页头占掉的高度扣出去 -->
      <main class="px-4 py-5 md:px-6 md:py-6 lg:px-8" :style="{ '--admin-page-header-h': `${pageHeaderHeight}px` }">
        <div v-if="!route.meta.hidePageHeader" ref="pageHeaderEl" class="flow-root">
          <AdminPageHeader>
            <template v-if="$slots['header-actions']" #actions>
              <slot name="header-actions" />
            </template>
          </AdminPageHeader>
        </div>
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useElementSize } from '@vueuse/core'
import { useAppStore } from '@/stores/app'
import { SITE_LAYOUT } from '@/app/siteLayout'
import AdminPageHeader from '@/components/admin/layout/AdminPageHeader.vue'
import AppHeader from './AppHeader.vue'

const siteLayout = inject(SITE_LAYOUT)
if (!siteLayout) {
  throw new Error('AppLayout requires a site layout; provide SITE_LAYOUT from the site root component')
}

const route = useRoute()
const appStore = useAppStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)

// flow-root 让页头的下外边距算进包裹层高度
const pageHeaderEl = ref<HTMLElement | null>(null)
const { height: pageHeaderHeight } = useElementSize(pageHeaderEl, undefined, { box: 'border-box' })
</script>
