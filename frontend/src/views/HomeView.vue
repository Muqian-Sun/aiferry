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
    默认首页（muqian 2026-09-22 定「正文三块」，9-23 定去底纹 / 不用卡片 / 位置重排）：
    ① 首屏（lg 起满一屏，左文右全球厂商环形图，自己转、不跟鼠标）→ ② 数字（一条横排，上下细线分隔）
    → ③ 五条特色（整幅左右交错，图一侧、字一侧，行间细线）→ 页脚。
    数据段拿不到就不出现，不放假数字。动效：各段 v-reveal 进视口淡入上浮；标题第二行流动渐变；数字进视口从 0 跳到位；示意图各自循环演示。
  -->
  <SiteShell v-else variant="public" flush>
    <div data-testid="default-home">
      <!--
        ① 首屏：lg 起占满一屏、内容垂直居中，左文右「全球模型厂商」环形图；窄屏只留文字，下面列目录里真有的厂商。
        文字列按序渐现，环形图晚 200ms。
      -->
      <section class="home-hero relative">
        <div class="mx-auto grid max-w-site items-center gap-12 px-6 pb-16 pt-14 sm:pt-20 lg:min-h-[calc(100svh_-_var(--af-topbar-h))] lg:grid-cols-[1.1fr_1fr] lg:gap-8 lg:py-12">
          <div v-reveal.stagger class="text-center lg:text-left" data-testid="hero-copy">
            <p class="section-eyebrow section-eyebrow-live" data-testid="hero-eyebrow">{{ t('userUi.home.hero.eyebrow') }}</p>
            <h1 class="mt-7 text-[2.5rem] font-semibold leading-[1.1] tracking-tight text-af-ink sm:text-5xl lg:text-6xl">
              {{ t('userUi.home.hero.title') }}<br />
              <span class="text-flow" data-testid="hero-title-accent">{{ t('userUi.home.hero.titleAccent') }}</span>
            </h1>
            <p class="mx-auto mt-6 max-w-xl text-base leading-7 text-af-ink-3 sm:text-lg sm:leading-8 lg:mx-0">{{ t('userUi.home.hero.description') }}</p>
            <div class="mt-10 flex flex-wrap items-center justify-center gap-3 lg:justify-start">
              <RouterLink :to="isAuthenticated ? consolePath : '/login'" class="btn btn-primary btn-pill btn-pill-arrow group" data-testid="home-primary-cta">
                {{ isAuthenticated ? t('userUi.home.hero.goToConsole') : t('userUi.home.hero.getStarted') }}
                <span class="btn-pill-icon"><Icon name="arrowRight" size="sm" /></span>
              </RouterLink>
              <RouterLink to="/model-plaza" class="btn btn-soft btn-pill" data-testid="home-secondary-cta">
                {{ t('userUi.home.hero.viewPricing') }}
              </RouterLink>
            </div>
            <p class="mt-7 flex items-center justify-center gap-2 text-13 text-af-ink-3 lg:justify-start" data-testid="hero-compat">
              <Icon name="checkCircle" size="sm" class="text-af-success" />
              {{ t('userUi.home.hero.compat') }}
            </p>
            <!-- 窄屏没有环形图：列目录里真有的厂商，拿不到目录就不出现 -->
            <div v-if="vendors.length" class="mt-12 lg:hidden">
              <p class="text-13 text-af-ink-4">{{ t('userUi.home.hero.vendorsLabel') }}</p>
              <VendorStrip class="mt-4 justify-center" :vendors="vendors" />
            </div>
          </div>
          <VendorOrbit v-reveal="200" :logo="siteLogo || '/logo.svg'" class="mx-auto hidden lg:block" />
        </div>
      </section>

      <!-- ② 数字：一条横排，上下细线；进视口时从 0 跳到位（.count-up 纯 CSS 计数，跟 v-reveal 联动） -->
      <section v-if="catalog.length" data-testid="home-stats-section">
        <div class="mx-auto max-w-site px-6">
          <dl
            v-reveal
            class="grid grid-cols-2 gap-y-8 border-y border-af-hairline py-10 sm:grid-cols-4 sm:gap-y-0 sm:divide-x sm:divide-af-hairline"
            data-testid="home-stats"
          >
            <div v-for="stat in stats" :key="stat.key" class="px-4 text-center">
              <dd class="text-4xl font-semibold tabular-nums text-af-ink sm:text-5xl">
                <span class="count-up" :style="{ '--count-to': stat.value }" aria-hidden="true" />
                <span class="sr-only" data-testid="home-stat-value">{{ stat.value }}</span>
              </dd>
              <dt class="mt-2 text-13 text-af-ink-3">{{ t(`userUi.home.stats.${stat.key}`) }}</dt>
            </div>
          </dl>
        </div>
      </section>

      <!-- ③ 特色：五条整幅左右交错——图一侧、字一侧，行与行之间一条细线 -->
      <section class="py-20 sm:py-24" data-testid="home-features">
        <div class="mx-auto max-w-site px-6">
          <div v-reveal class="max-w-2xl">
            <p class="section-eyebrow">{{ t('userUi.home.features.eyebrow') }}</p>
            <h2 class="section-title">{{ t('userUi.home.features.title') }}</h2>
            <p class="section-lead mx-0">{{ t('userUi.home.features.description') }}</p>
          </div>
          <ul class="mt-12 divide-y divide-af-hairline border-t border-af-hairline">
            <li
              v-for="(feature, index) in FEATURES"
              :key="feature"
              v-reveal
              class="grid items-center gap-8 py-12 lg:grid-cols-2 lg:gap-16"
              data-testid="home-feature"
            >
              <div :class="['min-w-0', index % 2 ? 'lg:order-2' : '']">
                <p class="font-mono text-13 text-af-ink-4">{{ String(index + 1).padStart(2, '0') }}</p>
                <h3 class="mt-3 text-2xl font-semibold tracking-tight text-af-ink">{{ t(`userUi.home.features.items.${feature}.title`) }}</h3>
                <p class="mt-3 max-w-lg text-base leading-7 text-af-ink-2">{{ t(`userUi.home.features.items.${feature}.body`) }}</p>
              </div>
              <HomeFigure :kind="feature" :class="index % 2 ? 'lg:order-1' : ''" />
            </li>
          </ul>
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
import VendorStrip from '@/components/user/home/VendorStrip.vue'
import VendorOrbit from '@/components/user/home/VendorOrbit.vue'
import HomeFigure, { type FigureKind } from '@/components/user/home/HomeFigure.vue'
import Icon from '@/components/icons/Icon.vue'
import { vReveal } from '@/directives/reveal'
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

// 自定义首页内容是 URL 时用 iframe 展示
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isAuthenticated = computed(() => authStore.isAuthenticated)
const consolePath = CONSOLE_HOME_PATH

/** 五张插画卡 = 五条特色（muqian 2026-09-22 定）：尽量透传 / 不做不同模型兜底 / 最大程度缓存 / 不记录用户数据 / 不出售用户数据 */
const FEATURES: readonly FigureKind[] = ['passthrough', 'failover', 'cache', 'privacy', 'noSale']

// 模型目录：与模型页同一个接口（对所有人开放）；拿不到（网络 / 空目录）就不渲染数字段，不放假数字
const catalog = ref<CatalogModel[]>([])
const vendors = computed(() => catalogVendors(catalog.value))
/** 数字段：前两个从目录算，后两个是产品事实（四条协议 / 「使用密钥」弹窗里有配置片段的客户端数） */
const stats = computed(() => [
  { key: 'models', value: catalog.value.length },
  { key: 'vendors', value: vendors.value.length },
  { key: 'protocols', value: PROTOCOL_ROUTES.length },
  { key: 'clients', value: HOME_CLIENTS.length }
])

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
