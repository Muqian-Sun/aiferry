<template>
  <!--
    渠道详情抽屉：点列表行打开。四个页签：概况 / 上架模型 / 用量 / 定时测试。
    列表只放一行名字 + 一行「厂商 · 接入方式」，套餐、隐私、到期、邮箱、协议地址、Compact 都在这里看；
    上游用量窗口和容量（列表放不下）在「用量」页签。测试连接、重新授权等动作仍走原来的对话框（盖在抽屉上面），
    ⋯ 菜单复用列表行的 AccountActionMenu。
  -->
  <DetailDrawer
    :show="account !== null"
    :title="account?.name ?? ''"
    :tabs="tabs"
    :tab="tab"
    width="lg"
    :close-on-escape="!menuOpen"
    @update:tab="emit('update:tab', $event as AccountDetailTab)"
    @close="emit('close')"
  >
    <template v-if="account" #subtitle>
      <span class="inline-flex flex-wrap items-center gap-x-1">
        <PlatformTypeBadge
          variant="plain"
          :platform="account.platform"
          :type="account.type"
          :vendor="account.vendor"
          :auth-mode="getOpenAIAuthMode(account)"
          :plan-type="getAccountPlanType(account)"
          :privacy-mode="privacyMode"
          :subscription-expires-at="subscriptionExpiresAt"
        />
        <template v-if="antigravityTierLabel">
          <span class="text-xs text-af-ink-3" aria-hidden="true">·</span>
          <span class="text-xs text-af-ink-3" data-testid="account-detail-tier">{{ antigravityTierLabel }}</span>
        </template>
      </span>
    </template>

    <template v-if="account" #actions>
      <button type="button" class="btn btn-secondary btn-sm" data-testid="account-detail-test" @click="emit('test', account)">
        <Icon name="play" size="sm" />
        {{ t('admin.accounts.testConnection') }}
      </button>
      <button type="button" class="btn btn-secondary btn-sm" data-testid="account-detail-edit" @click="emit('edit', account)">
        <Icon name="edit" size="sm" />
        {{ t('common.edit') }}
      </button>
      <button
        type="button"
        class="rounded-md p-1.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
        :class="menuOpen ? 'bg-af-sunken text-af-ink' : ''"
        :title="t('common.more')"
        :aria-label="t('common.more')"
        data-testid="account-detail-more"
        @click="emit('open-menu', account, $event)"
      >
        <Icon name="more" size="md" />
      </button>
    </template>

    <template v-if="account && banner" #banner>
      <p
        :class="[
          'rounded-md px-3 py-2 text-13 leading-5',
          banner.tone === 'danger' ? 'bg-af-danger-tint text-af-danger' : 'bg-af-warning-tint text-af-warning'
        ]"
        role="status"
        data-testid="account-detail-banner"
      >
        {{ banner.text }}
      </p>
    </template>

    <template v-if="account">
      <!-- 概况 -->
      <dl v-if="tab === 'overview'" class="divide-y divide-af-hairline" data-testid="account-detail-overview">
        <DetailField :label="t('admin.accounts.columns.status')">
          <AccountStatusIndicator :account="account" detailed @show-temp-unsched="emit('show-temp-unsched', account)" />
        </DetailField>
        <DetailField :label="t('admin.accounts.columns.schedulable')">
          <span class="inline-flex items-center gap-2">
            <MiniSwitch
              :model-value="account.schedulable"
              data-testid="account-detail-schedulable"
              @toggle="emit('toggle-schedulable', account)"
            />
            <span class="text-af-ink-2">
              {{ account.schedulable ? t('admin.accounts.schedulableEnabled') : t('admin.accounts.schedulableDisabled') }}
            </span>
          </span>
        </DetailField>
        <DetailField v-if="protocolRows.length" :label="t('admin.accounts.detail.endpoints')">
          <ul class="space-y-1">
            <li v-for="row in protocolRows" :key="row.protocol" class="flex min-w-0 items-baseline gap-2">
              <span class="w-20 shrink-0 text-xs text-af-ink-3">{{ t(`admin.accounts.protocolShort.${row.protocol}`) }}</span>
              <span class="min-w-0 break-all font-mono text-13 text-af-ink-2">{{ row.url }}</span>
            </li>
          </ul>
        </DetailField>
        <DetailField v-if="email" :label="t('admin.accounts.detail.email')" :value="email" />
        <DetailField v-if="compactText" label="Compact">
          <span :title="compactTitle">{{ compactText }}</span>
        </DetailField>
        <DetailField :label="t('admin.accounts.detail.concurrency')">
          <span class="tabular-nums">{{ account.current_concurrency ?? 0 }} / {{ account.concurrency }}</span>
        </DetailField>
        <DetailField :label="t('admin.accounts.columns.priority')">
          <span class="tabular-nums">{{ account.priority }}</span>
        </DetailField>
        <DetailField :label="t('admin.accounts.columns.billingRateMultiplier')">
          <span class="font-mono tabular-nums">{{ formatMultiplier(account.rate_multiplier ?? 1) }}x</span>
        </DetailField>
        <DetailField :label="t('admin.accounts.columns.proxy')" :value="account.proxy ? account.proxy.name : t('admin.accounts.detail.noProxy')" />
        <DetailField :label="t('admin.accounts.columns.expiresAt')">
          <span :class="isExpired ? 'text-af-warning' : ''">{{ expiresText }}</span>
        </DetailField>
        <DetailField :label="t('admin.accounts.columns.lastUsed')">
          <span :title="account.last_used_at ? formatDateTime(account.last_used_at) : undefined">{{ formatRelativeTime(account.last_used_at) }}</span>
        </DetailField>
        <DetailField :label="t('admin.accounts.columns.createdAt')" :value="formatDateTime(account.created_at)" />
        <DetailField :label="t('admin.accounts.columns.notes')" :value="account.notes" />
      </dl>

      <!-- 上架模型：模型目录里绑定了这个渠道的条目 -->
      <div v-else-if="tab === 'models'" data-testid="account-detail-models">
        <template v-if="catalogEntries.length">
          <p class="mb-3 text-13 text-af-ink-3">{{ t('admin.accounts.detail.modelsHint') }}</p>
          <ul class="divide-y divide-af-hairline border-y border-af-hairline">
            <li v-for="entry in catalogEntries" :key="entry.id" class="flex items-center justify-between gap-4 py-2.5">
              <div class="min-w-0">
                <div :class="['truncate font-mono text-13', entry.status === 'listed' ? 'text-af-ink' : 'text-af-ink-3 line-through']">
                  {{ entry.model_id }}
                </div>
                <div class="truncate text-xs text-af-ink-3">
                  {{ entry.display_name || entry.model_id }}
                  <template v-if="entry.status !== 'listed'"> · {{ t('admin.accounts.catalogUnlisted') }}</template>
                </div>
              </div>
              <button type="button" class="btn btn-ghost btn-sm shrink-0" @click="emit('diagnose', entry)">
                {{ t('admin.accounts.detail.diagnose') }}
              </button>
            </li>
          </ul>
        </template>
        <StatusState v-else kind="empty" :title="t('admin.accounts.catalogNone')" :description="t('admin.accounts.detail.modelsEmptyHint')" />
        <RouterLink to="/model-catalog" class="mt-4 inline-flex text-13 font-medium text-af-brand hover:text-af-brand-hover">
          {{ t('admin.accounts.detail.goToCatalog') }}
        </RouterLink>
      </div>

      <!-- 用量：上游用量窗口 + 容量 + 近 30 天 -->
      <div v-else-if="tab === 'usage'" class="space-y-6" data-testid="account-detail-usage">
        <SheetSection :title="t('admin.accounts.detail.usageWindows')" :description="t('admin.accounts.detail.usageWindowsHint')">
          <AccountUsageCell :account="account" @account-updated="emit('account-updated', $event)" />
        </SheetSection>
        <SheetSection :title="t('admin.accounts.detail.capacity')">
          <AccountCapacityCell :account="account" />
        </SheetSection>
        <SheetSection :title="t('admin.accounts.detail.recentUsage')">
          <AccountUsagePanel :account="account" />
        </SheetSection>
      </div>

      <!-- 定时测试 -->
      <div v-else-if="tab === 'schedule'" data-testid="account-detail-schedule">
        <ScheduledTestsPanel :account-id="account.id" :model-options="scheduleModelOptions" />
      </div>
    </template>
  </DetailDrawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import { DetailDrawer, DetailField, MiniSwitch } from '@/components/admin/list'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab } from '@/components/user/shell/types'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import type { SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import AccountUsageCell from '@/components/account/AccountUsageCell.vue'
import AccountCapacityCell from '@/components/account/AccountCapacityCell.vue'
import { durationUntilWords } from '@/components/account/durationWords'
import { tempUnschedReasonText } from '@/components/account/tempUnschedReason'
import { UPSTREAM_PROTOCOLS } from '@/components/account/protocolEndpoints'
import AccountUsagePanel from './AccountUsagePanel.vue'
import ScheduledTestsPanel from './ScheduledTestsPanel.vue'
import { accountDisplayEmail, antigravityTierKey, getAccountPlanType, getOpenAIAuthMode, openAICompactState } from './accountDisplay'
import type { AccountDetailTab } from './accountDetail'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import { formatMultiplier } from '@/utils/formatters'
import type { Account, AccountListItem, ClaudeModel } from '@/types'

const props = withDefaults(
  defineProps<{
    account: AccountListItem | null
    catalogEntries?: ModelCatalogEntry[]
    tab?: AccountDetailTab
    /** 行尾 ⋯ 菜单开着（它自己处理 Esc） */
    menuOpen?: boolean
  }>(),
  { catalogEntries: () => [], tab: 'overview', menuOpen: false }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:tab', tab: AccountDetailTab): void
  (e: 'edit', account: AccountListItem): void
  (e: 'test', account: AccountListItem): void
  (e: 'toggle-schedulable', account: AccountListItem): void
  (e: 'open-menu', account: AccountListItem, event: MouseEvent): void
  (e: 'diagnose', entry: ModelCatalogEntry): void
  (e: 'show-temp-unsched', account: Account): void
  (e: 'account-updated', account: Account): void
}>()

const { t } = useI18n()

const tabs = computed<SectionTab[]>(() => [
  { key: 'overview', label: t('admin.accounts.detail.tabs.overview') },
  { key: 'models', label: t('admin.accounts.detail.tabs.models'), count: props.catalogEntries.length },
  { key: 'usage', label: t('admin.accounts.detail.tabs.usage') },
  { key: 'schedule', label: t('admin.accounts.detail.tabs.schedule') }
])

const email = computed(() => (props.account ? accountDisplayEmail(props.account) : ''))
const asText = (value: unknown): string | undefined => (typeof value === 'string' && value ? value : undefined)
const privacyMode = computed(() => asText(props.account?.extra?.privacy_mode) ?? asText(props.account?.parent_privacy_mode))
const subscriptionExpiresAt = computed(
  () => asText(props.account?.credentials?.subscription_expires_at) ?? asText(props.account?.parent_subscription_expires_at)
)

const antigravityTierLabel = computed(() => {
  const key = props.account ? antigravityTierKey(props.account) : null
  return key ? t(`admin.accounts.tier.${key}`) : ''
})

// OpenAI 的 Compact 支持情况：自动（未探测）时不写
const compactText = computed(() => {
  const state = props.account ? openAICompactState(props.account) : null
  if (state === 'active') return t('admin.accounts.openai.compactSupported')
  if (state === 'blocked') return t('admin.accounts.openai.compactUnsupported')
  return ''
})
const compactTitle = computed(() => {
  const checkedAt = asText(props.account?.extra?.openai_compact_checked_at)
  return checkedAt ? `${t('admin.accounts.openai.compactLastChecked')}${t('common.labelSeparator')}${formatDateTime(new Date(checkedAt))}` : undefined
})

const protocolRows = computed(() => {
  const endpoints = props.account?.protocol_endpoints
  if (!endpoints) return []
  return UPSTREAM_PROTOCOLS.flatMap((protocol) => {
    const url = endpoints[protocol]?.trim()
    return url ? [{ protocol, url }] : []
  })
})

const isExpired = computed(() => {
  const value = props.account?.expires_at
  return !!value && value * 1000 <= Date.now()
})
const expiresText = computed(() => {
  const value = props.account?.expires_at
  if (!value) return t('admin.accounts.detail.neverExpires')
  const text = formatDateTime(new Date(value * 1000))
  return isExpired.value ? `${text} · ${t('admin.accounts.expired')}` : text
})

const isFuture = (value?: string | null) => !!value && new Date(value).getTime() > Date.now()

// 抽屉顶部一行：只在出问题时出现，直接写原因和恢复时间
const banner = computed<{ tone: 'danger' | 'warning'; text: string } | null>(() => {
  const account = props.account
  if (!account) return null
  if (account.status === 'error') {
    return {
      tone: 'danger',
      text: account.error_message
        ? t('admin.accounts.detail.bannerError', { reason: account.error_message })
        : t('admin.accounts.status.error')
    }
  }
  if (isFuture(account.rate_limit_reset_at)) {
    return { tone: 'warning', text: t('admin.accounts.detail.bannerRateLimited', { time: durationUntilWords(account.rate_limit_reset_at, t) }) }
  }
  if (isFuture(account.overload_until)) {
    return { tone: 'danger', text: t('admin.accounts.detail.bannerOverloaded', { time: durationUntilWords(account.overload_until, t) }) }
  }
  if (isFuture(account.temp_unschedulable_until)) {
    return {
      tone: 'warning',
      text: t('admin.accounts.detail.bannerTempUnsched', {
        time: formatDateTime(account.temp_unschedulable_until),
        reason: tempUnschedReasonText(account.temp_unschedulable_reason) || '—'
      })
    }
  }
  return null
})

// 定时测试要选模型：打开这个页签时按渠道拉一次可用模型
const scheduleModelOptions = ref<SelectOption[]>([])
const scheduleModelsFor = ref<number | null>(null)
watch(
  () => [props.tab, props.account?.id] as const,
  async ([tab, id]) => {
    if (tab !== 'schedule' || !id || scheduleModelsFor.value === id) return
    scheduleModelsFor.value = id
    scheduleModelOptions.value = []
    try {
      const models = await adminAPI.accounts.getAvailableModels(id)
      if (props.account?.id !== id) return
      scheduleModelOptions.value = models.map((m: ClaudeModel) => ({ value: m.id, label: m.display_name || m.id }))
    } catch {
      scheduleModelOptions.value = []
    }
  },
  { immediate: true }
)
</script>
