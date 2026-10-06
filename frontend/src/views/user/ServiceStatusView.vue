<template>
  <!--
    服务状态（原「渠道监控」，站长 2026-09-26 定名）：回答「各模型现在能不能用、快不快、缓存命中多少」。
    与模型页一样对未登录访客开放，入口在顶栏（muqian 2026-09-30）：从控制台点进来留在控制台壳（页头由壳画），
    从首页等公开页进来走公开壳（consoleShell.ts）；
    加载方式也与模型页一样：路由预加载默认时间范围，数据到了再换页（serviceStatusQuery）；
    切换时间范围时旧内容留在原处（变淡），新数据到了再替换，不收成加载占位、不丢滚动位置。
    ① 整体：可用率 / 首字延迟 / 缓存命中率 + 全站趋势（三项切换）；不写结论句与更新时间（muqian 2026-09-30 删掉）
    ② 各模型 = 上架目录里的模型（muqian 2026-09-30，后端按目录出名单，没上架的不计）：
       多列格子（一格 = 状态、逐段细色条、可用率、首字延迟、缓存命中率），有问题的排前面，可按状态筛选、搜索；
       这段时间一个请求都没有的模型不占格子，折成一行「另有 N 个模型没有请求」，展开只列名字。
    数据是本站真实请求的被动统计；后端对普通访客只回这些字段（没有平台、站点流量、用户排行）。
  -->
  <SiteShell :variant="shell">
    <!-- 控制台壳：页头（标题 / 说明取路由）由壳画，时间范围放在页头右侧 -->
    <template v-if="shell === 'console'" #actions>
      <SectionTabs v-model="range" :tabs="rangeTabs" :label="t('userUi.serviceStatus.range.label')" />
    </template>
    <!-- 公开壳：页首与模型页同一套（muqian 2026-09-30）：标题 32 / 40px、说明 15px、逐行淡入上浮；时间范围在右侧与标题底部对齐 -->
    <div v-if="shell === 'public'" class="mb-8 grid gap-6 pt-2 sm:pt-4 lg:grid-cols-[1fr_auto] lg:items-end">
      <header v-reveal.stagger data-testid="service-status-hero">
        <h1 class="text-[2rem] font-semibold leading-tight tracking-[-0.02em] text-af-ink sm:text-[2.5rem]">
          {{ t('userUi.serviceStatus.title') }}
        </h1>
        <p class="mt-3 max-w-2xl text-[15px] leading-7 text-af-ink-2">{{ t('userUi.serviceStatus.description') }}</p>
      </header>
      <SectionTabs v-model="range" :tabs="rangeTabs" :label="t('userUi.serviceStatus.range.label')" />
    </div>

    <StatusState
      v-if="loadError && !models"
      kind="error"
      :title="t('userUi.serviceStatus.loadFailed')"
      :action-label="t('userUi.usage.retry')"
      @action="reload"
    />
    <StatusState v-else-if="!models" kind="loading" :title="t('userUi.status.loading')" />

    <div v-else class="space-y-10 transition-opacity" :class="loading ? 'opacity-60' : ''" :aria-busy="loading ? 'true' : undefined">
      <!-- ① 整体 -->
      <section class="space-y-6" data-testid="service-status-summary">
        <StatRow :items="statItems" />
        <div>
          <div class="mb-3 flex items-center justify-between gap-4">
            <h2 class="text-13 font-medium text-af-ink-2">{{ t('userUi.serviceStatus.trend.title') }}</h2>
            <SegmentedControl v-model="trendMetric" :options="trendOptions" :label="t('userUi.serviceStatus.trend.title')" />
          </div>
          <ServiceStatusTrend :slots="trendSlots" :bucket-seconds="bucketSeconds" :metric="trendMetric" />
        </div>
      </section>

      <!-- ② 各模型 -->
      <SheetSection :title="t('userUi.serviceStatus.models.title')">
        <template v-if="rows.length > SEARCH_THRESHOLD" #actions>
          <SegmentedControl v-model="healthFilter" :options="filterOptions" :label="t('userUi.serviceStatus.models.filter.label')" />
          <div class="w-full sm:w-48">
            <SearchInput v-model="search" compact :placeholder="t('userUi.serviceStatus.models.search')" />
          </div>
        </template>

        <p v-if="rows.length === 0" class="py-10 text-center text-sm text-af-ink-3">{{ t('userUi.serviceStatus.models.empty') }}</p>
        <template v-else>
          <!-- 与模型页同一种 hairline 分格（不是卡片）：窄屏一列、sm 两列、lg 三列 -->
          <!-- 控制台里有侧栏、内容区窄一截：宽屏（xl）才排 3 列 -->
          <ul v-if="visibleActive.length" class="-mx-6 grid border-t border-af-hairline" :class="shell === 'console' ? 'sm:grid-cols-2 xl:grid-cols-3' : 'sm:grid-cols-2 lg:grid-cols-3'" data-testid="service-status-models">
            <li
              v-for="row in visibleActive"
              :key="row.model"
              class="min-w-0 border-b border-af-hairline px-6 py-4"
              :class="shell === 'console' ? 'sm:max-xl:[&:nth-child(2n)]:border-l xl:[&:not(:nth-child(3n+1))]:border-l' : 'sm:max-lg:[&:nth-child(2n)]:border-l lg:[&:not(:nth-child(3n+1))]:border-l'"
              data-testid="service-status-model"
            >
              <div class="flex items-center gap-2.5">
                <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="HEALTH_DOT[row.health.overall]" aria-hidden="true" />
                <span class="min-w-0 flex-1 truncate font-mono text-sm font-medium text-af-ink" :title="row.model">{{ row.model }}</span>
                <span class="shrink-0 text-xs" :class="HEALTH_TEXT[row.health.overall]">{{ healthLabel(row.health.overall) }}</span>
              </div>
              <ServiceStatusStrip
                class="mt-3"
                :slots="row.slots"
                :bucket-seconds="bucketSeconds"
                :label="t('userUi.serviceStatus.models.stripLabel', { model: row.model })"
              />
              <p class="mt-2.5 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs tabular-nums text-af-ink-3">
                <span>
                  {{ t('userUi.serviceStatus.columns.availability') }}
                  <span :class="HEALTH_TEXT[row.health.availability]">{{ formatPercent(row.metrics.availability) }}</span>
                </span>
                <span>
                  {{ t('userUi.serviceStatus.columns.ttft') }}
                  <span :class="HEALTH_TEXT[row.health.ttft]">{{ formatLatency(row.metrics.ttft_p50_ms) }}</span>
                </span>
                <span>
                  {{ t('userUi.serviceStatus.columns.cache') }}
                  <span :class="HEALTH_TEXT[row.health.cache]">{{ formatPercent(row.metrics.cache_hit_rate) }}</span>
                </span>
              </p>
            </li>
          </ul>
          <p v-else-if="healthFilter !== 'all' || visibleIdle.length === 0" class="py-10 text-center text-sm text-af-ink-3">
            {{ t('userUi.serviceStatus.models.noMatch') }}
          </p>

          <!-- 这段时间没有请求的模型：不占格子，折成一行；搜索命中时直接展开 -->
          <div v-if="healthFilter === 'all' && visibleIdle.length" class="mt-4 text-13 text-af-ink-3" data-testid="service-status-idle">
            <p class="flex flex-wrap items-center gap-x-3 gap-y-1">
              <span>{{ t('userUi.serviceStatus.models.idle', { count: visibleIdle.length }) }}</span>
              <button
                v-if="!searchKeyword"
                type="button"
                class="font-medium text-af-brand hover:text-af-brand-hover"
                :aria-expanded="idleExpanded"
                @click="idleExpanded = !idleExpanded"
              >
                {{ idleExpanded ? t('userUi.serviceStatus.models.hideIdle') : t('userUi.serviceStatus.models.showIdle') }}
              </button>
            </p>
            <ul v-if="idleExpanded || searchKeyword" class="mt-3 flex flex-wrap gap-x-5 gap-y-1.5 font-mono text-xs text-af-ink-3">
              <li v-for="row in visibleIdle" :key="row.model">{{ row.model }}</li>
            </ul>
          </div>

          <!-- 图例 -->
          <ul class="mt-6 flex flex-wrap gap-x-5 gap-y-1.5 text-xs text-af-ink-3">
            <li v-for="level in LEGEND" :key="level" class="inline-flex items-center gap-1.5">
              <span class="h-2.5 w-2.5 rounded-[2px]" :class="HEALTH_BAR[level]" aria-hidden="true" />
              {{ t(`userUi.serviceStatus.legend.${level}`) }}
            </li>
          </ul>
        </template>
      </SheetSection>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePreferredReducedMotion, useTransition } from '@vueuse/core'
import { vReveal } from '@/directives/reveal'
import type { ServiceHealth, ServiceStatusModels, ServiceStatusRange, ServiceStatusSnapshot } from '@/api/serviceStatus'
import { SERVICE_STATUS_DEFAULT_RANGE, loadServiceStatus } from './serviceStatusQuery'
import { useShellVariant } from '@/components/user/shell/consoleShell'
import { useAppStore } from '@/stores/app'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab, StatItem } from '@/components/user/shell/types'
import SearchInput from '@/components/common/SearchInput.vue'
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import ServiceStatusStrip from '@/components/user/status/ServiceStatusStrip.vue'
import ServiceStatusTrend, { type ServiceTrendMetric } from '@/components/user/status/ServiceStatusTrend.vue'
import {
  HEALTH_BAR,
  HEALTH_DOT,
  HEALTH_TEXT,
  fillSlots,
  formatPercent,
  formatLatency
} from '@/components/user/status/serviceStatus'

const { t } = useI18n()
const appStore = useAppStore()
const shell = useShellVariant()

const RANGES: ServiceStatusRange[] = ['90m', '24h', '7d', '30d']
const LEGEND: ServiceHealth[] = ['healthy', 'warning', 'critical', 'unknown']
/** 模型多于这个数才给筛选和搜索 */
const SEARCH_THRESHOLD = 8
/** 有问题的排前面；「请求太少」不下结论，排在正常之后 */
const HEALTH_RANK: Record<ServiceHealth, number> = { critical: 0, warning: 1, healthy: 2, unknown: 3 }

const range = ref<ServiceStatusRange>(SERVICE_STATUS_DEFAULT_RANGE)
const rangeTabs = computed<SectionTab[]>(() => RANGES.map((key) => ({ key, label: t(`userUi.serviceStatus.range.${key}`) })))

// ---------- 数据 ----------
const snapshot = ref<ServiceStatusSnapshot | null>(null)
const models = ref<ServiceStatusModels | null>(null)
const loading = ref(false)
const loadError = ref(false)
let controller: AbortController | null = null
let refreshTimer: number | null = null

async function reload() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  loadError.value = false
  try {
    const [nextSnapshot, nextModels] = await loadServiceStatus(range.value, request.signal)
    if (controller !== request) return
    snapshot.value = nextSnapshot
    models.value = nextModels
    scheduleRefresh()
  } catch (error) {
    if (controller !== request) return
    const e = error as { name?: string; code?: string }
    if (e?.name === 'CanceledError' || e?.code === 'ERR_CANCELED') return
    console.error('Failed to load service status:', error)
    loadError.value = true
  } finally {
    if (controller === request) loading.value = false
  }
}

/** 数据按分钟级汇总，跟着后端给的汇总频率刷新（至少隔 1 分钟） */
function scheduleRefresh() {
  if (refreshTimer) window.clearInterval(refreshTimer)
  const seconds = Math.max(60, snapshot.value?.refresh_interval_seconds || 300)
  refreshTimer = window.setInterval(() => {
    if (!loading.value) void reload()
  }, seconds * 1000)
}

// 旧内容留着（变淡），新数据到了再替换：不收成加载占位，页面高度与滚动位置不跳
watch(range, () => void reload())
onMounted(() => {
  // 公开壳的顶栏需要站点名 / Logo；有 __APP_CONFIG__ 注入时同步命中缓存
  void appStore.fetchPublicSettings()
  void reload()
})
onBeforeUnmount(() => {
  controller?.abort()
  if (refreshTimer) window.clearInterval(refreshTimer)
})

// ---------- 各模型 ----------
const bucketSeconds = computed(() => models.value?.coverage.bucket_seconds ?? 0)

/** 有问题的在前，同档按模型名 */
const rows = computed(() => {
  const data = models.value
  if (!data) return []
  return data.items
    .map((item) => ({ ...item, slots: fillSlots(data.coverage, item.buckets) }))
    .sort((a, b) => HEALTH_RANK[a.health.overall] - HEALTH_RANK[b.health.overall] || a.model.localeCompare(b.model))
})
/** 这段时间有请求的模型占格子；一个请求都没有的（可用率为空）折叠起来 */
const activeRows = computed(() => rows.value.filter((row) => row.metrics.availability != null))
const idleRows = computed(() => rows.value.filter((row) => row.metrics.availability == null))

type HealthFilter = 'all' | 'issues' | 'healthy'
const healthFilter = ref<HealthFilter>('all')
const filterOptions = computed(() => {
  const issues = activeRows.value.filter((row) => row.health.overall === 'critical' || row.health.overall === 'warning').length
  const healthy = activeRows.value.filter((row) => row.health.overall === 'healthy').length
  return [
    { key: 'all' as const, label: t('userUi.serviceStatus.models.filter.all') },
    { key: 'issues' as const, label: t('userUi.serviceStatus.models.filter.issues', { count: issues }) },
    { key: 'healthy' as const, label: t('userUi.serviceStatus.models.filter.healthy', { count: healthy }) }
  ]
})
function matchesFilter(level: ServiceHealth): boolean {
  if (healthFilter.value === 'issues') return level === 'critical' || level === 'warning'
  if (healthFilter.value === 'healthy') return level === 'healthy'
  return true
}

const search = ref('')
const searchKeyword = computed(() => search.value.trim().toLowerCase())
function matchesSearch(model: string): boolean {
  return !searchKeyword.value || model.toLowerCase().includes(searchKeyword.value)
}
const visibleActive = computed(() => activeRows.value.filter((row) => matchesFilter(row.health.overall) && matchesSearch(row.model)))
const visibleIdle = computed(() => idleRows.value.filter((row) => matchesSearch(row.model)))
const idleExpanded = ref(false)

function healthLabel(level: ServiceHealth): string {
  return t(`userUi.serviceStatus.legend.${level}`)
}

// ---------- 数字 ----------
// 页首数字从 0 跳到位，与模型页页首数字同一条缓动（style.css 的 .count-up：1.2 秒、cubic-bezier(0.22, 1, 0.36, 1)、
// 延后 0.15 秒）。模型页是 CSS 整数计数器，这里的数带小数和单位，改用 useTransition 补间后再格式化；
// 切换时间范围时从旧值过渡到新值；系统开了「减少动态效果」时直接显示
const reducedMotion = usePreferredReducedMotion()
function useCountUp(value: () => number | null | undefined) {
  return useTransition(
    computed(() => value() ?? 0),
    { duration: 1200, delay: 150, transition: [0.22, 1, 0.36, 1], disabled: computed(() => reducedMotion.value === 'reduce') }
  )
}
const availabilityCount = useCountUp(() => snapshot.value?.metrics.availability)
const ttftCount = useCountUp(() => snapshot.value?.metrics.ttft_p50_ms)
const cacheCount = useCountUp(() => snapshot.value?.metrics.cache_hit_rate)

// 页首三个数一律墨色（2026-10-04 muqian：按健康度上色时「请求太少」是灰的，看着像失效）；健康度看下面各模型的状态
const statItems = computed<StatItem[]>(() => {
  const metrics = snapshot.value?.metrics
  return [
    {
      key: 'availability',
      label: t('userUi.serviceStatus.stats.availability'),
      value: metrics?.availability == null ? formatPercent(null) : formatPercent(availabilityCount.value)
    },
    {
      key: 'ttft',
      label: t('userUi.serviceStatus.stats.ttft'),
      value: metrics?.ttft_p50_ms == null ? formatLatency(null) : formatLatency(ttftCount.value),
      hint: metrics?.ttft_p90_ms != null ? t('userUi.serviceStatus.stats.ttftP90', { value: formatLatency(metrics.ttft_p90_ms) }) : undefined
    },
    {
      key: 'cache',
      label: t('userUi.serviceStatus.stats.cache'),
      value: metrics?.cache_hit_rate == null ? formatPercent(null) : formatPercent(cacheCount.value)
    }
  ]
})

// ---------- 整体趋势 ----------
const trendMetric = ref<ServiceTrendMetric>('availability')
const trendOptions = computed<Array<{ key: ServiceTrendMetric; label: string }>>(() => [
  { key: 'availability', label: t('userUi.serviceStatus.columns.availability') },
  { key: 'ttft', label: t('userUi.serviceStatus.columns.ttft') },
  { key: 'cache', label: t('userUi.serviceStatus.columns.cache') }
])
const trendSlots = computed(() => (snapshot.value ? fillSlots(snapshot.value.coverage, snapshot.value.trend) : []))
</script>
