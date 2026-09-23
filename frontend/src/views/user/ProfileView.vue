<template>
  <!--
    账户：基本信息 / 安全 / 通知 三个子页（muqian 2026-09-23 侧栏「账户」组），路由传 section，这里只渲染对应一节。
    页头标题取路由 meta；内容单列 640，区块之间只用 hairline。
  -->
  <SiteShell>
    <div data-testid="profile-shell" class="max-w-form">
      <template v-if="section === 'profile'">
        <ProfileInfoCard :user="user" :oidc-provider-name="oidcOAuthProviderName" />
        <p v-if="contactInfo" class="mt-6 text-13 text-af-ink-3">
          {{ t('common.contactSupport') }}:
          <span class="font-medium text-af-ink-2">{{ contactInfo }}</span>
        </p>
      </template>

      <template v-else-if="section === 'security'">
        <div class="divide-y divide-af-hairline">
          <div data-testid="profile-auth-bindings-panel" class="pb-6">
            <ProfileIdentityBindingsSection
              :user="user"
              :linuxdo-enabled="linuxdoOAuthEnabled"
              :dingtalk-enabled="dingtalkOAuthEnabled"
              :oidc-enabled="oidcOAuthEnabled"
              :oidc-provider-name="oidcOAuthProviderName"
              :wechat-enabled="wechatOAuthEnabled"
              :wechat-open-enabled="wechatOAuthOpenEnabled"
              :wechat-mp-enabled="wechatOAuthMPEnabled"
              compact
            />
          </div>
          <div class="py-6">
            <ProfilePasswordForm />
          </div>
          <div class="py-6">
            <ProfileTotpCard />
          </div>
          <div class="pt-6">
            <ProfilePasskeyCard :enabled="passkeyEnabled" />
          </div>
        </div>
      </template>

      <template v-else>
        <ProfileBalanceNotifyCard
          v-if="user && balanceLowNotifyEnabled"
          :enabled="user.balance_notify_enabled ?? true"
          :threshold="user.balance_notify_threshold"
          :extra-emails="user.balance_notify_extra_emails ?? []"
          :system-default-threshold="systemDefaultThreshold"
          :user-email="user.email"
        />
        <StatusState v-else-if="settingsLoaded" kind="empty" :title="t('userUi.account.notificationsOff')" />
      </template>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import SiteShell from '@/components/user/shell/SiteShell.vue'
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
const linuxdoOAuthEnabled = ref(false)
const dingtalkOAuthEnabled = ref(false)
const wechatOAuthEnabled = ref(false)
const wechatOAuthOpenEnabled = ref<boolean | undefined>(undefined)
const wechatOAuthMPEnabled = ref<boolean | undefined>(undefined)
const oidcOAuthEnabled = ref(false)
const oidcOAuthProviderName = ref('OIDC')
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
      linuxdoOAuthEnabled.value = settings.linuxdo_oauth_enabled ?? false
      dingtalkOAuthEnabled.value = settings.dingtalk_oauth_enabled ?? false
      wechatOAuthEnabled.value = isWeChatWebOAuthEnabled(settings)
      wechatOAuthOpenEnabled.value = typeof settings.wechat_oauth_open_enabled === 'boolean'
        ? settings.wechat_oauth_open_enabled
        : undefined
      wechatOAuthMPEnabled.value = typeof settings.wechat_oauth_mp_enabled === 'boolean'
        ? settings.wechat_oauth_mp_enabled
        : undefined
      oidcOAuthEnabled.value = settings.oidc_oauth_enabled ?? false
      oidcOAuthProviderName.value = settings.oidc_oauth_provider_name || 'OIDC'
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
