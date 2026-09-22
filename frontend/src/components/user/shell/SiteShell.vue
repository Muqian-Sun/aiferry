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
    <footer v-if="variant === 'public' && !hideFooter" class="border-t border-af-hairline" data-testid="site-footer">
      <!-- 横排一行（muqian 2026-09-23）：品牌 · 链接 · 版权；窄屏按需折行，不分栏 -->
      <div class="mx-auto flex max-w-site flex-wrap items-center justify-between gap-x-10 gap-y-4 px-6 py-8">
        <div class="flex min-w-0 items-center gap-2.5" data-testid="footer-brand">
          <img :src="logoSrc" alt="" class="h-6 w-6 shrink-0 object-contain" />
          <span class="text-base font-semibold text-af-ink">{{ siteName }}</span>
          <span v-if="siteSubtitle" class="hidden truncate text-13 text-af-ink-4 lg:inline">· {{ siteSubtitle }}</span>
        </div>
        <nav class="flex flex-wrap items-center gap-x-6 gap-y-2 text-13 text-af-ink-3" data-testid="footer-links">
          <template v-for="link in footerLinks" :key="link.key">
            <a v-if="link.external" :href="link.to" target="_blank" rel="noopener noreferrer" class="hover:text-af-ink">{{ link.label }}</a>
            <RouterLink v-else-if="link.to" :to="link.to" class="hover:text-af-ink">{{ link.label }}</RouterLink>
            <span v-else class="text-af-ink-4">{{ link.label }}</span>
          </template>
        </nav>
        <p class="text-xs text-af-ink-4">© {{ currentYear }} {{ siteName }}</p>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { sanitizeUrl } from '@/utils/url'
import { CONSOLE_HOME_PATH } from './navItems'
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
const authStore = useAuthStore()
const { title: routeTitle, description: routeDescription } = usePageTitle()
const siteName = computed(() => appStore.siteName)
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')
const logoSrc = computed(
  () => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }) || '/logo.svg'
)
const currentYear = new Date().getFullYear()

interface FooterLink {
  key: string
  label: string
  /** 站内路径或外链；空 = 纯文字（联系方式） */
  to?: string
  external?: boolean
}
const footerLinks = computed<FooterLink[]>(() => {
  if (props.variant !== 'public') return []
  const settings = appStore.cachedPublicSettings
  const product: FooterLink[] = [
    { key: 'home', label: t('userUi.footer.home'), to: '/home' },
    { key: 'pricing', label: t('userUi.nav.pricing'), to: '/model-plaza' },
    authStore.isAuthenticated
      ? { key: 'console', label: t('userUi.nav.console'), to: CONSOLE_HOME_PATH }
      : { key: 'login', label: t('userUi.nav.login'), to: '/login' }
  ]
  if (!authStore.isAuthenticated && settings?.registration_enabled) {
    product.push({ key: 'register', label: t('userUi.footer.register'), to: '/register' })
  }
  const help: FooterLink[] = []
  const docUrl = sanitizeUrl(settings?.doc_url || appStore.docUrl)
  if (docUrl) help.push({ key: 'docs', label: t('userUi.nav.docs'), to: docUrl, external: true })
  const contact = (settings?.contact_info || appStore.contactInfo || '').trim()
  if (contact) help.push({ key: 'contact', label: contact })
  const legal: FooterLink[] = (settings?.login_agreement_documents ?? []).map((doc) => ({
    key: `legal-${doc.id}`,
    label: doc.title,
    to: `/legal/${doc.id}`
  }))
  return [...product, ...help, ...legal]
})

// 新手引导挂在控制台壳上（原 AppLayout 的职责）；storageKey 与旧实现一致，用户不会重新看到已看过的引导
if (props.variant === 'console') {
  const { replayTour } = useOnboardingTour({ storageKey: 'user_guide', autoStart: true })
  const onboardingStore = useOnboardingStore()
  onMounted(() => onboardingStore.setReplayCallback(replayTour))
}
</script>
