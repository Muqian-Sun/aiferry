<template>
  <!--
    概览模型表：色块与上方柱状图一致（modelSeries）；6 列与 UserBreakdownSubTable 对齐，占比跟在 Token 后面（与用户表一致），
    点行展开「这个模型被哪些用户在用」。
    默认前 limit 行，其余点「显示全部」，不在框里滚动。
  -->
  <div v-if="loading" class="flex h-32 items-center justify-center">
    <LoadingSpinner />
  </div>
  <div v-else-if="rows.length" class="overflow-x-auto">
    <table class="w-full text-xs" data-testid="dashboard-model-table">
      <thead>
        <tr class="text-af-ink-3">
          <th class="pb-2 text-left font-medium">{{ t('admin.dashboard.model') }}</th>
          <th class="pb-2 text-right font-medium">{{ t('admin.dashboard.requests') }}</th>
          <th class="pb-2 text-right font-medium">{{ t('admin.dashboard.tokens') }}</th>
          <th class="pb-2 text-right font-medium">{{ t('common.money.revenue') }}</th>
          <th class="pb-2 text-right font-medium">{{ t('common.money.cost') }}</th>
          <th class="pb-2 text-right font-medium" :title="t('common.money.profitHint')">{{ t('common.money.profit') }}</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="model in visibleRows" :key="model.model">
          <tr class="cursor-pointer border-t border-af-hairline transition-colors hover:bg-af-sunken" @click="toggle(model.model)">
            <td class="max-w-[260px] py-2 pr-3">
              <span class="flex min-w-0 items-center gap-2 font-medium text-af-ink" :title="model.model">
                <Icon :name="expanded === model.model ? 'chevronDown' : 'chevronRight'" size="xs" class="shrink-0 text-af-ink-3" />
                <span class="h-2.5 w-2.5 shrink-0 rounded-sm" :style="{ background: modelSwatchColor(series, model.model) }" aria-hidden="true" />
                <span class="truncate">{{ model.model }}</span>
              </span>
            </td>
            <td class="py-2 text-right tabular-nums text-af-ink-2">{{ formatCount(model.requests) }}</td>
            <td class="py-2 text-right tabular-nums text-af-ink">
              {{ formatTokens(model.total_tokens) }} <span class="text-af-ink-3">{{ formatShare(model.total_tokens) }}</span>
            </td>
            <td class="py-2 text-right tabular-nums text-af-ink">{{ formatMoney(model.actual_cost) }}</td>
            <td class="py-2 text-right tabular-nums text-af-ink-3">{{ formatMoney(model.account_cost) }}</td>
            <td class="py-2 text-right tabular-nums" :class="profitTextClass(profitOf(model.actual_cost, model.account_cost)) || 'text-af-ink-2'">
              {{ formatMoney(profitOf(model.actual_cost, model.account_cost)) }}
            </td>
          </tr>
          <UserBreakdownSubTable v-if="expanded === model.model" :items="breakdownItems" :loading="breakdownLoading" />
        </template>
      </tbody>
    </table>
    <button
      v-if="rows.length > limit"
      type="button"
      class="mt-3 inline-flex items-center gap-1 text-xs text-af-ink-3 hover:text-af-ink"
      @click="showAll = !showAll"
    >
      {{ showAll ? t('admin.dashboard.showLess') : t('admin.dashboard.showAll', { count: rows.length }) }}
      <Icon :name="showAll ? 'chevronUp' : 'chevronDown'" size="xs" />
    </button>
  </div>
  <div v-else class="flex h-32 items-center justify-center text-sm text-af-ink-3">
    {{ t('admin.dashboard.noDataAvailable') }}
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserBreakdownSubTable from '@/components/charts/UserBreakdownSubTable.vue'
import type { UserBreakdownLoader } from '@/components/charts/userBreakdown'
import { formatMoney, profitOf, profitTextClass } from '@/utils/money'
import { formatSharePercent } from '@/utils/format'
import type { ModelStat, UserBreakdownItem } from '@/types'
import type { RangeParams } from '@/utils/dateRange'
import { modelSwatchColor, modelsByTokens, type ModelSeries } from './modelSeries'

const props = withDefaults(defineProps<{
  modelStats: ModelStat[]
  series: ModelSeries
  loadUserBreakdown: UserBreakdownLoader
  /** 与概览图表同一个时间范围（近 24 小时是精确时刻） */
  range: RangeParams
  loading?: boolean
  limit?: number
}>(), {
  loading: false,
  limit: 10
})

const { t } = useI18n()

const rows = computed(() => modelsByTokens(props.modelStats ?? []))
const showAll = ref(false)
const visibleRows = computed(() => (showAll.value ? rows.value : rows.value.slice(0, props.limit)))
const tokenTotal = computed(() => rows.value.reduce((sum, m) => sum + (Number(m.total_tokens) || 0), 0))

const expanded = ref<string | null>(null)
const breakdownItems = ref<UserBreakdownItem[]>([])
const breakdownLoading = ref(false)

async function toggle(model: string) {
  if (expanded.value === model) {
    expanded.value = null
    return
  }
  expanded.value = model
  breakdownLoading.value = true
  breakdownItems.value = []
  try {
    const res = await props.loadUserBreakdown({ ...props.range, model, model_source: 'requested' })
    if (expanded.value === model) breakdownItems.value = res.users || []
  } catch {
    if (expanded.value === model) breakdownItems.value = []
  } finally {
    if (expanded.value === model) breakdownLoading.value = false
  }
}

const formatShare = (tokens: number): string => {
  if (tokenTotal.value <= 0) return '0%'
  return formatSharePercent(((Number(tokens) || 0) / tokenTotal.value) * 100)
}
const formatCount = (value: number): string => (Number(value) || 0).toLocaleString()
const formatTokens = (value: number): string => {
  const v = Number(value) || 0
  if (v >= 1_000_000_000) return `${(v / 1_000_000_000).toFixed(2)}B`
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}M`
  if (v >= 1_000) return `${(v / 1_000).toFixed(2)}K`
  return v.toLocaleString()
}
</script>
