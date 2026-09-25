<template>
  <!--
    收款概览的四个数字（A4）：与用户站控制台同一个 StatRow——行内大数字、竖 hairline 分隔，不再一格一张卡片。
    金额可能有多个币种：大数字只写第一个币种（按币种代码排序），其余币种写进旁边的小字，免得一格折成两行、把订单数挤掉。
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

/** 第一个币种进大数字，其余币种与额外说明一起进小字；没有任何币种的收入时写「—」 */
function splitAmounts(amounts: CurrencyAmounts | null | undefined, extra: string[] = []): { value: string; hint?: string } {
  const [first, ...rest] = sortedAmounts(amounts ?? {}).map(([currency, amount]) => formatMoney(currency, amount))
  const hint = [...rest, ...extra].join(' · ')
  return { value: first ?? '—', hint: hint || undefined }
}

const items = computed<StatItem[]>(() => {
  const stats = props.stats
  return [
    {
      key: 'today-revenue',
      label: t('payment.admin.todayRevenue'),
      ...splitAmounts(stats.today_amount, [`${stats.today_count} ${t('payment.admin.orders')}`])
    },
    {
      key: 'total-revenue',
      label: t('payment.admin.totalRevenue'),
      ...splitAmounts(stats.total_amount, [`${stats.total_count} ${t('payment.admin.orders')}`])
    },
    { key: 'today-orders', label: t('payment.admin.todayOrders'), value: String(stats.today_count) },
    { key: 'avg-amount', label: t('payment.admin.avgAmount'), ...splitAmounts(stats.avg_amount) }
  ]
})
</script>
