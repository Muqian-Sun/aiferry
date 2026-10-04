<template>
  <!--
    管理站用户列表「近 30 天消费」一格：收入口径（actual_cost），两位小数。
    花在两个及以上平台时带 ⓘ，悬停看按平台拆分（平台不写死，后端给几个列几个）。
  -->
  <div class="group/usage relative inline-flex items-center gap-1.5 text-sm">
    <span class="font-medium tabular-nums text-af-ink">{{ formatMoney(total) }}</span>
    <Icon v-if="showBreakdown" name="infoCircle" size="xs" class="text-af-ink-4" />

    <!-- 平时 display:none 而不是透明：透明的浮层照样占位，手机宽会把整页撑出横向滚动。
         手机宽（列表是卡片、数字靠右）浮层出在下方、右对齐；md 起（表格）出在右侧 -->
    <div
      v-if="showBreakdown"
      class="pointer-events-none absolute right-0 top-full z-50 mt-1 hidden min-w-[180px] whitespace-nowrap rounded-md bg-af-ink px-3 py-2 text-xs text-af-on-brand shadow-xl group-hover/usage:block md:left-full md:right-auto md:top-0 md:ml-2 md:mt-0"
    >
      <div class="mb-1.5 border-b border-af-sheet/10 pb-1 text-[11px] opacity-80">
        {{ t('admin.users.platformBreakdown') }}
      </div>
      <div
        v-for="item in breakdown"
        :key="item.platform"
        class="flex items-center justify-between gap-3 py-0.5"
        :class="{ 'opacity-70 italic': item.isOther }"
      >
        <span>{{ item.isOther ? t('admin.users.platformOther') : platformLabel(item.platform) }}</span>
        <span class="tabular-nums">{{ formatMoney(item.cost) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { PlatformUsage } from '@/api/admin/dashboard'
import { formatMoney } from '@/utils/money'
import { platformLabel } from '@/utils/platformLabel'

const props = defineProps<{
  /** 近 30 天收入（actual_cost） */
  total: number
  byPlatform?: PlatformUsage[]
}>()

const { t } = useI18n()

// 「总值 − 各平台之和」的差（没记上平台的请求）单列一行「其他」，免得悬停里的加总和格子里的数对不上
const OTHER_THRESHOLD = 0.0001

interface BreakdownRow {
  platform: string
  cost: number
  isOther?: boolean
}

const breakdown = computed<BreakdownRow[]>(() => {
  const rows: BreakdownRow[] = (props.byPlatform ?? [])
    .map((p) => ({ platform: p.platform, cost: p.total_actual_cost }))
    .filter((row) => row.cost > 0)
    .sort((a, b) => b.cost - a.cost)
  const other = props.total - rows.reduce((sum, row) => sum + row.cost, 0)
  if (other > OTHER_THRESHOLD) rows.push({ platform: '__other__', cost: other, isOther: true })
  return rows
})

// 只花在一个平台上时，拆分和格子里的数一样，不必悬停
const showBreakdown = computed(() => breakdown.value.length > 1)
</script>
