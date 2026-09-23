<template>
  <!--
    认证页外壳（用户站与管理站登录共用）：sunken 底、400px 单列表单面（sheet + hairline），
    品牌在上、列内左对齐。无渐变、无光斑、无玻璃。
  -->
  <div class="flex min-h-screen flex-col bg-af-sunken text-af-ink">
    <div class="mx-auto flex w-full max-w-[400px] flex-1 flex-col justify-center px-4 py-10">
      <!-- 品牌：品牌区不等设置加载，避免后端不可用时空白；logo 缺省用产品图标 -->
      <div class="mb-6 flex items-center gap-3">
        <BrandLogo :src="siteLogo" :alt="siteName" class="h-8 w-8 text-af-ink" />
        <div class="min-w-0">
          <div class="truncate text-base font-semibold text-af-ink">{{ siteName }}</div>
          <div class="truncate text-xs text-af-ink-3">{{ siteSubtitle }}</div>
        </div>
      </div>

      <div class="rounded-lg border border-af-hairline bg-af-sheet p-6 sm:p-8">
        <slot />
      </div>

      <div class="mt-5 text-center text-sm text-af-ink-2">
        <slot name="footer" />
      </div>

      <div class="mt-8 text-center text-xs text-af-ink-4">&copy; {{ currentYear }} {{ siteName }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { DEFAULT_SITE_NAME, DEFAULT_SITE_SUBTITLE } from '@/utils/branding'
import { sanitizeUrl } from '@/utils/url'
import BrandLogo from '@/components/common/BrandLogo.vue'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || DEFAULT_SITE_NAME)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || DEFAULT_SITE_SUBTITLE)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
