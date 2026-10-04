<template>
  <!--
    渠道的上游用量窗口：只在详情抽屉「用量」页签里出现（列表放不下，方案 2026-09-25 挪进抽屉）。
    按平台各有各的窗口，窗口名都写全（5 小时 / 7 天 / Gemini 3 Pro / 每日额度…）；本站在窗口里的用量写成
    「请求 · Token · 收入 · 成本」一行小字。今日用量不在这里重复——抽屉下面「近 30 天」里有。
  -->
  <div class="space-y-1.5" data-testid="account-usage-cell">
    <!-- Anthropic OAuth / Setup Token：主动 / 被动采样的 5 小时、7 天窗口 -->
    <template v-if="isAnthropicOAuthOrSetupToken">
      <div v-if="loading" class="space-y-1.5">
        <div v-for="n in account.type === 'oauth' ? 3 : 1" :key="n" class="h-3 w-56 animate-pulse rounded bg-af-hairline"></div>
      </div>
      <p v-else-if="error" class="text-xs text-af-danger">{{ error }}</p>
      <div v-else-if="usageInfo" class="space-y-1.5">
        <p v-if="usageInfo.error" class="truncate text-xs text-af-warning" :title="usageInfo.error">{{ usageInfo.error }}</p>
        <UsageProgressBar
          v-if="usageInfo.five_hour"
          :label="t('admin.accounts.usageWindow.fiveHour')"
          :utilization="usageInfo.five_hour.utilization"
          :resets-at="usageInfo.five_hour.resets_at"
          :window-stats="usageInfo.five_hour.window_stats"
        />
        <UsageProgressBar
          v-if="usageInfo.seven_day"
          :label="t('admin.accounts.usageWindow.sevenDay')"
          :utilization="usageInfo.seven_day.utilization"
          :resets-at="usageInfo.seven_day.resets_at"
        />
        <UsageProgressBar
          v-if="usageInfo.seven_day_sonnet"
          :label="t('admin.accounts.usageWindow.sevenDaySonnet')"
          :utilization="usageInfo.seven_day_sonnet.utilization"
          :resets-at="usageInfo.seven_day_sonnet.resets_at"
        />
        <UsageProgressBar
          v-if="usageInfo.seven_day_fable"
          :label="t('admin.accounts.usageWindow.sevenDayFable')"
          :utilization="usageInfo.seven_day_fable.utilization"
          :resets-at="usageInfo.seven_day_fable.resets_at"
        />
        <div class="flex items-center gap-2 text-xs text-af-ink-3">
          <span v-if="usageInfo.source === 'passive'">{{ t('admin.accounts.usageWindow.passiveSampled') }}</span>
          <button
            type="button"
            class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 font-medium text-af-ink-2 transition-colors hover:bg-af-sunken disabled:opacity-50"
            :disabled="activeQueryLoading"
            @click="loadActiveUsage"
          >
            <Icon name="refresh" size="xs" :class="{ 'animate-spin': activeQueryLoading }" />
            {{ t('admin.accounts.usageWindow.activeQuery') }}
          </button>
          <span v-if="activeQueryError" class="text-xs text-af-danger" :title="activeQueryError" role="alert">{{ t('admin.accounts.usageWindow.queryFailed') }}</span>
        </div>
      </div>
      <p v-else class="text-xs text-af-ink-3">{{ t('admin.accounts.usageWindow.noData') }}</p>
    </template>

    <!-- OpenAI OAuth：/usage 的 5 小时、7 天窗口 + 上游重置次数 -->
    <template v-else-if="account.platform === 'openai' && account.type === 'oauth'">
      <div v-if="hasOpenAIUsage" class="space-y-1.5">
        <UsageProgressBar
          v-if="usageInfo?.five_hour"
          :label="t('admin.accounts.usageWindow.fiveHour')"
          :utilization="usageInfo.five_hour.utilization"
          :resets-at="usageInfo.five_hour.resets_at"
          :window-stats="usageInfo.five_hour.window_stats"
          :show-now-when-idle="true"
        />
        <UsageProgressBar
          v-if="usageInfo?.seven_day"
          :label="t('admin.accounts.usageWindow.sevenDay')"
          :utilization="usageInfo.seven_day.utilization"
          :resets-at="usageInfo.seven_day.resets_at"
          :window-stats="usageInfo.seven_day.window_stats"
          :estimated-total-cost="openAISevenDayEstimatedTotalCost"
          :show-now-when-idle="true"
        />
        <!-- 本地主动采样的「查询」和上游重置次数放在同一排按钮里 -->
        <OpenAIQuotaResetCell :account="account" @account-updated="handleQuotaResetAccountUpdated">
          <template #pre-actions>
            <button
              type="button"
              class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium text-af-ink-2 transition-colors hover:bg-af-sunken disabled:cursor-not-allowed disabled:opacity-50"
              :disabled="activeQueryLoading"
              @click="loadActiveUsage"
            >
              <Icon name="refresh" size="xs" :class="{ 'animate-spin': activeQueryLoading }" />
              {{ t('admin.accounts.usageWindow.activeQuery') }}
            </button>
            <span v-if="activeQueryError" class="text-xs text-af-danger" :title="activeQueryError" role="alert">{{ t('admin.accounts.usageWindow.queryFailed') }}</span>
          </template>
        </OpenAIQuotaResetCell>
      </div>
      <div v-else-if="loading" class="space-y-1.5">
        <div v-for="n in 2" :key="n" class="h-3 w-56 animate-pulse rounded bg-af-hairline"></div>
      </div>
      <div v-else class="space-y-1">
        <p class="text-xs text-af-ink-3">{{ t('admin.accounts.usageWindow.noData') }}</p>
        <!-- 本地还没有数据时也能直接查上游 -->
        <OpenAIQuotaResetCell :account="account" @account-updated="handleQuotaResetAccountUpdated" />
      </div>
    </template>

    <!-- Antigravity OAuth：按模型的配额 -->
    <template v-else-if="account.platform === 'antigravity' && account.type === 'oauth'">
      <p v-if="hasIneligibleTiers" class="text-xs text-af-danger">{{ t('admin.accounts.ineligibleWarning') }}</p>
      <div v-if="isForbidden" class="space-y-1">
        <p :class="['text-xs font-medium', forbiddenType === 'validation' ? 'text-af-warning' : 'text-af-danger']">{{ forbiddenLabel }}</p>
        <div v-if="validationURL" class="flex items-center gap-2 text-xs">
          <a
            :href="validationURL"
            target="_blank"
            rel="noopener noreferrer"
            class="text-af-ink-2 hover:text-af-ink hover:underline"
          >
            {{ t('admin.accounts.openVerification') }}
          </a>
          <button type="button" class="text-af-ink-3 hover:text-af-ink-2" @click="copyValidationURL">
            {{ linkCopied ? t('admin.accounts.linkCopied') : t('admin.accounts.copyLink') }}
          </button>
        </div>
      </div>
      <p v-else-if="needsReauth" class="text-xs font-medium text-af-warning">{{ t('admin.accounts.needsReauth') }}</p>
      <p v-else-if="usageInfo?.error" class="text-xs font-medium text-af-warning">{{ usageErrorLabel }}</p>
      <div v-else-if="loading" class="h-3 w-56 animate-pulse rounded bg-af-hairline"></div>
      <p v-else-if="error" class="text-xs text-af-danger">{{ error }}</p>
      <div v-else-if="hasAntigravityQuota" class="space-y-1.5">
        <UsageProgressBar
          v-for="bar in antigravityBars"
          :key="bar.key"
          :label="bar.label"
          :utilization="bar.utilization"
          :resets-at="bar.resetTime"
        />
        <p v-if="aiCreditsDisplay" class="text-xs text-af-ink-3">
          {{ t('admin.accounts.aiCreditsBalance') }}{{ t('common.labelSeparator') }}{{ aiCreditsDisplay }}
        </p>
      </div>
      <p v-else-if="aiCreditsDisplay" class="text-xs text-af-ink-3">
        {{ t('admin.accounts.aiCreditsBalance') }}{{ t('common.labelSeparator') }}{{ aiCreditsDisplay }}
      </p>
      <p v-else class="text-xs text-af-ink-3">{{ t('admin.accounts.usageWindow.noData') }}</p>
    </template>

    <!-- Grok OAuth：免费档看滚动 24 小时 Token；付费档看 7 天 / 30 天 + 预付余额 -->
    <template v-else-if="account.platform === 'grok' && account.type === 'oauth'">
      <div v-if="loading" class="h-3 w-56 animate-pulse rounded bg-af-hairline"></div>
      <p v-else-if="error" class="text-xs text-af-danger">{{ error }}</p>
      <p v-else-if="needsReauth" class="text-xs font-medium text-af-warning">{{ t('admin.accounts.needsReauth') }}</p>
      <p v-else-if="isForbidden" class="text-xs font-medium text-af-danger">
        {{ usageInfo?.grok_entitlement_status || t('admin.accounts.forbidden') }}
      </p>
      <div v-else-if="usageInfo" class="space-y-1.5">
        <template v-if="grokIsFree">
          <UsageProgressBar
            v-if="grokFreeTokenBar"
            :label="t('admin.accounts.usageWindow.twentyFourHours')"
            :title="t('admin.accounts.usageWindow.grokFreeQuota24hHint', { limit: formatCompactNumber(grokFreeTokenBar.limit) })"
            :utilization="grokFreeTokenBar.utilization"
            :window-stats="grokFreeQuotaUsage"
            :show-now-when-idle="true"
          />
          <p v-else-if="grokQuotaUnknown" class="text-xs text-af-ink-3">{{ grokQuotaUnknownLabel }}</p>
        </template>
        <template v-else>
          <UsageProgressBar
            v-if="grokWeeklyBillingBar"
            :label="t('admin.accounts.usageWindow.sevenDay')"
            :utilization="grokWeeklyBillingBar.utilization"
            :resets-at="grokWeeklyBillingBar.resetsAt"
            :window-stats="grokWeeklyBillingBar.windowStats"
            :show-now-when-idle="true"
          />
          <UsageProgressBar
            v-if="grokMonthlyBillingBar"
            :label="t('admin.accounts.usageWindow.thirtyDays')"
            :utilization="grokMonthlyBillingBar.utilization"
            :resets-at="grokMonthlyBillingBar.resetsAt"
            :window-stats="grokMonthlyBillingBar.windowStats"
            :show-now-when-idle="true"
          />
          <p v-if="grokPrepaidMoneyLine" class="flex flex-wrap items-center gap-x-2 text-xs text-af-ink-3" data-testid="grok-money-line">
            <span v-if="grokPrepaidMoneyLine.prepaid !== null">
              {{ t('admin.accounts.usageWindow.grokPrepaid') }} {{ grokPrepaidMoneyLine.prepaid }}
            </span>
            <span v-if="grokPrepaidMoneyLine.used !== null" :title="t('admin.accounts.usageWindow.grokMonthlyLimit')">
              {{ t('admin.accounts.usageWindow.grokUsed') }} {{ grokPrepaidMoneyLine.used }} / {{ grokPrepaidMoneyLine.limit }}
            </span>
          </p>
          <p v-if="grokQuotaUnknown" class="text-xs text-af-ink-3">{{ grokQuotaUnknownLabel }}</p>
        </template>
        <p v-if="usageInfo.error" class="truncate text-xs text-af-warning" :title="usageInfo.error">{{ usageErrorLabel }}</p>
        <p v-if="grokRetryAfterLabel" class="text-xs text-af-warning">
          {{ t('admin.accounts.usageWindow.grokRetryAfter', { time: grokRetryAfterLabel }) }}
        </p>
        <GrokQuotaProbeCell :account="account" compact @probed="handleGrokProbed" />
      </div>
      <div v-else class="space-y-1">
        <p class="text-xs text-af-ink-3">{{ t('admin.accounts.usageWindow.noData') }}</p>
        <GrokQuotaProbeCell :account="account" compact @probed="handleGrokProbed" />
      </div>
    </template>

    <!-- 国产平台（Kimi / 智谱 / DeepSeek / MiniMax / OpenCode）：Coding Plan 额度或按量余额 -->
    <template v-else-if="isCNProvider">
      <!-- 挂在国产平台下的 Ollama Cloud 渠道（后端下发 eligible）：用量走 Ollama 自己的窗口；
           国产平台的额度 / 余额端点由 base_url 衍生，对 ollama.com 会被出站白名单拒绝，所以不渲染。 -->
      <OllamaCloudUsageCell
        v-if="account.ollama_cloud_usage?.eligible"
        :account="account"
        @updated="handleOllamaCloudUsageUpdated"
      />
      <template v-else>
        <!-- 两个子格按「计费方式 × 平台」各自判定；都不可见时（智谱按量没有公开余额端点）写一句说明 -->
        <p v-if="!cnQuotaCellVisible && !cnBalanceCellVisible" class="text-xs text-af-ink-3">
          {{ t('admin.accounts.cnProviders.noBalanceEndpoint') }}
        </p>
        <CNProviderQuotaCell :account="account" />
        <CNProviderBalanceCell :account="account" />
      </template>
    </template>

    <!-- Gemini：授权通道与等级 + 本地模拟的每日配额 -->
    <template v-else-if="isGeminiSubscription">
      <div v-if="geminiAuthTypeLabel" class="space-y-0.5 text-xs text-af-ink-3" data-testid="gemini-quota-policy">
        <p class="font-medium text-af-ink-2">{{ geminiAuthTypeLabel }}</p>
        <p>
          {{ geminiQuotaPolicyLimits }}
          <a :href="geminiQuotaPolicyDocsUrl" target="_blank" rel="noopener noreferrer" class="ml-1 text-af-ink-2 underline hover:text-af-ink">
            {{ t('admin.accounts.gemini.quotaPolicy.columns.docs') }}
          </a>
        </p>
      </div>
      <div v-if="loading" class="h-3 w-56 animate-pulse rounded bg-af-hairline"></div>
      <p v-else-if="error" class="text-xs text-af-danger">{{ error }}</p>
      <div v-else-if="geminiUsageBars.length" class="space-y-1.5">
        <UsageProgressBar
          v-for="bar in geminiUsageBars"
          :key="bar.key"
          :label="bar.label"
          :utilization="bar.utilization"
          :resets-at="bar.resetsAt"
          :window-stats="bar.windowStats"
        />
        <p class="text-xs text-af-ink-3">{{ t('admin.accounts.gemini.quotaPolicy.simulatedNote') }}</p>
      </div>
      <!-- AI Studio 客户端 OAuth 没有用量追踪 -->
      <p v-else class="text-xs text-af-ink-3">{{ t('admin.accounts.gemini.rateLimit.unlimited') }}</p>
    </template>

    <!-- 其余第三方 key / Bedrock：只有下面的额度进度条；没设额度时写一句 -->
    <template v-else-if="isQuotaEligible">
      <OllamaCloudUsageCell
        v-if="account.ollama_cloud_usage?.eligible"
        :account="account"
        @updated="handleOllamaCloudUsageUpdated"
      />
      <p v-if="!quotaBars.length && !account.ollama_cloud_usage?.eligible" class="text-xs text-af-ink-3">
        {{ t('admin.accounts.usageWindow.noQuota') }}
      </p>
    </template>

    <p v-else class="text-xs text-af-ink-3">{{ t('admin.accounts.usageWindow.none') }}</p>

    <!-- 第三方 key / Bedrock 设了配额时的日 / 周 / 总额度（成本口径）。国产平台的 key 也能设额度，
         所以不放进上面按平台分的分支里（容量里已不再重复画额度，这里是唯一一处） -->
    <UsageProgressBar
      v-for="bar in quotaBars"
      :key="bar.key"
      :label="bar.label"
      :utilization="bar.utilization"
      :resets-at="bar.resetsAt"
      :note="bar.note"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Account, AccountUsageInfo, GeminiCredentials, WindowStats } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import { buildOpenAIUsageRefreshKey } from '@/utils/accountUsageRefresh'
import { enqueueUsageRequest } from '@/utils/usageLoadQueue'
import { formatCompactNumber } from '@/utils/format'
import { formatMoney } from '@/utils/money'
import UsageProgressBar from './UsageProgressBar.vue'
import OpenAIQuotaResetCell from './OpenAIQuotaResetCell.vue'
import GrokQuotaProbeCell from './GrokQuotaProbeCell.vue'
import CNProviderQuotaCell from './CNProviderQuotaCell.vue'
import CNProviderBalanceCell from './CNProviderBalanceCell.vue'
import OllamaCloudUsageCell from './OllamaCloudUsageCell.vue'
import { cnQuotaCellVisible as cnQuotaCellVisibleFn, cnBalanceCellVisible as cnBalanceCellVisibleFn } from './credentialsBuilder'

// 同一个渠道短时间内反复打开抽屉时复用上次的结果
const _usageCache = new Map<number, { data: AccountUsageInfo; ts: number }>()
const USAGE_CACHE_TTL = 5 * 60 * 1000 // 5 minutes

const props = defineProps<{ account: Account }>()

const emit = defineEmits<{
  'account-updated': [account: Account]
}>()

const { t } = useI18n()

const unmounted = ref(false)
onBeforeUnmount(() => { unmounted.value = true })

const loading = ref(false)
const activeQueryLoading = ref(false)
const activeQueryError = ref('')
const error = ref<string | null>(null)
const usageInfo = ref<AccountUsageInfo | null>(null)

const CN_PROVIDER_PLATFORMS = new Set(['kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go'])
const isCNProvider = computed(() => CN_PROVIDER_PLATFORMS.has(props.account.platform))

const isAnthropicOAuthOrSetupToken = computed(() =>
  props.account.platform === 'anthropic' && (props.account.type === 'oauth' || props.account.type === 'setup-token')
)

// Gemini 的授权通道 / 等级 / 模拟配额只对成品号（OAuth、Vertex）成立：第三方 key 一律按中转处理
// （muqian 2026-09-29 删海外四家官方 Key），平台标签是 gemini 的 key 只是按 Gemini 协议归族，走下面的通用额度
const isGeminiSubscription = computed(() => props.account.platform === 'gemini' && props.account.type !== 'apikey')

// 只有这些渠道有上游用量接口可拉
const shouldFetchUsage = computed(() => {
  if (isAnthropicOAuthOrSetupToken.value) return true
  if (isGeminiSubscription.value) return true
  if (['antigravity', 'grok', 'openai'].includes(props.account.platform)) return props.account.type === 'oauth'
  return false
})

// 国产平台两个子格的可见性（与子格共用 credentialsBuilder 的同一实现）
const cnAccountMode = computed(() => {
  const mode = props.account.credentials?.account_mode
  return typeof mode === 'string' ? mode : ''
})
const cnQuotaCellVisible = computed(() => cnQuotaCellVisibleFn(props.account.platform, cnAccountMode.value))
const cnBalanceCellVisible = computed(() => cnBalanceCellVisibleFn(props.account.platform, cnAccountMode.value))

const hasOpenAIUsage = computed(() => !!usageInfo.value?.five_hour || !!usageInfo.value?.seven_day)

// 按 7 天窗口当前的成本和用量，推算用满这个窗口要花多少成本
const openAISevenDayEstimatedTotalCost = computed(() => {
  const sevenDay = usageInfo.value?.seven_day
  const utilization = sevenDay?.utilization
  const currentCost = sevenDay?.window_stats?.cost
  if (
    typeof utilization !== 'number' ||
    typeof currentCost !== 'number' ||
    !Number.isFinite(utilization) ||
    !Number.isFinite(currentCost) ||
    utilization <= 0 ||
    currentCost <= 0
  ) {
    return null
  }
  const estimate = (currentCost * 100) / utilization
  return Number.isFinite(estimate) && estimate > 0 ? estimate : null
})

const openAIUsageRefreshKey = computed(() => buildOpenAIUsageRefreshKey(props.account))

// ===== Antigravity：按模型组取最高使用率、最早重置时间 =====

interface AntigravityUsageResult {
  utilization: number
  resetTime: string | null
}

const hasAntigravityQuota = computed(() => {
  return !!usageInfo.value?.antigravity_quota && Object.keys(usageInfo.value.antigravity_quota).length > 0
})

const getAntigravityUsage = (modelNames: string[]): AntigravityUsageResult | null => {
  const quota = usageInfo.value?.antigravity_quota
  if (!quota) return null

  let maxUtilization = 0
  let earliestReset: string | null = null
  for (const model of modelNames) {
    const modelQuota = quota[model]
    if (!modelQuota) continue
    if (modelQuota.utilization > maxUtilization) maxUtilization = modelQuota.utilization
    if (modelQuota.reset_time && (!earliestReset || modelQuota.reset_time < earliestReset)) {
      earliestReset = modelQuota.reset_time
    }
  }

  if (maxUtilization === 0 && earliestReset === null && !modelNames.some((m) => quota[m])) return null
  return { utilization: maxUtilization, resetTime: earliestReset }
}

const ANTIGRAVITY_MODEL_GROUPS: Array<{ key: string; labelKey: string; models: string[] }> = [
  { key: 'gemini-3-pro', labelKey: 'admin.accounts.usageWindow.gemini3Pro', models: ['gemini-3-pro-low', 'gemini-3-pro-high', 'gemini-3-pro-preview'] },
  { key: 'gemini-3-flash', labelKey: 'admin.accounts.usageWindow.gemini3Flash', models: ['gemini-3-flash'] },
  { key: 'gemini-image', labelKey: 'admin.accounts.usageWindow.geminiImage', models: ['gemini-2.5-flash-image', 'gemini-3.1-flash-image', 'gemini-3-pro-image'] },
  {
    key: 'claude',
    labelKey: 'admin.accounts.usageWindow.claude',
    models: [
      'claude-fable-5-1',
      'claude-fable-5',
      'claude-sonnet-4-5', 'claude-opus-4-5-thinking',
      'claude-sonnet-4-6', 'claude-opus-4-6', 'claude-opus-4-6-thinking',
      'claude-opus-4-7', 'claude-opus-4-8'
    ]
  }
]

const antigravityBars = computed(() =>
  ANTIGRAVITY_MODEL_GROUPS.flatMap((group) => {
    const usage = getAntigravityUsage(group.models)
    return usage ? [{ key: group.key, label: t(group.labelKey), ...usage }] : []
  })
)

const aiCreditsDisplay = computed(() => {
  const credits = usageInfo.value?.ai_credits
  if (!credits || credits.length === 0) return null
  const total = credits.reduce((sum, credit) => sum + (credit.amount ?? 0), 0)
  if (total <= 0) return null
  return total.toFixed(0)
})

// 没有 Antigravity 使用资格（ineligibleTiers 非空）
const hasIneligibleTiers = computed(() => {
  const loadCodeAssist = (props.account.extra as Record<string, unknown> | undefined)?.load_code_assist as
    | Record<string, unknown>
    | undefined
  const ineligibleTiers = loadCodeAssist?.ineligibleTiers as unknown[] | undefined
  return Array.isArray(ineligibleTiers) && ineligibleTiers.length > 0
})

// ===== Gemini：授权通道 · 等级、配额政策 =====

const geminiCredentials = computed(() => props.account.credentials as GeminiCredentials | undefined)
const geminiTier = computed(() => geminiCredentials.value?.tier_id || null)
const geminiOAuthType = computed(() => (geminiCredentials.value?.oauth_type || '').trim() || null)

// project_id 存在即为 Code Assist（与后端一致）
const isGeminiCodeAssist = computed(() => {
  const creds = geminiCredentials.value
  return creds?.oauth_type === 'code_assist' || (!creds?.oauth_type && !!creds?.project_id)
})

type GeminiChannel = 'aiStudio' | 'codeAssist' | 'googleOne' | 'client'

const geminiChannel = computed((): GeminiChannel | null => {
  if (!isGeminiSubscription.value) return null
  if (geminiOAuthType.value === 'google_one') return 'googleOne'
  if (isGeminiCodeAssist.value) return 'codeAssist'
  if (geminiOAuthType.value === 'ai_studio') return 'client'
  // 旧数据未标类型：按 AI Studio 处理
  return 'aiStudio'
})

type GeminiLevel = 'free' | 'pro' | 'ultra' | 'standard' | 'enterprise' | 'paid'

const geminiUserLevel = computed((): GeminiLevel | null => {
  if (!isGeminiSubscription.value) return null

  const tier = (geminiTier.value || '').toString().trim()
  const tierLower = tier.toLowerCase()
  const tierUpper = tier.toUpperCase()

  // Google One：free / pro / ultra
  if (geminiOAuthType.value === 'google_one') {
    if (tierLower === 'google_one_free') return 'free'
    if (tierLower === 'google_ai_pro') return 'pro'
    if (tierLower === 'google_ai_ultra') return 'ultra'
    // 旧等级标记
    if (tierUpper === 'AI_PREMIUM' || tierUpper === 'GOOGLE_ONE_STANDARD') return 'pro'
    if (tierUpper === 'GOOGLE_ONE_UNLIMITED') return 'ultra'
    if (tierUpper === 'FREE' || tierUpper === 'GOOGLE_ONE_BASIC' || tierUpper === 'GOOGLE_ONE_UNKNOWN' || tierUpper === '') return 'free'
    return null
  }

  // GCP Code Assist：standard / enterprise
  if (isGeminiCodeAssist.value) {
    if (tierLower === 'gcp_enterprise') return 'enterprise'
    if (tierLower === 'gcp_standard') return 'standard'
    if (tierUpper.includes('ULTRA') || tierUpper.includes('ENTERPRISE')) return 'enterprise'
    return 'standard'
  }

  // AI Studio 客户端 OAuth：free / paid
  if (geminiOAuthType.value === 'ai_studio') {
    if (tierLower === 'aistudio_paid') return 'paid'
    if (tierLower === 'aistudio_free') return 'free'
    if (tierUpper.includes('PAID') || tierUpper.includes('PAYG') || tierUpper.includes('PAY')) return 'paid'
    if (tierUpper.includes('FREE')) return 'free'
    return null
  }

  return null
})

const geminiAuthTypeLabel = computed(() => {
  if (!geminiChannel.value) return null
  const channel = t(`admin.accounts.usageWindow.geminiChannel.${geminiChannel.value}`)
  return geminiUserLevel.value ? `${channel} · ${t(`admin.accounts.usageWindow.geminiLevel.${geminiUserLevel.value}`)}` : channel
})

const geminiQuotaPolicyLimits = computed(() => {
  const tierLower = (geminiTier.value || '').toString().trim().toLowerCase()

  if (geminiOAuthType.value === 'google_one') {
    if (tierLower === 'google_ai_ultra' || geminiUserLevel.value === 'ultra') {
      return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsUltra')
    }
    if (tierLower === 'google_ai_pro' || geminiUserLevel.value === 'pro') {
      return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsPro')
    }
    return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsFree')
  }

  if (isGeminiCodeAssist.value) {
    if (tierLower === 'gcp_enterprise' || geminiUserLevel.value === 'enterprise') {
      return t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsEnterprise')
    }
    return t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsStandard')
  }

  if (tierLower === 'aistudio_paid' || geminiUserLevel.value === 'paid') {
    return t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsPaid')
  }
  return t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsFree')
})

const geminiQuotaPolicyDocsUrl = computed(() => {
  if (geminiOAuthType.value === 'google_one' || isGeminiCodeAssist.value) {
    return 'https://developers.google.com/gemini-code-assist/resources/quotas'
  }
  return 'https://ai.google.dev/pricing'
})

// Google One 与 GCP 是共享的每日请求池，不分模型
const geminiUsesSharedDaily = computed(() => {
  return (
    !!usageInfo.value?.gemini_shared_daily ||
    !!usageInfo.value?.gemini_shared_minute ||
    geminiOAuthType.value === 'google_one' ||
    isGeminiCodeAssist.value
  )
})

const geminiUsageBars = computed(() => {
  const info = usageInfo.value
  if (!isGeminiSubscription.value || !info) return []

  const bars: Array<{ key: string; label: string; utilization: number; resetsAt: string | null; windowStats?: WindowStats | null }> = []
  if (geminiUsesSharedDaily.value) {
    const sharedDaily = info.gemini_shared_daily
    if (sharedDaily) {
      bars.push({
        key: 'shared_daily',
        label: t('admin.accounts.usageWindow.daily'),
        utilization: sharedDaily.utilization,
        resetsAt: sharedDaily.resets_at,
        windowStats: sharedDaily.window_stats
      })
    }
    return bars
  }

  if (info.gemini_pro_daily) {
    bars.push({
      key: 'pro_daily',
      label: t('admin.accounts.usageWindow.geminiProDaily'),
      utilization: info.gemini_pro_daily.utilization,
      resetsAt: info.gemini_pro_daily.resets_at,
      windowStats: info.gemini_pro_daily.window_stats
    })
  }
  if (info.gemini_flash_daily) {
    bars.push({
      key: 'flash_daily',
      label: t('admin.accounts.usageWindow.geminiFlashDaily'),
      utilization: info.gemini_flash_daily.utilization,
      resetsAt: info.gemini_flash_daily.resets_at,
      windowStats: info.gemini_flash_daily.window_stats
    })
  }
  return bars
})

// ===== Grok =====

interface GrokQuotaBarInfo {
  utilization: number
  resetsAt: string | null
  windowStats?: WindowStats | null
}

const grokBilling = computed(() => usageInfo.value?.grok_billing || null)
const grokLocalUsage7d = computed(() => (
  usageInfo.value?.grok_local_usage_7d || usageInfo.value?.seven_day?.window_stats || null
))
const grokLocalUsageMonthly = computed(() => (
  usageInfo.value?.grok_local_usage_monthly || usageInfo.value?.thirty_day?.window_stats || null
))
const grokWeeklyBillingBar = computed((): GrokQuotaBarInfo | null => {
  const billing = grokBilling.value
  if (billing?.period_type?.toLowerCase() !== 'weekly' || billing.usage_percent == null) {
    return null
  }
  return {
    utilization: Math.min(100, Math.max(0, billing.usage_percent)),
    resetsAt: billing.period_end || null,
    windowStats: grokLocalUsage7d.value
  }
})
// 月度已用 / 上限：优先 used_percent，否则按美分换算
const grokMonthlyBillingBar = computed((): GrokQuotaBarInfo | null => {
  const billing = grokBilling.value
  if (!billing) return null
  let utilization: number | null = null
  if (billing.used_percent != null && Number.isFinite(billing.used_percent)) {
    utilization = billing.used_percent
  } else if (
    billing.monthly_limit_cents != null &&
    billing.monthly_limit_cents > 0 &&
    billing.used_cents != null
  ) {
    utilization = (billing.used_cents / billing.monthly_limit_cents) * 100
  }
  if (utilization == null) return null
  // 只有周额度、没有月度上限时不重复画一条
  if (billing.period_type?.toLowerCase() === 'weekly' && billing.monthly_limit_cents == null) {
    return null
  }
  return {
    utilization: Math.min(100, Math.max(0, utilization)),
    resetsAt: billing.billing_period_end || billing.period_end || null,
    windowStats: grokLocalUsageMonthly.value
  }
})
// 预付余额只在大于 0 时写；已用 / 上限只在月度上限大于 0 时写（0 表示不限 / 没设）
const grokPrepaidMoneyLine = computed(() => {
  const billing = grokBilling.value
  if (!billing) return null
  const prepaid = billing.prepaid_balance
  const showPrepaid = prepaid != null && Number.isFinite(prepaid) && prepaid > 0
  const limitRaw =
    billing.monthly_limit != null
      ? billing.monthly_limit
      : billing.monthly_limit_cents != null
        ? billing.monthly_limit_cents / 100
        : null
  const showUsedLimit = limitRaw != null && Number.isFinite(limitRaw) && limitRaw > 0
  if (!showPrepaid && !showUsedLimit) return null
  const used =
    billing.monthly_used != null
      ? billing.monthly_used
      : billing.used_cents != null
        ? billing.used_cents / 100
        : 0
  return {
    prepaid: showPrepaid ? formatMoney(prepaid) : null,
    used: showUsedLimit ? formatMoney(used) : null,
    limit: showUsedLimit ? formatMoney(limitRaw) : null
  }
})
const grokPlanLabelIsFree = (value: string) => value.includes('free') || value.includes('basic')
const grokPlanLabelIsPaid = (value: string) => {
  return value !== '' && !grokPlanLabelIsFree(value) && !value.includes('unknown')
}
const grokIsFree = computed(() => {
  if (props.account.platform !== 'grok' || props.account.type !== 'oauth') return false
  const billing = grokBilling.value
  const plan = (billing?.plan || '').trim().toLowerCase()
  const tier = (usageInfo.value?.subscription_tier || '').trim().toLowerCase()
  const entitlement = (usageInfo.value?.grok_entitlement_status || '').toLowerCase()
  if (grokPlanLabelIsFree(tier)) return true
  if (grokPlanLabelIsPaid(tier)) return false
  if (
    billing?.usage_percent != null ||
    billing?.used_percent != null ||
    (billing?.monthly_limit_cents != null && billing.monthly_limit_cents > 0)
  ) return false
  if (grokPlanLabelIsPaid(plan)) return false
  if (grokPlanLabelIsFree(plan) || grokPlanLabelIsFree(entitlement)) return true
  return billing != null
})
const grokFreeQuotaUsage = computed(() => usageInfo.value?.grok_local_usage_24h || null)
const grokFreeTokenBar = computed(() => {
  if (!grokIsFree.value || !grokFreeQuotaUsage.value) return null
  const limit = usageInfo.value?.grok_free_token_limit
  if (typeof limit !== 'number' || limit <= 0) return null
  const used = Math.max(0, grokFreeQuotaUsage.value.tokens || 0)
  return { utilization: Math.min(100, (used / limit) * 100), limit }
})
const grokQuotaUnknown = computed(() => {
  if (props.account.platform !== 'grok') return false
  if (grokIsFree.value) return !grokFreeTokenBar.value
  if (grokWeeklyBillingBar.value || grokMonthlyBillingBar.value || grokPrepaidMoneyLine.value) return false
  return usageInfo.value?.grok_quota_snapshot_state !== 'observed'
})
const grokQuotaUnknownLabel = computed(() => {
  return usageInfo.value?.grok_quota_snapshot_state === 'no_headers'
    ? t('admin.accounts.usageWindow.grokNoHeaders')
    : t('admin.accounts.usageWindow.grokUnknown')
})
const grokRetryAfterLabel = computed(() => {
  const seconds = usageInfo.value?.grok_retry_after_seconds
  if (seconds == null || seconds <= 0) return null
  if (seconds < 60) return t('admin.accounts.duration.seconds', { s: seconds })
  return t('admin.accounts.duration.minutes', { m: Math.ceil(seconds / 60) })
})

// ===== 403 / 401 / 降级 =====

const isForbidden = computed(() => !!usageInfo.value?.is_forbidden)
const forbiddenType = computed(() => usageInfo.value?.forbidden_type || 'forbidden')
const validationURL = computed(() => usageInfo.value?.validation_url || '')
const needsReauth = computed(() => !!usageInfo.value?.needs_reauth)

const usageErrorLabel = computed(() => {
  if (usageInfo.value?.error_code === 'rate_limited') return t('admin.accounts.rateLimited')
  return t('admin.accounts.usageError')
})

const forbiddenLabel = computed(() => {
  switch (forbiddenType.value) {
    case 'validation':
      return t('admin.accounts.forbiddenValidation')
    case 'violation':
      return t('admin.accounts.forbiddenViolation')
    default:
      return t('admin.accounts.forbidden')
  }
})

const linkCopied = ref(false)
const copyValidationURL = async () => {
  if (!validationURL.value) return
  try {
    await navigator.clipboard.writeText(validationURL.value)
    linkCopied.value = true
    setTimeout(() => { linkCopied.value = false }, 2000)
  } catch {
    // 剪贴板不可用时不提示
  }
}

// ===== 第三方 key / Bedrock 的配额（按渠道成本累计） =====

interface QuotaBar {
  key: string
  label: string
  utilization: number
  resetsAt: string | null
  note: string
}

// 日 / 周限额固定按滚动窗口重置（2026-09-28 P5 写死）：起点 + 周期
const quotaResetsAt = (startKey: 'quota_daily_start' | 'quota_weekly_start'): string | null => {
  const extra = props.account.extra as Record<string, unknown> | undefined
  const isDaily = startKey === 'quota_daily_start'
  const startStr = extra?.[startKey] as string | undefined
  if (!startStr) return null
  const periodMs = isDaily ? 24 * 60 * 60 * 1000 : 7 * 24 * 60 * 60 * 1000
  return new Date(new Date(startStr).getTime() + periodMs).toISOString()
}

const isQuotaEligible = computed(() => props.account.type === 'apikey' || props.account.type === 'bedrock')

const quotaBars = computed<QuotaBar[]>(() => {
  const account = props.account
  if (!isQuotaEligible.value) return []
  const dims: Array<{ key: string; labelKey: string; used?: number | null; limit?: number | null; startKey?: 'quota_daily_start' | 'quota_weekly_start' }> = [
    { key: 'daily', labelKey: 'admin.accounts.usageWindow.quotaDaily', used: account.quota_daily_used, limit: account.quota_daily_limit, startKey: 'quota_daily_start' },
    { key: 'weekly', labelKey: 'admin.accounts.usageWindow.quotaWeekly', used: account.quota_weekly_used, limit: account.quota_weekly_limit, startKey: 'quota_weekly_start' },
    { key: 'total', labelKey: 'admin.accounts.usageWindow.quotaTotal', used: account.quota_used, limit: account.quota_limit }
  ]
  return dims.flatMap((dim) => {
    const limit = dim.limit ?? 0
    if (limit <= 0) return []
    const used = dim.used ?? 0
    return [{
      key: dim.key,
      label: t(dim.labelKey),
      utilization: (used / limit) * 100,
      resetsAt: dim.startKey ? quotaResetsAt(dim.startKey) : null,
      note: t('admin.accounts.usageWindow.quotaUsedOfLimit', { used: formatMoney(used), limit: formatMoney(limit) })
    }]
  })
})

// ===== 加载 =====

const loadUsage = async (options?: { source?: 'passive' | 'active'; bypassCache?: boolean }) => {
  if (!shouldFetchUsage.value) return

  if (!options?.bypassCache) {
    const cached = _usageCache.get(props.account.id)
    if (cached && Date.now() - cached.ts < USAGE_CACHE_TTL) {
      usageInfo.value = cached.data
      loading.value = false
      return
    }
  }

  loading.value = true
  error.value = null
  try {
    const fetchFn = () => options?.source
      ? adminAPI.accounts.getUsage(props.account.id, options.source, options.bypassCache === true)
      : adminAPI.accounts.getUsage(props.account.id)
    const result = await enqueueUsageRequest(props.account, fetchFn)
    if (!unmounted.value) {
      usageInfo.value = result
      _usageCache.set(props.account.id, { data: result, ts: Date.now() })
    }
  } catch (e: unknown) {
    if (!unmounted.value) {
      error.value = t('common.error')
      console.error('Failed to load usage:', e)
    }
  } finally {
    if (!unmounted.value) loading.value = false
  }
}

const loadActiveUsage = async () => {
  activeQueryLoading.value = true
  activeQueryError.value = ''
  try {
    usageInfo.value = await adminAPI.accounts.getUsage(props.account.id, 'active', true)
  } catch (e: unknown) {
    activeQueryError.value = extractApiErrorMessage(e, t('admin.accounts.usageWindow.queryFailed'))
  } finally {
    activeQueryLoading.value = false
  }
}

// 探测会写回上游配额快照，这里重拉一次让进度条和资格状态跟上
const handleGrokProbed = async () => {
  await loadUsage({ source: 'active', bypassCache: true })
}

const handleQuotaResetAccountUpdated = (account: Account) => {
  emit('account-updated', account)
}

const handleOllamaCloudUsageUpdated = (state: NonNullable<Account['ollama_cloud_usage']>) => {
  emit('account-updated', { ...props.account, ollama_cloud_usage: state })
}

const initialSource = () => (isAnthropicOAuthOrSetupToken.value ? 'passive' : undefined)

onMounted(() => {
  loadUsage({ source: initialSource() }).catch((e) => {
    console.error('Failed to load usage:', e)
  })
})

// 抽屉换了一个渠道：重新拉
watch(
  () => props.account.id,
  (id, previousId) => {
    if (id === previousId) return
    usageInfo.value = null
    error.value = null
    loadUsage({ source: initialSource() }).catch((e) => {
      console.error('Failed to load usage:', e)
    })
  }
)

// OpenAI 快照变了（重置、自动刷新）：绕过缓存重拉
watch(openAIUsageRefreshKey, (nextKey, prevKey) => {
  if (!prevKey || nextKey === prevKey) return
  if (props.account.platform !== 'openai' || props.account.type !== 'oauth') return
  _usageCache.delete(props.account.id)
  loadUsage().catch((e) => {
    console.error('Failed to reload usage:', e)
  })
})
</script>
