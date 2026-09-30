<template>
  <!-- 服务状态页的两处切换（整体趋势的指标、各模型的状态筛选）：浅底胶囊里一排小按钮，与概览页的指标切换同一个样子 -->
  <div class="inline-flex shrink-0 rounded-lg bg-af-sunken p-1" role="tablist" :aria-label="label">
    <button
      v-for="option in options"
      :key="option.key"
      type="button"
      role="tab"
      class="whitespace-nowrap rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
      :class="modelValue === option.key ? 'bg-af-sheet text-af-ink' : 'text-af-ink-3 hover:text-af-ink-2'"
      :aria-selected="modelValue === option.key"
      @click="emit('update:modelValue', option.key)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<script setup lang="ts" generic="K extends string">
defineProps<{
  modelValue: K
  options: Array<{ key: K; label: string }>
  label: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: K] }>()
</script>
