<template>
  <!--
    用量页数字摘要：请求 · Token · 收入 · 成本 · 利润；缓存命中率、平均响应（从概览挪来）作次要数字排在后面，小一号、墨色浅一档。
    版式照 StatRow（行内大数字、竖 hairline 分隔，lg 以下两列网格）。利润为负要标红，StatRow 没有这个口子，所以这里自己画。
  -->
  <dl class="grid grid-cols-2 gap-y-4 lg:flex lg:items-baseline lg:divide-x lg:divide-af-hairline" data-testid="usage-summary">
    <div
      v-for="(item, index) in items"
      :key="item.key"
      class="min-w-0 pr-6 lg:px-6 lg:first:pl-0 lg:last:pr-0"
      :class="[
        item.secondary ? 'lg:flex-none' : 'lg:flex-1',
        index >= 2 ? 'border-t border-af-hairline pt-4 lg:border-t-0 lg:pt-0' : ''
      ]"
      :data-testid="`stat-${item.key}`"
    >
      <dt class="truncate text-13 text-af-ink-3" :title="item.title">{{ item.label }}</dt>
      <dd
        class="mt-1.5 font-semibold tabular-nums"
        :class="[item.secondary ? 'text-lg' : 'text-2xl tracking-[-0.01em]', item.valueClass]"
        :title="item.title"
      >
        {{ item.value }}
      </dd>
    </div>
  </dl>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatMoney, profitOf, profitTextClass } from '@/utils/money'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'

const props = defineProps<{ stats: AdminUsageStatsResponse }>()

const { t } = useI18n()

interface SummaryItem {
  key: string
  label: string
  value: string
  title?: string
  valueClass: string
  secondary?: boolean
}

const num = (value: unknown): number => {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}

const formatDuration = (ms: number): string => (ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`)

const items = computed<SummaryItem[]>(() => {
  const s = props.stats
  const input = num(s.total_input_tokens)
  const output = num(s.total_output_tokens)
  const cacheRead = num(s.total_cache_read_tokens)
  const cacheCreation = num(s.total_cache_creation_tokens)
  const revenue = num(s.total_actual_cost)
  const cost = num(s.total_account_cost)
  const profit = profitOf(revenue, cost)
  // 与概览同一口径：缓存命中率 = 缓存读 ÷（输入 + 缓存读 + 缓存写）；没有输入侧 token 时显示「—」
  const promptTokens = input + cacheRead + cacheCreation
  const hitShare = promptTokens > 0 ? cacheRead / promptTokens : null
  const tokenBreakdown = [
    `${t('admin.usage.inputTokens')} ${input.toLocaleString()}`,
    `${t('admin.usage.outputTokens')} ${output.toLocaleString()}`,
    `${t('admin.usage.cacheReadTokens')} ${cacheRead.toLocaleString()}`,
    `${t('admin.usage.cacheCreationTokens')} ${cacheCreation.toLocaleString()}`
  ].join('\n')
  return [
    { key: 'requests', label: t('admin.usage.summary.requests'), value: num(s.total_requests).toLocaleString(), valueClass: 'text-af-ink' },
    { key: 'tokens', label: t('admin.usage.summary.tokens'), value: formatTokens(num(s.total_tokens)), title: tokenBreakdown, valueClass: 'text-af-ink' },
    { key: 'revenue', label: t('common.money.revenue'), value: formatMoney(revenue), title: t('common.money.revenueHint'), valueClass: 'text-af-ink' },
    { key: 'cost', label: t('common.money.cost'), value: formatMoney(cost), title: t('common.money.costHint'), valueClass: 'text-af-ink' },
    {
      key: 'profit',
      label: t('common.money.profit'),
      value: formatMoney(profit),
      title: t('common.money.profitHint'),
      valueClass: profitTextClass(profit) || 'text-af-ink'
    },
    {
      key: 'cacheHitRate',
      label: t('admin.usage.summary.cacheHitRate'),
      value: hitShare === null ? '—' : `${(hitShare * 100).toFixed(hitShare > 0 && hitShare < 0.1 ? 1 : 0)}%`,
      valueClass: 'text-af-ink-2',
      secondary: true
    },
    {
      key: 'avgResponse',
      label: t('admin.usage.summary.avgResponse'),
      value: formatDuration(num(s.average_duration_ms)),
      valueClass: 'text-af-ink-2',
      secondary: true
    }
  ]
})
</script>
