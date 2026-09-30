<template>
  <!--
    渠道状态（muqian 2026-09-30）：管理员按渠道看「能不能用、快不快、缓存命中多少」，与用户站服务状态同一份统计、同一套布局。
    ① 整体：可用率 / 首字延迟 / 缓存命中率 / 请求数 + 全部渠道合计的趋势（三项切换）；不写结论句与更新时间（同服务状态页）
    ② 各渠道：多列格子（名字、平台、状态、逐段细色条、可用率、首字延迟、缓存命中率、请求数），有问题的排前面，
       点格子进该渠道的编辑页；这段时间没有请求的渠道折成一行。没选到渠道就失败的请求单独一格「没选到渠道」。
    页头右侧：时间范围、按模型（只算请求解析到该上架模型的）、按平台。
    统计全部流量（不只上架模型）；切换筛选时旧内容留在原处变淡，新数据到了再替换。
  -->
  <AppLayout>
    <template #header-actions>
      <div class="w-40">
        <Select v-model="model" :options="modelOptions" :title="t('admin.channelStatus.modelFilter')" />
      </div>
      <div class="w-32">
        <Select v-model="platform" :options="platformOptions" :title="t('admin.channelStatus.platformFilter')" />
      </div>
      <SegmentedControl v-model="range" :options="rangeOptions" :label="t('userUi.serviceStatus.range.label')" />
    </template>

    <StatusState
      v-if="loadError && !data"
      kind="error"
      :title="t('admin.channelStatus.loadFailed')"
      :action-label="t('userUi.usage.retry')"
      @action="reload"
    />
    <StatusState v-else-if="!data" kind="loading" :title="t('common.loading')" />

    <div v-else class="space-y-10 transition-opacity" :class="loading ? 'opacity-60' : ''" :aria-busy="loading ? 'true' : undefined">
      <!-- ① 整体 -->
      <section class="space-y-6" data-testid="channel-status-summary">
        <StatRow :items="statItems" />
        <div>
          <div class="mb-3 flex items-center justify-between gap-4">
            <h2 class="text-13 font-medium text-af-ink-2">{{ t('userUi.serviceStatus.trend.title') }}</h2>
            <SegmentedControl v-model="trendMetric" :options="trendOptions" :label="t('userUi.serviceStatus.trend.title')" />
          </div>
          <ServiceStatusTrend :slots="trendSlots" :bucket-seconds="bucketSeconds" :metric="trendMetric" />
        </div>
      </section>

      <!-- ② 各渠道 -->
      <SheetSection :title="t('admin.channelStatus.channels.title')" :description="t('admin.channelStatus.channels.description')">
        <template #actions>
          <SegmentedControl v-model="healthFilter" :options="filterOptions" :label="t('admin.channelStatus.channels.filter.label')" />
          <div class="w-full sm:w-48">
            <SearchInput v-model="search" compact :placeholder="t('admin.channelStatus.channels.search')" />
          </div>
        </template>

        <p v-if="rows.length === 0" class="py-10 text-center text-sm text-af-ink-3">{{ t('admin.channelStatus.channels.empty') }}</p>
        <template v-else>
          <!-- hairline 分格（不是卡片）：窄屏一列、sm 两列；有侧栏、内容区窄一截，宽屏（xl）才排 3 列 -->
          <ul v-if="visibleActive.length" class="-mx-6 grid border-t border-af-hairline sm:grid-cols-2 xl:grid-cols-3" data-testid="channel-status-channels">
            <li
              v-for="row in visibleActive"
              :key="row.account_id"
              class="min-w-0 border-b border-af-hairline sm:max-xl:[&:nth-child(2n)]:border-l xl:[&:not(:nth-child(3n+1))]:border-l"
              data-testid="channel-status-channel"
            >
              <component
                :is="row.account_id > 0 && !row.deleted ? RouterLink : 'div'"
                v-bind="row.account_id > 0 && !row.deleted ? { to: `/accounts/${row.account_id}/edit` } : {}"
                class="block px-6 py-4"
                :class="row.account_id > 0 && !row.deleted ? 'transition-colors hover:bg-af-sunken/60' : ''"
              >
                <div class="flex items-center gap-2.5">
                  <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="HEALTH_DOT[row.health.overall]" aria-hidden="true" />
                  <span class="min-w-0 flex-1 truncate text-sm font-medium text-af-ink" :title="channelName(row)">{{ channelName(row) }}</span>
                  <span class="shrink-0 text-xs" :class="HEALTH_TEXT[row.health.overall]">{{ healthLabel(row.health.overall) }}</span>
                </div>
                <p class="mt-1 flex min-w-0 items-center gap-1 text-xs text-af-ink-3">
                  <template v-if="row.account_id > 0">
                    <PlatformIcon :platform="row.platform as AccountPlatform" size="xs" />
                    <span class="truncate">{{ platformLabel(row.platform) }}</span>
                    <span v-if="row.deleted" class="shrink-0">· {{ t('admin.channelStatus.channels.deleted') }}</span>
                  </template>
                  <span v-else class="truncate">{{ t('admin.channelStatus.channels.unroutedHint') }}</span>
                </p>
                <ServiceStatusStrip
                  class="mt-3"
                  :slots="row.slots"
                  :bucket-seconds="bucketSeconds"
                  :label="t('userUi.serviceStatus.models.stripLabel', { model: channelName(row) })"
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
                  <span>
                    {{ t('admin.channelStatus.channels.requests') }}
                    <span class="text-af-ink">{{ formatNumber(row.metrics.request_count) }}</span>
                  </span>
                </p>
              </component>
            </li>
          </ul>
          <p v-else-if="healthFilter !== 'all' || visibleIdle.length === 0" class="py-10 text-center text-sm text-af-ink-3">
            {{ t('admin.channelStatus.channels.noMatch') }}
          </p>

          <!-- 这段时间没有请求的渠道：不占格子，折成一行；搜索命中时直接展开 -->
          <div v-if="healthFilter === 'all' && visibleIdle.length" class="mt-4 text-13 text-af-ink-3" data-testid="channel-status-idle">
            <p class="flex flex-wrap items-center gap-x-3 gap-y-1">
              <span>{{ t('admin.channelStatus.channels.idle', { count: visibleIdle.length }) }}</span>
              <button
                v-if="!searchKeyword"
                type="button"
                class="font-medium text-af-brand hover:text-af-brand-hover"
                :aria-expanded="idleExpanded"
                @click="idleExpanded = !idleExpanded"
              >
                {{ idleExpanded ? t('admin.channelStatus.channels.hideIdle') : t('admin.channelStatus.channels.showIdle') }}
              </button>
            </p>
            <ul v-if="idleExpanded || searchKeyword" class="mt-3 flex flex-wrap gap-x-5 gap-y-1.5 text-xs">
              <li v-for="row in visibleIdle" :key="row.account_id">
                <RouterLink :to="`/accounts/${row.account_id}/edit`" class="text-af-ink-3 hover:text-af-ink">{{ channelName(row) }}</RouterLink>
              </li>
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

      <p class="text-xs text-af-ink-4">{{ t('userUi.serviceStatus.footnote') }}</p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getChannelStatus, type ChannelStatus, type ChannelStatusRow } from '@/api/admin/channelStatus'
import type { ServiceHealth, ServiceStatusRange } from '@/api/serviceStatus'
import type { AccountPlatform } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { StatItem } from '@/components/user/shell/types'
import SegmentedControl from '@/components/user/status/SegmentedControl.vue'
import ServiceStatusStrip from '@/components/user/status/ServiceStatusStrip.vue'
import ServiceStatusTrend, { type ServiceTrendMetric } from '@/components/user/status/ServiceStatusTrend.vue'
import { HEALTH_BAR, HEALTH_DOT, HEALTH_TEXT, fillSlots, formatLatency, formatPercent } from '@/components/user/status/serviceStatus'
import { platformLabel } from '@/utils/platformLabel'
import { formatNumber } from '@/utils/format'

const { t } = useI18n()

const RANGES: ServiceStatusRange[] = ['90m', '24h', '7d', '30d']
const LEGEND: ServiceHealth[] = ['healthy', 'warning', 'critical', 'unknown']
/** 有问题的排前面；「请求太少」不下结论，排在正常之后 */
const HEALTH_RANK: Record<ServiceHealth, number> = { critical: 0, warning: 1, healthy: 2, unknown: 3 }

// ---------- 筛选 ----------
const range = ref<ServiceStatusRange>('24h')
const rangeOptions = computed(() => RANGES.map((key) => ({ key, label: t(`userUi.serviceStatus.range.${key}`) })))
const model = ref('')
const modelOptions = computed(() => [
  { value: '', label: t('admin.channelStatus.allModels') },
  ...(data.value?.models ?? []).map((id) => ({ value: id, label: id }))
])
const platform = ref('')
const platformOptions = computed(() => {
  const platforms = [...new Set(rows.value.filter((row) => row.account_id > 0).map((row) => row.platform))].sort()
  return [{ value: '', label: t('admin.channelStatus.allPlatforms') }, ...platforms.map((p) => ({ value: p, label: platformLabel(p) }))]
})

// ---------- 数据 ----------
const data = ref<ChannelStatus | null>(null)
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
    const next = await getChannelStatus(range.value, model.value, request.signal)
    if (controller !== request) return
    data.value = next
    scheduleRefresh()
  } catch (error) {
    if (controller !== request) return
    const e = error as { name?: string; code?: string }
    if (e?.name === 'CanceledError' || e?.code === 'ERR_CANCELED') return
    console.error('Failed to load channel status:', error)
    loadError.value = true
  } finally {
    if (controller === request) loading.value = false
  }
}

/** 跟着后端的汇总频率刷新（至少隔 1 分钟） */
function scheduleRefresh() {
  if (refreshTimer) window.clearInterval(refreshTimer)
  const seconds = Math.max(60, data.value?.refresh_interval_seconds || 300)
  refreshTimer = window.setInterval(() => {
    if (!loading.value) void reload()
  }, seconds * 1000)
}

// 平台在前端筛（数据里已带平台）；时间范围与模型要重新取
watch([range, model], () => void reload())
onMounted(() => void reload())
onBeforeUnmount(() => {
  controller?.abort()
  if (refreshTimer) window.clearInterval(refreshTimer)
})

// ---------- 各渠道 ----------
const bucketSeconds = computed(() => data.value?.coverage.bucket_seconds ?? 0)

function channelName(row: ChannelStatusRow): string {
  return row.account_id > 0 ? row.name || `#${row.account_id}` : t('admin.channelStatus.channels.unrouted')
}

const rows = computed(() => {
  const current = data.value
  if (!current) return []
  return current.items
    .map((item) => ({ ...item, slots: fillSlots(current.coverage, item.buckets) }))
    .sort((a, b) => HEALTH_RANK[a.health.overall] - HEALTH_RANK[b.health.overall] || channelName(a).localeCompare(channelName(b)))
})
/** 平台筛选：「没选到渠道」不属于任何平台，选了平台时不显示 */
const platformRows = computed(() => (platform.value ? rows.value.filter((row) => row.account_id > 0 && row.platform === platform.value) : rows.value))
const activeRows = computed(() => platformRows.value.filter((row) => row.metrics.request_count > 0))
const idleRows = computed(() => platformRows.value.filter((row) => row.metrics.request_count === 0))

type HealthFilter = 'all' | 'issues' | 'healthy'
const healthFilter = ref<HealthFilter>('all')
const filterOptions = computed(() => {
  const issues = activeRows.value.filter((row) => row.health.overall === 'critical' || row.health.overall === 'warning').length
  const healthy = activeRows.value.filter((row) => row.health.overall === 'healthy').length
  return [
    { key: 'all' as const, label: t('admin.channelStatus.channels.filter.all') },
    { key: 'issues' as const, label: t('admin.channelStatus.channels.filter.issues', { count: issues }) },
    { key: 'healthy' as const, label: t('admin.channelStatus.channels.filter.healthy', { count: healthy }) }
  ]
})
function matchesFilter(level: ServiceHealth): boolean {
  if (healthFilter.value === 'issues') return level === 'critical' || level === 'warning'
  if (healthFilter.value === 'healthy') return level === 'healthy'
  return true
}

const search = ref('')
const searchKeyword = computed(() => search.value.trim().toLowerCase())
function matchesSearch(row: ChannelStatusRow): boolean {
  return !searchKeyword.value || channelName(row).toLowerCase().includes(searchKeyword.value)
}
const visibleActive = computed(() => activeRows.value.filter((row) => matchesFilter(row.health.overall) && matchesSearch(row)))
const visibleIdle = computed(() => idleRows.value.filter(matchesSearch))
const idleExpanded = ref(false)

function healthLabel(level: ServiceHealth): string {
  return t(`userUi.serviceStatus.legend.${level}`)
}

// ---------- 数字 ----------
const statItems = computed<StatItem[]>(() => {
  const metrics = data.value?.metrics
  const health = data.value?.health
  return [
    {
      key: 'availability',
      label: t('userUi.serviceStatus.stats.availability'),
      value: formatPercent(metrics?.availability),
      valueClass: health ? HEALTH_TEXT[health.availability] : undefined
    },
    {
      key: 'ttft',
      label: t('userUi.serviceStatus.stats.ttft'),
      value: formatLatency(metrics?.ttft_p50_ms),
      hint: metrics?.ttft_p90_ms != null ? t('userUi.serviceStatus.stats.ttftP90', { value: formatLatency(metrics.ttft_p90_ms) }) : undefined,
      valueClass: health ? HEALTH_TEXT[health.ttft] : undefined
    },
    {
      key: 'cache',
      label: t('userUi.serviceStatus.stats.cache'),
      value: formatPercent(metrics?.cache_hit_rate),
      valueClass: health ? HEALTH_TEXT[health.cache] : undefined
    },
    { key: 'requests', label: t('admin.channelStatus.stats.requests'), value: formatNumber(metrics?.request_count ?? 0) }
  ]
})

// ---------- 整体趋势 ----------
const trendMetric = ref<ServiceTrendMetric>('availability')
const trendOptions = computed<Array<{ key: ServiceTrendMetric; label: string }>>(() => [
  { key: 'availability', label: t('userUi.serviceStatus.columns.availability') },
  { key: 'ttft', label: t('userUi.serviceStatus.columns.ttft') },
  { key: 'cache', label: t('userUi.serviceStatus.columns.cache') }
])
const trendSlots = computed(() => (data.value ? fillSlots(data.value.coverage, data.value.trend) : []))
</script>
