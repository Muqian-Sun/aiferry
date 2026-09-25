<template>
  <!-- 不带卡片外框（A7）：由调用方用 hairline 分节。原多色环形图换成与模型分布同一种单色占比列 -->
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h3 class="text-base font-semibold text-af-ink">
        {{ title || t('usage.endpointDistribution') }}
      </h3>
      <div class="flex flex-wrap items-center justify-end gap-2">
        <div
          v-if="showSourceToggle"
          class="inline-flex rounded-lg border border-af-hairline bg-af-sunken p-0.5"
        >
          <button
            type="button"
            class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
            :class="source === 'inbound'
              ? 'bg-af-sheet text-af-ink'
              : 'text-af-ink-3 hover:text-af-ink-2'"
            @click="emit('update:source', 'inbound')"
          >
            {{ t('usage.inbound') }}
          </button>
          <button
            type="button"
            class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
            :class="source === 'upstream'
              ? 'bg-af-sheet text-af-ink'
              : 'text-af-ink-3 hover:text-af-ink-2'"
            @click="emit('update:source', 'upstream')"
          >
            {{ t('usage.upstream') }}
          </button>
          <button
            type="button"
            class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
            :class="source === 'path'
              ? 'bg-af-sheet text-af-ink'
              : 'text-af-ink-3 hover:text-af-ink-2'"
            @click="emit('update:source', 'path')"
          >
            {{ t('usage.path') }}
          </button>
        </div>

        <div
          v-if="showMetricToggle"
          class="inline-flex rounded-lg border border-af-hairline bg-af-sunken p-0.5"
        >
          <button
            type="button"
            class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
            :class="metric === 'tokens'
              ? 'bg-af-sheet text-af-ink'
              : 'text-af-ink-3 hover:text-af-ink-2'"
            @click="emit('update:metric', 'tokens')"
          >
            {{ t('admin.dashboard.tokens') }}
          </button>
          <button
            type="button"
            class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
            :class="metric === 'actual_cost'
              ? 'bg-af-sheet text-af-ink'
              : 'text-af-ink-3 hover:text-af-ink-2'"
            @click="emit('update:metric', 'actual_cost')"
          >
            {{ t('common.money.revenue') }}
          </button>
        </div>
      </div>
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
              <th class="pb-2 text-right" :title="t('common.money.revenueHint')">{{ t('common.money.revenue') }}</th>
              <th class="pb-2 text-right" :title="t('common.money.costHint')">{{ t('common.money.cost') }}</th>
              <th class="pb-2 text-right" :title="t('common.money.profitHint')">{{ t('common.money.profit') }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="item in displayEndpointStats" :key="item.endpoint">
              <tr
                class="border-t border-af-hairline transition-colors"
                :class="enableBreakdown ? 'cursor-pointer hover:bg-af-sunken' : ''"
                @click="enableBreakdown && toggleBreakdown(item.endpoint)"
              >
                <td class="max-w-[180px] truncate py-1.5 font-medium" :class="enableBreakdown ? 'text-af-ink-2 hover:text-af-ink' : 'text-af-ink'" :title="item.endpoint">
                  <span class="inline-flex items-center gap-1">
                    <svg v-if="enableBreakdown && expandedKey === item.endpoint" class="h-3 w-3 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
                    <svg v-else-if="enableBreakdown" class="h-3 w-3 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
                    {{ item.endpoint }}
                  </span>
                </td>
                <td class="w-32 py-1.5 pl-3">
                  <ShareBar :value="metricValue(item)" :total="metricTotal" />
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
              <UserBreakdownSubTable
                v-if="expandedKey === item.endpoint"
                :items="breakdownItems"
                :loading="breakdownLoading"
              />
            </template>
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
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserBreakdownSubTable from './UserBreakdownSubTable.vue'
import ShareBar from './ShareBar.vue'
import { formatMoney, profitOf, profitTextClass } from '@/utils/money'
import type { EndpointStat, UserBreakdownItem } from '@/types'
import type { UserBreakdownLoader } from './userBreakdown'

const { t } = useI18n()

type DistributionMetric = 'tokens' | 'actual_cost'
type EndpointSource = 'inbound' | 'upstream' | 'path'

const props = withDefaults(
  defineProps<{
    endpointStats: EndpointStat[]
    upstreamEndpointStats?: EndpointStat[]
    endpointPathStats?: EndpointStat[]
    loading?: boolean
    title?: string
    metric?: DistributionMetric
    source?: EndpointSource
    showMetricToggle?: boolean
    showSourceToggle?: boolean
    /** 按用户下钻的加载函数；不传则不提供下钻（用户站复用本图表时不传）。 */
    loadUserBreakdown?: UserBreakdownLoader
    startDate?: string
    endDate?: string
    filters?: Record<string, any>
  }>(),
  {
    upstreamEndpointStats: () => [],
    endpointPathStats: () => [],
    loading: false,
    title: '',
    metric: 'tokens',
    source: 'inbound',
    showMetricToggle: false,
    showSourceToggle: false
  }
)

const emit = defineEmits<{
  'update:metric': [value: DistributionMetric]
  'update:source': [value: EndpointSource]
}>()

const enableBreakdown = computed(() => props.loadUserBreakdown !== undefined)
const expandedKey = ref<string | null>(null)
const breakdownItems = ref<UserBreakdownItem[]>([])
const breakdownLoading = ref(false)

const toggleBreakdown = async (endpoint: string) => {
  const loadUserBreakdown = props.loadUserBreakdown
  if (!loadUserBreakdown) return
  if (expandedKey.value === endpoint) {
    expandedKey.value = null
    return
  }
  expandedKey.value = endpoint
  breakdownLoading.value = true
  breakdownItems.value = []
  try {
    const res = await loadUserBreakdown({
      ...props.filters,
      start_date: props.startDate,
      end_date: props.endDate,
      endpoint,
      endpoint_type: props.source,
    })
    breakdownItems.value = res.users || []
  } catch {
    breakdownItems.value = []
  } finally {
    breakdownLoading.value = false
  }
}

const displayEndpointStats = computed(() => {
  const sourceStats = props.source === 'upstream'
    ? props.upstreamEndpointStats
    : props.source === 'path'
      ? props.endpointPathStats
      : props.endpointStats
  if (!sourceStats?.length) return []

  const metricKey = props.metric === 'actual_cost' ? 'actual_cost' : 'total_tokens'
  return [...sourceStats].sort((a, b) => b[metricKey] - a[metricKey])
})

/** 占比按当前指标（Token / 收入） */
const metricValue = (item: EndpointStat) => (props.metric === 'actual_cost' ? item.actual_cost : item.total_tokens)
const metricTotal = computed(() => displayEndpointStats.value.reduce((sum, item) => sum + metricValue(item), 0))

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
