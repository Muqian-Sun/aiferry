<template>
  <!-- 承接行的状态：渠道此刻能不能调度；毛利低于门槛时再标一行「利润门会跳过」（只在忙时低于门槛的标「忙时会跳过」） -->
  <div class="whitespace-nowrap text-xs">
    <span :class="stateClass">{{ stateText }}</span>
    <div v-if="belowMinMargin(margin ?? null, minMargin)" class="text-af-danger">{{ t('admin.pricing.gateSkips') }}</div>
    <div v-else-if="margin != null && belowMinMargin(peakMargin ?? null, minMargin)" class="text-af-danger">{{ t('admin.pricing.peak.gateSkips') }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PricingAccount } from '@/api/admin/pricing'
import { belowMinMargin } from './pricingDraft'

const props = defineProps<{
  account: PricingAccount | undefined
  margin: number | null | undefined
  /** 忙时毛利；null / 不给 = 忙时不比平时差 */
  peakMargin?: number | null
  minMargin: number
}>()

const { t } = useI18n()

// 与渠道页同一套叫法：出错（status=error）叫「异常」，手动停用才叫「已停用」
const state = computed<'missing' | 'error' | 'disabled' | 'paused' | 'ok'>(() => {
  if (!props.account) return 'missing'
  if (props.account.status === 'error') return 'error'
  if (props.account.status !== 'active') return 'disabled'
  return props.account.schedulable ? 'ok' : 'paused'
})
const stateText = computed(() => t(`admin.pricing.channelState.${state.value}`))
const stateClass = computed(() => ({ ok: 'text-af-ink-2', paused: 'text-af-warning', error: 'text-af-danger', disabled: 'text-af-ink-3', missing: 'text-af-ink-3' })[state.value])
</script>
