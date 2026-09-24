<template>
  <!--
    近 24 小时每小时请求数（muqian 2026-09-24「概览页多一点图」）：24 根墨色柱，单色（量的大小只用柱高表达）、
    细网格、圆角柱顶、柱间留缝；悬停给出日期与该小时的请求数。没有任何请求时显示空态，不画一排零柱。
    用滚动的近 24 小时而不是「今天」：早上打开时今天还没请求，整张图会是空的。
  -->
  <div v-if="loading" class="flex h-44 items-center justify-center">
    <span class="spinner text-af-ink-3" />
  </div>
  <div v-else-if="hasData" class="h-44" data-testid="hourly-bars">
    <Bar :data="chartData" :options="options" />
  </div>
  <div v-else class="flex h-44 items-center justify-center text-sm text-af-ink-3">
    {{ t('userUi.overview.hourly.empty') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, BarElement, CategoryScale, LinearScale, Tooltip } from 'chart.js'
import { Bar } from 'vue-chartjs'
import { useChartTheme } from '@/composables/useChartTheme'
import { formatNumber } from '@/utils/format'
import type { TrendDataPoint } from '@/types'

ChartJS.register(BarElement, CategoryScale, LinearScale, Tooltip)

const props = defineProps<{
  /** 按小时的趋势点（date 形如 "YYYY-MM-DD HH:00"），覆盖昨天到今天 */
  points: TrendDataPoint[]
  loading?: boolean
}>()

const { t } = useI18n()
const theme = useChartTheme()

const pad = (n: number) => String(n).padStart(2, '0')

/**
 * 最近 24 个整点（含当前小时），按「YYYY-MM-DD HH:00」精确对位后端的小时桶。
 * 后端小时桶按服务端时区出（dev 与目标用户都是东八区，和浏览器一致）；时区不同时只会少几根柱，不会错位到别的小时。
 */
const slots = computed(() => {
  const byKey = new Map(props.points.map((p) => [p.date, p.requests]))
  const now = new Date()
  return Array.from({ length: 24 }, (_, i) => {
    const d = new Date(now.getTime() - (23 - i) * 60 * 60 * 1000)
    const key = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:00`
    return { key, hour: d.getHours(), day: `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`, requests: byKey.get(key) ?? 0 }
  })
})

const hasData = computed(() => slots.value.some((s) => s.requests > 0))

const labels = computed(() => slots.value.map((s) => pad(s.hour)))

const chartData = computed(() => ({
  labels: labels.value,
  datasets: [
    {
      label: t('userUi.overview.hourly.requests'),
      data: slots.value.map((s) => s.requests),
      backgroundColor: theme.value.ink,
      hoverBackgroundColor: theme.value.text,
      borderRadius: { topLeft: 4, topRight: 4 },
      borderSkipped: 'bottom' as const,
      categoryPercentage: 0.9,
      barPercentage: 0.8
    }
  ]
}))

const options = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        title: (items: Array<{ dataIndex: number }>) => {
          const slot = slots.value[items[0]?.dataIndex ?? 0]
          return slot ? `${slot.day} ${pad(slot.hour)}:00–${pad((slot.hour + 1) % 24)}:00` : ''
        },
        label: (context: { parsed: { y: number | null } }) =>
          `${t('userUi.overview.hourly.requests')}: ${formatNumber(context.parsed.y ?? 0)}`
      }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: {
        color: theme.value.text,
        font: { size: 10 },
        autoSkip: false,
        maxRotation: 0,
        callback: (_value: string | number, index: number) => (index % 3 === 2 ? labels.value[index] : '')
      }
    },
    y: {
      beginAtZero: true,
      grid: { color: theme.value.grid },
      border: { display: false },
      ticks: { color: theme.value.text, font: { size: 10 }, maxTicksLimit: 4, precision: 0 }
    }
  }
}))
</script>
