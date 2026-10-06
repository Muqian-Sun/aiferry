<template>
  <!--
    新建模型（muqian 2026-10-03 定两步，与新建渠道对称）：
    ① 模型：标识（输入即查价格文件，下面显示带出的官方价）、展示名、厂商 →「下一步」建条目（未上架、按 Token 计费）；
    ② 定价与渠道：就是价格页「按模型」那一块（同组件、同保存接口），改官方价、加渠道即承接；最后打开「上架」。
    渠道弹窗「检测上游」里目录没有的模型从这里加：initialModelId 预填标识。
  -->
  <!-- 第二步是价格页那一块（宽表格），弹窗加宽 -->
  <BaseDialog :show="show" :title="t('admin.modelCatalog.create')" :width="step === 2 ? 'extra-wide' : 'wide'" :z-index="zIndex" @close="handleClose">
    <ol class="mb-5 flex items-center gap-2 text-13" data-testid="model-create-steps">
      <li v-for="(label, index) in stepLabels" :key="label" class="flex items-center gap-2">
        <span v-if="index > 0" class="h-px w-6 bg-af-hairline-strong" aria-hidden="true"></span>
        <span
          :class="[
            'flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold',
            step >= index + 1 ? 'bg-af-ink text-af-on-brand' : 'bg-af-hairline text-af-ink-3'
          ]"
        >
          {{ index + 1 }}
        </span>
        <span :class="step === index + 1 ? 'font-medium text-af-ink' : 'text-af-ink-3'">{{ label }}</span>
      </li>
    </ol>

    <form v-if="step === 1" id="model-create-form" @submit.prevent="next">
      <ModelBasicsFields
        ref="basicsRef"
        v-model:model-id="form.model_id"
        v-model:display-name="form.display_name"
        v-model:vendor="form.vendor"
        :vendor-options="vendorOptions"
      >
        <template #model-id-hint>
          <p v-if="alreadyInCatalog" class="input-error-text" data-testid="model-create-exists">{{ t('admin.modelCatalog.dialog.exists') }}</p>
          <p v-else class="input-hint" data-testid="model-create-lookup">{{ lookupText }}</p>
        </template>
      </ModelBasicsFields>
    </form>

    <div v-else class="space-y-4">
      <p class="text-13 text-af-ink-3">{{ t('admin.modelCatalog.dialog.pricingHint') }}</p>
      <FormError v-if="pricingLoadError" :message="pricingLoadError" />
      <p v-else-if="!pricingEntry || !modelState" class="flex items-center gap-2 text-13 text-af-ink-3">
        <Icon name="refresh" size="sm" class="animate-spin" />
        {{ t('admin.modelCatalog.dialog.pricingLoading') }}
      </p>
      <template v-else>
        <PricingModelBlock
          :entry="pricingEntry"
          :state="modelState"
          :accounts="accountsById"
          :account-order="accountOrder"
          :default-sale-ratio="overview?.default_sale_ratio ?? 1"
          :min-margin="overview?.min_margin ?? 0"
          @saved="loadPricing"
        />
        <div class="flex items-center justify-between gap-4 rounded-lg border border-af-hairline px-4 py-3" data-testid="model-create-listing">
          <div class="min-w-0">
            <label class="input-label mb-0">{{ t('admin.modelCatalog.dialog.listing') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">{{ listingHint }}</p>
          </div>
          <Toggle
            :model-value="pricingEntry.status === 'listed'"
            :aria-label="t('admin.modelCatalog.dialog.listing')"
            :disabled="listingBusy || (pricingEntry.status !== 'listed' && listingBlocker !== '')"
            class="disabled:cursor-not-allowed disabled:opacity-50"
            data-testid="model-create-listing-toggle"
            @update:model-value="setListed"
          />
        </div>
      </template>
    </div>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-3">
        <FormError class="mr-auto min-w-0 flex-1" :message="submitError" />
        <template v-if="step === 1">
          <button type="button" class="btn btn-secondary" @click="handleClose">{{ t('common.cancel') }}</button>
          <button type="submit" form="model-create-form" class="btn btn-primary" :disabled="submitting || alreadyInCatalog" data-testid="model-create-next">
            <Icon v-if="submitting" name="refresh" size="sm" class="-ml-1 mr-2 animate-spin" />
            {{ t('admin.modelCatalog.dialog.next') }}
          </button>
        </template>
        <button v-else type="button" class="btn btn-primary" data-testid="model-create-done" @click="finish">
          {{ t('admin.modelCatalog.dialog.done') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogEntry, ModelCatalogEntryRequest } from '@/api/admin/modelCatalog'
import type { PricingOverview } from '@/api/admin/pricing'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import PricingModelBlock from '@/components/admin/pricing/PricingModelBlock.vue'
import {
  cloneModelDraft,
  modelDraftChanges,
  modelDraftFrom,
  type BlockState,
  type ModelDraft
} from '@/components/admin/pricing/pricingDraft'
import { extractApiErrorMessage } from '@/utils/apiError'
import ModelBasicsFields from './ModelBasicsFields.vue'
import { entryToRequest } from './entryRequest'
import type { CatalogVendorChoice } from './vendorLabel'
import { DETAIL_PRICE_MAX_DECIMALS, formatListPrice, perMillion, sharedPriceDecimals } from './priceFormat'
import { tokenIntervals } from '@/utils/tokenSegments'

const props = withDefaults(
  defineProps<{
    show: boolean
    /** 目录里已有的厂商（按展示名分组） */
    vendorOptions: CatalogVendorChoice[]
    /** 目录里已有的模型标识：输入撞上时直接提示，不必等后端拒 */
    existingModelIds?: string[]
    /** 预填的模型标识（从渠道「检测上游」里目录没有的模型进来） */
    initialModelId?: string
    /** 叠在别的弹窗上面时传更高的层级 */
    zIndex?: number
  }>(),
  { existingModelIds: () => [], initialModelId: '', zIndex: 50 }
)

const emit = defineEmits<{
  close: []
  /** 目录或价格有变化（建了条目、保存了定价、改了上架）：调用方据此重拉列表 */
  saved: [entry: ModelCatalogEntry]
}>()

const { t } = useI18n()

const stepLabels = computed(() => [t('admin.modelCatalog.dialog.steps.model'), t('admin.modelCatalog.dialog.steps.pricing')])
const step = ref<1 | 2>(1)
const submitting = ref(false)
const submitError = ref('')
const basicsRef = ref<InstanceType<typeof ModelBasicsFields> | null>(null)

const form = reactive({ model_id: '', display_name: '', vendor: '' })

// 模型标识唯一（不分大小写，与后端唯一索引 lower(model_id) 一致）
const existingIds = computed(() => new Set(props.existingModelIds.map((id) => id.toLowerCase())))
const alreadyInCatalog = computed(() => existingIds.value.has(form.model_id.trim().toLowerCase()))

// ---- 按模型标识查价格文件（输入停 400ms 查一次；直接点「下一步」时补查）。只认按 Token 计费的价
type LookupState = 'idle' | 'loading' | 'found' | 'missing' | 'error'
const lookup = reactive({ state: 'idle' as LookupState, modelId: '', entry: null as ModelCatalogEntry | null })
let lookupSeq = 0
let lookupTimer: ReturnType<typeof setTimeout> | null = null
// 上一次自动带出的展示名 / 厂商：管理员没改过就随新的查价结果换，改过就不动
const autoFilled = { display_name: '', vendor: '' }

function clearLookupTimer() {
  if (lookupTimer) clearTimeout(lookupTimer)
  lookupTimer = null
}

function applyAutoFill(entry: ModelCatalogEntry | null) {
  const displayName = entry?.display_name ?? ''
  const vendor = entry?.vendor ?? ''
  if (form.display_name === autoFilled.display_name) form.display_name = displayName
  if (form.vendor === autoFilled.vendor) {
    form.vendor = vendor
    basicsRef.value?.resetCustomVendor()
  }
  autoFilled.display_name = displayName
  autoFilled.vendor = vendor
}

async function runLookup(modelId: string) {
  clearLookupTimer()
  const seq = ++lookupSeq
  if (!modelId) {
    Object.assign(lookup, { state: 'idle', modelId: '', entry: null })
    applyAutoFill(null)
    return
  }
  lookup.state = 'loading'
  try {
    const found = await adminAPI.modelCatalog.priceLookup(modelId)
    if (seq !== lookupSeq) return
    const tokenEntry = found && (found.billing_mode || 'token') === 'token' ? found : null
    Object.assign(lookup, { state: tokenEntry ? 'found' : 'missing', modelId, entry: tokenEntry })
    applyAutoFill(tokenEntry)
  } catch {
    if (seq === lookupSeq) Object.assign(lookup, { state: 'error', modelId, entry: null })
  }
}

watch(
  () => form.model_id,
  (value) => {
    if (step.value !== 1) return
    clearLookupTimer()
    lookupTimer = setTimeout(() => void runLookup(value.trim()), 400)
  }
)

const lookupText = computed(() => {
  switch (lookup.state) {
    case 'loading':
      return t('admin.modelCatalog.editor.lookup.loading')
    case 'found':
      return t('admin.modelCatalog.dialog.lookup.found', { prices: lookupPricesText.value })
    case 'missing':
      return t('admin.modelCatalog.dialog.lookup.missing')
    case 'error':
      return t('admin.modelCatalog.dialog.lookup.error')
    default:
      return t('admin.modelCatalog.dialog.lookup.idle')
  }
})

const lookupPricesText = computed(() => {
  const entry = lookup.entry
  if (!entry) return ''
  const items: Array<[string, number | null]> = [
    [t('admin.modelCatalog.drawer.price.input'), perMillion(entry.input_price)],
    [t('admin.modelCatalog.drawer.price.output'), perMillion(entry.output_price)],
    [t('admin.modelCatalog.drawer.price.cacheRead'), perMillion(entry.cache_read_price)],
    [t('admin.modelCatalog.drawer.price.cacheWrite'), perMillion(entry.cache_write_price)],
    [t('admin.modelCatalog.drawer.price.cacheWrite1h'), perMillion(entry.cache_write_1h_price)]
  ]
  const present = items.filter(([, value]) => value != null)
  const decimals = sharedPriceDecimals(present.map(([, value]) => value), DETAIL_PRICE_MAX_DECIMALS)
  const parts = present.map(([label, value]) => `${label} ${formatListPrice(value, decimals)}`)
  const segments = tokenIntervals(entry.intervals).length
  if (segments > 0) parts.push(t('admin.modelCatalog.columns.segments', { count: segments + 1 }))
  return parts.join(' · ')
})

// ---- 第一步「下一步」：建条目（未上架、按 Token 计费；查到价就把官方价、分段、搜索价一起写进去）
const created = ref<ModelCatalogEntry | null>(null)

function createRequest(modelId: string): ModelCatalogEntryRequest {
  const base: ModelCatalogEntryRequest = lookup.entry
    ? entryToRequest(lookup.entry)
    : { model_id: modelId, protocols: [], intervals: [], time_pricing: null }
  return {
    ...base,
    model_id: modelId,
    display_name: form.display_name.trim(),
    vendor: form.vendor.trim(),
    billing_mode: 'token',
    status: 'unlisted'
  }
}

async function next() {
  const modelId = form.model_id.trim()
  if (!modelId || alreadyInCatalog.value) return
  submitError.value = ''
  submitting.value = true
  try {
    if (lookup.modelId !== modelId || lookup.state === 'loading') await runLookup(modelId)
    const entry = await adminAPI.modelCatalog.createEntry(createRequest(modelId))
    created.value = entry
    emit('saved', entry)
    step.value = 2
    await loadPricing()
  } catch (error) {
    submitError.value = extractApiErrorMessage(error, t('admin.modelCatalog.dialog.createFailed'), {
      MODEL_CATALOG_ENTRY_EXISTS: t('admin.modelCatalog.dialog.exists')
    })
  } finally {
    submitting.value = false
  }
}

// ---- 第二步：价格页「按模型」那一块
const overview = ref<PricingOverview | null>(null)
const modelState = ref<BlockState<ModelDraft> | null>(null)
const pricingLoadError = ref('')

const pricingEntry = computed(() => overview.value?.entries.find((entry) => entry.id === created.value?.id) ?? null)
const accountsById = computed(() => new Map((overview.value?.accounts ?? []).map((account) => [account.id, account])))

/** 渠道排序与价格页一致：渠道优先级（越小越先用），再按 ID */
function accountOrder(accountId: number): number {
  const priority = accountsById.value.get(accountId)?.priority ?? 1e6
  return priority * 1e9 + accountId
}

async function loadPricing() {
  pricingLoadError.value = ''
  try {
    const data = await adminAPI.pricing.overview()
    overview.value = data
    const entry = data.entries.find((item) => item.id === created.value?.id)
    if (!entry) {
      pricingLoadError.value = t('admin.modelCatalog.dialog.pricingMissing')
      return
    }
    // 改到一半的草稿保留；没改过（含刚保存完）的按服务端数据重建，带上后端算的毛利
    if (!modelState.value || modelDraftChanges(modelState.value) === 0) {
      const initial = modelDraftFrom(entry, accountOrder)
      modelState.value = { initial, draft: cloneModelDraft(initial) }
    }
    if (created.value) emit('saved', created.value)
  } catch (error) {
    pricingLoadError.value = extractApiErrorMessage(error, t('admin.pricing.loadFailed'))
  }
}

const hasUnsavedPricing = computed(() => modelState.value != null && modelDraftChanges(modelState.value) > 0)
// 「还有没保存的改动」只在改动还在时有意义：保存或撤销之后清掉
watch(hasUnsavedPricing, (unsaved) => {
  if (!unsaved && submitError.value === t('admin.modelCatalog.dialog.unsavedPricing')) submitError.value = ''
})

/** 不能上架的原因（空串 = 可以上架）：按服务端已保存的数据判断 */
const listingBlocker = computed(() => {
  const entry = pricingEntry.value
  if (!entry) return ''
  if (hasUnsavedPricing.value) return t('admin.modelCatalog.dialog.listingBlocked.unsaved')
  if (entry.input_price == null || entry.output_price == null) return t('admin.modelCatalog.dialog.listingBlocked.price')
  if (entry.bindings.length === 0) return t('admin.modelCatalog.dialog.listingBlocked.channel')
  return ''
})

const listingHint = computed(() => {
  if (pricingEntry.value?.status === 'listed') return t('admin.modelCatalog.dialog.listedHint')
  return listingBlocker.value || t('admin.modelCatalog.dialog.listingReady')
})

const listingBusy = ref(false)

async function setListed(listed: boolean) {
  const id = created.value?.id
  if (!id) return
  submitError.value = ''
  listingBusy.value = true
  try {
    // 整条覆盖：先拿服务端最新的条目（官方价刚在上面那一块改过），只换状态
    const latest = await adminAPI.modelCatalog.getEntry(id)
    const updated = await adminAPI.modelCatalog.updateEntry(id, { ...entryToRequest(latest), status: listed ? 'listed' : 'unlisted' })
    created.value = updated
    await loadPricing()
  } catch (error) {
    submitError.value = extractApiErrorMessage(error, t('admin.modelCatalog.dialog.listingFailed'))
  } finally {
    listingBusy.value = false
  }
}

function finish() {
  handleClose()
}

// ---- 开关弹窗
function reset() {
  clearLookupTimer()
  lookupSeq++
  step.value = 1
  submitting.value = false
  submitError.value = ''
  Object.assign(form, { model_id: '', display_name: '', vendor: '' })
  Object.assign(lookup, { state: 'idle', modelId: '', entry: null })
  autoFilled.display_name = ''
  autoFilled.vendor = ''
  basicsRef.value?.resetCustomVendor()
  created.value = null
  overview.value = null
  modelState.value = null
  pricingLoadError.value = ''
  listingBusy.value = false
}

watch(
  () => props.show,
  (show) => {
    reset()
    if (show && props.initialModelId) form.model_id = props.initialModelId
  },
  { immediate: true }
)

// 定价那一块改了没保存时不关（右上角关闭也一样）：先保存或点那一块的「撤销」
function handleClose() {
  if (hasUnsavedPricing.value) {
    submitError.value = t('admin.modelCatalog.dialog.unsavedPricing')
    return
  }
  emit('close')
}

onBeforeUnmount(clearLookupTimer)
</script>
