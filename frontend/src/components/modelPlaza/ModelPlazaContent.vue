<template>
  <!--
    模型页：厂商页签（彩色图标 + 计数）→ 工具行（搜索 / 计费 / 计数 / 价格单位）→ 网格。
    只有网格一种视图（muqian 2026-09-23 去掉了表格）：hairline 分格的单元（不是卡片），图标 + 名称 + 厂商 + 全部计费项 + 别名。
    价格单位只在工具行写一次；登录且账户倍率 ≠ 1 时格子里直接显示折算后的你的价格，工具行注明倍率。
    embedded=已登录（控制台壳提供页头）；否则公开壳，这里自己画页首——与首页首屏同一套（muqian 2026-09-23）：
    一行大字「全部模型，明码标价」（后半句流动光泽，muqian：放一行）+ 一句说明逐行淡入上浮，右侧模型数 / 厂商数进视口从 0 跳到位；厂商图标与首页一样用品牌色。
  -->
  <div class="space-y-6">
    <!-- 页首收紧（muqian：占的空间过大）：标题 40px、说明一行、数字小一档，整块约 120px 高 -->
    <div v-if="!embedded" class="grid gap-6 pt-2 sm:pt-4 lg:grid-cols-[1fr_auto] lg:items-end">
      <header v-reveal.stagger data-testid="plaza-hero">
        <h1 class="text-[2rem] font-semibold leading-tight tracking-[-0.02em] text-af-ink sm:text-[2.5rem]">
          {{ t('userUi.models.hero.title') }}<span class="text-flow">{{ t('userUi.models.hero.titleAccent') }}</span>
        </h1>
        <p class="mt-3 max-w-2xl text-[15px] leading-7 text-af-ink-2">{{ t('userUi.models.hero.description') }}</p>
      </header>
      <!-- 数字：目录加载完才出现，自己挂 v-reveal，出现时才从 0 跳到位 -->
      <dl v-if="catalog.length" v-reveal="200" class="flex divide-x divide-af-hairline" data-testid="plaza-stats">
        <div v-for="stat in stats" :key="stat.key" class="px-6 first:pl-0 last:pr-0">
          <dd class="text-[1.75rem] font-semibold leading-none tabular-nums text-af-ink">
            <span class="count-up" :style="{ '--count-to': stat.value }" aria-hidden="true" />
            <span class="sr-only">{{ stat.value }}</span>
          </dd>
          <dt class="mt-2 text-xs text-af-ink-3">{{ t(`userUi.home.stats.${stat.key}`) }}</dt>
        </div>
      </dl>
    </div>

    <!-- 管理员配置的全局价格说明（Markdown） -->
    <div v-if="descriptionHtml" class="plaza-description text-sm text-af-ink-2" v-html="descriptionHtml"></div>

    <StatusState v-if="loading" kind="loading" :title="t('userUi.status.loading')" />
    <StatusState v-else-if="error" kind="error" :title="t('userUi.models.loadFailed')" />

    <!-- 列表整块在数据到位时淡入上浮一次；筛选、切视图不再重播 -->
    <div v-else v-reveal="120" class="space-y-6">
      <!--
        厂商页签与工具同一行（muqian：放到跟厂商一行）：左边页签（品牌色图标 + 计数，多了横向滚动），
        右边计数 / 价格单位 / 计费模式 / 搜索（/ 聚焦）。价格单位只在这里写一次（格子里不再逐个写）。
        lg 以下工具掉到页签下面一行；lg 起整行一条底线，页签撑满行高、激活下划线压在底线上。
      -->
      <div class="flex flex-col gap-3 lg:flex-row lg:items-stretch lg:gap-8 lg:border-b lg:border-af-hairline">
        <div
          class="flex min-w-0 flex-1 gap-1 overflow-x-auto border-b border-af-hairline scrollbar-hide lg:border-b-0"
          role="tablist"
          :aria-label="t('userUi.models.vendorTabsLabel')"
          data-testid="vendor-tabs"
        >
          <button
            v-for="tab in vendorTabs"
            :key="tab.key"
            type="button"
            role="tab"
            :class="tabClass(selectedVendor === tab.key)"
            :aria-selected="selectedVendor === tab.key"
            :data-testid="`vendor-tab-${tab.key}`"
            @click="selectVendor(tab.key)"
          >
            <VendorIcon v-if="tab.key !== 'all'" :vendor="tab.key" :size="16" colored />
            {{ tab.label }}
            <span class="tabular-nums text-af-ink-4">{{ tab.count }}</span>
          </button>
        </div>
        <div class="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-2 lg:flex-nowrap lg:py-2">
          <span class="text-13 tabular-nums text-af-ink-3" data-testid="catalog-count">
            {{ t('userUi.models.count', { count: filtered.length }) }}
          </span>
          <p class="text-13 text-af-ink-3" data-testid="price-unit">
            {{ t('userUi.models.priceUnit') }}
            <span v-if="showUserPrice" class="text-af-ink" data-testid="your-price-note">
              · {{ t('userUi.models.yourPriceApplied', { multiplier: userMultiplier }) }}
            </span>
          </p>
          <Select v-if="billingModeOptions.length > 2" v-model="selectedBillingMode" :options="billingModeOptions" class="w-36" />
          <div ref="searchRef" class="plaza-search w-full sm:w-64">
            <SearchInput v-model="searchQuery" :placeholder="t('userUi.models.searchHint')" />
          </div>
        </div>
      </div>

      <StatusState
        v-if="filtered.length === 0"
        kind="empty"
        :title="searchActive ? t('userUi.models.noSearchResult') : t('userUi.models.empty')"
      />

      <!--
        网格：hairline 分格，不是卡片。竖线只画在同一行里非第一个格子的左边：
        sm–lg 两列（偶数格），lg 起三列（非 3n+1 格）——两条规则按断点互斥，不能互相覆盖。
      -->
      <ul v-else class="-mx-6 grid border-t border-af-hairline sm:grid-cols-2 lg:grid-cols-3" data-testid="catalog-grid">
        <li
          v-for="entry in filtered"
          :key="entry.id"
          class="group min-w-0 border-b border-af-hairline px-6 py-6 sm:max-lg:[&:nth-child(2n)]:border-l lg:[&:not(:nth-child(3n+1))]:border-l"
          data-testid="catalog-cell"
        >
          <div class="flex items-start gap-3">
            <VendorIcon :vendor="entry.vendor" :size="20" colored class="mt-0.5" />
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="truncate font-mono font-medium text-af-ink">{{ entry.id }}</span>
                <button
                  type="button"
                  class="rounded p-1 text-af-ink-4 opacity-0 transition-opacity hover:text-af-ink focus:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100"
                  :aria-label="t('userUi.models.copyId')"
                  :title="copiedId === entry.id ? t('userUi.models.copied') : t('userUi.models.copyId')"
                  @click="copyId(entry.id)"
                >
                  <Icon :name="copiedId === entry.id ? 'check' : 'copy'" size="xs" />
                </button>
              </div>
              <p class="mt-0.5 text-13 text-af-ink-3">
                {{ vendorLabel(entry.vendor) }} · {{ getBillingModeLabel(entry.billingMode, t) }}
              </p>
            </div>
          </div>
          <!-- 计费项：两列对齐，名在左、价在右；单位见工具行 -->
          <dl class="mt-5 grid grid-cols-2 gap-x-8 gap-y-2 text-13 tabular-nums">
            <div v-for="item in priceItems(entry)" :key="item.key" class="flex items-baseline justify-between gap-3">
              <dt class="text-af-ink-4">{{ item.label }}</dt>
              <dd class="font-medium text-af-ink" :data-testid="`price-${item.key}`">{{ formatPrice(item.value) }}</dd>
            </div>
          </dl>
          <div v-if="entry.aliases.length || entry.timePricing" class="mt-4 flex flex-wrap gap-1.5">
            <span v-for="alias in entry.aliases" :key="alias" class="badge badge-gray font-mono">{{ alias }}</span>
            <span v-if="entry.timePricing" class="badge badge-gray" :title="timePricingText(entry)" data-testid="time-pricing-badge">
              {{ t('userUi.models.timePricing') }}
            </span>
          </div>
        </li>
      </ul>

      <p class="max-w-3xl text-xs leading-5 text-af-ink-3">
        {{ t('userUi.models.priceNote') }}
        <template v-if="isAuthenticated"> {{ t('userUi.models.multiplierNote', { multiplier: userMultiplier }) }}</template>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import VendorIcon from '@/components/common/VendorIcon.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import { vReveal } from '@/directives/reveal'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { getBillingModeLabel } from '@/utils/billingMode'
import {
  applyMultiplier,
  buildCatalog,
  catalogBillingModes,
  catalogVendors,
  countByVendor,
  filterCatalog,
  formatCatalogPrice as formatPrice,
  formatTimePricing,
  vendorLabel,
  type CatalogModel,
  type CatalogPriceKey
} from './catalog'

const props = defineProps<{
  response: ModelPlazaResponse | null
  loading: boolean
  error: boolean
  embedded?: boolean
}>()

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const searchQuery = ref('')
const selectedBillingMode = ref('all')
const copiedId = ref<string | null>(null)
const searchRef = ref<HTMLElement | null>(null)

// 厂商记在 URL（?vendor=），刷新与分享都保留
function readVendor(): string {
  const value = route.query.vendor
  return typeof value === 'string' && value ? value : 'all'
}
const selectedVendor = ref(readVendor())
watch(() => route.query, () => {
  selectedVendor.value = readVendor()
})
function replaceQuery(patch: Record<string, string | undefined>) {
  const query: Record<string, string> = {}
  for (const [key, value] of Object.entries({ ...route.query, ...patch })) {
    if (typeof value === 'string' && value) query[key] = value
  }
  void router.replace({ query })
}
function selectVendor(vendor: string) {
  selectedVendor.value = vendor
  replaceQuery({ vendor: vendor === 'all' ? undefined : vendor })
}

const descriptionHtml = computed(() => {
  const md = props.response?.description?.trim()
  if (!md) return ''
  return DOMPurify.sanitize(marked.parse(md) as string)
})

const catalog = computed(() => buildCatalog(props.response?.models ?? []))
const vendors = computed(() => catalogVendors(catalog.value))
/** 页首数字：与首页数字段同一口径（上架模型数 / 厂商数） */
const stats = computed(() => [
  { key: 'models', value: catalog.value.length },
  { key: 'vendors', value: vendors.value.length }
])
const vendorTabs = computed(() => {
  const counts = countByVendor(catalog.value)
  return [
    { key: 'all', label: t('userUi.models.allVendors'), count: catalog.value.length },
    ...vendors.value.map((vendor) => ({ key: vendor, label: vendorLabel(vendor), count: counts.get(vendor) ?? 0 }))
  ]
})
const billingModeOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('userUi.models.allBilling') },
  ...catalogBillingModes(catalog.value).map((mode) => ({ value: mode, label: getBillingModeLabel(mode, t) }))
])
const searchActive = computed(() => searchQuery.value.trim() !== '')
const filtered = computed(() =>
  filterCatalog(catalog.value, searchQuery.value, selectedVendor.value, selectedBillingMode.value)
)

// 数据刷新后失效的筛选回到全部
watch(vendors, (list) => {
  if (selectedVendor.value !== 'all' && !list.includes(selectedVendor.value)) selectVendor('all')
})
watch(billingModeOptions, (options) => {
  if (!options.some((option) => option.value === selectedBillingMode.value)) selectedBillingMode.value = 'all'
})

// 你的价格：登录且账户倍率 ≠ 1 才多一组列；倍率 = 1 时标价即实付，只在脚注说明
const isAuthenticated = computed(() => authStore.isAuthenticated)
const userMultiplier = computed(() => Number(authStore.user?.rate_multiplier ?? 1))
const showUserPrice = computed(() => isAuthenticated.value && userMultiplier.value !== 1)
/**
 * 一个格子要列的计费项（口径 = 后端实际收费项）：
 * token 模式固定列输入 / 输出 / 缓存写入 / 缓存读取（没定价的显示破折号），1 小时缓存写入与图片输入输出只在定了价时列；
 * 按次 / 图片 / 视频模式只收一个单价，项名自带单位（每次 / 每张 / 每秒）。倍率 ≠ 1 时列的是折算后的价格。
 */
const TOKEN_PRICE_ITEMS: ReadonlyArray<{ key: CatalogPriceKey; always: boolean }> = [
  { key: 'input', always: true },
  { key: 'output', always: true },
  { key: 'cacheWrite', always: true },
  { key: 'cacheRead', always: true },
  { key: 'cacheWrite1h', always: false },
  { key: 'imageInput', always: false },
  { key: 'imageOutput', always: false }
]
const UNIT_PRICE_LABEL: Record<string, string> = { per_request: 'perRequest', image: 'perImage', video: 'perSecond' }

function priceItems(entry: CatalogModel): Array<{ key: string; label: string; value: number | null }> {
  const scale = showUserPrice.value ? userMultiplier.value : 1
  if (entry.billingMode !== 'token') {
    const labelKey = UNIT_PRICE_LABEL[entry.billingMode]
    return [{ key: 'unit', label: t(`userUi.models.prices.${labelKey}`), value: entry.unitPrice == null ? null : entry.unitPrice * scale }]
  }
  const price = applyMultiplier(entry.price, scale)
  return TOKEN_PRICE_ITEMS.filter(({ key, always }) => always || price?.[key] != null).map(({ key }) => ({
    key,
    label: t(`userUi.models.prices.${key}`),
    value: price?.[key] ?? null
  }))
}
function timePricingText(entry: CatalogModel): string {
  return entry.timePricing ? formatTimePricing(entry.timePricing, t('userUi.models.weekdaysOnly')) : ''
}

function tabClass(active: boolean): string {
  return [
    'relative -mb-px inline-flex min-h-10 shrink-0 items-center gap-1.5 whitespace-nowrap px-3 text-sm font-medium transition-colors',
    'border-b-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-af-brand/40',
    active ? 'border-af-brand text-af-ink' : 'border-transparent text-af-ink-2 hover:text-af-ink'
  ].join(' ')
}

// 「/」聚焦搜索（焦点不在输入框里时）
function onKeydown(event: KeyboardEvent) {
  if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) return
  const target = event.target as HTMLElement | null
  if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) return
  const input = searchRef.value?.querySelector('input')
  if (!input) return
  event.preventDefault()
  input.focus()
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

let copiedTimer: ReturnType<typeof setTimeout> | null = null
async function copyId(id: string) {
  const ok = await copyToClipboard(id)
  if (!ok) return
  copiedId.value = id
  if (copiedTimer) clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => (copiedId.value = null), 1500)
}
</script>

<style scoped>
/* 与页签同一行：搜索框压到 34px 高（通用 .input 是 42px），行高由页签与它一起定 */
.plaza-search :deep(.input) {
  @apply py-1.5;
}
.plaza-description :deep(p) {
  @apply mb-2 last:mb-0;
}
.plaza-description :deep(a) {
  @apply text-af-brand underline underline-offset-2 hover:text-af-brand-hover;
}
.plaza-description :deep(ul),
.plaza-description :deep(ol) {
  @apply mb-2 list-disc pl-5;
}
.plaza-description :deep(code) {
  @apply rounded bg-af-sunken px-1 py-0.5 font-mono text-xs text-af-ink;
}
</style>
