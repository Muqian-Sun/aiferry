<template>
  <component :is="isFullscreen ? 'div' : AppLayout" :class="isFullscreen ? 'min-h-screen bg-af-sheet p-4 md:p-6' : ''">
    <!--
      运维页（2026-10-04 重排，方案页 8ARyR9…）：不用卡片，分区靠标题和分隔线，同一个数只出现一次。
      从上到下：系统资源 → 请求（四个数 = 趋势图的切换按钮）→ 渠道 → 未恢复的告警 → 最近失败的请求 → 首字最慢的请求 → 系统日志。
    -->
    <div class="pb-12">
      <OpsToolbar
        :time-range="timeRange"
        :custom-start-time="customStartTime"
        :custom-end-time="customEndTime"
        :model="model"
        :account-id="accountId"
        :model-options="modelOptions"
        :channel-options="channelOptions"
        :loading="loading"
        :last-updated="lastUpdated"
        :auto-refresh-enabled="autoRefreshEnabled"
        :auto-refresh-countdown="autoRefreshCountdown"
        :fullscreen="isFullscreen"
        @update:time-range="onTimeRangeChange"
        @update:custom-time-range="onCustomTimeRangeChange"
        @update:model="(v) => (model = v)"
        @update:account-id="(v) => (accountId = v)"
        @refresh="fetchData"
        @open-alert-rules="showAlertRulesCard = true"
        @open-settings="showSettingsDialog = true"
        @enter-fullscreen="enterFullscreen"
        @exit-fullscreen="exitFullscreen"
      />

      <p v-if="errorMessage" class="mb-3 text-sm text-af-danger">{{ errorMessage }}</p>

      <template v-if="opsEnabled">
        <OpsSystemResources :metrics="overview?.system_metrics ?? null" :jobs="overview?.job_heartbeats ?? []" />
        <OpsRequestPanel
          :overview="overview"
          :throughput="throughputTrend?.points ?? []"
          :errors="errorTrend?.points ?? []"
          :thresholds="metricThresholds"
          :time-range="timeRange"
          :loading="loading"
        />
        <OpsChannelTable :account-id="accountId" :refresh-token="dashboardRefreshToken" @loaded="(list) => (channelOptions = list)" />
        <OpsActiveAlerts :refresh-token="dashboardRefreshToken" @open-alert-rules="showAlertRulesCard = true" />
        <OpsRecentFailures
          :params="scopeParams"
          :refresh-token="dashboardRefreshToken"
          @open-error="openErrorFromList"
          @open-all="openErrorDetails"
        />
        <OpsSlowRequests
          :params="scopeParams"
          :channels="channelOptions"
          :refresh-token="dashboardRefreshToken"
          @open-all="handleOpenRequestDetails({ title: t('admin.ops.page.slow.title'), kind: 'success', sort: 'ttft_desc' })"
        />
        <OpsSystemLogTable :time-params="timeParams" :refresh-token="dashboardRefreshToken" />
      </template>

      <!-- Settings Dialog (hidden in fullscreen mode) -->
      <template v-if="!isFullscreen">
        <OpsSettingsDialog :show="showSettingsDialog" @close="showSettingsDialog = false" @saved="onSettingsSaved" />

        <!-- 告警规则与历史告警事件放在一起 -->
        <BaseDialog :show="showAlertRulesCard" :title="t('admin.ops.alertRules.title')" width="extra-wide" @close="showAlertRulesCard = false">
          <OpsAlertRulesCard />
          <div class="mt-6">
            <OpsAlertEventsCard />
          </div>
        </BaseDialog>

        <OpsErrorDetailsModal
          :show="showErrorDetails"
          :time-range="timeRange"
          :custom-start-time="customStartTime"
          :custom-end-time="customEndTime"
          :error-type="errorDetailsType"
          :resume-state="resumeListState"
          @update:show="showErrorDetails = $event"
          @openErrorDetail="openError"
        />

        <OpsErrorDetailModal v-model:show="showErrorModal" :error-id="selectedErrorId" :error-type="errorDetailsType" :back-to-list="detailReturnTarget !== null" @back="handleBackToList" />

        <OpsRequestDetailsModal
          v-model="showRequestDetails"
          :time-range="timeRange"
          :preset="requestDetailsPreset"
          :resume-state="resumeListState"
          @openErrorDetail="openError"
        />
      </template>
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useDebounceFn, useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import {
  opsAPI,
  type OpsDashboardOverview,
  type OpsDashboardParams,
  type OpsErrorTrendResponse,
  type OpsThroughputTrendResponse,
  type OpsMetricThresholds
} from '@/api/admin/ops'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import OpsToolbar, { type OpsChannelOption } from './components/OpsToolbar.vue'
import OpsSystemResources from './components/OpsSystemResources.vue'
import OpsRequestPanel from './components/OpsRequestPanel.vue'
import OpsChannelTable from './components/OpsChannelTable.vue'
import OpsActiveAlerts from './components/OpsActiveAlerts.vue'
import OpsRecentFailures from './components/OpsRecentFailures.vue'
import OpsSlowRequests from './components/OpsSlowRequests.vue'
import OpsErrorDetailModal from './components/OpsErrorDetailModal.vue'
import OpsErrorDetailsModal from './components/OpsErrorDetailsModal.vue'
import OpsAlertEventsCard from './components/OpsAlertEventsCard.vue'
import OpsSystemLogTable from './components/OpsSystemLogTable.vue'
import OpsRequestDetailsModal, { type OpsRequestDetailsPreset } from './components/OpsRequestDetailsModal.vue'
import OpsSettingsDialog from './components/OpsSettingsDialog.vue'
import OpsAlertRulesCard from './components/OpsAlertRulesCard.vue'

const route = useRoute()
const router = useRouter()
const adminSettingsStore = useAdminSettingsStore()
const { t } = useI18n()

const opsEnabled = computed(() => adminSettingsStore.opsMonitoringEnabled)

type TimeRange = '5m' | '30m' | '1h' | '6h' | '24h' | 'custom'
const allowedTimeRanges = new Set<TimeRange>(['5m', '30m', '1h', '6h', '24h', 'custom'])

type QueryMode = 'auto' | 'raw' | 'preagg'
const allowedQueryModes = new Set<QueryMode>(['auto', 'raw', 'preagg'])

const loading = ref(true)
const errorMessage = ref('')
const lastUpdated = ref<Date | null>(new Date())

const timeRange = ref<TimeRange>('1h')
// 按模型、渠道收窄（2026-10-04 取代平台筛选）
const model = ref<string>('')
const accountId = ref<number | null>(null)
const queryMode = ref<QueryMode>('auto')
const customStartTime = ref<string | null>(null)
const customEndTime = ref<string | null>(null)
const modelOptions = ref<string[]>([])
const channelOptions = ref<OpsChannelOption[]>([])

const QUERY_KEYS = {
  timeRange: 'tr',
  model: 'model',
  accountId: 'account_id',
  queryMode: 'mode',
  fullscreen: 'fullscreen',

  // Deep links
  openErrorDetails: 'open_error_details',
  errorType: 'error_type',
  alertRuleId: 'alert_rule_id',
  openAlertRules: 'open_alert_rules'
} as const

const isApplyingRouteQuery = ref(false)
const isSyncingRouteQuery = ref(false)

// Fullscreen mode
const isFullscreen = computed(() => {
  const val = route.query[QUERY_KEYS.fullscreen]
  return val === '1' || val === 'true'
})

function exitFullscreen() {
  const nextQuery = { ...route.query }
  delete nextQuery[QUERY_KEYS.fullscreen]
  router.replace({ query: nextQuery })
}

function enterFullscreen() {
  const nextQuery = { ...route.query, [QUERY_KEYS.fullscreen]: '1' }
  router.replace({ query: nextQuery })
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isFullscreen.value) {
    exitFullscreen()
  }
}

let dashboardFetchController: AbortController | null = null
let dashboardFetchSeq = 0

function isCanceledRequest(err: unknown): boolean {
  return (
    !!err &&
    typeof err === 'object' &&
    'code' in err &&
    (err as Record<string, unknown>).code === 'ERR_CANCELED'
  )
}

function abortDashboardFetch() {
  if (dashboardFetchController) {
    dashboardFetchController.abort()
    dashboardFetchController = null
  }
}

const readQueryString = (key: string): string => {
  const value = route.query[key]
  if (typeof value === 'string') return value
  if (Array.isArray(value) && typeof value[0] === 'string') return value[0]
  return ''
}

const readQueryNumber = (key: string): number | null => {
  const raw = readQueryString(key)
  if (!raw) return null
  const n = Number.parseInt(raw, 10)
  return Number.isFinite(n) ? n : null
}

const applyRouteQueryToState = () => {
  const nextTimeRange = readQueryString(QUERY_KEYS.timeRange)
  if (nextTimeRange && allowedTimeRanges.has(nextTimeRange as TimeRange)) {
    timeRange.value = nextTimeRange as TimeRange
  }

  model.value = readQueryString(QUERY_KEYS.model) || ''
  const nextAccount = readQueryNumber(QUERY_KEYS.accountId)
  accountId.value = typeof nextAccount === 'number' && nextAccount > 0 ? nextAccount : null

  const nextMode = readQueryString(QUERY_KEYS.queryMode)
  if (nextMode && allowedQueryModes.has(nextMode as QueryMode)) {
    queryMode.value = nextMode as QueryMode
  } else {
    const fallback = adminSettingsStore.opsQueryModeDefault || 'auto'
    queryMode.value = allowedQueryModes.has(fallback as QueryMode) ? (fallback as QueryMode) : 'auto'
  }

  // Deep links
  const openRules = readQueryString(QUERY_KEYS.openAlertRules)
  if (openRules === '1' || openRules === 'true') {
    showAlertRulesCard.value = true
  }

  const ruleID = readQueryNumber(QUERY_KEYS.alertRuleId)
  if (typeof ruleID === 'number' && ruleID > 0) {
    showAlertRulesCard.value = true
  }

  const openErr = readQueryString(QUERY_KEYS.openErrorDetails)
  if (openErr === '1' || openErr === 'true') {
    const typ = readQueryString(QUERY_KEYS.errorType)
    errorDetailsType.value = typ === 'upstream' ? 'upstream' : 'request'
    showErrorDetails.value = true
  }
}

const buildQueryFromState = () => {
  const next: Record<string, any> = { ...route.query }

  Object.values(QUERY_KEYS).forEach((k) => {
    delete next[k]
  })

  if (timeRange.value !== '1h') next[QUERY_KEYS.timeRange] = timeRange.value
  if (model.value) next[QUERY_KEYS.model] = model.value
  if (accountId.value) next[QUERY_KEYS.accountId] = String(accountId.value)
  if (queryMode.value !== 'auto') next[QUERY_KEYS.queryMode] = queryMode.value

  return next
}

const syncQueryToRoute = useDebounceFn(async () => {
  if (isApplyingRouteQuery.value) return
  const nextQuery = buildQueryFromState()

  const curr = route.query as Record<string, any>
  const nextKeys = Object.keys(nextQuery)
  const currKeys = Object.keys(curr)
  const sameLength = nextKeys.length === currKeys.length
  const sameValues = sameLength && nextKeys.every((k) => String(curr[k] ?? '') === String(nextQuery[k] ?? ''))
  if (sameValues) return

  try {
    isSyncingRouteQuery.value = true
    await router.replace({ query: nextQuery })
  } finally {
    isSyncingRouteQuery.value = false
  }
}, 250)

const overview = ref<OpsDashboardOverview | null>(null)
const metricThresholds = ref<OpsMetricThresholds | null>(null)

const throughputTrend = ref<OpsThroughputTrendResponse | null>(null)
const errorTrend = ref<OpsErrorTrendResponse | null>(null)

const selectedErrorId = ref<number | null>(null)
const showErrorModal = ref(false)

const showErrorDetails = ref(false)
const errorDetailsType = ref<'request' | 'upstream'>('request')

const showRequestDetails = ref(false)
const requestDetailsPreset = ref<OpsRequestDetailsPreset>({
  title: '',
  kind: 'all',
  sort: 'created_at_desc'
})

// 记录单条错误详情来自哪个列表，便于"返回列表"时重新打开对应弹窗并保留状态。
type DetailReturnTarget = 'errorList' | 'requestList' | null
const detailReturnTarget = ref<DetailReturnTarget>(null)

// 从详情返回时，列表弹窗应保留上一次的筛选/分页状态而非重置。
const resumeListState = ref(false)

const showSettingsDialog = ref(false)
const showAlertRulesCard = ref(false)

applyRouteQueryToState()

// Auto refresh settings
const autoRefreshEnabled = ref(false)
const autoRefreshIntervalMs = ref(30000) // default 30 seconds
const autoRefreshCountdown = ref(0)

// Used to trigger child component refreshes in a single shared cadence.
const dashboardRefreshToken = ref(0)

// Countdown timer (drives auto refresh; updates every second)
const { pause: pauseCountdown, resume: resumeCountdown } = useIntervalFn(
  () => {
    if (!autoRefreshEnabled.value) return
    if (!opsEnabled.value) return
    if (loading.value) return

    if (autoRefreshCountdown.value <= 0) {
      // Fetch immediately when the countdown reaches 0.
      // fetchData() will reset the countdown to the full interval.
      fetchData()
      return
    }

    autoRefreshCountdown.value -= 1
  },
  1000,
  { immediate: false }
)

// Load ops dashboard presentation settings from backend.
async function loadDashboardAdvancedSettings() {
  try {
    const settings = await opsAPI.getAdvancedSettings()
    autoRefreshEnabled.value = settings.auto_refresh_enabled
    autoRefreshIntervalMs.value = settings.auto_refresh_interval_seconds * 1000
    autoRefreshCountdown.value = settings.auto_refresh_interval_seconds
  } catch (err) {
    console.error('[OpsDashboard] Failed to load dashboard advanced settings', err)
    autoRefreshEnabled.value = false
    autoRefreshIntervalMs.value = 30000
    autoRefreshCountdown.value = 0
  }
}

function handleOpenRequestDetails(preset?: OpsRequestDetailsPreset) {
  const basePreset: OpsRequestDetailsPreset = {
    title: t('admin.ops.requestDetails.title'),
    kind: 'all',
    sort: 'created_at_desc'
  }

  requestDetailsPreset.value = { ...basePreset, ...(preset ?? {}) }
  if (!requestDetailsPreset.value.title) requestDetailsPreset.value.title = basePreset.title
  // Ensure only one modal visible at a time.
  showErrorDetails.value = false
  showErrorModal.value = false
  showRequestDetails.value = true
}

function openErrorDetails(kind: 'request' | 'upstream') {
  errorDetailsType.value = kind
  // Ensure only one modal visible at a time.
  showRequestDetails.value = false
  showErrorModal.value = false
  showErrorDetails.value = true
}

function onTimeRangeChange(v: string | number | boolean | null) {
  if (typeof v !== 'string') return
  if (!allowedTimeRanges.has(v as TimeRange)) return
  timeRange.value = v as TimeRange
}

function onCustomTimeRangeChange(startTime: string, endTime: string) {
  customStartTime.value = startTime
  customEndTime.value = endTime
}

async function onSettingsSaved() {
  await loadDashboardAdvancedSettings()
  loadThresholds()
  fetchData()
}

function openError(id: number) {
  selectedErrorId.value = id
  // 记录来源列表，便于详情页"返回列表"。
  detailReturnTarget.value = showRequestDetails.value ? 'requestList' : showErrorDetails.value ? 'errorList' : null
  // Ensure only one modal visible at a time.
  showErrorDetails.value = false
  showRequestDetails.value = false
  showErrorModal.value = true
}

// 「最近失败的请求」里点一行：换渠道恢复的是上游错误详情，其余是请求错误详情
function openErrorFromList(id: number, type: 'request' | 'upstream') {
  errorDetailsType.value = type
  openError(id)
}

// 从单条错误详情返回其来源列表，重新打开关联弹窗（保留筛选/分页状态）。
function handleBackToList() {
  const target = detailReturnTarget.value
  resumeListState.value = true
  if (target === 'requestList') {
    showErrorModal.value = false
    showErrorDetails.value = false
    showRequestDetails.value = true
  } else if (target === 'errorList') {
    showErrorModal.value = false
    showRequestDetails.value = false
    showErrorDetails.value = true
  }
  detailReturnTarget.value = null
  // 子组件 watch 在本次 show 变化中消费 resumeState 后复位，保证下次手动打开仍会重置筛选。
  window.setTimeout(() => {
    resumeListState.value = false
  }, 0)
}

// 时间范围参数：页头选的相对范围，或自定义起止时间
const timeParams = computed<Pick<OpsDashboardParams, 'time_range' | 'start_time' | 'end_time'>>(() => {
  if (timeRange.value === 'custom') {
    if (customStartTime.value && customEndTime.value) return { start_time: customStartTime.value, end_time: customEndTime.value }
    return { time_range: '1h' }
  }
  return { time_range: timeRange.value }
})

// 时间 + 模型 + 渠道：看板、失败列表、慢请求共用
const scopeParams = computed(() => ({
  ...timeParams.value,
  model: model.value || undefined,
  account_id: accountId.value ?? undefined
}))

function buildApiParams(): OpsDashboardParams {
  return { ...scopeParams.value, mode: queryMode.value }
}

async function refreshCoreSnapshotWithCancel(fetchSeq: number, signal: AbortSignal) {
  if (!opsEnabled.value) return
  const data = await opsAPI.getDashboardSnapshotV2(buildApiParams(), { signal })
  if (fetchSeq !== dashboardFetchSeq) return
  overview.value = data.overview
  throughputTrend.value = data.throughput_trend
  errorTrend.value = data.error_trend
}

// 模型下拉：近 7 天有流量的模型（与用量页同一来源）
async function loadModelOptions() {
  const end = new Date()
  const start = new Date(end.getTime() - 6 * 24 * 60 * 60 * 1000)
  const ymd = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  try {
    const res = await adminAPI.dashboard.getModelStats({ start_date: ymd(start), end_date: ymd(end), model_source: 'requested' })
    const names = new Set((res.models || []).map((m) => m.model).filter(Boolean))
    if (model.value) names.add(model.value)
    modelOptions.value = [...names].sort()
  } catch (err) {
    console.error('[OpsDashboard] failed to load model options', err)
  }
}

function isOpsDisabledError(err: unknown): boolean {
  return (
    !!err &&
    typeof err === 'object' &&
    'code' in err &&
    typeof (err as Record<string, unknown>).code === 'string' &&
    (err as Record<string, unknown>).code === 'OPS_DISABLED'
  )
}

async function fetchData() {
  if (!opsEnabled.value) return

  abortDashboardFetch()
  dashboardFetchSeq += 1
  const fetchSeq = dashboardFetchSeq
  dashboardFetchController = new AbortController()

  loading.value = true
  errorMessage.value = ''
  try {
    await refreshCoreSnapshotWithCancel(fetchSeq, dashboardFetchController.signal)
    if (fetchSeq !== dashboardFetchSeq) return

    lastUpdated.value = new Date()

    // Trigger child component refreshes using the same cadence as the header.
    dashboardRefreshToken.value += 1

    // Reset auto refresh countdown after successful fetch
    if (autoRefreshEnabled.value) {
      autoRefreshCountdown.value = Math.floor(autoRefreshIntervalMs.value / 1000)
    }
  } catch (err) {
    if (fetchSeq !== dashboardFetchSeq || isCanceledRequest(err)) return
    if (!isOpsDisabledError(err)) {
      console.error('[ops] failed to fetch dashboard data', err)
      errorMessage.value = t('admin.ops.failedToLoadData')
    }
  } finally {
    if (fetchSeq === dashboardFetchSeq) {
      loading.value = false
    }
  }
}

watch(
  () => [timeRange.value, model.value, accountId.value, queryMode.value, customStartTime.value, customEndTime.value] as const,
  () => {
    if (isApplyingRouteQuery.value) return
    if (opsEnabled.value) {
      fetchData()
    }
    syncQueryToRoute()
  }
)

watch(
  () => route.query,
  () => {
    if (isSyncingRouteQuery.value) return

    const prevTimeRange = timeRange.value
    const prevModel = model.value
    const prevAccount = accountId.value

    isApplyingRouteQuery.value = true
    applyRouteQueryToState()
    isApplyingRouteQuery.value = false

    const changed =
      prevTimeRange !== timeRange.value || prevModel !== model.value || prevAccount !== accountId.value
    if (changed) {
      if (opsEnabled.value) {
        fetchData()
      }
    }
  }
)

onMounted(async () => {
  // Fullscreen mode: listen for ESC key
  window.addEventListener('keydown', handleKeydown)

  await adminSettingsStore.fetch()
  if (!adminSettingsStore.opsMonitoringEnabled) {
    await router.replace('/settings')
    return
  }

  // Load thresholds configuration
  loadThresholds()

  // Load auto refresh settings
  await loadDashboardAdvancedSettings()
  void loadModelOptions()

  if (opsEnabled.value) {
    await fetchData()
  }

  // Start auto refresh if enabled
  if (autoRefreshEnabled.value) {
    resumeCountdown()
  }
})

async function loadThresholds() {
  try {
    const thresholds = await opsAPI.getMetricThresholds()
    metricThresholds.value = thresholds || null
  } catch (err) {
    console.warn('[OpsDashboard] Failed to load thresholds', err)
    metricThresholds.value = null
  }
}

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  abortDashboardFetch()
  pauseCountdown()
})

// Watch auto refresh settings changes
watch(autoRefreshEnabled, (enabled) => {
  if (enabled) {
    autoRefreshCountdown.value = Math.floor(autoRefreshIntervalMs.value / 1000)
    resumeCountdown()
  } else {
    pauseCountdown()
    autoRefreshCountdown.value = 0
  }
})

// Reload auto refresh settings after settings dialog is closed
watch(showSettingsDialog, async (show) => {
  if (!show) {
    await loadDashboardAdvancedSettings()
  }
})
</script>
