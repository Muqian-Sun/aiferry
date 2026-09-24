<template>
  <!--
    近 7 天各模型占比（muqian 2026-09-24「概览页多一点图」）：横条，一种颜色（占比只用条长表达，不给模型分配彩色），
    前 5 个模型 + 「其他」；每行条尾直接写出数值与占比，不靠悬停才能读到；数值列定宽右对齐，各行条的起止对齐。按实付费用排序；全是 0 元时改按请求数。
  -->
  <div v-if="loading" class="flex h-44 items-center justify-center">
    <span class="spinner text-af-ink-3" />
  </div>
  <ol v-else-if="rows.length" class="space-y-3.5" data-testid="model-share">
    <li
      v-for="row in rows"
      :key="row.key"
      class="grid grid-cols-[minmax(0,8.5rem)_minmax(0,1fr)_8.5rem] items-center gap-x-4 text-13"
    >
      <span class="truncate font-mono text-af-ink-2" :title="row.label">{{ row.label }}</span>
      <span
        class="h-2 overflow-hidden rounded-full bg-af-sunken"
        role="img"
        :aria-label="`${row.label} ${row.percentText}`"
      >
        <span class="block h-full rounded-full bg-af-ink" :style="{ width: row.width }" />
      </span>
      <span class="whitespace-nowrap text-right tabular-nums text-af-ink">
        {{ row.valueText }}<span class="ml-1.5 text-af-ink-4">{{ row.percentText }}</span>
      </span>
    </li>
  </ol>
  <div v-else class="flex h-44 items-center justify-center text-sm text-af-ink-3">
    {{ t('userUi.overview.models.empty') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatCurrency, formatNumber } from '@/utils/format'
import type { ModelStat } from '@/types'

const props = defineProps<{
  models: ModelStat[]
  loading?: boolean
}>()

const { t } = useI18n()

/** 最多列几个模型，其余并成「其他」（超过这个数条目太多、条太细，读不出差别） */
const TOP_N = 5

interface ShareRow {
  key: string
  label: string
  value: number
  valueText: string
  percentText: string
  width: string
}

const rows = computed<ShareRow[]>(() => {
  const models = props.models ?? []
  const byCost = models.some((m) => m.actual_cost > 0)
  const valueOf = (m: ModelStat) => (byCost ? m.actual_cost : m.requests)
  const format = (v: number) => (byCost ? formatCurrency(v) : `${formatNumber(v)} ${t('userUi.overview.models.requestsUnit')}`)
  const sorted = models.filter((m) => valueOf(m) > 0).sort((a, b) => valueOf(b) - valueOf(a))
  const total = sorted.reduce((sum, m) => sum + valueOf(m), 0)
  if (total <= 0) return []

  const head = sorted.slice(0, TOP_N).map((m) => ({ key: m.model, label: m.model, value: valueOf(m) }))
  const rest = sorted.slice(TOP_N).reduce((sum, m) => sum + valueOf(m), 0)
  if (rest > 0) head.push({ key: '__other__', label: t('userUi.overview.models.other'), value: rest })

  return head.map((r) => {
    const share = r.value / total
    return {
      ...r,
      valueText: format(r.value),
      percentText: `${(share * 100).toFixed(share < 0.1 ? 1 : 0)}%`,
      width: `${Math.max(share * 100, 1.5)}%`
    }
  })
})
</script>
