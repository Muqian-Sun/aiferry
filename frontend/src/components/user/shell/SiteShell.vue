<template>
  <!--
    用户站唯一的壳：顶部导航 + 1200px 居中内容。页面本身就是一张面（sheet），
    没有侧栏、没有第二层卡片，区块之间只用 hairline。
    variant=public：首页 / 模型页 / 法律文档 / 404；variant=console：登录后的五个页签页。
  -->
  <div class="flex min-h-screen flex-col bg-af-sheet text-af-ink">
    <SiteNav :variant="variant" />
    <main class="mx-auto w-full max-w-site flex-1 px-6" :class="flush ? 'py-0' : 'py-8'">
      <PageHeader v-if="variant === 'console' && !hideHeader" :title="title ?? routeTitle" :description="description ?? routeDescription">
        <template v-if="$slots.actions" #actions><slot name="actions" /></template>
        <template v-if="$slots.tabs" #tabs><slot name="tabs" /></template>
      </PageHeader>
      <div :class="variant === 'console' && !hideHeader ? 'pt-6' : ''">
        <slot />
      </div>
    </main>
    <footer v-if="variant === 'public' && !hideFooter" class="border-t border-af-hairline">
      <div class="mx-auto flex max-w-site flex-wrap items-center justify-between gap-3 px-6 py-6 text-xs text-af-ink-3">
        <span>© {{ currentYear }} {{ siteName }}</span>
        <div v-if="legalLinks.length" class="flex gap-4">
          <RouterLink v-for="doc in legalLinks" :key="doc.id" :to="`/legal/${doc.id}`" class="hover:text-af-ink">
            {{ doc.title }}
          </RouterLink>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { useOnboardingStore } from '@/stores/onboarding'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { usePageTitle } from '@/composables/usePageTitle'
import PageHeader from './PageHeader.vue'
import SiteNav from './SiteNav.vue'

const props = withDefaults(
  defineProps<{
    variant?: 'public' | 'console'
    /** 覆盖路由 meta 的标题 / 说明 */
    title?: string
    description?: string
    /** 页面自带标题区（如自定义页 iframe）时隐藏页头 */
    hideHeader?: boolean
    /** 去掉主区域上下内边距（iframe 出血） */
    flush?: boolean
    hideFooter?: boolean
  }>(),
  { variant: 'console', hideHeader: false, flush: false, hideFooter: false }
)

const appStore = useAppStore()
const { title: routeTitle, description: routeDescription } = usePageTitle()
const siteName = computed(() => appStore.siteName)
const currentYear = new Date().getFullYear()
const legalLinks = computed(() =>
  props.variant === 'public' ? appStore.cachedPublicSettings?.login_agreement_documents ?? [] : []
)

// 新手引导挂在控制台壳上（原 AppLayout 的职责）；storageKey 与旧实现一致，用户不会重新看到已看过的引导
if (props.variant === 'console') {
  const { replayTour } = useOnboardingTour({ storageKey: 'user_guide', autoStart: true })
  const onboardingStore = useOnboardingStore()
  onMounted(() => onboardingStore.setReplayCallback(replayTour))
}
</script>
