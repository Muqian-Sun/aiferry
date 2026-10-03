<template>
  <BaseDialog :show="show" :title="t('admin.users.editUser')" width="normal" @close="$emit('close')">
    <form v-if="user" id="edit-user-form" novalidate class="space-y-4" @submit.prevent="handleUpdateUser">
      <UserFormFields
        v-model:form="form"
        mode="edit"
        :errors="errors"
        :current-balance="user.balance"
        @adjust-balance="(operation) => emit('adjust-balance', operation)"
      />
      <!-- 自定义属性：只在定义过属性时出现（组件自己判断） -->
      <UserAttributeForm v-model="customAttributes" :user-id="user.id" />
    </form>
    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-3">
        <FormError class="mr-auto min-w-0 flex-1" :message="submitError" />
        <button type="button" class="btn btn-secondary" @click="$emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="edit-user-form" :disabled="submitting" class="btn btn-primary" data-testid="user-edit-submit">
          {{ submitting ? t('admin.users.updating') : t('common.update') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <!-- 角色提升为管理员时后端要求 step-up 2FA，弹出 TOTP 验证后自动重试 -->
  <TotpStepUpDialog :controller="stepUp" />
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUser, UserAttributeValuesMap } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import UserAttributeForm from '@/components/user/UserAttributeForm.vue'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import UserFormFields from './UserFormFields.vue'
import { emptyUserForm, validateUserForm, type UserFormErrors, type UserFormState } from './userForm'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits<{
  close: []
  success: []
  /** 「充值 / 扣减」：由列表页打开现有的余额弹窗 */
  'adjust-balance': [operation: 'add' | 'subtract']
}>()
const { t } = useI18n()

const form = ref<UserFormState>(emptyUserForm())
const customAttributes = ref<UserAttributeValuesMap>({})
const errors = ref<UserFormErrors>({})
const submitError = ref('')
const submitting = ref(false)
const stepUp = useStepUp()

// 只在打开 / 换了一个用户时回填：在这里点「充值」改了余额，列表把新余额传进来，不能把正在改的字段冲掉
watch(
  [() => props.show, () => props.user?.id],
  ([show, id], [wasShow, previousId]) => {
    const user = props.user
    if (!show || !user || (wasShow && id === previousId)) return
    form.value = {
      email: user.email,
      password: '',
      username: user.username || '',
      role: user.role || 'user',
      rate_multiplier: user.custom_rate_multiplier ?? '',
      balance: '',
      concurrency: user.concurrency,
      rpm_limit: user.rpm_limit ?? 0,
      notes: user.notes || ''
    }
    customAttributes.value = {}
    errors.value = {}
    submitError.value = ''
  },
  { immediate: true }
)

const handleUpdateUser = async () => {
  const user = props.user
  if (!user) return
  submitError.value = ''
  const { errors: fieldErrors, values } = validateUserForm(form.value, 'edit', t)
  errors.value = fieldErrors
  if (Object.keys(fieldErrors).length > 0) return

  const data: Record<string, unknown> = {
    email: form.value.email.trim(),
    username: form.value.username.trim(),
    notes: form.value.notes.trim(),
    role: form.value.role,
    // 清空按 0（= 不限）：原来空着直接发 "" 给后端，必定 400
    concurrency: values.concurrency,
    rpm_limit: values.rpm_limit
  }
  // 倍率留空 = 改回全站默认；填了就单独设
  if (values.rate_multiplier === undefined) data.use_default_rate_multiplier = true
  else data.rate_multiplier = values.rate_multiplier
  const password = form.value.password.trim()
  if (password) data.password = password

  submitting.value = true
  try {
    // 提升为管理员属敏感操作：后端返回 STEP_UP_REQUIRED 时弹 TOTP 验证并重试
    await stepUp.run(() => adminAPI.users.update(user.id, data))
    if (Object.keys(customAttributes.value).length > 0) {
      await adminAPI.userAttributes.updateUserAttributeValues(user.id, customAttributes.value)
    }
    emit('success')
    emit('close')
  } catch (error) {
    if (isStepUpCancelled(error)) {
      // 用户主动取消二次验证：表单保持打开
    } else if (isStepUpBlocked(error)) {
      submitError.value =
        stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' ? t('stepUp.adminApiKeyForbidden') : t('stepUp.notEnabled')
    } else if (extractApiErrorCode(error) === 'EMAIL_EXISTS') {
      errors.value = { ...errors.value, email: t('admin.users.form.emailExists') }
    } else {
      submitError.value = extractApiErrorMessage(error, t('admin.users.failedToUpdate'))
    }
  } finally {
    submitting.value = false
  }
}
</script>
