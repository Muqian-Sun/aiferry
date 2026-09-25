<template>
  <!--
    渠道详情抽屉「用量」页签的近 30 天统计：数字摘要（收入 / 成本 / 利润 / 请求）→ 趋势（单色一条线，
    收入 / 请求 / Token 切换）→ 今日与峰值日明细 → 模型 / 端点分布。金额只有收入、成本、利润三个数。
    接口 /admin/accounts/:id/stats 与 models[] 同名同义：actual_cost 是收入，account_cost 是渠道成本。
  -->
  <div class="space-y-8" data-testid="account-usage-panel">
    <div v-if="loading" class="flex items-center justify-center py-12">
      <LoadingSpinner />
    </div>

    <template v-else-if="stats">
      <StatRow :items="summaryItems" data-testid="account-usage-summary" />

      <section data-testid="account-usage-trend">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-af-ink">{{ t('admin.accounts.stats.usageTrend') }}</h3>
          <div class="inline-flex rounded-lg bg-af-sunken p-1" role="tablist" :aria-label="t('admin.accounts.stats.usageTrend')">
            <button
              v-for="tab in trendTabs"
              :key="tab.key"
              type="button"
              role="tab"
              class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
              :class="trendMetric === tab.key ? 'bg-af-sheet text-af-ink' : 'text-af-ink-3 hover:text-af-ink-2'"
              :aria-selected="trendMetric === tab.key"
              @click="trendMetric = tab.key"
            >
              {{ tab.label }}
            </button>
          </div>
        </div>
        <UsageMetricTrend :trend-data="trendPoints" :metric="trendMetric" />
      </section>

      <section class="grid grid-cols-1 gap-x-8 gap-y-6 sm:grid-cols-2" data-testid="account-usage-details">
        <div v-for="group in detailGroups" :key="group.key">
          <h3 class="text-13 font-medium text-af-ink-3">{{ group.title }}</h3>
          <dl class="divide-y divide-af-hairline">
            <DetailField v-for="row in group.rows" :key="row.label" :label="row.label">
              <span :class="['tabular-nums', row.valueClass]">{{ row.value ?? '—' }}</span>
            </DetailField>
          </dl>
        </div>
      </section>

      <ModelDistributionChart :model-stats="stats.models" :loading="false" />
      <EndpointDistributionChart :endpoint-stats="stats.endpoints || []" :loading="false" :title="t('usage.inboundEndpoint')" />
      <EndpointDistributionChart :endpoint-stats="stats.upstream_endpoints || []" :loading="false" :title="t('usage.upstreamEndpoint')" />
    </template>

    <p v-else class="py-12 text-center text-sm text-af-ink-3">{{ t('admin.accounts.stats.noData') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import UsageMetricTrend, { type UsageTrendMetric } from '@/components/user/usage/UsageMetricTrend.vue'
import DetailField from '@/components/admin/list/DetailField.vue'
import { adminAPI } from '@/api/admin'
import { formatMoney, profitOf, profitTextClass } from '@/utils/money'
import type { Account, AccountUsageHistory, AccountUsageStatsResponse, TrendDataPoint } from '@/types'

const props = defineProps<{ account: Account }>()

const { t } = useI18n()

const loading = ref(false)
const stats = ref<AccountUsageStatsResponse | null>(null)

let loadSeq = 0
async function loadStats() {
  const seq = ++loadSeq
  loading.value = true
  try {
    const response = await adminAPI.accounts.getStats(props.account.id, 30)
    if (seq === loadSeq) stats.value = response
  } catch (error) {
    if (seq !== loadSeq) return
    console.error('Failed to load account stats:', error)
    stats.value = null
  } finally {
    if (seq === loadSeq) loading.value = false
  }
}

watch(() => props.account.id, () => void loadStats(), { immediate: true })

// ---------- 格式（金额一律 formatMoney） ----------
const formatCount = (value: number): string => {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}
const formatTokens = (value: number): string => (value >= 1_000_000_000 ? `${(value / 1_000_000_000).toFixed(2)}B` : formatCount(value))
const formatDuration = (ms: number): string =>
  ms >= 1000 ? t('admin.accounts.stats.seconds', { value: (ms / 1000).toFixed(2) }) : t('admin.accounts.stats.milliseconds', { value: Math.round(ms) })

// ---------- 摘要 ----------
const summaryItems = computed<StatItem[]>(() => {
  const s = stats.value?.summary
  if (!s) return []
  const dailyHint = (value: string) => t('admin.accounts.stats.dailyAverage', { value })
  return [
    { key: 'revenue', label: t('common.money.revenue'), value: formatMoney(s.total_actual_cost), hint: dailyHint(formatMoney(s.avg_daily_actual_cost)) },
    { key: 'cost', label: t('common.money.cost'), value: formatMoney(s.total_account_cost), hint: dailyHint(formatMoney(s.avg_daily_account_cost)) },
    { key: 'profit', label: t('common.money.profit'), value: formatMoney(profitOf(s.total_actual_cost, s.total_account_cost)), hint: t('common.money.profitHint') },
    { key: 'requests', label: t('admin.accounts.stats.requests'), value: formatCount(s.total_requests), hint: dailyHint(formatCount(Math.round(s.avg_daily_requests))) }
  ]
})

// ---------- 趋势 ----------
// 趋势组件的 cost 指标画的是点上的 actual_cost，这里就是收入
const trendMetric = ref<UsageTrendMetric>('cost')
const trendTabs = computed<Array<{ key: UsageTrendMetric; label: string }>>(() => [
  { key: 'cost', label: t('common.money.revenue') },
  { key: 'requests', label: t('admin.accounts.stats.requests') },
  { key: 'tokens', label: t('admin.accounts.stats.tokens') }
])
const trendPoints = computed<TrendDataPoint[]>(() =>
  (stats.value?.history ?? []).map((day) => ({
    date: day.date,
    requests: day.requests,
    input_tokens: 0,
    output_tokens: 0,
    cache_creation_tokens: 0,
    cache_read_tokens: 0,
    total_tokens: day.tokens,
    // 趋势组件的点结构要求有标价字段；管理站不显示标价，接口也不再返回，这里不填
    cost: 0,
    actual_cost: day.actual_cost
  }))
)

// ---------- 明细 ----------
interface DetailRow {
  label: string
  value: string | null
  valueClass?: string
}

const moneyRows = (day: AccountUsageHistory | null): DetailRow[] => {
  const revenue = day?.actual_cost ?? 0
  const cost = day?.account_cost ?? 0
  const profit = profitOf(revenue, cost)
  return [
    { label: t('common.money.revenue'), value: formatMoney(revenue) },
    { label: t('common.money.cost'), value: formatMoney(cost) },
    { label: t('common.money.profit'), value: formatMoney(profit), valueClass: profitTextClass(profit) }
  ]
}

const detailGroups = computed(() => {
  const s = stats.value?.summary
  if (!s) return []
  const requests = t('admin.accounts.stats.requests')
  const date = t('admin.accounts.stats.date')
  return [
    {
      key: 'today',
      title: t('admin.accounts.stats.todayOverview'),
      rows: [
        ...moneyRows(s.today),
        { label: requests, value: formatCount(s.today?.requests ?? 0) },
        { label: t('admin.accounts.stats.tokens'), value: formatTokens(s.today?.tokens ?? 0) }
      ]
    },
    {
      key: 'totals',
      title: t('admin.accounts.stats.totalTokens'),
      rows: [
        { label: t('admin.accounts.stats.tokens'), value: formatTokens(s.total_tokens) },
        { label: t('admin.accounts.stats.dailyAvgTokens'), value: formatTokens(Math.round(s.avg_daily_tokens)) },
        { label: t('admin.accounts.stats.avgResponseTime'), value: formatDuration(s.avg_duration_ms) },
        { label: t('admin.accounts.stats.daysActive'), value: `${s.actual_days_used} / ${s.days}` }
      ]
    },
    {
      key: 'highest-revenue',
      title: t('admin.accounts.stats.highestRevenueDay'),
      rows: [
        { label: date, value: s.highest_revenue_day?.label || null },
        ...moneyRows(s.highest_revenue_day),
        { label: requests, value: formatCount(s.highest_revenue_day?.requests ?? 0) }
      ]
    },
    {
      key: 'highest-requests',
      title: t('admin.accounts.stats.highestRequestDay'),
      rows: [
        { label: date, value: s.highest_request_day?.label || null },
        { label: requests, value: formatCount(s.highest_request_day?.requests ?? 0) },
        ...moneyRows(s.highest_request_day)
      ]
    }
  ]
})
</script>
