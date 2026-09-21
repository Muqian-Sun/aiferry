<template>
  <!--
    兑换：余额两格 → 兑换码表单（结果 / 错误就地显示）→ 规则说明 → 最近记录（hairline 行）。
    单列 640；没有渐变余额卡、没有彩色提示框。
  -->
  <div class="max-w-form space-y-8">
    <dl class="grid grid-cols-2 divide-x divide-af-hairline border-b border-af-hairline pb-4">
      <div class="min-w-0 pr-6">
        <dt class="text-13 text-af-ink-3">{{ t('redeem.currentBalance') }}</dt>
        <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">${{ user?.balance?.toFixed(2) || '0.00' }}</dd>
      </div>
      <div class="min-w-0 pl-6">
        <dt class="text-13 text-af-ink-3">{{ t('redeem.concurrency') }}</dt>
        <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">{{ user?.concurrency || 0 }} <span class="text-13 font-normal text-af-ink-3">{{ t('redeem.requests') }}</span></dd>
      </div>
    </dl>

    <SheetSection :title="t('redeem.redeemCodeLabel')">
      <form class="space-y-4" @submit.prevent="handleRedeem">
        <div>
          <label for="code" class="sr-only">{{ t('redeem.redeemCodeLabel') }}</label>
          <input
            id="code"
            v-model="redeemCode"
            type="text"
            required
            autocomplete="off"
            spellcheck="false"
            :placeholder="t('redeem.redeemCodePlaceholder')"
            :disabled="submitting"
            class="input font-mono"
          />
          <p class="input-hint">{{ t('redeem.redeemCodeHint') }}</p>
        </div>
        <button type="submit" :disabled="!redeemCode || submitting" class="btn btn-primary btn-md">
          {{ submitting ? t('redeem.redeeming') : t('redeem.redeemButton') }}
        </button>
      </form>

      <div v-if="redeemResult" role="status" class="mt-4 border-l-2 border-af-success pl-3 text-13 text-af-ink-2">
        <p class="font-medium text-af-success">{{ t('redeem.redeemSuccess') }}</p>
        <p v-if="redeemResult.type === 'balance'" class="mt-1 tabular-nums">
          {{ t('redeem.added') }}: ${{ redeemResult.value.toFixed(2) }}
        </p>
        <p v-else-if="redeemResult.type === 'concurrency'" class="mt-1 tabular-nums">
          {{ t('redeem.added') }}: {{ redeemResult.value }} {{ t('redeem.concurrentRequests') }}
        </p>
        <p v-else-if="redeemResult.type === 'subscription'" class="mt-1">
          {{ t('redeem.subscriptionAssigned') }}
          <span v-if="redeemResult.plan?.name" data-testid="redeem-plan-name"> - {{ redeemResult.plan.name }}</span>
          <span v-if="redeemResult.validity_days"> ({{ t('redeem.subscriptionDays', { days: redeemResult.validity_days }) }})</span>
        </p>
      </div>

      <div v-if="errorMessage" role="alert" class="mt-4 border-l-2 border-af-danger pl-3 text-13 text-af-ink-2">
        <p class="font-medium text-af-danger">{{ t('redeem.redeemFailed') }}</p>
        <p class="mt-1">{{ errorMessage }}</p>
      </div>
    </SheetSection>

    <SheetSection :title="t('redeem.aboutCodes')">
      <ul class="list-disc space-y-1 pl-5 text-13 leading-5 text-af-ink-3">
        <li>{{ t('redeem.codeRule1') }}</li>
        <li>{{ t('redeem.codeRule2') }}</li>
        <li>
          {{ t('redeem.codeRule3') }}
          <span v-if="contactInfo" class="ml-1 font-medium text-af-ink-2">{{ contactInfo }}</span>
        </li>
        <li>{{ t('redeem.codeRule4') }}</li>
      </ul>
    </SheetSection>

    <SheetSection :title="t('redeem.recentActivity')">
      <StatusState v-if="loadingHistory" kind="loading" :title="t('userUi.status.loading')" />
      <ul v-else-if="history.length > 0" class="-my-3 divide-y divide-af-hairline">
        <li v-for="item in history" :key="item.id" class="flex items-start justify-between gap-4 py-3">
          <div class="min-w-0">
            <p class="text-sm font-medium text-af-ink">{{ getHistoryItemTitle(item) }}</p>
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{ formatDateTime(item.used_at) }}
              <template v-if="!isAdminAdjustment(item.type)"> · <span class="font-mono">{{ item.code.slice(0, 8) }}…</span></template>
              <template v-else> · {{ t('redeem.adminAdjustment') }}</template>
            </p>
            <p v-if="item.notes" class="mt-0.5 truncate text-xs text-af-ink-3" :title="item.notes">{{ item.notes }}</p>
          </div>
          <p
            class="shrink-0 text-sm font-semibold tabular-nums"
            :class="isBalanceType(item.type) || !isSubscriptionType(item.type)
              ? (item.value >= 0 ? 'text-af-success' : 'text-af-danger')
              : 'text-af-ink'"
          >
            {{ formatHistoryValue(item) }}
          </p>
        </li>
      </ul>
      <StatusState v-else kind="empty" :title="t('redeem.historyWillAppear')" />
    </SheetSection>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { redeemAPI, authAPI, type RedeemHistoryItem } from '@/api'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)

const redeemCode = ref('')
const submitting = ref(false)
const redeemResult = ref<RedeemHistoryItem | null>(null)
const errorMessage = ref('')

// History data
const history = ref<RedeemHistoryItem[]>([])
const loadingHistory = ref(false)
const contactInfo = ref('')

// Helper functions for history display
const isBalanceType = (type: string) => {
  return type === 'balance' || type === 'admin_balance'
}

const isSubscriptionType = (type: string) => {
  return type === 'subscription'
}

const isAdminAdjustment = (type: string) => {
  return type === 'admin_balance' || type === 'admin_concurrency'
}

const getHistoryItemTitle = (item: RedeemHistoryItem) => {
  if (item.type === 'balance') {
    return t('redeem.balanceAddedRedeem')
  } else if (item.type === 'admin_balance') {
    return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
  } else if (item.type === 'concurrency') {
    return t('redeem.concurrencyAddedRedeem')
  } else if (item.type === 'admin_concurrency') {
    return item.value >= 0 ? t('redeem.concurrencyAddedAdmin') : t('redeem.concurrencyReducedAdmin')
  } else if (item.type === 'subscription') {
    return t('redeem.subscriptionAssigned')
  }
  return t('common.unknown')
}

const formatHistoryValue = (item: RedeemHistoryItem) => {
  if (isBalanceType(item.type)) {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}$${item.value.toFixed(2)}`
  } else if (isSubscriptionType(item.type)) {
    // 订阅类型显示有效天数和套餐名称
    const days = item.validity_days || Math.round(item.value)
    const planName = item.plan?.name || ''
    return planName ? `${days}${t('redeem.days')} - ${planName}` : `${days}${t('redeem.days')}`
  } else {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}${item.value} ${t('redeem.requests')}`
  }
}

const fetchHistory = async () => {
  loadingHistory.value = true
  try {
    history.value = await redeemAPI.getHistory()
  } catch (error) {
    console.error('Failed to fetch history:', error)
  } finally {
    loadingHistory.value = false
  }
}

const handleRedeem = async () => {
  if (!redeemCode.value.trim()) {
    appStore.showError(t('redeem.pleaseEnterCode'))
    return
  }

  submitting.value = true
  errorMessage.value = ''
  redeemResult.value = null

  try {
    const result = await redeemAPI.redeem(redeemCode.value.trim())

    redeemResult.value = result

    // Refresh user data to get updated balance/concurrency
    try {
      await authStore.refreshUser()
    } catch (error) {
      console.error('Failed to refresh user after redeem:', error)
      appStore.showWarning(t('redeem.userRefreshFailed'))
    }

    // If subscription type, immediately refresh subscription status
    if (result.type === 'subscription') {
      try {
        await subscriptionStore.fetchActiveSubscriptions(true) // force refresh
      } catch (error) {
        console.error('Failed to refresh subscriptions after redeem:', error)
        appStore.showWarning(t('redeem.subscriptionRefreshFailed'))
      }
    }

    // Clear the input
    redeemCode.value = ''

    // Refresh history
    await fetchHistory()

    // Show success toast
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
  } catch (error: any) {
    errorMessage.value = error.response?.data?.detail || t('redeem.failedToRedeem')

    appStore.showError(t('redeem.redeemFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  fetchHistory()
  try {
    const settings = await authAPI.getPublicSettings()
    contactInfo.value = settings.contact_info || ''
  } catch (error) {
    console.error('Failed to load contact info:', error)
  }
})
</script>
