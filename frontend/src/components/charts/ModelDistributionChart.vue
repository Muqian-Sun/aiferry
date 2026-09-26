<template>
  <!-- 不带卡片外框（A7）：由调用方用 hairline 分节；标题随「模型分布 / 消费排行」切换 -->
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h3 class="text-base font-semibold text-af-ink">
        {{ !enableRankingView || activeView === 'model_distribution'
          ? t('admin.dashboard.modelDistribution')
          : t('admin.dashboard.spendingRankingTitle') }}
      </h3>
      <div class="flex flex-wrap items-center justify-end gap-2">
        <div v-if="enableRankingView" class="inline-flex rounded-lg bg-af-sunken p-1">
          <button
            type="button"
            class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
            :class="
              activeView === 'model_distribution'
                ? 'bg-af-sheet text-af-ink'
                : 'text-af-ink-3 hover:text-af-ink-2'
            "
            @click="activeView = 'model_distribution'"
          >
            {{ t('admin.dashboard.viewModelDistribution') }}
          </button>
          <button
            type="button"
            class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
            :class="
              activeView === 'spending_ranking'
                ? 'bg-af-sheet text-af-ink'
                : 'text-af-ink-3 hover:text-af-ink-2'
            "
            @click="activeView = 'spending_ranking'"
          >
            {{ t('admin.dashboard.viewSpendingRanking') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="activeView === 'model_distribution' && loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="activeView === 'model_distribution' && displayModelStats.length > 0">
      <div class="max-h-72 w-full overflow-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="text-af-ink-3">
              <th class="pb-2 text-left">{{ t('admin.dashboard.model') }}</th>
              <th class="pb-2 pl-3 text-left">{{ t('admin.dashboard.share') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
              <th class="pb-2 text-right" :title="t('common.money.revenueHint')">{{ t('common.money.revenue') }}</th>
              <th class="pb-2 text-right" :title="t('common.money.costHint')">{{ t('common.money.cost') }}</th>
              <th class="pb-2 text-right" :title="t('common.money.profitHint')">{{ t('common.money.profit') }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="model in displayModelStats" :key="model.model">
              <tr
                class="border-t border-af-hairline transition-colors"
                :class="enableBreakdown ? 'cursor-pointer hover:bg-af-sunken' : ''"
                @click="enableBreakdown && toggleBreakdown('model', model.model)"
              >
                <td
                  class="max-w-[180px] truncate py-1.5 font-medium"
                  :class="enableBreakdown ? 'text-af-ink-2 hover:text-af-ink' : 'text-af-ink'"
                  :title="model.model"
                >
                  <span class="inline-flex items-center gap-1">
                    <svg v-if="enableBreakdown && expandedKey === `model-${model.model}`" class="h-3 w-3 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
                    <svg v-else-if="enableBreakdown" class="h-3 w-3 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
                    {{ model.model }}
                  </span>
                </td>
                <td class="w-32 py-1.5 pl-3">
                  <ShareBar :value="modelMetricValue(model)" :total="modelMetricTotal" />
                </td>
                <td class="py-1.5 text-right text-af-ink-2">
                  {{ formatNumber(model.requests) }}
                </td>
                <td class="py-1.5 text-right text-af-ink-2">
                  {{ formatTokens(model.total_tokens) }}
                </td>
                <td class="py-1.5 text-right tabular-nums text-af-ink">
                  {{ formatMoney(model.actual_cost) }}
                </td>
                <td class="py-1.5 text-right tabular-nums text-af-ink-3">
                  {{ formatMoney(model.account_cost) }}
                </td>
                <td class="py-1.5 text-right tabular-nums" :class="profitTextClass(profitOf(model.actual_cost, model.account_cost)) || 'text-af-ink-2'">
                  {{ formatMoney(profitOf(model.actual_cost, model.account_cost)) }}
                </td>
              </tr>
              <UserBreakdownSubTable
                v-if="expandedKey === `model-${model.model}`"
                :items="breakdownItems"
                :loading="breakdownLoading"
              />
            </template>
          </tbody>
        </table>
      </div>
    </div>
    <div
      v-else-if="activeView === 'model_distribution'"
      class="flex h-48 items-center justify-center text-sm text-af-ink-3"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>

    <div v-else-if="rankingLoading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div
      v-else-if="rankingError"
      class="flex h-48 items-center justify-center text-sm text-af-ink-3"
    >
      {{ t('admin.dashboard.failedToLoad') }}
    </div>
    <div v-else-if="rankingDisplayItems.length > 0">
      <div class="max-h-72 w-full overflow-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="text-af-ink-3">
              <th class="pb-2 text-left">{{ t('admin.dashboard.spendingRankingUser') }}</th>
              <th class="pb-2 pl-3 text-left">{{ t('admin.dashboard.share') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.spendingRankingRequests') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.spendingRankingTokens') }}</th>
              <th class="pb-2 text-right" :title="t('common.money.revenueHint')">{{ t('common.money.revenue') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(item, index) in rankingDisplayItems"
              :key="item.isOther ? 'others' : `${item.user_id}-${index}`"
              class="border-t border-af-hairline transition-colors"
              :class="item.isOther
                ? 'bg-af-sunken/70'
                : 'cursor-pointer hover:bg-af-sunken'"
              @click="item.isOther ? undefined : emit('ranking-click', item)"
            >
              <td class="py-1.5">
                <div class="flex min-w-0 items-center gap-2">
                  <span class="shrink-0 text-[11px] font-semibold text-af-ink-3">
                    {{ item.isOther ? 'Σ' : `#${index + 1}` }}
                  </span>
                  <span
                    class="block max-w-[140px] truncate font-medium text-af-ink"
                    :title="getRankingRowLabel(item)"
                  >
                    {{ getRankingRowLabel(item) }}
                  </span>
                </div>
              </td>
              <td class="w-32 py-1.5 pl-3">
                <ShareBar :value="toFiniteNumber(item.actual_cost)" :total="rankingCostTotal" />
              </td>
              <td class="py-1.5 text-right text-af-ink-2">
                {{ formatNumber(item.requests) }}
              </td>
              <td class="py-1.5 text-right text-af-ink-2">
                {{ formatTokens(item.tokens) }}
              </td>
              <td class="py-1.5 text-right tabular-nums text-af-ink">
                {{ formatMoney(item.actual_cost) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <div
      v-else
      class="flex h-48 items-center justify-center text-sm text-af-ink-3"
    >
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
import type { ModelStat, UserSpendingRankingItem, UserBreakdownItem } from '@/types'
import type { UserBreakdownLoader } from './userBreakdown'

const { t } = useI18n()

type RankingDisplayItem = UserSpendingRankingItem & { isOther?: boolean }
// 用量页「分析」页签删了之后，调用方只剩概览（带下钻、消费榜）和渠道抽屉（只看表），
// 原来给分析页签用的「请求 / 上游 / 映射」来源切换和「Token / 收入」指标切换一并去掉：按请求模型、按 Token 排
const props = withDefaults(defineProps<{
  modelStats: ModelStat[]
  enableRankingView?: boolean
  rankingItems?: UserSpendingRankingItem[]
  rankingTotalActualCost?: number
  rankingTotalRequests?: number
  rankingTotalTokens?: number
  loading?: boolean
  /** 按用户下钻的加载函数；不传则不提供下钻（渠道抽屉不传）。 */
  loadUserBreakdown?: UserBreakdownLoader
  rankingLoading?: boolean
  rankingError?: boolean
  startDate?: string
  endDate?: string
}>(), {
  enableRankingView: false,
  rankingItems: () => [],
  rankingTotalActualCost: 0,
  rankingTotalRequests: 0,
  rankingTotalTokens: 0,
  loading: false,
  rankingLoading: false,
  rankingError: false
})

const enableBreakdown = computed(() => props.loadUserBreakdown !== undefined)
const expandedKey = ref<string | null>(null)
const breakdownItems = ref<UserBreakdownItem[]>([])
const breakdownLoading = ref(false)

const toggleBreakdown = async (type: string, id: string) => {
  const loadUserBreakdown = props.loadUserBreakdown
  if (!loadUserBreakdown) return
  const key = `${type}-${id}`
  if (expandedKey.value === key) {
    expandedKey.value = null
    return
  }
  expandedKey.value = key
  breakdownLoading.value = true
  breakdownItems.value = []
  try {
    const res = await loadUserBreakdown({
      start_date: props.startDate,
      end_date: props.endDate,
      model: id,
      model_source: 'requested',
    })
    breakdownItems.value = res.users || []
  } catch {
    breakdownItems.value = []
  } finally {
    breakdownLoading.value = false
  }
}

const emit = defineEmits<{
  'ranking-click': [item: UserSpendingRankingItem]
}>()

const enableRankingView = computed(() => props.enableRankingView)
const activeView = ref<'model_distribution' | 'spending_ranking'>('model_distribution')

const displayModelStats = computed(() => {
  if (!props.modelStats?.length) return []
  return [...props.modelStats].sort((a, b) => toFiniteNumber(b.total_tokens) - toFiniteNumber(a.total_tokens))
})

/** 占比条：模型按 Token，消费榜按收入 */
const modelMetricValue = (m: ModelStat) => toFiniteNumber(m.total_tokens)
const modelMetricTotal = computed(() => displayModelStats.value.reduce((sum, m) => sum + modelMetricValue(m), 0))
const rankingCostTotal = computed(() => rankingDisplayItems.value.reduce((sum, item) => sum + toFiniteNumber(item.actual_cost), 0))

const otherRankingItem = computed<RankingDisplayItem | null>(() => {
  if (!props.rankingItems?.length) return null

  const rankedActualCost = props.rankingItems.reduce((sum, item) => sum + toFiniteNumber(item.actual_cost), 0)
  const rankedRequests = props.rankingItems.reduce((sum, item) => sum + toFiniteNumber(item.requests), 0)
  const rankedTokens = props.rankingItems.reduce((sum, item) => sum + toFiniteNumber(item.tokens), 0)

  const otherActualCost = Math.max((props.rankingTotalActualCost || 0) - rankedActualCost, 0)
  const otherRequests = Math.max((props.rankingTotalRequests || 0) - rankedRequests, 0)
  const otherTokens = Math.max((props.rankingTotalTokens || 0) - rankedTokens, 0)

  if (otherActualCost <= 0.000001 && otherRequests <= 0 && otherTokens <= 0) return null

  return {
    user_id: 0,
    email: '',
    username: '',
    actual_cost: otherActualCost,
    requests: otherRequests,
    tokens: otherTokens,
    isOther: true
  }
})

const rankingDisplayItems = computed<RankingDisplayItem[]>(() => {
  if (!props.rankingItems?.length) return []
  return otherRankingItem.value
    ? [...props.rankingItems, otherRankingItem.value]
    : [...props.rankingItems]
})

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
  return toFiniteNumber(value).toLocaleString()
}

const getRankingUserLabel = (item: UserSpendingRankingItem): string => {
  if (item.username?.trim()) return item.username.trim()
  if (item.email?.trim()) return item.email.trim()
  return t('common.deletedUser')
}

const getRankingRowLabel = (item: RankingDisplayItem): string => {
  if (item.isOther) return t('admin.dashboard.spendingRankingOther')
  return getRankingUserLabel(item)
}

const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}
</script>
