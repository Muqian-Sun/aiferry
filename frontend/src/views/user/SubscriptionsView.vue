<template>
  <!--
    订阅（muqian 2026-09-24「排版还是很差」「重复信息太多了」）：「当前订阅」整宽描边面板 → 「选择套餐」整宽卡片网格。
    每样信息只出现一次：面板管状态 / 到期 / 用量 / 订阅密钥；套餐内容（价格、额度、模型）与续费只在卡片上。
    支付功能关闭时没有卡片，模型才在面板里出现。
  -->
  <div class="space-y-10">
    <SheetSection :title="t('payment.activeSubscription')">
      <StatusState v-if="loading" kind="loading" :title="t('userUi.status.loading')" />
      <StatusState
        v-else-if="subscriptions.length === 0"
        kind="empty"
        :title="t('userSubscriptions.noActiveSubscriptions')"
        :description="t('userSubscriptions.noActiveSubscriptionsDesc')"
      />
      <ul v-else class="space-y-4">
        <li
          v-for="subscription in subscriptions"
          :key="subscription.id"
          class="rounded-lg border border-af-hairline bg-af-sheet p-6"
          data-testid="subscription-row"
        >
          <div>
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-lg font-semibold tracking-[-0.01em] text-af-ink">
                  {{ subscription.plan?.name || `Plan #${subscription.plan_id}` }}
                </h3>
                <span
                  :class="[
                    'badge',
                    subscription.status === 'active' ? 'badge-gray' : subscription.status === 'expired' ? 'badge-gray opacity-70' : 'badge-danger'
                  ]"
                >
                  {{ t(`userSubscriptions.status.${subscription.status}`) }}
                </span>
              </div>
              <p class="mt-1 text-13">
                <span class="text-af-ink-4">{{ t('userSubscriptions.expires') }}</span>
                <span v-if="subscription.expires_at" class="ml-1.5" :class="getExpirationClass(subscription.expires_at)">
                  {{ formatExpirationDate(subscription.expires_at) }}
                </span>
                <span v-else class="ml-1.5 text-af-ink-2">{{ t('userSubscriptions.noExpiration') }}</span>
              </p>
            </div>
          </div>

          <div class="mt-5 grid gap-x-12 gap-y-5 border-t border-af-hairline pt-5 md:grid-cols-2">
            <div v-if="hasAnyLimit(subscription)" class="space-y-4">
              <div v-for="meter in limitMeters(subscription)" :key="meter.key">
                <div class="flex items-baseline justify-between text-13">
                  <span class="text-af-ink-3">{{ meter.label }}</span>
                  <span class="tabular-nums text-af-ink">
                    ${{ meter.used.toFixed(2) }} <span class="text-af-ink-4">/ ${{ meter.limit.toFixed(2) }}</span>
                  </span>
                </div>
                <div
                  class="mt-2 h-1.5 overflow-hidden rounded-full bg-af-sunken"
                  role="meter"
                  :aria-label="meter.label"
                  :aria-valuemin="0"
                  :aria-valuemax="meter.limit"
                  :aria-valuenow="Math.min(meter.used, meter.limit)"
                >
                  <div class="h-full rounded-full" :class="getProgressBarClass(meter.used, meter.limit)" :style="{ width: getProgressWidth(meter.used, meter.limit) }"></div>
                </div>
                <p v-if="meter.hint" class="mt-1.5 text-xs text-af-ink-4">{{ meter.hint }}</p>
              </div>
            </div>
            <p v-else class="text-13 text-af-ink-3">
              <span class="font-medium text-af-ink">{{ t('userSubscriptions.unlimited') }}</span>
              · {{ t('userSubscriptions.unlimitedDesc') }}
            </p>

            <dl class="grid content-start gap-x-6 gap-y-2 text-13 grid-cols-[5rem_minmax(0,1fr)]">
              <template v-if="!canPurchase">
                <dt class="text-af-ink-3">{{ t('payment.planCard.models') }}</dt>
                <dd class="text-af-ink" data-testid="subscription-models">{{ planModelsLabel(subscription) }}</dd>
              </template>
              <template v-if="subscription.api_key">
                <dt class="text-af-ink-3">{{ t('payment.planCard.apiKey') }}</dt>
                <dd class="text-af-ink" data-testid="subscription-key">
                  {{ subscription.api_key.name }}
                  <code class="ml-1 tabular-nums text-af-ink-4">{{ subscription.api_key.key_masked }}</code>
                </dd>
              </template>
            </dl>
          </div>
        </li>
      </ul>
    </SheetSection>

    <!-- 可购套餐 + 购买流程：整宽卡片网格；确认购买时限宽。支付关闭时不渲染（套餐无法下单） -->
    <SheetSection v-if="canPurchase" :title="t('payment.selectPlan')" :description="t('purchase.subscriptionDescription')">
      <PaymentView mode="subscription" />
    </SheetSection>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import PaymentView from '@/views/user/PaymentView.vue'
import { useBillingFlags } from '@/views/user/billing/useBillingFlags'
import { formatDateTimeToMinute } from '@/utils/format'
import {
  getExpirationDateRelation,
  getRemainingDurationParts,
  isOneTimeDailyQuota,
  type RemainingDurationParts
} from '@/utils/subscriptionQuota'

const { t } = useI18n()
const appStore = useAppStore()
const billingFlags = useBillingFlags()

const subscriptions = ref<UserSubscription[]>([])
const loading = ref(true)

// 能不能买：支付功能开着才渲染套餐卡片（续费也在卡片上）；关着时模型集改在面板里显示
const canPurchase = computed(() => billingFlags.value.payment)

function hasAnyLimit(subscription: UserSubscription): boolean {
  const plan = subscription.plan
  return Boolean(plan?.daily_limit_usd || plan?.weekly_limit_usd || plan?.monthly_limit_usd)
}

interface LimitMeter {
  key: 'daily' | 'weekly' | 'monthly'
  label: string
  used: number
  limit: number
  hint: string
}

/** 有哪个限额就给哪条额度条（日 / 周 / 月），附上重置时间 */
function limitMeters(subscription: UserSubscription): LimitMeter[] {
  const plan = subscription.plan
  const meters: LimitMeter[] = []
  if (plan?.daily_limit_usd) {
    meters.push({
      key: 'daily',
      label: t('userSubscriptions.daily'),
      used: subscription.daily_usage_usd || 0,
      limit: plan.daily_limit_usd,
      hint: subscription.daily_window_start ? formatDailyUsageWindow(subscription) : ''
    })
  }
  if (plan?.weekly_limit_usd) {
    meters.push({
      key: 'weekly',
      label: t('userSubscriptions.weekly'),
      used: subscription.weekly_usage_usd || 0,
      limit: plan.weekly_limit_usd,
      hint: subscription.weekly_window_start ? t('userSubscriptions.resetIn', { time: formatResetTime(subscription.weekly_window_start, 168) }) : ''
    })
  }
  if (plan?.monthly_limit_usd) {
    meters.push({
      key: 'monthly',
      label: t('userSubscriptions.monthly'),
      used: subscription.monthly_usage_usd || 0,
      limit: plan.monthly_limit_usd,
      hint: subscription.monthly_window_start ? t('userSubscriptions.resetIn', { time: formatResetTime(subscription.monthly_window_start, 720) }) : ''
    })
  }
  return meters
}

/** 套餐模型集：显示名优先，没有就 model_id */
function planModelsLabel(subscription: UserSubscription): string {
  const models = subscription.plan?.models ?? []
  if (models.length === 0) return '-'
  return models.map(m => m.display_name || m.model_id).join(' / ')
}

async function loadSubscriptions() {
  try {
    loading.value = true
    subscriptions.value = await subscriptionsAPI.getMySubscriptions()
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  const percentage = Math.min(((used || 0) / limit) * 100, 100)
  return `${percentage}%`
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'bg-af-hairline-strong'
  const percentage = ((used || 0) / limit) * 100
  if (percentage >= 90) return 'bg-af-danger'
  if (percentage >= 70) return 'bg-af-warning'
  return 'bg-af-brand'
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  const relation = getExpirationDateRelation(expires, now)

  if (relation === null) return ''

  if (relation === 'expired') {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateTimeToMinute(expires)

  if (relation === 'today') {
    return `${dateStr} (${t('common.today')})`
  }
  if (relation === 'tomorrow') {
    return `${dateStr} (${t('common.tomorrow')})`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function getExpirationClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (diff <= 0) return 'text-af-danger font-medium'
  if (days <= 3) return 'text-af-danger'
  if (days <= 7) return 'text-af-warning'
  return 'text-af-ink-2'
}

function formatDurationParts(parts: RemainingDurationParts): string {
  if (parts.days > 0) {
    return `${parts.days}d ${parts.hours}h`
  }

  if (parts.hours > 0) {
    return `${parts.hours}h ${parts.minutes}m`
  }

  return `${parts.minutes}m`
}

function formatDailyUsageWindow(subscription: UserSubscription): string {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    if (!parts) return t('userSubscriptions.windowNotActive')
    return t('userSubscriptions.quotaEndsIn', { time: formatDurationParts(parts) })
  }

  return t('userSubscriptions.resetIn', {
    time: formatResetTime(subscription.daily_window_start, 24)
  })
}

function formatResetTime(windowStart: string | null, windowHours: number): string {
  if (!windowStart) return t('userSubscriptions.windowNotActive')

  const start = new Date(windowStart)
  const end = new Date(start.getTime() + windowHours * 60 * 60 * 1000)
  const parts = getRemainingDurationParts(end)

  return parts ? formatDurationParts(parts) : t('userSubscriptions.windowNotActive')
}

onMounted(() => {
  loadSubscriptions()
})
</script>
