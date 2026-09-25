<template>
  <!-- plain：一行小字「厂商 · 类型 · 套餐 · 隐私 · 到期」，给渠道列表名称下面那行用（A5）；只有异常才上色 -->
  <span v-if="variant === 'plain'" class="inline-flex min-w-0 flex-wrap items-center gap-x-1 text-xs text-af-ink-3" data-testid="platform-plain">
    <PlatformIcon :platform="displayPlatform" size="xs" class="shrink-0" />
    <span :title="platformTitle" data-testid="platform-badge">{{ plainPlatformLabel }}</span>
    <span aria-hidden="true">·</span>
    <span>{{ plainTypeLabel }}</span>
    <template v-if="planLabel">
      <span aria-hidden="true">·</span>
      <span :class="normalizedPlanType === 'abnormal' ? 'text-af-danger' : ''">{{ planLabel }}</span>
    </template>
    <template v-if="privacyBadge">
      <span aria-hidden="true">·</span>
      <span :class="privacyBadge.plainClass" :title="privacyBadge.title">{{ privacyBadge.plainLabel }}</span>
    </template>
    <template v-if="expiresLabel">
      <span aria-hidden="true">·</span>
      <span :title="subscriptionExpiresAt">{{ expiresLabel }}</span>
    </template>
  </span>
  <div v-else class="inline-flex flex-col gap-0.5 text-xs font-medium">
    <!-- Row 1: Platform + Type -->
    <div class="inline-flex items-center overflow-hidden rounded-md">
      <span :class="['inline-flex items-center gap-1 px-2 py-1', platformClass]" :title="platformTitle" data-testid="platform-badge">
        <PlatformIcon :platform="displayPlatform" size="xs" />
        <span>{{ platformLabel }}</span>
      </span>
      <span :class="['inline-flex items-center gap-1 px-1.5 py-1', typeClass]">
        <!-- OAuth icon -->
        <svg
          v-if="type === 'oauth'"
          class="h-3 w-3"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
          />
        </svg>
        <!-- Setup Token icon -->
        <Icon v-else-if="type === 'setup-token'" name="shield" size="xs" />
        <!-- API Key icon -->
        <Icon v-else-if="type === 'service_account'" name="cloud" size="xs" />
        <Icon v-else name="key" size="xs" />
        <span>{{ typeLabel }}</span>
      </span>
    </div>
    <!-- Row 2: Plan type + Privacy mode (only if either exists) -->
    <div v-if="planLabel || privacyBadge" class="inline-flex items-center overflow-hidden rounded-md">
      <span v-if="planLabel" :class="['inline-flex items-center gap-1 px-1.5 py-1', planBadgeClass]">
        <GrokFreeIcon
          v-if="isGrokFreePlan"
          data-testid="grok-free-plan-icon"
        />
        <Icon
          v-else-if="planIconName"
          :name="planIconName"
          size="xs"
          data-testid="grok-plan-icon"
          aria-hidden="true"
        />
        <span>{{ planLabel }}</span>
      </span>
      <span
        v-if="privacyBadge"
        :class="['inline-flex items-center gap-1 px-1.5 py-1', privacyBadge.class]"
        :title="privacyBadge.title"
      >
        <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" :d="privacyBadge.icon" />
        </svg>
        <span>{{ privacyBadge.label }}</span>
      </span>
    </div>
    <!-- Row 3: Subscription expiration (non-free paid accounts only) -->
    <div v-if="expiresLabel" class="text-[10px] leading-tight text-af-ink-3 pl-0.5" :title="subscriptionExpiresAt">
      {{ expiresLabel }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountPlatform, AccountType } from '@/types'
import { RELAY_PLATFORM, platformLabel as sharedPlatformLabel } from '@/utils/platformLabel'
import { normalizePlanType, openAIPlanTypeLabel } from '@/utils/planType'
import { accountAccessKey } from '@/components/admin/account/accountDisplay'
import GrokFreeIcon from './GrokFreeIcon.vue'
import PlatformIcon from './PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

interface Props {
  platform: AccountPlatform
  type: AccountType
  /**
   * 按上游地址识别出的官方厂商（后端 vendor 字段）。第三方 key 的 platform 只是展示标签，
   * 徽章按 vendor 显示；没识别出厂商的 key 显示为「中转」。成品号忽略此项。
   */
  vendor?: string | null
  authMode?: string
  planType?: string
  privacyMode?: string
  subscriptionExpiresAt?: string
  /** badge：原来的灰底分段徽章；plain：一行小字 */
  variant?: 'badge' | 'plain'
}

const props = withDefaults(defineProps<Props>(), { variant: 'badge' })

const isThirdPartyKey = computed(() => props.type === 'apikey')

// 徽章展示的平台：成品号看 platform；第三方 key 看按地址识别的厂商，未识别即中转。
const displayPlatform = computed<AccountPlatform | typeof RELAY_PLATFORM>(() => {
  if (!isThirdPartyKey.value) return props.platform
  return (props.vendor as AccountPlatform | undefined) || RELAY_PLATFORM
})

const platformLabel = computed(() => sharedPlatformLabel(displayPlatform.value))
// plain 形态写字、不写缩写，和渠道列表名称行（accountDisplay）同一套叫法：中转 / 第三方 key / OAuth 授权……
const plainPlatformLabel = computed(() =>
  displayPlatform.value === RELAY_PLATFORM ? t('admin.accounts.vendorRelay') : platformLabel.value
)
const plainTypeLabel = computed(() =>
  t(accountAccessKey({ platform: props.platform, type: props.type, credentials: { auth_mode: props.authMode ?? '' } }))
)

const platformTitle = computed(() => {
  if (!isThirdPartyKey.value) return undefined
  const label = sharedPlatformLabel(props.platform)
  return props.vendor
    ? `${sharedPlatformLabel(props.vendor)} (official) · label: ${label}`
    : `Relay / aggregator (no official vendor identified) · label: ${label}`
})

const normalizedAuthMode = computed(() =>
  (props.authMode || '').trim().toLowerCase().replace(/[\s_-]+/g, '')
)

const typeLabel = computed(() => {
  if (props.platform === 'openai' && props.type === 'oauth') {
    if (normalizedAuthMode.value === 'agentidentity') return 'Agent Identity'
    if (normalizedAuthMode.value === 'personalaccesstoken') return 'PAT'
  }
  switch (props.type) {
    case 'oauth':
      return 'OAuth'
    case 'setup-token':
      return 'Token'
    case 'apikey':
      return 'Key'
    case 'bedrock':
      return 'AWS'
    case 'service_account':
      return 'Vertex'
    default:
      return props.type
  }
})

const normalizedPlanType = computed(() => normalizePlanType(props.planType))

const planLabel = computed(() => {
  if (!normalizedPlanType.value) return ''
  // ChatGPT 档位命名（Pro 5x / Pro 20x、Business Standard / Business Premium）只适用于
  // OpenAI：Antigravity 与 Grok 各自的 pro/team 沿用下面的通用标签。
  if (props.platform === 'openai') {
    const label = openAIPlanTypeLabel(props.planType)
    if (label) return label
  }
  switch (normalizedPlanType.value) {
    case 'plus':
      return 'Plus'
    case 'team':
      return 'Team'
    case 'chatgptpro':
    case 'pro':
      return 'Pro'
    case 'free':
    case 'basic':
      return props.platform === 'grok' ? 'Grok Free' : 'Free'
    case 'supergrok':
      return 'SuperGrok'
    case 'supergroklite':
      return 'SuperGrok Lite'
    case 'supergrokplus':
      return 'SuperGrok Plus'
    case 'supergrokheavy':
      return 'SuperGrok Heavy'
    case 'heavy':
      return 'Heavy'
    case 'xbasic':
      return 'X Basic'
    case 'abnormal':
      return t('admin.accounts.subscriptionAbnormal')
    default:
      return props.planType
  }
})

const isGrokFreePlan = computed(() =>
  props.platform === 'grok' &&
  (normalizedPlanType.value === 'free' ||
    normalizedPlanType.value === 'basic' ||
    normalizedPlanType.value === 'xbasic')
)

const planIconName = computed<'bolt' | null>(() => {
  if (props.platform !== 'grok') return null
  // Paid Grok tiers (SuperGrok / Heavy) share the bolt mark; free uses GrokFreeIcon.
  if (
    normalizedPlanType.value === 'supergrok' ||
    normalizedPlanType.value === 'supergrokheavy' ||
    normalizedPlanType.value === 'heavy' ||
    normalizedPlanType.value.includes('heavy')
  ) {
    return 'bolt'
  }
  return null
})

/**
 * 平台 / 认证类型 / 套餐标签一律灰底墨字（muqian 2026-09-24：管理站装饰色收成墨色）——
 * 平台靠图标与名称区分，不靠颜色；只有异常套餐标红。
 */
const NEUTRAL_BADGE = 'bg-af-sunken text-af-ink-2'
const platformClass = NEUTRAL_BADGE
const typeClass = NEUTRAL_BADGE
const planBadgeClass = computed(() =>
  normalizedPlanType.value === 'abnormal' ? 'bg-af-danger-tint text-af-danger' : NEUTRAL_BADGE
)

// Subscription expiration label (non-free only)
const expiresLabel = computed(() => {
  if (!props.subscriptionExpiresAt || !props.planType) return ''
  if (
    normalizedPlanType.value === 'free' ||
    normalizedPlanType.value === 'basic' ||
    normalizedPlanType.value === 'xbasic'
  ) return ''
  try {
    const d = new Date(props.subscriptionExpiresAt)
    if (isNaN(d.getTime())) return ''
    const yyyy = d.getFullYear()
    const mm = String(d.getMonth() + 1).padStart(2, '0')
    const dd = String(d.getDate()).padStart(2, '0')
    return `${t('admin.accounts.subscriptionExpires')} ${yyyy}-${mm}-${dd}`
  } catch {
    return ''
  }
})

// Privacy badge — shows different states for OpenAI/Antigravity OAuth privacy setting
const privacyBadge = computed(() => {
  if (props.type !== 'oauth' || !props.privacyMode) return null
  // 支持 OpenAI 和 Antigravity 平台
  if (props.platform !== 'openai' && props.platform !== 'antigravity') return null

  const shieldCheck = 'M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z'
  const shieldX = 'M12 9v3.75m0-10.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285zM12 18h.008v.008H12V18z'
  switch (props.privacyMode) {
    // OpenAI states
    case 'training_off':
      return { label: 'Private', plainLabel: t('admin.accounts.privacyOptions.trainingOff'), icon: shieldCheck, title: t('admin.accounts.privacyTrainingOff'), class: 'bg-af-success-tint text-af-success', plainClass: '' }
    case 'training_set_cf_blocked':
      return { label: 'CF', plainLabel: t('admin.accounts.privacyOptions.cfBlocked'), icon: shieldX, title: t('admin.accounts.privacyCfBlocked'), class: 'bg-af-warning-tint text-af-warning', plainClass: 'text-af-warning' }
    case 'training_set_failed':
      return { label: 'Fail', plainLabel: t('admin.accounts.privacyOptions.failed'), icon: shieldX, title: t('admin.accounts.privacyFailed'), class: 'bg-af-danger-tint text-af-danger', plainClass: 'text-af-danger' }
    // Antigravity states
    case 'privacy_set':
      return { label: 'Private', plainLabel: t('admin.accounts.privacyAntigravitySet'), icon: shieldCheck, title: t('admin.accounts.privacyAntigravitySet'), class: 'bg-af-success-tint text-af-success', plainClass: '' }
    case 'privacy_set_failed':
      return { label: 'Fail', plainLabel: t('admin.accounts.privacyAntigravityFailed'), icon: shieldX, title: t('admin.accounts.privacyAntigravityFailed'), class: 'bg-af-danger-tint text-af-danger', plainClass: 'text-af-danger' }
    default:
      return null
  }
})
</script>
