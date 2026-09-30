<template>
  <!--
    目录条目表单（muqian 2026-09-25 改成独立页 /model-catalog/new、/model-catalog/:id/edit，分区导航在左）：
    基本（模型 ID 输入后按价格文件自动带出厂商 / 计费 / 价格）/ 价格（按每百万 Token 填）/ 承接的渠道（直接勾选）。
    按 Token 计费的「按 Token 分段」（muqian 2026-09-29）：上面的价格就是第一段，这里只加「超过 N Token」之后的各段。
    保存是整条覆盖：表单从条目整条投影（entryToRequest），没露出的字段（Fast 档价、倍率、分时…）原样写回。
    保存顺序：先存条目（新建时拿到 ID），再整份覆盖绑定；绑定被拒时条目已保存，弹出后端原因、表单保持打开。
  -->
  <FormPageShell :show="true" :title="title">
    <form id="model-catalog-form" class="space-y-5" @submit.prevent="save">
      <FormSectionHeading section="basics" :title="t('admin.modelCatalog.editor.basics')" />

      <div>
        <label class="input-label">{{ t('admin.modelCatalog.fields.modelId') }}</label>
        <div class="flex gap-2">
          <input
            v-model="form.model_id"
            class="input flex-1 font-mono"
            required
            autocomplete="off"
            :placeholder="t('admin.modelCatalog.editor.modelIdPlaceholder')"
            data-testid="model-catalog-model-id"
          />
          <button
            v-if="editingId"
            type="button"
            class="btn btn-secondary shrink-0"
            :disabled="lookup.state === 'loading' || !form.model_id.trim()"
            data-testid="model-catalog-price-lookup"
            @click="lookupPrice(true)"
          >
            {{ t('admin.modelCatalog.editor.lookup.refill') }}
          </button>
        </div>
        <p class="input-hint" data-testid="model-catalog-lookup-status">
          {{ lookupHint }}
          <button
            v-if="lookup.state === 'found' && !lookup.applied"
            type="button"
            class="ml-1 text-af-brand hover:text-af-brand-hover"
            data-testid="model-catalog-lookup-apply"
            @click="lookup.entry && applyLookup(lookup.entry)"
          >
            {{ t('admin.modelCatalog.editor.lookup.apply') }}
          </button>
        </p>
      </div>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <label class="input-label">{{ t('admin.modelCatalog.fields.displayName') }}</label>
          <input v-model="form.display_name" class="input" :placeholder="form.model_id" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.modelCatalog.fields.vendor') }}</label>
          <select v-model="vendorChoice" class="input" data-testid="model-catalog-vendor">
            <option value="">{{ t('admin.modelCatalog.editor.vendorNone') }}</option>
            <option v-for="vendor in vendorChoices" :key="vendor" :value="vendor">{{ vendor }}</option>
            <option :value="CUSTOM_VENDOR">{{ t('admin.modelCatalog.editor.vendorCustom') }}</option>
          </select>
          <input
            v-if="customVendor"
            v-model="form.vendor"
            class="input mt-2"
            :placeholder="t('admin.modelCatalog.editor.vendorCustomPlaceholder')"
            data-testid="model-catalog-vendor-custom"
          />
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
          <p class="input-hint">{{ t('admin.modelCatalog.listedRequiresPrice') }}</p>
        </div>
      </div>

      <FormSectionHeading section="pricing" :title="t('admin.modelCatalog.editor.pricing')" />

      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <label class="input-label">{{ t('admin.modelCatalog.fields.inputPrice') }}</label>
          <PriceInput v-model="form.input_price" :scale="PER_MILLION" :unit="perMillionUnit" test-id="model-catalog-input-price" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.modelCatalog.fields.outputPrice') }}</label>
          <PriceInput v-model="form.output_price" :scale="PER_MILLION" :unit="perMillionUnit" test-id="model-catalog-output-price" />
        </div>
      </div>
      <div v-if="form.billing_mode === 'token'" class="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div v-for="field in CACHE_PRICE_FIELDS" :key="field.key">
          <label class="input-label">{{ t(`admin.modelCatalog.fields.${field.label}`) }}</label>
          <PriceInput v-model="form[field.key]" :scale="PER_MILLION" :unit="perMillionUnit" :test-id="`model-catalog-${field.testId}`" />
        </div>
      </div>
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div v-if="isMediaMode || form.billing_mode === 'per_request'">
          <label class="input-label">{{ t(`admin.modelCatalog.fields.${perRequestPriceLabelKey}`) }}</label>
          <PriceInput v-model="form.per_request_price" :unit="perRequestUnit" test-id="model-catalog-per-request-price" />
        </div>
        <div v-else>
          <label class="input-label">{{ t('admin.modelCatalog.fields.searchPricePerCall') }}</label>
          <PriceInput v-model="form.search_price_per_call" :unit="t('admin.modelCatalog.editor.units.perCall')" test-id="model-catalog-search-price-per-call" />
        </div>
      </div>

      <!-- 更多价格：图片 / 音频的 Token 单价（按 Token 以外的计费还有缓存价），默认收起；收起时也按原值写回 -->
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
            <div v-for="field in morePriceFields" :key="field.key">
              <label class="input-label">{{ t(`admin.modelCatalog.fields.${field.label}`) }}</label>
              <PriceInput v-model="form[field.key]" :scale="PER_MILLION" :unit="perMillionUnit" :test-id="`model-catalog-${field.testId}`" />
            </div>
          </div>
        </template>
      </div>

      <!-- 按 Token 分段：上面的价格是第一段；每段「超过多少 Token」+ 这一段的官方价，没填的价按第一段算 -->
      <div v-if="form.billing_mode === 'token'" class="space-y-3" data-testid="model-catalog-token-segments">
        <div class="flex items-start justify-between gap-4">
          <div>
            <div class="text-sm font-medium text-af-ink">{{ t('admin.modelCatalog.segments.title') }}</div>
            <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.modelCatalog.segments.hint') }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm shrink-0" data-testid="model-catalog-token-segment-add" @click="addTokenSegment">
            {{ t('admin.modelCatalog.segments.add') }}
          </button>
        </div>
        <p v-if="tokenSegmentRows.length === 0" class="text-xs text-af-ink-3">{{ t('admin.modelCatalog.segments.empty') }}</p>
        <template v-else>
          <p class="text-13 text-af-ink-2" data-testid="model-catalog-token-segment-first">{{ firstSegmentText }}</p>
          <div
            v-for="(row, index) in tokenSegmentRows"
            :key="index"
            class="space-y-3 border-t border-af-hairline pt-3"
            data-testid="model-catalog-token-segment-row"
          >
            <div class="flex items-center justify-between gap-4">
              <span class="text-13 font-medium text-af-ink">{{ segmentTitle(index) }}</span>
              <button
                type="button"
                class="text-13 text-af-ink-3 transition-colors hover:text-af-ink"
                data-testid="model-catalog-token-segment-remove"
                @click="removeTokenSegment(index)"
              >
                {{ t('admin.modelCatalog.segments.remove') }}
              </button>
            </div>
            <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
              <div>
                <label class="input-label">{{ t('admin.modelCatalog.segments.above') }}</label>
                <div class="relative">
                  <input
                    v-model="row.above"
                    type="text"
                    inputmode="numeric"
                    autocomplete="off"
                    :placeholder="t('admin.modelCatalog.segments.abovePlaceholder')"
                    :class="['input pr-16 tabular-nums', thresholdInvalid(index) ? 'border-af-danger' : '']"
                    data-testid="model-catalog-token-segment-above"
                  />
                  <span class="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-af-ink-3">Token</span>
                </div>
              </div>
              <div v-for="field in SEGMENT_MAIN_PRICE_FIELDS" :key="field.key">
                <label class="input-label">{{ t(`admin.modelCatalog.fields.${field.label}`) }}</label>
                <PriceInput v-model="row[field.key]" :scale="PER_MILLION" :unit="perMillionUnit" :test-id="`model-catalog-token-segment-${field.testId}`" />
              </div>
            </div>
            <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
              <div v-for="field in SEGMENT_CACHE_PRICE_FIELDS" :key="field.key">
                <label class="input-label">{{ t(`admin.modelCatalog.fields.${field.label}`) }}</label>
                <PriceInput v-model="row[field.key]" :scale="PER_MILLION" :unit="perMillionUnit" :test-id="`model-catalog-token-segment-${field.testId}`" />
              </div>
            </div>
            <p v-if="visibleSegmentErrors[index]" class="text-xs text-af-danger" data-testid="model-catalog-token-segment-error">
              {{ t(`admin.modelCatalog.segments.errors.${visibleSegmentErrors[index]}`) }}
            </p>
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
            <PriceInput v-model="tier.per_request_price" :unit="perRequestUnit" test-id="model-catalog-media-tier-price" />
          </div>
          <button type="button" class="btn btn-secondary btn-sm" data-testid="model-catalog-media-tier-remove" @click="removeMediaTier(index)">
            {{ t('admin.modelCatalog.tiers.remove') }}
          </button>
        </div>
      </div>
      <p class="text-xs text-af-ink-3">{{ t('admin.modelCatalog.fullReplaceHint') }}</p>

      <FormSectionHeading section="channels" :title="t('admin.modelCatalog.editor.channels')" />
      <CatalogChannelPicker v-model="bindings" :entry-id="editingId" />
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="model-catalog-form" class="btn btn-primary" :disabled="saving" data-testid="model-catalog-save">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </FormPageShell>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ModelCatalogBinding, ModelCatalogEntry, ModelCatalogEntryRequest } from '@/api/admin/modelCatalog'
import FormPageShell from '@/components/admin/form/FormPageShell.vue'
import FormSectionHeading from '@/components/admin/form/FormSectionHeading.vue'
import Icon from '@/components/icons/Icon.vue'
import CatalogChannelPicker from './CatalogChannelPicker.vue'
import PriceInput from './PriceInput.vue'
import {
  IMAGE_TIER_LABELS,
  NUMERIC_FIELDS,
  OPTIONAL_NUMERIC_FIELDS,
  VIDEO_TIER_LABELS,
  applyOptionalPrices,
  entryToRequest,
  mediaTiersFromIntervals,
  mediaTiersToIntervals,
  numberOrNull,
  parseTokenThreshold,
  tokenSegmentErrors,
  tokenSegmentsFromIntervals,
  tokenSegmentsToIntervals,
  type MediaTierForm,
  type TokenSegmentError,
  type TokenSegmentForm
} from './entryRequest'
import { formatSegmentRange } from '@/utils/tokenSegments'

const props = defineProps<{
  /** null = 新建 */
  entry: ModelCatalogEntry | null
  /** 目录里已有的厂商标签，给厂商下拉做选项 */
  vendorOptions: string[]
}>()
const emit = defineEmits<{ close: []; saved: [] }>()

const { t } = useI18n()

const PER_MILLION = 1_000_000
const perMillionUnit = computed(() => t('admin.modelCatalog.editor.units.perMillion'))

const saving = ref(false)
const editingId = ref<number | null>(null)
const loadedEntry = ref<ModelCatalogEntry | null>(null)
const bindings = ref<ModelCatalogBinding[]>([])

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
  notes: null,
  intervals: [],
  time_pricing: null
})
const form = reactive<ModelCatalogEntryRequest>(emptyForm())

type TokenPriceKey =
  | 'cache_write_price'
  | 'cache_write_1h_price'
  | 'cache_read_price'
  | 'image_input_price'
  | 'image_output_price'
  | 'image_cache_read_price'
  | 'audio_input_price'
  | 'audio_output_price'
interface PriceField {
  key: TokenPriceKey
  label: string
  testId: string
}
// 缓存价：按 Token 计费时直接露出；其它计费模式收进「更多价格」
const CACHE_PRICE_FIELDS: readonly PriceField[] = [
  { key: 'cache_read_price', label: 'cacheReadPrice', testId: 'cache-read-price' },
  { key: 'cache_write_price', label: 'cacheWritePrice', testId: 'cache-write-price' },
  { key: 'cache_write_1h_price', label: 'cacheWrite1hPrice', testId: 'cache-write-1h-price' }
]
const MEDIA_TOKEN_PRICE_FIELDS: readonly PriceField[] = [
  { key: 'image_input_price', label: 'imageInputPrice', testId: 'image-input-price' },
  { key: 'image_output_price', label: 'imageOutputPrice', testId: 'image-output-price' },
  { key: 'image_cache_read_price', label: 'imageCacheReadPrice', testId: 'image-cache-read-price' },
  { key: 'audio_input_price', label: 'audioInputPrice', testId: 'audio-input-price' },
  { key: 'audio_output_price', label: 'audioOutputPrice', testId: 'audio-output-price' }
]
const morePriceFields = computed(() =>
  form.billing_mode === 'token' ? MEDIA_TOKEN_PRICE_FIELDS : [...CACHE_PRICE_FIELDS, ...MEDIA_TOKEN_PRICE_FIELDS]
)
const showMorePrices = ref(false)
const morePricesFilled = computed(() => morePriceFields.value.filter((field) => numberOrNull(form[field.key]) != null).length)

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
const perRequestUnit = computed(() => {
  if (form.billing_mode === 'image') return t('admin.modelCatalog.editor.units.perImage')
  if (form.billing_mode === 'video') return t('admin.modelCatalog.editor.units.perSecond')
  return t('admin.modelCatalog.editor.units.perCall')
})
const title = computed(() => (editingId.value ? t('admin.modelCatalog.edit') : t('admin.modelCatalog.create')))

// ── 厂商：目录已有的厂商做下拉，另可自定义 ──
const CUSTOM_VENDOR = '__custom__'
const customVendor = ref(false)
const vendorChoices = computed(() =>
  [...new Set([...props.vendorOptions, ...(customVendor.value || !form.vendor ? [] : [form.vendor])])].sort()
)
const vendorChoice = computed({
  get: () => (customVendor.value ? CUSTOM_VENDOR : form.vendor ?? ''),
  set: (value: string) => {
    if (value === CUSTOM_VENDOR) {
      customVendor.value = true
      form.vendor = ''
      return
    }
    customVendor.value = false
    form.vendor = value
  }
})

// ── 按模型 ID 从价格文件带价（新建时输入即查；编辑时点按钮） ──
type LookupState = 'idle' | 'loading' | 'found' | 'missing' | 'error'
const lookup = reactive({
  state: 'idle' as LookupState,
  entry: null as ModelCatalogEntry | null,
  applied: false
})
let lookupSeq = 0
let lookupTimer: ReturnType<typeof setTimeout> | null = null

// 带价会改的字段；管理员没动过（与上次带出的值一致，或还全是空）才自动覆盖
function autofillSnapshot(): string {
  return JSON.stringify({
    vendor: form.vendor,
    billing_mode: form.billing_mode,
    prices: [...NUMERIC_FIELDS, ...OPTIONAL_NUMERIC_FIELDS].map((field) => numberOrNull(form[field])),
    tiers: mediaTiers.value,
    segments: tokenSegmentRows.value
  })
}
let lastAutofill: string | null = null
function formUntouched(): boolean {
  if (lastAutofill !== null) return autofillSnapshot() === lastAutofill
  const pricesEmpty = [...NUMERIC_FIELDS, ...OPTIONAL_NUMERIC_FIELDS].every((field) => numberOrNull(form[field]) == null)
  return !form.vendor && pricesEmpty && mediaTiers.value.length === 0 && tokenSegmentRows.value.length === 0
}

function applyLookup(entry: ModelCatalogEntry) {
  const found = entryToRequest(entry)
  customVendor.value = false
  form.vendor = found.vendor ?? ''
  form.billing_mode = found.billing_mode ?? 'token'
  form.protocols = found.protocols ?? []
  if (!form.display_name?.trim()) form.display_name = found.display_name ?? ''
  for (const field of NUMERIC_FIELDS) form[field] = numberOrNull(found[field])
  for (const field of OPTIONAL_NUMERIC_FIELDS) form[field] = numberOrNull(found[field])
  mediaTiers.value = mediaTiersFromIntervals(found.intervals)
  tokenSegmentRows.value = (found.billing_mode || 'token') === 'token' ? tokenSegmentsFromIntervals(found.intervals, found) : []
  segmentsSubmitted.value = false
  if (!loadedEntry.value) {
    // 新建：按次模式的分档与分时随带价一起写入（编辑时这两项按原值写回，见 payload）
    form.intervals = found.intervals ?? []
    form.time_pricing = found.time_pricing ?? null
  }
  lastAutofill = autofillSnapshot()
  lookup.applied = true
}

async function lookupPrice(force: boolean) {
  const modelId = form.model_id.trim()
  const seq = ++lookupSeq
  if (!modelId) {
    lookup.state = 'idle'
    return
  }
  lookup.state = 'loading'
  try {
    const entry = await adminAPI.modelCatalog.priceLookup(modelId)
    if (seq !== lookupSeq) return
    lookup.entry = entry
    lookup.applied = false
    if (!entry) {
      lookup.state = 'missing'
      return
    }
    lookup.state = 'found'
    if (force || formUntouched()) applyLookup(entry)
  } catch {
    if (seq === lookupSeq) lookup.state = 'error'
  }
}

watch(
  () => form.model_id,
  () => {
    if (editingId.value) return
    if (lookupTimer) clearTimeout(lookupTimer)
    lookupTimer = setTimeout(() => {
      lookupTimer = null
      void lookupPrice(false)
    }, 400)
  }
)

const lookupHint = computed(() => {
  switch (lookup.state) {
    case 'loading':
      return t('admin.modelCatalog.editor.lookup.loading')
    case 'found':
      return lookup.applied ? t('admin.modelCatalog.editor.lookup.applied') : t('admin.modelCatalog.editor.lookup.found')
    case 'missing':
      return t('admin.modelCatalog.editor.lookup.missing')
    case 'error':
      return t('admin.modelCatalog.editor.lookup.error')
    default:
      return editingId.value ? t('admin.modelCatalog.editor.lookup.editIdle') : t('admin.modelCatalog.editor.lookup.idle')
  }
})

// ── 按 Token 分段（只在按 Token 计费时编辑和保存） ──
type SegmentPriceKey = 'input_price' | 'output_price' | 'cache_read_price' | 'cache_write_price' | 'cache_write_1h_price'
interface SegmentPriceField {
  key: SegmentPriceKey
  label: string
  testId: string
}
const SEGMENT_MAIN_PRICE_FIELDS: readonly SegmentPriceField[] = [
  { key: 'input_price', label: 'inputPrice', testId: 'input-price' },
  { key: 'output_price', label: 'outputPrice', testId: 'output-price' }
]
// 与上面第一段的缓存价同序同名
const SEGMENT_CACHE_PRICE_FIELDS: readonly SegmentPriceField[] = [
  { key: 'cache_read_price', label: 'cacheReadPrice', testId: 'cache-read-price' },
  { key: 'cache_write_price', label: 'cacheWritePrice', testId: 'cache-write-price' },
  { key: 'cache_write_1h_price', label: 'cacheWrite1hPrice', testId: 'cache-write-1h-price' }
]

const tokenSegmentRows = ref<TokenSegmentForm[]>([])
const segmentErrors = computed(() => tokenSegmentErrors(tokenSegmentRows.value))
// 「没填 Token 数」「没填价」点保存时才报，别让刚加的空行一出来就标红；Token 数写错随输随报
const segmentsSubmitted = ref(false)
const visibleSegmentErrors = computed(() =>
  segmentErrors.value.map((error) => ((error === 'required' || error === 'noPrice') && !segmentsSubmitted.value ? null : error))
)

function isThresholdError(error: TokenSegmentError | null | undefined): boolean {
  return error === 'required' || error === 'integer' || error === 'notAscending'
}

function thresholdInvalid(index: number): boolean {
  return isThresholdError(visibleSegmentErrors.value[index])
}

/** 第 index 行填对了的 Token 数；空、不是正整数、不比上一行大都算没填对 */
function validThreshold(index: number): number | null {
  if (isThresholdError(segmentErrors.value[index])) return null
  return parseTokenThreshold(tokenSegmentRows.value[index]?.above ?? '')
}

/** 第 index 行的范围（>272K、32K–128K）；这一行或下一行的 Token 数还没填对时为 null */
function segmentRange(index: number): string | null {
  const min = validThreshold(index)
  if (min == null) return null
  if (index === tokenSegmentRows.value.length - 1) return formatSegmentRange({ min, max: null })
  const max = validThreshold(index + 1)
  return max == null ? null : formatSegmentRange({ min, max })
}

const firstSegmentText = computed(() => {
  const first = validThreshold(0)
  return first == null
    ? t('admin.modelCatalog.segments.firstUntitled')
    : t('admin.modelCatalog.segments.first', { range: formatSegmentRange({ min: 0, max: first }) })
})

function segmentTitle(index: number): string {
  const range = segmentRange(index)
  return range == null
    ? t('admin.modelCatalog.segments.segmentUntitled', { index: index + 2 })
    : t('admin.modelCatalog.segments.segment', { index: index + 2, range })
}

function addTokenSegment() {
  tokenSegmentRows.value.push({
    above: '',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_write_1h_price: null,
    cache_read_price: null
  })
}

function removeTokenSegment(index: number) {
  tokenSegmentRows.value.splice(index, 1)
}

function addMediaTier() {
  const label = availableTierLabels.value[0]
  if (!label) return
  mediaTiers.value.push({ tier_label: label, per_request_price: null })
}

function removeMediaTier(index: number) {
  mediaTiers.value.splice(index, 1)
}

function logApiError(error: unknown) {
  console.error(extractApiErrorMessage(error, t('common.unknownError')), error)
}

async function loadFor(entry: ModelCatalogEntry | null) {
  bindings.value = []
  showMorePrices.value = false
  customVendor.value = false
  lastAutofill = null
  lookup.state = 'idle'
  lookup.entry = null
  lookup.applied = false
  if (!entry) {
    editingId.value = null
    loadedEntry.value = null
    Object.assign(form, emptyForm())
    mediaTiers.value = []
    tokenSegmentRows.value = []
    segmentsSubmitted.value = false
    return
  }
  editingId.value = entry.id
  loadedEntry.value = entry
  Object.assign(form, emptyForm(), entryToRequest(entry))
  mediaTiers.value = mediaTiersFromIntervals(entry.intervals)
  // 只有按 Token 计费的区间是分段；按次模式按 Token 区间的分档另算，按原值写回
  tokenSegmentRows.value = (entry.billing_mode || 'token') === 'token' ? tokenSegmentsFromIntervals(entry.intervals, entry) : []
  segmentsSubmitted.value = false
  try {
    bindings.value = await adminAPI.modelCatalog.getBindings(entry.id)
  } catch (error) {
    logApiError(error)
  }
}

watch(
  () => props.entry,
  (entry) => {
    void loadFor(entry)
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  if (lookupTimer) clearTimeout(lookupTimer)
})

function payloadIntervals() {
  // 按 Token 的分段、图片 / 视频的分档由本页编辑；按次模式的分档按原值写回。
  if (form.billing_mode === 'token') return tokenSegmentsToIntervals(tokenSegmentRows.value)
  if (isMediaMode.value) return mediaTiersToIntervals(mediaTiers.value)
  return loadedEntry.value?.intervals ?? form.intervals ?? []
}

function payload(): ModelCatalogEntryRequest {
  const body: ModelCatalogEntryRequest = {
    ...form,
    intervals: payloadIntervals(),
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
  if (form.billing_mode === 'token' && segmentErrors.value.some((error) => error != null)) {
    segmentsSubmitted.value = true
    return
  }
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
    logApiError(error)
  } finally {
    saving.value = false
  }
}
</script>
