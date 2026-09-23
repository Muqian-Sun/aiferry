<template>
  <!--
    用户站唯一的壳：顶部导航 + 1200px 居中内容。页面本身就是一张面（sheet），
    没有侧栏、没有第二层卡片，区块之间只用 hairline。
    variant=public：首页 / 模型页 / 法律文档 / 404；variant=console：登录后的五个页签页。
  -->
  <div class="flex min-h-screen flex-col bg-af-sheet text-af-ink">
    <SiteNav :variant="variant" />
    <main class="mx-auto flex w-full flex-1 flex-col" :class="flush ? 'max-w-none' : 'max-w-site px-6 py-8'">
      <PageHeader v-if="variant === 'console' && !hideHeader" :title="title ?? routeTitle" :description="description ?? routeDescription">
        <template v-if="$slots.actions" #actions><slot name="actions" /></template>
        <template v-if="$slots.tabs" #tabs><slot name="tabs" /></template>
      </PageHeader>
      <div :class="[variant === 'console' && !hideHeader ? 'pt-6' : '', flush ? 'flex min-h-0 flex-1 flex-col' : '']">
        <slot />
      </div>
    </main>
    <!-- 公开站页脚：三栏链接全部来自公开设置（文档地址 / 联系方式 / 协议文档），没有的栏不出现 -->
    <footer v-if="variant === 'public' && !hideFooter" class="border-t border-af-hairline bg-af-sunken" data-testid="site-footer">
      <!-- 横排一行（muqian 2026-09-23：页脚只留版权与条款）：版权在左、条款在右，窄屏按需折行 -->
      <div class="mx-auto flex max-w-site flex-wrap items-center justify-between gap-x-6 gap-y-2 px-6 py-6 text-13 text-af-ink-3">
        <p>© {{ currentYear }} {{ siteName }} · {{ t('userUi.footer.rights') }}</p>
        <nav v-if="legalLinks.length" class="flex flex-wrap items-center gap-x-6 gap-y-1" data-testid="footer-legal">
          <RouterLink v-for="link in legalLinks" :key="link.key" :to="link.to" class="hover:text-af-ink">{{ link.label }}</RouterLink>
        </nav>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
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
    /** 满宽出血且纵向填满视口（自定义页 iframe / Markdown）：无最大宽、无内边距 */
    flush?: boolean
    hideFooter?: boolean
  }>(),
  { variant: 'console', hideHeader: false, flush: false, hideFooter: false }
)

const { t } = useI18n()
const appStore = useAppStore()
const { title: routeTitle, description: routeDescription } = usePageTitle()
const siteName = computed(() => appStore.siteName)
const currentYear = new Date().getFullYear()

/** 条款文档（后台「登录协议」里配置的）：和版权同一行 */
const legalLinks = computed(() => {
  if (props.variant !== 'public') return []
  return (appStore.cachedPublicSettings?.login_agreement_documents ?? []).map((doc) => ({
    key: `legal-${doc.id}`,
    label: doc.title,
    to: `/legal/${doc.id}`
  }))
})

// 新手引导挂在控制台壳上（原 AppLayout 的职责）；storageKey 与旧实现一致，用户不会重新看到已看过的引导
if (props.variant === 'console') {
  const { replayTour } = useOnboardingTour({ storageKey: 'user_guide', autoStart: true })
  const onboardingStore = useOnboardingStore()
  onMounted(() => onboardingStore.setReplayCallback(replayTour))
}
</script>
