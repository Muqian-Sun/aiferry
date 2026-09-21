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
          <template #cell-resources="{ row }">
            <span
              v-if="row.status === 'listed' && bindingCount(row) === 0"
              class="badge badge-warning"
              data-testid="model-catalog-no-resources"
            >
              {{ t('admin.modelCatalog.noResources') }}
            </span>
            <span v-else data-testid="model-catalog-resource-count">{{ bindingCount(row) }}</span>
          </template>
          <template #cell-managed_by="{ value }">
            {{ t(`admin.modelCatalog.managedBy.${value}`) }}
          </template>
          <template #cell-actions="{ row }">
            <div class="flex justify-end gap-2">
              <button type="button" class="btn btn-secondary btn-sm" data-testid="model-catalog-actions-edit" @click="openEdit(row)">
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
            <select v-model="form.billing_mode" class="input" data-testid="model-catalog-billing-mode">
              <option v-for="mode in billingModes" :key="mode" :value="mode">
                {{ t(`admin.modelCatalog.billingModes.${mode}`) }}
              </option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.status') }}</label>
            <select v-model="form.status" class="input" data-testid="model-catalog-status">
              <option value="listed">{{ t('admin.modelCatalog.status.listed') }}</option>
              <option value="unlisted">{{ t('admin.modelCatalog.status.unlisted') }}</option>
            </select>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.inputPrice') }}</label>
            <input v-model.number="form.input_price" type="number" step="any" class="input" data-testid="model-catalog-input-price" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.outputPrice') }}</label>
            <input v-model.number="form.output_price" type="number" step="any" class="input" data-testid="model-catalog-output-price" />
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div v-if="isMediaMode || form.billing_mode === 'per_request'">
            <label class="input-label">{{ t(`admin.modelCatalog.fields.${perRequestPriceLabelKey}`) }}</label>
            <input v-model.number="form.per_request_price" type="number" step="any" class="input" data-testid="model-catalog-per-request-price" />
          </div>
          <div v-else>
            <label class="input-label">{{ t('admin.modelCatalog.fields.searchPricePerCall') }}</label>
            <input v-model.number="form.search_price_per_call" type="number" step="any" class="input" data-testid="model-catalog-search-price-per-call" />
          </div>
        </div>
        <div v-if="isMediaMode" class="space-y-2" data-testid="model-catalog-media-tiers">
          <div class="flex items-center justify-between">
            <div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.modelCatalog.tiers.title') }}</div>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t(`admin.modelCatalog.tiers.hint.${form.billing_mode}`) }}</p>
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="availableTierLabels.length === 0"
              data-testid="model-catalog-media-tier-add"
              @click="addMediaTier"
            >
              {{ t('admin.modelCatalog.tiers.add') }}
            </button>
          </div>
          <p v-if="mediaTiers.length === 0" class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.modelCatalog.tiers.empty') }}</p>
          <div
            v-for="(tier, index) in mediaTiers"
            :key="index"
            class="grid grid-cols-[1fr_1fr_auto] items-end gap-3"
            data-testid="model-catalog-media-tier-row"
          >
            <div>
              <label class="input-label">{{ t('admin.modelCatalog.tiers.tier') }}</label>
              <select v-model="tier.tier_label" class="input" data-testid="model-catalog-media-tier-label">
                <option v-for="label in mediaTierLabels" :key="label" :value="label">{{ label }}</option>
              </select>
            </div>
            <div>
              <label class="input-label">{{ t('admin.modelCatalog.tiers.price') }}</label>
              <input v-model.number="tier.per_request_price" type="number" step="any" class="input" data-testid="model-catalog-media-tier-price" />
            </div>
            <button type="button" class="btn btn-secondary btn-sm" data-testid="model-catalog-media-tier-remove" @click="removeMediaTier(index)">
              {{ t('admin.modelCatalog.tiers.remove') }}
            </button>
          </div>
        </div>
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.modelCatalog.listedRequiresPrice') }}</p>
        <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.modelCatalog.fullReplaceHint') }}</p>

        <div class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-600">
          <div>
            <div class="font-medium text-gray-900 dark:text-white">{{ t('admin.modelCatalog.bindings.title') }}</div>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.modelCatalog.bindings.hint') }}</p>
          </div>
          <div>
            <input
              v-model="resourceQuery"
              type="text"
              class="input"
              :placeholder="t('admin.modelCatalog.bindings.search')"
              data-testid="model-catalog-resource-search"
              @input="scheduleResourceSearch"
            />
            <ul
              v-if="resourceResults.length > 0"
              class="mt-2 max-h-48 divide-y divide-gray-100 overflow-y-auto rounded-md border border-gray-200 dark:divide-dark-700 dark:border-dark-600"
              data-testid="model-catalog-resource-results"
            >
              <li
                v-for="account in resourceResults"
                :key="account.id"
                class="flex items-center justify-between gap-3 px-3 py-2 text-sm"
              >
                <span class="flex min-w-0 items-center gap-2">
                  <PlatformTypeBadge :platform="account.platform" :type="account.type" :vendor="account.vendor" />
                  <span class="truncate">{{ account.name }}</span>
                </span>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="isBound(account.id)"
                  data-testid="model-catalog-resource-add"
                  @click="addBinding(account)"
                >
                  {{ t('admin.modelCatalog.bindings.add') }}
                </button>
              </li>
            </ul>
            <p v-else-if="resourceSearched" class="mt-2 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.modelCatalog.bindings.noResults') }}
            </p>
          </div>
          <ul v-if="bindings.length > 0" class="space-y-2" data-testid="model-catalog-bindings">
            <li
              v-for="binding in bindings"
              :key="binding.account_id"
              class="flex flex-wrap items-center gap-3 rounded-md border border-gray-200 px-3 py-2 dark:border-dark-600"
            >
              <span class="flex min-w-0 flex-1 items-center gap-2 text-sm">
                <PlatformTypeBadge
                  v-if="binding.account"
                  :platform="badgePlatform(binding.account.platform)"
                  :type="badgeType(binding.account.type)"
                  :vendor="binding.account.vendor"
                />
                <span class="truncate">{{ binding.account?.name ?? `#${binding.account_id}` }}</span>
              </span>
              <label class="flex items-center gap-2 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.modelCatalog.bindings.priority') }}
                <input
                  v-model.number="binding.priority"
                  type="number"
                  class="input w-24"
                  data-testid="model-catalog-binding-priority"
                />
              </label>
              <button
                type="button"
                class="btn btn-danger btn-sm"
                data-testid="model-catalog-binding-remove"
                @click="removeBinding(binding.account_id)"
              >
                {{ t('admin.modelCatalog.bindings.remove') }}
              </button>
            </li>
          </ul>
          <p v-else class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.modelCatalog.bindings.empty') }}</p>
        </div>
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
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type {
  ModelCatalogBinding,
  ModelCatalogBindingAccount,
  ModelCatalogEntry,
  ModelCatalogEntryRequest,
  PricingInterval
} from '@/api/admin/modelCatalog'
import type { AccountListItem, AccountPlatform, AccountType } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
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

// 绑定资源：编辑器里的工作副本，保存时整份覆盖。
const bindings = ref<ModelCatalogBinding[]>([])
const resourceQuery = ref('')
const resourceResults = ref<AccountListItem[]>([])
const resourceSearched = ref(false)
let resourceSearchTimer: ReturnType<typeof setTimeout> | null = null

const emptyForm = (): ModelCatalogEntryRequest => ({
  model_id: '',
  display_name: '',
  vendor: '',
  protocols: [],
  billing_mode: 'token',
  status: 'listed',
  input_price: null,
  output_price: null,
  per_request_price: null,
  search_price_per_call: null
})

const form = reactive<ModelCatalogEntryRequest>(emptyForm())

// 与后端 BillingMode 一致；目录条目的计费模式只能是这四种。
const billingModes = ['token', 'per_request', 'image', 'video'] as const

// 图片 / 视频分档：档位只能是后端认的这几个（计费查档区分大小写），每档一个按次价。
const imageTierLabels = ['1K', '2K', '4K'] as const
const videoTierLabels = ['480p', '720p', '1080p'] as const

interface MediaTierForm {
  tier_label: string
  per_request_price: number | null
}

const mediaTiers = ref<MediaTierForm[]>([])
const isMediaMode = computed(() => form.billing_mode === 'image' || form.billing_mode === 'video')
const mediaTierLabels = computed<readonly string[]>(() => (form.billing_mode === 'video' ? videoTierLabels : imageTierLabels))
const availableTierLabels = computed(() => mediaTierLabels.value.filter((label) => !mediaTiers.value.some((tier) => tier.tier_label === label)))
const perRequestPriceLabelKey = computed(() => {
  if (form.billing_mode === 'image') return 'perImagePrice'
  if (form.billing_mode === 'video') return 'perSecondPrice'
  return 'perRequestPrice'
})

function addMediaTier() {
  const label = availableTierLabels.value[0]
  if (!label) return
  mediaTiers.value.push({ tier_label: label, per_request_price: null })
}

function removeMediaTier(index: number) {
  mediaTiers.value.splice(index, 1)
}

// 编辑器里的分档从条目 intervals 投影；只认带 tier_label 的（token 区间分档不进这张表）。
function mediaTiersFromIntervals(intervals: PricingInterval[] | undefined): MediaTierForm[] {
  return (intervals ?? [])
    .filter((iv) => iv.tier_label)
    .map((iv) => ({ tier_label: iv.tier_label, per_request_price: iv.per_request_price }))
}

function mediaTiersToIntervals(tiers: MediaTierForm[]): PricingInterval[] {
  return tiers.map((tier, index) => ({
    min_tokens: 0,
    max_tokens: null,
    tier_label: tier.tier_label,
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    input_multiplier: null,
    output_multiplier: null,
    cache_write_multiplier: null,
    cache_read_multiplier: null,
    per_request_price: numberOrNull(tier.per_request_price),
    sort_order: index
  }))
}

// 数字输入清空后 v-model.number 得到 ''，后端按 *float64 解析会报 400：清空即「未配置」，发 null。
const numericFields = [
  'input_price',
  'output_price',
  'cache_write_price',
  'cache_write_1h_price',
  'cache_read_price',
  'image_input_price',
  'image_output_price',
  'image_cache_read_price',
  'input_price_priority',
  'output_price_priority',
  'cache_write_price_priority',
  'cache_read_price_priority',
  'per_request_price',
  'search_price_per_call',
  'long_context_input_threshold',
  'long_context_input_multiplier',
  'long_context_output_multiplier',
  'fast_multiplier',
  'flex_multiplier',
  'max_reasoning_effort_multiplier'
] as const satisfies readonly (keyof ModelCatalogEntryRequest)[]

function numberOrNull(value: unknown): number | null {
  if (value === '' || value === null || value === undefined) return null
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function showApiError(error: unknown) {
  appStore.showError(extractApiErrorMessage(error, t('common.unknownError')))
}

function bindingCount(entry: ModelCatalogEntry): number {
  return entry.bindings?.length ?? 0
}

// 绑定摘要里的 platform / type 是后端字符串，徽章按前端联合类型接收。
function badgePlatform(platform: string): AccountPlatform {
  return platform as AccountPlatform
}

function badgeType(type: string): AccountType {
  return type as AccountType
}

const columns = computed<Column[]>(() => [
  { key: 'model_id', label: t('admin.modelCatalog.fields.modelId') },
  { key: 'vendor', label: t('admin.modelCatalog.fields.vendor') },
  { key: 'status', label: t('admin.modelCatalog.fields.status') },
  { key: 'resources', label: t('admin.modelCatalog.fields.resources') },
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
    showApiError(error)
  } finally {
    loading.value = false
  }
}

function assignForm(entry: Partial<ModelCatalogEntryRequest>) {
  Object.assign(form, emptyForm(), entry)
  mediaTiers.value = mediaTiersFromIntervals(entry.intervals)
}

function resetBindingsEditor() {
  bindings.value = []
  resourceQuery.value = ''
  resourceResults.value = []
  resourceSearched.value = false
}

function openCreate() {
  editingId.value = null
  loadedEntry.value = null
  assignForm({})
  resetBindingsEditor()
  showEditor.value = true
}

async function openEdit(entry: ModelCatalogEntry) {
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
    search_price_per_call: entry.search_price_per_call,
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
  resetBindingsEditor()
  showEditor.value = true
  try {
    bindings.value = await adminAPI.modelCatalog.getBindings(entry.id)
  } catch (error) {
    showApiError(error)
  }
}

function closeEditor() {
  showEditor.value = false
}

function isBound(accountId: number): boolean {
  return bindings.value.some((binding) => binding.account_id === accountId)
}

function toBindingAccount(account: AccountListItem): ModelCatalogBindingAccount {
  return {
    id: account.id,
    name: account.name,
    platform: account.platform,
    type: account.type,
    vendor: account.vendor ?? '',
    status: account.status
  }
}

function addBinding(account: AccountListItem) {
  if (isBound(account.id)) return
  bindings.value = [
    ...bindings.value,
    { entry_id: editingId.value ?? 0, account_id: account.id, priority: null, account: toBindingAccount(account) }
  ]
}

function removeBinding(accountId: number) {
  bindings.value = bindings.value.filter((binding) => binding.account_id !== accountId)
}

function scheduleResourceSearch() {
  if (resourceSearchTimer) clearTimeout(resourceSearchTimer)
  resourceSearchTimer = setTimeout(() => {
    resourceSearchTimer = null
    void searchResources()
  }, 300)
}

async function searchResources() {
  const query = resourceQuery.value.trim()
  if (!query) {
    resourceResults.value = []
    resourceSearched.value = false
    return
  }
  try {
    const result = await adminAPI.accounts.list(1, 20, { search: query, lite: 'true' })
    resourceResults.value = result.items ?? []
    resourceSearched.value = true
  } catch (error) {
    showApiError(error)
  }
}

function payload(): ModelCatalogEntryRequest {
  const body: ModelCatalogEntryRequest = {
    ...form,
    // 图片 / 视频模式的分档由本页编辑；其余模式的区间分档按原值写回。
    intervals: isMediaMode.value ? mediaTiersToIntervals(mediaTiers.value) : (loadedEntry.value?.intervals ?? form.intervals ?? []),
    time_pricing: loadedEntry.value?.time_pricing ?? form.time_pricing ?? null
  }
  for (const field of numericFields) {
    body[field] = numberOrNull(form[field])
  }
  return body
}

function bindingsPayload() {
  return bindings.value.map((binding) => ({
    account_id: binding.account_id,
    priority: numberOrNull(binding.priority)
  }))
}

// 保存顺序：先存条目（新建时拿到 ID），再整份覆盖绑定。绑定被拒（资源承接不了该网关族）
// 时条目已保存，弹出后端给的原因，编辑器保持打开让用户改绑定。
async function saveEntry() {
  saving.value = true
  try {
    let entryId = editingId.value
    if (entryId) {
      await adminAPI.modelCatalog.updateEntry(entryId, payload())
    } else {
      const created = await adminAPI.modelCatalog.createEntry(payload())
      entryId = created.id
      editingId.value = created.id
    }
    await adminAPI.modelCatalog.updateBindings(entryId, bindingsPayload())
    showEditor.value = false
    await loadEntries()
  } catch (error) {
    showApiError(error)
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
    showApiError(error)
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
onBeforeUnmount(() => {
  if (resourceSearchTimer) clearTimeout(resourceSearchTimer)
})
</script>
