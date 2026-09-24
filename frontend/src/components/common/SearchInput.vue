<template>
  <div class="relative w-full">
    <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center" :class="compact ? 'pl-2.5' : 'pl-3'">
      <Icon name="search" :size="compact ? 'sm' : 'md'" class="text-af-ink-4" />
    </div>
    <input
      :value="modelValue"
      type="text"
      :class="['input', compact ? 'h-8 py-0 pl-8 pr-3 text-13' : 'pl-10']"
      :placeholder="placeholder"
      @input="handleInput"
    />
  </div>
</template>

<script setup lang="ts">
import { useDebounceFn } from '@vueuse/core'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  debounceMs?: number
  /** 管理站列表工具行用的 32px 高紧凑版，和筛选标签同高 */
  compact?: boolean
}>(), {
  placeholder: 'Search...',
  debounceMs: 300
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'search', value: string): void
}>()

const debouncedEmitSearch = useDebounceFn((value: string) => {
  emit('search', value)
}, props.debounceMs)

const handleInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  emit('update:modelValue', value)
  debouncedEmitSearch(value)
}
</script>
