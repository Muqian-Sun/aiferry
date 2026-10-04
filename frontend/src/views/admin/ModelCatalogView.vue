<template>
  <!--
    模型目录（A4 列表模板）：上架 = 用户能看到并调用；上架必有价、必有渠道承接。
    标题右侧「⋯」（从价格文件导入）+「新建模型」；数字摘要（模型 / 已上架 / 上架但无渠道，可一键筛出）；
    工具行 = 搜索 + 状态（默认只看已上架：价格文件带进来的几百个模型大多没上架）/ 厂商 / 计费 / 渠道筛选标签 + 刷新；
    列 = 模型、厂商、标价（输入 / 输出，每百万 Token；整列同一个小数位数）、承接渠道数、状态；行尾「编辑」图标 +「⋯」（删除）；选中行时批量上下架。
    点行打开详情抽屉（A5）：概况（全部标价、别名…）/ 渠道（承接的渠道此刻能否调度 + 诊断）；点承接渠道数直接打开渠道页签。
    新建 / 编辑是弹窗（2026-10-03）：新建两步（模型 → 定价与渠道），编辑一步；改价去价格页。
  -->
  <AppLayout>
    <template #header-actions>
      <PopoverMenu width-class="w-52">
        <template #trigger="{ open }">
          <button
            type="button"
            class="btn btn-ghost btn-md px-2.5"
            :class="open ? 'bg-af-sunken text-af-ink' : ''"
            :title="t('common.more')"
            :aria-label="t('common.more')"
            data-testid="model-catalog-tools"
          >
            <Icon name="more" size="md" />
          </button>
        </template>
        <MenuItem icon="download" :disabled="seeding" data-testid="model-catalog-seed" @click="runSeed">
          {{ seeding ? t('admin.modelCatalog.seeding') : t('admin.modelCatalog.seed') }}
        </MenuItem>
      </PopoverMenu>
      <button type="button" class="btn btn-primary btn-md" data-testid="model-catalog-create" @click="openCreate">
        <Icon name="plus" size="md" />
        {{ t('admin.modelCatalog.create') }}
      </button>
    </template>

    <TablePageLayout>
      <template v-if="summaryItems" #summary>
        <StatRow :items="summaryItems" data-testid="model-catalog-summary" />
      </template>

      <template #filters>
        <ListToolbar>
          <SearchInput
            v-model="searchQuery"
            compact
            class="w-full sm:w-64"
            :placeholder="t('admin.modelCatalog.search')"
            data-testid="model-catalog-search"
          />
          <FilterChip
            v-model="statusFilter"
            :label="t('admin.modelCatalog.columns.status')"
            :options="statusOptions"
            test-id="model-catalog-filter-status"
          />
          <FilterChip
            v-model="vendorFilter"
            :label="t('admin.modelCatalog.fields.vendor')"
            :options="vendorOptions"
            test-id="model-catalog-filter-vendor"
          />
          <FilterChip
            v-model="billingFilter"
            :label="t('admin.modelCatalog.fields.billingMode')"
            :options="billingOptions"
            test-id="model-catalog-filter-billing"
          />
          <FilterChip
            v-model="resourceFilter"
            :label="t('admin.modelCatalog.fields.resources')"
            :options="resourceOptions"
            test-id="model-catalog-filter-resources"
          />
          <span v-if="isFiltered" class="px-1 text-13 tabular-nums text-af-ink-3" data-testid="model-catalog-filtered">
            {{ t('admin.modelCatalog.filtered', { count: filteredEntries.length }) }}
          </span>

          <template #end>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadEntries"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </template>
        </ListToolbar>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="pagedEntries"
          :loading="loading"
          selectable
          row-key="id"
          :selected-keys="selectedIds"
          :selection-label="(entry: ModelCatalogEntry) => t('admin.modelCatalog.bulk.selectEntry', { model: entry.model_id })"
          clickable-rows
          @update:selected-keys="handleSelectionChange"
          @row-click="openDrawer($event)"
        >
          <template #cell-model_id="{ row }">
            <div class="min-w-0">
              <div class="truncate font-mono font-medium text-af-ink">{{ row.model_id }}</div>
              <div v-if="row.display_name || row.aliases?.length" class="mt-0.5 truncate text-xs text-af-ink-3">
                <span v-if="row.display_name">{{ row.display_name }}</span>
                <span v-if="row.display_name && row.aliases?.length" class="text-af-ink-4"> · </span>
                <span v-if="row.aliases?.length" :title="row.aliases.map((a: ModelCatalogAlias) => a.alias).join(', ')">
                  {{ t('admin.modelCatalog.aliasCount', { count: row.aliases.length }) }}
                </span>
              </div>
            </div>
          </template>
          <template #cell-vendor="{ row }">
            <span v-if="catalogVendorLabel(row)" class="text-af-ink-2">{{ catalogVendorLabel(row) }}</span>
            <span v-else class="text-af-ink-4">—</span>
          </template>
          <template #cell-price="{ row }">
            <PriceCell :entry="row" :decimals="priceDecimals" />
          </template>
          <!-- 上架却没有能调度的渠道是真异常（没有承接、渠道都停了、或都被利润门跳过，D6）：橙点橙字，悬停看原因；
               点数字 / 这行字都打开详情抽屉的渠道页签 -->
          <template #cell-resources="{ row }">
            <button
              v-if="row.status === 'listed' && hasNoSchedulableChannel(row)"
              type="button"
              class="inline-flex items-center gap-1.5 text-af-warning hover:underline"
              :title="unschedulableReasonText(row, t)"
              data-testid="model-catalog-no-resources"
              @click.stop="openDrawer(row, 'channels')"
            >
              <span class="inline-block h-2 w-2 rounded-full bg-af-warning"></span>
              {{ t('admin.modelCatalog.unschedulable.label') }}
            </button>
            <button
              v-else
              type="button"
              class="tabular-nums text-af-ink-2 underline decoration-af-ink-4 decoration-dotted underline-offset-2 hover:text-af-ink"
              data-testid="model-catalog-resource-count"
              @click.stop="openDrawer(row, 'channels')"
            >
              {{ bindingCount(row) }}
            </button>
          </template>
          <!-- 已上架 / 未上架都是常态：灰点纯文字，已上架的点与字深一档 -->
          <template #cell-status="{ value }">
            <div class="flex items-center gap-1.5">
              <span :class="['inline-block h-2 w-2 rounded-full', value === 'listed' ? 'bg-af-ink-3' : 'bg-af-hairline-strong']"></span>
              <span :class="value === 'listed' ? 'text-af-ink' : 'text-af-ink-3'">{{ t(`admin.modelCatalog.status.${value}`) }}</span>
            </div>
          </template>
          <template #cell-actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>

          <template #empty>
            <!-- 默认只看已上架：一个都没上架时别只说「没有匹配」，给一键看全部 -->
            <EmptyState
              v-if="entries.length && onlyDefaultFilter && listedCount === 0"
              :title="t('admin.modelCatalog.noneListed')"
              :action-text="t('admin.modelCatalog.showAll')"
              @action="statusFilter = ''"
            />
            <EmptyState
              v-else
              :title="entries.length ? t('admin.modelCatalog.noMatch') : t('admin.modelCatalog.empty')"
              :action-text="entries.length ? undefined : t('admin.modelCatalog.create')"
              @action="openCreate"
            />
          </template>
        </DataTable>
      </template>

      <template #bulk>
        <BulkBar :count="selectedIds.length" @clear="selectedIds = []">
          <button type="button" class="bulk-btn" :disabled="bulkRunning" data-testid="model-catalog-bulk-list" @click="bulkSetStatus('listed')">
            {{ t('admin.modelCatalog.bulk.list') }}
          </button>
          <button type="button" class="bulk-btn" :disabled="bulkRunning" data-testid="model-catalog-bulk-unlist" @click="bulkSetStatus('unlisted')">
            {{ t('admin.modelCatalog.bulk.unlist') }}
          </button>
        </BulkBar>
        <!-- 批量上下架部分失败：逐条列出模型与后端原因（上架必有价等校验在后端），失败的仍留在选中集里 -->
        <div v-if="bulkFailures.length" role="alert" class="mt-2 space-y-1 text-sm text-af-danger" data-testid="model-catalog-bulk-failures">
          <p>{{ t('admin.modelCatalog.bulk.partial', { done: bulkDone, failed: bulkFailures.length }) }}</p>
          <ul class="max-h-40 space-y-0.5 overflow-y-auto">
            <li v-for="failure in bulkFailures" :key="failure.id" class="break-words">{{ failure.model }}: {{ failure.message }}</li>
          </ul>
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="filteredEntries.length > 0"
          :page="page"
          :total="filteredEntries.length"
          :page-size="pageSize"
          @update:page="page = $event"
          @update:page-size="onPageSizeChange"
        />
      </template>
    </TablePageLayout>

    <CatalogEntryDrawer
      :show="drawerOpen"
      :entry="drawerEntry"
      v-model:tab="drawerTab"
      @close="drawerOpen = false"
      @edit="drawerEntry && openEdit(drawerEntry)"
      @diagnose="drawerEntry && openDiagnosis(drawerEntry)"
      @set-status="drawerEntry && setEntryStatus(drawerEntry, $event)"
      @delete="drawerEntry && askDelete(drawerEntry)"
    />

    <ModelCreateDialog
      :show="showCreate"
      :vendor-options="catalogVendors"
      :existing-model-ids="catalogModelIds"
      @close="showCreate = false"
      @saved="loadEntries"
    />
    <ModelEditDialog
      :show="editingEntry !== null"
      :entry="editingEntry"
      :vendor-options="catalogVendors"
      @close="editingEntry = null"
      @saved="loadEntries"
      @aliases-changed="loadEntries"
    />

    <CatalogEntryDiagnosisModal
      :show="diagnosisEntry !== null"
      :entry-id="diagnosisEntry?.id ?? null"
      :model-id="diagnosisEntry?.model_id ?? ''"
      @close="diagnosisEntry = null"
    />

    <ConfirmDialog
      :show="pendingListing !== null"
      :title="t('admin.modelCatalog.unschedulable.confirmTitle')"
      :message="t('admin.modelCatalog.unschedulable.confirmMessage', { models: unschedulableListText(pendingListing?.entries ?? [], t) })"
      :confirm-text="t('admin.modelCatalog.unschedulable.confirm')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmPendingListing"
      @cancel="pendingListing = null"
    />

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.modelCatalog.deleteTitle')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ModelCatalogAlias, ModelCatalogEntry } from '@/api/admin/modelCatalog'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import { BulkBar, FilterChip, ListToolbar, MenuItem, PopoverMenu, RowActions } from '@/components/admin/list'
import type { FilterOption, RowAction } from '@/components/admin/list'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { hasNoSchedulableChannel, unschedulableListText, unschedulableReasonText } from '@/components/admin/catalog/schedulable'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import CatalogEntryDiagnosisModal from '@/components/admin/catalog/CatalogEntryDiagnosisModal.vue'
import CatalogEntryDrawer from '@/components/admin/catalog/CatalogEntryDrawer.vue'
import ModelCreateDialog from '@/components/admin/catalog/ModelCreateDialog.vue'
import ModelEditDialog from '@/components/admin/catalog/ModelEditDialog.vue'
import PriceCell from '@/components/admin/catalog/CatalogPriceCell.vue'
import { entryToRequest } from '@/components/admin/catalog/entryRequest'
import { LIST_PRICE_MAX_DECIMALS, listPriceValues, sharedPriceDecimals } from '@/components/admin/catalog/priceFormat'
import { catalogVendorChoices, catalogVendorLabel } from '@/components/admin/catalog/vendorLabel'
import { getPersistedPageSize, setPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()

const loading = ref(false)
const seeding = ref(false)
const bulkRunning = ref(false)
const bulkDone = ref(0)
const bulkFailures = ref<Array<{ id: number; model: string; message: string }>>([])
const entries = ref<ModelCatalogEntry[]>([])
const selectedIds = ref<number[]>([])

// 筛选（空串 = 全部；厂商筛选里「无厂商」用 NO_VENDOR 占位，空串已表示不筛）
// 状态默认只看已上架：价格文件带进来的几百个模型大多没上架，默认全列出来会把真在卖的几个淹掉
const NO_VENDOR = '__none__'
const DEFAULT_STATUS_FILTER = 'listed'
const searchQuery = ref('')
const statusFilter = ref(DEFAULT_STATUS_FILTER)
const vendorFilter = ref('')
const billingFilter = ref('')
const resourceFilter = ref('')

// 弹窗
const diagnosisEntry = ref<ModelCatalogEntry | null>(null)
const showDeleteDialog = ref(false)
const pendingDelete = ref<ModelCatalogEntry | null>(null)

// 详情抽屉（A5）：按 ID 记住打开的条目，列表重载后自动换成新数据；条目被删就跟着关
const drawerOpen = ref(false)
const drawerTab = ref('overview')
const drawerEntryId = ref<number | null>(null)
const drawerEntry = computed(() => entries.value.find((entry) => entry.id === drawerEntryId.value) ?? null)

// 分页（客户端：列表接口一次返回全部）
const page = ref(1)
const pageSize = ref(getPersistedPageSize())

function logApiError(error: unknown) {
  console.error(extractApiErrorMessage(error, t('common.unknownError')), error)
}

function bindingCount(entry: ModelCatalogEntry): number {
  return entry.bindings?.length ?? 0
}

const columns = computed<Column[]>(() => [
  { key: 'model_id', label: t('admin.modelCatalog.columns.model') },
  { key: 'vendor', label: t('admin.modelCatalog.fields.vendor') },
  { key: 'price', label: t('admin.modelCatalog.columns.price'), class: 'text-right' },
  { key: 'resources', label: t('admin.modelCatalog.columns.channels') },
  { key: 'status', label: t('admin.modelCatalog.columns.status') },
  { key: 'actions', label: t('common.actions'), class: 'text-right' }
])

// 厂商筛选按展示名分组：同一厂商族的几个原始标识（如 gemini / vertex_ai-*）是一个选项
const vendorValues = computed(() => [...new Set(entries.value.map(catalogVendorLabel).filter(Boolean))].sort((a, b) => a.localeCompare(b)))
const statusOptions = computed<FilterOption[]>(() => [
  { value: '', label: t('common.all') },
  { value: 'listed', label: t('admin.modelCatalog.status.listed') },
  { value: 'unlisted', label: t('admin.modelCatalog.status.unlisted') }
])
const vendorOptions = computed<FilterOption[]>(() => [
  ...vendorValues.value.map((vendor) => ({ value: vendor, label: vendor })),
  ...(entries.value.some((entry) => !catalogVendorLabel(entry)) ? [{ value: NO_VENDOR, label: t('admin.modelCatalog.filters.noVendor') }] : [])
])
const billingOptions = computed<FilterOption[]>(() =>
  ['token', 'per_request', 'image', 'video'].map((mode) => ({ value: mode, label: t(`admin.modelCatalog.billingModes.${mode}`) }))
)
const resourceOptions = computed<FilterOption[]>(() => [
  { value: 'bound', label: t('admin.modelCatalog.filters.withResources') },
  { value: 'unbound', label: t('admin.modelCatalog.filters.withoutResources') }
])

const listedCount = computed(() => entries.value.filter((entry) => entry.status === 'listed').length)
const listedWithoutResources = computed(() => entries.value.filter((entry) => entry.status === 'listed' && hasNoSchedulableChannel(entry)).length)

/** 摘要「上架但无渠道」旁的「筛选」：一键筛出这些条目（其余筛选清掉，免得叠加后看不全） */
function showListedWithoutResources() {
  searchQuery.value = ''
  vendorFilter.value = ''
  billingFilter.value = ''
  statusFilter.value = 'listed'
  resourceFilter.value = 'unbound'
}

// 数字摘要：直接用已加载的全量条目；目录为空（或加载失败）就不显示，不摆一排 0
const summaryItems = computed<StatItem[] | null>(() => {
  if (entries.value.length === 0) return null
  const fmt = (n: number) => n.toLocaleString()
  const unbound = listedWithoutResources.value
  return [
    { key: 'total', label: t('admin.modelCatalog.summaryStats.total'), value: fmt(entries.value.length) },
    { key: 'listed', label: t('admin.modelCatalog.summaryStats.listed'), value: fmt(listedCount.value) },
    {
      key: 'unbound',
      label: t('admin.modelCatalog.summaryStats.listedWithoutResources'),
      value: fmt(unbound),
      action: unbound > 0 ? { label: t('admin.modelCatalog.summaryStats.showThem'), onClick: showListedWithoutResources } : undefined
    }
  ]
})

const filteredEntries = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return entries.value.filter((entry) => {
    const vendor = catalogVendorLabel(entry)
    if (statusFilter.value && entry.status !== statusFilter.value) return false
    if (vendorFilter.value && (vendor || NO_VENDOR) !== vendorFilter.value) return false
    if (billingFilter.value && (entry.billing_mode || 'token') !== billingFilter.value) return false
    if (resourceFilter.value === 'bound' && hasNoSchedulableChannel(entry)) return false
    if (resourceFilter.value === 'unbound' && !hasNoSchedulableChannel(entry)) return false
    if (!q) return true
    return [entry.model_id, entry.display_name, entry.vendor, vendor, ...(entry.aliases ?? []).map((alias) => alias.alias)].some((value) =>
      (value || '').toLowerCase().includes(q)
    )
  })
})

const isFiltered = computed(() => filteredEntries.value.length !== entries.value.length)

/** 只有默认的「已上架」筛选在生效（空态据此给「看全部」） */
const onlyDefaultFilter = computed(
  () =>
    statusFilter.value === DEFAULT_STATUS_FILTER &&
    !searchQuery.value.trim() &&
    !vendorFilter.value &&
    !billingFilter.value &&
    !resourceFilter.value
)

/** 标价列整列共用的小数位数：按当前筛选结果（不止本页）算，翻页不跳位数 */
const priceDecimals = computed(() =>
  sharedPriceDecimals(filteredEntries.value.flatMap(listPriceValues), LIST_PRICE_MAX_DECIMALS)
)

const pagedEntries = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return filteredEntries.value.slice(start, start + pageSize.value)
})

// 筛选变了回第一页；条目数变了把越界的页码拉回来
watch([searchQuery, statusFilter, vendorFilter, billingFilter, resourceFilter], () => {
  page.value = 1
})
watch(filteredEntries, (list) => {
  const last = Math.max(1, Math.ceil(list.length / pageSize.value))
  if (page.value > last) page.value = last
})
// 选中集清空（手动取消或全部处理完）时，上一次批量的失败清单一起收起
watch(() => selectedIds.value.length, (count) => {
  if (count === 0) bulkFailures.value = []
})

function onPageSizeChange(size: number) {
  pageSize.value = size
  setPersistedPageSize(size)
  page.value = 1
}

function handleSelectionChange(ids: Array<string | number>) {
  const visible = new Set(entries.value.map((entry) => entry.id))
  selectedIds.value = [...new Set(ids.map(Number))].filter((id) => visible.has(id))
}

async function loadEntries() {
  loading.value = true
  try {
    entries.value = await adminAPI.modelCatalog.listEntries()
    const known = new Set(entries.value.map((entry) => entry.id))
    selectedIds.value = selectedIds.value.filter((id) => known.has(id))
  } catch (error) {
    logApiError(error)
  } finally {
    loading.value = false
  }
}

const showCreate = ref(false)
const editingEntry = ref<ModelCatalogEntry | null>(null)
/** 弹窗厂商下拉的选项：目录里已有的厂商 */
const catalogVendors = computed(() => catalogVendorChoices(entries.value))
const catalogModelIds = computed(() => entries.value.map((entry) => entry.model_id))

function openCreate() {
  showCreate.value = true
}

function openEdit(entry: ModelCatalogEntry) {
  editingEntry.value = entry
}

function openDiagnosis(entry: ModelCatalogEntry) {
  diagnosisEntry.value = entry
}

function openDrawer(entry: ModelCatalogEntry, tab = 'overview') {
  drawerEntryId.value = entry.id
  drawerTab.value = tab
  drawerOpen.value = true
}

// 上架没有能调度的渠道的模型：不拦，先确认（D6）。确认后照常走原来的上架
const pendingListing = ref<{ entries: ModelCatalogEntry[]; run: () => void } | null>(null)
function confirmPendingListing() {
  const pending = pendingListing.value
  pendingListing.value = null
  pending?.run()
}

/** 抽屉「⋯」里的上架 / 下架：与批量上下架同一条路（整条覆盖 PUT，价格 / 绑定校验在后端） */
async function setEntryStatus(entry: ModelCatalogEntry, status: 'listed' | 'unlisted', confirmed = false) {
  if (entry.status === status) return
  if (status === 'listed' && !confirmed && hasNoSchedulableChannel(entry)) {
    pendingListing.value = { entries: [entry], run: () => void setEntryStatus(entry, status, true) }
    return
  }
  try {
    await adminAPI.modelCatalog.updateEntry(entry.id, { ...entryToRequest(entry), status })
    await loadEntries()
  } catch (error) {
    logApiError(error)
  }
}

// 行操作（A4）：编辑是图标；删除进「⋯」，红字且仍走确认框。
// 诊断只留一个入口：详情抽屉「渠道」页签（点承接渠道数直达）里的「诊断」按钮
function rowActions(entry: ModelCatalogEntry): RowAction[] {
  return [
    { key: 'edit', label: t('common.edit'), icon: 'edit', primary: true, onSelect: () => openEdit(entry) },
    { key: 'delete', label: t('common.delete'), icon: 'trash', danger: true, onSelect: () => askDelete(entry) }
  ]
}

// 删除确认写上模型标识；按 Token 计费的价格分「段」，按次 / 图片 / 视频分「档」（与条目详情同一叫法）
const deleteConfirmMessage = computed(() => {
  const entry = pendingDelete.value
  if (!entry) return ''
  const isToken = !entry.billing_mode || entry.billing_mode === 'token'
  return t('admin.modelCatalog.deleteConfirm', {
    model: entry.model_id,
    intervals: t(isToken ? 'admin.modelCatalog.deleteIntervals.segments' : 'admin.modelCatalog.deleteIntervals.tiers')
  })
})

function askDelete(entry: ModelCatalogEntry) {
  pendingDelete.value = entry
  showDeleteDialog.value = true
}

async function confirmDelete() {
  const entry = pendingDelete.value
  showDeleteDialog.value = false
  if (!entry) return
  try {
    await adminAPI.modelCatalog.deleteEntry(entry.id)
    if (drawerEntryId.value === entry.id) drawerOpen.value = false
    await loadEntries()
  } catch (error) {
    logApiError(error)
  }
}

/**
 * 批量上下架：没有批量接口，逐条整条覆盖 PUT（上架必有价 / 绑定校验都在后端，被拒的逐条列出来）。
 * 已是目标状态的跳过；全部成功提示条数，部分失败把失败的模型与原因一起摆出来。
 */
async function bulkSetStatus(status: 'listed' | 'unlisted', confirmed = false) {
  const targets = entries.value.filter((entry) => selectedIds.value.includes(entry.id) && entry.status !== status)
  if (targets.length === 0) {
    return
  }
  const unschedulable = targets.filter(hasNoSchedulableChannel)
  if (status === 'listed' && !confirmed && unschedulable.length > 0) {
    pendingListing.value = { entries: unschedulable, run: () => void bulkSetStatus(status, true) }
    return
  }
  bulkRunning.value = true
  bulkFailures.value = []
  const failures: Array<{ id: number; model: string; message: string }> = []
  try {
    for (const entry of targets) {
      try {
        await adminAPI.modelCatalog.updateEntry(entry.id, { ...entryToRequest(entry), status })
      } catch (error) {
        failures.push({ id: entry.id, model: entry.model_id, message: extractApiErrorMessage(error, t('common.unknownError')) })
      }
    }
    if (failures.length === 0) {
      selectedIds.value = []
    } else {
      bulkDone.value = targets.length - failures.length
      bulkFailures.value = failures
      // 失败的留在选中集里，方便修完价格 / 绑定再试
      const failedIds = new Set(failures.map((failure) => failure.id))
      selectedIds.value = selectedIds.value.filter((id) => failedIds.has(id))
    }
    await loadEntries()
  } finally {
    bulkRunning.value = false
  }
}

async function runSeed() {
  seeding.value = true
  try {
    const result = await adminAPI.modelCatalog.seed()
    const summary = t('admin.modelCatalog.seedDone', {
      inserted: result.inserted,
      refreshed: result.refreshed,
      skipped: result.skipped_admin
    })
    if (result.failed > 0) {
      // 单条写库失败不拖垮整批，但不能静默：把失败数和前几条原因记下来。
      console.error(
        t('admin.modelCatalog.seedPartial', {
          summary,
          failed: result.failed,
          errors: (result.errors ?? []).join('；')
        })
      )
    }
    await loadEntries()
  } catch (error) {
    logApiError(error)
  } finally {
    seeding.value = false
  }
}

onMounted(loadEntries)
</script>
