<template>
  <!--
    收款概览的四个数字（A4）：与用户站控制台同一个 StatRow——行内大数字、竖 hairline 分隔，不再一格一张卡片。
    金额可能有多个币种，同一格里用「·」连起来。
  -->
  <StatRow :items="items" data-testid="order-stats" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import type { CurrencyAmounts, DashboardStats } from '@/types/payment'

const { t } = useI18n()

const props = defineProps<{
  stats: DashboardStats
}>()

function sortedAmounts(amounts: CurrencyAmounts): [string, number][] {
  return Object.entries(amounts).sort(([left], [right]) => left.localeCompare(right))
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}

/** 多币种金额拼成一串；没有任何币种的收入时写「—」 */
function formatAmounts(amounts: CurrencyAmounts | null | undefined): string {
  const entries = sortedAmounts(amounts ?? {})
  return entries.length ? entries.map(([currency, amount]) => formatMoney(currency, amount)).join(' · ') : '—'
}

const items = computed<StatItem[]>(() => {
  const stats = props.stats
  return [
    {
      key: 'today-revenue',
      label: t('payment.admin.todayRevenue'),
      value: formatAmounts(stats.today_amount),
      hint: `${stats.today_count} ${t('payment.admin.orders')}`
    },
    {
      key: 'total-revenue',
      label: t('payment.admin.totalRevenue'),
      value: formatAmounts(stats.total_amount),
      hint: `${stats.total_count} ${t('payment.admin.orders')}`
    },
    { key: 'today-orders', label: t('payment.admin.todayOrders'), value: String(stats.today_count) },
    { key: 'avg-amount', label: t('payment.admin.avgAmount'), value: formatAmounts(stats.avg_amount) }
  ]
})
</script>
