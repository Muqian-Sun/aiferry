<template>
  <!--
    管理端概览（A2-1 定内容，A7 改成一张面，瘦身方案 2026-09-25 定数字）：
    ① 今日：请求 / Token / 收入 / 成本 / 利润；累计：收入 / 成本 / 利润 / 用户 / 渠道；右上角实时 RPM · TPM
       （缓存命中率、平均响应挪到用量页；平均响应原先排在今日行、算的却是全部历史）
    ② 需要处理：只放渠道异常 / 限流 / 过载，点进渠道页带状态筛选；都是 0 时整段不出现
    ③ 用量趋势单线 + 页签（Token / 请求 / 收入 / 利润）；模型用量（按模型分色的堆叠柱 + 模型表）；用户用量（每人一条热力条 + 用户表）。
       2026-09-29 muqian：重点看每天的 Token 量与走势、各模型走势、每天哪个模型用得最多、哪个用户用得最多与各用户走势，
       默认近 7 天按天；模型柱状图按他的要求用分类色区分模型，其余仍单色，利润为负时标红。累计行渠道的附注「N 可调度」和渠道页摘要是同一个数（normal_accounts），叫法保持一致。区块之间只用 hairline 分隔，不套卡片；时间范围与粒度在页头，只作用于③（①②是今日 / 累计 / 当前状态）。
  -->
  <AppLayout>
    <template #header-actions>
      <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" :preset="datePreset" @change="onDateRangeChange" />
      <SegmentedControl :model-value="granularity" :options="granularityOptions" :label="t('admin.dashboard.granularity')" @update:model-value="onGranularityChange" />
      <button
        type="button"
        class="btn btn-ghost btn-sm h-8 px-2"
        :disabled="chartsLoading"
        :title="t('common.refresh')"
        :aria-label="t('common.refresh')"
        data-testid="dashboard-refresh"
        @click="loadDashboardStats"
      >
        <Icon name="refresh" size="md" :class="loading || chartsLoading ? 'animate-spin' : ''" />
      </button>
    </template>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <LoadingSpinner />
    </div>

    <div v-else-if="stats" class="space-y-8">
      <section data-testid="dashboard-numbers">
        <div class="divide-y divide-af-hairline">
          <div
            v-for="row in numberRows"
            :key="row.key"
            class="flex flex-col gap-3 py-4 first:pt-0 last:pb-0"
            :data-testid="`dashboard-row-${row.key}`"
          >
            <!-- 行标题；实时 RPM · TPM 跟在「今日」这一行的右侧（原来单独一行挂在右上角） -->
            <div class="flex items-baseline justify-between gap-4">
              <p class="text-13 font-medium text-af-ink-2">{{ row.title }}</p>
              <p v-if="row.key === 'today'" class="text-xs tabular-nums text-af-ink-3" data-testid="dashboard-realtime">
                {{ t('admin.dashboard.realtime', { rpm: formatNumber(stats.rpm), tpm: formatTokens(stats.tpm) }) }}
              </p>
            </div>
            <!-- 标签在上、数字在下：与用量页、各列表页的数字带同一种排法（原来这里数字在上） -->
            <dl class="grid flex-1 grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-3 lg:grid-cols-5">
              <div v-for="cell in row.cells" :key="cell.key" class="flex min-w-0 flex-col">
                <dt class="truncate text-13 text-af-ink-3" :title="cell.title">
                  {{ cell.label }}<span v-if="cell.hint"> · {{ cell.hint }}</span>
                </dt>
                <dd class="mt-1 truncate text-xl font-semibold tabular-nums" :class="cell.valueClass || 'text-af-ink'">{{ cell.value }}</dd>
              </div>
            </dl>
          </div>
        </div>
      </section>

      <SheetSection v-if="attentionItems.length" :title="t('admin.dashboard.attentionTitle')" data-testid="dashboard-attention">
        <ul class="-mt-2 divide-y divide-af-hairline">
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
      </SheetSection>

      <SheetSection :title="t('admin.dashboard.usageTrend')" data-testid="dashboard-trend">
        <template #actions>
          <SegmentedControl v-model="trendMetric" :options="trendTabs" :label="t('admin.dashboard.usageTrend')" />
        </template>
        <UsageMetricTrend :trend-data="trendFilled" :metric="trendMetric" :loading="chartsLoading" />
      </SheetSection>

      <SheetSection :title="t('admin.dashboard.modelSection')" :description="t('admin.dashboard.modelSectionHint')" data-testid="dashboard-models">
        <ModelTokenTrendChart :points="modelTrend" :days="bucketKeys" :series="modelSeries" :loading="chartsLoading" />
        <DashboardModelTable
          class="mt-6"
          :model-stats="modelStats"
          :series="modelSeries"
          :load-user-breakdown="getUserBreakdown"
          :range="rangeQuery"
          :loading="chartsLoading"
        />
      </SheetSection>

      <SheetSection :title="t('admin.dashboard.userSection')" :description="t('admin.dashboard.userSectionHint')" data-testid="dashboard-users">
        <DashboardUserTable :points="userTrend" :days="bucketKeys" :total-tokens="rangeTotalTokens" :loading="chartsLoading" @select="goToUserUsage" />
      </SheetSection>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import { getUserBreakdown } from '@/api/admin/dashboard'
import type { DashboardStats, TrendDataPoint, ModelStat, ModelTrendPoint, UserUsageTrendPoint } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import UsageMetricTrend, { type UsageTrendMetric } from '@/components/user/usage/UsageMetricTrend.vue'
import ModelTokenTrendChart from '@/components/admin/dashboard/ModelTokenTrendChart.vue'
import DashboardModelTable from '@/components/admin/dashboard/DashboardModelTable.vue'
import DashboardUserTable from '@/components/admin/dashboard/DashboardUserTable.vue'
import { splitModelSeries } from '@/components/admin/dashboard/modelSeries'
import { fillTrendBuckets, formatLocalDate, trendBucketKeys, trendBucketKeysBetween, type TrendGranularity } from '@/utils/trendBuckets'
import { rangeParams, windowForPreset } from '@/utils/dateRange'
import { formatMoney, profitOf, profitTextClass } from '@/utils/money'

const { t } = useI18n()
const router = useRouter()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)

const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const modelTrend = ref<ModelTrendPoint[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
let chartLoadSeq = 0
/** 用户表最多取多少人（后端按区间 Token 取前 N，上限 50）；默认显示前 10，其余点「显示全部」 */
const USERS_TREND_LIMIT = 50

/** 默认近 7 天（与日期选择器「近 7 天」预设同一算法：今天往前 6 天），按天看趋势 */
const getLast7DaysRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - 6)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const granularity = ref<TrendGranularity>('day')
const defaultRange = getLast7DaysRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
// 默认是按天的近 7 天（null：日期选择器按日期认出预设）
const datePreset = ref<string | null>(null)
// 近 24 小时按精确时刻查，窗口在每次加载图表时按此刻重算；按天的范围为 null
const timeWindow = ref(windowForPreset(datePreset.value))
const rangeQuery = computed(() => rangeParams(startDate.value, endDate.value, timeWindow.value))

const granularityOptions = computed<Array<{ key: TrendGranularity; label: string }>>(() => [
  { key: 'day', label: t('admin.dashboard.day') },
  { key: 'hour', label: t('admin.dashboard.hour') }
])
const onGranularityChange = (value: TrendGranularity) => {
  granularity.value = value
  loadChartData()
}

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
interface NumberCell { key: string; label: string; value: string; title?: string; hint?: string; valueClass?: string }

/** 收入 / 成本 / 利润三格（今日、累计各一组）；利润为负时标红 */
const moneyCells = (revenue: number, cost: number): NumberCell[] => {
  const profit = profitOf(revenue, cost)
  return [
    { key: 'revenue', label: t('common.money.revenue'), value: formatMoney(revenue) },
    { key: 'cost', label: t('common.money.cost'), value: formatMoney(cost) },
    { key: 'profit', label: t('common.money.profit'), title: t('common.money.profitHint'), value: formatMoney(profit), valueClass: profitTextClass(profit) }
  ]
}

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
        ...moneyCells(s.today_actual_cost, s.today_account_cost)
      ]
    },
    {
      key: 'total',
      title: t('admin.dashboard.rowTotal'),
      cells: [
        ...moneyCells(s.total_actual_cost, s.total_account_cost),
        { key: 'users', label: t('admin.dashboard.users'), value: formatNumber(s.total_users) },
        { key: 'channels', label: t('admin.dashboard.channels'), value: formatNumber(s.total_accounts), hint: t('admin.dashboard.schedulableCount', { count: formatNumber(s.normal_accounts) }) }
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

// ---------- ③ 趋势 / 分布 / Top 用户 ----------
const trendMetric = ref<UsageTrendMetric>('tokens')
const trendTabs = computed<Array<{ key: UsageTrendMetric; label: string }>>(() => [
  { key: 'tokens', label: t('admin.dashboard.tokens') },
  { key: 'requests', label: t('admin.dashboard.requests') },
  { key: 'revenue', label: t('common.money.revenue') },
  { key: 'profit', label: t('common.money.profit') }
])

const bucketKeys = computed(() =>
  timeWindow.value
    ? trendBucketKeysBetween(new Date(timeWindow.value.start_time), new Date(timeWindow.value.end_time), granularity.value)
    : trendBucketKeys(startDate.value, endDate.value, granularity.value)
)
const trendFilled = computed(() => fillTrendBuckets(trendData.value, bucketKeys.value))

/** 模型配色：前 8 个模型各一色，其余并进「其他」；柱状图与模型表共用 */
const modelSeries = computed(() => splitModelSeries(modelStats.value))
/** 区间内全站 Token（用户表占比的分母，不是前 N 名之和） */
const rangeTotalTokens = computed(() => trendData.value.reduce((sum, point) => sum + toFiniteNumber(point.total_tokens), 0))

/** 跳到用量页看这个人：按天的范围原样带过去；近 24 小时不带日期，用量页默认就是近 24 小时 */
const goToUserUsage = (userId: number) => {
  void router.push({
    path: '/usage',
    query: { user_id: String(userId), ...(timeWindow.value ? {} : { start_date: startDate.value, end_date: endDate.value }) }
  })
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  datePreset.value = range.preset
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
  timeWindow.value = windowForPreset(datePreset.value)
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      ...rangeQuery.value,
      granularity: granularity.value,
      include_stats: includeStats,
      include_trend: true,
      include_model_stats: true,
      include_model_trend: true,
      include_users_trend: true,
      users_trend_limit: USERS_TREND_LIMIT
    })
    if (currentSeq !== chartLoadSeq) return
    if (includeStats && response.stats) stats.value = response.stats
    trendData.value = response.trend || []
    modelStats.value = response.models || []
    modelTrend.value = response.model_trend || []
    userTrend.value = response.users_trend || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

const loadDashboardStats = () => loadDashboardSnapshot(true)

const loadChartData = () => loadDashboardSnapshot(false)

onMounted(() => {
  loadDashboardStats()
})
</script>
