<template>
  <!--
    渠道状态（A5）：灰点 + 文字是正常；只有异常才上色，并把原因直接写在下面一行（不再是彩色徽章 + 悬停问号）。
    限流 / 过载 / 临时不可调度下面一行写什么时候恢复；按模型的限流另起几行小字。
  -->
  <div class="flex min-w-0 flex-col gap-0.5">
    <div class="flex items-center gap-1.5">
      <span :class="['inline-block h-2 w-2 shrink-0 rounded-full', dotClass]" aria-hidden="true"></span>
      <button
        v-if="isTempUnschedulable && !isRateLimited && !isOverloaded"
        type="button"
        :class="['text-left underline decoration-dotted underline-offset-2', labelClass]"
        :title="t('admin.accounts.status.viewTempUnschedDetails')"
        data-testid="account-status-label"
        @click.stop="handleTempUnschedClick"
      >
        {{ label }}
      </button>
      <span v-else :class="labelClass" :title="labelTitle" data-testid="account-status-label">{{ label }}</span>
    </div>
    <span
      v-if="detail"
      :class="['line-clamp-2 max-w-[7.5rem] whitespace-normal break-words pl-3.5 text-xs leading-4', detailClass]"
      :title="detailTitle"
      data-testid="account-status-detail"
    >
      {{ detail }}
    </span>

    <!-- 按模型的限流 / 积分状态 -->
    <div v-if="activeModelStatuses.length > 0" class="flex flex-wrap gap-x-2 gap-y-0.5 pl-3.5">
      <span
        v-for="item in activeModelStatuses"
        :key="`${item.kind}-${item.model}`"
        :class="[
          'inline-flex items-center gap-1 text-xs',
          item.kind === 'credits_exhausted' ? 'text-af-danger' : item.kind === 'credits_active' ? 'text-af-warning' : 'text-af-ink-3'
        ]"
        :title="modelStatusTitle(item)"
      >
        <span v-if="item.kind === 'credits_active'">⚡</span>
        <template v-if="item.kind === 'credits_exhausted'">{{ t('admin.accounts.status.creditsExhausted') }}</template>
        <template v-else>{{ formatScopeName(item.model) }}</template>
        <span class="tabular-nums text-af-ink-4">{{ formatCountdown(item.reset_at) }}</span>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import { formatCountdown, formatDateTime, formatDateTimeToMinute, formatCountdownWithSuffix, formatTime } from '@/utils/format'

const { t } = useI18n()

const props = defineProps<{
  account: Account
}>()

const emit = defineEmits<{
  (e: 'show-temp-unsched', account: Account): void
}>()

// Computed: is rate limited (429)
const isRateLimited = computed(() => {
  if (!props.account.rate_limit_reset_at) return false
  return new Date(props.account.rate_limit_reset_at) > new Date()
})

type AccountModelStatusItem = {
  kind: 'rate_limit' | 'credits_exhausted' | 'credits_active'
  model: string
  reset_at: string
}

// Computed: active model statuses (普通模型限流 + 积分耗尽 + 走积分中)
const activeModelStatuses = computed<AccountModelStatusItem[]>(() => {
  const extra = props.account.extra as Record<string, unknown> | undefined
  const modelLimits = extra?.model_rate_limits as
    | Record<string, { rate_limited_at: string; rate_limit_reset_at: string }>
    | undefined
  const now = new Date()
  const items: AccountModelStatusItem[] = []

  if (!modelLimits) return items

  // 检查 AICredits key 是否生效（积分是否耗尽）
  const aiCreditsEntry = modelLimits['AICredits']
  const hasActiveAICredits = aiCreditsEntry && new Date(aiCreditsEntry.rate_limit_reset_at) > now
  const allowOverages = !!(extra?.allow_overages)

  for (const [model, info] of Object.entries(modelLimits)) {
    if (new Date(info.rate_limit_reset_at) <= now) continue

    if (model === 'AICredits') {
      // AICredits key → 积分已用尽
      items.push({ kind: 'credits_exhausted', model, reset_at: info.rate_limit_reset_at })
    } else if (allowOverages && !hasActiveAICredits) {
      // 普通模型限流 + overages 启用 + 积分可用 → 正在走积分
      items.push({ kind: 'credits_active', model, reset_at: info.rate_limit_reset_at })
    } else {
      // 普通模型限流
      items.push({ kind: 'rate_limit', model, reset_at: info.rate_limit_reset_at })
    }
  }

  return items
})

const formatScopeName = (scope: string): string => {
  const aliases: Record<string, string> = {
    // Claude 系列
    'claude-fable-5-1': 'CFable51',
    'claude-fable-5': 'CFable5',
    'claude-opus-4-6': 'COpus46',
    'claude-opus-4-6-thinking': 'COpus46T',
    'claude-opus-4-7': 'COpus47',
    'claude-opus-4-8': 'COpus48',
    'claude-opus-5': 'COpus5',
    'claude-sonnet-4-6': 'CSon46',
    'claude-sonnet-4-5': 'CSon45',
    'claude-sonnet-4-5-thinking': 'CSon45T',
    'claude-sonnet-5': 'CSon5',
    // Gemini 2.5 系列
    'gemini-2.5-flash': 'G25F',
    'gemini-2.5-flash-lite': 'G25FL',
    'gemini-2.5-flash-thinking': 'G25FT',
    'gemini-2.5-pro': 'G25P',
    'gemini-2.5-flash-image': 'G25I',
    // Gemini 3.5 系列
    'gemini-3.5-flash': 'G35F',
    // Gemini 3 系列
    'gemini-3-flash': 'G3F',
    'gemini-3.1-pro-high': 'G3PH',
    'gemini-3.1-pro-low': 'G3PL',
    'gemini-3-pro-image': 'G3PI',
    'gemini-3.1-flash-image': 'G31FI',
    // 其他
    'gpt-oss-120b-medium': 'GPT120',
    'tab_flash_lite_preview': 'TabFL',
    // 旧版 scope 别名（兼容）
    claude: 'Claude',
    claude_sonnet: 'CSon',
    claude_opus: 'COpus',
    claude_haiku: 'CHaiku',
    gemini_text: 'Gemini',
    gemini_image: 'GImg',
    gemini_flash: 'GFlash',
    gemini_pro: 'GPro',
  }
  return aliases[scope] || scope
}

// Computed: is overloaded (529)
const isOverloaded = computed(() => {
  if (!props.account.overload_until) return false
  return new Date(props.account.overload_until) > new Date()
})

// Computed: is temp unschedulable
const isTempUnschedulable = computed(() => {
  if (!props.account.temp_unschedulable_until) return false
  return new Date(props.account.temp_unschedulable_until) > new Date()
})

// Computed: has error status
const hasError = computed(() => {
  return props.account.status === 'error'
})

const isQuotaExceeded = computed(() => {
  const exceeded = (used?: number | null, limit?: number | null) =>
    typeof limit === 'number' && limit > 0 && typeof used === 'number' && used >= limit
  return (
    exceeded(props.account.quota_used, props.account.quota_limit) ||
    exceeded(props.account.quota_daily_used, props.account.quota_daily_limit) ||
    exceeded(props.account.quota_weekly_used, props.account.quota_weekly_limit)
  )
})

// Computed: countdown text for rate limit (429)
const rateLimitCountdown = computed(() => {
  return formatCountdown(props.account.rate_limit_reset_at)
})

const rateLimitResumeText = computed(() => {
  if (!rateLimitCountdown.value) return ''
  return t('admin.accounts.status.rateLimitedAutoResume', { time: rateLimitCountdown.value })
})

// Computed: countdown text for overload (529)
const overloadCountdown = computed(() => {
  return formatCountdownWithSuffix(props.account.overload_until)
})

const tempUnschedRecoveryText = computed(() => {
  if (!isTempUnschedulable.value || !props.account.temp_unschedulable_until) return ''
  return t('admin.accounts.status.tempUnschedulableUntil', {
    time: formatDateTime(props.account.temp_unschedulable_until)
  })
})

// Computed: status text
const statusText = computed(() => {
  if (hasError.value) {
    return t('admin.accounts.status.error')
  }
  if (isTempUnschedulable.value) {
    return t('admin.accounts.status.tempUnschedulable')
  }
  if (props.account.status !== 'active') {
    return t(`admin.accounts.status.${props.account.status}`)
  }
  if (isQuotaExceeded.value) {
    return t('admin.accounts.status.quotaExceeded')
  }
  if (!props.account.schedulable) {
    return t('admin.accounts.status.paused')
  }
  return t(`admin.accounts.status.${props.account.status}`)
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

const label = computed(() => {
  if (isRateLimited.value) return t('admin.accounts.status.rateLimited')
  if (isOverloaded.value) return t('admin.accounts.status.overloaded')
  return statusText.value
})

const labelTitle = computed(() => {
  if (isRateLimited.value && props.account.rate_limit_reset_at) {
    return t('admin.accounts.status.rateLimitedUntil', { time: formatDateTime(props.account.rate_limit_reset_at) })
  }
  if (isOverloaded.value && props.account.overload_until) {
    return t('admin.accounts.status.overloadedUntil', { time: formatTime(props.account.overload_until) })
  }
  return undefined
})

// 第二行：限流 / 过载 / 临时不可调度写恢复时间；异常直接写上游返回的原因
const detail = computed(() => {
  if (isRateLimited.value) return rateLimitResumeText.value
  if (isOverloaded.value) return overloadCountdown.value
  if (isTempUnschedulable.value) return tempUnschedRecoveryText.value
  if (hasError.value) return props.account.error_message || ''
  return ''
})

const detailClass = computed(() => (hasError.value && !isRateLimited.value && !isOverloaded.value && !isTempUnschedulable.value ? 'text-af-danger' : 'text-af-ink-3'))
const detailTitle = computed(() => (hasError.value ? props.account.error_message || undefined : undefined))

const modelStatusTitle = (item: AccountModelStatusItem): string => {
  if (item.kind === 'credits_exhausted') {
    return t('admin.accounts.status.creditsExhaustedUntil', { time: formatDateTimeToMinute(item.reset_at) })
  }
  if (item.kind === 'credits_active') {
    return t('admin.accounts.status.modelCreditOveragesUntil', { model: formatScopeName(item.model), time: formatDateTimeToMinute(item.reset_at) })
  }
  return t('admin.accounts.status.modelRateLimitedUntil', { model: formatScopeName(item.model), time: formatDateTimeToMinute(item.reset_at) })
}

const handleTempUnschedClick = () => {
  if (!isTempUnschedulable.value) return
  emit('show-temp-unsched', props.account)
}
</script>
