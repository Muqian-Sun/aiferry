<template>
  <!--
    可购套餐卡片（muqian 2026-09-23「订阅套餐做成类似卡片的形式」）：hairline 描边 + 圆角，不用 .card、不做 hover 上浮，
    悬停只加深描边。自上而下：名称 / 说明 → 价格 + 有效期 → 额度与模型 → 底部整宽按钮；网格里等高，按钮对齐在底部。
  -->
  <div
    class="flex h-full flex-col rounded-lg border border-af-hairline bg-af-sheet p-5 transition-colors hover:border-af-ink-4"
    data-testid="plan-row"
  >
    <div class="min-w-0 flex-1">
      <div class="min-w-0 flex-1">
        <h3
          :title="plan.name"
          class="min-w-0 break-words [overflow-wrap:anywhere] text-base font-semibold leading-6 text-af-ink line-clamp-2"
        >
          {{ plan.name }}
        </h3>
      </div>
      <p v-if="plan.description" class="mt-1 text-13 leading-5 text-af-ink-3 line-clamp-2">
        {{ plan.description }}
      </p>

      <div class="mt-5 shrink-0">
        <div class="flex items-baseline gap-0.5">
          <span class="text-base font-medium text-af-ink-2">{{ planCurrencySymbol }}</span>
          <span class="text-3xl font-semibold tabular-nums tracking-[-0.02em] text-af-ink">{{ plan.price }}</span>
          <span v-if="plan.currency" class="text-xs text-af-ink-4">{{ plan.currency }}</span>
          <span class="ml-1.5 text-13 text-af-ink-3">/ {{ validitySuffix }}</span>
        </div>
        <p v-if="plan.original_price" class="mt-1 text-xs text-af-ink-4">
          <span class="line-through">{{ planCurrencySymbol }}{{ plan.original_price }}<template v-if="plan.currency"> {{ plan.currency }}</template></span>
          <span v-if="discountText" class="ml-1.5 font-medium text-af-ink-2">{{ discountText }}</span>
        </p>
      </div>

      <!-- 额度与模型：一项一行，标签在左、数值在右 -->
      <dl class="mt-5 space-y-2 border-t border-af-hairline pt-4 text-13">
        <div v-if="plan.daily_limit_usd != null" class="flex items-baseline justify-between gap-4">
          <dt class="text-af-ink-3">{{ t('payment.planCard.dailyLimit') }}</dt>
          <dd class="tabular-nums text-af-ink">${{ plan.daily_limit_usd }}</dd>
        </div>
        <div v-if="plan.weekly_limit_usd != null" class="flex items-baseline justify-between gap-4">
          <dt class="text-af-ink-3">{{ t('payment.planCard.weeklyLimit') }}</dt>
          <dd class="tabular-nums text-af-ink">${{ plan.weekly_limit_usd }}</dd>
        </div>
        <div v-if="plan.monthly_limit_usd != null" class="flex items-baseline justify-between gap-4">
          <dt class="text-af-ink-3">{{ t('payment.planCard.monthlyLimit') }}</dt>
          <dd class="tabular-nums text-af-ink">${{ plan.monthly_limit_usd }}</dd>
        </div>
        <div
          v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null"
          class="flex items-baseline justify-between gap-4"
        >
          <dt class="text-af-ink-3">{{ t('payment.planCard.quota') }}</dt>
          <dd class="text-af-ink">{{ t('payment.planCard.unlimited') }}</dd>
        </div>
        <div class="flex items-baseline justify-between gap-4" data-testid="plan-models">
          <dt class="shrink-0 text-af-ink-3">{{ t('payment.planCard.models') }}</dt>
          <dd class="min-w-0 text-right text-af-ink">{{ modelLabels.join(' / ') || '-' }}</dd>
        </div>
      </dl>

      <ul v-if="plan.features.length > 0" class="mt-4 space-y-1.5">
        <li v-for="feature in plan.features" :key="feature" class="flex items-start gap-2 text-13 text-af-ink-2">
          <Icon name="check" size="xs" class="mt-1 shrink-0 text-af-ink-4" />
          <span>{{ feature }}</span>
        </li>
      </ul>
    </div>

    <button
      type="button"
      class="btn btn-secondary btn-md mt-6 w-full"
      :disabled="blocked"
      data-testid="plan-select"
      @click="emit('select', plan)"
    >
      {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
    </button>
    <p v-if="blocked" class="mt-2 text-center text-xs text-af-ink-4" data-testid="plan-blocked">{{ t('payment.planCard.blockedByActive') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { planValiditySuffix } from './validity'
import Icon from '@/components/icons/Icon.vue'
import { currencySymbol } from '@/components/payment/currency'

const props = defineProps<{ plan: SubscriptionPlan; activeSubscriptions?: UserSubscription[] }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

// 同套餐已有有效订阅 → 续费；别的套餐有效 → 后端会 409 SUBSCRIPTION_ALREADY_ACTIVE，按钮直接禁用
const isRenewal = computed(() =>
  props.activeSubscriptions?.some(s => s.plan_id === props.plan.id && s.status === 'active') ?? false
)
const blocked = computed(() =>
  props.activeSubscriptions?.some(s => s.status === 'active' && s.plan_id !== props.plan.id) ?? false
)

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const planCurrencySymbol = computed(() => currencySymbol(props.plan.currency || 'USD'))

/** 套餐模型集：显示名优先，没有就 model_id */
const modelLabels = computed(() => (props.plan.models ?? []).map(m => m.display_name || m.model_id))

const validitySuffix = computed(() => planValiditySuffix(props.plan, t))
</script>
