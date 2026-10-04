<template>
  <p
    v-if="message"
    class="flex items-start gap-1 text-13 text-af-success"
    role="status"
    data-testid="form-success"
  >
    <Icon name="check" size="sm" class="mt-px shrink-0" aria-hidden="true" />
    <span class="min-w-0 break-words">{{ message }}</span>
  </p>
</template>

<script setup lang="ts">
/**
 * 表单行内成功提示：和 FormError 成对，放在同一位置。
 * 站内没有 toast 层，保存类操作成功后靠它让人看见「做成了」；配合 useTransientMessage 几秒后自动消失。
 */
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  message?: string | null
}>()
</script>
