<template>
  <!-- 表格行里用的小开关（A5）：比表单里的 Toggle 小一号、颜色淡一档，一列开关不会压过内容。点它不会触发整行点击。 -->
  <button
    type="button"
    role="switch"
    :aria-checked="modelValue ? 'true' : 'false'"
    :aria-label="label"
    :title="label"
    :disabled="disabled"
    class="relative inline-flex h-4 w-7 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-af-brand/40 disabled:cursor-not-allowed disabled:opacity-50"
    :class="modelValue ? 'bg-af-ink-2' : 'bg-af-hairline-strong'"
    @click.stop="emit('toggle')"
  >
    <span
      class="pointer-events-none inline-block h-3 w-3 rounded-full bg-af-sheet transition-transform duration-150"
      :class="modelValue ? 'translate-x-3.5' : 'translate-x-0.5'"
    />
  </button>
</template>

<script setup lang="ts">
defineProps<{ modelValue: boolean; disabled?: boolean; label?: string }>()
const emit = defineEmits<{ (e: 'toggle'): void }>()
</script>
