<template>
  <!--
    渠道状态只写一行：灰点 + 「正常」；出问题才上色，并在同一行写原因（异常原文 / 多久后恢复）。
    按模型的限流在一行里只写「N 个模型限流中」，悬停看完整模型名；detailed（抽屉概况）时逐个列出。
  -->
  <div class="flex min-w-0 flex-col gap-1">
    <div class="flex min-w-0 items-center gap-1.5" :title="lineTitle">
      <span :class="['inline-block h-2 w-2 shrink-0 rounded-full', dotClass]" aria-hidden="true"></span>
      <button
        v-if="isTempUnschedulable && !isRateLimited && !isOverloaded"
        type="button"
        :class="['shrink-0 text-left underline decoration-dotted underline-offset-2', labelClass]"
        :title="t('admin.accounts.status.viewTempUnschedDetails')"
        data-testid="account-status-label"
        @click.stop="handleTempUnschedClick"
      >
        {{ label }}
      </button>
      <span v-else :class="['shrink-0', labelClass]" data-testid="account-status-label">{{ label }}</span>
      <span
        v-if="reason"
        :class="['min-w-0 text-xs', detailed ? 'break-words' : 'max-w-[14rem] truncate', reasonClass]"
        data-testid="account-status-detail"
      >
        {{ reason }}
      </span>
    </div>

    <!-- 抽屉里逐个列出按模型的限流 / 积分状态，模型名写全 -->
    <ul v-if="detailed && activeModelStatuses.length > 0" class="space-y-0.5 pl-3.5" data-testid="account-status-models">
      <li
        v-for="item in activeModelStatuses"
        :key="`${item.kind}-${item.model}`"
        :class="['text-xs', item.kind === 'credits_exhausted' ? 'text-af-danger' : item.kind === 'credits_active' ? 'text-af-warning' : 'text-af-ink-3']"
      >
        <span v-if="item.kind !== 'credits_exhausted'" class="font-mono">{{ item.model }}</span>
        {{ modelStatusText(item) }}
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import { formatDateTime, formatDateTimeToMinute } from '@/utils/format'
import { durationUntilWords } from './durationWords'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    account: Account
    /** 抽屉概况：原因不截断，按模型的限流逐个列出 */
    detailed?: boolean
  }>(),
  { detailed: false }
)

const emit = defineEmits<{
  (e: 'show-temp-unsched', account: Account): void
}>()

const isFuture = (value?: string | null) => !!value && new Date(value) > new Date()

const isRateLimited = computed(() => isFuture(props.account.rate_limit_reset_at))
const isOverloaded = computed(() => isFuture(props.account.overload_until))
const isTempUnschedulable = computed(() => isFuture(props.account.temp_unschedulable_until))
const hasError = computed(() => props.account.status === 'error')

const isQuotaExceeded = computed(() => {
  const exceeded = (used?: number | null, limit?: number | null) =>
    typeof limit === 'number' && limit > 0 && typeof used === 'number' && used >= limit
  return (
    exceeded(props.account.quota_used, props.account.quota_limit) ||
    exceeded(props.account.quota_daily_used, props.account.quota_daily_limit) ||
    exceeded(props.account.quota_weekly_used, props.account.quota_weekly_limit)
  )
})

type AccountModelStatusItem = {
  kind: 'rate_limit' | 'credits_exhausted' | 'credits_active'
  model: string
  reset_at: string
}

// 按模型的状态：普通模型限流 / AI Credits 用尽 / 正在用 AI Credits 顶
const activeModelStatuses = computed<AccountModelStatusItem[]>(() => {
  const extra = props.account.extra as Record<string, unknown> | undefined
  const modelLimits = extra?.model_rate_limits as
    | Record<string, { rate_limited_at: string; rate_limit_reset_at: string }>
    | undefined
  if (!modelLimits) return []

  const now = new Date()
  const aiCreditsEntry = modelLimits['AICredits']
  const hasActiveAICredits = aiCreditsEntry && new Date(aiCreditsEntry.rate_limit_reset_at) > now
  const allowOverages = !!extra?.allow_overages

  const items: AccountModelStatusItem[] = []
  for (const [model, info] of Object.entries(modelLimits)) {
    if (new Date(info.rate_limit_reset_at) <= now) continue
    if (model === 'AICredits') {
      items.push({ kind: 'credits_exhausted', model, reset_at: info.rate_limit_reset_at })
    } else if (allowOverages && !hasActiveAICredits) {
      items.push({ kind: 'credits_active', model, reset_at: info.rate_limit_reset_at })
    } else {
      items.push({ kind: 'rate_limit', model, reset_at: info.rate_limit_reset_at })
    }
  }
  return items
})

// 状态词：与筛选下拉、数字摘要同一套叫法
const label = computed(() => {
  if (isRateLimited.value) return t('admin.accounts.status.rateLimited')
  if (isOverloaded.value) return t('admin.accounts.status.overloaded')
  if (hasError.value) return t('admin.accounts.status.error')
  if (isTempUnschedulable.value) return t('admin.accounts.status.tempUnschedulable')
  if (props.account.status !== 'active') return t(`admin.accounts.status.${props.account.status}`)
  if (isQuotaExceeded.value) return t('admin.accounts.status.quotaExceeded')
  if (!props.account.schedulable) return t('admin.accounts.status.unschedulable')
  return t('admin.accounts.status.active')
})

type StatusTone = 'ok' | 'muted' | 'warning' | 'danger'

const tone = computed<StatusTone>(() => {
  if (isRateLimited.value) return 'warning'
  if (isOverloaded.value) return 'danger'
  if (hasError.value) return 'danger'
  if (isTempUnschedulable.value) return 'warning'
  if (props.account.status !== 'active') return 'muted'
  if (isQuotaExceeded.value) return 'warning'
  if (!props.account.schedulable) return 'muted'
  return 'ok'
})

const dotClass = computed(() => {
  switch (tone.value) {
    case 'danger':
      return 'bg-af-danger'
    case 'warning':
      return 'bg-af-warning'
    case 'muted':
      return 'border border-af-ink-4'
    default:
      return 'bg-af-ink-4'
  }
})

const labelClass = computed(() => {
  switch (tone.value) {
    case 'danger':
      return 'text-af-danger'
    case 'warning':
      return 'text-af-warning'
    case 'muted':
      return 'text-af-ink-3'
    default:
      return 'text-af-ink-2'
  }
})

const recoverIn = (value?: string | null) => durationUntilWords(value, t)

// 同一行的原因：限流 / 过载写多久后恢复；临时停调写原因或恢复时间；异常写上游原文；
// 本身正常但有模型限流时写「N 个模型限流中」
const reason = computed(() => {
  if (isRateLimited.value) {
    const time = recoverIn(props.account.rate_limit_reset_at)
    return time ? t('admin.accounts.status.recoverIn', { time }) : ''
  }
  if (isOverloaded.value) {
    const time = recoverIn(props.account.overload_until)
    return time ? t('admin.accounts.status.recoverIn', { time }) : ''
  }
  if (hasError.value) return props.account.error_message || ''
  if (isTempUnschedulable.value) {
    return props.account.temp_unschedulable_reason
      || t('admin.accounts.status.tempUnschedulableUntil', { time: formatDateTime(props.account.temp_unschedulable_until) })
  }
  if (!props.detailed && tone.value === 'ok' && activeModelStatuses.value.length > 0) {
    return t('admin.accounts.status.modelsLimited', { count: activeModelStatuses.value.length })
  }
  return ''
})

const reasonClass = computed(() => (hasError.value && tone.value === 'danger' && !isOverloaded.value ? 'text-af-danger' : 'text-af-ink-3'))

// 模型名（完整 ID）单独写在前面，这里只是后半句
const modelStatusText = (item: AccountModelStatusItem): string => {
  const time = formatDateTimeToMinute(item.reset_at)
  if (item.kind === 'credits_exhausted') return t('admin.accounts.status.creditsExhaustedUntil', { time })
  if (item.kind === 'credits_active') return t('admin.accounts.status.modelCreditOveragesUntil', { time })
  return t('admin.accounts.status.modelRateLimitedUntil', { time })
}
const modelStatusLine = (item: AccountModelStatusItem): string =>
  item.kind === 'credits_exhausted' ? modelStatusText(item) : `${item.model} ${modelStatusText(item)}`

// 一行放不下的内容放进悬停：原因全文 + 按模型的限流明细
const lineTitle = computed(() => {
  if (props.detailed) return undefined
  const lines: string[] = []
  if (reason.value) lines.push(`${label.value}${t('common.labelSeparator')}${reason.value}`)
  if (isRateLimited.value && props.account.rate_limit_reset_at) {
    lines.push(t('admin.accounts.status.rateLimitedUntil', { time: formatDateTime(props.account.rate_limit_reset_at) }))
  }
  for (const item of activeModelStatuses.value) lines.push(modelStatusLine(item))
  return lines.length ? lines.join('\n') : undefined
})

const handleTempUnschedClick = () => {
  if (!isTempUnschedulable.value) return
  emit('show-temp-unsched', props.account)
}
</script>
