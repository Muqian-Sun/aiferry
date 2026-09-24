<template>
  <!--
    概览：控制台落地页（muqian 2026-09-23 定）。与首页同一套写法——大字、细线分区、文字链接、进场淡入；只有错误态用色。
    ① 账户：余额大字 + 充值入口 + 累计一行；右侧今日三数（/usage/dashboard/stats，不随时间范围变）
    ② 近 7 天趋势：Token / 请求 / 费用 页签，墨色单线
    ③ 近 24 小时每小时请求（柱）+ 近 7 天模型分布（横条）并排——muqian 2026-09-24「概览页多一点图」；全部单色
    ④ 快速开始：接口地址 + 第一把可用密钥 + 调用示例，一键复制（放在图表之后：老用户进来先看用量）
    ⑤ 公告：最近 5 条，点开走全站同一个弹窗；没有公告整段不出现
    每块独立加载与重试，任一接口失败不把别的块显示成零。
  -->
  <SiteShell :title="greeting" :description="t('userUi.overview.description')">
    <div class="space-y-10">
      <section :aria-busy="statsLoading ? 'true' : undefined" data-testid="overview-account">
        <StatusState
          v-if="statsError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :description="t('userUi.usage.loadFailedHint')"
          :action-label="t('userUi.usage.retry')"
          @action="loadStats"
        />
        <div v-else v-reveal.stagger class="grid gap-y-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)] lg:items-end lg:gap-x-12">
          <div>
            <p class="text-13 text-af-ink-3">{{ headline.label }}</p>
            <p class="mt-2 text-[2.75rem] font-semibold leading-none tracking-[-0.02em] tabular-nums text-af-ink" data-testid="overview-headline">
              {{ headline.value }}
            </p>
            <div class="mt-5 flex flex-wrap items-center gap-x-6 gap-y-2">
              <RouterLink v-if="canRecharge" to="/billing/recharge" class="hero-link hero-link-primary text-base" data-testid="overview-recharge">
                {{ t('userUi.usage.stats.recharge') }}
                <Icon name="arrowRight" size="sm" class="hero-link-arrow" />
              </RouterLink>
              <span class="text-13 tabular-nums text-af-ink-3">{{ totalsLine }}</span>
            </div>
          </div>
          <dl class="grid grid-cols-3 divide-x divide-af-hairline" data-testid="overview-today">
            <div v-for="item in todayItems" :key="item.key" class="min-w-0 px-5 first:pl-0 last:pr-0 sm:px-8">
              <dt class="text-13 text-af-ink-3">{{ item.label }}</dt>
              <dd class="mt-2 truncate text-2xl font-semibold tabular-nums text-af-ink">{{ item.value }}</dd>
              <p v-if="item.hint" class="mt-1 truncate text-xs text-af-ink-4">{{ item.hint }}</p>
            </div>
          </dl>
        </div>
      </section>

      <SheetSection :title="t('userUi.overview.trend.title')" :description="trendSummary">
        <template #actions>
          <SectionTabs v-model="trendMetric" :tabs="trendMetricTabs" :label="t('userUi.overview.trend.title')" />
        </template>
        <StatusState
          v-if="trendError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :description="t('userUi.usage.loadFailedHint')"
          :action-label="t('userUi.usage.retry')"
          @action="loadTrend"
        />
        <UsageMetricTrend v-else :trend-data="trend" :metric="trendMetric" :loading="trendLoading" data-testid="overview-trend" />
      </SheetSection>

      <!-- 今日每小时 + 近 7 天模型分布：并排两块，窄屏上下叠；各自加载、各自失败 -->
      <section class="grid gap-x-12 gap-y-10 border-t border-af-hairline pt-6 lg:grid-cols-2" data-testid="overview-charts">
        <div class="min-w-0">
          <h2 class="text-base font-semibold text-af-ink">{{ t('userUi.overview.hourly.title') }}</h2>
          <p class="mt-0.5 text-13 text-af-ink-3">{{ hourlySummary }}</p>
          <div class="mt-4">
            <StatusState
              v-if="hourlyError"
              kind="error"
              :title="t('userUi.usage.loadFailed')"
              :action-label="t('userUi.usage.retry')"
              @action="loadHourly"
            />
            <UsageHourlyBars v-else :points="hourly" :loading="hourlyLoading" />
          </div>
        </div>
        <div class="min-w-0">
          <h2 class="text-base font-semibold text-af-ink">{{ t('userUi.overview.models.title') }}</h2>
          <p class="mt-0.5 text-13 text-af-ink-3">{{ t('userUi.overview.models.description', { days: TREND_DAYS }) }}</p>
          <div class="mt-4">
            <StatusState
              v-if="trendError"
              kind="error"
              :title="t('userUi.usage.loadFailed')"
              :action-label="t('userUi.usage.retry')"
              @action="loadTrend"
            />
            <UsageModelShare v-else :models="modelStats" :loading="trendLoading" />
          </div>
        </div>
      </section>

      <SheetSection :title="t('userUi.overview.quickStart.title')" :description="t('userUi.overview.quickStart.description')">
        <dl class="divide-y divide-af-hairline" data-testid="overview-quick-start">
          <div class="flex flex-wrap items-center gap-x-6 gap-y-1 py-3.5">
            <dt class="w-24 shrink-0 text-13 text-af-ink-3">{{ t('userUi.overview.quickStart.baseUrl') }}</dt>
            <dd class="min-w-0 flex-1 truncate font-mono text-sm text-af-ink" data-testid="overview-base-url">{{ baseUrl }}</dd>
            <button type="button" :class="COPY_BUTTON" @click="copy('url', baseUrl)">
              <Icon :name="copied === 'url' ? 'check' : 'copy'" size="sm" />
              {{ copied === 'url' ? t('userUi.overview.quickStart.copied') : t('userUi.overview.quickStart.copy') }}
            </button>
          </div>
          <div class="flex flex-wrap items-center gap-x-6 gap-y-1 py-3.5">
            <dt class="w-24 shrink-0 text-13 text-af-ink-3">{{ t('userUi.overview.quickStart.key') }}</dt>
            <dd class="min-w-0 flex-1 truncate text-sm" data-testid="overview-key">
              <template v-if="firstKey">
                <span class="font-medium text-af-ink">{{ firstKey.name }}</span>
                <code class="ml-3 font-mono text-af-ink-3">{{ maskApiKey(firstKey.key) }}</code>
              </template>
              <span v-else-if="!keysLoading" class="text-af-ink-3">{{ t('userUi.overview.quickStart.noKey') }}</span>
            </dd>
            <button v-if="firstKey" type="button" :class="COPY_BUTTON" @click="copy('key', firstKey.key)">
              <Icon :name="copied === 'key' ? 'check' : 'copy'" size="sm" />
              {{ copied === 'key' ? t('userUi.overview.quickStart.copied') : t('userUi.overview.quickStart.copy') }}
            </button>
            <RouterLink to="/keys" class="hero-link text-13 font-medium">
              {{ firstKey ? t('userUi.overview.quickStart.allKeys') : t('userUi.overview.quickStart.createKey') }}
              <Icon name="arrowRight" size="xs" class="hero-link-arrow" />
            </RouterLink>
          </div>
          <!-- 调用示例：用目录里真有的模型、按它的原生协议写；密钥写成 $API_KEY，不把完整密钥摆在屏幕上 -->
          <div v-if="example" class="flex flex-wrap items-start gap-x-6 gap-y-2 py-3.5" data-testid="overview-example">
            <dt class="w-24 shrink-0 pt-2.5 text-13 text-af-ink-3">{{ t('userUi.overview.quickStart.example') }}</dt>
            <dd class="min-w-0 flex-1">
              <pre class="overflow-x-auto rounded-md bg-af-sunken px-4 py-3 font-mono text-13 leading-6 text-af-ink-2"><code>{{ example }}</code></pre>
              <p class="mt-2 text-xs text-af-ink-4">{{ t('userUi.overview.quickStart.exampleHint') }}</p>
            </dd>
            <button type="button" :class="[COPY_BUTTON, 'pt-2.5']" @click="copy('example', example)">
              <Icon :name="copied === 'example' ? 'check' : 'copy'" size="sm" />
              {{ copied === 'example' ? t('userUi.overview.quickStart.copied') : t('userUi.overview.quickStart.copy') }}
            </button>
          </div>
        </dl>
      </SheetSection>

      <SheetSection v-if="recentAnnouncements.length" :title="t('userUi.usage.sections.announcements')">
        <template v-if="unreadAnnouncements > 0" #actions>
          <span class="text-13 text-af-ink-3">{{ t('userUi.usage.announcements.unread', { count: unreadAnnouncements }) }}</span>
        </template>
        <ul class="divide-y divide-af-hairline" data-testid="announcement-list">
          <li v-for="item in recentAnnouncements" :key="item.id">
            <button type="button" class="flex w-full items-baseline gap-3 py-3 text-left transition-colors hover:text-af-ink-2" @click="openAnnouncement(item)">
              <span class="h-1.5 w-1.5 shrink-0 self-center rounded-full" :class="item.read_at ? 'bg-transparent' : 'bg-af-ink'" aria-hidden="true" />
              <span class="min-w-0 flex-1 truncate text-sm font-medium text-af-ink">{{ item.title }}</span>
              <time class="shrink-0 text-xs tabular-nums text-af-ink-4" :datetime="item.created_at">{{ formatDateOnly(item.created_at) }}</time>
            </button>
          </li>
        </ul>
      </SheetSection>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { keysAPI, usageAPI } from '@/api'
import type { UserDashboardStats } from '@/api/usage'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useAnnouncementStore } from '@/stores/announcements'
import { useClipboard } from '@/composables/useClipboard'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'
import { formatCurrency, formatDateOnly, formatNumber, formatTokensK } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import { getModelPlaza } from '@/api/modelPlaza'
import { buildCatalog, type CatalogModel } from '@/components/modelPlaza/catalog'
import { vReveal } from '@/directives/reveal'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab } from '@/components/user/shell/types'
import UsageMetricTrend, { type UsageTrendMetric } from '@/components/user/usage/UsageMetricTrend.vue'
import UsageHourlyBars from '@/components/user/usage/UsageHourlyBars.vue'
import UsageModelShare from '@/components/user/usage/UsageModelShare.vue'
import Icon from '@/components/icons/Icon.vue'
import type { ApiKey, ModelStat, TrendDataPoint, UserAnnouncement } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const announcementStore = useAnnouncementStore()
const { copyToClipboard } = useClipboard()

const COPY_BUTTON = 'inline-flex shrink-0 items-center gap-1.5 text-13 font-medium text-af-ink-2 transition-colors hover:text-af-ink'

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

// ---------- 账户与今日 ----------
const stats = ref<UserDashboardStats | null>(null)
const statsLoading = ref(false)
const statsError = ref(false)
const simpleMode = computed(() => authStore.isSimpleMode)
const paymentEnabled = computed(() => resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.payment))
const canRecharge = computed(() => paymentEnabled.value && !simpleMode.value)

/** 左侧大字：余额；simple mode 没有余额概念，换成平均耗时（同原用量页的账户带） */
const headline = computed(() =>
  simpleMode.value
    ? { label: t('userUi.usage.stats.avgLatency'), value: `${Math.round(stats.value?.average_duration_ms ?? 0)} ms` }
    : { label: t('userUi.usage.stats.balance'), value: formatCurrency(Number(authStore.user?.balance ?? 0)) }
)
const totalsLine = computed(() =>
  t('userUi.overview.totals', {
    cost: formatCurrency(stats.value?.total_actual_cost ?? 0),
    requests: formatNumber(stats.value?.total_requests ?? 0),
    rpm: formatNumber(stats.value?.rpm ?? 0)
  })
)
const todayItems = computed(() => {
  const s = stats.value
  return [
    {
      key: 'today-cost',
      label: t('userUi.usage.stats.todayCost'),
      value: formatCurrency(s?.today_actual_cost ?? 0),
      hint: s && s.today_cost > s.today_actual_cost ? `${t('userUi.usage.stats.standardCost')} ${formatCurrency(s.today_cost)}` : ''
    },
    { key: 'today-requests', label: t('userUi.usage.stats.todayRequests'), value: formatNumber(s?.today_requests ?? 0), hint: '' },
    { key: 'today-tokens', label: t('userUi.usage.stats.todayTokens'), value: formatTokensK(s?.today_tokens ?? 0), hint: '' }
  ]
})

async function loadStats() {
  statsLoading.value = true
  statsError.value = false
  try {
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
    statsError.value = true
  } finally {
    statsLoading.value = false
  }
}

// ---------- 快速开始 ----------
const baseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || window.location.origin)
const firstKey = ref<ApiKey | null>(null)
const keysLoading = ref(false)
const copied = ref<'url' | 'key' | 'example' | null>(null)
let copiedTimer: ReturnType<typeof setTimeout> | null = null

/** 第一把可用的密钥（按创建时间倒序）；拿不到就显示「创建密钥」入口，不报错 */
async function loadFirstKey() {
  keysLoading.value = true
  try {
    const page = await keysAPI.list(1, 1, { status: 'active', sort_by: 'created_at', sort_order: 'desc' })
    firstKey.value = page.items[0] ?? null
  } catch (error) {
    console.error('Failed to load keys:', error)
    firstKey.value = null
  } finally {
    keysLoading.value = false
  }
}

// 调用示例的模型：目录里真有的（优先 OpenAI 厂商，Chat Completions 最通用）；拿不到目录就不出示例
const exampleModel = ref<CatalogModel | null>(null)
async function loadExampleModel() {
  try {
    const catalog = buildCatalog((await getModelPlaza()).models ?? [])
    exampleModel.value = catalog.find((m) => m.vendor === 'openai') ?? catalog[0] ?? null
  } catch (error) {
    console.error('Failed to load model catalog:', error)
    exampleModel.value = null
  }
}

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
  const hello = t('userUi.overview.quickStart.exampleMessage')
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

// ---------- 近 7 天趋势 ----------
const TREND_DAYS = 7
const trend = ref<TrendDataPoint[]>([])
const trendLoading = ref(false)
const trendError = ref(false)
const trendMetric = ref<UsageTrendMetric>('tokens')
const trendMetricTabs = computed<SectionTab[]>(() => [
  { key: 'tokens', label: t('userUi.usage.trend.tokens') },
  { key: 'requests', label: t('userUi.usage.trend.requests') },
  { key: 'cost', label: t('userUi.usage.trend.cost') }
])

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

/** 区间合计写在标题下：趋势点逐日求和（拿不到数据时留空，不显示零） */
const trendSummary = computed(() => {
  if (trendError.value || trend.value.length === 0) return t('userUi.overview.trend.range', { days: TREND_DAYS })
  const sum = trend.value.reduce(
    (acc, point) => ({
      requests: acc.requests + point.requests,
      tokens: acc.tokens + point.total_tokens,
      cost: acc.cost + point.actual_cost
    }),
    { requests: 0, tokens: 0, cost: 0 }
  )
  return t('userUi.overview.trend.summary', {
    days: TREND_DAYS,
    requests: formatNumber(sum.requests),
    tokens: formatTokensK(sum.tokens),
    cost: formatCurrency(sum.cost)
  })
})

/** 按天补齐：接口只返回有请求的日子，趋势要连续 N 天，缺的日子各项记 0 */
function fillDays(points: TrendDataPoint[], start: Date, days: number): TrendDataPoint[] {
  const byDate = new Map(points.map((p) => [p.date, p]))
  return Array.from({ length: days }, (_, i) => {
    const date = formatLocalDate(new Date(start.getTime() + i * 24 * 60 * 60 * 1000))
    return (
      byDate.get(date) ?? {
        date,
        requests: 0,
        input_tokens: 0,
        output_tokens: 0,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        total_tokens: 0,
        cost: 0,
        actual_cost: 0
      }
    )
  })
}

// ---------- 近 24 小时 ----------
const hourly = ref<TrendDataPoint[]>([])
const hourlyLoading = ref(true)
const hourlyError = ref(false)
const modelStats = ref<ModelStat[]>([])

/** 近 24 小时内的小时桶（接口取的是昨天 + 今天，这里截掉 24 小时以前的） */
const recentHourly = computed(() => {
  const cutoff = Date.now() - 24 * 60 * 60 * 1000
  return hourly.value.filter((p) => {
    const ts = new Date(p.date.replace(' ', 'T')).getTime()
    return Number.isFinite(ts) && ts + 60 * 60 * 1000 > cutoff
  })
})

/** 「近 24 小时 N 次请求 · 高峰 HH:00」；没有请求时只说「近 24 小时暂无请求」 */
const hourlySummary = computed(() => {
  const points = recentHourly.value
  const total = points.reduce((sum, p) => sum + p.requests, 0)
  if (total <= 0) return t('userUi.overview.hourly.none')
  const peak = points.reduce((best, p) => (p.requests > best.requests ? p : best), points[0])
  const peakHour = /(\d{2}):00$/.exec(peak.date)?.[1] ?? ''
  return t('userUi.overview.hourly.summary', { requests: formatNumber(total), peak: `${peakHour}:00` })
})

async function loadHourly() {
  hourlyLoading.value = true
  hourlyError.value = false
  const now = new Date()
  try {
    const snapshot = await usageAPI.getDashboardSnapshotV2({
      start_date: formatLocalDate(new Date(now.getTime() - 24 * 60 * 60 * 1000)),
      end_date: formatLocalDate(now),
      granularity: 'hour',
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      include_trend: true,
      include_model_stats: false
    })
    hourly.value = snapshot.trend || []
  } catch (error) {
    console.error('Failed to load hourly usage:', error)
    hourlyError.value = true
  } finally {
    hourlyLoading.value = false
  }
}

async function loadTrend() {
  trendLoading.value = true
  trendError.value = false
  const end = new Date()
  const start = new Date(end.getTime() - (TREND_DAYS - 1) * 24 * 60 * 60 * 1000)
  try {
    const snapshot = await usageAPI.getDashboardSnapshotV2({
      start_date: formatLocalDate(start),
      end_date: formatLocalDate(end),
      granularity: 'day',
      include_trend: true,
      include_model_stats: true
    })
    trend.value = fillDays(snapshot.trend || [], start, TREND_DAYS)
    modelStats.value = snapshot.models || []
  } catch (error) {
    console.error('Failed to load trend:', error)
    trendError.value = true
  } finally {
    trendLoading.value = false
  }
}

// ---------- 公告 ----------
const RECENT_ANNOUNCEMENTS = 5
const recentAnnouncements = computed(() =>
  [...announcementStore.announcements]
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    .slice(0, RECENT_ANNOUNCEMENTS)
)
const unreadAnnouncements = computed(() => announcementStore.unreadCount)
function openAnnouncement(item: UserAnnouncement) {
  announcementStore.currentPopup = item
}

onMounted(() => {
  // 余额来自当前用户：进页时刷新一次（原用量页的行为，随账户带一起挪过来）
  void authStore.refreshUser().catch(() => undefined)
  void loadStats()
  void loadFirstKey()
  void loadTrend()
  void loadHourly()
  void loadExampleModel()
})
</script>
