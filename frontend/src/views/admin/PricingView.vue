<template>
  <!--
    价格页（供给 › 价格，muqian 2026-09-30 D1）：官方价与上游价都在这里改，给模型加一个渠道就是承接。
    默认按模型分组，可切到按渠道；每一块单独保存。草稿由本页保管，筛选、翻页不丢；切视图、离开页面前先问。
  -->
  <AppLayout>
    <template #header-actions>
      <SegmentedControl
        :model-value="view"
        :options="viewOptions"
        :label="t('admin.pricing.views.label')"
        test-id-prefix="pricing-view"
        @update:model-value="requestView"
      />
    </template>

    <div class="space-y-4">
      <ListToolbar>
        <SearchInput
          v-model="search"
          compact
          class="w-full sm:w-64"
          :placeholder="view === 'model' ? t('admin.pricing.searchModels') : t('admin.pricing.searchChannels')"
        />
        <FilterChip v-if="view === 'model'" v-model="vendorFilter" :label="t('admin.pricing.filters.vendor')" :options="vendorOptions" test-id="pricing-filter-vendor" />
        <FilterChip v-if="view === 'model'" v-model="statusFilter" :label="t('admin.pricing.filters.status')" :options="statusOptions" test-id="pricing-filter-status" />
        <FilterChip v-model="focusFilter" :label="t('admin.pricing.filters.focus')" :options="focusOptions" test-id="pricing-filter-focus" />
        <template #end>
          <span v-if="dirtyCount > 0" class="mr-2 text-xs text-af-warning" data-testid="pricing-dirty-count">
            {{ t('admin.pricing.unsavedBlocks', { count: dirtyCount }) }}
          </span>
          <button
            type="button"
            class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
            :disabled="loading"
            :aria-label="t('common.refresh')"
            @click="load(false)"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </template>
      </ListToolbar>

      <p v-if="overview" class="text-xs text-af-ink-3">{{ basisText }}</p>

      <StatusState v-if="loading && !overview" kind="loading" :title="t('common.loading')" />
      <StatusState v-else-if="loadError && !overview" kind="error" :title="loadError" :action-label="t('admin.pricing.reload')" @action="load(true)" />
      <template v-else-if="overview">
        <FormError v-if="loadError" :message="loadError" />
        <StatusState v-if="visibleCount === 0" kind="empty" :title="t('admin.pricing.empty')" />
        <div v-else class="space-y-8">
          <template v-if="view === 'model'">
            <PricingModelBlock
              v-for="entry in pagedEntries"
              :key="entry.id"
              :entry="entry"
              :state="modelStates.get(entry.id)!"
              :accounts="accountsById"
              :account-order="accountOrder"
              :default-sale-ratio="overview.default_sale_ratio"
              :min-margin="overview.min_margin"
              @saved="load(false)"
            />
          </template>
          <template v-else>
            <PricingChannelBlock
              v-for="account in pagedAccounts"
              :key="account.id"
              :account="account"
              :state="channelStates.get(account.id)!"
              :accounts="overview.accounts"
              :entries="overview.entries"
              :default-sale-ratio="overview.default_sale_ratio"
              :min-margin="overview.min_margin"
              @saved="load(false)"
            />
          </template>
        </div>
        <Pagination
          v-if="visibleCount > PAGE_SIZE"
          :page="page"
          :total="visibleCount"
          :page-size="PAGE_SIZE"
          :show-page-size-selector="false"
          @update:page="page = $event"
        />
      </template>
    </div>

    <ConfirmDialog
      :show="leaveDialog.show"
      :title="t('admin.pricing.discardTitle')"
      :message="t('admin.pricing.discardMessage', { count: dirtyCount })"
      :confirm-text="t('admin.pricing.discard')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="resolveLeave(true)"
      @cancel="resolveLeave(false)"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave, useRoute } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { PricingAccount, PricingEntry, PricingOverview } from '@/api/admin/pricing'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import FormError from '@/components/common/FormError.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import { FilterChip, ListToolbar, type FilterOption } from '@/components/admin/list'
import PricingModelBlock from '@/components/admin/pricing/PricingModelBlock.vue'
import PricingChannelBlock from '@/components/admin/pricing/PricingChannelBlock.vue'
import {
  belowMinMargin,
  channelDraftChanges,
  channelDraftFrom,
  cloneChannelDraft,
  cloneModelDraft,
  entryHasProblem,
  marginOf,
  modelDraftChanges,
  modelDraftFrom,
  type BlockState,
  type ChannelDraft,
  type ModelDraft
} from '@/components/admin/pricing/pricingDraft'
import { extractApiErrorMessage } from '@/utils/apiError'

const PAGE_SIZE = 20

type View = 'model' | 'channel'

const { t } = useI18n()
const route = useRoute()

const overview = ref<PricingOverview | null>(null)
const loading = ref(false)
const loadError = ref('')

// ---- 每块的草稿：键为模型条目 ID / 渠道 ID
const modelStates = reactive(new Map<number, BlockState<ModelDraft>>())
const channelStates = reactive(new Map<number, BlockState<ChannelDraft>>())

const accountsById = computed(() => new Map((overview.value?.accounts ?? []).map((account) => [account.id, account])))

/** 渠道排序：渠道自己的优先级（越小越先用），再按 ID */
function accountOrder(accountId: number): number {
  const priority = accountsById.value.get(accountId)?.priority ?? 1e6
  return priority * 1e9 + accountId
}

/** 服务端数据到了：没改过的块按新数据重建，改到一半的块保留草稿 */
function syncStates(data: PricingOverview) {
  const entryIds = new Set(data.entries.map((entry) => entry.id))
  for (const id of [...modelStates.keys()]) if (!entryIds.has(id)) modelStates.delete(id)
  for (const entry of data.entries) {
    const current = modelStates.get(entry.id)
    if (current && modelDraftChanges(current) > 0) continue
    const initial = modelDraftFrom(entry, accountOrder)
    modelStates.set(entry.id, { initial, draft: cloneModelDraft(initial) })
  }
  const accountIds = new Set(data.accounts.map((account) => account.id))
  for (const id of [...channelStates.keys()]) if (!accountIds.has(id)) channelStates.delete(id)
  for (const account of data.accounts) {
    const current = channelStates.get(account.id)
    if (current && channelDraftChanges(current) > 0) continue
    const initial = channelDraftFrom(account.id, data.entries)
    channelStates.set(account.id, { initial, draft: cloneChannelDraft(initial) })
  }
}

function discardAll() {
  modelStates.clear()
  channelStates.clear()
  if (overview.value) syncStates(overview.value)
}

async function load(showSpinner: boolean) {
  loading.value = true
  if (showSpinner) loadError.value = ''
  try {
    const data = await adminAPI.pricing.overview()
    overview.value = data
    syncStates(data)
    loadError.value = ''
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('admin.pricing.loadFailed'))
  } finally {
    loading.value = false
  }
}

const dirtyModelCount = computed(() => [...modelStates.values()].filter((state) => modelDraftChanges(state) > 0).length)
const dirtyChannelCount = computed(() => [...channelStates.values()].filter((state) => channelDraftChanges(state) > 0).length)
const dirtyCount = computed(() => dirtyModelCount.value + dirtyChannelCount.value)

// ---- 视图与筛选（?view=channel、?model=<模型 ID>、?channel=<渠道 ID> 可从别的页面直接跳过来）
const view = ref<View>(route.query.view === 'channel' || route.query.channel ? 'channel' : 'model')
const viewOptions = computed(() => [
  { key: 'model' as View, label: t('admin.pricing.views.model') },
  { key: 'channel' as View, label: t('admin.pricing.views.channel') }
])
const search = ref(typeof route.query.model === 'string' ? route.query.model : '')
const channelFilter = ref<number | null>(typeof route.query.channel === 'string' ? Number(route.query.channel) || null : null)
const vendorFilter = ref('')
const statusFilter = ref(search.value ? '' : 'listed')
const focusFilter = ref('')
const page = ref(1)

watch([view, search, vendorFilter, statusFilter, focusFilter], () => {
  page.value = 1
})
watch(search, () => {
  channelFilter.value = null
})

const vendorOptions = computed<FilterOption[]>(() => {
  const vendors = new Set((overview.value?.entries ?? []).map((entry) => entry.vendor).filter(Boolean))
  return [...vendors].sort().map((vendor) => ({ value: vendor, label: vendor }))
})
const statusOptions = computed<FilterOption[]>(() => [
  { value: 'listed', label: t('admin.pricing.status.listed') },
  { value: 'unlisted', label: t('admin.pricing.status.unlisted') }
])
const focusOptions = computed<FilterOption[]>(() => [
  { value: 'problems', label: t('admin.pricing.filters.problems') },
  { value: 'unsaved', label: t('admin.pricing.filters.unsaved') }
])

const basisText = computed(() => {
  if (!overview.value) return ''
  const rate = overview.value.default_sale_ratio
  const inverse = 1 / rate
  const rateText = Math.abs(inverse - Math.round(inverse)) < 1e-6 ? `1/${Math.round(inverse)}` : String(rate)
  const minMargin = overview.value.min_margin
  return minMargin > 0
    ? t('admin.pricing.basis', { rate: rateText, margin: `${Math.round(minMargin * 1000) / 10}%` })
    : t('admin.pricing.basisGateOff', { rate: rateText })
})

function includesQuery(text: string, query: string): boolean {
  return text.toLowerCase().includes(query)
}

const filteredEntries = computed<PricingEntry[]>(() => {
  const data = overview.value
  if (!data) return []
  const query = search.value.trim().toLowerCase()
  return data.entries
    .filter((entry) => {
      if (vendorFilter.value && entry.vendor !== vendorFilter.value) return false
      if (statusFilter.value && entry.status !== statusFilter.value) return false
      if (focusFilter.value === 'problems' && !entryHasProblem(entry, data.default_sale_ratio, data.min_margin)) return false
      if (focusFilter.value === 'unsaved') {
        const state = modelStates.get(entry.id)
        if (!state || modelDraftChanges(state) === 0) return false
      }
      if (!query) return true
      return (
        includesQuery(entry.model_id, query) ||
        includesQuery(entry.display_name, query) ||
        entry.bindings.some((binding) => includesQuery(accountsById.value.get(binding.account_id)?.name ?? '', query))
      )
    })
    .sort((a, b) => a.model_id.localeCompare(b.model_id))
})

function channelHasProblem(account: PricingAccount, data: PricingOverview): boolean {
  return data.entries.some((entry) =>
    entry.bindings.some(
      (binding) => binding.account_id === account.id && belowMinMargin(marginOf(binding.cost_ratio, data.default_sale_ratio), data.min_margin)
    )
  )
}

const filteredAccounts = computed<PricingAccount[]>(() => {
  const data = overview.value
  if (!data) return []
  const query = search.value.trim().toLowerCase()
  return data.accounts
    .filter((account) => {
      if (channelFilter.value != null) return account.id === channelFilter.value
      if (focusFilter.value === 'problems' && !channelHasProblem(account, data)) return false
      if (focusFilter.value === 'unsaved') {
        const state = channelStates.get(account.id)
        if (!state || channelDraftChanges(state) === 0) return false
      }
      if (!query) return true
      return (
        includesQuery(account.name, query) ||
        data.entries.some((entry) => entry.bindings.some((binding) => binding.account_id === account.id) && includesQuery(entry.model_id, query))
      )
    })
    .sort((a, b) => accountOrder(a.id) - accountOrder(b.id))
})

const visibleCount = computed(() => (view.value === 'model' ? filteredEntries.value.length : filteredAccounts.value.length))
const pagedEntries = computed(() => filteredEntries.value.slice((page.value - 1) * PAGE_SIZE, page.value * PAGE_SIZE))
const pagedAccounts = computed(() => filteredAccounts.value.slice((page.value - 1) * PAGE_SIZE, page.value * PAGE_SIZE))

// ---- 有没保存的块时：切视图、离开页面先问；放弃就恢复成已保存的值
const leaveDialog = reactive<{ show: boolean; resolve: ((leave: boolean) => void) | null }>({ show: false, resolve: null })

function resolveLeave(leave: boolean) {
  leaveDialog.show = false
  leaveDialog.resolve?.(leave)
  leaveDialog.resolve = null
}

async function confirmDiscard(): Promise<boolean> {
  if (dirtyCount.value === 0) return true
  const leave = await new Promise<boolean>((resolve) => {
    leaveDialog.resolve = resolve
    leaveDialog.show = true
  })
  if (leave) discardAll()
  return leave
}

// 两个视图各自整块覆盖同一批承接关系：带着一边的草稿去另一边保存会互相盖掉，所以切视图前先放弃草稿
async function requestView(next: View) {
  if (next === view.value) return
  if (await confirmDiscard()) {
    view.value = next
    channelFilter.value = null
  }
}

onBeforeRouteLeave(confirmDiscard)

function onBeforeUnload(event: BeforeUnloadEvent) {
  if (dirtyCount.value > 0) event.preventDefault()
}

onMounted(() => {
  window.addEventListener('beforeunload', onBeforeUnload)
  void load(true)
})
onBeforeUnmount(() => window.removeEventListener('beforeunload', onBeforeUnload))
</script>
