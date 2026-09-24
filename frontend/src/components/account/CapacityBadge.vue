<script setup lang="ts">
defineProps<{
  colorClass: string
  tooltip?: string
  current: string | number
  max: string | number
  suffix?: string
}>()
</script>

<template>
  <span
    :class="[
      'inline-flex items-center gap-1 text-xs leading-4 tabular-nums',
      colorClass
    ]"
    :title="tooltip"
  >
    <slot />
    <span class="font-mono">{{ current }}</span>
    <span class="text-af-ink-4">/</span>
    <span class="font-mono">{{ max }}</span>
    <span v-if="suffix" class="text-[9px] opacity-60">{{ suffix }}</span>
  </span>
</template>
