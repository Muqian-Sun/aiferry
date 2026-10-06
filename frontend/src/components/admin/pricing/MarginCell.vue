<template>
  <!--
    毛利 = 1 − 上游成本比 ÷ 默认售价倍率；低于最低毛利率标红（利润门会跳过）。改过的行要保存后才重算。
    上游忙时涨价、我们没涨时，下面再标一行忙时毛利（一周里最差的时段）。
  -->
  <span v-if="margin === undefined" class="whitespace-nowrap text-xs text-af-ink-3">{{ t('admin.pricing.marginAfterSave') }}</span>
  <span v-else-if="margin === null" class="text-af-ink-3">—</span>
  <span v-else class="inline-flex flex-col items-end gap-0.5">
    <span :class="['inline-flex rounded-full px-2 py-0.5 text-xs font-medium tabular-nums', toneOf(margin)]" data-testid="pricing-margin">
      {{ format(margin) }}
    </span>
    <span
      v-if="peakMargin != null"
      :class="['whitespace-nowrap text-xs tabular-nums', belowMinMargin(peakMargin, minMargin) ? 'text-af-danger' : 'text-af-ink-3']"
      data-testid="pricing-peak-margin"
    >
      {{ t('admin.pricing.peak.margin', { margin: format(peakMargin) }) }}
    </span>
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { belowMinMargin } from './pricingDraft'

const props = defineProps<{
  /** undefined = 改过、要保存后重算；null = 官方价没有可比的项 */
  margin: number | null | undefined
  /** 忙时毛利；null / 不给 = 忙时不比平时差 */
  peakMargin?: number | null
  minMargin: number
}>()

const { t } = useI18n()

function format(margin: number): string {
  return `${Math.round(margin * 1000) / 10}%`
}

function toneOf(margin: number): string {
  if (belowMinMargin(margin, props.minMargin)) return 'bg-af-danger-tint text-af-danger'
  return props.minMargin > 0 ? 'bg-af-success-tint text-af-success' : 'bg-af-sunken text-af-ink-2'
}
</script>
