<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-64">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.modelCatalog.search')"
              class="input"
              data-testid="model-catalog-search"
            />
          </div>
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
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
      </template>

      <template #table>
        <DataTable :columns="columns" :data="filteredEntries" :loading="loading">
          <template #cell-model_id="{ row }">
            <div class="min-w-0">
              <div class="truncate font-medium text-gray-900 dark:text-white">{{ row.model_id }}</div>
              <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ row.display_name }}</div>
            </div>
          </template>
          <template #cell-status="{ value }">
            <span :class="['badge', value === 'listed' ? 'badge-success' : 'badge-gray']">
              {{ t(`admin.modelCatalog.status.${value}`) }}
            </span>
          </template>
          <template #cell-managed_by="{ value }">
            {{ t(`admin.modelCatalog.managedBy.${value}`) }}
          </template>
          <template #cell-actions="{ row }">
            <div class="flex justify-end gap-2">
              <button type="button" class="btn btn-secondary btn-sm" @click="openEdit(row)">
                {{ t('common.edit') }}
              </button>
              <button type="button" class="btn btn-danger btn-sm" @click="askDelete(row)">
                {{ t('common.delete') }}
              </button>
            </div>
          </template>
        </DataTable>
        <EmptyState v-if="!loading && filteredEntries.length === 0" :title="t('admin.modelCatalog.empty')" />
      </template>
    </TablePageLayout>

    <BaseDialog :show="showEditor" :title="editorTitle" @close="closeEditor">
      <form id="model-catalog-form" class="space-y-4" @submit.prevent="saveEntry">
        <div>
          <label class="input-label">{{ t('admin.modelCatalog.fields.modelId') }}</label>
          <input v-model="form.model_id" class="input" required data-testid="model-catalog-model-id" />
        </div>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.displayName') }}</label>
            <input v-model="form.display_name" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.vendor') }}</label>
            <input v-model="form.vendor" class="input" />
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.billingMode') }}</label>
            <input v-model="form.billing_mode" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.status') }}</label>
            <select v-model="form.status" class="input">
              <option value="listed">{{ t('admin.modelCatalog.status.listed') }}</option>
              <option value="unlisted">{{ t('admin.modelCatalog.status.unlisted') }}</option>
            </select>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.inputPrice') }}</label>
            <input v-model.number="form.input_price" type="number" step="any" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.outputPrice') }}</label>
            <input v-model.number="form.output_price" type="number" step="any" class="input" />
          </div>
        </div>
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.modelCatalog.fullReplaceHint') }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeEditor">{{ t('common.cancel') }}</button>
          <button type="submit" form="model-catalog-form" class="btn btn-primary" :disabled="saving" data-testid="model-catalog-save">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

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
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogEntry, ModelCatalogEntryRequest } from '@/api/admin/modelCatalog'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const seeding = ref(false)
const saving = ref(false)
const entries = ref<ModelCatalogEntry[]>([])
const searchQuery = ref('')
const showEditor = ref(false)
const showDeleteDialog = ref(false)
const editingId = ref<number | null>(null)
const pendingDelete = ref<ModelCatalogEntry | null>(null)
const loadedEntry = ref<ModelCatalogEntry | null>(null)

const emptyForm = (): ModelCatalogEntryRequest => ({
  model_id: '',
  display_name: '',
  vendor: '',
  protocols: [],
  billing_mode: 'token',
  status: 'listed',
  input_price: null,
  output_price: null
})

const form = reactive<ModelCatalogEntryRequest>(emptyForm())

const columns = computed<Column[]>(() => [
  { key: 'model_id', label: t('admin.modelCatalog.fields.modelId') },
  { key: 'vendor', label: t('admin.modelCatalog.fields.vendor') },
  { key: 'status', label: t('admin.modelCatalog.fields.status') },
  { key: 'managed_by', label: t('admin.modelCatalog.fields.managedBy') },
  { key: 'actions', label: t('common.actions') }
])

const filteredEntries = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return entries.value
  return entries.value.filter((entry) =>
    [entry.model_id, entry.display_name, entry.vendor].some((value) => value.toLowerCase().includes(q))
  )
})

const editorTitle = computed(() =>
  editingId.value ? t('admin.modelCatalog.edit') : t('admin.modelCatalog.create')
)

async function loadEntries() {
  loading.value = true
  try {
    entries.value = await adminAPI.modelCatalog.listEntries()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('common.unknownError'))
  } finally {
    loading.value = false
  }
}

function assignForm(entry: Partial<ModelCatalogEntryRequest>) {
  Object.assign(form, emptyForm(), entry)
}

function openCreate() {
  editingId.value = null
  loadedEntry.value = null
  assignForm({})
  showEditor.value = true
}

function openEdit(entry: ModelCatalogEntry) {
  editingId.value = entry.id
  loadedEntry.value = entry
  assignForm({
    model_id: entry.model_id,
    display_name: entry.display_name,
    vendor: entry.vendor,
    protocols: entry.protocols,
    billing_mode: entry.billing_mode,
    status: entry.status,
    input_price: entry.input_price,
    output_price: entry.output_price,
    cache_write_price: entry.cache_write_price,
    cache_write_1h_price: entry.cache_write_1h_price,
    cache_read_price: entry.cache_read_price,
    image_input_price: entry.image_input_price,
    image_output_price: entry.image_output_price,
    image_cache_read_price: entry.image_cache_read_price,
    input_price_priority: entry.input_price_priority,
    output_price_priority: entry.output_price_priority,
    cache_write_price_priority: entry.cache_write_price_priority,
    cache_read_price_priority: entry.cache_read_price_priority,
    per_request_price: entry.per_request_price,
    long_context_input_threshold: entry.long_context_input_threshold,
    long_context_threshold_inclusive: entry.long_context_threshold_inclusive,
    long_context_input_multiplier: entry.long_context_input_multiplier,
    long_context_output_multiplier: entry.long_context_output_multiplier,
    fast_multiplier: entry.fast_multiplier,
    flex_multiplier: entry.flex_multiplier,
    max_reasoning_effort_multiplier: entry.max_reasoning_effort_multiplier,
    notes: entry.notes,
    intervals: entry.intervals,
    time_pricing: entry.time_pricing
  })
  showEditor.value = true
}

function closeEditor() {
  showEditor.value = false
}

function payload(): ModelCatalogEntryRequest {
  return {
    ...form,
    intervals: loadedEntry.value?.intervals ?? form.intervals ?? [],
    time_pricing: loadedEntry.value?.time_pricing ?? form.time_pricing ?? null
  }
}

async function saveEntry() {
  saving.value = true
  try {
    if (editingId.value) {
      await adminAPI.modelCatalog.updateEntry(editingId.value, payload())
    } else {
      await adminAPI.modelCatalog.createEntry(payload())
    }
    showEditor.value = false
    await loadEntries()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('common.unknownError'))
  } finally {
    saving.value = false
  }
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
    appStore.showError((error as { message?: string }).message || t('common.unknownError'))
  }
}

async function runSeed() {
  seeding.value = true
  try {
    const result = await adminAPI.modelCatalog.seed()
    appStore.showSuccess(
      t('admin.modelCatalog.seedDone', {
        inserted: result.inserted,
        refreshed: result.refreshed,
        skipped: result.skipped_admin
      })
    )
    await loadEntries()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('common.unknownError'))
  } finally {
    seeding.value = false
  }
}

onMounted(loadEntries)
</script>
