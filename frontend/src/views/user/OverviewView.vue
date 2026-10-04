<template>
  <!--
    概览：控制台落地页，回答「我现在怎么样、有没有要处理的」（muqian 2026-09-25 定；24 日定的「以 Token 为主」不变）。
    ① 三个数：今日 / 区间 / 累计 Token，下面小字写实付；今日与区间可点，去用量明细并带上时间范围
    ② 需要处理：密钥过期 / 用尽 / 限额将满 / 快到期、余额快用完、订阅快到期或额度将满、今天的失败请求；没事整块不出现
    ③ 订阅额度：有生效中的订阅才出现，一个订阅一行
    ④ 用量趋势：一条墨线 + 淡面积（Token / 实付切换），替代原来每个模型一行的迷你柱
    一类信息只放一页（muqian 2026-09-26）：按模型的分布只在用量明细（费用分布），这里不再放模型用量表；
    Token 构成删掉（只能解释「Token 多、钱少是因为缓存读」，用户据此做不了什么；缓存命中率在用量明细摘要里）。
    新用户（还没有任何请求）把 ①④ 换成「开始使用」：接口地址 → 密钥 → 调用示例。
    右上角 7 / 30 天只作用于区间数与 ④。全部单色，只有异常用色；每块独立加载与重试，任一接口失败不把别的块显示成零。
    公告不在这里列——登录后弹窗（AnnouncementNotice）。
  -->
  <SiteShell :title="greeting">
    <template v-if="!isNewUser" #actions>
      <SectionTabs v-model="rangeKey" :tabs="rangeTabs" :label="t('userUi.overview.range.label')" />
    </template>
    <div class="space-y-10">
      <!-- 新用户：开始使用 -->
      <SheetSection
        v-if="isNewUser"
        :title="t('userUi.overview.gettingStarted.title')"
        :description="t('userUi.overview.gettingStarted.description')"
      >
        <dl class="divide-y divide-af-hairline" data-testid="overview-getting-started">
          <div class="flex flex-wrap items-center gap-x-6 gap-y-1 py-3.5">
            <dt class="w-24 shrink-0 text-13 text-af-ink-3">{{ t('userUi.overview.gettingStarted.baseUrl') }}</dt>
            <dd class="min-w-0 flex-1 truncate font-mono text-sm text-af-ink" data-testid="overview-base-url">{{ baseUrl }}</dd>
            <button type="button" :class="COPY_BUTTON" @click="copy('url', baseUrl)">
              <Icon :name="copied === 'url' ? 'check' : 'copy'" size="sm" />
              {{ copied === 'url' ? t('userUi.overview.gettingStarted.copied') : t('userUi.overview.gettingStarted.copy') }}
            </button>
          </div>
          <div class="flex flex-wrap items-center gap-x-6 gap-y-1 py-3.5">
            <dt class="w-24 shrink-0 text-13 text-af-ink-3">{{ t('userUi.overview.gettingStarted.key') }}</dt>
            <dd class="min-w-0 flex-1 truncate text-sm" data-testid="overview-key">
              <template v-if="firstKey">
                <span class="font-medium text-af-ink">{{ firstKey.name }}</span>
                <code class="ml-3 font-mono text-af-ink-3">{{ maskApiKey(firstKey.key) }}</code>
              </template>
              <span v-else-if="keysLoaded" class="text-af-ink-3">{{ t('userUi.overview.gettingStarted.noKey') }}</span>
            </dd>
            <button v-if="firstKey" type="button" :class="COPY_BUTTON" @click="copy('key', firstKey.key)">
              <Icon :name="copied === 'key' ? 'check' : 'copy'" size="sm" />
              {{ copied === 'key' ? t('userUi.overview.gettingStarted.copied') : t('userUi.overview.gettingStarted.copy') }}
            </button>
            <RouterLink to="/keys" class="hero-link text-13 font-medium" data-testid="overview-keys-link">
              {{ firstKey ? t('userUi.overview.gettingStarted.allKeys') : t('userUi.overview.gettingStarted.createKey') }}
              <Icon name="arrowRight" size="xs" class="hero-link-arrow" />
            </RouterLink>
          </div>
          <!-- 调用示例：用目录里真有的模型、按它的原生协议写；密钥写成 $API_KEY，不把完整密钥摆在屏幕上 -->
          <div v-if="example" class="flex flex-wrap items-start gap-x-6 gap-y-2 py-3.5" data-testid="overview-example">
            <dt class="w-24 shrink-0 pt-2.5 text-13 text-af-ink-3">{{ t('userUi.overview.gettingStarted.example') }}</dt>
            <dd class="min-w-0 flex-1">
              <pre class="overflow-x-auto rounded-md bg-af-sunken px-4 py-3 font-mono text-13 leading-6 text-af-ink-2"><code>{{ example }}</code></pre>
              <p class="mt-2 text-xs text-af-ink-4">{{ t('userUi.overview.gettingStarted.exampleHint') }}</p>
            </dd>
            <button type="button" :class="[COPY_BUTTON, 'pt-2.5']" @click="copy('example', example)">
              <Icon :name="copied === 'example' ? 'check' : 'copy'" size="sm" />
              {{ copied === 'example' ? t('userUi.overview.gettingStarted.copied') : t('userUi.overview.gettingStarted.copy') }}
            </button>
          </div>
        </dl>
      </SheetSection>

      <!-- ① 三个数：大字 Token，小字实付 -->
      <section v-else :aria-busy="statsLoading ? 'true' : undefined" data-testid="overview-numbers">
        <StatusState
          v-if="statsError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :action-label="t('userUi.usage.retry')"
          @action="loadStats"
        />
        <dl v-else v-reveal.stagger class="grid grid-cols-3 divide-x divide-af-hairline">
          <div v-for="item in numbers" :key="item.key" class="min-w-0 px-3 first:pl-0 last:pr-0 sm:px-8" :data-testid="`overview-number-${item.key}`">
            <dt class="text-xs text-af-ink-3 sm:truncate sm:text-13">{{ item.label }}</dt>
            <dd class="mt-2 truncate text-[1.625rem] font-semibold leading-none tracking-[-0.02em] text-af-ink sm:text-[2.5rem]">
              <RouterLink v-if="item.to" :to="item.to" class="decoration-af-hairline-strong decoration-2 underline-offset-8 hover:underline">
                {{ item.value }}
              </RouterLink>
              <template v-else>{{ item.value }}</template>
            </dd>
            <dd class="mt-2.5 truncate text-13 tabular-nums text-af-ink-3">
              {{ t('userUi.usage.stats.actualCost') }} <span class="text-af-ink-2">{{ item.cost }}</span>
            </dd>
          </div>
        </dl>
      </section>

      <!-- ② 需要处理：没事整块不出现 -->
      <SheetSection v-if="attentionItems.length" :title="t('userUi.overview.attention.title')">
        <ul class="divide-y divide-af-hairline" data-testid="overview-attention">
          <li v-for="item in attentionItems" :key="item.key" :data-testid="`overview-attention-${item.key}`">
            <component
              :is="item.to ? RouterLink : 'div'"
              :to="item.to"
              class="group flex items-center justify-between gap-4 py-2.5 text-sm text-af-ink"
            >
              <span class="flex min-w-0 items-center gap-2.5">
                <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="DOT[item.level]" aria-hidden="true" />
                <span class="min-w-0">{{ item.label }}</span>
              </span>
              <span v-if="item.to" class="inline-flex shrink-0 items-center gap-1 text-13 text-af-ink-3 group-hover:text-af-ink">
                {{ item.action }}
                <Icon name="chevronRight" size="sm" />
              </span>
            </component>
          </li>
        </ul>
      </SheetSection>

      <!-- ③ 订阅额度：有生效中的订阅才出现 -->
      <SheetSection v-if="subscriptionRows.length" :title="t('userUi.overview.subscriptions.title')">
        <template #actions>
          <RouterLink to="/billing/subscriptions" class="text-13 text-af-ink-3 hover:text-af-ink">{{ t('userUi.overview.subscriptions.manage') }}</RouterLink>
        </template>
        <ul class="divide-y divide-af-hairline" data-testid="overview-subscriptions">
          <li v-for="row in subscriptionRows" :key="row.id" class="grid gap-x-8 gap-y-3 py-3 sm:grid-cols-[minmax(0,12rem)_minmax(0,1fr)] sm:items-center">
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-af-ink">{{ row.name }}</p>
              <p class="mt-0.5 text-xs text-af-ink-3">{{ row.expiry }}</p>
            </div>
            <div v-if="row.meters.length" class="grid gap-x-6 gap-y-2 sm:grid-cols-3">
              <div v-for="meter in row.meters" :key="meter.key" class="min-w-0">
                <div class="flex items-baseline justify-between gap-2 text-xs">
                  <span class="text-af-ink-3">{{ meter.label }}</span>
                  <span class="tabular-nums" :class="LEVEL_TEXT[limitLevel(meter.ratio)]">{{ Math.round(meter.ratio * 100) }}%</span>
                </div>
                <div class="mt-1 h-1 overflow-hidden rounded-full bg-af-hairline">
                  <div class="h-full rounded-full" :class="LEVEL_BAR[limitLevel(meter.ratio)]" :style="{ width: `${Math.min(meter.ratio, 1) * 100}%` }" />
                </div>
              </div>
            </div>
            <p v-else class="text-13 text-af-ink-3">{{ t('userUi.overview.subscriptions.unlimited') }}</p>
          </li>
        </ul>
      </SheetSection>

      <template v-if="!isNewUser">
        <!-- ④ 用量趋势 -->
        <SheetSection :title="t('userUi.overview.trend.title')" data-testid="overview-trend">
          <template #actions>
            <div class="inline-flex rounded-lg bg-af-sunken p-1" role="tablist" :aria-label="t('userUi.overview.trend.title')">
              <button
                v-for="option in trendMetricOptions"
                :key="option.key"
                type="button"
                role="tab"
                class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
                :class="trendMetric === option.key ? 'bg-af-sheet text-af-ink' : 'text-af-ink-3 hover:text-af-ink-2'"
                :aria-selected="trendMetric === option.key"
                @click="trendMetric = option.key"
              >
                {{ option.label }}
              </button>
            </div>
          </template>
          <StatusState
            v-if="snapshotError"
            kind="error"
            :title="t('userUi.usage.loadFailed')"
            :action-label="t('userUi.usage.retry')"
            @action="loadSnapshot"
          />
          <UsageMetricTrend v-else :trend-data="trend" :metric="trendMetric" :loading="snapshotLoading" />
        </SheetSection>
      </template>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { usageAPI } from '@/api'
import type { UserDashboardStats } from '@/api/usage'
import subscriptionsAPI from '@/api/subscriptions'
import { adoptPreloaded } from '@/router/routePreload'
import { loadModelPlaza } from '../modelPlazaQuery'
import {
  OVERVIEW_DEFAULT_RANGE,
  OVERVIEW_RANGE_DAYS,
  overviewFailuresParams,
  overviewRange,
  overviewRequestKey,
  overviewSnapshotParams,
  overviewSubscriptionEnabled,
  type OverviewRangeKey
} from './overviewQuery'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'
import { formatCurrency, formatNumber, formatTokensK } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import { fillTrendBuckets, formatLocalDate, trendBucketKeys } from '@/utils/trendBuckets'
import { vReveal } from '@/directives/reveal'
import { buildCatalog, vendorLabel, type CatalogModel } from '@/components/modelPlaza/catalog'
import { newestFirst } from '@/components/keys/keyCatalog'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab } from '@/components/user/shell/types'
import UsageMetricTrend, { type UsageTrendMetric } from '@/components/user/usage/UsageMetricTrend.vue'
import { daysUntilExpiry, keyAttention, limitLevel, loadAllKeys, tightestLimit, type KeyAttention } from '@/components/user/keys/keyAttention'
import Icon from '@/components/icons/Icon.vue'
import type { ApiKey, TrendDataPoint, UserSubscription } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const COPY_BUTTON = 'inline-flex shrink-0 items-center gap-1.5 text-13 font-medium text-af-ink-2 transition-colors hover:text-af-ink'
const DOT = { danger: 'bg-af-danger', warning: 'bg-af-warning', normal: 'bg-af-ink-4' } as const
const LEVEL_BAR = { normal: 'bg-af-ink', warning: 'bg-af-warning', danger: 'bg-af-danger' } as const
const LEVEL_TEXT = { normal: 'text-af-ink-2', warning: 'text-af-warning', danger: 'text-af-danger' } as const

// ---------- 问候 ----------
/** 按本地时间分五段：0–5 夜深 / 5–11 早上 / 11–13 中午 / 13–18 下午 / 18–24 晚上 */
function greetingKey(hour: number): string {
  if (hour < 5) return 'lateNight'
  if (hour < 11) return 'morning'
  if (hour < 13) return 'noon'
  if (hour < 18) return 'afternoon'
  return 'evening'
}
const greeting = computed(() => {
  const user = authStore.user
  const name = user?.username?.trim() || user?.email || ''
  return t(`userUi.overview.greeting.${greetingKey(new Date().getHours())}`, { name })
})

const simpleMode = computed(() => authStore.isSimpleMode)
// 订阅显不显示由代码决定（SITE_FEATURES，见 overviewSubscriptionEnabled），不再读设置
const subscriptionEnabled = computed(() => overviewSubscriptionEnabled(simpleMode.value))
const canRecharge = computed(() => !simpleMode.value && resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.payment))
const errorViewEnabled = computed(() => appStore.cachedPublicSettings?.allow_user_view_error_requests ?? false)

// ---------- 时间范围（与各请求参数、进入页面前的预加载共用，见 ./overviewQuery.ts） ----------
const rangeKey = ref<OverviewRangeKey>(OVERVIEW_DEFAULT_RANGE)
const rangeDays = computed(() => OVERVIEW_RANGE_DAYS[rangeKey.value])
const rangeTabs = computed<SectionTab[]>(() =>
  (Object.keys(OVERVIEW_RANGE_DAYS) as OverviewRangeKey[]).map((key) => ({
    key,
    label: t('userUi.overview.range.days', { days: OVERVIEW_RANGE_DAYS[key] })
  }))
)

const today = formatLocalDate(new Date())
/** 区间起止（最后一天是今天）；用量明细的地址栏参数同名 */
const rangeQuery = computed(() => overviewRange(rangeDays.value))

// ---------- 今日 / 累计（不随范围变） ----------
const stats = ref<UserDashboardStats | null>(null)
const statsLoading = ref(false)
const statsError = ref(false)

async function loadStats() {
  statsLoading.value = true
  statsError.value = false
  try {
    stats.value = await adoptPreloaded(overviewRequestKey.stats, () => usageAPI.getDashboardStats())
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
    statsError.value = true
  } finally {
    statsLoading.value = false
  }
}

/** 还没有任何请求：换成「开始使用」 */
const isNewUser = computed(() => stats.value !== null && stats.value.total_requests === 0)

// ---------- 区间趋势 ----------
const trend = ref<TrendDataPoint[]>([])
const snapshotLoading = ref(false)
const snapshotError = ref(false)
let snapshotSeq = 0

async function loadSnapshot() {
  const seq = ++snapshotSeq
  const { start, end } = rangeQuery.value
  snapshotLoading.value = true
  snapshotError.value = false
  try {
    const params = overviewSnapshotParams(start, end)
    const snapshot = await adoptPreloaded(overviewRequestKey.snapshot(params), () => usageAPI.getDashboardSnapshotV2(params))
    if (seq !== snapshotSeq) return
    trend.value = fillTrendBuckets(snapshot.trend || [], trendBucketKeys(start, end, 'day'))
  } catch (error) {
    if (seq !== snapshotSeq) return
    console.error('Failed to load usage snapshot:', error)
    snapshotError.value = true
  } finally {
    if (seq === snapshotSeq) snapshotLoading.value = false
  }
}

watch(rangeKey, () => void loadSnapshot())

const trendMetric = ref<UsageTrendMetric>('tokens')
const trendMetricOptions = computed<Array<{ key: UsageTrendMetric; label: string }>>(() => [
  { key: 'tokens', label: t('userUi.usage.trend.tokens') },
  { key: 'cost', label: t('userUi.usage.stats.actualCost') }
])

// ---------- 三个数 ----------
const numbers = computed(() => {
  const s = stats.value
  const rangeReady = !snapshotLoading.value && !snapshotError.value
  const rangeSum = trend.value.reduce((acc, point) => ({ tokens: acc.tokens + point.total_tokens, cost: acc.cost + point.actual_cost }), { tokens: 0, cost: 0 })
  const items: Array<{ key: string; label: string; value: string; cost: string; to?: RouteLocationRaw }> = [
    {
      key: 'today',
      label: t('userUi.overview.numbers.today'),
      value: s ? formatTokensK(s.today_tokens) : '—',
      cost: s ? formatCurrency(s.today_actual_cost) : '—',
      to: { path: '/usage', query: { start: today, end: today } }
    },
    {
      key: 'range',
      label: t('userUi.overview.numbers.range', { days: rangeDays.value }),
      value: rangeReady ? formatTokensK(rangeSum.tokens) : '—',
      cost: rangeReady ? formatCurrency(rangeSum.cost) : '—',
      to: { path: '/usage', query: { ...rangeQuery.value } }
    },
    {
      key: 'total',
      label: t('userUi.overview.numbers.total'),
      value: s ? formatTokensK(s.total_tokens) : '—',
      cost: s ? formatCurrency(s.total_actual_cost) : '—'
    }
  ]
  return items
})

// ---------- 需要处理 ----------
const keysState = ref<{ keys: ApiKey[]; attention: KeyAttention } | null>(null)

async function loadKeys() {
  try {
    const { keys, complete } = await adoptPreloaded(overviewRequestKey.keys, () => loadAllKeys())
    keysState.value = complete ? { keys, attention: keyAttention(keys) } : null
  } catch (error) {
    console.error('Failed to load keys:', error)
    keysState.value = null
  } finally {
    keysLoaded.value = true
  }
}

const subscriptions = ref<UserSubscription[]>([])
async function loadSubscriptions() {
  if (!subscriptionEnabled.value) return
  try {
    subscriptions.value = await adoptPreloaded(overviewRequestKey.subscriptions, () => subscriptionsAPI.getActiveSubscriptions())
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    subscriptions.value = []
  }
}

const failuresToday = ref<number | null>(null)
async function loadFailures() {
  if (!errorViewEnabled.value) return
  try {
    const params = overviewFailuresParams(today)
    const resp = await adoptPreloaded(overviewRequestKey.failures(params), () => usageAPI.listMyErrorRequests(params))
    failuresToday.value = resp.total
  } catch (error) {
    console.error('Failed to load error count:', error)
    failuresToday.value = null
  }
}

interface AttentionItem {
  key: string
  label: string
  level: 'danger' | 'warning' | 'normal'
  to?: RouteLocationRaw
  action?: string
}

interface SubscriptionMeter {
  key: 'daily' | 'weekly' | 'monthly'
  label: string
  ratio: number
}

function subscriptionMeters(subscription: UserSubscription): SubscriptionMeter[] {
  const plan = subscription.plan
  const meters: SubscriptionMeter[] = []
  const push = (key: SubscriptionMeter['key'], used: number, limit: number | null | undefined) => {
    if (limit && limit > 0) meters.push({ key, label: t(`userUi.overview.subscriptions.${key}`), ratio: (used || 0) / limit })
  }
  push('daily', subscription.daily_usage_usd, plan?.daily_limit_usd)
  push('weekly', subscription.weekly_usage_usd, plan?.weekly_limit_usd)
  push('monthly', subscription.monthly_usage_usd, plan?.monthly_limit_usd)
  return meters
}

const subscriptionRows = computed(() =>
  subscriptions.value
    .filter((subscription) => subscription.status === 'active')
    .map((subscription) => {
      const days = daysUntilExpiry(subscription)
      return {
        id: subscription.id,
        name: subscription.plan?.name || t('userUi.overview.subscriptions.fallbackName'),
        expiry: days === null ? t('userUi.overview.subscriptions.noExpiry') : t('userUi.overview.subscriptions.expiresIn', { days }),
        daysLeft: days,
        meters: subscriptionMeters(subscription)
      }
    })
)

/** 近 7 天日均实付（取区间趋势的最后 7 天）；没有数据为 null */
const recentDailyCost = computed(() => {
  if (snapshotError.value || trend.value.length === 0) return null
  const lastWeek = trend.value.slice(-7)
  return lastWeek.reduce((sum, point) => sum + point.actual_cost, 0) / lastWeek.length
})

const attentionItems = computed<AttentionItem[]>(() => {
  const items: AttentionItem[] = []
  const view = t('userUi.overview.attention.view')

  const a = keysState.value?.attention
  if (a) {
    const keyItem = (
      list: ApiKey[],
      key: string,
      level: AttentionItem['level'],
      one: (item: ApiKey) => string,
      many: string,
      to: RouteLocationRaw
    ) => {
      if (list.length === 0) return
      items.push({ key, level, to, action: view, label: list.length === 1 ? one(list[0]) : t(many, { count: list.length }) })
    }
    keyItem(a.expired, 'keys-expired', 'danger', (k) => t('userUi.overview.attention.keyExpiredOne', { name: k.name }), 'userUi.overview.attention.keyExpired', { path: '/keys', query: { status: 'expired' } })
    keyItem(a.quotaExhausted, 'keys-quota', 'danger', (k) => t('userUi.overview.attention.keyQuotaOne', { name: k.name }), 'userUi.overview.attention.keyQuota', { path: '/keys', query: { status: 'quota_exhausted' } })
    keyItem(
      a.nearLimit,
      'keys-near-limit',
      'warning',
      (k) => {
        const meter = tightestLimit(k)
        const percent = Math.round((meter?.ratio ?? 0) * 100)
        return !meter || meter.kind === 'quota'
          ? t('userUi.overview.attention.keyQuotaNearOne', { name: k.name, percent })
          : t('userUi.overview.attention.keyNearLimitOne', { name: k.name, limit: t(`keys.detail.window.${meter.kind}`), percent })
      },
      'userUi.overview.attention.keyNearLimit',
      { path: '/keys', query: { attention: 'near_limit' } }
    )
    keyItem(
      a.expiringSoon,
      'keys-expiring',
      'warning',
      (k) => t('userUi.overview.attention.keyExpiringOne', { name: k.name, days: daysUntilExpiry(k) ?? 1 }),
      'userUi.overview.attention.keyExpiring',
      { path: '/keys', query: { attention: 'expiring' } }
    )
  }

  // 余额：只看按余额计费的用户（有生效订阅的由订阅额度那块管）
  const user = authStore.user
  if (user && !simpleMode.value && subscriptionRows.value.length === 0) {
    const balance = Number(user.balance ?? 0)
    const daily = recentDailyCost.value
    const recharge = canRecharge.value ? { to: '/billing/recharge', action: t('userUi.overview.attention.recharge') } : {}
    if (daily !== null && daily > 0 && balance <= 0) {
      items.push({ key: 'balance', level: 'danger', label: t('userUi.overview.attention.balanceEmpty'), ...recharge })
    } else if (daily !== null && daily > 0 && balance / daily < 7) {
      items.push({
        key: 'balance',
        level: 'warning',
        label: t('userUi.overview.attention.balanceRunway', { balance: formatCurrency(balance), days: Math.max(1, Math.floor(balance / daily)) }),
        ...recharge
      })
    } else if (user.balance_notify_enabled && user.balance_notify_threshold != null && balance < user.balance_notify_threshold) {
      items.push({
        key: 'balance',
        level: 'warning',
        label: t('userUi.overview.attention.balanceBelowThreshold', { balance: formatCurrency(balance), threshold: formatCurrency(user.balance_notify_threshold) }),
        ...recharge
      })
    }
  }

  for (const row of subscriptionRows.value) {
    const fullest = row.meters.reduce<SubscriptionMeter | null>((best, meter) => (!best || meter.ratio > best.ratio ? meter : best), null)
    if (fullest && fullest.ratio >= 0.8) {
      items.push({
        key: `subscription-${row.id}-quota`,
        level: fullest.ratio >= 1 ? 'danger' : 'warning',
        label: t('userUi.overview.attention.subscriptionQuota', { name: row.name, window: fullest.label, percent: Math.round(fullest.ratio * 100) }),
        to: '/billing/subscriptions',
        action: view
      })
    }
    if (row.daysLeft !== null && row.daysLeft <= 7) {
      items.push({
        key: `subscription-${row.id}-expiry`,
        level: 'warning',
        label: t('userUi.overview.attention.subscriptionExpiring', { name: row.name, days: row.daysLeft }),
        to: '/billing/subscriptions',
        action: t('userUi.overview.attention.renew')
      })
    }
  }

  if (failuresToday.value) {
    items.push({
      key: 'failures',
      level: 'warning',
      label: t('userUi.overview.attention.failuresToday', { count: formatNumber(failuresToday.value) }),
      to: { path: '/usage', query: { tab: 'errors', start: today, end: today } },
      action: view
    })
  }
  return items
})

// ---------- 开始使用（新用户） ----------
const baseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || window.location.origin)
const keysLoaded = ref(false)
/** 第一把可用的密钥（最新创建的在前）；没有就给「创建密钥」入口 */
const firstKey = computed<ApiKey | null>(() => {
  const keys = keysState.value?.keys ?? []
  return [...keys].filter((key) => key.status === 'active').sort((a, b) => b.created_at.localeCompare(a.created_at))[0] ?? null
})
const copied = ref<'url' | 'key' | 'example' | null>(null)
let copiedTimer: ReturnType<typeof setTimeout> | null = null

// 调用示例的模型：目录里真有的对话模型（优先 OpenAI 厂商，Chat Completions 最通用；同一家里取新的，
// 原来按字母取第一个，示例写成了 codex-auto-review）；拿不到目录就不出示例
const exampleModel = ref<CatalogModel | null>(null)
let exampleRequested = false
async function loadExampleModel() {
  if (exampleRequested) return
  exampleRequested = true
  try {
    const chat = buildCatalog((await loadModelPlaza()).models ?? []).filter((m) => m.billingMode === 'token')
    exampleModel.value = newestFirst(chat.filter((m) => vendorLabel(m.vendor) === 'OpenAI'))[0] ?? newestFirst(chat)[0] ?? null
  } catch (error) {
    console.error('Failed to load model catalog:', error)
    exampleModel.value = null
  }
}
watch(isNewUser, (value) => {
  if (value) void loadExampleModel()
}, { immediate: true })

/**
 * 按模型厂商的原生协议写 curl（网关各入口都认 Authorization: Bearer，见 server/middleware/api_key_auth*.go）：
 * Anthropic → /v1/messages；Gemini → /v1beta/models/{model}:generateContent；其余 → /v1/chat/completions。
 */
const example = computed(() => {
  const model = exampleModel.value
  if (!model) return ''
  const base = baseUrl.value.replace(/\/+$/, '')
  const auth = '  -H "Authorization: Bearer $API_KEY" \\'
  const json = '  -H "Content-Type: application/json" \\'
  const hello = t('userUi.overview.gettingStarted.exampleMessage')
  if (model.vendor === 'anthropic') {
    return [
      `curl ${base}/v1/messages \\`,
      auth,
      json,
      '  -H "anthropic-version: 2023-06-01" \\',
      `  -d '{"model": "${model.id}", "max_tokens": 256, "messages": [{"role": "user", "content": "${hello}"}]}'`
    ].join('\n')
  }
  if (model.vendor === 'google' || model.vendor === 'gemini') {
    return [
      `curl ${base}/v1beta/models/${model.id}:generateContent \\`,
      auth,
      json,
      `  -d '{"contents": [{"parts": [{"text": "${hello}"}]}]}'`
    ].join('\n')
  }
  return [
    `curl ${base}/v1/chat/completions \\`,
    auth,
    json,
    `  -d '{"model": "${model.id}", "messages": [{"role": "user", "content": "${hello}"}]}'`
  ].join('\n')
})

async function copy(which: 'url' | 'key' | 'example', value: string) {
  if (!(await copyToClipboard(value))) return
  copied.value = which
  if (copiedTimer) clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => (copied.value = null), 1500)
}

onBeforeUnmount(() => {
  if (copiedTimer) clearTimeout(copiedTimer)
})

// 订阅、失败请求要看公开设置里的开关：设置可能比页面晚到，到了（或变了）再查
watch(subscriptionEnabled, (enabled) => { if (enabled) void loadSubscriptions() }, { immediate: true })
watch(errorViewEnabled, (enabled) => { if (enabled) void loadFailures() }, { immediate: true })

onMounted(() => {
  // 顶栏余额来自当前用户：进概览时刷新一次
  void authStore.refreshUser().catch(() => undefined)
  void loadStats()
  void loadSnapshot()
  void loadKeys()
})
</script>
