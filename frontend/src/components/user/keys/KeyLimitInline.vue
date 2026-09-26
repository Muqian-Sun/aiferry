<template>
  <!-- 密钥表「用量 / 限额」列的第二行：用得最满的一项限额（标签 · 细条 · 百分比）；没设任何限额写「不限额」 -->
  <div v-if="meter" class="flex items-center gap-2 text-xs" data-testid="key-limit-meter">
    <span class="w-8 shrink-0 text-af-ink-3">{{ meter.kind === 'quota' ? t('keys.quota') : meter.kind }}</span>
    <span class="h-1 min-w-0 flex-1 overflow-hidden rounded-full bg-af-hairline">
      <span class="block h-full rounded-full" :class="LEVEL_BAR[level]" :style="{ width: `${Math.min(meter.ratio, 1) * 100}%` }" />
    </span>
    <span class="w-9 shrink-0 text-right tabular-nums" :class="LEVEL_TEXT[level]">{{ Math.round(meter.ratio * 100) }}%</span>
  </div>
  <div v-else class="text-xs text-af-ink-4">{{ t('keys.noLimit') }}</div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { limitLevel, type KeyLimitMeter } from './keyAttention'

const props = defineProps<{ meter: KeyLimitMeter | null }>()
const { t } = useI18n()

const LEVEL_BAR = { normal: 'bg-af-ink', warning: 'bg-af-warning', danger: 'bg-af-danger' } as const
const LEVEL_TEXT = { normal: 'text-af-ink-3', warning: 'text-af-warning', danger: 'text-af-danger' } as const
const level = computed(() => limitLevel(props.meter?.ratio ?? 0))
</script>
