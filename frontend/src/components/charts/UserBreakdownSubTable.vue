<template>
  <!--
    模型分布表（概览）一行展开后的「按用户下钻」：直接是父表 tbody 里的几行，和父表同一套 7 列对齐
    （名称 · 占比 · 请求 · Token · 收入 · 成本 · 利润），用户邮箱占前两列。原来是嵌在单元格里的无表头小表，列对不上。
  -->
  <tr v-if="loading" class="bg-af-sunken/50">
    <td :colspan="COLUMN_COUNT" class="py-3">
      <div class="flex items-center justify-center"><LoadingSpinner /></div>
    </td>
  </tr>
  <tr v-else-if="items.length === 0" class="bg-af-sunken/50">
    <td :colspan="COLUMN_COUNT" class="py-2 text-center text-af-ink-3">{{ t('admin.dashboard.noDataAvailable') }}</td>
  </tr>
  <template v-else>
    <tr
      v-for="user in items"
      :key="user.user_id"
      class="border-t border-af-hairline/50 bg-af-sunken/50"
      data-testid="user-breakdown-row"
    >
      <td colspan="2" class="max-w-[180px] truncate py-1 pl-6 text-af-ink-2" :title="user.email">
        {{ user.email || `#${user.user_id}` }}
      </td>
      <td class="py-1 text-right tabular-nums text-af-ink-3">{{ user.requests.toLocaleString() }}</td>
      <td class="py-1 text-right tabular-nums text-af-ink-3">{{ formatTokens(user.total_tokens) }}</td>
      <td class="py-1 text-right tabular-nums text-af-ink">{{ formatMoney(user.actual_cost) }}</td>
      <td class="py-1 text-right tabular-nums text-af-ink-3">{{ formatMoney(user.account_cost) }}</td>
      <td
        class="py-1 text-right tabular-nums"
        :class="profitTextClass(profitOf(user.actual_cost, user.account_cost)) || 'text-af-ink-3'"
      >
        {{ formatMoney(profitOf(user.actual_cost, user.account_cost)) }}
      </td>
    </tr>
  </template>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatMoney, profitOf, profitTextClass } from '@/utils/money'
import type { UserBreakdownItem } from '@/types'

withDefaults(defineProps<{
  items: UserBreakdownItem[]
  loading?: boolean
}>(), {
  loading: false,
})

const { t } = useI18n()

/** 父表的列数：名称、占比、请求、Token、收入、成本、利润 */
const COLUMN_COUNT = 7

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}
</script>
