<template>
  <p
    v-if="message"
    class="input-error-text flex items-start gap-1"
    role="alert"
    data-testid="form-error"
  >
    <Icon name="exclamationCircle" size="sm" class="mt-px shrink-0" aria-hidden="true" />
    <span class="min-w-0 whitespace-pre-line break-words">{{ message }}</span>
  </p>
</template>

<script setup lang="ts">
/**
 * 表单行内报错：放在提交按钮上方或对应字段下方，小号 danger 色文字 + 错误图标。
 * message 为空时不渲染；由调用方在重新输入 / 再次提交时清空。
 */
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  message?: string | null
}>()
</script>
