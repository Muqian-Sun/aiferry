<template>
  <!-- 渠道弹窗里的一行开关：左边标题 + 说明，右边开关；打开后下面接着显示默认插槽（数值输入、提示等） -->
  <div :data-testid="testId">
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <label class="input-label mb-0">{{ label }}</label>
        <p v-if="description" class="mt-1 text-xs text-af-ink-3">{{ description }}</p>
      </div>
      <Toggle
        :model-value="modelValue"
        :aria-label="label"
        :data-testid="toggleTestId ?? (testId ? `${testId}-toggle` : undefined)"
        @update:model-value="emit('update:modelValue', $event)"
      />
    </div>
    <div v-if="modelValue && $slots.default" class="mt-3">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import Toggle from '@/components/common/Toggle.vue'

defineProps<{
  modelValue: boolean
  label: string
  description?: string
  testId?: string
  /** 开关按钮的 testid；不传时为 `${testId}-toggle` */
  toggleTestId?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
</script>
