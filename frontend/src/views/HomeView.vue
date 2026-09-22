<template>
  <!-- 管理员自定义首页：整页 iframe 或 HTML，优先级最高（内容由管理员配置，安全边界不变） -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <iframe v-if="isHomeContentUrl" :src="homeContent.trim()" class="h-screen w-full border-0" allowfullscreen></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- 精简首页：品牌 + 一句话 + 一个入口 -->
  <SiteShell v-else-if="compactHomeEnabled" variant="public">
    <div data-testid="compact-home" class="flex min-h-[60vh] flex-col items-start justify-center py-16">
      <img :src="siteLogo || '/logo.svg'" alt="" class="h-12 w-12 object-contain" />
      <h1 class="mt-6 text-28 font-semibold text-af-ink [overflow-wrap:anywhere]">{{ siteName }}</h1>
      <p class="mt-3 max-w-xl whitespace-pre-wrap text-base text-af-ink-2 [overflow-wrap:anywhere]">{{ siteSubtitle }}</p>
      <RouterLink :to="isAuthenticated ? consolePath : '/login'" class="btn btn-primary mt-8">
        {{ isAuthenticated ? t('userUi.home.hero.goToConsole') : t('userUi.nav.login') }}
      </RouterLink>
    </div>
  </SiteShell>

  <!--
    默认首页（2026-09-22 按 gpt.ge / new.12ai.org 的结构重排）：
    居中首屏 → 六张卖点卡 → 三步 + 协议代码窗口 → 模型与标价卡 → 客户端卡片 → 数字卡 → 墨色 CTA 带 → 页脚。
    每段「眉题 / 大标题 / 副标题」居中；内容用 sheet-card；数据段拿不到就不出现，不放假数字。
  -->
  <SiteShell v-else variant="public" flush>
    <div data-testid="default-home">
      <!-- 首屏：居中，背景一层淡网格向四周渐隐 -->
      <section class="home-hero relative overflow-hidden">
        <div class="relative mx-auto max-w-site px-6 pb-20 pt-20 text-center sm:pb-24 sm:pt-28">
          <p v-if="catalog.length" class="section-eyebrow" data-testid="hero-eyebrow">
            {{ t('userUi.home.hero.eyebrow', { models: catalog.length }) }}
          </p>
          <h1 class="mx-auto mt-6 max-w-4xl whitespace-pre-line text-4xl font-semibold leading-[1.15] tracking-tight text-af-ink sm:text-5xl lg:text-6xl">
            {{ t('userUi.home.hero.title') }}
          </h1>
          <p class="mx-auto mt-6 max-w-2xl text-base leading-7 text-af-ink-2 sm:text-lg sm:leading-8">{{ t('userUi.home.hero.description') }}</p>
          <div class="mt-10 flex flex-wrap items-center justify-center gap-3">
            <RouterLink :to="isAuthenticated ? consolePath : '/login'" class="btn btn-primary btn-pill" data-testid="home-primary-cta">
              {{ isAuthenticated ? t('userUi.home.hero.goToConsole') : t('userUi.home.hero.getStarted') }}
              <Icon name="arrowRight" size="sm" />
            </RouterLink>
            <RouterLink to="/model-plaza" class="btn btn-secondary btn-pill">
              {{ t('userUi.home.hero.viewPricing') }}
            </RouterLink>
          </div>
          <!-- 厂商行：目录里真有的厂商，拿不到目录就不出现 -->
          <div v-if="vendors.length" class="mt-14">
            <p class="text-13 text-af-ink-4">{{ t('userUi.home.hero.vendorsLabel') }}</p>
            <VendorStrip class="mt-4 justify-center" :vendors="vendors" />
          </div>
        </div>
      </section>

      <!-- 卖点：六张卡 -->
      <section class="bg-af-sunken py-20 sm:py-24" data-testid="home-features">
        <div class="mx-auto max-w-site px-6">
          <div class="text-center">
            <p class="section-eyebrow">{{ t('userUi.home.features.eyebrow') }}</p>
            <h2 class="section-title">{{ t('userUi.home.features.title') }}</h2>
            <p class="section-lead">{{ t('userUi.home.features.description') }}</p>
          </div>
          <ul class="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            <li v-for="feature in FEATURES" :key="feature.key" class="sheet-card p-6" data-testid="home-feature">
              <span class="icon-tile"><Icon :name="feature.icon" size="md" /></span>
              <h3 class="mt-5 text-base font-semibold text-af-ink">{{ t(`userUi.home.features.items.${feature.key}.title`) }}</h3>
              <p class="mt-2 text-sm leading-6 text-af-ink-3">{{ t(`userUi.home.features.items.${feature.key}.body`) }}</p>
            </li>
          </ul>
        </div>
      </section>

      <!-- 快速接入：左三步 + 两个入口卡，右协议代码窗口 -->
      <section class="py-20 sm:py-24" data-testid="home-quickstart">
        <div class="mx-auto max-w-site px-6">
          <div class="text-center">
            <p class="section-eyebrow">{{ t('userUi.home.quickstart.eyebrow') }}</p>
            <h2 class="section-title">{{ t('userUi.home.quickstart.title') }}</h2>
            <p class="section-lead">{{ t('userUi.home.quickstart.description') }}</p>
          </div>
          <div class="mt-12 grid gap-6 lg:grid-cols-12">
            <div class="space-y-4 lg:col-span-5">
              <ol class="space-y-3">
                <li v-for="(step, index) in STEPS" :key="step" class="sheet-card flex gap-4 p-5" data-testid="home-step">
                  <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-af-ink text-sm font-semibold text-af-sheet">{{ index + 1 }}</span>
                  <div class="min-w-0">
                    <h3 class="text-base font-semibold text-af-ink">{{ t(`userUi.home.quickstart.steps.${step}.title`) }}</h3>
                    <p class="mt-1 text-sm leading-6 text-af-ink-3">{{ t(`userUi.home.quickstart.steps.${step}.body`) }}</p>
                  </div>
                </li>
              </ol>
              <div class="grid gap-3 sm:grid-cols-2">
                <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="sheet-card group flex items-start justify-between gap-3 p-5" data-testid="home-link-docs">
                  <span>
                    <Icon name="book" size="md" class="text-af-ink-3" />
                    <span class="mt-3 block text-sm font-semibold text-af-ink">{{ t('userUi.home.quickstart.links.docs.title') }}</span>
                    <span class="mt-0.5 block text-xs text-af-ink-3">{{ t('userUi.home.quickstart.links.docs.body') }}</span>
                  </span>
                  <Icon name="arrowRight" size="sm" class="mt-1 shrink-0 text-af-ink-4 transition-colors group-hover:text-af-ink" />
                </a>
                <RouterLink :to="keysDestination" class="sheet-card group flex items-start justify-between gap-3 p-5" data-testid="home-link-clients">
                  <span>
                    <Icon name="terminal" size="md" class="text-af-ink-3" />
                    <span class="mt-3 block text-sm font-semibold text-af-ink">{{ t('userUi.home.quickstart.links.clients.title') }}</span>
                    <span class="mt-0.5 block text-xs text-af-ink-3">{{ t('userUi.home.quickstart.links.clients.body') }}</span>
                  </span>
                  <Icon name="arrowRight" size="sm" class="mt-1 shrink-0 text-af-ink-4 transition-colors group-hover:text-af-ink" />
                </RouterLink>
              </div>
            </div>
            <div class="min-w-0 lg:col-span-7">
              <ProtocolSample :base-url="apiBaseUrl" />
            </div>
          </div>
        </div>
      </section>

      <!-- 模型与标价：只在拿到目录时出现 -->
      <section v-if="catalogPreview.length" class="bg-af-sunken py-20 sm:py-24" data-testid="home-catalog-section">
        <div class="mx-auto max-w-site px-6">
          <div class="text-center">
            <p class="section-eyebrow">{{ t('userUi.home.catalog.eyebrow') }}</p>
            <h2 class="section-title">{{ t('userUi.home.catalog.title') }}</h2>
            <p class="section-lead">{{ t('userUi.home.catalog.description') }}</p>
          </div>
          <div class="sheet-card mt-12 overflow-hidden">
            <HomeCatalog :entries="catalogPreview" />
            <div class="flex justify-center border-t border-af-hairline px-6 py-4">
              <RouterLink to="/model-plaza" class="btn btn-secondary btn-sm">
                {{ t('userUi.home.catalog.viewAll', { count: catalog.length }) }}
                <Icon name="arrowRight" size="xs" />
              </RouterLink>
            </div>
          </div>
        </div>
      </section>

      <!-- 客户端：与「使用密钥」弹窗同一份清单 + 一张「任何 SDK」卡 -->
      <section class="py-20 sm:py-24" data-testid="home-clients">
        <div class="mx-auto max-w-site px-6">
          <div class="text-center">
            <p class="section-eyebrow">{{ t('userUi.home.clients.eyebrow') }}</p>
            <h2 class="section-title">{{ t('userUi.home.clients.title') }}</h2>
            <p class="section-lead">{{ t('userUi.home.clients.description') }}</p>
          </div>
          <ul class="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            <li v-for="client in HOME_CLIENTS" :key="client.id" class="sheet-card p-6" data-testid="home-client">
              <span class="icon-tile"><Icon :name="client.id === 'gemini' ? 'sparkles' : 'terminal'" size="md" /></span>
              <h3 class="mt-5 text-base font-semibold text-af-ink">{{ t(client.labelKey) }}</h3>
              <p class="mt-2 text-sm leading-6 text-af-ink-3">{{ t(client.homeKey) }}</p>
            </li>
            <li class="sheet-card p-6" data-testid="home-client-sdk">
              <span class="icon-tile"><Icon name="cube" size="md" /></span>
              <h3 class="mt-5 text-base font-semibold text-af-ink">{{ t('userUi.home.clients.sdk.title') }}</h3>
              <p class="mt-2 text-sm leading-6 text-af-ink-3">{{ t('userUi.home.clients.sdk.body') }}</p>
            </li>
          </ul>
          <div class="mt-8 text-center">
            <RouterLink :to="keysDestination" class="btn btn-secondary" data-testid="home-clients-cta">
              {{ t('userUi.home.clients.cta') }}
              <Icon name="arrowRight" size="xs" />
            </RouterLink>
          </div>
        </div>
      </section>

      <!-- 数字卡：数字都从目录算，拿不到不出现 -->
      <section v-if="catalog.length" class="pb-20 sm:pb-24">
        <div class="mx-auto max-w-site px-6">
          <dl class="sheet-card grid grid-cols-2 divide-y divide-af-hairline sm:grid-cols-4 sm:divide-x sm:divide-y-0" data-testid="home-stats">
            <div class="px-6 py-8 text-center">
              <dt class="text-13 text-af-ink-3">{{ t('userUi.home.stats.models') }}</dt>
              <dd class="mt-2 text-3xl font-semibold tabular-nums text-af-ink">{{ catalog.length }}</dd>
            </div>
            <div class="px-6 py-8 text-center">
              <dt class="text-13 text-af-ink-3">{{ t('userUi.home.stats.vendors') }}</dt>
              <dd class="mt-2 text-3xl font-semibold tabular-nums text-af-ink">{{ vendors.length }}</dd>
            </div>
            <div class="px-6 py-8 text-center">
              <dt class="text-13 text-af-ink-3">{{ t('userUi.home.stats.protocols') }}</dt>
              <dd class="mt-2 text-3xl font-semibold tabular-nums text-af-ink">{{ PROTOCOL_ROUTES.length }}</dd>
            </div>
            <div class="px-6 py-8 text-center">
              <dt class="text-13 text-af-ink-3">{{ t('userUi.home.stats.ledgerLabel') }}</dt>
              <dd class="mt-2 text-3xl font-semibold text-af-ink">{{ t('userUi.home.stats.ledgerValue') }}</dd>
            </div>
          </dl>
        </div>
      </section>

      <!-- CTA 带：墨色底，暗色模式下随 token 反转 -->
      <section class="bg-af-ink py-20 text-af-sheet sm:py-24" data-testid="home-cta">
        <div class="mx-auto max-w-site px-6 text-center">
          <p class="text-13 text-af-ink-4">{{ t('userUi.home.cta.eyebrow') }}</p>
          <h2 class="mt-4 text-3xl font-semibold tracking-tight sm:text-4xl">{{ t('userUi.home.cta.title') }}</h2>
          <RouterLink :to="isAuthenticated ? consolePath : '/login'" class="btn btn-pill mt-8 bg-af-sheet text-af-ink hover:bg-af-sunken" data-testid="home-cta-button">
            {{ isAuthenticated ? t('userUi.home.cta.goToConsole') : t('userUi.home.cta.getStarted') }}
            <Icon name="arrowRight" size="sm" />
          </RouterLink>
        </div>
      </section>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import HomeCatalog from '@/components/user/home/HomeCatalog.vue'
import ProtocolSample from '@/components/user/home/ProtocolSample.vue'
import VendorStrip from '@/components/user/home/VendorStrip.vue'
import Icon from '@/components/icons/Icon.vue'
import { PROTOCOL_ROUTES } from '@/components/user/home/protocols'
import { HOME_CLIENTS } from '@/components/user/clients'
import { CONSOLE_HOME_PATH } from '@/components/user/shell/navItems'
import { getModelPlaza } from '@/api/modelPlaza'
import { buildCatalog, catalogVendors, type CatalogModel } from '@/components/modelPlaza/catalog'
import { sanitizeUrl } from '@/utils/url'
import { DEFAULT_SITE_NAME, DEFAULT_SITE_SUBTITLE } from '@/utils/branding'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

// 站点设置直接读 appStore（注入配置已在挂载前初始化）
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || DEFAULT_SITE_NAME)
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || DEFAULT_SITE_SUBTITLE)
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))

// 自定义首页内容是 URL 时用 iframe 展示
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isAuthenticated = computed(() => authStore.isAuthenticated)
const consolePath = CONSOLE_HOME_PATH
const keysDestination = computed(() => (isAuthenticated.value ? '/keys' : { path: '/login', query: { redirect: '/keys' } }))
const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || appStore.apiBaseUrl || '')

/** 六张卖点卡：四条核心主张（同协议直连 / 不换模型 / 上架即标价 / 同会话固定上游）+ 逐条记账 + 余额透明 */
const FEATURES = [
  { key: 'protocol', icon: 'swap' },
  { key: 'noFallback', icon: 'shield' },
  { key: 'pricing', icon: 'dollar' },
  { key: 'sticky', icon: 'link' },
  { key: 'ledger', icon: 'document' },
  { key: 'balance', icon: 'creditCard' }
] as const
const STEPS = ['create', 'baseUrl', 'call'] as const

// 模型目录：与模型页同一个接口（对所有人开放）；拿不到（网络 / 空目录）就不渲染数字卡与价目预览，不放假数字
const HOME_CATALOG_ROWS = 8
const catalog = ref<CatalogModel[]>([])
const vendors = computed(() => catalogVendors(catalog.value))
const catalogPreview = computed(() => catalog.value.filter((entry) => entry.price).slice(0, HOME_CATALOG_ROWS))

async function loadCatalog() {
  try {
    const response = await getModelPlaza()
    catalog.value = buildCatalog(response.models ?? [])
  } catch {
    catalog.value = []
  }
}

onMounted(async () => {
  authStore.checkAuth()
  // 注入配置缺失时（纯静态部署）再拉一次公开设置
  if (!appStore.publicSettingsLoaded) {
    await appStore.fetchPublicSettings().catch(() => undefined)
  }
  if (!hasHomeContent.value && !compactHomeEnabled.value) {
    void loadCatalog()
  }
})
</script>

<style scoped>
/* 首屏底纹：淡网格，中心清晰、四周渐隐（学 gpt.ge），颜色走 hairline token，暗色自动跟随 */
.home-hero::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    linear-gradient(rgb(var(--af-hairline)) 1px, transparent 1px),
    linear-gradient(90deg, rgb(var(--af-hairline)) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(ellipse 70% 70% at 50% 40%, black 30%, transparent 75%);
  -webkit-mask-image: radial-gradient(ellipse 70% 70% at 50% 40%, black 30%, transparent 75%);
}
</style>
