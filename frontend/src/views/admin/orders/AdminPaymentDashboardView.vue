<template>
  <!--
    收款概览（A7 改成一张面）：四个数字（StatRow）→ 每日收款（单色线，按币种 / 订单数切换）→ 支付方式 · 充值排行。
    天数切换与刷新在页头；区块之间只用 hairline 分隔，不套卡片，排名与支付方式不再用颜色区分。
  -->
  <AppLayout>
    <template #header-actions>
      <SegmentedControl v-model="days" :options="dayOptions" :label="t('payment.admin.dailyRevenue')" />
      <button
        type="button"
        class="btn btn-ghost btn-md px-2.5"
        :disabled="loading"
        :title="t('common.refresh')"
        :aria-label="t('common.refresh')"
        @click="loadDashboard"
      >
        <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
      </button>
    </template>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <LoadingSpinner />
    </div>
    <div v-else-if="stats" class="space-y-8">
      <OrderStatsCards :stats="stats" />
      <DailyRevenueChart :data="stats.daily_series || []" :loading="loading" />
      <section class="grid grid-cols-1 gap-x-10 gap-y-8 border-t border-af-hairline pt-6 lg:grid-cols-2">
        <div data-testid="payment-methods">
          <h3 class="mb-4 text-base font-semibold text-af-ink">{{ t('payment.admin.paymentDistribution') }}</h3>
          <div v-if="!stats.payment_methods?.length" class="flex h-32 items-center justify-center text-sm text-af-ink-3">{{ t('payment.admin.noData') }}</div>
          <table v-else class="w-full text-sm">
            <tbody>
              <tr v-for="method in stats.payment_methods" :key="method.type" class="border-t border-af-hairline first:border-t-0">
                <td class="py-2 pr-3 text-af-ink-2">{{ t('payment.methods.' + method.type, method.type) }}</td>
                <td class="w-40 py-2 pr-3 text-xs">
                  <ShareBar :value="method.count" :total="methodCountTotal" />
                </td>
                <td class="py-2 text-right tabular-nums">
                  <span v-for="[currency, amount] in sortedAmounts(method.amount)" :key="currency" class="block font-medium text-af-ink">{{ formatMoney(currency, amount) }}</span>
                  <span class="text-xs text-af-ink-3">{{ method.count }} {{ t('payment.admin.orders') }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div data-testid="payment-top-users">
          <h3 class="mb-4 text-base font-semibold text-af-ink">{{ t('payment.admin.topUsers') }}</h3>
          <div v-if="!hasTopUsers(stats.top_users)" class="flex h-32 items-center justify-center text-sm text-af-ink-3">{{ t('payment.admin.noData') }}</div>
          <div v-else class="space-y-4">
            <div v-for="[currency, users] in sortedTopUsers(stats.top_users)" :key="currency">
              <p v-if="Object.keys(stats.top_users).length > 1" class="mb-1 text-xs font-semibold text-af-ink-3">{{ currency }}</p>
              <ol class="divide-y divide-af-hairline">
                <li v-for="(user, idx) in users" :key="user.user_id" class="flex items-center justify-between gap-3 py-2 text-sm">
                  <span class="flex min-w-0 items-center gap-3">
                    <span class="w-6 shrink-0 text-xs font-semibold tabular-nums text-af-ink-3">#{{ idx + 1 }}</span>
                    <span class="truncate text-af-ink-2">{{ user.email }}</span>
                  </span>
                  <span class="shrink-0 font-medium tabular-nums text-af-ink">{{ formatMoney(currency, user.amount) }}</span>
                </li>
              </ol>
            </div>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import { computed, ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { CurrencyAmounts, DashboardStats, TopUserPaymentStats } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderStatsCards from '@/components/admin/payment/OrderStatsCards.vue'
import DailyRevenueChart from '@/components/admin/payment/DailyRevenueChart.vue'
import ShareBar from '@/components/charts/ShareBar.vue'

const { t } = useI18n()

const DAYS_OPTIONS = [7, 30, 90] as const
const dayOptions = computed(() => DAYS_OPTIONS.map((d) => ({ key: d, label: `${d}${t('payment.admin.daySuffix')}` })))
const days = ref<number>(30)
const loading = ref(false)
const stats = ref<DashboardStats | null>(null)

/** 支付方式占比按笔数算（金额可能分属多个币种，没法直接相加） */
const methodCountTotal = computed(() => (stats.value?.payment_methods ?? []).reduce((sum, method) => sum + method.count, 0))

function sortedAmounts(amounts: CurrencyAmounts): [string, number][] {
  return Object.entries(amounts).sort(([left], [right]) => left.localeCompare(right))
}

function sortedTopUsers(usersByCurrency: Record<string, TopUserPaymentStats[]>): [string, TopUserPaymentStats[]][] {
  return Object.entries(usersByCurrency).sort(([left], [right]) => left.localeCompare(right))
}

function hasTopUsers(usersByCurrency: Record<string, TopUserPaymentStats[]>): boolean {
  return Object.values(usersByCurrency).some(users => users.length > 0)
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}

async function loadDashboard() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getDashboard(days.value)
    stats.value = res.data
  } catch (err: unknown) {
    console.error(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')), err)
  } finally {
    loading.value = false
  }
}

watch(days, () => loadDashboard())
onMounted(() => loadDashboard())
</script>
