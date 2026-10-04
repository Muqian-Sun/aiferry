<template>
  <!--
    概览用户表（muqian 2026-09-29）：按 Token 从高到低，第一行就是用得最多的用户；每行一条热力条看这个人的趋势
    （每格一个时间桶，颜色按本行自己的峰值归一，只看走势；多少看右边数字）。收入 / 成本 / 利润与模型表同口径。
    默认前 limit 行，其余点「显示全部」；点行去用量页看这个用户的明细。
  -->
  <div v-if="loading" class="flex h-32 items-center justify-center">
    <LoadingSpinner />
  </div>
  <div v-else-if="rows.length" class="overflow-x-auto">
    <table class="w-full text-xs" data-testid="dashboard-user-table">
      <thead>
        <tr class="text-af-ink-3">
          <th class="pb-2 text-left font-medium">{{ t('admin.dashboard.user') }}</th>
          <th class="hidden w-[36%] pb-2 font-medium md:table-cell">
            <span class="flex justify-between gap-2 px-1 font-normal tabular-nums text-af-ink-3">
              <span>{{ firstLabel }}</span><span>{{ lastLabel }}</span>
            </span>
          </th>
          <th class="pb-2 text-right font-medium">{{ t('admin.dashboard.requests') }}</th>
          <th class="pb-2 text-right font-medium">{{ t('admin.dashboard.tokens') }}</th>
          <th class="pb-2 text-right font-medium">{{ t('common.money.revenue') }}</th>
          <th class="pb-2 text-right font-medium">{{ t('common.money.cost') }}</th>
          <th class="pb-2 text-right font-medium" :title="t('common.money.profitHint')">{{ t('common.money.profit') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in visibleRows"
          :key="row.userId"
          class="cursor-pointer border-t border-af-hairline transition-colors hover:bg-af-sunken"
          data-testid="dashboard-user-row"
          @click="emit('select', row.userId)"
        >
          <td class="max-w-[200px] truncate py-2 pr-3 font-medium text-af-ink" :title="row.email || row.name">{{ row.name }}</td>
          <td class="hidden py-2 md:table-cell">
            <span class="flex gap-0.5 px-1" :aria-label="t('admin.dashboard.userTrend', { name: row.name })">
              <span
                v-for="(cell, i) in row.cells"
                :key="i"
                class="h-3.5 min-w-0 flex-1 rounded-sm"
                :class="cell > 0 ? 'bg-af-ink' : 'bg-af-hairline/70'"
                :style="cell > 0 ? { opacity: cell } : undefined"
                :title="`${days[i]} · ${formatTokens(row.buckets[i])}`"
              />
            </span>
          </td>
          <td class="py-2 text-right tabular-nums text-af-ink-2">{{ row.requests.toLocaleString() }}</td>
          <td class="py-2 text-right tabular-nums text-af-ink">
            {{ formatTokens(row.tokens) }} <span class="text-af-ink-3">{{ formatShare(row.tokens) }}</span>
          </td>
          <td class="py-2 text-right tabular-nums text-af-ink">{{ formatMoney(row.revenue) }}</td>
          <td class="py-2 text-right tabular-nums text-af-ink-3">{{ formatMoney(row.cost) }}</td>
          <td class="py-2 text-right tabular-nums" :class="profitTextClass(profitOf(row.revenue, row.cost)) || 'text-af-ink-2'">
            {{ formatMoney(profitOf(row.revenue, row.cost)) }}
          </td>
        </tr>
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
import { formatMoney, profitOf, profitTextClass } from '@/utils/money'
import { formatSharePercent } from '@/utils/format'
import type { UserUsageTrendPoint } from '@/types'

const props = withDefaults(defineProps<{
  /** 用户趋势点（后端按区间 Token 取前 N 名，按时间桶 × 用户分组） */
  points: UserUsageTrendPoint[]
  /** 时间桶（与页头范围、粒度一致） */
  days: string[]
  /** 区间内全站 Token 总量，算占比用（不是前 N 名之和） */
  totalTokens: number
  loading?: boolean
  limit?: number
}>(), {
  loading: false,
  limit: 10
})

const emit = defineEmits<{ select: [userId: number] }>()
const { t } = useI18n()

interface UserRow {
  userId: number
  name: string
  email: string
  requests: number
  tokens: number
  revenue: number
  cost: number
  buckets: number[]
  /** 热力条每格的不透明度（0 = 这个桶没用），按本行峰值归一 */
  cells: number[]
}

const num = (value: unknown): number => {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

const rows = computed<UserRow[]>(() => {
  const bucketIndex = new Map(props.days.map((day, i) => [day, i]))
  const byUser = new Map<number, UserRow>()
  for (const p of props.points ?? []) {
    let row = byUser.get(p.user_id)
    if (!row) {
      row = {
        userId: p.user_id,
        name: p.username?.trim() || p.email?.trim() || t('common.deletedUser'),
        email: p.email?.trim() || '',
        requests: 0,
        tokens: 0,
        revenue: 0,
        cost: 0,
        buckets: new Array<number>(props.days.length).fill(0),
        cells: []
      }
      byUser.set(p.user_id, row)
    }
    row.requests += num(p.requests)
    row.tokens += num(p.tokens)
    row.revenue += num(p.actual_cost)
    row.cost += num(p.account_cost)
    const col = bucketIndex.get(p.date)
    if (col !== undefined) row.buckets[col] += num(p.tokens)
  }
  const list = [...byUser.values()].sort((a, b) => b.tokens - a.tokens || a.userId - b.userId)
  for (const row of list) {
    const peak = Math.max(0, ...row.buckets)
    // 开方拉开小值，最淡也留 0.22：空格是浅灰实心块，太淡会和「没用」分不清
    row.cells = row.buckets.map((v) => (v > 0 && peak > 0 ? Math.max(0.22, Math.sqrt(v / peak)) : 0))
  }
  return list
})

const showAll = ref(false)
const visibleRows = computed(() => (showAll.value ? rows.value : rows.value.slice(0, props.limit)))

const shortLabel = (date: string): string => (/^\d{4}-\d{2}-\d{2}(?: \d{2}:00)?$/.test(date) ? date.slice(5) : date)
const firstLabel = computed(() => (props.days.length ? shortLabel(props.days[0]) : ''))
const lastLabel = computed(() => (props.days.length ? shortLabel(props.days[props.days.length - 1]) : ''))

const formatShare = (tokens: number): string => {
  if (props.totalTokens <= 0) return ''
  return formatSharePercent((tokens / props.totalTokens) * 100)
}
const formatTokens = (value: number): string => {
  const v = num(value)
  if (v >= 1_000_000_000) return `${(v / 1_000_000_000).toFixed(2)}B`
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}M`
  if (v >= 1_000) return `${(v / 1_000).toFixed(2)}K`
  return v.toLocaleString()
}
</script>
