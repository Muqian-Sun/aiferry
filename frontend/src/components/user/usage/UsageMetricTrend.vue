<template>
  <!--
    单指标趋势（Token 总量 / 请求数 / 实付费用）：一份 trend 数据，只画一条墨色线 + 很淡的面积。
    控制台配色单色为主（muqian 2026-09-23），不再用分类彩色；管理站概览与用量「分析」也用它（A7 起管理端也不再画多色折线）。
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
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler } from 'chart.js'
import { Line } from 'vue-chartjs'
import { useChartTheme } from '@/composables/useChartTheme'
import { formatCurrency, formatNumber, formatTokensK } from '@/utils/format'
import type { TrendDataPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

export type UsageTrendMetric = 'tokens' | 'requests' | 'cost'

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

function valueOf(point: TrendDataPoint): number {
  if (props.metric === 'tokens') return point.total_tokens
  return props.metric === 'requests' ? point.requests : point.actual_cost
}

function formatValue(value: number): string {
  if (props.metric === 'tokens') return formatTokensK(value)
  return props.metric === 'requests' ? formatNumber(value) : formatCurrency(value)
}

const chartData = computed(() => {
  if (!props.trendData?.length) return null
  return {
    labels: props.trendData.map((d) => shortLabel(d.date)),
    datasets: [
      {
        label: t(`userUi.usage.trend.${props.metric}`),
        data: props.trendData.map(valueOf),
        borderColor: theme.value.ink,
        backgroundColor: theme.value.inkFill,
        borderWidth: 2,
        pointRadius: 0,
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
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: { parsed: { y: number | null } }) =>
          `${t(`userUi.usage.trend.${props.metric}`)}: ${formatValue(context.parsed.y ?? 0)}`
      }
    }
  },
  scales: {
    x: {
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
        callback: (value: string | number) => formatValue(Number(value))
      }
    }
  }
}))
</script>
