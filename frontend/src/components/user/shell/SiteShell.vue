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
      <div class="mx-auto max-w-site px-6 py-10">
        <div class="grid gap-8 sm:grid-cols-3">
          <div v-for="column in footerColumns" :key="column.key" class="min-w-0">
            <h2 class="text-13 font-medium text-af-ink">{{ column.title }}</h2>
            <ul class="mt-3 space-y-2 text-13 text-af-ink-3">
              <li v-for="link in column.links" :key="link.key">
                <a v-if="link.external" :href="link.to" target="_blank" rel="noopener noreferrer" class="hover:text-af-ink">{{ link.label }}</a>
                <RouterLink v-else-if="link.to" :to="link.to" class="hover:text-af-ink">{{ link.label }}</RouterLink>
                <span v-else class="text-af-ink-2">{{ link.label }}</span>
              </li>
            </ul>
          </div>
        </div>
        <p class="mt-10 text-xs text-af-ink-4">© {{ currentYear }} {{ siteName }}</p>
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
const currentYear = new Date().getFullYear()

interface FooterLink {
  key: string
  label: string
  /** 站内路径或外链；空 = 纯文字（联系方式） */
  to?: string
  external?: boolean
}
interface FooterColumn {
  key: string
  title: string
  links: FooterLink[]
}

const footerColumns = computed<FooterColumn[]>(() => {
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
  return [
    { key: 'product', title: t('userUi.footer.product'), links: product },
    { key: 'help', title: t('userUi.footer.help'), links: help },
    { key: 'legal', title: t('userUi.footer.legal'), links: legal }
  ].filter((column) => column.links.length > 0)
})

// 新手引导挂在控制台壳上（原 AppLayout 的职责）；storageKey 与旧实现一致，用户不会重新看到已看过的引导
if (props.variant === 'console') {
  const { replayTour } = useOnboardingTour({ storageKey: 'user_guide', autoStart: true })
  const onboardingStore = useOnboardingStore()
  onMounted(() => onboardingStore.setReplayCallback(replayTour))
}
</script>
