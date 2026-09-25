<template>
  <!--
    按模型的 Token 折线（muqian 2026-09-25「概览换成折线图，统计维度是模型 token 数」）：一个模型一条线，横轴按天。
    配色（dataviz 校验脚本实测）：现有 --af-chart-1..8 里，浅色 / 深色两套都能「两两可区分」（色盲 ΔE ≥ 8、正常视力 ≥ 15）的
    最多 3 色——第 3、4、7 槽（靛 / 琥珀 / 梅）；4 色以上最好也只有色盲 ΔE 5.9。所以前 3 个模型各一条彩色线，
    其余并成「其他」一条浅墨虚线（中性色，不占分类色位）。颜色按模型名排序分配，换 7 / 30 天时同一组模型不换色。
    图上方是图例（色条 + 模型 + 区间合计与占比），悬停看当天各模型的数。
  -->
  <div v-if="loading" class="flex h-64 items-center justify-center">
    <span class="spinner text-af-ink-3" />
  </div>
  <div v-else-if="series.length" data-testid="model-trend-lines">
    <ul class="mb-4 flex flex-wrap gap-x-6 gap-y-2" data-testid="model-trend-legend">
      <li v-for="item in series" :key="item.key" class="flex min-w-0 items-center gap-2 text-13">
        <span
          class="h-0 w-4 shrink-0 border-t-2"
          :class="item.isOther ? 'border-dashed' : ''"
          :style="{ borderColor: item.color }"
          aria-hidden="true"
        />
        <span class="max-w-[14rem] truncate font-mono text-af-ink-2" :title="item.label">{{ item.label }}</span>
        <span class="whitespace-nowrap tabular-nums text-af-ink">
          {{ formatTokensK(item.total) }}<span class="ml-1 text-af-ink-4">{{ item.percentText }}</span>
        </span>
      </li>
    </ul>
    <div class="h-64">
      <Line :data="chartData" :options="chartOptions" />
    </div>
  </div>
  <div v-else class="flex h-64 items-center justify-center text-sm text-af-ink-3">
    {{ t('userUi.overview.models.empty') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip } from 'chart.js'
import { Line } from 'vue-chartjs'
import { readTokenRgb, useChartTheme } from '@/composables/useChartTheme'
import { formatTokensK } from '@/utils/format'
import type { ModelTrendPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip)

const props = withDefaults(
  defineProps<{
    points: ModelTrendPoint[]
    /** 区间内逐日的日期（YYYY-MM-DD），决定横轴的点数与顺序 */
    days: string[]
    loading?: boolean
  }>(),
  { loading: false }
)

const { t } = useI18n()
const theme = useChartTheme()

/** 校验过的三个分类色槽（--af-chart-3 / 4 / 7），见模板顶部说明；超过的模型并进「其他」 */
const SERIES_SLOTS = ['chart-3', 'chart-4', 'chart-7'] as const
const OTHER_KEY = '__other__'

interface Series {
  key: string
  label: string
  values: number[]
  total: number
  percentText: string
  color: string
  isOther: boolean
}

const series = computed<Series[]>(() => {
  // 依赖主题：切深色时重新读色
  void theme.value
  const dayIndex = new Map(props.days.map((day, index) => [day, index]))
  const byModel = new Map<string, number[]>()
  for (const point of props.points) {
    const index = dayIndex.get(point.date)
    if (index === undefined) continue
    const values = byModel.get(point.model) ?? new Array<number>(props.days.length).fill(0)
    values[index] += point.total_tokens
    byModel.set(point.model, values)
  }

  const sum = (values: number[]) => values.reduce((acc, v) => acc + v, 0)
  const ranked = [...byModel.entries()]
    .map(([model, values]) => ({ model, values, total: sum(values) }))
    .filter((row) => row.total > 0)
    .sort((a, b) => b.total - a.total)
  const grand = ranked.reduce((acc, row) => acc + row.total, 0)
  if (grand <= 0) return []

  const percentText = (total: number) => {
    const share = total / grand
    return `${(share * 100).toFixed(share < 0.1 ? 1 : 0)}%`
  }

  const head = ranked.slice(0, SERIES_SLOTS.length)
  // 颜色跟着模型走、不跟名次：前几名按模型名排序后依次取色
  const colorOf = new Map(
    [...head].sort((a, b) => a.model.localeCompare(b.model)).map((row, i) => [row.model, readTokenRgb(SERIES_SLOTS[i])])
  )
  const out: Series[] = head.map((row) => ({
    key: row.model,
    label: row.model,
    values: row.values,
    total: row.total,
    percentText: percentText(row.total),
    color: colorOf.get(row.model) ?? readTokenRgb('ink'),
    isOther: false
  }))

  const rest = ranked.slice(SERIES_SLOTS.length)
  if (rest.length) {
    const values = props.days.map((_, index) => rest.reduce((acc, row) => acc + row.values[index], 0))
    const total = sum(values)
    out.push({
      key: OTHER_KEY,
      label: t('userUi.overview.models.other'),
      values,
      total,
      percentText: percentText(total),
      color: readTokenRgb('ink-4'),
      isOther: true
    })
  }
  return out
})

/** 横轴去掉年份：YYYY-MM-DD → MM-DD */
const shortLabel = (date: string) => (/^\d{4}-\d{2}-\d{2}$/.test(date) ? date.slice(5) : date)

const chartData = computed(() => ({
  labels: props.days.map(shortLabel),
  datasets: series.value.map((item) => ({
    label: item.label,
    data: item.values,
    borderColor: item.color,
    backgroundColor: item.color,
    borderWidth: 2,
    borderDash: item.isOther ? [4, 4] : [],
    pointRadius: 0,
    pointHoverRadius: 4,
    pointHoverBackgroundColor: item.color,
    pointHitRadius: 8,
    fill: false,
    // 单调插值：普通 tension 在很多 0 的稀疏数据两侧会冲到 0 以下
    cubicInterpolationMode: 'monotone' as const
  }))
}))

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      usePointStyle: true,
      callbacks: {
        label: (context: { dataset: { label?: string }; parsed: { y: number | null } }) =>
          `${context.dataset.label ?? ''}: ${formatTokensK(context.parsed.y ?? 0)}`
      }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { color: theme.value.text, maxTicksLimit: 10, font: { size: 10 } }
    },
    y: {
      beginAtZero: true,
      grid: { color: theme.value.grid },
      ticks: {
        color: theme.value.text,
        font: { size: 10 },
        maxTicksLimit: 6,
        callback: (value: string | number) => formatTokensK(Number(value))
      }
    }
  }
}))
</script>
