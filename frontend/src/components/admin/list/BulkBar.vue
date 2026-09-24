<template>
  <!--
    批量操作条（A4）：只在选中行时出现，墨色底，一眼看出正处在批量模式。
    放在 TablePageLayout 的 #bulk 插槽里，位于表格和分页之间，不随表格滚动。
  -->
  <div
    v-if="count > 0"
    class="bulk-bar flex flex-wrap items-center justify-between gap-3 rounded-lg bg-af-ink px-4 py-2.5 text-sm text-af-sheet"
    role="region"
    :aria-label="label"
    data-testid="bulk-bar"
  >
    <span class="tabular-nums">{{ label }}</span>
    <div class="bulk-actions flex flex-wrap items-center gap-2">
      <slot />
      <button type="button" class="bulk-btn" data-testid="bulk-clear" @click="$emit('clear')">
        {{ t('common.cancelSelection') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ count: number }>()
defineEmits<{ (e: 'clear'): void }>()

const { t } = useI18n()
const label = computed(() => t('common.selectedItems', { count: props.count }))
</script>

<style scoped>
/* 出现时轻轻上滑；消失不做动画，取消选择后立刻收起 */
.bulk-bar {
  animation: bulk-bar-in 150ms ease-out;
}
@keyframes bulk-bar-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
}
@media (prefers-reduced-motion: reduce) {
  .bulk-bar {
    animation: none;
  }
}
/* 批量条里的按钮：墨色底上的描边按钮，危险操作用 .bulk-btn-danger */
.bulk-actions :deep(.bulk-btn) {
  @apply inline-flex h-8 items-center gap-1.5 rounded-md border px-3 text-13 transition-colors disabled:cursor-not-allowed disabled:opacity-40;
  border-color: rgb(var(--af-sheet) / 0.3);
  color: rgb(var(--af-sheet));
}
.bulk-actions :deep(.bulk-btn:hover:not(:disabled)) {
  background-color: rgb(var(--af-sheet) / 0.12);
}
.bulk-actions :deep(.bulk-btn-danger) {
  border-color: rgb(var(--af-danger) / 0.7);
}
</style>
