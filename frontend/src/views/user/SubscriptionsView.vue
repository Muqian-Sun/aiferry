<template>
  <!--
    订阅页签：上半是我的订阅（每个订阅一行：名称 / 状态 / 到期 / 额度进度），下半是可购套餐与购买流程
    （PaymentView mode=subscription）。支付功能关闭时只有上半。行间只用 hairline，不做卡片、不按平台上色。
  -->
  <div class="space-y-8">
    <SheetSection :title="t('userSubscriptions.title')">
      <StatusState v-if="loading" kind="loading" :title="t('userUi.status.loading')" />
      <StatusState
        v-else-if="subscriptions.length === 0"
        kind="empty"
        :title="t('userSubscriptions.noActiveSubscriptions')"
        :description="t('userSubscriptions.noActiveSubscriptionsDesc')"
      />
      <ul v-else class="-my-5 divide-y divide-af-hairline">
        <li v-for="subscription in subscriptions" :key="subscription.id" class="py-5" data-testid="subscription-row">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="font-semibold text-af-ink">
                  {{ subscription.plan?.name || `Plan #${subscription.plan_id}` }}
                </h3>
                <span
                  :class="[
                    'badge',
                    subscription.status === 'active' ? 'badge-success' : subscription.status === 'expired' ? 'badge-gray' : 'badge-danger'
                  ]"
                >
                  {{ t(`userSubscriptions.status.${subscription.status}`) }}
                </span>
              </div>
              <dl class="mt-2 flex flex-wrap gap-x-6 gap-y-1 text-13">
                <div class="flex gap-1.5" data-testid="subscription-models">
                  <dt class="text-af-ink-4">{{ t('payment.planCard.models') }}</dt>
                  <dd class="text-af-ink-2">{{ planModelsLabel(subscription) }}</dd>
                </div>
                <div v-if="subscription.api_key" class="flex gap-1.5" data-testid="subscription-key">
                  <dt class="text-af-ink-4">{{ t('payment.planCard.apiKey') }}</dt>
                  <dd class="text-af-ink-2">
                    {{ subscription.api_key.name }}
                    <code class="ml-1 tabular-nums text-af-ink-4">{{ subscription.api_key.key_masked }}</code>
                  </dd>
                </div>
                <div class="flex gap-1.5">
                  <dt class="text-af-ink-4">{{ t('userSubscriptions.expires') }}</dt>
                  <dd v-if="subscription.expires_at" :class="getExpirationClass(subscription.expires_at)">
                    {{ formatExpirationDate(subscription.expires_at) }}
                  </dd>
                  <dd v-else class="text-af-ink-2">{{ t('userSubscriptions.noExpiration') }}</dd>
                </div>
              </dl>
            </div>
            <button
              v-if="subscription.status === 'active' && canPurchase"
              type="button"
              class="btn btn-secondary btn-sm shrink-0"
              @click="renew(subscription.plan_id)"
            >
              {{ t('payment.renewNow') }}
            </button>
          </div>

          <!-- 额度进度：有哪个限额画哪条 -->
          <div v-if="hasAnyLimit(subscription)" class="mt-4 grid gap-3 sm:grid-cols-3">
            <div v-if="subscription.plan?.daily_limit_usd">
              <div class="flex items-baseline justify-between text-13">
                <span class="text-af-ink-3">{{ t('userSubscriptions.daily') }}</span>
                <span class="tabular-nums text-af-ink-2">
                  ${{ (subscription.daily_usage_usd || 0).toFixed(2) }} / ${{ subscription.plan.daily_limit_usd.toFixed(2) }}
                </span>
              </div>
              <div
                class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-af-hairline"
                role="meter"
                :aria-label="t('userSubscriptions.daily')"
                :aria-valuemin="0"
                :aria-valuemax="subscription.plan.daily_limit_usd"
                :aria-valuenow="Math.min(subscription.daily_usage_usd || 0, subscription.plan.daily_limit_usd)"
              >
                <div
                  class="h-full rounded-full"
                  :class="getProgressBarClass(subscription.daily_usage_usd, subscription.plan.daily_limit_usd)"
                  :style="{ width: getProgressWidth(subscription.daily_usage_usd, subscription.plan.daily_limit_usd) }"
                ></div>
              </div>
              <p v-if="subscription.daily_window_start" class="mt-1 text-xs text-af-ink-4">
                {{ formatDailyUsageWindow(subscription) }}
              </p>
            </div>

            <div v-if="subscription.plan?.weekly_limit_usd">
              <div class="flex items-baseline justify-between text-13">
                <span class="text-af-ink-3">{{ t('userSubscriptions.weekly') }}</span>
                <span class="tabular-nums text-af-ink-2">
                  ${{ (subscription.weekly_usage_usd || 0).toFixed(2) }} / ${{ subscription.plan.weekly_limit_usd.toFixed(2) }}
                </span>
              </div>
              <div
                class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-af-hairline"
                role="meter"
                :aria-label="t('userSubscriptions.weekly')"
                :aria-valuemin="0"
                :aria-valuemax="subscription.plan.weekly_limit_usd"
                :aria-valuenow="Math.min(subscription.weekly_usage_usd || 0, subscription.plan.weekly_limit_usd)"
              >
                <div
                  class="h-full rounded-full"
                  :class="getProgressBarClass(subscription.weekly_usage_usd, subscription.plan.weekly_limit_usd)"
                  :style="{ width: getProgressWidth(subscription.weekly_usage_usd, subscription.plan.weekly_limit_usd) }"
                ></div>
              </div>
              <p v-if="subscription.weekly_window_start" class="mt-1 text-xs text-af-ink-4">
                {{ t('userSubscriptions.resetIn', { time: formatResetTime(subscription.weekly_window_start, 168) }) }}
              </p>
            </div>

            <div v-if="subscription.plan?.monthly_limit_usd">
              <div class="flex items-baseline justify-between text-13">
                <span class="text-af-ink-3">{{ t('userSubscriptions.monthly') }}</span>
                <span class="tabular-nums text-af-ink-2">
                  ${{ (subscription.monthly_usage_usd || 0).toFixed(2) }} / ${{ subscription.plan.monthly_limit_usd.toFixed(2) }}
                </span>
              </div>
              <div
                class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-af-hairline"
                role="meter"
                :aria-label="t('userSubscriptions.monthly')"
                :aria-valuemin="0"
                :aria-valuemax="subscription.plan.monthly_limit_usd"
                :aria-valuenow="Math.min(subscription.monthly_usage_usd || 0, subscription.plan.monthly_limit_usd)"
              >
                <div
                  class="h-full rounded-full"
                  :class="getProgressBarClass(subscription.monthly_usage_usd, subscription.plan.monthly_limit_usd)"
                  :style="{ width: getProgressWidth(subscription.monthly_usage_usd, subscription.plan.monthly_limit_usd) }"
                ></div>
              </div>
              <p v-if="subscription.monthly_window_start" class="mt-1 text-xs text-af-ink-4">
                {{ t('userSubscriptions.resetIn', { time: formatResetTime(subscription.monthly_window_start, 720) }) }}
              </p>
            </div>
          </div>
          <p v-else class="mt-3 text-13 text-af-ink-3">
            <span class="font-medium text-af-success">{{ t('userSubscriptions.unlimited') }}</span>
            · {{ t('userSubscriptions.unlimitedDesc') }}
          </p>
        </li>
      </ul>
    </SheetSection>

    <!-- 可购套餐 + 购买流程；支付关闭时不渲染（套餐无法下单） -->
    <SheetSection v-if="canPurchase" ref="purchaseSection" :title="t('payment.selectPlan')">
      <PaymentView ref="purchase" mode="subscription" />
    </SheetSection>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, type ComponentPublicInstance } from 'vue'
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

// 能不能买：支付功能开着才渲染套餐区与「续费」按钮
const canPurchase = computed(() => billingFlags.value.payment)
const purchase = ref<InstanceType<typeof PaymentView> | null>(null)
const purchaseSection = ref<ComponentPublicInstance | null>(null)

/** 续费：交给嵌入的支付引擎选套餐，并把视口滚到套餐区 */
function renew(planId: number) {
  purchase.value?.startRenewal(planId)
  purchaseSection.value?.$el?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
}

function hasAnyLimit(subscription: UserSubscription): boolean {
  const plan = subscription.plan
  return Boolean(plan?.daily_limit_usd || plan?.weekly_limit_usd || plan?.monthly_limit_usd)
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
