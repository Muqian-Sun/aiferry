<template>
  <!--
    模型用量（概览）：按 Token 排的前 5 + 其他，墨色占比条 + Token + 实付。替代原来每个模型一行的迷你柱（muqian 2026-09-25「迷你柱不太好看」）：
    「什么时候用得多」交给上面的趋势线，这里只回答「用在哪几个模型上」，能直接读数、比大小。
    点一行去用量明细、筛好这个模型和同一时间范围；「其他」不可点。
  -->
  <div v-if="loading" class="flex h-40 items-center justify-center">
    <span class="spinner text-af-ink-3" />
  </div>
  <!-- 定宽列：模型名给足 40%，长名截断时悬停看全名 -->
  <table v-else-if="rows.length" class="w-full table-fixed text-13" data-testid="model-token-share">
    <thead>
      <tr class="border-b border-af-hairline text-left text-af-ink-3">
        <th class="w-[40%] py-2 pr-4 font-medium">{{ t('dashboard.model') }}</th>
        <th class="w-[30%] py-2 pr-4 font-medium">{{ t('userUi.overview.share') }}</th>
        <th class="w-[15%] py-2 pr-4 text-right font-medium">{{ t('userUi.usage.stats.tokens') }}</th>
        <th class="w-[15%] py-2 text-right font-medium">{{ t('userUi.usage.stats.actualCost') }}</th>
      </tr>
    </thead>
    <tbody class="divide-y divide-af-hairline">
      <tr v-for="row in rows" :key="row.key" class="h-11">
        <td class="truncate pr-4 font-mono" :title="row.model">
          <span v-if="row.other" class="text-af-ink-3">{{ row.model }}</span>
          <RouterLink
            v-else
            :to="{ path: '/usage', query: { ...rangeQuery, model: row.model } }"
            class="text-af-ink hover:underline"
            data-testid="model-token-share-link"
          >
            {{ row.model }}
          </RouterLink>
        </td>
        <td class="pr-4"><ShareBar :value="row.total_tokens" :total="total" /></td>
        <td class="pr-4 text-right tabular-nums text-af-ink-2">{{ formatTokensK(row.total_tokens) }}</td>
        <td class="text-right tabular-nums text-af-ink">{{ formatCurrency(row.actual_cost) }}</td>
      </tr>
    </tbody>
  </table>
  <div v-else class="flex h-40 items-center justify-center text-sm text-af-ink-3">
    {{ t('userUi.overview.models.empty') }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ShareBar from '@/components/charts/ShareBar.vue'
import { formatCurrency, formatTokensK } from '@/utils/format'
import type { ModelStat } from '@/types'

const props = withDefaults(
  defineProps<{
    models: ModelStat[]
    /** 跳转用量明细时带上的时间范围 */
    rangeQuery: { start: string; end: string }
    loading?: boolean
    limit?: number
  }>(),
  { loading: false, limit: 5 }
)

const { t } = useI18n()

interface Row {
  key: string
  model: string
  other: boolean
  total_tokens: number
  actual_cost: number
}

const total = computed(() => props.models.reduce((sum, item) => sum + item.total_tokens, 0))

const rows = computed<Row[]>(() => {
  const sorted = [...props.models].filter((item) => item.total_tokens > 0).sort((a, b) => b.total_tokens - a.total_tokens)
  const toRow = (item: ModelStat): Row => ({ key: item.model, model: item.model, other: false, total_tokens: item.total_tokens, actual_cost: item.actual_cost })
  // 只多出一个时直接列出来，不为一行开「其他」
  if (sorted.length <= props.limit + 1) return sorted.map(toRow)
  const tail = sorted.slice(props.limit)
  return [
    ...sorted.slice(0, props.limit).map(toRow),
    {
      key: '__other__',
      model: t('userUi.overview.models.other'),
      other: true,
      total_tokens: tail.reduce((sum, item) => sum + item.total_tokens, 0),
      actual_cost: tail.reduce((sum, item) => sum + item.actual_cost, 0)
    }
  ]
})
</script>
