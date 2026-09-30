<template>
  <!--
    一个模型在时间窗里的逐段状态：一段一格，颜色是那一段的综合状态，悬停看那一段的可用率和首字延迟。
    灰格 = 那一段请求太少或没有请求。放在模型格子里，是一条细条（最多约 30 段）。
  -->
  <ul class="flex h-2 items-stretch gap-px" :aria-label="label">
    <li
      v-for="slot in slots"
      :key="slot.start.getTime()"
      class="min-w-[2px] flex-1 rounded-[1px]"
      :class="HEALTH_BAR[slot.point?.health.overall ?? 'unknown']"
      :title="describe(slot)"
      :aria-label="describe(slot)"
    />
  </ul>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { HEALTH_BAR, formatAvailability, formatLatency, formatSlotTime, type StatusSlot } from './serviceStatus'

const props = defineProps<{
  slots: StatusSlot[]
  bucketSeconds: number
  label: string
}>()

const { t, locale } = useI18n()

function describe(slot: StatusSlot): string {
  const time = formatSlotTime(slot.start, props.bucketSeconds, locale.value)
  const metrics = slot.point?.metrics
  if (!metrics || metrics.availability == null) return t('userUi.serviceStatus.slot.noRequests', { time })
  const detail = t('userUi.serviceStatus.slot.detail', {
    time,
    availability: formatAvailability(metrics.availability),
    ttft: formatLatency(metrics.ttft_p50_ms)
  })
  // 有请求但太少时格子是灰的，说明一下为什么不评状态
  return slot.point?.health.overall === 'unknown' ? `${detail}${t('userUi.serviceStatus.slot.fewRequests')}` : detail
}
</script>
