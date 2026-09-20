<template>
  <!--
    账户「基本信息」：身份行（头像 / 名称 / 角色状态 / 邮箱 / 资料来源）→ 三格指标 → 头像上传 → 昵称。
    单列 640，区块之间只用 hairline；没有卡片、渐变、瓷砖。登录方式绑定放在「安全」节（ProfileView）。
  -->
  <div class="space-y-6">
    <section data-testid="profile-overview-hero" class="flex items-start gap-4">
      <div
        class="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-md bg-af-brand-tint text-lg font-semibold text-af-brand"
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
          <span :class="['badge', user?.status === 'active' ? 'badge-success' : 'badge-danger']">
            {{ user?.status === 'active' ? t('common.active') : t('common.disabled') }}
          </span>
        </div>
        <p v-if="primaryEmailDisplay" class="mt-0.5 truncate text-13 text-af-ink-3">
          {{ primaryEmailDisplay }}
        </p>
        <ul v-if="sourceHints.length" class="mt-1 space-y-0.5 text-xs text-af-ink-4">
          <li v-for="hint in sourceHints" :key="hint.key">{{ hint.text }}</li>
        </ul>
      </div>
    </section>

    <dl class="grid grid-cols-3 divide-x divide-af-hairline border-y border-af-hairline py-4">
      <div data-testid="profile-overview-metric-balance" class="min-w-0 pr-4">
        <dt class="truncate text-13 text-af-ink-3">{{ t('profile.accountBalance') }}</dt>
        <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">{{ formatCurrency(user?.balance || 0) }}</dd>
      </div>
      <div data-testid="profile-overview-metric-concurrency" class="min-w-0 px-4">
        <dt class="truncate text-13 text-af-ink-3">{{ t('profile.concurrencyLimit') }}</dt>
        <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">{{ user?.concurrency || 0 }}</dd>
      </div>
      <div data-testid="profile-overview-metric-member-since" class="min-w-0 pl-4">
        <dt class="truncate text-13 text-af-ink-3">{{ t('profile.memberSince') }}</dt>
        <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">{{ memberSinceLabel }}</dd>
      </div>
    </dl>

    <div data-testid="profile-basics-panel" class="divide-y divide-af-hairline">
      <div class="pb-6">
        <ProfileAvatarCard :user="user" />
      </div>
      <div class="pt-6">
        <ProfileEditForm :initial-username="user?.username || ''" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ProfileAvatarCard from '@/components/user/profile/ProfileAvatarCard.vue'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import type { User, UserAuthBindingStatus, UserAuthProvider, UserProfileSourceContext } from '@/types'

const props = withDefaults(defineProps<{
  user: User | null
  /** OIDC 提供方名，用于「资料来源」提示文案 */
  oidcProviderName?: string
}>(), {
  oidcProviderName: 'OIDC',
})

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
  linuxdo: t('profile.authBindings.providers.linuxdo'),
  dingtalk: t('profile.authBindings.providers.dingtalk'),
  oidc: t('profile.authBindings.providers.oidc', { providerName: props.oidcProviderName }),
  wechat: t('profile.authBindings.providers.wechat'),
  github: 'GitHub',
  google: 'Google'
}))

function formatCurrency(value: number): string {
  return `$${value.toFixed(2)}`
}

function normalizeProvider(value: string): UserAuthProvider | null {
  const normalized = value.trim().toLowerCase()
  if (
    normalized === 'email' ||
    normalized === 'linuxdo' ||
    normalized === 'wechat' ||
    normalized === 'github' ||
    normalized === 'google'
  ) {
    return normalized
  }
  if (normalized === 'oidc' || normalized.startsWith('oidc:') || normalized.startsWith('oidc/')) {
    return 'oidc'
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
