<template>
  <!--
    不带卡片外框（A7）：由调用方用 hairline 分节。原多色环形图换成与模型分布同一种单色占比列。
    调用方只剩渠道抽屉（入站 / 上游端点各一张，只看表）；原来给用量页「分析」页签用的来源切换、指标切换、
    按用户下钻随那个页签一起删了。按 Token 排、占比按 Token。
  -->
  <div>
    <div class="mb-4">
      <h3 class="text-base font-semibold text-af-ink">
        {{ title || t('usage.endpointDistribution') }}
      </h3>
    </div>
    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="displayEndpointStats.length > 0">
      <div class="max-h-72 w-full overflow-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="text-af-ink-3">
              <th class="pb-2 text-left">{{ t('usage.endpoint') }}</th>
              <th class="pb-2 pl-3 text-left">{{ t('admin.dashboard.share') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
              <th class="pb-2 text-right">{{ t('common.money.revenue') }}</th>
              <th class="pb-2 text-right">{{ t('common.money.cost') }}</th>
              <th class="pb-2 text-right" :title="t('common.money.profitHint')">{{ t('common.money.profit') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in displayEndpointStats"
              :key="item.endpoint"
              class="border-t border-af-hairline"
            >
              <td class="max-w-[180px] truncate py-1.5 font-medium text-af-ink" :title="item.endpoint">
                {{ item.endpoint }}
              </td>
              <td class="w-32 py-1.5 pl-3">
                <ShareBar :value="item.total_tokens" :total="tokenTotal" />
              </td>
              <td class="py-1.5 text-right text-af-ink-2">
                {{ formatNumber(item.requests) }}
              </td>
              <td class="py-1.5 text-right text-af-ink-2">
                {{ formatTokens(item.total_tokens) }}
              </td>
              <td class="py-1.5 text-right tabular-nums text-af-ink">
                {{ formatMoney(item.actual_cost) }}
              </td>
              <td class="py-1.5 text-right tabular-nums text-af-ink-3">
                {{ formatMoney(item.account_cost) }}
              </td>
              <td class="py-1.5 text-right tabular-nums" :class="profitTextClass(profitOf(item.actual_cost, item.account_cost)) || 'text-af-ink-2'">
                {{ formatMoney(profitOf(item.actual_cost, item.account_cost)) }}
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
import type { EndpointStat } from '@/types'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    endpointStats: EndpointStat[]
    loading?: boolean
    title?: string
  }>(),
  {
    loading: false,
    title: ''
  }
)

const displayEndpointStats = computed(() => {
  if (!props.endpointStats?.length) return []
  return [...props.endpointStats].sort((a, b) => b.total_tokens - a.total_tokens)
})

const tokenTotal = computed(() => displayEndpointStats.value.reduce((sum, item) => sum + item.total_tokens, 0))

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatNumber = (value: number): string => {
  return value.toLocaleString()
}
</script>
