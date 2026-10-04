<template>
  <!--
    单指标趋势（Token / 请求 / 费用 / 收入 / 利润）：一份 trend 数据，只画一条墨色线 + 很淡的面积（利润为负时线走到 0 以下）。
    控制台配色单色为主（muqian 2026-09-23），不再用分类彩色；两站共用：用户站概览、密钥详情，管理站概览、用量页、渠道抽屉。
  -->
  <div v-if="loading" class="flex h-48 items-center justify-center">
    <span class="spinner text-af-ink-3" />
  </div>
  <div v-else-if="chartData" class="h-48">
    <Line :data="chartData" :options="lineOptions" />
  </div>
  <div v-else class="flex h-48 items-center justify-center text-sm text-af-ink-3">
    {{ t('userUi.usage.trend.empty') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler, Ticks } from 'chart.js'
import type { Scale, Tick } from 'chart.js'
import { Line } from 'vue-chartjs'
import { useChartTheme } from '@/composables/useChartTheme'
import { formatNumber, formatTokensK } from '@/utils/format'
import { formatMoney, profitOf } from '@/utils/money'
import type { TrendDataPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

/**
 * cost = 用户站「费用」（用户实付 actual_cost）；revenue = 管理站「收入」（同一个 actual_cost，站在平台这边叫法不同）；
 * profit = 利润（actual_cost − account_cost，只有管理端的趋势数据带 account_cost）。
 */
export type UsageTrendMetric = 'tokens' | 'requests' | 'cost' | 'revenue' | 'profit'

const props = defineProps<{
  trendData: TrendDataPoint[]
  metric: UsageTrendMetric
  loading?: boolean
}>()

const { t } = useI18n()
const theme = useChartTheme()

/** 横轴去掉年份：按天 YYYY-MM-DD → MM-DD；按小时 YYYY-MM-DD HH:00 → MM-DD HH:00 */
function shortLabel(date: string): string {
  return /^\d{4}-\d{2}-\d{2}(?: \d{2}:00)?$/.test(date) ? date.slice(5) : date
}

const labelKeys: Record<UsageTrendMetric, string> = {
  tokens: 'userUi.usage.trend.tokens',
  requests: 'userUi.usage.trend.requests',
  cost: 'userUi.usage.trend.cost',
  revenue: 'common.money.revenue',
  profit: 'common.money.profit'
}
const metricLabel = computed(() => t(labelKeys[props.metric]))

function valueOf(point: TrendDataPoint): number {
  switch (props.metric) {
    case 'tokens':
      return point.total_tokens
    case 'requests':
      return point.requests
    case 'profit':
      return profitOf(point.actual_cost, point.account_cost)
    default:
      return point.actual_cost
  }
}

const isMoneyMetric = computed(() => props.metric !== 'tokens' && props.metric !== 'requests')

function formatValue(value: number): string {
  if (props.metric === 'tokens') return formatTokensK(value)
  return props.metric === 'requests' ? formatNumber(value) : formatMoney(value)
}

/**
 * 金额纵轴刻度：小数位跟着刻度间隔走（间隔 0.002 就写到 3 位），用 Chart.js 自带的数值刻度格式化 + 美元格式（ticks.format）。
 * 不能用汇总金额的 formatMoney：小额区间里每个刻度都是同一个 `<$0.01`，轴上看不出大小（2026-10-04 走查：利润轴全是 `<$0.0`）。
 */
function moneyTick(this: Scale, value: string | number, index: number, ticks: Tick[]): string {
  const v = Number(value)
  return v === 0 ? '$0' : Ticks.formatters.numeric.call(this, v, index, ticks)
}

/** 只有一个时间桶（如「今天」按天）时折线画不出线，要把这一个点画出来 */
const singlePoint = computed(() => props.trendData?.length === 1)

const chartData = computed(() => {
  if (!props.trendData?.length) return null
  return {
    labels: props.trendData.map((d) => shortLabel(d.date)),
    datasets: [
      {
        label: metricLabel.value,
        data: props.trendData.map(valueOf),
        borderColor: theme.value.ink,
        backgroundColor: theme.value.inkFill,
        borderWidth: 2,
        pointRadius: singlePoint.value ? 4 : 0,
        pointBackgroundColor: theme.value.ink,
        pointHoverRadius: 4,
        pointHoverBackgroundColor: theme.value.ink,
        pointHitRadius: 8,
        fill: 'origin',
        // 单调插值：普通 tension 在很多 0 的稀疏数据两侧会冲到 0 以下
        cubicInterpolationMode: 'monotone' as const
      }
    ]
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  // 刻度数字的格式与汇总金额（formatMoney）一致按 en-US，不随浏览器语言变成「US$」
  locale: 'en-US',
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: { parsed: { y: number | null } }) =>
          `${metricLabel.value}: ${formatValue(context.parsed.y ?? 0)}`
      }
    }
  },
  scales: {
    x: {
      // 单个点时类目轴两侧留半格，点落在正中而不是贴着左边
      offset: singlePoint.value,
      grid: { display: false },
      ticks: { color: theme.value.text, maxTicksLimit: 12, font: { size: 10 } }
    },
    y: {
      beginAtZero: true,
      grid: { color: theme.value.grid },
      ticks: {
        color: theme.value.text,
        font: { size: 10 },
        maxTicksLimit: 6,
        ...(isMoneyMetric.value
          ? { format: { style: 'currency', currency: 'USD' }, callback: moneyTick }
          : { callback: (value: string | number) => formatValue(Number(value)) })
      }
    }
  }
}))
</script>
