<template>
  <BaseDialog :show="show" :title="plan ? t('payment.admin.editPlan') : t('payment.admin.createPlan')" width="wide" @close="emit('close')">
    <form id="plan-form" @submit.prevent="handleSavePlan" class="space-y-4">
      <div>
        <label class="input-label">{{ t('payment.admin.planName') }} <span class="text-red-500">*</span></label>
        <input v-model="planForm.name" type="text" class="input" required />
      </div>

      <div><label class="input-label">{{ t('payment.admin.planDescription') }} <span class="text-red-500">*</span></label><textarea v-model="planForm.description" rows="2" class="input" required></textarea></div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('payment.admin.price') }} <span class="text-red-500">*</span></label>
          <input v-model.number="planForm.price" type="number" step="0.01" min="0.01" class="input" required />
          <p v-if="subscriptionCnyPreview" class="mt-1 text-xs font-medium text-primary-600 dark:text-primary-400">
            {{ t('payment.admin.subscriptionCnyPayPreview', { amount: subscriptionCnyPreview.amount }) }}
            <span v-if="subscriptionCnyPreview.feeRate > 0">
              {{ t('payment.admin.subscriptionCnyPayPreviewWithFee', { feeRate: subscriptionCnyPreview.feeRate, total: subscriptionCnyPreview.total }) }}
            </span>
          </p>
        </div>
        <div><label class="input-label">{{ t('payment.admin.originalPrice') }}</label><input v-model.number="planForm.original_price" type="number" step="0.01" min="0" class="input" /></div>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div><label class="input-label">{{ t('payment.admin.validity') }} <span class="text-red-500">*</span></label><input v-model.number="planForm.validity_days" type="number" min="1" class="input" required /></div>
        <div><label class="input-label">{{ t('payment.admin.validityUnit') }} <span class="text-red-500">*</span></label><Select v-model="planForm.validity_unit" :options="validityUnitOptions" /></div>
      </div>
      <!-- 三档限额：空 = 不限 -->
      <div class="grid grid-cols-3 gap-4">
        <div>
          <label class="input-label">{{ t('payment.admin.dailyLimit') }}</label>
          <input v-model.number="planForm.daily_limit_usd" type="number" step="0.01" min="0" class="input" :placeholder="t('payment.admin.unlimited')" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.admin.weeklyLimit') }}</label>
          <input v-model.number="planForm.weekly_limit_usd" type="number" step="0.01" min="0" class="input" :placeholder="t('payment.admin.unlimited')" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.admin.monthlyLimit') }}</label>
          <input v-model.number="planForm.monthly_limit_usd" type="number" step="0.01" min="0" class="input" :placeholder="t('payment.admin.unlimited')" />
        </div>
      </div>

      <!-- 套餐模型集：只列目录里已上架的条目，至少选一个 -->
      <div>
        <label class="input-label">{{ t('payment.admin.models') }} <span class="text-red-500">*</span></label>
        <div class="relative">
          <div
            data-testid="plan-models-toggle"
            class="cursor-pointer rounded-lg border border-gray-300 bg-white px-3 py-2 dark:border-dark-500 dark:bg-dark-700"
            @click="showModelDropdown = !showModelDropdown"
          >
            <div v-if="selectedEntries.length" class="flex flex-wrap gap-1.5">
              <span
                v-for="entry in selectedEntries"
                :key="entry.id"
                class="inline-flex items-center gap-1 rounded bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-dark-600 dark:text-gray-300"
              >
                <span class="truncate">{{ entryLabel(entry) }}</span>
                <button type="button" class="shrink-0 rounded-full hover:bg-gray-200 dark:hover:bg-dark-500" @click.stop="toggleEntry(entry.id)">
                  <Icon name="x" size="xs" class="h-3.5 w-3.5" :stroke-width="2" />
                </button>
              </span>
            </div>
            <span v-else class="text-sm text-gray-400">{{ t('payment.admin.selectModels') }}</span>
          </div>
          <div
            v-if="showModelDropdown"
            class="absolute left-0 right-0 top-full z-50 mt-1 rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-700"
          >
            <div class="sticky top-0 border-b border-gray-200 bg-white p-2 dark:border-dark-600 dark:bg-dark-700">
              <input v-model="modelSearch" type="text" class="input w-full text-sm" :placeholder="t('admin.accounts.searchModels')" @click.stop />
            </div>
            <div class="max-h-52 overflow-auto">
              <button
                v-for="entry in filteredEntries"
                :key="entry.id"
                type="button"
                data-testid="plan-model-option"
                class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-600"
                @click="toggleEntry(entry.id)"
              >
                <span
                  :class="[
                    'flex h-4 w-4 shrink-0 items-center justify-center rounded border',
                    planForm.entry_ids.includes(entry.id) ? 'border-primary-500 bg-primary-500 text-white' : 'border-gray-300 dark:border-dark-500'
                  ]"
                >
                  <Icon v-if="planForm.entry_ids.includes(entry.id)" name="check" size="xs" class="h-3 w-3" :stroke-width="3" />
                </span>
                <span class="truncate text-gray-900 dark:text-white">{{ entryLabel(entry) }}</span>
              </button>
              <div v-if="filteredEntries.length === 0" class="px-3 py-4 text-center text-sm text-gray-500">
                {{ t('admin.accounts.noMatchingModels') }}
              </div>
            </div>
          </div>
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.modelsHint') }}</p>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div><label class="input-label">{{ t('payment.admin.sortOrder') }}</label><input v-model.number="planForm.sort_order" type="number" min="0" class="input" /></div>
        <div>
          <label class="input-label">{{ t('payment.admin.currency') }}</label>
          <input v-model="planForm.currency" type="text" maxlength="3" class="input uppercase" :placeholder="t('payment.admin.currencyPlaceholder')" />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.currencyHint') }}</p>
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('payment.admin.features') }}</label>
        <textarea v-model="planFeaturesText" rows="3" class="input" :placeholder="t('payment.admin.featuresPlaceholder')"></textarea>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.featuresHint') }}</p>
      </div>
      <div class="flex items-center gap-3">
        <label class="text-sm text-gray-700 dark:text-gray-300">{{ t('payment.admin.forSale') }}</label>
        <button
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
            planForm.for_sale ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'
          ]"
          @click="planForm.for_sale = !planForm.for_sale"
        >
          <span :class="[
            'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
            planForm.for_sale ? 'translate-x-5' : 'translate-x-0'
          ]" />
        </button>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" @click="emit('close')" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button type="submit" form="plan-form" :disabled="saving" class="btn btn-primary">{{ saving ? t('common.saving') : t('common.save') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { AdminPaymentConfig } from '@/api/admin/payment'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatPaymentAmount } from '@/components/payment/currency'
import type { SubscriptionPlan } from '@/types/payment'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  show: boolean
  plan: SubscriptionPlan | null
  paymentConfig?: AdminPaymentConfig | null
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const saving = ref(false)
// 限额：null / '' = 不限（提交成 -1，后端 <= 0 即不限）
type LimitInput = number | null | ''
const planForm = reactive({
  name: '', description: '', price: 0, original_price: 0, currency: '', validity_days: 30, validity_unit: 'days', sort_order: 0, for_sale: true,
  daily_limit_usd: null as LimitInput, weekly_limit_usd: null as LimitInput, monthly_limit_usd: null as LimitInput,
  entry_ids: [] as number[],
})
const planFeaturesText = ref('')

// 套餐模型集候选 = 目录里已上架的条目
const listedEntries = ref<ModelCatalogEntry[]>([])
const showModelDropdown = ref(false)
const modelSearch = ref('')

async function loadListedEntries() {
  try {
    const entries = await adminAPI.modelCatalog.listEntries()
    listedEntries.value = entries.filter(e => e.status === 'listed')
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

function entryLabel(entry: Pick<ModelCatalogEntry, 'model_id' | 'display_name'>): string {
  return entry.display_name && entry.display_name !== entry.model_id ? `${entry.display_name} (${entry.model_id})` : entry.model_id
}

const selectedEntries = computed(() => {
  const byID = new Map(listedEntries.value.map(e => [e.id, e]))
  return planForm.entry_ids.map(id => byID.get(id) ?? { id, model_id: `#${id}`, display_name: '' })
})

const filteredEntries = computed(() => {
  const q = modelSearch.value.trim().toLowerCase()
  if (!q) return listedEntries.value
  return listedEntries.value.filter(e => e.model_id.toLowerCase().includes(q) || (e.display_name || '').toLowerCase().includes(q))
})

function toggleEntry(id: number) {
  const idx = planForm.entry_ids.indexOf(id)
  if (idx >= 0) planForm.entry_ids.splice(idx, 1)
  else planForm.entry_ids.push(id)
}

const validityUnitOptions = computed(() => [
  { value: 'days', label: t('payment.admin.days') },
  { value: 'weeks', label: t('payment.admin.weeks') },
  { value: 'months', label: t('payment.admin.months') },
])

function roundCnyAmount(value: number): number {
  return Math.round(value * 100) / 100
}

function ceilCnyAmount(value: number): number {
  return Math.ceil(value * 100) / 100
}

const subscriptionCnyPreview = computed(() => {
  const price = Number(planForm.price) || 0
  const rate = Number(props.paymentConfig?.subscription_usd_to_cny_rate) || 0
  if (price <= 0 || rate <= 0) return null

  const amount = roundCnyAmount(price * rate)
  const feeRate = Number(props.paymentConfig?.recharge_fee_rate) || 0
  const fee = feeRate > 0 ? ceilCnyAmount((amount * feeRate) / 100) : 0
  const total = feeRate > 0 ? roundCnyAmount(amount + fee) : amount

  return {
    amount: formatPaymentAmount(amount, 'CNY'),
    feeRate,
    total: formatPaymentAmount(total, 'CNY'),
  }
})

// Reset form when dialog opens（immediate：挂载时就是打开状态也要初始化）
watch(() => props.show, (visible) => {
  if (!visible) return
  showModelDropdown.value = false
  modelSearch.value = ''
  void loadListedEntries()
  if (props.plan) {
    Object.assign(planForm, {
      name: props.plan.name, description: props.plan.description, price: props.plan.price, original_price: props.plan.original_price || 0,
      currency: props.plan.currency || '', validity_days: props.plan.validity_days, validity_unit: props.plan.validity_unit || 'days',
      sort_order: props.plan.sort_order || 0, for_sale: props.plan.for_sale,
      daily_limit_usd: props.plan.daily_limit_usd ?? null, weekly_limit_usd: props.plan.weekly_limit_usd ?? null, monthly_limit_usd: props.plan.monthly_limit_usd ?? null,
      entry_ids: props.plan.entry_ids ? [...props.plan.entry_ids] : (props.plan.models || []).map(m => m.entry_id),
    })
    planFeaturesText.value = (props.plan.features || []).join('\n')
  } else {
    Object.assign(planForm, {
      name: '', description: '', price: 0, original_price: 0, currency: '', validity_days: 30, validity_unit: 'days', sort_order: 0, for_sale: true,
      daily_limit_usd: null, weekly_limit_usd: null, monthly_limit_usd: null, entry_ids: [],
    })
    planFeaturesText.value = ''
  }
}, { immediate: true })

/** 空 / 非法 → -1（不限）；后端 <= 0 一律当不限 */
function limitForPayload(value: LimitInput): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : -1
}

/** Build request payload with snake_case keys matching backend JSON tags */
function buildPlanPayload() {
  const features = planFeaturesText.value.split('\n').map(f => f.trim()).filter(Boolean).join('\n')
  return {
    name: planForm.name,
    description: planForm.description,
    price: planForm.price,
    original_price: planForm.original_price || 0,
    currency: planForm.currency.trim().toUpperCase(),
    validity_days: planForm.validity_days,
    validity_unit: planForm.validity_unit,
    sort_order: planForm.sort_order,
    for_sale: planForm.for_sale,
    features,
    daily_limit_usd: limitForPayload(planForm.daily_limit_usd),
    weekly_limit_usd: limitForPayload(planForm.weekly_limit_usd),
    monthly_limit_usd: limitForPayload(planForm.monthly_limit_usd),
    entry_ids: [...planForm.entry_ids],
  }
}

async function handleSavePlan() {
  if (planForm.entry_ids.length === 0) {
    appStore.showError(t('payment.admin.modelsRequired'))
    return
  }
  if (!planForm.price || planForm.price <= 0) {
    appStore.showError(t('payment.admin.priceRequired'))
    return
  }
  if (!planForm.validity_days || planForm.validity_days < 1) {
    appStore.showError(t('payment.admin.validityRequired'))
    return
  }
  saving.value = true
  try {
    const data = buildPlanPayload()
    if (props.plan) { await adminPaymentAPI.updatePlan(props.plan.id, data) }
    else { await adminPaymentAPI.createPlan(data) }
    appStore.showSuccess(t('common.saved'))
    emit('close')
    emit('saved')
  } catch (err: unknown) { appStore.showError(extractApiErrorMessage(err, t('common.error'))) }
  finally { saving.value = false }
}
</script>
