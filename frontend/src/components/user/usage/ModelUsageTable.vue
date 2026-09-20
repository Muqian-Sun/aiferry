<template>
  <!--
    模型用量：表格 + 行内占比条（按实付费用），替代环形图——排序清楚、能读数、无需图例。
    最多显示前 8 个，其余合并为「其他」（分类色只有 8 槽，且长尾模型没有阅读价值）。
  -->
  <div class="overflow-x-auto">
    <table class="w-full min-w-[560px] text-13" data-testid="model-usage-table">
      <thead>
        <tr class="border-b border-af-hairline text-left text-af-ink-3">
          <th class="py-2 pr-4 font-medium">{{ t('dashboard.model') }}</th>
          <th class="w-[28%] py-2 pr-4 font-medium">{{ t('userUi.usage.share') }}</th>
          <th class="py-2 pr-4 text-right font-medium">{{ t('dashboard.requests') }}</th>
          <th class="py-2 pr-4 text-right font-medium">{{ t('dashboard.tokens') }}</th>
          <th class="py-2 text-right font-medium">{{ t('usage.cost') }}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-af-hairline">
        <tr v-for="row in rows" :key="row.model" class="h-11">
          <td class="max-w-[260px] truncate pr-4 font-mono text-af-ink" :title="row.model">{{ row.model }}</td>
          <td class="pr-4">
            <div class="flex items-center gap-2">
              <div class="h-1.5 flex-1 overflow-hidden rounded-full bg-af-sunken">
                <div class="h-full rounded-full" :style="{ width: `${row.share}%`, backgroundColor: row.color }"></div>
              </div>
              <span class="w-12 shrink-0 text-right tabular-nums text-af-ink-3">{{ row.share.toFixed(1) }}%</span>
            </div>
          </td>
          <td class="pr-4 text-right tabular-nums text-af-ink-2">{{ formatNumber(row.requests) }}</td>
          <td class="pr-4 text-right tabular-nums text-af-ink-2">{{ formatTokensK(row.total_tokens) }}</td>
          <td class="text-right tabular-nums text-af-ink">{{ formatCurrency(row.actual_cost) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { CHART_SERIES_SLOTS, useChartTheme } from '@/composables/useChartTheme'
import { formatCurrency, formatNumber, formatTokensK } from '@/utils/format'
import type { ModelStat } from '@/types'

const props = defineProps<{ models: ModelStat[] }>()
const { t } = useI18n()
const theme = useChartTheme()

interface Row {
  model: string
  requests: number
  total_tokens: number
  actual_cost: number
  share: number
  color: string
}

const rows = computed<Row[]>(() => {
  const sorted = [...props.models].sort((a, b) => b.actual_cost - a.actual_cost)
  const total = sorted.reduce((sum, item) => sum + item.actual_cost, 0)
  const head = sorted.slice(0, CHART_SERIES_SLOTS - 1)
  const tail = sorted.slice(CHART_SERIES_SLOTS - 1)
  const merged: Array<Pick<ModelStat, 'model' | 'requests' | 'total_tokens' | 'actual_cost'>> = [...head]
  if (tail.length > 1) {
    merged.push({
      model: t('dashboard.platformOther'),
      requests: tail.reduce((sum, item) => sum + item.requests, 0),
      total_tokens: tail.reduce((sum, item) => sum + item.total_tokens, 0),
      actual_cost: tail.reduce((sum, item) => sum + item.actual_cost, 0)
    })
  } else if (tail.length === 1) {
    merged.push(tail[0])
  }
  return merged.map((item, index) => ({
    model: item.model,
    requests: item.requests,
    total_tokens: item.total_tokens,
    actual_cost: item.actual_cost,
    share: total > 0 ? (item.actual_cost / total) * 100 : 0,
    color: theme.value.color(index)
  }))
})
</script>
