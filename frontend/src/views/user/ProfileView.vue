<template>
  <!--
    账户：基本信息 / 安全 / 通知 三个子页（muqian 2026-09-23 侧栏「账户」组），路由传 section，这里只渲染对应一节。
    页头标题取路由 meta；每节是两栏设置行（左标题 + 说明，右内容，muqian 2026-09-23 定），行间 hairline。
    -mt-8 抵掉第一行的顶部留白（页头已有 mb-8）。
  -->
  <SiteShell>
    <div data-testid="profile-shell" class="-mt-8">
      <template v-if="section === 'profile'">
        <ProfileInfoCard :user="user" />
        <p v-if="contactInfo" class="border-t border-af-hairline py-6 text-13 text-af-ink-3">
          {{ t('common.contactSupport') }}:
          <span class="font-medium text-af-ink-2">{{ contactInfo }}</span>
        </p>
      </template>

      <div v-else-if="section === 'security'" class="divide-y divide-af-hairline">
        <SettingsRow :title="t('profile.authBindings.title')" :description="t('profile.authBindings.description')">
          <div data-testid="profile-auth-bindings-panel">
            <ProfileIdentityBindingsSection
              :user="user"
              :wechat-enabled="wechatOAuthEnabled"
              :wechat-open-enabled="wechatOAuthOpenEnabled"
              :wechat-mp-enabled="wechatOAuthMPEnabled"
              compact
            />
          </div>
        </SettingsRow>
        <SettingsRow :title="t('profile.changePassword')" :description="t('userUi.account.rows.passwordDesc')">
          <ProfilePasswordForm headless />
        </SettingsRow>
        <SettingsRow :title="t('profile.totp.title')" :description="t('profile.totp.description')">
          <ProfileTotpCard headless />
        </SettingsRow>
        <SettingsRow :title="t('profile.passkey.title')" :description="t('profile.passkey.description')">
          <ProfilePasskeyCard :enabled="passkeyEnabled" headless />
        </SettingsRow>
      </div>

      <template v-else>
        <SettingsRow v-if="user && balanceLowNotifyEnabled" :title="t('profile.balanceNotify.title')" :description="t('profile.balanceNotify.description')">
          <ProfileBalanceNotifyCard
          :enabled="user.balance_notify_enabled ?? true"
          :threshold="user.balance_notify_threshold"
          :extra-emails="user.balance_notify_extra_emails ?? []"
          :system-default-threshold="systemDefaultThreshold"
            :user-email="user.email"
          />
        </SettingsRow>
        <StatusState v-else-if="settingsLoaded" kind="empty" :title="t('userUi.account.notificationsOff')" class="mt-8" />
      </template>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import SettingsRow from '@/components/user/shell/SettingsRow.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import ProfileBalanceNotifyCard from '@/components/user/profile/ProfileBalanceNotifyCard.vue'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import ProfileIdentityBindingsSection from '@/components/user/profile/ProfileIdentityBindingsSection.vue'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'
import ProfileTotpCard from '@/components/user/profile/ProfileTotpCard.vue'
import ProfilePasskeyCard from '@/components/user/profile/ProfilePasskeyCard.vue'
import { isWeChatWebOAuthEnabled } from '@/api/auth'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

/** 渲染哪一节：由路由 props 传入（/profile、/profile/security、/profile/notifications） */
defineProps<{ section: 'profile' | 'security' | 'notifications' }>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const user = computed(() => authStore.user)

const contactInfo = ref('')
const balanceLowNotifyEnabled = ref(false)
const systemDefaultThreshold = ref(0)
const wechatOAuthEnabled = ref(false)
const wechatOAuthOpenEnabled = ref<boolean | undefined>(undefined)
const wechatOAuthMPEnabled = ref<boolean | undefined>(undefined)
const passkeyEnabled = ref(false)
const settingsLoaded = ref(false)

onMounted(async () => {
  const profileRefresh = authStore.refreshUser().catch((error) => {
    console.error('Failed to refresh profile:', error)
  })

  const settingsLoad = appStore.fetchPublicSettings()
    .then((settings) => {
      if (!settings) {
        return
      }
      contactInfo.value = settings.contact_info || ''
      balanceLowNotifyEnabled.value = settings.balance_low_notify_enabled ?? false
      systemDefaultThreshold.value = settings.balance_low_notify_threshold ?? 0
      wechatOAuthEnabled.value = isWeChatWebOAuthEnabled(settings)
      wechatOAuthOpenEnabled.value = typeof settings.wechat_oauth_open_enabled === 'boolean'
        ? settings.wechat_oauth_open_enabled
        : undefined
      wechatOAuthMPEnabled.value = typeof settings.wechat_oauth_mp_enabled === 'boolean'
        ? settings.wechat_oauth_mp_enabled
        : undefined
      passkeyEnabled.value = settings.passkey_enabled === true
    })
    .catch((error) => {
      console.error('Failed to load settings:', error)
    })
    .finally(() => {
      settingsLoaded.value = true
    })

  await Promise.all([profileRefresh, settingsLoad])
})
</script>
