<template>
  <!--
    渠道详情抽屉「用量」页签（A7，取代原 AccountStatsModal 的对话框 / 卡片形态）：近 30 天。
    数字摘要一行 → 趋势（单色一条线，Token / 请求 / 费用切换，不再是三色双轴）→ 今日与峰值日明细 → 模型 / 端点分布。不套卡片。
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
            <DetailField v-for="row in group.rows" :key="row.label" :label="row.label" :value="row.value" />
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
import type { Account, AccountUsageStatsResponse, TrendDataPoint } from '@/types'

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

// ---------- 格式 ----------
const formatCost = (value: number): string => {
  if (value >= 1000) return `$${(value / 1000).toFixed(2)}K`
  if (value >= 1) return `$${value.toFixed(2)}`
  if (value >= 0.01) return `$${value.toFixed(3)}`
  return `$${value.toFixed(4)}`
}
const formatCount = (value: number): string => {
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}
const formatTokens = (value: number): string => (value >= 1_000_000_000 ? `${(value / 1_000_000_000).toFixed(2)}B` : formatCount(value))
const formatDuration = (ms: number): string => (ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`)

// ---------- 摘要 ----------
const summaryItems = computed<StatItem[]>(() => {
  const s = stats.value?.summary
  if (!s) return []
  return [
    {
      key: 'cost',
      label: t('admin.accounts.stats.totalCost'),
      value: formatCost(s.total_cost),
      hint: `${t('usage.userBilled')} ${formatCost(s.total_user_cost)} · ${t('admin.accounts.stats.standardCost')} ${formatCost(s.total_standard_cost)}`
    },
    { key: 'requests', label: t('admin.accounts.stats.totalRequests'), value: formatCount(s.total_requests) },
    {
      key: 'avg-cost',
      label: t('admin.accounts.stats.avgDailyCost'),
      value: formatCost(s.avg_daily_cost),
      hint: t('admin.accounts.stats.basedOnActualDays', { days: s.actual_days_used })
    },
    { key: 'avg-requests', label: t('admin.accounts.stats.avgDailyRequests'), value: formatCount(Math.round(s.avg_daily_requests)) }
  ]
})

// ---------- 趋势 ----------
const trendMetric = ref<UsageTrendMetric>('cost')
const trendTabs = computed<Array<{ key: UsageTrendMetric; label: string }>>(() => [
  { key: 'cost', label: t('admin.accounts.stats.cost') },
  { key: 'requests', label: t('admin.accounts.stats.requests') },
  { key: 'tokens', label: t('admin.accounts.stats.tokens') }
])
/** 按天的历史换成趋势图的点；费用取渠道成本（account 倍率后） */
const trendPoints = computed<TrendDataPoint[]>(() =>
  (stats.value?.history ?? []).map((day) => ({
    date: day.date,
    requests: day.requests,
    input_tokens: 0,
    output_tokens: 0,
    cache_creation_tokens: 0,
    cache_read_tokens: 0,
    total_tokens: day.tokens,
    cost: day.cost,
    actual_cost: day.actual_cost
  }))
)

// ---------- 明细 ----------
const detailGroups = computed(() => {
  const s = stats.value?.summary
  if (!s) return []
  const accountBilled = t('usage.accountBilled')
  const userBilled = t('usage.userBilled')
  const requests = t('admin.accounts.stats.requests')
  const date = t('admin.accounts.stats.date')
  return [
    {
      key: 'today',
      title: t('admin.accounts.stats.todayOverview'),
      rows: [
        { label: accountBilled, value: formatCost(s.today?.cost ?? 0) },
        { label: userBilled, value: formatCost(s.today?.user_cost ?? 0) },
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
      key: 'highest-cost',
      title: t('admin.accounts.stats.highestCostDay'),
      rows: [
        { label: date, value: s.highest_cost_day?.label || null },
        { label: accountBilled, value: formatCost(s.highest_cost_day?.cost ?? 0) },
        { label: userBilled, value: formatCost(s.highest_cost_day?.user_cost ?? 0) },
        { label: requests, value: formatCount(s.highest_cost_day?.requests ?? 0) }
      ]
    },
    {
      key: 'highest-requests',
      title: t('admin.accounts.stats.highestRequestDay'),
      rows: [
        { label: date, value: s.highest_request_day?.label || null },
        { label: requests, value: formatCount(s.highest_request_day?.requests ?? 0) },
        { label: accountBilled, value: formatCost(s.highest_request_day?.cost ?? 0) },
        { label: userBilled, value: formatCost(s.highest_request_day?.user_cost ?? 0) }
      ]
    }
  ]
})
</script>
