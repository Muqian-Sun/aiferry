<template>
  <!--
    服务状态（原「渠道监控」，站长 2026-09-26 定名并放进导航）：回答「各模型现在能不能用、快不快」。
    ① 一句话结论 + 可用率 / 首字延迟 / 更新时间
    ② 每个模型一行：状态、逐段色条（悬停看那一段的数）、可用率、首字延迟
    ③ 全站整体趋势（可用率 / 首字延迟切换）
    数据是本站真实请求的被动统计；后端对普通用户只回这些字段（没有平台、站点流量、用户排行、缓存率）。
  -->
  <SiteShell>
    <template v-if="!disabled" #actions>
      <SectionTabs v-model="range" :tabs="rangeTabs" :label="t('userUi.serviceStatus.range.label')" />
    </template>

    <StatusState
      v-if="disabled"
      kind="empty"
      :title="t('userUi.serviceStatus.disabled.title')"
      :description="t('userUi.serviceStatus.disabled.description')"
    />
    <StatusState
      v-else-if="loadError && !models"
      kind="error"
      :title="t('userUi.serviceStatus.loadFailed')"
      :action-label="t('userUi.usage.retry')"
      @action="reload"
    />
    <StatusState v-else-if="!models" kind="loading" :title="t('userUi.status.loading')" />

    <div v-else class="space-y-10" :aria-busy="loading ? 'true' : undefined">
      <!-- ① 结论 -->
      <section data-testid="service-status-summary">
        <p class="flex items-center gap-2.5 text-base font-semibold text-af-ink">
          <span class="h-2 w-2 shrink-0 rounded-full" :class="HEALTH_DOT[headline.level]" aria-hidden="true" />
          {{ headline.text }}
        </p>
        <p class="mt-1 text-13 text-af-ink-3">
          {{ updatedText }}
          <template v-if="backfillPercent != null">
            · {{ t('userUi.serviceStatus.backfill', { percent: backfillPercent }) }}
          </template>
        </p>
        <StatRow class="mt-6" :items="statItems" />
      </section>

      <!-- ② 各模型 -->
      <SheetSection :title="t('userUi.serviceStatus.models.title')" :description="t('userUi.serviceStatus.models.description')">
        <template v-if="rows.length > SEARCH_THRESHOLD" #actions>
          <div class="w-56">
            <SearchInput v-model="search" compact :placeholder="t('userUi.serviceStatus.models.search')" />
          </div>
        </template>

        <p v-if="rows.length === 0" class="py-10 text-center text-sm text-af-ink-3">{{ t('userUi.serviceStatus.models.empty') }}</p>
        <template v-else>
          <div class="hidden grid-cols-[minmax(0,14rem)_minmax(0,1fr)_5.5rem_5.5rem] gap-x-6 border-b border-af-hairline pb-2 text-xs text-af-ink-3 sm:grid">
            <span>{{ t('userUi.serviceStatus.columns.model') }}</span>
            <span>{{ t('userUi.serviceStatus.columns.trend') }}</span>
            <span class="text-right">{{ t('userUi.serviceStatus.columns.availability') }}</span>
            <span class="text-right">{{ t('userUi.serviceStatus.columns.ttft') }}</span>
          </div>
          <ul class="divide-y divide-af-hairline" data-testid="service-status-models">
            <li
              v-for="row in visibleRows"
              :key="row.model"
              class="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-x-6 gap-y-2 py-3 sm:grid-cols-[minmax(0,14rem)_minmax(0,1fr)_5.5rem_5.5rem]"
            >
              <div class="flex min-w-0 items-center gap-2.5">
                <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="HEALTH_DOT[row.health.overall]" aria-hidden="true" />
                <span class="min-w-0">
                  <span class="block truncate text-sm font-medium text-af-ink" :title="row.label">{{ row.label }}</span>
                  <span class="block text-xs text-af-ink-3">{{ healthLabel(row.health.overall) }}</span>
                </span>
              </div>
              <ServiceStatusStrip
                class="col-span-3 row-start-2 sm:col-span-1 sm:row-start-auto"
                :slots="row.slots"
                :bucket-seconds="bucketSeconds"
                :label="t('userUi.serviceStatus.models.stripLabel', { model: row.label })"
              />
              <span class="text-right text-sm tabular-nums" :class="HEALTH_TEXT[row.health.availability]">
                {{ formatAvailability(row.metrics.availability) }}
              </span>
              <span class="text-right text-sm tabular-nums" :class="HEALTH_TEXT[row.health.ttft]">
                {{ formatLatency(row.metrics.ttft_p50_ms) }}
              </span>
            </li>
          </ul>
          <p v-if="visibleRows.length === 0" class="py-10 text-center text-sm text-af-ink-3">{{ t('userUi.serviceStatus.models.noMatch') }}</p>
          <!-- 图例 -->
          <ul class="mt-4 flex flex-wrap gap-x-5 gap-y-1.5 text-xs text-af-ink-3">
            <li v-for="level in LEGEND" :key="level" class="inline-flex items-center gap-1.5">
              <span class="h-2.5 w-2.5 rounded-[2px]" :class="HEALTH_BAR[level]" aria-hidden="true" />
              {{ t(`userUi.serviceStatus.legend.${level}`) }}
            </li>
          </ul>
        </template>
      </SheetSection>

      <!-- ③ 整体趋势 -->
      <SheetSection :title="t('userUi.serviceStatus.trend.title')">
        <template #actions>
          <div class="inline-flex rounded-lg bg-af-sunken p-1" role="tablist" :aria-label="t('userUi.serviceStatus.trend.title')">
            <button
              v-for="option in trendOptions"
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
        <ServiceStatusTrend :slots="trendSlots" :bucket-seconds="bucketSeconds" :metric="trendMetric" />
      </SheetSection>

      <p class="text-xs text-af-ink-4">{{ t('userUi.serviceStatus.footnote') }}</p>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  OTHER_MODELS,
  SERVICE_STATUS_DISABLED_REASON,
  getServiceStatusModels,
  getServiceStatusSnapshot,
  type ServiceHealth,
  type ServiceStatusModels,
  type ServiceStatusRange,
  type ServiceStatusSnapshot
} from '@/api/serviceStatus'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab, StatItem } from '@/components/user/shell/types'
import SearchInput from '@/components/common/SearchInput.vue'
import ServiceStatusStrip from '@/components/user/status/ServiceStatusStrip.vue'
import ServiceStatusTrend, { type ServiceTrendMetric } from '@/components/user/status/ServiceStatusTrend.vue'
import {
  HEALTH_BAR,
  HEALTH_DOT,
  HEALTH_TEXT,
  fillSlots,
  formatAvailability,
  formatLatency
} from '@/components/user/status/serviceStatus'

const { t, locale } = useI18n()

const RANGES: ServiceStatusRange[] = ['90m', '24h', '7d', '30d']
const LEGEND: ServiceHealth[] = ['healthy', 'warning', 'critical', 'unknown']
/** 模型多于这个数才给搜索框 */
const SEARCH_THRESHOLD = 8

const range = ref<ServiceStatusRange>('24h')
const rangeTabs = computed<SectionTab[]>(() => RANGES.map((key) => ({ key, label: t(`userUi.serviceStatus.range.${key}`) })))

// ---------- 数据 ----------
const snapshot = ref<ServiceStatusSnapshot | null>(null)
const models = ref<ServiceStatusModels | null>(null)
const loading = ref(false)
const loadError = ref(false)
const disabled = ref(false)
let controller: AbortController | null = null
let refreshTimer: number | null = null

async function reload() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  loadError.value = false
  try {
    const [nextSnapshot, nextModels] = await Promise.all([
      getServiceStatusSnapshot(range.value, request.signal),
      getServiceStatusModels(range.value, request.signal)
    ])
    if (controller !== request) return
    snapshot.value = nextSnapshot
    models.value = nextModels
    disabled.value = false
    scheduleRefresh()
  } catch (error) {
    if (controller !== request) return
    const e = error as { name?: string; code?: string; reason?: string }
    if (e?.name === 'CanceledError' || e?.code === 'ERR_CANCELED') return
    if (e?.reason === SERVICE_STATUS_DISABLED_REASON) {
      disabled.value = true
      return
    }
    console.error('Failed to load service status:', error)
    loadError.value = true
  } finally {
    if (controller === request) loading.value = false
  }
}

/** 数据按分钟级汇总，跟着站长配置的汇总频率刷新（至少隔 1 分钟） */
function scheduleRefresh() {
  if (refreshTimer) window.clearInterval(refreshTimer)
  const seconds = Math.max(60, snapshot.value?.refresh_interval_seconds || 300)
  refreshTimer = window.setInterval(() => {
    if (!loading.value) void reload()
  }, seconds * 1000)
}

watch(range, () => {
  models.value = null
  snapshot.value = null
  void reload()
})
onMounted(() => void reload())
onBeforeUnmount(() => {
  controller?.abort()
  if (refreshTimer) window.clearInterval(refreshTimer)
})

// ---------- 各模型 ----------
const bucketSeconds = computed(() => models.value?.coverage.bucket_seconds ?? 0)

function modelLabel(model: string): string {
  return model === OTHER_MODELS ? t('userUi.serviceStatus.models.other') : model
}

/** 按模型名排，「其他模型」垫底 */
const rows = computed(() => {
  const data = models.value
  if (!data) return []
  return data.items
    .map((item) => ({ ...item, label: modelLabel(item.model), slots: fillSlots(data.coverage, item.buckets) }))
    .sort((a, b) => Number(a.model === OTHER_MODELS) - Number(b.model === OTHER_MODELS) || a.label.localeCompare(b.label))
})

const search = ref('')
const visibleRows = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  return keyword ? rows.value.filter((row) => row.label.toLowerCase().includes(keyword)) : rows.value
})

function healthLabel(level: ServiceHealth): string {
  return t(`userUi.serviceStatus.legend.${level}`)
}

// ---------- 结论与数字 ----------
const headline = computed<{ level: ServiceHealth; text: string }>(() => {
  const count = (level: ServiceHealth) => rows.value.filter((row) => row.health.overall === level).length
  const critical = count('critical')
  const warning = count('warning')
  if (critical) return { level: 'critical', text: t('userUi.serviceStatus.headline.critical', { count: critical }) }
  if (warning) return { level: 'warning', text: t('userUi.serviceStatus.headline.warning', { count: warning }) }
  if (count('healthy')) return { level: 'healthy', text: t('userUi.serviceStatus.headline.healthy') }
  return { level: 'unknown', text: t('userUi.serviceStatus.headline.unknown') }
})

const statItems = computed<StatItem[]>(() => {
  const metrics = snapshot.value?.metrics
  const health = snapshot.value?.health
  return [
    {
      key: 'availability',
      label: t('userUi.serviceStatus.stats.availability'),
      value: formatAvailability(metrics?.availability),
      valueClass: health ? HEALTH_TEXT[health.availability] : undefined
    },
    {
      key: 'ttft',
      label: t('userUi.serviceStatus.stats.ttft'),
      value: formatLatency(metrics?.ttft_p50_ms),
      hint: metrics?.ttft_p90_ms != null ? t('userUi.serviceStatus.stats.ttftP90', { value: formatLatency(metrics.ttft_p90_ms) }) : undefined,
      valueClass: health ? HEALTH_TEXT[health.ttft] : undefined
    }
  ]
})

/** 没有汇总过任何数据时 data_through 等于窗口起点，不写「更新于」 */
const updatedText = computed(() => {
  const coverage = snapshot.value?.coverage
  if (!coverage || Date.parse(coverage.data_through) <= Date.parse(coverage.requested_start)) {
    return t('userUi.serviceStatus.noDataYet')
  }
  const time = new Intl.DateTimeFormat(locale.value || undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(
    new Date(coverage.data_through)
  )
  return t('userUi.serviceStatus.updatedAt', { time })
})

const backfillPercent = computed(() => snapshot.value?.coverage.backfill_percent ?? null)

// ---------- 整体趋势 ----------
const trendMetric = ref<ServiceTrendMetric>('availability')
const trendOptions = computed<Array<{ key: ServiceTrendMetric; label: string }>>(() => [
  { key: 'availability', label: t('userUi.serviceStatus.columns.availability') },
  { key: 'ttft', label: t('userUi.serviceStatus.columns.ttft') }
])
const trendSlots = computed(() => (snapshot.value ? fillSlots(snapshot.value.coverage, snapshot.value.trend) : []))
</script>
