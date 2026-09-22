<template>
  <!--
    单指标趋势（请求数 / 实付费用）：与 Token 趋势共用同一份 trend 数据，只画一条线。
    Token 视图仍由 TokenUsageTrend 画（四条线 + 缓存命中率），这里不重复。
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
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import { useChartTheme } from '@/composables/useChartTheme'
import { formatCurrency, formatNumber } from '@/utils/format'
import type { TrendDataPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip)

export type UsageTrendMetric = 'requests' | 'cost'

const props = defineProps<{
  trendData: TrendDataPoint[]
  metric: UsageTrendMetric
  loading?: boolean
}>()

const { t } = useI18n()
const theme = useChartTheme()

// 请求数取色槽 0（与 Token 视图的 Input 同色），费用取槽 1
const color = computed(() => theme.value.color(props.metric === 'requests' ? 0 : 1))

function valueOf(point: TrendDataPoint): number {
  return props.metric === 'requests' ? point.requests : point.actual_cost
}

function formatValue(value: number): string {
  return props.metric === 'requests' ? formatNumber(value) : formatCurrency(value)
}

const chartData = computed(() => {
  if (!props.trendData?.length) return null
  return {
    labels: props.trendData.map((d) => d.date),
    datasets: [
      {
        label: t(`userUi.usage.trend.${props.metric}`),
        data: props.trendData.map(valueOf),
        borderColor: color.value,
        backgroundColor: color.value,
        borderWidth: 2,
        pointRadius: 0,
        pointHitRadius: 8,
        fill: false,
        tension: 0.3
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
