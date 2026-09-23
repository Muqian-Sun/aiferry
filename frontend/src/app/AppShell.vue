<script setup lang="ts">
/**
 * 两个站点共用的根外壳：站点品牌（favicon / 标题）、公开设置加载、全局导航进度与提示。
 * 站点专属的全局行为（用户站的订阅与公告、管理后台的合规确认）放在各自根组件的默认插槽里。
 */
import { RouterView, useRoute } from 'vue-router'
import { onMounted, watch } from 'vue'
import Toast from '@/components/common/Toast.vue'
import NavigationProgress from '@/components/common/NavigationProgress.vue'
import { resolveRouteDocumentTitle } from '@/router/title'
import { getSiteContext } from '@/app/siteContext'
import { useAppStore } from '@/stores/app'
import { updateFavicon } from '@/utils/branding'
import { resolveSiteBillingMode } from '@/utils/siteBillingMode'

const route = useRoute()
const appStore = useAppStore()

function updateDocumentTitle() {
  document.title = resolveRouteDocumentTitle(route, appStore.siteName, getSiteContext().getCustomMenuItems(), {
    billingMode: resolveSiteBillingMode(appStore.cachedPublicSettings),
    legalDocuments: appStore.cachedPublicSettings?.login_agreement_documents,
  })
}

// Watch for site settings changes and update favicon/title
watch(
  () => appStore.siteLogo,
  (newLogo) => {
    if (newLogo) {
      updateFavicon(newLogo)
    }
  },
  { immediate: true }
)

watch(
  [
    () => route.fullPath,
    () => route.meta.title,
    () => route.meta.titleKey,
    () => appStore.siteName,
    () => getSiteContext().getCustomMenuItems(),
    () => appStore.cachedPublicSettings?.subscription_enabled,
    () => appStore.cachedPublicSettings?.payment_balance_disabled,
  ],
  updateDocumentTitle,
  { deep: true }
)

onMounted(async () => {
  // Load public settings into appStore (will be cached for other components)
  await appStore.fetchPublicSettings()

  // Re-resolve document title now that site settings are available
  updateDocumentTitle()
})
</script>

<template>
  <NavigationProgress />
  <RouterView />
  <Toast />
  <slot />
</template>
