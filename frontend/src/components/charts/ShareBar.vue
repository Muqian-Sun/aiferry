<template>
  <!-- 分布表的占比列：单色墨条 + 百分比（A2 起替代多色环形图）。不到 10% 的非零占比保留一位小数，免得小份额都显示成 0%。 -->
  <span class="flex items-center gap-2">
    <span class="h-1.5 min-w-0 flex-1 overflow-hidden rounded-full bg-af-sunken">
      <span class="block h-full rounded-full bg-af-ink" :style="{ width: `${(share * 100).toFixed(1)}%` }" />
    </span>
    <span class="w-11 shrink-0 text-right tabular-nums text-af-ink-3">{{ percent }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ value: number; total: number }>()

const share = computed(() => (props.total > 0 ? props.value / props.total : 0))
const percent = computed(() => `${(share.value * 100).toFixed(share.value > 0 && share.value < 0.1 ? 1 : 0)}%`)
</script>
