<template>
  <!--
    模型页：厂商页签（图标 + 计数）→ 工具行（搜索 / 计费 / 计数 / 视图切换）→ 表格或网格。
    表格每个上架条目一行，标价按百万 Token；登录且账户倍率 ≠ 1 时多一组「你的价格」列。
    网格视图是 hairline 分格的单元（不是卡片）：图标 + 名称 + 厂商 + 标价 + 别名。
    embedded=已登录（控制台壳提供页头）；否则公开壳，这里自己画标题。
  -->
  <div class="space-y-6">
    <div v-if="!embedded" class="border-b border-af-hairline pb-4">
      <h1 class="text-xl font-semibold text-af-ink">{{ t('userUi.models.title') }}</h1>
      <p class="mt-1 text-13 text-af-ink-3">{{ t('userUi.models.description') }}</p>
    </div>

    <!-- 管理员配置的全局价格说明（Markdown） -->
    <div v-if="descriptionHtml" class="plaza-description text-sm text-af-ink-2" v-html="descriptionHtml"></div>

    <StatusState v-if="loading" kind="loading" :title="t('userUi.status.loading')" />
    <StatusState v-else-if="error" kind="error" :title="t('userUi.models.loadFailed')" />

    <template v-else>
      <!-- 厂商页签：全部 + 目录里出现过的厂商，带图标与计数 -->
      <div class="flex gap-1 overflow-x-auto border-b border-af-hairline scrollbar-hide" role="tablist" :aria-label="t('userUi.models.columns.vendor')" data-testid="vendor-tabs">
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
          <VendorIcon v-if="tab.key !== 'all'" :vendor="tab.key" :size="16" class="text-af-ink-3" />
          {{ tab.label }}
          <span class="tabular-nums text-af-ink-4">{{ tab.count }}</span>
        </button>
      </div>

      <!-- 工具行：搜索（/ 聚焦）/ 计费模式 / 计数 / 视图 -->
      <div class="flex flex-wrap items-center gap-3">
        <div ref="searchRef" class="w-full sm:w-72">
          <SearchInput v-model="searchQuery" :placeholder="t('userUi.models.searchHint')" />
        </div>
        <Select v-if="billingModeOptions.length > 2" v-model="selectedBillingMode" :options="billingModeOptions" class="w-40" />
        <span class="text-13 tabular-nums text-af-ink-3" data-testid="catalog-count">
          {{ t('userUi.models.count', { count: filtered.length }) }}
        </span>
        <div class="ml-auto flex items-center gap-0.5" role="group" :aria-label="t('userUi.models.view.label')">
          <button
            v-for="option in viewOptions"
            :key="option.key"
            type="button"
            class="rounded-md p-1.5 transition-colors"
            :class="view === option.key ? 'bg-af-sunken text-af-ink' : 'text-af-ink-4 hover:text-af-ink'"
            :aria-pressed="view === option.key"
            :title="option.label"
            :aria-label="option.label"
            :data-testid="`view-${option.key}`"
            @click="setView(option.key)"
          >
            <Icon :name="option.icon" size="sm" />
          </button>
        </div>
      </div>

      <StatusState
        v-if="filtered.length === 0"
        kind="empty"
        :title="searchActive ? t('userUi.models.noSearchResult') : t('userUi.models.empty')"
      />

      <!-- 表格视图 -->
      <div v-else-if="view === 'table'" class="-mx-6 overflow-x-auto">
        <table class="w-full min-w-[720px] text-13" data-testid="catalog-table">
          <thead>
            <tr class="border-b border-af-hairline text-left text-af-ink-3">
              <th class="py-2 pl-6 pr-4 font-medium">{{ t('userUi.models.columns.model') }}</th>
              <th class="py-2 pr-4 font-medium">{{ t('userUi.models.columns.vendor') }}</th>
              <th class="py-2 pr-4 font-medium">{{ t('userUi.models.columns.billing') }}</th>
              <th class="py-2 pr-4 text-right font-medium" colspan="3">
                {{ t('userUi.models.listPrice') }}
                <span class="ml-1 font-normal text-af-ink-4">{{ t('userUi.models.perMillion') }}</span>
              </th>
              <th v-if="showUserPrice" class="py-2 pr-6 text-right font-medium text-af-brand" colspan="2" data-testid="user-price-header">
                {{ t('userUi.models.yourPrice') }}
                <span class="ml-1 font-normal text-af-ink-4">{{ t('userUi.models.yourPriceHint', { multiplier: userMultiplier }) }}</span>
              </th>
            </tr>
            <tr class="border-b border-af-hairline text-left text-xs text-af-ink-4">
              <th class="py-1.5 pl-6 pr-4 font-normal" colspan="3"></th>
              <th class="py-1.5 pr-4 text-right font-normal">{{ t('userUi.models.columns.input') }}</th>
              <th class="py-1.5 pr-4 text-right font-normal">{{ t('userUi.models.columns.output') }}</th>
              <th class="py-1.5 text-right font-normal" :class="showUserPrice ? 'pr-4' : 'pr-6'">{{ t('userUi.models.columns.cacheRead') }}</th>
              <template v-if="showUserPrice">
                <th class="py-1.5 pr-4 text-right font-normal">{{ t('userUi.models.columns.input') }}</th>
                <th class="py-1.5 pr-6 text-right font-normal">{{ t('userUi.models.columns.output') }}</th>
              </template>
            </tr>
          </thead>
          <tbody class="divide-y divide-af-hairline">
            <tr v-for="entry in filtered" :key="entry.id" class="group h-11 hover:bg-af-sunken" data-testid="catalog-row">
              <td class="py-2 pl-6 pr-4">
                <div class="flex items-center gap-2">
                  <span class="font-mono font-medium text-af-ink">{{ entry.id }}</span>
                  <span v-if="entry.timePricing" class="badge badge-gray" :title="timePricingText(entry)" data-testid="time-pricing-badge">
                    {{ t('userUi.models.timePricing') }}
                  </span>
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
                <div v-if="entry.aliases.length" class="mt-0.5 font-mono text-xs text-af-ink-4">{{ entry.aliases.join(' · ') }}</div>
              </td>
              <td class="pr-4 text-af-ink-2">{{ vendorLabel(entry.vendor) }}</td>
              <td class="pr-4 text-af-ink-3">{{ getBillingModeLabel(entry.billingMode, t) }}</td>
              <td class="pr-4 text-right tabular-nums text-af-ink">{{ formatPrice(entry.price?.input) }}</td>
              <td class="pr-4 text-right tabular-nums text-af-ink">{{ formatPrice(entry.price?.output) }}</td>
              <td class="text-right tabular-nums text-af-ink-2" :class="showUserPrice ? 'pr-4' : 'pr-6'">{{ formatPrice(entry.price?.cacheRead) }}</td>
              <template v-if="showUserPrice">
                <td class="pr-4 text-right tabular-nums text-af-ink" data-testid="user-price-input">{{ formatPrice(userPrice(entry)?.input) }}</td>
                <td class="pr-6 text-right tabular-nums text-af-ink">{{ formatPrice(userPrice(entry)?.output) }}</td>
              </template>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 网格视图：hairline 分格，不是卡片 -->
      <ul v-else class="-mx-6 grid border-t border-af-hairline sm:grid-cols-2 lg:grid-cols-3" data-testid="catalog-grid">
        <li
          v-for="entry in filtered"
          :key="entry.id"
          class="group min-w-0 border-b border-af-hairline px-6 py-5 sm:[&:nth-child(2n)]:border-l lg:[&:nth-child(2n)]:border-l-0 lg:[&:not(:nth-child(3n+1))]:border-l"
          data-testid="catalog-cell"
        >
          <div class="flex items-start gap-3">
            <VendorIcon :vendor="entry.vendor" :size="20" class="mt-0.5 text-af-ink-3" />
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
          <dl class="mt-4 flex items-baseline gap-4 text-13 tabular-nums">
            <div>
              <dt class="inline text-af-ink-4">{{ t('userUi.models.columns.input') }}</dt>
              <dd class="inline font-medium text-af-ink">{{ formatPrice((showUserPrice ? userPrice(entry) : entry.price)?.input) }}</dd>
            </div>
            <div>
              <dt class="inline text-af-ink-4">{{ t('userUi.models.columns.output') }}</dt>
              <dd class="inline font-medium text-af-ink">{{ formatPrice((showUserPrice ? userPrice(entry) : entry.price)?.output) }}</dd>
            </div>
            <span class="text-xs text-af-ink-4">{{ showUserPrice ? t('userUi.models.yourPrice') : t('userUi.models.perMillionShort') }}</span>
          </dl>
          <div v-if="entry.aliases.length || entry.timePricing" class="mt-3 flex flex-wrap gap-1.5">
            <span v-for="alias in entry.aliases" :key="alias" class="badge badge-gray font-mono">{{ alias }}</span>
            <span v-if="entry.timePricing" class="badge badge-gray" :title="timePricingText(entry)">{{ t('userUi.models.timePricing') }}</span>
          </div>
        </li>
      </ul>

      <p class="max-w-3xl text-xs leading-5 text-af-ink-3">
        {{ t('userUi.models.priceNote') }}
        <template v-if="isAuthenticated"> {{ t('userUi.models.multiplierNote', { multiplier: userMultiplier }) }}</template>
      </p>
    </template>
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
  type CatalogModel
} from './catalog'

type PlazaView = 'table' | 'grid'

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

// 厂商与视图记在 URL（?vendor= / ?view=），刷新与分享都保留
function readVendor(): string {
  const value = route.query.vendor
  return typeof value === 'string' && value ? value : 'all'
}
function readView(): PlazaView {
  return route.query.view === 'grid' ? 'grid' : 'table'
}
const selectedVendor = ref(readVendor())
const view = ref<PlazaView>(readView())
watch(() => route.query, () => {
  selectedVendor.value = readVendor()
  view.value = readView()
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
function setView(next: PlazaView) {
  view.value = next
  replaceQuery({ view: next === 'table' ? undefined : next })
}

const viewOptions = computed<Array<{ key: PlazaView; label: string; icon: 'menu' | 'grid' }>>(() => [
  { key: 'table', label: t('userUi.models.view.table'), icon: 'menu' },
  { key: 'grid', label: t('userUi.models.view.grid'), icon: 'grid' }
])

const descriptionHtml = computed(() => {
  const md = props.response?.description?.trim()
  if (!md) return ''
  return DOMPurify.sanitize(marked.parse(md) as string)
})

const catalog = computed(() => buildCatalog(props.response?.models ?? []))
const vendors = computed(() => catalogVendors(catalog.value))
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
function userPrice(entry: CatalogModel) {
  return applyMultiplier(entry.price, userMultiplier.value)
}
function timePricingText(entry: CatalogModel): string {
  return entry.timePricing ? formatTimePricing(entry.timePricing, t('userUi.models.weekdaysOnly')) : ''
}

function tabClass(active: boolean): string {
  return [
    'relative -mb-px inline-flex h-10 shrink-0 items-center gap-1.5 whitespace-nowrap px-3 text-sm font-medium transition-colors',
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
