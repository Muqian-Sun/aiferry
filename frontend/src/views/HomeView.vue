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
        {{ isAuthenticated ? t('userUi.home.goToConsole') : t('userUi.nav.login') }}
      </RouterLink>
    </div>
  </SiteShell>

  <!--
    默认首页：一个产品页该有的顺序——先看到产品（首屏代码块），再看到规模（数字带）、看到货（模型与官方价）、
    看到契约（四条协议）、看到路径（三步），最后是可核对的事。模型相关两节由 /model-plaza 数据驱动，拿不到就不出现。
  -->
  <SiteShell v-else variant="public">
    <div data-testid="default-home">
      <section class="grid gap-10 py-14 lg:grid-cols-12 lg:items-center lg:gap-12 lg:py-20">
        <div class="min-w-0 lg:col-span-6">
          <h1 class="whitespace-pre-line text-28 font-semibold text-af-ink sm:text-4xl lg:text-44">
            {{ t('userUi.home.heroTitle') }}
          </h1>
          <p class="mt-5 max-w-xl text-base leading-7 text-af-ink-2">{{ t('userUi.home.heroDescription') }}</p>
          <div class="mt-8 flex flex-wrap items-center gap-3">
            <RouterLink :to="isAuthenticated ? consolePath : '/login'" class="btn btn-primary btn-lg" data-testid="home-primary-cta">
              {{ isAuthenticated ? t('userUi.home.goToConsole') : t('userUi.home.getStarted') }}
            </RouterLink>
            <RouterLink to="/model-plaza" class="btn btn-secondary btn-lg">
              {{ t('userUi.home.viewPricing') }}
            </RouterLink>
          </div>
        </div>
        <div class="min-w-0 lg:col-span-6">
          <CodeSample :base-url="apiBaseUrl" />
        </div>
      </section>

      <!-- 数字带：只在拿到模型目录时出现，数字都从目录算 -->
      <dl v-if="catalog.length" class="grid grid-cols-2 border-y border-af-hairline lg:grid-cols-4 lg:divide-x lg:divide-af-hairline" data-testid="home-stats">
        <div class="py-6 pr-6 lg:px-6 lg:first:pl-0">
          <dt class="text-13 text-af-ink-3">{{ t('userUi.home.stats.models') }}</dt>
          <dd class="mt-1 text-2xl font-semibold tabular-nums text-af-ink">{{ catalog.length }}</dd>
        </div>
        <div class="py-6 pl-6 lg:px-6">
          <dt class="text-13 text-af-ink-3">{{ t('userUi.home.stats.vendors') }}</dt>
          <dd class="mt-1 text-2xl font-semibold tabular-nums text-af-ink">{{ vendorCount }}</dd>
        </div>
        <div class="border-t border-af-hairline py-6 pr-6 lg:border-t-0 lg:px-6">
          <dt class="text-13 text-af-ink-3">{{ t('userUi.home.stats.protocols') }}</dt>
          <dd class="mt-1 text-2xl font-semibold tabular-nums text-af-ink">{{ PROTOCOL_ROUTES.length }}</dd>
        </div>
        <div class="border-t border-af-hairline py-6 pl-6 lg:border-t-0 lg:px-6 lg:last:pr-0">
          <dt class="text-13 text-af-ink-3">{{ t('userUi.home.stats.ledgerLabel') }}</dt>
          <dd class="mt-1 text-2xl font-semibold text-af-ink">{{ t('userUi.home.stats.ledgerValue') }}</dd>
        </div>
      </dl>

      <!-- 模型与标价：模型页同一张表的前几行 -->
      <section v-if="catalogPreview.length" class="py-16">
        <div class="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h2 class="text-xl font-semibold text-af-ink">{{ t('userUi.home.catalog.title') }}</h2>
            <p class="mt-1 text-sm text-af-ink-3">{{ t('userUi.home.catalog.description') }}</p>
          </div>
          <RouterLink to="/model-plaza" class="text-sm font-medium text-af-brand hover:text-af-brand-hover">
            {{ t('userUi.home.catalog.viewAll', { count: catalog.length }) }}
          </RouterLink>
        </div>
        <div class="mt-6">
          <HomeCatalog :entries="catalogPreview" />
        </div>
      </section>

      <!-- 四条官方协议 -->
      <section class="border-t border-af-hairline py-16">
        <h2 class="text-xl font-semibold text-af-ink">{{ t('userUi.home.protocols.title') }}</h2>
        <p class="mt-1 max-w-2xl text-sm text-af-ink-3">{{ t('userUi.home.protocols.description') }}</p>
        <ul class="mt-8 grid gap-8 sm:grid-cols-2 lg:grid-cols-4">
          <li v-for="route in PROTOCOL_ROUTES" :key="route.key" class="min-w-0">
            <h3 class="text-lg font-semibold text-af-ink">{{ t(`userUi.home.routeMap.routes.${route.key}`) }}</h3>
            <code class="mt-2 block break-all font-mono text-xs text-af-ink-3">{{ route.method }} {{ route.path }}</code>
            <p class="mt-2 text-sm text-af-ink-2">{{ t(`userUi.home.routeMap.vendors.${route.key}`) }}</p>
          </li>
        </ul>
      </section>

      <!-- 接入三步 -->
      <section class="border-t border-af-hairline py-16">
        <h2 class="text-xl font-semibold text-af-ink">{{ t('userUi.home.steps.title') }}</h2>
        <ol class="mt-8 grid gap-8 sm:grid-cols-3">
          <li v-for="(step, index) in steps" :key="step" class="flex gap-4">
            <span class="w-5 shrink-0 text-base tabular-nums text-af-ink-4">{{ index + 1 }}.</span>
            <div class="min-w-0">
              <h3 class="text-base font-semibold text-af-ink">{{ t(`userUi.home.steps.${step}.title`) }}</h3>
              <p class="mt-1.5 text-sm leading-6 text-af-ink-3">{{ t(`userUi.home.steps.${step}.body`) }}</p>
            </div>
          </li>
        </ol>
      </section>

      <!-- 可核对的事 -->
      <section class="border-t border-af-hairline py-16">
        <h2 class="text-xl font-semibold text-af-ink">{{ t('userUi.home.facts.title') }}</h2>
        <dl class="mt-8 grid gap-x-10 gap-y-8 sm:grid-cols-2 lg:grid-cols-4">
          <div v-for="fact in facts" :key="fact">
            <dt class="text-base font-semibold text-af-ink">{{ t(`userUi.home.facts.${fact}.title`) }}</dt>
            <dd class="mt-1.5 text-sm leading-6 text-af-ink-3">{{ t(`userUi.home.facts.${fact}.body`) }}</dd>
          </div>
        </dl>
      </section>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import CodeSample from '@/components/user/home/CodeSample.vue'
import HomeCatalog from '@/components/user/home/HomeCatalog.vue'
import { PROTOCOL_ROUTES } from '@/components/user/home/protocols'
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

// 自定义首页内容是 URL 时用 iframe 展示
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isAuthenticated = computed(() => authStore.isAuthenticated)
const consolePath = CONSOLE_HOME_PATH

const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || appStore.apiBaseUrl || '')

const steps = ['create', 'baseUrl', 'watch'] as const
const facts = ['protocol', 'pricing', 'ledger', 'balance'] as const

// 模型目录：与模型页同一个接口（对所有人开放）；拿不到（网络 / 空目录）就不渲染数字带与价目预览，不放假数字
const HOME_CATALOG_ROWS = 8
const catalog = ref<CatalogModel[]>([])
const vendorCount = computed(() => catalogVendors(catalog.value).length)
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
