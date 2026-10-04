<template>
  <!--
    分段切换：同一块内容换一种看法（指标、时间范围、状态筛选）时用它；页内分区（明细 / 错误）用 SectionTabs。
    浅底胶囊里一排小按钮。2026-10-04 起两站这类切换都用这一个组件（原来概览、管理站概览、收款概览、渠道 / 密钥详情各手写一份，
    价格页是另一种实心墨色样子，用户站概览的时间范围用的是下划线页签）。
  -->
  <div class="inline-flex shrink-0 rounded-lg bg-af-sunken p-1" role="radiogroup" :aria-label="label">
    <button
      v-for="option in options"
      :key="option.key"
      type="button"
      role="radio"
      class="whitespace-nowrap rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
      :class="modelValue === option.key ? 'bg-af-sheet text-af-ink' : 'text-af-ink-3 hover:text-af-ink-2'"
      :aria-checked="modelValue === option.key"
      :data-testid="testIdPrefix ? `${testIdPrefix}-${option.key}` : undefined"
      @click="modelValue !== option.key && emit('update:modelValue', option.key)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<script setup lang="ts" generic="K extends string | number">
defineProps<{
  modelValue: K
  options: Array<{ key: K; label: string }>
  label: string
  testIdPrefix?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: K] }>()
</script>
