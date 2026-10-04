<template>
  <!--
    每日收款（A7）：一条墨色线 + 很淡的面积。原来把各币种收入和订单数叠在一张双轴彩色图上，
    改成页签一次看一条（每个币种的收入各一个页签 + 订单数），不再用双轴。
  -->
  <SheetSection :title="t('payment.admin.dailyRevenue')" data-testid="daily-revenue">
    <template v-if="metricTabs.length > 1" #actions>
      <SegmentedControl :model-value="activeMetric" @update:model-value="selectedMetric = $event" :options="metricTabs" :label="t('payment.admin.dailyRevenue')" test-id-prefix="daily-revenue-metric" />
    </template>
    <div class="h-64">
      <div v-if="loading" class="flex h-full items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <Line v-else-if="chartData" :data="chartData" :options="chartOptions" />
      <div v-else class="flex h-full items-center justify-center text-sm text-af-ink-3">
        {{ t('payment.admin.noData') }}
      </div>
    </div>
  </SheetSection>
</template>

<script setup lang="ts">
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler } from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import { useChartTheme } from '@/composables/useChartTheme'
import type { DailyPaymentStats } from '@/types/payment'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)

const { t } = useI18n()
const theme = useChartTheme()

const props = defineProps<{
  data: DailyPaymentStats[]
  loading?: boolean
}>()

const COUNT_METRIC = 'count'
const amountMetric = (currency: string) => `amount:${currency}`

const currencies = computed(() => [...new Set(props.data.flatMap((day) => Object.keys(day.amount)))].sort())

/** 只有一个币种时页签只写「收入」；多个币种时带上币种 */
const metricTabs = computed(() => [
  ...currencies.value.map((currency) => ({
    key: amountMetric(currency),
    label: currencies.value.length > 1 ? `${currency} ${t('payment.admin.revenue')}` : t('payment.admin.revenue')
  })),
  { key: COUNT_METRIC, label: t('payment.admin.orderCount') }
])

const selectedMetric = ref('')
/** 换了时间范围后币种可能变了：选中的页签不在了就回到第一个 */
const activeMetric = computed(() =>
  metricTabs.value.some((tab) => tab.key === selectedMetric.value) ? selectedMetric.value : metricTabs.value[0].key
)
const activeCurrency = computed(() =>
  activeMetric.value === COUNT_METRIC ? null : activeMetric.value.slice(amountMetric('').length)
)

function formatValue(value: number, compact = false): string {
  const currency = activeCurrency.value
  if (!currency) return value.toLocaleString()
  return new Intl.NumberFormat(undefined, { style: 'currency', currency, ...(compact ? { notation: 'compact' as const } : {}) }).format(value)
}

/** 横轴去掉年份：YYYY-MM-DD → MM-DD */
const shortLabel = (date: string) => (/^\d{4}-\d{2}-\d{2}$/.test(date) ? date.slice(5) : date)

const chartData = computed(() => {
  if (!props.data.length) return null
  const currency = activeCurrency.value
  const label = metricTabs.value.find((tab) => tab.key === activeMetric.value)?.label ?? ''
  return {
    labels: props.data.map((day) => shortLabel(day.date)),
    datasets: [
      {
        label,
        data: props.data.map((day) => (currency ? day.amount[currency] || 0 : day.count)),
        borderColor: theme.value.ink,
        backgroundColor: theme.value.inkFill,
        borderWidth: 2,
        // 点不多时（≤ 31 天）标出每天的观测点，曲线之间的部分不是数据
        pointRadius: props.data.length <= 31 ? 2.5 : 0,
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

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: { dataset: { label?: string }; parsed: { y: number | null } }) =>
          `${context.dataset.label ?? ''}: ${formatValue(context.parsed.y ?? 0)}`
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
        // 订单数只取整数刻度
        ...(activeCurrency.value ? {} : { precision: 0 }),
        callback: (value: string | number) => formatValue(Number(value), true)
      }
    }
  }
}))
</script>
