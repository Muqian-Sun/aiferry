<template>
  <!-- 全站整体趋势：一条墨色线（可用率、首字延迟 P50 或缓存命中率），没有请求的段断开不连线 -->
  <div v-if="chartData" class="h-48">
    <Line :data="chartData" :options="lineOptions" />
  </div>
  <div v-else class="flex h-48 items-center justify-center text-sm text-af-ink-3">
    {{ t('userUi.serviceStatus.trend.empty') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler } from 'chart.js'
import { Line } from 'vue-chartjs'
import { useChartTheme } from '@/composables/useChartTheme'
import { formatPercent, formatLatency, formatSlotTime, type StatusSlot } from './serviceStatus'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

export type ServiceTrendMetric = 'availability' | 'ttft' | 'cache'

const props = defineProps<{
  slots: StatusSlot[]
  bucketSeconds: number
  metric: ServiceTrendMetric
}>()

const { t, locale } = useI18n()
const theme = useChartTheme()

const metricLabel = computed(() => t(`userUi.serviceStatus.columns.${props.metric}`))
/** 可用率、缓存命中率是百分比（纵轴 0–100），首字延迟是毫秒 */
const isPercent = computed(() => props.metric !== 'ttft')

function valueOf(slot: StatusSlot): number | null {
  const metrics = slot.point?.metrics
  if (!metrics) return null
  if (props.metric === 'ttft') return metrics.ttft_p50_ms
  const rate = props.metric === 'availability' ? metrics.availability : metrics.cache_hit_rate
  return rate == null ? null : rate * 100
}

function formatValue(value: number): string {
  return isPercent.value ? formatPercent(value / 100) : formatLatency(value)
}

const chartData = computed(() => {
  const values = props.slots.map(valueOf)
  if (!values.some((value) => value != null)) return null
  return {
    labels: props.slots.map((slot) => formatSlotTime(slot.start, props.bucketSeconds, locale.value)),
    datasets: [
      {
        label: metricLabel.value,
        data: values,
        borderColor: theme.value.ink,
        backgroundColor: theme.value.inkFill,
        borderWidth: 2,
        // 每段画一个小圆点：前后都没有请求的段连不成线，点半径为 0 时什么也看不到（2026-10-04 走查：
        // 24 小时里隔了十个小时才又有请求的那一段没画出来，像是数据停在了凌晨）
        pointRadius: 2,
        pointBackgroundColor: theme.value.ink,
        pointHoverRadius: 4,
        pointHoverBackgroundColor: theme.value.ink,
        pointHitRadius: 8,
        // 不填色：请求稀疏时只有相邻两段都有数据才填得出来，会剩下一块孤立的灰柱（2026-10-04 走查）
        fill: false,
        // 100% 的点压在顶线上，允许画出绘图区，不被裁掉半个
        clip: false as const,
        spanGaps: false,
        cubicInterpolationMode: 'monotone' as const
      }
    ]
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  // 顶部留出圆点半径的空间
  layout: { padding: { top: 6 } },
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: { parsed: { y: number | null } }) =>
          context.parsed.y == null ? '' : `${metricLabel.value}: ${formatValue(context.parsed.y)}`
      }
    }
  },
  scales: {
    x: {
      // 首尾的点不贴坐标轴、不压在纵轴刻度上：类目轴两端各留半格（2026-10-05 走查：渠道状态 100% 的点被刻度挡住）
      offset: true,
      grid: { display: false },
      ticks: { color: theme.value.text, maxTicksLimit: 8, font: { size: 10 } }
    },
    y: {
      // 可用率封顶 100、默认从 90 起（低于 90 时坐标轴自动往下扩），免得 99.2% 与 99.8% 被放大成悬崖；
      // 缓存命中率 0–100 全幅（常在 50%–90% 之间摆动）；延迟从 0 起
      beginAtZero: props.metric !== 'availability',
      suggestedMin: props.metric === 'availability' ? 90 : undefined,
      max: isPercent.value ? 100 : undefined,
      grid: { color: theme.value.grid },
      ticks: {
        color: theme.value.text,
        font: { size: 10 },
        maxTicksLimit: 5,
        // 刻度只写整数百分比；延迟照常
        callback: (value: string | number) => (isPercent.value ? `${Math.round(Number(value))}%` : formatLatency(Number(value)))
      }
    }
  }
}))
</script>
