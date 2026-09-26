<template>
  <!--
    一个上游用量窗口（渠道详情抽屉「用量」页签）：窗口名写全（5 小时 / 7 天 / Gemini 3 Pro…），
    进度条 + 百分比 + 多久后重置；本站在这个窗口里的用量另起一行小字：请求 · Token · 收入 · 成本。
    进度条平时是墨色，≥75% 黄、≥90% 红（颜色只表示状态）。
  -->
  <div>
    <div class="flex items-center gap-2">
      <span class="w-28 shrink-0 truncate text-xs text-af-ink-2" :title="label" data-testid="usage-window-label">{{ label }}</span>
      <div class="h-1.5 w-24 shrink-0 overflow-hidden rounded-full bg-af-hairline">
        <div :class="['h-full transition-all duration-300', barClass]" :style="{ width: barWidth }"></div>
      </div>
      <span :class="['w-10 shrink-0 text-right text-xs font-medium tabular-nums', textClass]">
        {{ displayPercent }}
      </span>
      <span v-if="resetText" class="min-w-0 truncate text-xs text-af-ink-3" data-testid="usage-window-reset">
        {{ resetText }}
      </span>
    </div>

    <p
      v-if="windowStats && (windowStats.requests > 0 || windowStats.tokens > 0)"
      class="mt-0.5 pl-[7.5rem] text-xs leading-4 tabular-nums text-af-ink-3"
      data-testid="usage-window-stats"
    >
      {{ t('admin.accounts.usageWindow.requests', { count: formatRequests }) }}
      · {{ formatTokens }} Token
      <template v-if="windowStats.user_cost != null">
        · {{ t('common.money.revenue') }} {{ formatMoney(windowStats.user_cost) }}
      </template>
      · {{ t('common.money.cost') }} {{ formatMoney(windowStats.cost) }}
      <span
        v-if="estimatedTotalCost != null"
        data-test="estimated-total-cost"
        :title="t('admin.accounts.usageWindow.estimatedTotalCostTooltip')"
      >
        · {{ t('admin.accounts.usageWindow.estimatedTotalCost', { cost: formatMoney(estimatedTotalCost) }) }}
      </span>
    </p>
    <p v-else-if="note" class="mt-0.5 pl-[7.5rem] text-xs leading-4 tabular-nums text-af-ink-3" data-testid="usage-window-note">
      {{ note }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import type { WindowStats } from '@/types'
import { formatCompactNumber } from '@/utils/format'
import { formatMoney } from '@/utils/money'
import { durationUntilWords } from './durationWords'

const props = defineProps<{
  label: string
  utilization: number // Percentage (0-100+)
  resetsAt?: string | null
  /** 本站在这个窗口里的用量；cost 是渠道成本，user_cost 是收入 */
  windowStats?: WindowStats | null
  /** 按当前成本和用量推算的「用满这个窗口的成本」 */
  estimatedTotalCost?: number | null
  showNowWhenIdle?: boolean
  /** 没有窗口用量时，第二行写的一句说明（如配额「已用 $x / 限额 $y」） */
  note?: string
}>()

const { t } = useI18n()

// 倒计时只在有重置时间时走表，避免抽屉里多个窗口各起一个空转的定时器
const now = ref(new Date())
const { pause: pauseClock, resume: resumeClock } = useIntervalFn(
  () => {
    now.value = new Date()
  },
  60_000,
  { immediate: false },
)
if (props.resetsAt) resumeClock()
watch(
  () => props.resetsAt,
  (val) => {
    if (val) {
      now.value = new Date()
      resumeClock()
    } else {
      pauseClock()
    }
  },
)

const barClass = computed(() => {
  if (props.utilization >= 90) return 'bg-af-danger'
  if (props.utilization >= 75) return 'bg-af-warning'
  return 'bg-af-ink-3'
})

const textClass = computed(() => {
  if (props.utilization >= 90) return 'text-af-danger'
  if (props.utilization >= 75) return 'text-af-warning'
  return 'text-af-ink-2'
})

const barWidth = computed(() => `${Math.min(Math.max(props.utilization, 0), 100)}%`)

// 超过 999% 就不再写具体数
const displayPercent = computed(() => {
  const percent = Math.round(props.utilization)
  return percent > 999 ? '>999%' : `${percent}%`
})

const resetText = computed(() => {
  // 滚动窗口用量为 0：现在就能用
  if (props.showNowWhenIdle && props.utilization <= 0) return t('admin.accounts.usageWindow.resetNow')
  if (!props.resetsAt) return ''
  const remaining = durationUntilWords(props.resetsAt, t, now.value)
  if (remaining) return t('admin.accounts.usageWindow.resetsIn', { time: remaining })
  // 重置时间已过但用量还 > 0：后端窗口数据还没刷新
  return props.utilization > 0 ? t('admin.accounts.usageWindow.resetPending') : t('admin.accounts.usageWindow.resetNow')
})

const formatRequests = computed(() => formatCompactNumber(props.windowStats?.requests ?? 0, { allowBillions: false }))
const formatTokens = computed(() => formatCompactNumber(props.windowStats?.tokens ?? 0))
</script>
