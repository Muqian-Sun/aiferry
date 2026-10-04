<script setup lang="ts">
/**
 * 运维页「请求」（2026-10-04 重排）：四个数就是下面这张图的切换按钮，不另设指标区。
 * 原来的页头指标卡、吞吐趋势、请求时长分布、渠道切换趋势、错误趋势、错误分布都并进这一处。
 *
 * - 成功率：没选到渠道算失败（后端已改口径）；用户自己的限额、余额这类业务限制不算。
 * - 首字延迟：大字是 P50，小字是 P99；P99 按告警线标色（默认 20 秒，与服务状态的「异常」线一致）。
 * - 换渠道恢复：上游出错、换渠道后成功的请求，用户没受影响，不算失败。
 */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, LineElement, PointElement, Tooltip } from 'chart.js'
import { Bar, Line } from 'vue-chartjs'
import { readTokenRgb, useChartTheme } from '@/composables/useChartTheme'
import { formatDurationMs } from '@/components/usage/usageRow'
import type { OpsDashboardOverview, OpsErrorTrendPoint, OpsMetricThresholds, OpsThroughputTrendPoint } from '@/api/admin/ops'
import { formatHistoryLabel } from '../utils/opsFormatters'

ChartJS.register(BarElement, CategoryScale, LinearScale, LineElement, PointElement, Tooltip, Legend)

const props = defineProps<{
  overview: OpsDashboardOverview | null
  throughput: OpsThroughputTrendPoint[]
  errors: OpsErrorTrendPoint[]
  thresholds: OpsMetricThresholds | null
  timeRange: string
  loading: boolean
}>()

const { t } = useI18n()
const theme = useChartTheme()

type Tab = 'requests' | 'success' | 'ttft' | 'recovered'
const active = ref<Tab>('requests')

const ttftThresholdMs = computed(() => props.thresholds?.ttft_p99_ms_max ?? 20000)

function ttftClass(ms: number | null | undefined): string {
  if (ms == null) return 'text-af-ink-3'
  if (ms >= ttftThresholdMs.value) return 'text-af-danger'
  if (ms >= ttftThresholdMs.value * 0.8) return 'text-af-warning'
  return 'text-af-ink-3'
}

const tabs = computed(() => {
  const ov = props.overview
  const noData = '—'
  const requestTotal = ov?.request_count_total ?? 0
  const slaBase = ov?.request_count_sla ?? 0
  const ttftP50 = ov?.ttft?.p50_ms ?? null
  const ttftP99 = ov?.ttft?.p99_ms ?? null
  return [
    {
      key: 'requests' as Tab,
      label: t('admin.ops.page.metrics.requests'),
      value: ov ? requestTotal.toLocaleString() : noData,
      detail: ov ? t('admin.ops.page.metrics.currentQps', { qps: (ov.qps?.current ?? 0).toFixed(1) }) : '',
      detailClass: 'text-af-ink-3'
    },
    {
      key: 'success' as Tab,
      label: t('admin.ops.page.metrics.successRate'),
      value: ov && slaBase > 0 ? `${(ov.sla * 100).toFixed(1)}%` : noData,
      detail: ov ? t('admin.ops.page.metrics.failed', { count: ov.error_count_sla }) : '',
      detailClass: ov && ov.error_count_sla > 0 ? 'text-af-danger' : 'text-af-ink-3'
    },
    {
      key: 'ttft' as Tab,
      label: t('admin.ops.page.metrics.ttft'),
      value: ttftP50 != null ? formatDurationMs(ttftP50) : noData,
      detail: ttftP99 != null ? t('admin.ops.page.metrics.p99', { value: formatDurationMs(ttftP99) }) : '',
      detailClass: ttftClass(ttftP99)
    },
    {
      key: 'recovered' as Tab,
      label: t('admin.ops.page.metrics.recovered'),
      value: ov ? (ov.upstream_recovered_count ?? 0).toLocaleString() : noData,
      detail: t('admin.ops.page.metrics.recoveredHint'),
      detailClass: 'text-af-ink-3'
    }
  ]
})

const labels = computed(() => {
  const source = active.value === 'success' || active.value === 'recovered' ? props.errors : props.throughput
  return source.map((p) => formatHistoryLabel(p.bucket_start, props.timeRange))
})

const barData = computed(() => {
  const palette = theme.value
  if (active.value === 'requests') {
    return {
      labels: labels.value,
      datasets: [{ label: t('admin.ops.page.metrics.requests'), data: props.throughput.map((p) => p.request_count), backgroundColor: palette.ink, borderRadius: 2 }]
    }
  }
  if (active.value === 'success') {
    const other = (p: OpsErrorTrendPoint) => Math.max((p.error_count_sla ?? 0) - (p.upstream_failed_count ?? 0) - (p.routing_failed_count ?? 0), 0)
    return {
      labels: labels.value,
      datasets: [
        { label: t('admin.ops.page.phase.upstream'), data: props.errors.map((p) => p.upstream_failed_count ?? 0), backgroundColor: readTokenRgb('danger'), stack: 'f' },
        { label: t('admin.ops.page.phase.routing'), data: props.errors.map((p) => p.routing_failed_count ?? 0), backgroundColor: readTokenRgb('warning'), stack: 'f' },
        { label: t('admin.ops.page.phase.other'), data: props.errors.map(other), backgroundColor: palette.text, stack: 'f' }
      ]
    }
  }
  return {
    labels: labels.value,
    datasets: [{ label: t('admin.ops.page.metrics.recovered'), data: props.errors.map((p) => p.recovered_count ?? 0), backgroundColor: readTokenRgb('warning'), borderRadius: 2 }]
  }
})

const lineData = computed(() => {
  const palette = theme.value
  const seconds = (v: number | null | undefined) => (v == null ? null : v / 1000)
  return {
    labels: labels.value,
    datasets: [
      { label: 'P50', data: props.throughput.map((p) => seconds(p.ttft_p50_ms)), borderColor: palette.ink, backgroundColor: palette.ink, pointRadius: 2, spanGaps: false, tension: 0.2 },
      { label: 'P99', data: props.throughput.map((p) => seconds(p.ttft_p99_ms)), borderColor: palette.text, backgroundColor: palette.text, borderDash: [4, 4], pointRadius: 2, spanGaps: false, tension: 0.2 }
    ]
  }
})

const hasData = computed(() => {
  if (active.value === 'requests') return props.throughput.some((p) => p.request_count > 0)
  if (active.value === 'ttft') return props.throughput.some((p) => p.ttft_p50_ms != null)
  if (active.value === 'success') return props.errors.some((p) => (p.error_count_sla ?? 0) > 0)
  return props.errors.some((p) => (p.recovered_count ?? 0) > 0)
})

const options = computed(() => {
  const palette = theme.value
  const stacked = active.value === 'success'
  return {
    responsive: true,
    maintainAspectRatio: false,
    animation: false as const,
    interaction: { intersect: false, mode: 'index' as const },
    plugins: {
      legend: {
        display: active.value === 'success' || active.value === 'ttft',
        position: 'top' as const,
        align: 'end' as const,
        labels: { color: palette.text, usePointStyle: true, boxWidth: 6, font: { size: 11 } }
      },
      tooltip: {
        backgroundColor: palette.tooltipBg,
        titleColor: palette.tooltipText,
        bodyColor: palette.tooltipText,
        borderColor: palette.tooltipBorder,
        borderWidth: 1,
        callbacks:
          active.value === 'ttft'
            ? { label: (ctx: any) => `${ctx.dataset.label}: ${ctx.parsed.y == null ? '—' : formatDurationMs(Math.round(ctx.parsed.y * 1000))}` }
            : {}
      }
    },
    scales: {
      x: { stacked, grid: { display: false }, ticks: { color: palette.text, font: { size: 10 }, maxTicksLimit: 8, autoSkip: true } },
      // 从 0 开始：全是 0 的时间段不再出现负刻度
      y: {
        stacked,
        beginAtZero: true,
        suggestedMax: active.value === 'ttft' ? undefined : 1,
        grid: { color: palette.grid },
        ticks: {
          color: palette.text,
          font: { size: 10 },
          precision: active.value === 'ttft' ? undefined : 0,
          callback: active.value === 'ttft' ? (v: number | string) => `${v}s` : undefined
        }
      }
    }
  }
})
</script>

<template>
  <section class="border-t border-af-hairline py-4" data-testid="ops-request-panel">
    <h2 class="mb-2 text-sm font-semibold text-af-ink">{{ t('admin.ops.page.metrics.title') }}</h2>
    <div class="grid grid-cols-2 gap-x-6 md:grid-cols-4" role="tablist">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        role="tab"
        :aria-selected="active === tab.key"
        class="min-w-0 border-t-2 pb-2 pt-2 text-left transition-colors"
        :class="active === tab.key ? 'border-af-ink' : 'border-transparent hover:border-af-hairline-strong'"
        :data-testid="`ops-metric-${tab.key}`"
        @click="active = tab.key"
      >
        <div class="text-xs text-af-ink-3">{{ tab.label }}</div>
        <div class="text-2xl font-semibold tabular-nums text-af-ink">{{ tab.value }}</div>
        <div class="truncate text-xs tabular-nums" :class="tab.detailClass">{{ tab.detail }}</div>
      </button>
    </div>
    <div class="relative mt-3 h-56">
      <Line v-if="active === 'ttft' && hasData" :data="lineData as any" :options="options as any" />
      <Bar v-else-if="hasData" :data="barData as any" :options="options as any" />
      <div v-else class="flex h-full items-center justify-center text-sm text-af-ink-3">
        {{ props.loading ? t('admin.ops.loadingText') : t('admin.ops.page.noDataInRange') }}
      </div>
    </div>
  </section>
</template>
