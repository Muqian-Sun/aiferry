<template>
  <AppLayout>
    <div data-testid="account-security-shell" class="mx-auto max-w-[950px] space-y-6">
      <ProfilePasswordForm />
      <ProfileTotpCard />
      <ProfilePasskeyCard :enabled="passkeyEnabled" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
/**
 * 管理员账号安全：密码、双因素、Passkey。管理后台不提供余额提醒、三方账号绑定等
 * 用户站功能，对应接口也不在管理后台注册。
 */
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'
import ProfileTotpCard from '@/components/user/profile/ProfileTotpCard.vue'
import ProfilePasskeyCard from '@/components/user/profile/ProfilePasskeyCard.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const appStore = useAppStore()
const authStore = useAuthStore()
const passkeyEnabled = ref(false)

onMounted(async () => {
  const profileRefresh = authStore.refreshUser().catch((error) => {
    console.error('Failed to refresh profile:', error)
  })
  const settingsLoad = appStore.fetchPublicSettings()
    .then((settings) => {
      passkeyEnabled.value = settings?.passkey_enabled === true
    })
    .catch((error) => {
      console.error('Failed to load settings:', error)
    })
  await Promise.all([profileRefresh, settingsLoad])
})
</script>
