<template>
  <!--
    模型目录（A4 列表模板）：上架 = 用户能看到并调用；上架必有价、必有资源承接。
    标题右侧「⋯」（从价格文件播种）+「新建模型」；数字摘要（模型 / 已上架 / 上架但无渠道，可一键筛出）；
    工具行 = 搜索 + 状态 / 厂商 / 计费 / 资源筛选标签 + 刷新；行尾「编辑」图标 +「⋯」（诊断、删除）；选中行时批量上下架。
    点行打开详情抽屉（A5）：概况（全部价格、别名…）/ 渠道（绑定的渠道此刻能否调度 + 诊断）；抽屉右上「编辑」「⋯」。
    新建 / 编辑是独立页（/model-catalog/new、/model-catalog/:id/edit）。
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
            :label="t('admin.modelCatalog.fields.status')"
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
          <template #cell-vendor="{ value }">
            <span v-if="value" class="text-af-ink-2">{{ value }}</span>
            <span v-else class="text-af-ink-4">—</span>
          </template>
          <template #cell-billing_mode="{ value }">
            <span class="text-af-ink-2">{{ t(`admin.modelCatalog.billingModes.${value || 'token'}`) }}</span>
          </template>
          <template #cell-price="{ row }">
            <PriceCell :entry="row" />
          </template>
          <!-- 上架 / 下架都是常态：灰点纯文字，上架的点与字深一档 -->
          <template #cell-status="{ value }">
            <div class="flex items-center gap-1.5">
              <span :class="['inline-block h-2 w-2 rounded-full', value === 'listed' ? 'bg-af-ink-3' : 'bg-af-hairline-strong']"></span>
              <span :class="value === 'listed' ? 'text-af-ink' : 'text-af-ink-3'">{{ t(`admin.modelCatalog.status.${value}`) }}</span>
            </div>
          </template>
          <!-- 上架却没有资源承接是真异常：橙点橙字，点开诊断 -->
          <template #cell-resources="{ row }">
            <button
              v-if="row.status === 'listed' && bindingCount(row) === 0"
              type="button"
              class="inline-flex items-center gap-1.5 text-af-warning hover:underline"
              data-testid="model-catalog-no-resources"
              @click.stop="openDiagnosis(row)"
            >
              <span class="inline-block h-2 w-2 rounded-full bg-af-warning"></span>
              {{ t('admin.modelCatalog.noResources') }}
            </button>
            <button
              v-else
              type="button"
              class="tabular-nums text-af-ink-2 underline decoration-af-ink-4 decoration-dotted underline-offset-2 hover:text-af-ink"
              data-testid="model-catalog-resource-count"
              @click.stop="openDiagnosis(row)"
            >
              {{ bindingCount(row) }}
            </button>
          </template>
          <template #cell-managed_by="{ value }">
            <span class="text-af-ink-3">{{ t(`admin.modelCatalog.managedBy.${value}`) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>

          <template #empty>
            <EmptyState
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

    <CatalogEntryDiagnosisModal
      :show="diagnosisEntry !== null"
      :entry-id="diagnosisEntry?.id ?? null"
      :model-id="diagnosisEntry?.model_id"
      @close="diagnosisEntry = null"
    />

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.modelCatalog.deleteTitle')"
      :message="t('admin.modelCatalog.deleteConfirm')"
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
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
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
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import CatalogEntryDiagnosisModal from '@/components/admin/catalog/CatalogEntryDiagnosisModal.vue'
import CatalogEntryDrawer from '@/components/admin/catalog/CatalogEntryDrawer.vue'
import PriceCell from '@/components/admin/catalog/CatalogPriceCell.vue'
import { entryToRequest } from '@/components/admin/catalog/entryRequest'
import { getPersistedPageSize, setPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(false)
const seeding = ref(false)
const bulkRunning = ref(false)
const entries = ref<ModelCatalogEntry[]>([])
const selectedIds = ref<number[]>([])

// 筛选（空串 = 全部；厂商筛选里「无厂商」用 NO_VENDOR 占位，空串已表示不筛）
const NO_VENDOR = '__none__'
const searchQuery = ref('')
const statusFilter = ref('')
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

function showApiError(error: unknown) {
  appStore.showError(extractApiErrorMessage(error, t('common.unknownError')))
}

function bindingCount(entry: ModelCatalogEntry): number {
  return entry.bindings?.length ?? 0
}

const columns = computed<Column[]>(() => [
  { key: 'model_id', label: t('admin.modelCatalog.fields.modelId') },
  { key: 'vendor', label: t('admin.modelCatalog.fields.vendor') },
  { key: 'billing_mode', label: t('admin.modelCatalog.fields.billingMode') },
  { key: 'price', label: t('admin.modelCatalog.columns.price'), class: 'text-right' },
  { key: 'status', label: t('admin.modelCatalog.fields.status') },
  { key: 'resources', label: t('admin.modelCatalog.fields.resources') },
  { key: 'managed_by', label: t('admin.modelCatalog.fields.managedBy') },
  { key: 'actions', label: t('common.actions'), class: 'text-right' }
])

const vendorValues = computed(() => [...new Set(entries.value.map((entry) => entry.vendor).filter(Boolean))].sort())
const statusOptions = computed<FilterOption[]>(() => [
  { value: 'listed', label: t('admin.modelCatalog.status.listed') },
  { value: 'unlisted', label: t('admin.modelCatalog.status.unlisted') }
])
const vendorOptions = computed<FilterOption[]>(() => [
  ...vendorValues.value.map((vendor) => ({ value: vendor, label: vendor })),
  ...(entries.value.some((entry) => !entry.vendor) ? [{ value: NO_VENDOR, label: t('admin.modelCatalog.filters.noVendor') }] : [])
])
const billingOptions = computed<FilterOption[]>(() =>
  ['token', 'per_request', 'image', 'video'].map((mode) => ({ value: mode, label: t(`admin.modelCatalog.billingModes.${mode}`) }))
)
const resourceOptions = computed<FilterOption[]>(() => [
  { value: 'bound', label: t('admin.modelCatalog.filters.withResources') },
  { value: 'unbound', label: t('admin.modelCatalog.filters.withoutResources') }
])

const listedCount = computed(() => entries.value.filter((entry) => entry.status === 'listed').length)
const listedWithoutResources = computed(() => entries.value.filter((entry) => entry.status === 'listed' && bindingCount(entry) === 0).length)

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
    if (statusFilter.value && entry.status !== statusFilter.value) return false
    if (vendorFilter.value && (entry.vendor || NO_VENDOR) !== vendorFilter.value) return false
    if (billingFilter.value && (entry.billing_mode || 'token') !== billingFilter.value) return false
    if (resourceFilter.value === 'bound' && bindingCount(entry) === 0) return false
    if (resourceFilter.value === 'unbound' && bindingCount(entry) > 0) return false
    if (!q) return true
    return [entry.model_id, entry.display_name, entry.vendor, ...(entry.aliases ?? []).map((alias) => alias.alias)].some((value) =>
      (value || '').toLowerCase().includes(q)
    )
  })
})

const isFiltered = computed(() => filteredEntries.value.length !== entries.value.length)

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
    showApiError(error)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  void router.push('/model-catalog/new')
}

function openEdit(entry: ModelCatalogEntry) {
  void router.push(`/model-catalog/${entry.id}/edit`)
}

function openDiagnosis(entry: ModelCatalogEntry) {
  diagnosisEntry.value = entry
}

function openDrawer(entry: ModelCatalogEntry, tab = 'overview') {
  drawerEntryId.value = entry.id
  drawerTab.value = tab
  drawerOpen.value = true
}

/** 抽屉「⋯」里的上架 / 下架：与批量上下架同一条路（整条覆盖 PUT，价格 / 绑定校验在后端） */
async function setEntryStatus(entry: ModelCatalogEntry, status: 'listed' | 'unlisted') {
  if (entry.status === status) return
  try {
    await adminAPI.modelCatalog.updateEntry(entry.id, { ...entryToRequest(entry), status })
    appStore.showSuccess(t(`admin.modelCatalog.drawer.${status}Done`, { model: entry.model_id }))
    await loadEntries()
  } catch (error) {
    showApiError(error)
  }
}

// 行操作（A4）：编辑是图标；诊断、删除进「⋯」，删除红字且仍走确认框
function rowActions(entry: ModelCatalogEntry): RowAction[] {
  return [
    { key: 'edit', label: t('common.edit'), icon: 'edit', primary: true, onSelect: () => openEdit(entry) },
    { key: 'diagnose', label: t('admin.modelCatalog.diagnose'), icon: 'beaker', onSelect: () => openDiagnosis(entry) },
    { key: 'delete', label: t('common.delete'), icon: 'trash', danger: true, dividerBefore: true, onSelect: () => askDelete(entry) }
  ]
}

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
    showApiError(error)
  }
}

/**
 * 批量上下架：没有批量接口，逐条整条覆盖 PUT（上架必有价 / 绑定校验都在后端，被拒的逐条列出来）。
 * 已是目标状态的跳过；全部成功提示条数，部分失败把失败的模型与原因一起摆出来。
 */
async function bulkSetStatus(status: 'listed' | 'unlisted') {
  const targets = entries.value.filter((entry) => selectedIds.value.includes(entry.id) && entry.status !== status)
  if (targets.length === 0) {
    appStore.showInfo(t('admin.modelCatalog.bulk.nothingToDo'))
    return
  }
  bulkRunning.value = true
  const failures: string[] = []
  try {
    for (const entry of targets) {
      try {
        await adminAPI.modelCatalog.updateEntry(entry.id, { ...entryToRequest(entry), status })
      } catch (error) {
        failures.push(`${entry.model_id}: ${extractApiErrorMessage(error, t('common.unknownError'))}`)
      }
    }
    const done = targets.length - failures.length
    if (failures.length === 0) {
      appStore.showSuccess(t(`admin.modelCatalog.bulk.${status}Done`, { count: done }))
      selectedIds.value = []
    } else {
      appStore.showError(t('admin.modelCatalog.bulk.partial', { done, failed: failures.length, errors: failures.join('；') }))
      // 失败的留在选中集里，方便修完价格 / 绑定再试
      const failedIds = new Set(targets.filter((entry) => failures.some((line) => line.startsWith(`${entry.model_id}:`))).map((entry) => entry.id))
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
      // 单条写库失败不拖垮整批，但不能静默：把失败数和前几条原因摆出来。
      appStore.showError(
        t('admin.modelCatalog.seedPartial', {
          summary,
          failed: result.failed,
          errors: (result.errors ?? []).join('；')
        })
      )
    } else {
      appStore.showSuccess(summary)
    }
    await loadEntries()
  } catch (error) {
    showApiError(error)
  } finally {
    seeding.value = false
  }
}

onMounted(loadEntries)
</script>
