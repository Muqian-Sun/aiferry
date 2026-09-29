<template>
  <!--
    模型 Token 趋势（muqian 2026-09-29）：每个时间桶一根柱子，柱高 = 当桶 Token 总量，按模型分色堆叠，
    同时看总量走势、各模型走势和当桶谁用得最多（悬停时按用量从高到低列出，第一行就是用得最多的）。
    颜色由 modelSeries 按区间 Token 排名分配，与下方模型表的色块一致。
  -->
  <div v-if="loading" class="flex h-64 items-center justify-center">
    <span class="spinner text-af-ink-3" />
  </div>
  <div v-else-if="chartData" class="h-64" data-testid="model-token-trend">
    <Bar :data="chartData" :options="options" />
  </div>
  <div v-else class="flex h-64 items-center justify-center text-sm text-af-ink-3">
    {{ t('admin.dashboard.noDataAvailable') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, BarElement, CategoryScale, LinearScale, Tooltip, type TooltipItem } from 'chart.js'
import { Bar } from 'vue-chartjs'
import { readTokenRgb, useChartTheme } from '@/composables/useChartTheme'
import { formatTokensK } from '@/utils/format'
import type { ModelTrendPoint } from '@/types'
import type { ModelSeries } from './modelSeries'

ChartJS.register(BarElement, CategoryScale, LinearScale, Tooltip)

const props = defineProps<{
  points: ModelTrendPoint[]
  /** 时间桶（与页头范围、粒度一致；没有数据的桶也占一根空柱） */
  days: string[]
  series: ModelSeries
  loading?: boolean
}>()

const { t } = useI18n()
const theme = useChartTheme()

/** 横轴去掉年份：按天 YYYY-MM-DD → MM-DD；按小时 YYYY-MM-DD HH:00 → MM-DD HH:00 */
const shortLabel = (date: string): string => (/^\d{4}-\d{2}-\d{2}(?: \d{2}:00)?$/.test(date) ? date.slice(5) : date)

const chartData = computed(() => {
  if (!props.points?.length || !props.days.length) return null
  const bucketIndex = new Map(props.days.map((day, i) => [day, i]))
  const rowOf = new Map<string, number>()
  props.series.colored.forEach((model, i) => rowOf.set(model, i))
  const otherRow = props.series.colored.length
  const rows = Array.from({ length: otherRow + (props.series.others.length ? 1 : 0) }, () => new Array<number>(props.days.length).fill(0))
  for (const point of props.points) {
    const col = bucketIndex.get(point.date)
    if (col === undefined) continue
    const row = rowOf.get(point.model) ?? (props.series.others.length ? otherRow : undefined)
    if (row === undefined) continue
    rows[row][col] += Number(point.total_tokens) || 0
  }
  const datasets = props.series.colored.map((model, i) => ({
    label: model,
    data: rows[i],
    backgroundColor: theme.value.color(i),
    stack: 'tokens'
  }))
  // 有「其他」说明 8 个色槽都用上了，上面已读过 theme，明暗切换时这里会随之重算
  if (props.series.others.length) {
    datasets.push({
      label: t('admin.dashboard.otherModels', { count: props.series.others.length }),
      data: rows[otherRow],
      backgroundColor: readTokenRgb('ink-4'),
      stack: 'tokens'
    })
  }
  return { labels: props.days.map(shortLabel), datasets }
})

const options = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  datasets: { bar: { maxBarThickness: 36, categoryPercentage: 0.72, barPercentage: 0.9 } },
  plugins: {
    legend: { display: false },
    tooltip: {
      backgroundColor: theme.value.tooltipBg,
      titleColor: theme.value.tooltipText,
      bodyColor: theme.value.tooltipText,
      footerColor: theme.value.tooltipText,
      borderColor: theme.value.tooltipBorder,
      borderWidth: 1,
      boxWidth: 8,
      boxHeight: 8,
      // 当桶按用量从高到低，第一行就是用得最多的模型；没用到的模型不列
      itemSort: (a: TooltipItem<'bar'>, b: TooltipItem<'bar'>) => (b.parsed.y ?? 0) - (a.parsed.y ?? 0),
      filter: (item: TooltipItem<'bar'>) => (item.parsed.y ?? 0) > 0,
      callbacks: {
        label: (item: TooltipItem<'bar'>) => {
          const total = bucketTotal(item.dataIndex)
          const value = item.parsed.y ?? 0
          const share = total > 0 ? Math.round((value / total) * 100) : 0
          return ` ${item.dataset.label}  ${formatTokensK(value)} · ${share}%`
        },
        footer: (items: TooltipItem<'bar'>[]) =>
          items.length ? t('admin.dashboard.bucketTotal', { value: formatTokensK(bucketTotal(items[0].dataIndex)) }) : ''
      }
    }
  },
  scales: {
    x: {
      stacked: true,
      grid: { display: false },
      ticks: { color: theme.value.text, maxTicksLimit: 16, font: { size: 10 } }
    },
    y: {
      stacked: true,
      beginAtZero: true,
      grid: { color: theme.value.grid },
      ticks: { color: theme.value.text, font: { size: 10 }, maxTicksLimit: 6, callback: (value: string | number) => formatTokensK(Number(value)) }
    }
  }
}))

function bucketTotal(index: number): number {
  return chartData.value?.datasets.reduce((sum, ds) => sum + (ds.data[index] ?? 0), 0) ?? 0
}
</script>
