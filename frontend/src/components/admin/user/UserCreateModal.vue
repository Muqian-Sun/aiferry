<template>
  <BaseDialog :show="show" :title="t('admin.users.createUser')" width="normal" @close="$emit('close')">
    <form id="create-user-form" novalidate @submit.prevent="submit">
      <UserFormFields v-model:form="form" mode="create" :errors="errors" />
    </form>
    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-3">
        <FormError class="mr-auto min-w-0 flex-1" :message="submitError" />
        <button type="button" class="btn btn-secondary" @click="$emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="create-user-form" :disabled="loading" class="btn btn-primary" data-testid="user-create-submit">
          {{ loading ? t('admin.users.creating') : t('common.create') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <!-- 创建管理员账号时后端要求 step-up 2FA，弹出 TOTP 验证后自动重试 -->
  <TotpStepUpDialog :controller="stepUp" />
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { NewUserDefaults } from '@/api/admin/users'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import UserFormFields from './UserFormFields.vue'
import { emptyUserForm, validateUserForm, type UserFormErrors, type UserFormState } from './userForm'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits(['close', 'success'])
const { t } = useI18n()

const form = ref<UserFormState>(emptyUserForm())
const errors = ref<UserFormErrors>({})
const submitError = ref('')
const loading = ref(false)
const stepUp = useStepUp()

// 新用户默认并发 / RPM（后端常量）：打开就填上（muqian 2026-09-30「直接显示默认值」）。
// 拿不到时两格留空，空的就不传、由后端按默认值建——不能按「清空 = 0」发 0
const defaults = ref<NewUserDefaults | null>(null)

async function loadDefaults() {
  try {
    defaults.value = await adminAPI.users.getNewUserDefaults()
    if (form.value.concurrency === '') form.value.concurrency = defaults.value.concurrency
    if (form.value.rpm_limit === '') form.value.rpm_limit = defaults.value.rpm_limit
  } catch {
    defaults.value = null
  }
}

watch(
  () => props.show,
  (show) => {
    if (!show) return
    form.value = emptyUserForm()
    errors.value = {}
    submitError.value = ''
    void loadDefaults()
  },
  { immediate: true }
)

const isBlank = (value: string | number) => String(value ?? '').trim() === ''

const submit = async () => {
  if (loading.value) return
  submitError.value = ''
  const { errors: fieldErrors, values } = validateUserForm(form.value, 'create', t)
  errors.value = fieldErrors
  if (Object.keys(fieldErrors).length > 0) return

  const payload = {
    email: form.value.email.trim(),
    password: form.value.password.trim(),
    username: form.value.username.trim(),
    notes: form.value.notes.trim(),
    role: form.value.role,
    balance: values.balance,
    concurrency: !defaults.value && isBlank(form.value.concurrency) ? undefined : values.concurrency,
    rpm_limit: !defaults.value && isBlank(form.value.rpm_limit) ? undefined : values.rpm_limit,
    rate_multiplier: values.rate_multiplier
  }
  loading.value = true
  try {
    // 创建管理员属敏感操作：后端返回 STEP_UP_REQUIRED 时弹 TOTP 验证并重试
    await stepUp.run(() => adminAPI.users.create(payload))
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
      submitError.value = extractApiErrorMessage(error, t('admin.users.failedToCreate'))
    }
  } finally {
    loading.value = false
  }
}
</script>
