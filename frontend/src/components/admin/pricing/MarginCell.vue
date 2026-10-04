<template>
  <!-- 毛利 = 1 − 上游成本比 ÷ 默认售价倍率；低于最低毛利率标红（利润门会跳过）。改过的行要保存后才重算。 -->
  <span v-if="margin === undefined" class="whitespace-nowrap text-xs text-af-ink-3">{{ t('admin.pricing.marginAfterSave') }}</span>
  <span v-else-if="margin === null" class="text-af-ink-3">—</span>
  <span
    v-else
    :class="['inline-flex rounded-full px-2 py-0.5 text-xs font-medium tabular-nums', tone]"
    data-testid="pricing-margin"
  >
    {{ formatted }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { belowMinMargin } from './pricingDraft'

const props = defineProps<{
  /** undefined = 改过、要保存后重算；null = 官方价没有可比的项 */
  margin: number | null | undefined
  minMargin: number
}>()

const { t } = useI18n()

const formatted = computed(() => `${Math.round((props.margin ?? 0) * 1000) / 10}%`)
const tone = computed(() => {
  if (props.margin == null) return ''
  if (belowMinMargin(props.margin, props.minMargin)) return 'bg-af-danger-tint text-af-danger'
  return props.minMargin > 0 ? 'bg-af-success-tint text-af-success' : 'bg-af-sunken text-af-ink-2'
})
</script>
