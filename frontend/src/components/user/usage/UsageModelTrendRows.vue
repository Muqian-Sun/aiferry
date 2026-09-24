<template>
  <!--
    按模型的 Token 趋势（muqian 2026-09-24「趋势图用模型来统计」「概览放几个图大致看一看，少点文字」）：
    每个模型一行迷你柱（小倍数），前 5 个模型 + 「其他」，单色。
    每行按自己的最大值缩放——看的是各模型自己的起伏；量级看右侧的合计与占比。柱限宽 36px 居中（7 天时不至于成块），
    悬停某根柱，右侧换成那一天的数。
  -->
  <div v-if="loading" class="flex h-56 items-center justify-center">
    <span class="spinner text-af-ink-3" />
  </div>
  <div v-else-if="rows.length" data-testid="model-trend">
    <ol class="space-y-3">
      <li v-for="row in rows" :key="row.key" :class="GRID">
        <span class="truncate font-mono text-13 text-af-ink-2" :title="row.label">{{ row.label }}</span>
        <div class="flex h-9 items-end gap-[2px] border-b border-af-hairline" @mouseleave="hover = null">
          <span
            v-for="(value, index) in row.values"
            :key="index"
            class="flex h-full min-w-0 flex-1 items-end justify-center"
            :aria-label="`${row.label} ${dayLabel(index)} ${formatTokensK(value)}`"
            @mouseenter="hover = { key: row.key, index }"
          >
            <span
              v-if="value > 0"
              class="block w-full max-w-9 rounded-t-[2px] transition-colors"
              :class="isHovered(row.key, index) ? 'bg-af-ink-3' : 'bg-af-ink'"
              :style="{ height: barHeight(value, row.max) }"
            />
          </span>
        </div>
        <span class="whitespace-nowrap text-right text-13 tabular-nums text-af-ink">
          <template v-if="hover?.key === row.key">
            <span class="text-af-ink-3">{{ dayLabel(hover.index) }}</span> {{ formatTokensK(row.values[hover.index] ?? 0) }}
          </template>
          <template v-else>
            {{ formatTokensK(row.total) }}<span class="ml-1.5 text-af-ink-4">{{ row.percentText }}</span>
          </template>
        </span>
      </li>
    </ol>
    <div :class="[GRID, 'mt-2 text-xs tabular-nums text-af-ink-4']" aria-hidden="true">
      <span />
      <span class="flex justify-between">
        <span>{{ dayLabel(0) }}</span>
        <span>{{ dayLabel(days.length - 1) }}</span>
      </span>
      <span />
    </div>
  </div>
  <div v-else class="flex h-56 items-center justify-center text-sm text-af-ink-3">
    {{ t('userUi.overview.models.empty') }}
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatTokensK } from '@/utils/format'
import type { ModelTrendPoint } from '@/types'

const props = defineProps<{
  points: ModelTrendPoint[]
  /** 区间内逐日的日期（YYYY-MM-DD），决定柱的根数与顺序 */
  days: string[]
  loading?: boolean
}>()

const { t } = useI18n()

/** 最多列几个模型，其余并成「其他」（再多行就太密，概览只求大致一看） */
const TOP_N = 5
const GRID = 'grid grid-cols-[minmax(0,9rem)_minmax(0,1fr)_7.5rem] items-end gap-x-5'

interface TrendRow {
  key: string
  label: string
  values: number[]
  total: number
  max: number
  percentText: string
}

const rows = computed<TrendRow[]>(() => {
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
  const sorted = [...byModel.entries()]
    .map(([model, values]) => ({ key: model, label: model, values, total: sum(values) }))
    .filter((row) => row.total > 0)
    .sort((a, b) => b.total - a.total)
  const grand = sorted.reduce((acc, row) => acc + row.total, 0)
  if (grand <= 0) return []

  const head = sorted.slice(0, TOP_N)
  const rest = sorted.slice(TOP_N)
  if (rest.length) {
    const values = props.days.map((_, index) => rest.reduce((acc, row) => acc + row.values[index], 0))
    head.push({ key: '__other__', label: t('userUi.overview.models.other'), values, total: sum(values) })
  }

  return head.map((row) => {
    const share = row.total / grand
    return {
      ...row,
      max: Math.max(...row.values),
      percentText: `${(share * 100).toFixed(share < 0.1 ? 1 : 0)}%`
    }
  })
})

const hover = ref<{ key: string; index: number } | null>(null)
const isHovered = (key: string, index: number) => hover.value?.key === key && hover.value.index === index

/** 有量的柱至少 8% 高，免得小量看不见（没有量的日子不画柱，只剩行底的基线） */
function barHeight(value: number, max: number): string {
  return `${Math.max((value / max) * 100, 8)}%`
}

const dayLabel = (index: number) => props.days[index]?.slice(5) ?? ''
</script>
