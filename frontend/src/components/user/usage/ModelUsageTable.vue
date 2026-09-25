<template>
  <!--
    费用分布（用量明细页）：表格 + 行内占比条（按实付费用），替代环形图——排序清楚、能读数、无需图例。
    占比条一律墨色（控制台单色为主，muqian 2026-09-23）；前 5 行 + 「其他」（muqian 2026-09-25：让请求明细回到首屏附近）。
    点一行 = 在请求明细上加这个模型的筛选，当前筛着的那行加粗；「其他」不可点。
  -->
  <div class="overflow-x-auto">
    <table class="w-full min-w-[560px] text-13" data-testid="model-usage-table">
      <thead>
        <tr class="border-b border-af-hairline text-left text-af-ink-3">
          <th class="py-2 pr-4 font-medium">{{ t('dashboard.model') }}</th>
          <th class="w-[28%] py-2 pr-4 font-medium">{{ t('userUi.usage.costShare') }}</th>
          <th class="py-2 pr-4 text-right font-medium">{{ t('dashboard.requests') }}</th>
          <th class="py-2 pr-4 text-right font-medium">{{ t('dashboard.tokens') }}</th>
          <th class="py-2 text-right font-medium">{{ t('usage.cost') }}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-af-hairline">
        <tr
          v-for="row in rows"
          :key="row.key"
          class="h-11"
          :class="row.other ? '' : 'cursor-pointer transition-colors hover:bg-af-sunken'"
          :data-testid="row.other ? undefined : 'model-usage-row'"
          :aria-selected="row.other ? undefined : row.model === selectedModel"
          :tabindex="row.other ? undefined : 0"
          @click="!row.other && emit('select', row.model)"
          @keydown.enter="!row.other && emit('select', row.model)"
        >
          <td
            class="max-w-[260px] truncate pr-4 font-mono"
            :class="row.model === selectedModel ? 'font-semibold text-af-ink' : row.other ? 'text-af-ink-3' : 'text-af-ink'"
            :title="row.model"
          >
            {{ row.model }}
          </td>
          <td class="pr-4"><ShareBar :value="row.actual_cost" :total="total" /></td>
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
import ShareBar from '@/components/charts/ShareBar.vue'
import { formatCurrency, formatNumber, formatTokensK } from '@/utils/format'
import type { ModelStat } from '@/types'

const props = withDefaults(defineProps<{ models: ModelStat[]; selectedModel?: string; limit?: number }>(), {
  selectedModel: '',
  limit: 5
})
const emit = defineEmits<{ (e: 'select', model: string): void }>()
const { t } = useI18n()

interface Row {
  key: string
  model: string
  other: boolean
  requests: number
  total_tokens: number
  actual_cost: number
}

const total = computed(() => props.models.reduce((sum, item) => sum + item.actual_cost, 0))

const rows = computed<Row[]>(() => {
  const sorted = [...props.models].sort((a, b) => b.actual_cost - a.actual_cost)
  const toRow = (item: ModelStat): Row => ({
    key: item.model,
    model: item.model,
    other: false,
    requests: item.requests,
    total_tokens: item.total_tokens,
    actual_cost: item.actual_cost
  })
  // 只多出一个时直接列出来，不为一行开「其他」
  if (sorted.length <= props.limit + 1) return sorted.map(toRow)
  const tail = sorted.slice(props.limit)
  return [
    ...sorted.slice(0, props.limit).map(toRow),
    {
      key: '__other__',
      model: t('dashboard.platformOther'),
      other: true,
      requests: tail.reduce((sum, item) => sum + item.requests, 0),
      total_tokens: tail.reduce((sum, item) => sum + item.total_tokens, 0),
      actual_cost: tail.reduce((sum, item) => sum + item.actual_cost, 0)
    }
  ]
})
</script>
