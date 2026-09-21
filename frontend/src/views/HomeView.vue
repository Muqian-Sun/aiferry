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

  <!-- 默认首页：首屏（文案 + 航线图）→ 接入三步 → 可核对的事 -->
  <SiteShell v-else variant="public">
    <div data-testid="default-home">
      <section class="grid gap-12 py-12 lg:grid-cols-12 lg:items-center lg:gap-10 lg:py-20">
        <div class="min-w-0 lg:col-span-5">
          <h1 class="text-28 font-semibold text-af-ink sm:text-4xl sm:leading-[1.15]">
            {{ t('userUi.home.heroTitle') }}
          </h1>
          <p class="mt-5 max-w-lg text-base leading-7 text-af-ink-2">{{ t('userUi.home.heroDescription') }}</p>
          <div class="mt-8 flex flex-wrap items-center gap-x-6 gap-y-3">
            <RouterLink :to="isAuthenticated ? consolePath : '/login'" class="btn btn-primary" data-testid="home-primary-cta">
              {{ isAuthenticated ? t('userUi.home.goToConsole') : t('userUi.home.getStarted') }}
            </RouterLink>
            <RouterLink v-if="showModelPlazaEntry" to="/model-plaza" class="text-sm font-medium text-af-ink-2 hover:text-af-ink">
              {{ t('userUi.home.viewPricing') }}
            </RouterLink>
          </div>
        </div>
        <div class="min-w-0 lg:col-span-7">
          <RouteMap :site-name="siteName" :logo="siteLogo || '/logo.svg'" />
        </div>
      </section>

      <section class="border-t border-af-hairline py-12">
        <h2 class="text-base font-semibold text-af-ink">{{ t('userUi.home.steps.title') }}</h2>
        <ol class="mt-6 grid gap-8 sm:grid-cols-3">
          <li v-for="(step, index) in steps" :key="step" class="flex gap-3">
            <span class="w-4 shrink-0 text-sm tabular-nums text-af-ink-4">{{ index + 1 }}.</span>
            <div>
              <h3 class="text-sm font-medium text-af-ink">{{ t(`userUi.home.steps.${step}.title`) }}</h3>
              <p class="mt-1 text-13 leading-5 text-af-ink-3">{{ t(`userUi.home.steps.${step}.body`) }}</p>
            </div>
          </li>
        </ol>
      </section>

      <section class="border-t border-af-hairline py-12">
        <h2 class="text-base font-semibold text-af-ink">{{ t('userUi.home.facts.title') }}</h2>
        <dl class="mt-6 grid gap-x-10 gap-y-8 sm:grid-cols-2 lg:grid-cols-4">
          <div v-for="fact in facts" :key="fact">
            <dt class="text-sm font-medium text-af-ink">{{ t(`userUi.home.facts.${fact}.title`) }}</dt>
            <dd class="mt-1 text-13 leading-5 text-af-ink-3">{{ t(`userUi.home.facts.${fact}.body`) }}</dd>
          </div>
        </dl>
      </section>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import RouteMap from '@/components/user/home/RouteMap.vue'
import { CONSOLE_HOME_PATH } from '@/components/user/shell/navItems'
import { sanitizeUrl } from '@/utils/url'
import { DEFAULT_SITE_NAME, DEFAULT_SITE_SUBTITLE } from '@/utils/branding'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

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
// 首屏次入口：模型页开关 + 可选强制登录（顶栏的页签也走同一逻辑）
const modelPlazaRequiresAuth = computed(() => appStore.cachedPublicSettings?.model_plaza_require_auth === true)
const showModelPlazaEntry = computed(
  () => isFeatureFlagEnabled(FeatureFlags.modelPlaza) && (isAuthenticated.value || !modelPlazaRequiresAuth.value)
)

const steps = ['create', 'baseUrl', 'watch'] as const
const facts = ['protocol', 'pricing', 'ledger', 'balance'] as const

onMounted(() => {
  authStore.checkAuth()
  // 注入配置缺失时（纯静态部署）再拉一次公开设置
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
