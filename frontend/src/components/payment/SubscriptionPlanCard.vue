<template>
  <!--
    可购套餐一行：左侧名称 / 说明 / 额度事实，右侧价格与操作。
    行间由父级 divide-y 分隔，不做卡片、不按平台上色（平台只以文字标出）。
  -->
  <div class="flex flex-col gap-4 py-5 sm:flex-row sm:items-start sm:justify-between" data-testid="plan-row">
    <div class="min-w-0 flex-1">
      <h3
        :title="plan.name"
        class="min-w-0 break-words [overflow-wrap:anywhere] text-base font-semibold leading-6 text-af-ink line-clamp-2"
      >
        {{ plan.name }}
      </h3>
      <p v-if="plan.description" class="mt-0.5 text-13 leading-5 text-af-ink-3 line-clamp-2">
        {{ plan.description }}
      </p>

      <!-- 额度事实 -->
      <dl class="mt-3 grid grid-cols-2 gap-x-6 gap-y-1 text-13 sm:grid-cols-3">
        <div class="flex items-baseline justify-between gap-2 sm:block">
          <dt class="text-af-ink-4">{{ t('payment.planCard.rate') }}</dt>
          <dd class="font-medium tabular-nums text-af-ink-2">{{ rateDisplay }}</dd>
        </div>
        <div v-if="hasPeakRate" class="col-span-2 flex items-baseline justify-between gap-2 sm:col-span-1 sm:block">
          <dt class="text-af-ink-4">{{ t('payment.planCard.peakRate') }}</dt>
          <dd class="font-medium text-af-warning">{{ peakRateDisplay }}</dd>
        </div>
        <div v-if="plan.daily_limit_usd != null" class="flex items-baseline justify-between gap-2 sm:block">
          <dt class="text-af-ink-4">{{ t('payment.planCard.dailyLimit') }}</dt>
          <dd class="font-medium tabular-nums text-af-ink-2">${{ plan.daily_limit_usd }}</dd>
        </div>
        <div v-if="plan.weekly_limit_usd != null" class="flex items-baseline justify-between gap-2 sm:block">
          <dt class="text-af-ink-4">{{ t('payment.planCard.weeklyLimit') }}</dt>
          <dd class="font-medium tabular-nums text-af-ink-2">${{ plan.weekly_limit_usd }}</dd>
        </div>
        <div v-if="plan.monthly_limit_usd != null" class="flex items-baseline justify-between gap-2 sm:block">
          <dt class="text-af-ink-4">{{ t('payment.planCard.monthlyLimit') }}</dt>
          <dd class="font-medium tabular-nums text-af-ink-2">${{ plan.monthly_limit_usd }}</dd>
        </div>
        <div
          v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null"
          class="flex items-baseline justify-between gap-2 sm:block"
        >
          <dt class="text-af-ink-4">{{ t('payment.planCard.quota') }}</dt>
          <dd class="font-medium text-af-ink-2">{{ t('payment.planCard.unlimited') }}</dd>
        </div>
        <div v-if="modelScopeLabels.length > 0" class="col-span-2 flex items-baseline justify-between gap-2 sm:col-span-3 sm:block">
          <dt class="text-af-ink-4">{{ t('payment.planCard.models') }}</dt>
          <dd class="font-medium text-af-ink-2">{{ modelScopeLabels.join(' / ') }}</dd>
        </div>
      </dl>

      <ul v-if="plan.features.length > 0" class="mt-3 space-y-1">
        <li v-for="feature in plan.features" :key="feature" class="flex items-start gap-1.5 text-13 text-af-ink-2">
          <svg class="mt-1 h-3.5 w-3.5 flex-shrink-0 text-af-success" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
          </svg>
          <span>{{ feature }}</span>
        </li>
      </ul>
    </div>

    <div class="shrink-0 text-right">
      <div class="flex items-baseline justify-end gap-1">
        <span class="text-xs text-af-ink-4">{{ planCurrencySymbol }}</span>
        <span class="text-2xl font-semibold tabular-nums text-af-ink">{{ plan.price }}</span>
        <span v-if="plan.currency" class="text-xs font-medium text-af-ink-4">{{ plan.currency }}</span>
      </div>
      <div class="flex items-center justify-end gap-1">
        <span class="badge badge-gray shrink-0">{{ pLabel }}</span>
        <span class="text-xs text-af-ink-4">/ {{ validitySuffix }}</span>
      </div>
      <div v-if="plan.original_price" class="mt-0.5 flex items-center justify-end gap-1.5">
        <span class="text-xs text-af-ink-4 line-through">{{ planCurrencySymbol }}{{ plan.original_price }}<template v-if="plan.currency"> {{ plan.currency }}</template></span>
        <span class="text-xs font-medium text-af-success">{{ discountText }}</span>
      </div>

      <button type="button" class="btn btn-secondary btn-sm mt-3 w-full sm:w-auto" @click="emit('select', plan)">
        {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { useAppStore } from '@/stores/app'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { planValiditySuffix } from './validity'
import { currencySymbol } from '@/components/payment/currency'
import { platformLabel } from '@/utils/platformColors'

const props = defineProps<{ plan: SubscriptionPlan; activeSubscriptions?: UserSubscription[] }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

const platform = computed(() => props.plan.group_platform || '')
const isRenewal = computed(() =>
  props.activeSubscriptions?.some(s => s.group_id === props.plan.group_id && s.status === 'active') ?? false
)
const pLabel = computed(() => platformLabel(platform.value))

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
  const rate = props.plan.rate_multiplier ?? 1
  return `×${Number(rate.toPrecision(10))}`
})

const appStore = useAppStore()
const planCurrencySymbol = computed(() => currencySymbol(props.plan.currency || 'USD'))

const hasPeakRate = computed(() => groupHasPeakRate(props.plan))

const peakRateDisplay = computed(() => {
  return formatPeakRateWindow(props.plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

const modelScopeLabels = computed(() => {
  if (platform.value !== 'antigravity') return []
  const scopes = props.plan.supported_model_scopes
  if (!scopes || scopes.length === 0) return []
  return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const validitySuffix = computed(() => planValiditySuffix(props.plan, t))
</script>
