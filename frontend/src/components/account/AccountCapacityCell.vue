<template>
  <!--
    渠道容量（详情抽屉「用量」页签；列表放不下，方案 2026-09-25 挪进抽屉）：并发，以及 Anthropic 成品号的
    5 小时窗口费用、活跃会话、RPM。每行「当前 / 上限」，下面一行小字说明当前状态；只有接近或到达上限才上色。
    第三方 key 的日 / 周 / 总额度在上面「上游用量窗口」里画进度条，这里不重复。
  -->
  <dl class="divide-y divide-af-hairline" data-testid="account-capacity">
    <DetailField v-for="row in rows" :key="row.key" :label="row.label">
      <div :data-testid="`capacity-${row.key}`">
        <span :class="['tabular-nums', row.toneClass]">{{ row.current }} / {{ row.max }}</span>
        <p v-if="row.hint" class="text-xs text-af-ink-3">{{ row.hint }}</p>
      </div>
    </DetailField>
  </dl>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import DetailField from '@/components/common/DetailField.vue'
import { formatMoney } from '@/utils/money'

const props = defineProps<{ account: Account }>()

const { t } = useI18n()

interface CapacityRow {
  key: string
  label: string
  current: string | number
  max: string | number
  toneClass: string
  hint?: string
}

// 接近上限（≥80%）黄，到达上限红，其余不上色
const toneOf = (current: number, max: number, full = max) => {
  if (current >= full) return 'text-af-danger'
  if (current >= max * 0.8) return 'text-af-warning'
  return ''
}

// 运行时的窗口费用 / 会话 / RPM 只有 Anthropic OAuth / Setup Token 才会由后端算出来
const isAnthropicOAuthOrSetupToken = computed(() =>
  props.account.platform === 'anthropic' &&
  (props.account.type === 'oauth' || props.account.type === 'setup-token')
)

const concurrencyRow = computed<CapacityRow>(() => {
  const current = props.account.current_concurrency || 0
  const max = props.account.concurrency
  return {
    key: 'concurrency',
    label: t('admin.accounts.capacity.concurrency'),
    current,
    max,
    // 并发只在占满时标红；有请求在跑是正常状态
    toneClass: current >= max ? 'text-af-danger' : ''
  }
})

// 5 小时窗口费用按标价累计（不乘任何倍率），到上限后渠道停到窗口结束
const windowCostRow = computed<CapacityRow | null>(() => {
  const limit = props.account.window_cost_limit ?? 0
  if (!isAnthropicOAuthOrSetupToken.value || limit <= 0) return null
  const current = props.account.current_window_cost ?? 0
  return {
    key: 'window-cost',
    label: t('admin.accounts.capacity.windowCost.label'),
    current: formatMoney(current),
    max: formatMoney(limit),
    toneClass: toneOf(current, limit),
    hint: current >= limit ? t('admin.accounts.capacity.windowCost.blocked') : t('admin.accounts.capacity.windowCost.normal')
  }
})

const sessionsRow = computed<CapacityRow | null>(() => {
  const max = props.account.max_sessions ?? 0
  if (!isAnthropicOAuthOrSetupToken.value || max <= 0) return null
  const current = props.account.active_sessions ?? 0
  const idle = props.account.session_idle_timeout_minutes || 5
  return {
    key: 'sessions',
    label: t('admin.accounts.capacity.sessions.label'),
    current,
    max,
    toneClass: toneOf(current, max),
    hint: current >= max ? t('admin.accounts.capacity.sessions.full', { idle }) : t('admin.accounts.capacity.sessions.normal', { idle })
  }
})

// RPM：分层限流 = 到基准后只接粘性会话、再超缓冲区才停；粘性豁免 = 到基准后只接粘性会话
const rpmRow = computed<CapacityRow | null>(() => {
  const base = props.account.base_rpm ?? 0
  if (!isAnthropicOAuthOrSetupToken.value || base <= 0) return null
  const current = props.account.current_rpm ?? 0
  const tiered = (props.account.rpm_strategy || 'tiered') !== 'sticky_exempt'
  const buffer = props.account.rpm_sticky_buffer ?? Math.max(1, Math.floor(base / 5))
  let hint: string
  if (tiered) {
    if (current >= base + buffer) hint = t('admin.accounts.capacity.rpm.tieredBlocked', { buffer })
    else if (current >= base) hint = t('admin.accounts.capacity.rpm.tieredStickyOnly', { buffer })
    else if (current >= base * 0.8) hint = t('admin.accounts.capacity.rpm.tieredWarning')
    else hint = t('admin.accounts.capacity.rpm.tieredNormal')
  } else if (current >= base) hint = t('admin.accounts.capacity.rpm.stickyExemptOver')
  else if (current >= base * 0.8) hint = t('admin.accounts.capacity.rpm.stickyExemptWarning')
  else hint = t('admin.accounts.capacity.rpm.stickyExemptNormal')
  return {
    key: 'rpm',
    label: t('admin.accounts.capacity.rpm.label'),
    current,
    max: base,
    // 分层限流到基准只是黄区，超出缓冲区才红；粘性豁免没有红区
    toneClass: tiered ? toneOf(current, base, base + buffer) : current >= base * 0.8 ? 'text-af-warning' : '',
    hint
  }
})

const rows = computed(() =>
  [concurrencyRow.value, windowCostRow.value, sessionsRow.value, rpmRow.value].filter((row): row is CapacityRow => row !== null)
)
</script>
