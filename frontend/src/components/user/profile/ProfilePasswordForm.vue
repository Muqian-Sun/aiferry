<template>
  <!--
    小标题 + 表单；外层容器由调用方决定。用户站的两栏设置行自己画标题（headless），管理端卡片里保留小标题。
  -->
  <div class="space-y-4">
    <h3 v-if="!headless" class="text-sm font-semibold text-af-ink">
      {{ t('profile.changePassword') }}
    </h3>
    <div>
      <form @submit.prevent="handleChangePassword" class="space-y-4">
        <div>
          <label for="old_password" class="input-label">
            {{ t('profile.currentPassword') }}
          </label>
          <input
            id="old_password"
            v-model="form.old_password"
            type="password"
            required
            autocomplete="current-password"
            class="input"
          />
        </div>

        <div>
          <label for="new_password" class="input-label">
            {{ t('profile.newPassword') }}
          </label>
          <input
            id="new_password"
            v-model="form.new_password"
            type="password"
            required
            autocomplete="new-password"
            class="input"
          />
          <p class="input-hint">
            {{ t('profile.passwordHint') }}
          </p>
        </div>

        <div>
          <label for="confirm_password" class="input-label">
            {{ t('profile.confirmNewPassword') }}
          </label>
          <input
            id="confirm_password"
            v-model="form.confirm_password"
            type="password"
            required
            autocomplete="new-password"
            class="input"
          />
        </div>

        <div class="flex justify-end pt-2">
          <button type="submit" :disabled="loading" class="btn btn-primary btn-sm">
            {{ loading ? t('profile.changingPassword') : t('profile.changePasswordButton') }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { userAPI } from '@/api'
import { extractApiErrorMessage } from '@/utils/apiError'

/** 用户站两栏设置行里不画自带小标题 */
defineProps<{ headless?: boolean }>()

const { t } = useI18n()
const loading = ref(false)
const form = ref({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

const handleChangePassword = async () => {
  if (form.value.new_password !== form.value.confirm_password) {
    console.error(t('profile.passwordsNotMatch'))
    return
  }

  if (form.value.new_password.length < 8) {
    console.error(t('profile.passwordTooShort'))
    return
  }

  loading.value = true
  try {
    await userAPI.changePassword(form.value.old_password, form.value.new_password)
    form.value = { old_password: '', new_password: '', confirm_password: '' }
  } catch (error: unknown) {
    console.error(extractApiErrorMessage(error, t('profile.passwordChangeFailed')), error)
  } finally {
    loading.value = false
  }
}
</script>
