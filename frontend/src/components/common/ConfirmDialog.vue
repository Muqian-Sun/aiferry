<template>
  <BaseDialog :show="show" :title="title" width="narrow" @close="handleCancel">
    <div class="space-y-4">
      <p class="text-sm text-af-ink-2">{{ message }}</p>
      <slot></slot>
    </div>

    <template #footer>
      <!-- 打开时焦点：危险操作落在「取消」（误按 Enter 不会删），其余落在确认 -->
      <div class="flex justify-end space-x-3">
        <button
          @click="handleCancel"
          type="button"
          class="btn btn-secondary btn-md"
          :data-autofocus="danger ? '' : undefined"
        >
          {{ cancelText }}
        </button>
        <button
          @click="handleConfirm"
          type="button"
          :class="['btn btn-md', danger ? 'btn-danger' : 'btn-primary']"
          :disabled="loading"
          :aria-busy="loading ? 'true' : undefined"
          :data-autofocus="danger ? undefined : ''"
        >
          {{ confirmText }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'

const { t } = useI18n()

interface Props {
  show: boolean
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
  /** 确认后请求进行中（调用方保持弹窗打开时传）：确认按钮禁用并转圈，防止重复提交 */
  loading?: boolean
}

interface Emits {
  (e: 'confirm'): void
  (e: 'cancel'): void
}

const props = withDefaults(defineProps<Props>(), {
  danger: false,
  loading: false
})

const confirmText = computed(() => props.confirmText || t('common.confirm'))
const cancelText = computed(() => props.cancelText || t('common.cancel'))

const emit = defineEmits<Emits>()

// 弹窗关闭的过渡（200ms）里按钮还在、还能点：已经在关或请求进行中时不再发 confirm，双击不会提交两次
const handleConfirm = () => {
  if (!props.show || props.loading) return
  emit('confirm')
}

const handleCancel = () => {
  emit('cancel')
}
</script>
