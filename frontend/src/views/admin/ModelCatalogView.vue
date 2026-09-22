<template>
  <!--
    模型目录：上架 = 用户能看到并调用；上架必有价、必有资源承接。
    筛选（状态 / 厂商 / 计费 / 资源）→ 汇总行 → 表格（标价按百万 Token 列出）→ 分页；勾选后批量上下架。
  -->
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-3">
          <div class="flex flex-wrap items-center gap-3">
            <SearchInput
              v-model="searchQuery"
              :placeholder="t('admin.modelCatalog.search')"
              class="w-full sm:w-72"
              data-testid="model-catalog-search"
            />
            <Select v-model="statusFilter" :options="statusOptions" class="w-36" data-testid="model-catalog-filter-status" />
            <Select v-model="vendorFilter" :options="vendorOptions" class="w-44" data-testid="model-catalog-filter-vendor" />
            <Select v-model="billingFilter" :options="billingOptions" class="w-36" data-testid="model-catalog-filter-billing" />
            <Select v-model="resourceFilter" :options="resourceOptions" class="w-36" data-testid="model-catalog-filter-resources" />
            <div class="ml-auto flex items-center gap-2">
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="seeding"
                data-testid="model-catalog-seed"
                @click="runSeed"
              >
                {{ seeding ? t('admin.modelCatalog.seeding') : t('admin.modelCatalog.seed') }}
              </button>
              <button type="button" class="btn btn-primary" data-testid="model-catalog-create" @click="openCreate">
                <Icon name="plus" size="md" class="mr-1" />
                {{ t('admin.modelCatalog.create') }}
              </button>
            </div>
          </div>

          <!-- 汇总 + 批量操作 -->
          <div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-13 text-af-ink-3">
            <span data-testid="model-catalog-summary">
              {{ t('admin.modelCatalog.summary', { total: entries.length, listed: listedCount, noResources: listedWithoutResources }) }}
            </span>
            <span v-if="filteredEntries.length !== entries.length" class="text-af-ink-4">
              {{ t('admin.modelCatalog.filtered', { count: filteredEntries.length }) }}
            </span>
            <template v-if="selectedIds.length">
              <span class="text-af-ink">{{ t('admin.modelCatalog.bulk.selected', { count: selectedIds.length }) }}</span>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="bulkRunning" data-testid="model-catalog-bulk-list" @click="bulkSetStatus('listed')">
                {{ t('admin.modelCatalog.bulk.list') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="bulkRunning" data-testid="model-catalog-bulk-unlist" @click="bulkSetStatus('unlisted')">
                {{ t('admin.modelCatalog.bulk.unlist') }}
              </button>
              <button type="button" class="btn btn-ghost btn-sm" @click="selectedIds = []">{{ t('admin.modelCatalog.bulk.clear') }}</button>
            </template>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="pagedEntries"
          :loading="loading"
          selectable
          row-key="id"
          :selected-keys="selectedIds"
          @update:selected-keys="handleSelectionChange"
        >
          <template #cell-model_id="{ row }">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="truncate font-mono font-medium text-af-ink">{{ row.model_id }}</span>
                <span v-if="row.aliases?.length" class="badge badge-gray" :title="row.aliases.map((a: ModelCatalogAlias) => a.alias).join(', ')">
                  {{ t('admin.modelCatalog.aliasCount', { count: row.aliases.length }) }}
                </span>
              </div>
              <div v-if="row.display_name" class="mt-0.5 truncate text-xs text-af-ink-3">{{ row.display_name }}</div>
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
          <template #cell-status="{ value }">
            <span :class="['badge', value === 'listed' ? 'badge-success' : 'badge-gray']">
              {{ t(`admin.modelCatalog.status.${value}`) }}
            </span>
          </template>
          <template #cell-resources="{ row }">
            <button
              v-if="row.status === 'listed' && bindingCount(row) === 0"
              type="button"
              class="badge badge-warning"
              data-testid="model-catalog-no-resources"
              @click="openDiagnosis(row)"
            >
              {{ t('admin.modelCatalog.noResources') }}
            </button>
            <button
              v-else
              type="button"
              class="tabular-nums text-af-ink-2 underline decoration-af-ink-4 decoration-dotted underline-offset-2 hover:text-af-ink"
              data-testid="model-catalog-resource-count"
              @click="openDiagnosis(row)"
            >
              {{ bindingCount(row) }}
            </button>
          </template>
          <template #cell-managed_by="{ value }">
            <span class="text-af-ink-3">{{ t(`admin.modelCatalog.managedBy.${value}`) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-0.5">
              <button
                type="button"
                class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                :title="t('admin.modelCatalog.diagnose')"
                :aria-label="t('admin.modelCatalog.diagnose')"
                data-testid="model-catalog-actions-diagnose"
                @click="openDiagnosis(row)"
              >
                <Icon name="beaker" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                :title="t('common.edit')"
                :aria-label="t('common.edit')"
                data-testid="model-catalog-actions-edit"
                @click="openEdit(row)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-danger-tint hover:text-af-danger"
                :title="t('common.delete')"
                :aria-label="t('common.delete')"
                data-testid="model-catalog-actions-delete"
                @click="askDelete(row)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>
        </DataTable>
        <EmptyState v-if="!loading && filteredEntries.length === 0" :title="entries.length ? t('admin.modelCatalog.noMatch') : t('admin.modelCatalog.empty')" />
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

    <CatalogEntryDiagnosisModal
      :show="diagnosisEntry !== null"
      :entry-id="diagnosisEntry?.id ?? null"
      :model-id="diagnosisEntry?.model_id"
      @close="diagnosisEntry = null"
    />

    <CatalogEntryEditor :show="showEditor" :entry="editingEntry" :vendor-options="vendorValues" @close="showEditor = false" @saved="onSaved" />

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
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ModelCatalogAlias, ModelCatalogEntry } from '@/api/admin/modelCatalog'
import type { Column } from '@/components/common/types'
import type { SelectOption } from '@/components/common/Select.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import CatalogEntryDiagnosisModal from '@/components/admin/catalog/CatalogEntryDiagnosisModal.vue'
import CatalogEntryEditor from '@/components/admin/catalog/CatalogEntryEditor.vue'
import PriceCell from '@/components/admin/catalog/CatalogPriceCell.vue'
import { entryToRequest } from '@/components/admin/catalog/entryRequest'
import { getPersistedPageSize, setPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const seeding = ref(false)
const bulkRunning = ref(false)
const entries = ref<ModelCatalogEntry[]>([])
const selectedIds = ref<number[]>([])

// 筛选
const searchQuery = ref('')
const statusFilter = ref('all')
const vendorFilter = ref('all')
const billingFilter = ref('all')
const resourceFilter = ref('all')

// 弹窗
const showEditor = ref(false)
const editingEntry = ref<ModelCatalogEntry | null>(null)
const diagnosisEntry = ref<ModelCatalogEntry | null>(null)
const showDeleteDialog = ref(false)
const pendingDelete = ref<ModelCatalogEntry | null>(null)

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
const statusOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('admin.modelCatalog.filters.allStatus') },
  { value: 'listed', label: t('admin.modelCatalog.status.listed') },
  { value: 'unlisted', label: t('admin.modelCatalog.status.unlisted') }
])
const vendorOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('admin.modelCatalog.filters.allVendors') },
  ...vendorValues.value.map((vendor) => ({ value: vendor, label: vendor })),
  ...(entries.value.some((entry) => !entry.vendor) ? [{ value: '', label: t('admin.modelCatalog.filters.noVendor') }] : [])
])
const billingOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('admin.modelCatalog.filters.allBilling') },
  ...['token', 'per_request', 'image', 'video'].map((mode) => ({ value: mode, label: t(`admin.modelCatalog.billingModes.${mode}`) }))
])
const resourceOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('admin.modelCatalog.filters.allResources') },
  { value: 'bound', label: t('admin.modelCatalog.filters.withResources') },
  { value: 'unbound', label: t('admin.modelCatalog.filters.withoutResources') }
])

const listedCount = computed(() => entries.value.filter((entry) => entry.status === 'listed').length)
const listedWithoutResources = computed(() => entries.value.filter((entry) => entry.status === 'listed' && bindingCount(entry) === 0).length)

const filteredEntries = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return entries.value.filter((entry) => {
    if (statusFilter.value !== 'all' && entry.status !== statusFilter.value) return false
    if (vendorFilter.value !== 'all' && (entry.vendor || '') !== vendorFilter.value) return false
    if (billingFilter.value !== 'all' && (entry.billing_mode || 'token') !== billingFilter.value) return false
    if (resourceFilter.value === 'bound' && bindingCount(entry) === 0) return false
    if (resourceFilter.value === 'unbound' && bindingCount(entry) > 0) return false
    if (!q) return true
    return [entry.model_id, entry.display_name, entry.vendor, ...(entry.aliases ?? []).map((alias) => alias.alias)].some((value) =>
      (value || '').toLowerCase().includes(q)
    )
  })
})

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
  editingEntry.value = null
  showEditor.value = true
}

function openEdit(entry: ModelCatalogEntry) {
  editingEntry.value = entry
  showEditor.value = true
}

function openDiagnosis(entry: ModelCatalogEntry) {
  diagnosisEntry.value = entry
}

async function onSaved() {
  showEditor.value = false
  await loadEntries()
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
