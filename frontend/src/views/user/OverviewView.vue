<template>
  <!--
    概览：控制台落地页。muqian 2026-09-24「以 Token 统计为主」「具体用量去用量页，余额顶部有，概览就是放几个图大致看一看，少点文字」：
    ① 三个数：今日 / 区间 / 累计 Token（/usage/dashboard/stats + 区间趋势求和）
    ② 模型用量：按模型的 Token 折线（muqian 2026-09-25 从迷你柱改成折线；前 3 个模型 + 其他，按天）
    ③ Token 构成：输入 / 输出 / 缓存读 / 缓存写占比 + 缓存命中率
    右上角 7 / 30 天切换，②③ 跟着变。② 的几条线要靠颜色区分模型（校验过的三色），其余单色，只有错误态用色；
    每块独立加载与重试，任一接口失败不把别的块显示成零。
    公告不在这里列——登录后弹窗（AnnouncementNotice）。
  -->
  <SiteShell :title="greeting">
    <template #actions>
      <SectionTabs v-model="rangeKey" :tabs="rangeTabs" :label="t('userUi.overview.range.label')" />
    </template>
    <div class="space-y-10">
      <section :aria-busy="statsLoading ? 'true' : undefined" data-testid="overview-numbers">
        <StatusState
          v-if="statsError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :action-label="t('userUi.usage.retry')"
          @action="loadStats"
        />
        <dl v-else v-reveal.stagger class="grid grid-cols-3 divide-x divide-af-hairline">
          <div v-for="item in numbers" :key="item.key" class="min-w-0 px-5 first:pl-0 last:pr-0 sm:px-8">
            <dt class="truncate text-13 text-af-ink-3">{{ item.label }}</dt>
            <dd class="mt-2 truncate text-[2rem] font-semibold leading-none tracking-[-0.02em] tabular-nums text-af-ink sm:text-[2.5rem]">
              {{ item.value }}
            </dd>
          </div>
        </dl>
      </section>

      <SheetSection :title="t('userUi.overview.models.title')">
        <StatusState
          v-if="snapshotError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :action-label="t('userUi.usage.retry')"
          @action="loadSnapshot"
        />
        <UsageModelTrendLines v-else :points="modelTrend" :days="days" :loading="snapshotLoading" />
      </SheetSection>

      <SheetSection :title="t('userUi.overview.composition.title')">
        <StatusState
          v-if="snapshotError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :action-label="t('userUi.usage.retry')"
          @action="loadSnapshot"
        />
        <UsageTokenComposition v-else :totals="composition" :loading="snapshotLoading" />
      </SheetSection>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usageAPI } from '@/api'
import type { UserDashboardStats } from '@/api/usage'
import { useAuthStore } from '@/stores/auth'
import { formatTokensK } from '@/utils/format'
import { vReveal } from '@/directives/reveal'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab } from '@/components/user/shell/types'
import UsageModelTrendLines from '@/components/user/usage/UsageModelTrendLines.vue'
import UsageTokenComposition from '@/components/user/usage/UsageTokenComposition.vue'
import type { ModelTrendPoint, TrendDataPoint } from '@/types'

const { t } = useI18n()
const authStore = useAuthStore()

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

// ---------- 时间范围 ----------
const RANGE_DAYS = { '7d': 7, '30d': 30 } as const
type RangeKey = keyof typeof RANGE_DAYS
const rangeKey = ref<RangeKey>('7d')
const rangeDays = computed(() => RANGE_DAYS[rangeKey.value])
const rangeTabs = computed<SectionTab[]>(() =>
  (Object.keys(RANGE_DAYS) as RangeKey[]).map((key) => ({ key, label: t('userUi.overview.range.days', { days: RANGE_DAYS[key] }) }))
)

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

/** 区间内逐日的日期，最后一天是今天 */
const days = computed(() => {
  const today = new Date()
  return Array.from({ length: rangeDays.value }, (_, i) => {
    const date = new Date(today)
    date.setDate(today.getDate() - (rangeDays.value - 1 - i))
    return formatLocalDate(date)
  })
})

// ---------- 今日 / 累计（不随范围变） ----------
const stats = ref<UserDashboardStats | null>(null)
const statsLoading = ref(false)
const statsError = ref(false)

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

// ---------- 区间：按模型趋势 + Token 构成（一次请求） ----------
const trend = ref<TrendDataPoint[]>([])
const modelTrend = ref<ModelTrendPoint[]>([])
const snapshotLoading = ref(false)
const snapshotError = ref(false)
let snapshotSeq = 0

async function loadSnapshot() {
  const seq = ++snapshotSeq
  snapshotLoading.value = true
  snapshotError.value = false
  try {
    const snapshot = await usageAPI.getDashboardSnapshotV2({
      start_date: days.value[0],
      end_date: days.value[days.value.length - 1],
      granularity: 'day',
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      include_trend: true,
      include_model_stats: false,
      include_model_trend: true
    })
    if (seq !== snapshotSeq) return
    trend.value = snapshot.trend || []
    modelTrend.value = snapshot.model_trend || []
  } catch (error) {
    if (seq !== snapshotSeq) return
    console.error('Failed to load usage snapshot:', error)
    snapshotError.value = true
  } finally {
    if (seq === snapshotSeq) snapshotLoading.value = false
  }
}

watch(rangeKey, () => void loadSnapshot())

const composition = computed(() =>
  trend.value.reduce(
    (acc, point) => ({
      input: acc.input + point.input_tokens,
      output: acc.output + point.output_tokens,
      cacheRead: acc.cacheRead + point.cache_read_tokens,
      cacheWrite: acc.cacheWrite + point.cache_creation_tokens
    }),
    { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 }
  )
)

// ---------- 三个数 ----------
const numbers = computed(() => {
  const rangeTokens = trend.value.reduce((sum, point) => sum + point.total_tokens, 0)
  return [
    { key: 'today', label: t('userUi.overview.numbers.today'), value: stats.value ? formatTokensK(stats.value.today_tokens) : '—' },
    {
      key: 'range',
      label: t('userUi.overview.numbers.range', { days: rangeDays.value }),
      value: snapshotLoading.value || snapshotError.value ? '—' : formatTokensK(rangeTokens)
    },
    { key: 'total', label: t('userUi.overview.numbers.total'), value: stats.value ? formatTokensK(stats.value.total_tokens) : '—' }
  ]
})

onMounted(() => {
  // 顶栏余额来自当前用户：进概览时刷新一次
  void authStore.refreshUser().catch(() => undefined)
  void loadStats()
  void loadSnapshot()
})
</script>
