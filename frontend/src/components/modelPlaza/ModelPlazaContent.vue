<template>
  <!--
    模型页：厂商页签（彩色图标 + 计数）→ 工具（计费 / 搜索 / 价格单位，与页签同一行）→ 网格。
    只有网格一种视图（muqian 2026-09-23 去掉了表格）：hairline 分格的单元（不是卡片）。
    格子只放摘要（muqian 2026-09-30 方案 A）：图标 + 名称 + 厂商 · 计费方式 + 一行起价 + 该模型实际有的计费项标签；
    点格子打开详情抽屉（ModelPricingDrawer）按块列全部计费项、别名。
    价格单位只在工具行写一次；格子与抽屉里的价格已按访问者的倍率折算（倍率本身不显示）。
    embedded = 在控制台壳里（从控制台点进来，muqian 2026-09-30）：页头由壳画，这里只补一条数字摘要；
    否则公开壳，页首自己画——与首页首屏同一套（muqian 2026-09-23）：
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

    <!-- 控制台形态：页头由壳画，这里补一条数字摘要（上架模型 / 厂商），与其它列表页一致 -->
    <StatRow v-if="embedded && catalog.length" :items="consoleSummary" data-testid="plaza-console-summary" />

    <!-- 管理员配置的全局价格说明（Markdown） -->
    <div v-if="descriptionHtml" class="plaza-description text-sm text-af-ink-2" v-html="descriptionHtml"></div>

    <StatusState v-if="loading" kind="loading" :title="t('userUi.status.loading')" />
    <StatusState v-else-if="error" kind="error" :title="t('userUi.models.loadFailed')" />

    <!-- 列表整块在数据到位时淡入上浮一次；筛选、切视图不再重播 -->
    <div v-else v-reveal="120" class="space-y-6">
      <!--
        厂商页签与工具同一行（muqian：放到跟厂商一行）：左边页签（品牌色图标 + 计数，多了横向滚动），
        右边计费模式 / 搜索（/ 聚焦）/ 价格单位（muqian：不显示模型个数，搜索在单位左边）。价格单位只在这里写一次（格子里不再逐个写）。
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
          <Select v-if="billingModeOptions.length > 2" v-model="selectedBillingMode" :options="billingModeOptions" class="w-36" />
          <div ref="searchRef" class="plaza-search w-full sm:w-64">
            <SearchInput v-model="searchQuery" :placeholder="t('userUi.models.searchHint')" />
          </div>
          <p class="text-13 text-af-ink-3" data-testid="price-unit">
            {{ t('userUi.models.priceUnit') }}
          </p>
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
        <!--
          整格可点：鼠标点格子任意处打开详情（选中文字时不打开）；键盘 Tab 到模型名按钮，Enter / Space 打开。
          复制按钮单独处理、不冒泡。格子是 flex 列，标签行留一行高，没有标签的格子与有标签的一样高。
        -->
        <li
          v-for="entry in filtered"
          :key="entry.id"
          class="group flex min-w-0 cursor-pointer flex-col border-b border-af-hairline px-6 py-6 transition-colors hover:bg-af-sunken/60 focus-within:bg-af-sunken/60 sm:max-lg:[&:nth-child(2n)]:border-l lg:[&:not(:nth-child(3n+1))]:border-l"
          data-testid="catalog-cell"
          @click="onCellClick(entry)"
        >
          <div class="flex items-start gap-3">
            <VendorIcon :vendor="entry.vendor" :model="entry.id" :size="20" colored class="mt-0.5" />
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class="min-w-0 truncate rounded-sm text-left font-mono font-medium text-af-ink focus:outline-none focus-visible:ring-2 focus-visible:ring-af-brand/40"
                  aria-haspopup="dialog"
                  :title="t('userUi.models.openDetail')"
                  data-testid="catalog-cell-open"
                >
                  {{ entry.id }}
                </button>
                <button
                  type="button"
                  class="shrink-0 rounded p-1 text-af-ink-4 opacity-0 transition-opacity hover:text-af-ink focus:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100"
                  :aria-label="t('userUi.models.copyId')"
                  :title="copiedId === entry.id ? t('userUi.models.copied') : t('userUi.models.copyId')"
                  data-testid="copy-model-id"
                  @click.stop="copyId(entry.id)"
                >
                  <Icon :name="copiedId === entry.id ? 'check' : 'copy'" size="xs" />
                </button>
              </div>
              <p class="mt-0.5 text-13 text-af-ink-3">
                {{ vendorLabel(entry.vendor) }} · {{ getBillingModeLabel(entry.billingMode, t) }}
              </p>
            </div>
          </div>
          <!--
            格子只列基础计费项（muqian 2026-09-30「格子里面只写基础的输入输出、缓存读写」）：两列对齐，名在左、价在右，
            没定价的显示破折号；分段模型列第一段。分段价、Fast / Flex、1 小时缓存、图片音频、搜索、分时等都在抽屉里，
            格子底部一行灰字提示还有哪些。按次 / 图片 / 视频模式只有一个单价。单位见工具行。
          -->
          <dl class="mt-5 grid grid-cols-2 gap-x-8 gap-y-2 text-13 tabular-nums" data-testid="price-summary">
            <div v-for="item in cellItems(entry)" :key="item.key" class="flex items-baseline justify-between gap-3">
              <dt class="text-af-ink-4">{{ item.label }}</dt>
              <dd class="font-medium text-af-ink" :data-testid="`price-${item.key}`">{{ formatPrice(item.value) }}</dd>
            </div>
          </dl>
          <p v-if="cellExtras(entry).length" class="mt-4 truncate text-xs text-af-ink-3" data-testid="price-extras">
            {{ cellExtras(entry).join(' · ') }} ›
          </p>
        </li>
      </ul>

      <p class="max-w-3xl text-xs leading-5 text-af-ink-3">
        {{ t('userUi.models.priceNote') }}
      </p>
    </div>

    <ModelPricingDrawer
      :entry="detailEntry"
      :scale="priceScale"
      @close="detailId = null"
    />
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
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import { vReveal } from '@/directives/reveal'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { getBillingModeLabel } from '@/utils/billingMode'
import ModelPricingDrawer from './ModelPricingDrawer.vue'
import {
  applyMultiplier,
  buildCatalog,
  catalogBillingModes,
  catalogVendors,
  countByVendor,
  filterCatalog,
  formatCatalogPrice as formatPrice,
  vendorLabel,
  type CatalogModel
} from './catalog'

const props = defineProps<{
  response: ModelPlazaResponse | null
  loading: boolean
  error: boolean
  /** 在控制台壳里（不画页首，补数字摘要） */
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
const consoleSummary = computed<StatItem[]>(() => [
  { key: 'models', label: t('userUi.home.stats.models'), value: String(catalog.value.length) },
  { key: 'vendors', label: t('userUi.home.stats.vendors'), value: String(vendors.value.length) }
])
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

// 展示价 = 官方价 × 倍率：登录按账户（生效）倍率，未登录按接口给的全站默认倍率。
// 倍率与官方价只用来算，不在页面上给用户看（muqian 2026-09-30）
const isAuthenticated = computed(() => authStore.isAuthenticated)
const defaultMultiplier = computed(() => Number(props.response?.default_rate_multiplier ?? 1))
const priceScale = computed(() =>
  isAuthenticated.value ? Number(authStore.user?.rate_multiplier ?? defaultMultiplier.value) : defaultMultiplier.value
)

/**
 * 格子里的基础计费项：token 模式固定列输入 / 输出 / 缓存写 / 缓存读（分段模型列第一段，没定价的显示破折号）；
 * 按次 / 图片 / 视频模式只收一个单价，项名自带单位（每次 / 每张 / 每秒）。价格已乘访问者倍率。
 */
function cellItems(entry: CatalogModel): Array<{ key: string; label: string; value: number | null }> {
  const scale = priceScale.value
  if (entry.billingMode !== 'token') {
    const label =
      entry.billingMode === 'image'
        ? t('userUi.models.prices.perImage')
        : entry.billingMode === 'video'
          ? t('userUi.models.prices.perSecond')
          : t('userUi.models.prices.perRequest')
    return [{ key: 'unit', label, value: entry.unitPrice == null ? null : entry.unitPrice * scale }]
  }
  const price = applyMultiplier(entry.price, scale)
  return (['input', 'output', 'cacheWrite', 'cacheRead'] as const).map((key) => ({
    key,
    label: t(`userUi.models.prices.${key}`),
    value: price?.[key] ?? null
  }))
}

/** 格子底部的灰字：还有哪些计费项在抽屉里（只列这个模型实际有的，不写价格） */
function cellExtras(entry: CatalogModel): string[] {
  const extras: string[] = []
  const price = entry.price
  if (entry.rows.length > 1) extras.push(t('userUi.models.tags.segments', { count: entry.rows.length }))
  if (entry.tiers.length) extras.push(t('userUi.models.tags.tiers', { count: entry.tiers.length }))
  if (price?.cacheWrite1h != null) extras.push(t('userUi.models.tags.cache1h'))
  if (entry.fastRows) extras.push(t('userUi.models.tags.fast'))
  if (entry.flexMultiplier != null) extras.push(t('userUi.models.tags.flex'))
  if (price && (price.imageInput != null || price.imageOutput != null || price.imageCacheRead != null)) {
    extras.push(t('userUi.models.tags.image'))
  }
  if (price && (price.audioInput != null || price.audioOutput != null)) extras.push(t('userUi.models.tags.audio'))
  if (entry.searchPerThousand != null || entry.toolSearchPerThousand != null) extras.push(t('userUi.models.tags.search'))
  if (entry.maxReasoningMultiplier != null) extras.push(t('userUi.models.tags.maxReasoning'))
  if (entry.timePricing) extras.push(t('userUi.models.tags.timePricing'))
  return extras
}

// 详情抽屉：记模型 id，目录刷新后仍指向同一条（下架了就自动关掉）
const detailId = ref<string | null>(null)
const detailEntry = computed(() => (detailId.value ? (catalog.value.find((entry) => entry.id === detailId.value) ?? null) : null))
function onCellClick(entry: CatalogModel) {
  // 拖选格子里的文字（比如模型名）时不打开
  if (window.getSelection()?.toString()) return
  detailId.value = entry.id
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
