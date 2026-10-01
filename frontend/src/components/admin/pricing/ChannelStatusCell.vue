<template>
  <!-- 承接行的状态：渠道此刻能不能调度；毛利低于门槛时再标一行「利润门会跳过」 -->
  <div class="whitespace-nowrap text-xs">
    <span :class="stateClass">{{ stateText }}</span>
    <div v-if="belowMinMargin(margin ?? null, minMargin)" class="text-af-danger">{{ t('admin.pricing.gateSkips') }}</div>
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
  minMargin: number
}>()

const { t } = useI18n()

const state = computed<'missing' | 'disabled' | 'paused' | 'ok'>(() => {
  if (!props.account) return 'missing'
  if (props.account.status !== 'active') return 'disabled'
  return props.account.schedulable ? 'ok' : 'paused'
})
const stateText = computed(() => t(`admin.pricing.channelState.${state.value}`))
const stateClass = computed(() => ({ ok: 'text-af-ink-2', paused: 'text-af-warning', disabled: 'text-af-ink-4', missing: 'text-af-ink-4' })[state.value])
</script>
