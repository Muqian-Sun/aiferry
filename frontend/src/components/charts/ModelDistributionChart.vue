<template>
  <!--
    不带卡片外框（A7）：由调用方用 hairline 分节。现在只有渠道抽屉在用（只看表）；
    概览的模型 / 用户拆分改成 components/admin/dashboard 下的专用组件，消费榜与下钻随之移走。
  -->
  <div>
    <h3 class="mb-4 text-base font-semibold text-af-ink">{{ t('admin.dashboard.modelDistribution') }}</h3>

    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="displayModelStats.length > 0">
      <div class="max-h-72 w-full overflow-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="text-af-ink-3">
              <th class="pb-2 text-left">{{ t('admin.dashboard.model') }}</th>
              <th class="pb-2 pl-3 text-left">{{ t('admin.dashboard.share') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
              <th class="pb-2 text-right">{{ t('common.money.revenue') }}</th>
              <th class="pb-2 text-right">{{ t('common.money.cost') }}</th>
              <th class="pb-2 text-right" :title="t('common.money.profitHint')">{{ t('common.money.profit') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="model in displayModelStats" :key="model.model" class="border-t border-af-hairline">
              <td class="max-w-[180px] truncate py-1.5 font-medium text-af-ink" :title="model.model">{{ model.model }}</td>
              <td class="w-32 py-1.5 pl-3">
                <ShareBar :value="toFiniteNumber(model.total_tokens)" :total="tokenTotal" />
              </td>
              <td class="py-1.5 text-right text-af-ink-2">{{ formatNumber(model.requests) }}</td>
              <td class="py-1.5 text-right text-af-ink-2">{{ formatTokens(model.total_tokens) }}</td>
              <td class="py-1.5 text-right tabular-nums text-af-ink">{{ formatMoney(model.actual_cost) }}</td>
              <td class="py-1.5 text-right tabular-nums text-af-ink-3">{{ formatMoney(model.account_cost) }}</td>
              <td class="py-1.5 text-right tabular-nums" :class="profitTextClass(profitOf(model.actual_cost, model.account_cost)) || 'text-af-ink-2'">
                {{ formatMoney(profitOf(model.actual_cost, model.account_cost)) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <div v-else class="flex h-48 items-center justify-center text-sm text-af-ink-3">
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ShareBar from './ShareBar.vue'
import { formatMoney, profitOf, profitTextClass } from '@/utils/money'
import type { ModelStat } from '@/types'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  modelStats: ModelStat[]
  loading?: boolean
}>(), {
  loading: false
})

const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

/** 按请求模型、按 Token 排；占比条按 Token */
const displayModelStats = computed(() =>
  [...(props.modelStats ?? [])].sort((a, b) => toFiniteNumber(b.total_tokens) - toFiniteNumber(a.total_tokens))
)
const tokenTotal = computed(() => displayModelStats.value.reduce((sum, m) => sum + toFiniteNumber(m.total_tokens), 0))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}

const formatNumber = (value: number): string => toFiniteNumber(value).toLocaleString()
</script>
