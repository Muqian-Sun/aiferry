<template>
  <!--
    管理端仪表盘（A2-1，muqian 2026-09-24 定）：
    ① 今日 / 累计两行数字（原 8 张彩色卡片）；右上角实时 RPM · TPM
    ② 需要处理：只放渠道异常 / 限流 / 过载，点进渠道页带状态筛选；都是 0 时整段不出现
    ③ 快捷入口（保留）
    ④ 时间范围 + 粒度；用量趋势单线 + 页签（Token / 请求 / 费用，去掉原双轴图，缓存命中率改成①里的数字）；
       模型分布 / 用户消费榜（表格 + 墨色占比条）；Top 12 用户每人一行迷你柱。全部单色。
  -->
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <section class="card p-5" data-testid="dashboard-numbers">
          <div class="mb-3 flex justify-end">
            <p class="text-xs tabular-nums text-af-ink-3" data-testid="dashboard-realtime">
              {{ t('admin.dashboard.realtime', { rpm: formatNumber(stats.rpm), tpm: formatTokens(stats.tpm) }) }}
            </p>
          </div>
          <div class="divide-y divide-af-hairline">
            <div
              v-for="row in numberRows"
              :key="row.key"
              class="flex flex-col gap-3 py-4 first:pt-0 last:pb-0 lg:flex-row lg:items-start"
              :data-testid="`dashboard-row-${row.key}`"
            >
              <p class="w-16 shrink-0 pt-1 text-13 font-medium text-af-ink-3">{{ row.title }}</p>
              <dl class="grid flex-1 grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-3 lg:grid-cols-6">
                <div v-for="cell in row.cells" :key="cell.key" class="min-w-0">
                  <dd class="truncate text-xl font-semibold tabular-nums text-af-ink">{{ cell.value }}</dd>
                  <dt class="mt-0.5 truncate text-xs text-af-ink-3">
                    {{ cell.label }}<span v-if="cell.hint" class="text-af-ink-4"> · {{ cell.hint }}</span>
                  </dt>
                </div>
              </dl>
            </div>
          </div>
        </section>

        <section v-if="attentionItems.length" class="card p-5" data-testid="dashboard-attention">
          <h2 class="text-sm font-semibold text-af-ink">{{ t('admin.dashboard.attentionTitle') }}</h2>
          <ul class="mt-2 divide-y divide-af-hairline">
            <li v-for="item in attentionItems" :key="item.key">
              <RouterLink
                :to="item.to"
                class="group flex items-center justify-between gap-4 py-2.5 text-sm text-af-ink"
                :data-testid="`dashboard-attention-${item.key}`"
              >
                <span class="flex items-center gap-2.5">
                  <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="item.dot" aria-hidden="true" />
                  {{ item.label }}
                </span>
                <span class="inline-flex items-center gap-1 text-13 text-af-ink-3 group-hover:text-af-ink">
                  {{ t('admin.dashboard.attentionGo') }}
                  <Icon name="chevronRight" size="sm" />
                </span>
              </RouterLink>
            </li>
          </ul>
        </section>

        <div class="space-y-6">
          <div class="card p-4">
            <div class="flex flex-wrap items-center gap-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-af-ink-2">{{ t('admin.dashboard.timeRange') }}:</span>
                <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" @change="onDateRangeChange" />
              </div>
              <button :disabled="chartsLoading" class="btn btn-secondary" @click="loadDashboardStats">
                {{ t('common.refresh') }}
              </button>
              <div class="ml-auto flex items-center gap-2">
                <span class="text-sm font-medium text-af-ink-2">{{ t('admin.dashboard.granularity') }}:</span>
                <div class="w-28">
                  <Select v-model="granularity" :options="granularityOptions" @change="loadChartData" />
                </div>
              </div>
            </div>
          </div>

          <section class="card p-4" data-testid="dashboard-trend">
            <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
              <h3 class="text-sm font-semibold text-af-ink">{{ t('admin.dashboard.usageTrend') }}</h3>
              <div class="inline-flex rounded-lg bg-af-sunken p-1" role="tablist" :aria-label="t('admin.dashboard.usageTrend')">
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
            <UsageMetricTrend :trend-data="trendFilled" :metric="trendMetric" :loading="chartsLoading" />
          </section>

          <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <ModelDistributionChart
              :model-stats="modelStats"
              :enable-ranking-view="true"
              :ranking-items="rankingItems"
              :ranking-total-actual-cost="rankingTotalActualCost"
              :ranking-total-requests="rankingTotalRequests"
              :ranking-total-tokens="rankingTotalTokens"
              :loading="chartsLoading"
              :ranking-loading="rankingLoading"
              :ranking-error="rankingError"
              :start-date="startDate"
              :end-date="endDate"
              :load-user-breakdown="getUserBreakdown"
              @ranking-click="goToUserUsage"
            />
            <section class="card p-4" data-testid="dashboard-top-users">
              <h3 class="mb-4 text-sm font-semibold text-af-ink">{{ t('admin.dashboard.userUsageTrend') }}</h3>
              <UsageModelTrendRows :points="userTrendRows" :days="bucketKeys" :limit="12" :loading="userTrendLoading" />
            </section>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { getUserBreakdown } from '@/api/admin/dashboard'
import type { DashboardStats, TrendDataPoint, ModelStat, UserUsageTrendPoint, UserSpendingRankingItem } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import UsageMetricTrend, { type UsageTrendMetric } from '@/components/user/usage/UsageMetricTrend.vue'
import UsageModelTrendRows from '@/components/user/usage/UsageModelTrendRows.vue'

const { t } = useI18n()
const appStore = useAppStore()
const router = useRouter()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const userTrendLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)

const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
const rankingItems = ref<UserSpendingRankingItem[]>([])
const rankingTotalActualCost = ref(0)
const rankingTotalRequests = ref(0)
const rankingTotalTokens = ref(0)
let chartLoadSeq = 0
let usersTrendLoadSeq = 0
let rankingLoadSeq = 0
const rankingLimit = 12

const pad = (n: number) => String(n).padStart(2, '0')
const formatLocalDate = (date: Date): string => `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const granularity = ref<'day' | 'hour'>('hour')
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)

const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
])

// ---------- ① 今日 / 累计 ----------
const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}
const formatNumber = (value: number | null | undefined): string => toFiniteNumber(value).toLocaleString()
const formatTokens = (value: number | undefined): string => {
  const v = toFiniteNumber(value)
  if (v >= 1_000_000_000) return `${(v / 1_000_000_000).toFixed(2)}B`
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}M`
  if (v >= 1_000) return `${(v / 1_000).toFixed(2)}K`
  return v.toLocaleString()
}
const formatCost = (value: number | null | undefined): string => {
  const v = toFiniteNumber(value)
  if (v >= 1000) return `$${(v / 1000).toFixed(2)}K`
  if (v >= 1) return `$${v.toFixed(2)}`
  if (v >= 0.01) return `$${v.toFixed(3)}`
  return `$${v.toFixed(4)}`
}
const formatDuration = (ms: number): string => (ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`)

/** 缓存命中率 = 缓存读 ÷（输入 + 缓存读 + 缓存写）；四类 token 记账时互斥。没有输入侧 token 时显示「—」 */
const todayCacheHitRate = computed(() => {
  const s = stats.value
  if (!s) return '—'
  const denominator = toFiniteNumber(s.today_input_tokens) + toFiniteNumber(s.today_cache_read_tokens) + toFiniteNumber(s.today_cache_creation_tokens)
  if (denominator <= 0) return '—'
  const share = toFiniteNumber(s.today_cache_read_tokens) / denominator
  return `${(share * 100).toFixed(share > 0 && share < 0.1 ? 1 : 0)}%`
})

interface NumberCell { key: string; label: string; value: string; hint?: string }
const numberRows = computed<Array<{ key: string; title: string; cells: NumberCell[] }>>(() => {
  const s = stats.value
  if (!s) return []
  return [
    {
      key: 'today',
      title: t('admin.dashboard.rowToday'),
      cells: [
        { key: 'requests', label: t('admin.dashboard.requests'), value: formatNumber(s.today_requests) },
        { key: 'tokens', label: t('admin.dashboard.tokens'), value: formatTokens(s.today_tokens) },
        {
          key: 'charged',
          label: t('admin.dashboard.charged'),
          value: formatCost(s.today_actual_cost),
          hint: t('admin.dashboard.standardHint', { amount: formatCost(s.today_cost) })
        },
        { key: 'accountCost', label: t('admin.dashboard.accountCost'), value: formatCost(s.today_account_cost) },
        { key: 'cacheHitRate', label: t('admin.dashboard.cacheHitRate'), value: todayCacheHitRate.value },
        { key: 'latency', label: t('admin.dashboard.avgResponse'), value: formatDuration(toFiniteNumber(s.average_duration_ms)) }
      ]
    },
    {
      key: 'total',
      title: t('admin.dashboard.rowTotal'),
      cells: [
        { key: 'users', label: t('admin.dashboard.users'), value: formatNumber(s.total_users), hint: t('admin.dashboard.newToday', { count: formatNumber(s.today_new_users) }) },
        { key: 'apiKeys', label: t('admin.dashboard.apiKeys'), value: formatNumber(s.total_api_keys), hint: t('admin.dashboard.enabledCount', { count: formatNumber(s.active_api_keys) }) },
        { key: 'channels', label: t('admin.dashboard.channels'), value: formatNumber(s.total_accounts), hint: t('admin.dashboard.healthyCount', { count: formatNumber(s.normal_accounts) }) },
        { key: 'requests', label: t('admin.dashboard.requests'), value: formatNumber(s.total_requests) },
        { key: 'tokens', label: t('admin.dashboard.tokens'), value: formatTokens(s.total_tokens) },
        { key: 'charged', label: t('admin.dashboard.charged'), value: formatCost(s.total_actual_cost) }
      ]
    }
  ]
})

// ---------- ② 需要处理 ----------
const attentionItems = computed(() => {
  const s = stats.value
  if (!s) return []
  const items = [
    { key: 'error', count: toFiniteNumber(s.error_accounts), label: 'attentionError', dot: 'bg-af-danger', to: { path: '/accounts', query: { status: 'error' } } },
    { key: 'rate-limited', count: toFiniteNumber(s.ratelimit_accounts), label: 'attentionRateLimited', dot: 'bg-af-warning', to: { path: '/accounts', query: { status: 'rate_limited' } } },
    // 过载没有对应的状态筛选，只跳渠道页
    { key: 'overloaded', count: toFiniteNumber(s.overload_accounts), label: 'attentionOverloaded', dot: 'bg-af-warning', to: { path: '/accounts' } }
  ]
  return items
    .filter((item) => item.count > 0)
    .map((item) => ({ ...item, label: t(`admin.dashboard.${item.label}`, { count: formatNumber(item.count) }) }))
})

// ---------- ④ 趋势 / 分布 / Top 用户 ----------
const trendMetric = ref<UsageTrendMetric>('tokens')
const trendTabs = computed<Array<{ key: UsageTrendMetric; label: string }>>(() => [
  { key: 'tokens', label: t('admin.dashboard.tokens') },
  { key: 'requests', label: t('admin.dashboard.requests') },
  { key: 'cost', label: t('admin.dashboard.cost') }
])

/** 区间内逐个时间桶（与后端 TO_CHAR 同格式）：按天 YYYY-MM-DD；按小时 YYYY-MM-DD HH:00，截到当前小时 */
const bucketKeys = computed(() => {
  const start = new Date(`${startDate.value}T00:00:00`)
  const end = new Date(`${endDate.value}T00:00:00`)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return []
  const keys: string[] = []
  if (granularity.value === 'day') {
    for (const d = new Date(start); d <= end; d.setDate(d.getDate() + 1)) keys.push(formatLocalDate(d))
    return keys
  }
  const last = Math.min(end.getTime() + 23 * 60 * 60 * 1000, Date.now())
  for (const d = new Date(start); d.getTime() <= last; d.setHours(d.getHours() + 1)) {
    keys.push(`${formatLocalDate(d)} ${pad(d.getHours())}:00`)
  }
  return keys
})

/** 趋势按连续时间桶补零：接口只返回有请求的桶，直接连线会把中间没请求的时段连过去 */
const trendFilled = computed<TrendDataPoint[]>(() => {
  const byDate = new Map(trendData.value.map((point) => [point.date, point]))
  return bucketKeys.value.map(
    (date) =>
      byDate.get(date) ?? {
        date,
        requests: 0,
        input_tokens: 0,
        output_tokens: 0,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        total_tokens: 0,
        cost: 0,
        actual_cost: 0
      }
  )
})

const userDisplayName = (point: UserUsageTrendPoint): string =>
  point.username?.trim() || point.email?.trim() || t('admin.redeem.userPrefix', { id: point.user_id })

/** Top 12 用户：按用户 id 分组（同名不合并），显示名字；值用 Token */
const userTrendRows = computed(() =>
  userTrend.value.map((point) => ({
    date: point.date,
    model: String(point.user_id),
    label: userDisplayName(point),
    requests: point.requests,
    total_tokens: point.tokens
  }))
)

const goToUserUsage = (item: UserSpendingRankingItem) => {
  void router.push({
    path: '/usage',
    query: { user_id: String(item.user_id), start_date: startDate.value, end_date: endDate.value }
  })
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  const start = new Date(range.startDate)
  const end = new Date(range.endDate)
  const daysDiff = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))
  granularity.value = daysDiff <= 1 ? 'hour' : 'day'
  loadChartData()
}

const loadDashboardSnapshot = async (includeStats: boolean) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) loading.value = true
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      include_stats: includeStats,
      include_trend: true,
      include_model_stats: true,
      include_users_trend: false
    })
    if (currentSeq !== chartLoadSeq) return
    if (includeStats && response.stats) stats.value = response.stats
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

const loadUsersTrend = async () => {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      limit: 12
    })
    if (currentSeq !== usersTrendLoadSeq) return
    userTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== usersTrendLoadSeq) return
    console.error('Error loading users trend:', error)
    userTrend.value = []
  } finally {
    if (currentSeq === usersTrendLoadSeq) userTrendLoading.value = false
  }
}

const loadUserSpendingRanking = async () => {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      start_date: startDate.value,
      end_date: endDate.value,
      limit: rankingLimit
    })
    if (currentSeq !== rankingLoadSeq) return
    rankingItems.value = response.ranking || []
    rankingTotalActualCost.value = response.total_actual_cost || 0
    rankingTotalRequests.value = response.total_requests || 0
    rankingTotalTokens.value = response.total_tokens || 0
  } catch (error) {
    if (currentSeq !== rankingLoadSeq) return
    console.error('Error loading user spending ranking:', error)
    rankingItems.value = []
    rankingTotalActualCost.value = 0
    rankingTotalRequests.value = 0
    rankingTotalTokens.value = 0
    rankingError.value = true
  } finally {
    if (currentSeq === rankingLoadSeq) rankingLoading.value = false
  }
}

const loadDashboardStats = async () => {
  await Promise.all([loadDashboardSnapshot(true), loadUsersTrend(), loadUserSpendingRanking()])
}

const loadChartData = async () => {
  await Promise.all([loadDashboardSnapshot(false), loadUsersTrend(), loadUserSpendingRanking()])
}

onMounted(() => {
  loadDashboardStats()
})
</script>
