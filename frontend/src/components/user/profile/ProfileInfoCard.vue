<template>
  <!--
    账户「基本信息」：两栏设置式三行（muqian 2026-09-23）——账户概况（身份 + 四格指标）/ 头像 / 用户名。
    行间 hairline；没有卡片、渐变、瓷砖。登录方式绑定在「安全」子页（ProfileView）。
  -->
  <div class="divide-y divide-af-hairline">
    <SettingsRow :title="t('userUi.account.rows.overview')" :description="t('userUi.account.rows.overviewDesc')">
    <section data-testid="profile-overview-hero" class="flex items-start gap-4">
      <div
        class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-full bg-af-sunken text-lg font-semibold text-af-ink-2"
      >
        <img
          v-if="avatarUrl"
          :src="avatarUrl"
          :alt="displayName"
          class="h-full w-full object-cover"
        >
        <span v-else>{{ avatarInitial }}</span>
      </div>

      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-2">
          <h3 class="truncate text-base font-semibold text-af-ink">
            {{ displayName }}
          </h3>
          <span :class="['badge', user?.role === 'admin' ? 'badge-primary' : 'badge-gray']">
            {{ user?.role === 'admin' ? t('profile.administrator') : t('profile.user') }}
          </span>
          <span :class="['badge', user?.status === 'active' ? 'badge-gray' : 'badge-danger']">
            {{ user?.status === 'active' ? t('common.active') : t('common.disabled') }}
          </span>
        </div>
        <p v-if="primaryEmailDisplay && primaryEmailDisplay !== displayName" class="mt-0.5 truncate text-13 text-af-ink-3">
          {{ primaryEmailDisplay }}
        </p>
        <ul v-if="sourceHints.length" class="mt-1 space-y-0.5 text-xs text-af-ink-4">
          <li v-for="hint in sourceHints" :key="hint.key">{{ hint.text }}</li>
        </ul>
      </div>
    </section>

    <!-- 余额 / 计价倍率 / 并发 / 注册时间：倍率决定实付 = 标价 × 倍率，用户该看得到 -->
    <dl class="mt-6 grid grid-cols-2 gap-y-4 border-t border-af-hairline pt-5 sm:grid-cols-4 sm:divide-x sm:divide-af-hairline">
      <div data-testid="profile-overview-metric-balance" class="min-w-0 pr-4">
        <dt class="truncate text-13 text-af-ink-3">{{ t('profile.accountBalance') }}</dt>
        <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">{{ formatCurrency(user?.balance || 0) }}</dd>
      </div>
      <div data-testid="profile-overview-metric-multiplier" class="min-w-0 sm:px-4">
        <dt class="truncate text-13 text-af-ink-3">{{ t('profile.rateMultiplier') }}</dt>
        <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">× {{ rateMultiplierLabel }}</dd>
      </div>
      <div data-testid="profile-overview-metric-concurrency" class="min-w-0 pr-4 sm:px-4">
        <dt class="truncate text-13 text-af-ink-3">{{ t('profile.concurrencyLimit') }}</dt>
        <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">{{ user?.concurrency || 0 }}</dd>
      </div>
      <div data-testid="profile-overview-metric-member-since" class="min-w-0 sm:pl-4">
        <dt class="truncate text-13 text-af-ink-3">{{ t('profile.memberSince') }}</dt>
        <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">{{ memberSinceLabel }}</dd>
      </div>
    </dl>

    </SettingsRow>

    <div data-testid="profile-basics-panel" class="divide-y divide-af-hairline">
      <SettingsRow :title="t('profile.avatar.title')">
        <ProfileAvatarCard :user="user" />
      </SettingsRow>
      <SettingsRow :title="t('profile.username')" :description="t('userUi.account.rows.usernameDesc')">
        <ProfileEditForm :initial-username="user?.username || ''" />
      </SettingsRow>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { formatCurrency } from '@/utils/format'
import { useI18n } from 'vue-i18n'
import SettingsRow from '@/components/user/shell/SettingsRow.vue'
import ProfileAvatarCard from '@/components/user/profile/ProfileAvatarCard.vue'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import type { User, UserAuthBindingStatus, UserAuthProvider, UserProfileSourceContext } from '@/types'
import { formatMultiplier } from '@/utils/formatters'

const props = defineProps<{
  user: User | null
}>()

const { t } = useI18n()

function normalizeBindingStatus(binding: boolean | UserAuthBindingStatus | undefined): boolean | null {
  if (typeof binding === 'boolean') {
    return binding
  }
  if (!binding) {
    return null
  }
  if (typeof binding.bound === 'boolean') {
    return binding.bound
  }
  return Boolean(binding.provider_subject || binding.issuer || binding.provider_key)
}

function isEmailBound(user: User | null | undefined): boolean {
  if (typeof user?.email_bound === 'boolean') {
    return user.email_bound
  }

  const nested = user?.auth_bindings?.email ?? user?.identity_bindings?.email
  const normalized = normalizeBindingStatus(nested)
  return normalized ?? false
}

const avatarUrl = computed(() => props.user?.avatar_url?.trim() || '')
const displayName = computed(() => props.user?.username?.trim() || props.user?.email?.trim() || t('profile.user'))
const primaryEmailDisplay = computed(() => {
  const email = props.user?.email?.trim() || ''
  if (!email) {
    return ''
  }
  if (email.endsWith('.invalid') && !isEmailBound(props.user)) {
    return ''
  }
  return email
})
const avatarInitial = computed(() => displayName.value.charAt(0).toUpperCase() || 'U')
// 倍率缺省按 1（接口没给时不显示 0——0 是「免费」的意思）
const rateMultiplierLabel = computed(() => {
  const value = props.user?.rate_multiplier
  return formatMultiplier(Number(value ?? 1))
})

const memberSinceLabel = computed(() => {
  const raw = props.user?.created_at?.trim()
  if (!raw) {
    return '-'
  }

  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }

  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
  }).format(date)
})

const providerLabels = computed<Record<UserAuthProvider, string>>(() => ({
  email: t('profile.authBindings.providers.email'),
  wechat: t('profile.authBindings.providers.wechat'),
  github: 'GitHub',
  google: 'Google'
}))

function normalizeProvider(value: string): UserAuthProvider | null {
  const normalized = value.trim().toLowerCase()
  if (
    normalized === 'email' ||
    normalized === 'wechat' ||
    normalized === 'github' ||
    normalized === 'google'
  ) {
    return normalized
  }
  return null
}

function readObjectString(source: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = source[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}

function resolveThirdPartySource(
  rawSource: string | UserProfileSourceContext | null | undefined
): { provider: UserAuthProvider; label: string } | null {
  if (!rawSource) {
    return null
  }

  if (typeof rawSource === 'string') {
    const provider = normalizeProvider(rawSource)
    if (!provider || provider === 'email') {
      return null
    }
    return {
      provider,
      label: providerLabels.value[provider]
    }
  }

  const sourceRecord = rawSource as Record<string, unknown>
  const provider = normalizeProvider(
    readObjectString(sourceRecord, 'provider', 'source', 'provider_type', 'auth_provider')
  )
  if (!provider || provider === 'email') {
    return null
  }

  const explicitLabel = readObjectString(
    sourceRecord,
    'provider_label',
    'label',
    'provider_name',
    'providerName'
  )

  return {
    provider,
    label: explicitLabel || providerLabels.value[provider]
  }
}

const sourceHints = computed(() => {
  const currentUser = props.user
  if (!currentUser) {
    return []
  }

  const hints: Array<{ key: string; text: string }> = []
  const avatarSource = resolveThirdPartySource(
    currentUser.profile_sources?.avatar ?? currentUser.avatar_source
  )
  const usernameSource = resolveThirdPartySource(
    currentUser.profile_sources?.username ??
      currentUser.profile_sources?.display_name ??
      currentUser.profile_sources?.nickname ??
      currentUser.display_name_source ??
      currentUser.username_source ??
      currentUser.nickname_source
  )

  if (avatarSource) {
    hints.push({
      key: 'avatar',
      text: t('profile.authBindings.source.avatar', { providerName: avatarSource.label })
    })
  }

  if (usernameSource) {
    hints.push({
      key: 'username',
      text: t('profile.authBindings.source.username', { providerName: usernameSource.label })
    })
  }

  return hints
})
</script>
