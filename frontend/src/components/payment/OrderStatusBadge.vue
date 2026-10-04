<template>
  <!-- 订单状态：小圆点 + 文字（控制台单色为主，muqian 2026-09-23）；只有失败类标红，终止类淡灰 -->
  <span class="inline-flex items-center gap-1.5 whitespace-nowrap text-13" :class="tone.text">
    <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="tone.dot" aria-hidden="true" />
    {{ statusLabel }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OrderStatus } from '@/types/payment'

const props = defineProps<{
  status: OrderStatus
}>()

const { t } = useI18n()

type Tone = 'normal' | 'muted' | 'failed'

const TONES: Record<Tone, { dot: string; text: string }> = {
  normal: { dot: 'bg-af-ink', text: 'text-af-ink-2' },
  muted: { dot: 'bg-af-ink-4', text: 'text-af-ink-3' },
  failed: { dot: 'bg-af-danger', text: 'text-af-danger' }
}

const statusMap: Record<OrderStatus, { key: string; tone: Tone }> = {
  PENDING: { key: 'payment.status.pending', tone: 'normal' },
  PAID: { key: 'payment.status.paid', tone: 'normal' },
  RECHARGING: { key: 'payment.status.recharging', tone: 'normal' },
  COMPLETED: { key: 'payment.status.completed', tone: 'normal' },
  EXPIRED: { key: 'payment.status.expired', tone: 'muted' },
  CANCELLED: { key: 'payment.status.cancelled', tone: 'muted' },
  FAILED: { key: 'payment.status.failed', tone: 'failed' },
  REFUND_REQUESTED: { key: 'payment.status.refund_requested', tone: 'normal' },
  REFUNDING: { key: 'payment.status.refunding', tone: 'normal' },
  REFUND_PENDING: { key: 'payment.status.refund_pending', tone: 'normal' },
  REFUNDED: { key: 'payment.status.refunded', tone: 'normal' },
  PARTIALLY_REFUNDED: { key: 'payment.status.partially_refunded', tone: 'normal' },
  REFUND_FAILED: { key: 'payment.status.refund_failed', tone: 'failed' }
}

const statusLabel = computed(() => {
  const entry = statusMap[props.status]
  return entry ? t(entry.key) : props.status
})

const tone = computed(() => TONES[statusMap[props.status]?.tone ?? 'muted'])
</script>
