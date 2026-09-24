<template>
  <!--
    订阅（muqian 2026-09-24「已经订阅了就不要再显示可选套餐」「分成周限额和月限额」）：
    「当前订阅」整宽描边面板：顶行名称 / 状态 / 到期 + 续费；额度固定分「周限额」「月限额」两块（没设写无限制），
    日限额设了才多一块；底部模型与订阅密钥。
    有生效中的订阅时不列可选套餐（同一时间只能持有一条，后端也会拒）；续费在面板上点开，确认购买出现在面板下方。
    没有生效订阅、且支付开着时，才列「选择套餐」卡片。
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
          <div class="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
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
            <button
              v-if="subscription.status === 'active' && canPurchase"
              type="button"
              class="hero-link shrink-0 text-13 font-medium"
              data-testid="subscription-renew"
              @click="startRenew(subscription.plan_id)"
            >
              {{ t('payment.renewNow') }}
              <Icon name="arrowRight" size="xs" class="hero-link-arrow" />
            </button>
          </div>

          <!-- 额度：周、月两块固定；日限额设了才多一块（排最前） -->
          <div
            :class="['mt-5 grid gap-x-10 gap-y-5 border-t border-af-hairline pt-5', limitBlocks(subscription).length > 2 ? 'md:grid-cols-3' : 'md:grid-cols-2']"
            data-testid="subscription-limits"
          >
            <div v-for="block in limitBlocks(subscription)" :key="block.key" class="min-w-0">
              <div class="flex items-baseline justify-between gap-3 text-13">
                <span class="text-af-ink-3">{{ block.label }}</span>
                <span v-if="block.limit !== null" class="tabular-nums text-af-ink">
                  ${{ block.used.toFixed(2) }} <span class="text-af-ink-4">/ ${{ block.limit.toFixed(2) }}</span>
                </span>
                <span v-else class="text-af-ink">{{ t('payment.planCard.unlimited') }}</span>
              </div>
              <template v-if="block.limit !== null">
                <div
                  class="mt-2 h-1.5 overflow-hidden rounded-full bg-af-sunken"
                  role="meter"
                  :aria-label="block.label"
                  :aria-valuemin="0"
                  :aria-valuemax="block.limit"
                  :aria-valuenow="Math.min(block.used, block.limit)"
                >
                  <div class="h-full rounded-full" :class="getProgressBarClass(block.used, block.limit)" :style="{ width: getProgressWidth(block.used, block.limit) }"></div>
                </div>
                <p v-if="block.hint" class="mt-1.5 text-xs text-af-ink-4">{{ block.hint }}</p>
              </template>
            </div>
          </div>

          <dl
            v-if="!showPlanSection || subscription.api_key"
            class="mt-5 grid gap-x-6 gap-y-2 border-t border-af-hairline pt-4 text-13 grid-cols-[5rem_minmax(0,1fr)]"
          >
            <!-- 可选套餐卡片不出现时（已订阅 / 支付关闭），模型集在这里 -->
            <template v-if="!showPlanSection">
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
        </li>
      </ul>
    </SheetSection>

    <!-- 续费：只为当前这个套餐确认购买，出现在面板下方；取消就收起 -->
    <SheetSection v-if="renewingPlanId !== null" ref="renewSection" :title="t('payment.renewNow')">
      <PaymentView mode="subscription" :renew-plan-id="renewingPlanId" @cancel="renewingPlanId = null" />
    </SheetSection>

    <!-- 可选套餐：没有生效订阅、且支付开着时才列 -->
    <SheetSection v-if="showPlanSection" :title="t('payment.selectPlan')" :description="t('purchase.subscriptionDescription')">
      <PaymentView mode="subscription" />
    </SheetSection>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, onMounted, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import Icon from '@/components/icons/Icon.vue'
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

// 能不能买：支付功能开着才有续费与可选套餐
const canPurchase = computed(() => billingFlags.value.payment)

// 同一时间只能持有一条订阅：有生效中的订阅时不列可选套餐，只在面板上续费
const hasActiveSubscription = computed(() => subscriptions.value.some((s) => s.status === 'active'))
const showPlanSection = computed(() => canPurchase.value && !loading.value && !hasActiveSubscription.value)

// 续费：面板上点开，确认购买出现在面板下方
const renewingPlanId = ref<number | null>(null)
const renewSection = ref<ComponentPublicInstance | null>(null)
async function startRenew(planId: number) {
  renewingPlanId.value = planId
  await nextTick()
  renewSection.value?.$el?.scrollIntoView?.({ behavior: 'smooth', block: 'start' })
}

interface LimitBlock {
  key: 'daily' | 'weekly' | 'monthly'
  label: string
  used: number
  /** null = 该周期不限额 */
  limit: number | null
  hint: string
}

/** 额度块：周、月固定两块（没设 → 无限制）；日限额设了才多一块，排最前。附重置时间 */
function limitBlocks(subscription: UserSubscription): LimitBlock[] {
  const plan = subscription.plan
  const blocks: LimitBlock[] = []
  if (plan?.daily_limit_usd) {
    blocks.push({
      key: 'daily',
      label: t('payment.planCard.dailyLimit'),
      used: subscription.daily_usage_usd || 0,
      limit: plan.daily_limit_usd,
      hint: subscription.daily_window_start ? formatDailyUsageWindow(subscription) : ''
    })
  }
  const weekly = plan?.weekly_limit_usd || null
  blocks.push({
    key: 'weekly',
    label: t('payment.planCard.weeklyLimit'),
    used: subscription.weekly_usage_usd || 0,
    limit: weekly,
    hint: weekly && subscription.weekly_window_start ? t('userSubscriptions.resetIn', { time: formatResetTime(subscription.weekly_window_start, 168) }) : ''
  })
  const monthly = plan?.monthly_limit_usd || null
  blocks.push({
    key: 'monthly',
    label: t('payment.planCard.monthlyLimit'),
    used: subscription.monthly_usage_usd || 0,
    limit: monthly,
    hint: monthly && subscription.monthly_window_start ? t('userSubscriptions.resetIn', { time: formatResetTime(subscription.monthly_window_start, 720) }) : ''
  })
  return blocks
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
