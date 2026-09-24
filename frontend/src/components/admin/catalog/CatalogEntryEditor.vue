<template>
  <!--
    目录条目编辑器：基本信息 / 计费与标价（含收起的「更多价格」）/ 图片视频分档 / 绑定资源，一个弹窗。
    保存是整条覆盖：表单从条目整条投影（entryToRequest），没露出的字段（优先级价、长上下文、倍率、分时…）原样写回。
    保存顺序：先存条目（新建时拿到 ID），再整份覆盖绑定；绑定被拒时条目已保存，弹出后端原因、编辑器保持打开。
  -->
  <BaseDialog :show="show" :title="title" width="wide" @close="emit('close')">
    <form id="model-catalog-form" class="space-y-6" @submit.prevent="save">
      <section class="space-y-4">
        <h3 class="text-13 font-medium text-af-ink-3">{{ t('admin.modelCatalog.editor.basics') }}</h3>
        <div>
          <label class="input-label">{{ t('admin.modelCatalog.fields.modelId') }}</label>
          <input v-model="form.model_id" class="input font-mono" required data-testid="model-catalog-model-id" />
        </div>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.displayName') }}</label>
            <input v-model="form.display_name" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.vendor') }}</label>
            <input v-model="form.vendor" class="input" list="model-catalog-vendor-options" data-testid="model-catalog-vendor" />
            <datalist id="model-catalog-vendor-options">
              <option v-for="vendor in vendorOptions" :key="vendor" :value="vendor" />
            </datalist>
            <p class="input-hint">{{ t('admin.modelCatalog.editor.vendorHint') }}</p>
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
      </section>

      <section class="space-y-4 border-t border-af-hairline pt-5">
        <h3 class="text-13 font-medium text-af-ink-3">{{ t('admin.modelCatalog.editor.pricing') }}</h3>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.inputPrice') }}</label>
            <input v-model.number="form.input_price" type="number" step="any" class="input" data-testid="model-catalog-input-price" />
            <p class="input-hint">{{ perMillionHint(form.input_price) }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.modelCatalog.fields.outputPrice') }}</label>
            <input v-model.number="form.output_price" type="number" step="any" class="input" data-testid="model-catalog-output-price" />
            <p class="input-hint">{{ perMillionHint(form.output_price) }}</p>
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
        <!-- 更多价格：缓存 / 图片 / 音频的 Token 单价，默认收起；收起时也按原值写回 -->
        <div class="space-y-3" data-testid="model-catalog-more-prices">
          <button
            type="button"
            class="inline-flex items-center gap-1 text-13 font-medium text-af-ink-2 transition-colors hover:text-af-ink"
            :aria-expanded="showMorePrices ? 'true' : 'false'"
            data-testid="model-catalog-more-prices-toggle"
            @click="showMorePrices = !showMorePrices"
          >
            <Icon name="chevronRight" size="sm" :class="['transition-transform', showMorePrices ? 'rotate-90' : '']" />
            {{ t('admin.modelCatalog.editor.morePrices') }}
            <span v-if="morePricesFilled > 0" class="font-normal text-af-ink-3">
              · {{ t('admin.modelCatalog.editor.morePricesFilled', { count: morePricesFilled }) }}
            </span>
          </button>
          <template v-if="showMorePrices">
            <p class="text-xs text-af-ink-3">{{ t('admin.modelCatalog.editor.morePricesHint') }}</p>
            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <div v-for="field in MORE_PRICE_FIELDS" :key="field.key">
                <label class="input-label">{{ t(`admin.modelCatalog.fields.${field.label}`) }}</label>
                <input v-model.number="form[field.key]" type="number" step="any" class="input" :data-testid="`model-catalog-${field.testId}`" />
                <p class="input-hint">{{ perMillionHint(form[field.key]) }}</p>
              </div>
            </div>
          </template>
        </div>
        <div v-if="isMediaMode" class="space-y-2" data-testid="model-catalog-media-tiers">
          <div class="flex items-center justify-between">
            <div>
              <div class="text-sm font-medium text-af-ink">{{ t('admin.modelCatalog.tiers.title') }}</div>
              <p class="mt-1 text-xs text-af-ink-3">{{ t(`admin.modelCatalog.tiers.hint.${form.billing_mode}`) }}</p>
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
          <p v-if="mediaTiers.length === 0" class="text-xs text-af-ink-3">{{ t('admin.modelCatalog.tiers.empty') }}</p>
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
        <p class="text-xs text-af-ink-3">{{ t('admin.modelCatalog.listedRequiresPrice') }}</p>
        <p class="text-xs text-af-ink-3">{{ t('admin.modelCatalog.fullReplaceHint') }}</p>
      </section>

      <section class="border-t border-af-hairline pt-5">
        <CatalogBindingsEditor ref="bindingsEditorRef" v-model="bindings" :entry-id="editingId" />
      </section>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="model-catalog-form" class="btn btn-primary" :disabled="saving" data-testid="model-catalog-save">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ModelCatalogBinding, ModelCatalogEntry, ModelCatalogEntryRequest } from '@/api/admin/modelCatalog'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import CatalogBindingsEditor from './CatalogBindingsEditor.vue'
import {
  IMAGE_TIER_LABELS,
  NUMERIC_FIELDS,
  VIDEO_TIER_LABELS,
  applyOptionalPrices,
  entryToRequest,
  mediaTiersFromIntervals,
  mediaTiersToIntervals,
  numberOrNull,
  type MediaTierForm
} from './entryRequest'

const props = defineProps<{
  show: boolean
  /** null = 新建 */
  entry: ModelCatalogEntry | null
  /** 目录里已有的厂商标签，给厂商输入框做联想 */
  vendorOptions: string[]
}>()
const emit = defineEmits<{ close: []; saved: [] }>()

const { t } = useI18n()
const appStore = useAppStore()

const saving = ref(false)
const editingId = ref<number | null>(null)
const loadedEntry = ref<ModelCatalogEntry | null>(null)
const bindings = ref<ModelCatalogBinding[]>([])
const bindingsEditorRef = ref<InstanceType<typeof CatalogBindingsEditor> | null>(null)

// 新建用的空表单要把每个字段都列全：Object.assign 只覆盖列出的键，漏一个就会把上一次编辑的条目的值带进新条目
const emptyForm = (): ModelCatalogEntryRequest => ({
  model_id: '',
  display_name: '',
  vendor: '',
  protocols: [],
  billing_mode: 'token',
  status: 'listed',
  ...(Object.fromEntries(NUMERIC_FIELDS.map((field) => [field, null])) as Record<(typeof NUMERIC_FIELDS)[number], null>),
  audio_input_price: null,
  audio_output_price: null,
  long_context_threshold_inclusive: false,
  notes: null,
  intervals: [],
  time_pricing: null
})
const form = reactive<ModelCatalogEntryRequest>(emptyForm())

// 「更多价格」里的字段：都是 $/token，和输入 / 输出价一样给百万 Token 换算
type MorePriceKey =
  | 'cache_write_price'
  | 'cache_write_1h_price'
  | 'cache_read_price'
  | 'image_input_price'
  | 'image_output_price'
  | 'image_cache_read_price'
  | 'audio_input_price'
  | 'audio_output_price'
const MORE_PRICE_FIELDS: ReadonlyArray<{ key: MorePriceKey; label: string; testId: string }> = [
  { key: 'cache_write_price', label: 'cacheWritePrice', testId: 'cache-write-price' },
  { key: 'cache_write_1h_price', label: 'cacheWrite1hPrice', testId: 'cache-write-1h-price' },
  { key: 'cache_read_price', label: 'cacheReadPrice', testId: 'cache-read-price' },
  { key: 'image_input_price', label: 'imageInputPrice', testId: 'image-input-price' },
  { key: 'image_output_price', label: 'imageOutputPrice', testId: 'image-output-price' },
  { key: 'image_cache_read_price', label: 'imageCacheReadPrice', testId: 'image-cache-read-price' },
  { key: 'audio_input_price', label: 'audioInputPrice', testId: 'audio-input-price' },
  { key: 'audio_output_price', label: 'audioOutputPrice', testId: 'audio-output-price' }
]
const showMorePrices = ref(false)
const morePricesFilled = computed(() => MORE_PRICE_FIELDS.filter((field) => numberOrNull(form[field.key]) != null).length)

// 与后端 BillingMode 一致；目录条目的计费模式只能是这四种。
const billingModes = ['token', 'per_request', 'image', 'video'] as const

const mediaTiers = ref<MediaTierForm[]>([])
const isMediaMode = computed(() => form.billing_mode === 'image' || form.billing_mode === 'video')
const mediaTierLabels = computed<readonly string[]>(() => (form.billing_mode === 'video' ? VIDEO_TIER_LABELS : IMAGE_TIER_LABELS))
const availableTierLabels = computed(() => mediaTierLabels.value.filter((label) => !mediaTiers.value.some((tier) => tier.tier_label === label)))
const perRequestPriceLabelKey = computed(() => {
  if (form.billing_mode === 'image') return 'perImagePrice'
  if (form.billing_mode === 'video') return 'perSecondPrice'
  return 'perRequestPrice'
})
const title = computed(() => (editingId.value ? t('admin.modelCatalog.edit') : t('admin.modelCatalog.create')))

/** 单价是 $/token，编辑时给一行「= $x / 百万 Token」的换算，免得数零 */
function perMillionHint(value: number | null | undefined): string {
  const n = numberOrNull(value)
  if (n == null) return ''
  return t('admin.modelCatalog.editor.perMillion', { price: Number((n * 1_000_000).toFixed(4)) })
}

function addMediaTier() {
  const label = availableTierLabels.value[0]
  if (!label) return
  mediaTiers.value.push({ tier_label: label, per_request_price: null })
}

function removeMediaTier(index: number) {
  mediaTiers.value.splice(index, 1)
}

function showApiError(error: unknown) {
  appStore.showError(extractApiErrorMessage(error, t('common.unknownError')))
}

async function loadFor(entry: ModelCatalogEntry | null) {
  bindingsEditorRef.value?.reset()
  bindings.value = []
  showMorePrices.value = false
  if (!entry) {
    editingId.value = null
    loadedEntry.value = null
    Object.assign(form, emptyForm())
    mediaTiers.value = []
    return
  }
  editingId.value = entry.id
  loadedEntry.value = entry
  Object.assign(form, emptyForm(), entryToRequest(entry))
  mediaTiers.value = mediaTiersFromIntervals(entry.intervals)
  try {
    bindings.value = await adminAPI.modelCatalog.getBindings(entry.id)
  } catch (error) {
    showApiError(error)
  }
}

// 每次打开按传入的条目重置（同一条目重复打开也重新拉绑定，避免显示上一次没保存的工作副本）
watch(
  () => [props.show, props.entry] as const,
  ([show, entry]) => {
    if (show) void loadFor(entry)
  },
  { immediate: true }
)

function payload(): ModelCatalogEntryRequest {
  const body: ModelCatalogEntryRequest = {
    ...form,
    // 图片 / 视频模式的分档由本页编辑；其余模式的区间分档按原值写回。
    intervals: isMediaMode.value ? mediaTiersToIntervals(mediaTiers.value) : (loadedEntry.value?.intervals ?? form.intervals ?? []),
    time_pricing: loadedEntry.value?.time_pricing ?? form.time_pricing ?? null
  }
  for (const field of NUMERIC_FIELDS) {
    body[field] = numberOrNull(form[field])
  }
  return applyOptionalPrices(body, form)
}

function bindingsPayload() {
  return bindings.value.map((binding) => ({
    account_id: binding.account_id,
    priority: numberOrNull(binding.priority)
  }))
}

async function save() {
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
    emit('saved')
  } catch (error) {
    showApiError(error)
  } finally {
    saving.value = false
  }
}
</script>
