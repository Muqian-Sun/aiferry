<template>
  <!--
    渠道列表「今日」列：一行「N 次 · 收入 $x · 利润 $y」。收入 = 用户付的钱，利润 = 收入 − 渠道成本，亏本标红。
    数据是列表批量拉的今日统计（window_stats：user_cost 是收入，cost 是渠道成本）。
  -->
  <div v-if="loading && !stats" class="h-3 w-40 animate-pulse rounded bg-af-hairline"></div>
  <span v-else-if="error && !stats" class="text-xs text-af-danger">{{ error }}</span>
  <!-- 间距用 gap 定，不靠模板里的空白（跨行的空白文本节点会被 Vue 压掉，间距就时有时无） -->
  <span
    v-else-if="stats && stats.requests > 0"
    class="inline-flex items-baseline gap-1.5 whitespace-nowrap text-sm tabular-nums text-af-ink-2"
    :title="t('admin.accounts.today.tooltip', { cost: formatMoney(cost) })"
    data-testid="account-today"
  >
    <span>{{ t('admin.accounts.today.requests', { count: formatNumber(stats.requests) }) }}</span>
    <span class="text-af-ink-3" aria-hidden="true">·</span>
    <span class="inline-flex items-baseline gap-1">
      <span class="text-af-ink-3">{{ t('common.money.revenue') }}</span>
      <span>{{ formatMoney(revenue) }}</span>
    </span>
    <span class="text-af-ink-3" aria-hidden="true">·</span>
    <span class="inline-flex items-baseline gap-1">
      <span class="text-af-ink-3">{{ t('common.money.profit') }}</span>
      <span :class="profitTextClass(profit)" data-testid="account-today-profit">{{ formatMoney(profit) }}</span>
    </span>
  </span>
  <span v-else class="text-sm text-af-ink-3">—</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WindowStats } from '@/types'
import { formatNumber } from '@/utils/format'
import { formatMoney, profitOf, profitTextClass } from '@/utils/money'

const props = withDefaults(
  defineProps<{
    stats?: WindowStats | null
    loading?: boolean
    error?: string | null
  }>(),
  { stats: null, loading: false, error: null }
)

const { t } = useI18n()

const revenue = computed(() => props.stats?.user_cost ?? 0)
const cost = computed(() => props.stats?.cost ?? 0)
const profit = computed(() => profitOf(revenue.value, cost.value))
</script>
