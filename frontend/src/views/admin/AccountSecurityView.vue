<template>
  <AppLayout>
    <!--
      与用户站「基本信息」同一种两栏设置行（左标题 + 说明，右内容），行间 hairline，不套卡片（2026-10-05）；
      Passkey 只在站点开启了才出现。-mt-8 抵掉第一行的顶部留白。
    -->
    <div data-testid="account-security-shell" class="-mt-8 divide-y divide-af-hairline">
      <SettingsRow :title="t('profile.changePassword')" :description="t('userUi.account.rows.passwordDesc')">
        <ProfilePasswordForm headless />
      </SettingsRow>
      <SettingsRow :title="t('profile.totp.title')" :description="t('profile.totp.description')">
        <ProfileTotpCard headless />
      </SettingsRow>
      <SettingsRow v-if="passkeyEnabled" :title="t('profile.passkey.title')" :description="t('profile.passkey.description')">
        <ProfilePasskeyCard :enabled="passkeyEnabled" headless />
      </SettingsRow>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
/**
 * 管理员账号安全：密码、双因素、Passkey。管理后台不提供余额提醒、三方账号绑定等
 * 用户站功能，对应接口也不在管理后台注册。
 */
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import SettingsRow from '@/components/user/shell/SettingsRow.vue'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'
import ProfileTotpCard from '@/components/user/profile/ProfileTotpCard.vue'
import ProfilePasskeyCard from '@/components/user/profile/ProfilePasskeyCard.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
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
