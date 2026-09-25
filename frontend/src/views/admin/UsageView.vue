<template>
  <!--
    用量（A7）：区间数字摘要 + 页签（明细 · 错误 · 排行 · 分析）。
    时间范围、刷新、导出 / 清理在页头，作用于整页；筛选对四个页签都生效（错误页签换成错误相关字段）。
    图表只在「分析」里，全部单色；不套卡片，区块之间只用 hairline 分隔。
  -->
  <AppLayout>
    <template #header-actions>
      <DateRangePicker
        v-model:start-date="startDate"
        v-model:end-date="endDate"
        @change="onDateRangeChange"
      />
      <button
        type="button"
        class="btn btn-ghost btn-md px-2.5"
        :title="t('common.refresh')"
        :aria-label="t('common.refresh')"
        data-testid="usage-refresh"
        @click="refreshData"
      >
        <Icon name="refresh" size="md" />
      </button>
      <PopoverMenu width-class="w-44">
        <template #trigger="{ open }">
          <button
            type="button"
            class="btn btn-ghost btn-md px-2.5"
            :class="open ? 'bg-af-sunken text-af-ink' : ''"
            :title="t('common.more')"
            :aria-label="t('common.more')"
            data-testid="usage-tools"
          >
            <Icon name="more" size="md" />
          </button>
        </template>
        <MenuItem icon="download" :disabled="exporting" data-testid="usage-export" @click="exportToExcel">
          {{ exporting ? t('usage.exporting') : t('usage.exportExcel') }}
        </MenuItem>
        <MenuItem divider />
        <MenuItem icon="trash" danger data-testid="usage-cleanup" @click="openCleanupDialog">
          {{ t('admin.usage.cleanup.button') }}
        </MenuItem>
      </PopoverMenu>
    </template>

    <div class="space-y-6">
      <!-- 区间摘要：统计接口失败就不出现，不摆一排 0 -->
      <StatRow v-if="summaryItems" :items="summaryItems" data-testid="usage-summary" />

      <div>
        <SectionTabs :model-value="activeTab" :tabs="detailTabs" :label="t('nav.usage')" @update:model-value="onTabChange" />

        <UsageFilters v-model="filters" ref="usageFiltersRef" flat :mode="activeTab" :start-date="startDate" :end-date="endDate" :model-options="modelNameOptions" @change="applyFilters" @reset="resetFilters">
          <template #after-reset>
            <div v-if="activeTab === 'usage' || activeTab === 'errors'" class="relative" ref="columnDropdownRef">
              <button
                data-testid="usage-column-settings"
                @click="showColumnDropdown = !showColumnDropdown"
                class="btn btn-secondary px-2 md:px-3"
                :title="t('admin.users.columnSettings')"
              >
                <Icon name="columns" size="sm" class="md:mr-1.5" />
                <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
              </button>
              <div
                v-if="showColumnDropdown"
                class="absolute right-0 top-full z-50 mt-1 max-h-80 w-48 overflow-y-auto rounded-lg border border-af-hairline bg-af-sheet py-1 shadow-lg"
              >
                <button
                  v-for="col in currentToggleableColumns"
                  :key="col.key"
                  :data-testid="`usage-column-toggle-${col.key}`"
                  @click="toggleCurrentColumn(col.key)"
                  class="flex w-full items-center justify-between px-4 py-2 text-left text-sm text-af-ink-2 hover:bg-af-sunken"
                >
                  <span>{{ col.label }}</span>
                  <Icon
                    v-if="isCurrentColumnVisible(col.key)"
                    name="check"
                    size="sm"
                    class="text-af-brand"
                    :stroke-width="2"
                  />
                </button>
              </div>
            </div>
          </template>
        </UsageFilters>

        <div v-show="activeTab === 'usage'" class="border-t border-af-hairline">
          <UsageTable
            :data="usageLogs"
            :loading="loading"
            :columns="visibleColumns"
            :server-side-sort="true"
            :default-sort-key="'created_at'"
            :default-sort-order="'desc'"
            @sort="handleSort"
            @userClick="handleUserClick"
            @ipGeoBatchFailed="handleIpGeoBatchFailed"
          />
          <Pagination v-if="pagination.total > 0" :page="pagination.page" :total="pagination.total" :page-size="pagination.page_size" @update:page="handlePageChange" @update:pageSize="handlePageSizeChange" />
        </div>
        <div v-show="activeTab === 'errors'" class="border-t border-af-hairline">
          <OpsErrorLogTable
            flat
            :rows="errRows" :total="errTotal" :loading="errLoading"
            :page="errPage" :page-size="errPageSize"
            :visible-column-keys="errVisibleColumnKeys"
            user-clickable
            @userClick="handleUserClick"
            @openErrorDetail="openError"
            @sort="onErrSort"
            @update:page="onErrPage"
            @update:pageSize="onErrPageSize"
            @ipGeoBatchFailed="handleIpGeoBatchFailed" />
        </div>
        <!-- 懒挂载：首次切到该 tab 才请求排行数据，之后随筛选自动刷新 -->
        <div v-if="rankingMounted" v-show="activeTab === 'ranking'" class="border-t border-af-hairline">
          <UserTokenRanking
            ref="rankingRef"
            :start-date="startDate"
            :end-date="endDate"
            :filters="breakdownFilters"
            :model="filters.model"
            @select-user="handleRankingSelectUser"
          />
        </div>
        <!-- 分析：趋势 / 模型分布 / 端点分布，随时间范围与筛选变；进页就拉（模型筛选的候选项来自模型分布） -->
        <div v-show="activeTab === 'analysis'" class="space-y-8 border-t border-af-hairline pt-6" data-testid="usage-analysis">
          <SheetSection :title="t('admin.dashboard.usageTrend')" data-testid="usage-trend">
            <template #actions>
              <div class="w-28">
                <Select v-model="granularity" :options="granularityOptions" :title="t('admin.dashboard.granularity')" @change="loadChartData" />
              </div>
              <div class="inline-flex rounded-lg bg-af-sunken p-1" role="tablist" :aria-label="t('admin.dashboard.usageTrend')">
                <button
                  v-for="tab in trendTabs"
                  :key="tab.key"
                  type="button"
                  role="tab"
                  class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
                  :class="trendMetric === tab.key ? 'bg-af-sheet text-af-ink' : 'text-af-ink-3 hover:text-af-ink-2'"
                  :aria-selected="trendMetric === tab.key"
                  @click="trendMetric = tab.key"
                >
                  {{ tab.label }}
                </button>
              </div>
            </template>
            <UsageMetricTrend :trend-data="trendFilled" :metric="trendMetric" :loading="chartsLoading" />
          </SheetSection>
          <SheetSection>
            <ModelDistributionChart
              v-model:source="modelDistributionSource"
              v-model:metric="modelDistributionMetric"
              :model-stats="requestedModelStats"
              :upstream-model-stats="upstreamModelStats"
              :mapping-model-stats="mappingModelStats"
              :loading="modelStatsLoading"
              :show-source-toggle="true"
              :show-metric-toggle="true"
              :start-date="startDate"
              :end-date="endDate"
              :filters="breakdownFilters"
              :load-user-breakdown="getUserBreakdown"
            />
          </SheetSection>
          <SheetSection>
            <EndpointDistributionChart
              v-model:source="endpointDistributionSource"
              v-model:metric="endpointDistributionMetric"
              :endpoint-stats="inboundEndpointStats"
              :upstream-endpoint-stats="upstreamEndpointStats"
              :endpoint-path-stats="endpointPathStats"
              :loading="endpointStatsLoading"
              :show-source-toggle="true"
              :show-metric-toggle="true"
              :title="t('usage.endpointDistribution')"
              :start-date="startDate"
              :end-date="endDate"
              :filters="breakdownFilters"
              :load-user-breakdown="getUserBreakdown"
            />
          </SheetSection>
        </div>
      </div>
      <OpsErrorDetailModal v-model:show="showErrorModal" :error-id="selectedErrorId" :error-type="'request'" />
    </div>
  </AppLayout>
  <UsageExportProgress :show="exportProgress.show" :progress="exportProgress.progress" :current="exportProgress.current" :total="exportProgress.total" :estimated-time="exportProgress.estimatedTime" @cancel="cancelExport" />
  <UsageCleanupDialog
    :show="cleanupDialogVisible"
    :filters="filters"
    :start-date="startDate"
    :end-date="endDate"
    @close="cleanupDialogVisible = false"
  />
  <!-- Balance history modal triggered from usage table user click -->
  <UserBalanceHistoryModal
    :show="showBalanceHistoryModal"
    :user="balanceHistoryUser"
    :hide-actions="true"
    @close="showBalanceHistoryModal = false; balanceHistoryUser = null"
  />
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'; import { adminAPI } from '@/api/admin'; import { adminUsageAPI } from '@/api/admin/usage'; import { getUserBreakdown } from '@/api/admin/dashboard'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatReasoningEffort } from '@/utils/format'
import { resolveUsageRequestType, requestTypeToLegacyStream } from '@/utils/usageRequestType'
import AppLayout from '@/components/layout/AppLayout.vue'; import Pagination from '@/components/common/Pagination.vue'; import Select from '@/components/common/Select.vue'; import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageFilters from '@/components/admin/usage/UsageFilters.vue'
import UsageTable from '@/components/usage/UsageTable.vue'; import UsageExportProgress from '@/components/admin/usage/UsageExportProgress.vue'
import UserTokenRanking from '@/components/admin/usage/UserTokenRanking.vue'
import UsageCleanupDialog from '@/components/admin/usage/UsageCleanupDialog.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import OpsErrorLogTable from '@/views/admin/ops/components/OpsErrorLogTable.vue'
import OpsErrorDetailModal from '@/views/admin/ops/components/OpsErrorDetailModal.vue'
import { listErrorLogs } from '@/api/admin/ops'
import type { OpsErrorLog } from '@/api/admin/ops'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import Icon from '@/components/icons/Icon.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import type { SectionTab, StatItem } from '@/components/user/shell/types'
import UsageMetricTrend, { type UsageTrendMetric } from '@/components/user/usage/UsageMetricTrend.vue'
import { MenuItem, PopoverMenu } from '@/components/admin/list'
import { fillTrendBuckets, trendBucketKeys, type TrendGranularity } from '@/utils/trendBuckets'
import type { AdminUsageLog, TrendDataPoint, ModelStat, EndpointStat, AdminUser } from '@/types'; import type { AdminUsageStatsResponse, AdminUsageQueryParams } from '@/api/admin/usage'

const { t } = useI18n()
const appStore = useAppStore()
type DistributionMetric = 'tokens' | 'actual_cost'
type EndpointSource = 'inbound' | 'upstream' | 'path'
type ModelDistributionSource = 'requested' | 'upstream' | 'mapping'
const route = useRoute()
const usageStats = ref<AdminUsageStatsResponse | null>(null); const usageLogs = ref<AdminUsageLog[]>([]); const loading = ref(false); const exporting = ref(false)
const trendData = ref<TrendDataPoint[]>([]); const requestedModelStats = ref<ModelStat[]>([]); const upstreamModelStats = ref<ModelStat[]>([]); const mappingModelStats = ref<ModelStat[]>([]); const chartsLoading = ref(false); const modelStatsLoading = ref(false); const granularity = ref<TrendGranularity>('hour')
const modelDistributionMetric = ref<DistributionMetric>('tokens')
const modelDistributionSource = ref<ModelDistributionSource>('requested')
const loadedModelSources = reactive<Record<ModelDistributionSource, boolean>>({
  requested: false,
  upstream: false,
  mapping: false,
})
const endpointDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionSource = ref<EndpointSource>('inbound')
const inboundEndpointStats = ref<EndpointStat[]>([])
const upstreamEndpointStats = ref<EndpointStat[]>([])
const endpointPathStats = ref<EndpointStat[]>([])
const endpointStatsLoading = ref(false)
let abortController: AbortController | null = null; let exportAbortController: AbortController | null = null
let chartReqSeq = 0
let statsReqSeq = 0
let modelStatsReqSeq = 0
const exportProgress = reactive({ show: false, progress: 0, current: 0, total: 0, estimatedTime: '' })
const cleanupDialogVisible = ref(false)
// Balance history modal state
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)

const breakdownFilters = computed(() => {
  const f: Record<string, any> = {}
  if (filters.value.user_id) f.user_id = filters.value.user_id
  if (filters.value.api_key_id) f.api_key_id = filters.value.api_key_id
  if (filters.value.account_id) f.account_id = filters.value.account_id
  if (filters.value.request_type != null) f.request_type = filters.value.request_type
  if (filters.value.native_compaction_v2 != null) f.native_compaction_v2 = filters.value.native_compaction_v2
  if (filters.value.billing_type != null) f.billing_type = filters.value.billing_type
  return f
})

// ---------- 区间摘要 ----------
const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}
const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(2)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(2)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(2)}K`
  return value.toLocaleString()
}
const formatCost = (value: number): string => {
  if (value >= 1000) return `$${(value / 1000).toFixed(2)}K`
  if (value >= 1) return `$${value.toFixed(2)}`
  if (value >= 0.01) return `$${value.toFixed(3)}`
  return `$${value.toFixed(4)}`
}
const formatDuration = (ms: number): string => (ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`)

/** 与概览同一套口径：缓存命中率 = 缓存读 ÷（输入 + 缓存读 + 缓存写）；没有输入侧 token 时显示「—」 */
const summaryItems = computed<StatItem[] | null>(() => {
  const s = usageStats.value
  if (!s) return null
  const input = toFiniteNumber(s.total_input_tokens)
  const cacheRead = toFiniteNumber(s.total_cache_read_tokens)
  const cacheCreation = toFiniteNumber(s.total_cache_creation_tokens)
  const promptTokens = input + cacheRead + cacheCreation
  const hitShare = promptTokens > 0 ? cacheRead / promptTokens : null
  return [
    { key: 'requests', label: t('admin.dashboard.requests'), value: toFiniteNumber(s.total_requests).toLocaleString() },
    {
      key: 'tokens',
      label: t('admin.dashboard.tokens'),
      value: formatTokens(toFiniteNumber(s.total_tokens)),
      hint: `${t('usage.in')} ${formatTokens(input)} · ${t('usage.out')} ${formatTokens(toFiniteNumber(s.total_output_tokens))}`
    },
    {
      key: 'cacheHitRate',
      label: t('admin.dashboard.cacheHitRate'),
      value: hitShare === null ? '—' : `${(hitShare * 100).toFixed(hitShare > 0 && hitShare < 0.1 ? 1 : 0)}%`
    },
    {
      key: 'charged',
      label: t('admin.dashboard.charged'),
      value: formatCost(toFiniteNumber(s.total_actual_cost)),
      hint: t('admin.dashboard.standardHint', { amount: formatCost(toFiniteNumber(s.total_cost)) })
    },
    { key: 'accountCost', label: t('admin.dashboard.accountCost'), value: formatCost(toFiniteNumber(s.total_account_cost)) },
    { key: 'latency', label: t('admin.dashboard.avgResponse'), value: formatDuration(toFiniteNumber(s.average_duration_ms)) }
  ]
})

// ---------- 分析：用量趋势 ----------
const trendMetric = ref<UsageTrendMetric>('tokens')
const trendTabs = computed<Array<{ key: UsageTrendMetric; label: string }>>(() => [
  { key: 'tokens', label: t('admin.dashboard.tokens') },
  { key: 'requests', label: t('admin.dashboard.requests') },
  { key: 'cost', label: t('admin.dashboard.cost') }
])
const trendFilled = computed(() =>
  fillTrendBuckets(
    trendData.value,
    trendBucketKeys(filters.value.start_date || startDate.value, filters.value.end_date || endDate.value, granularity.value)
  )
)

const modelNameOptions = computed(() =>
  Array.from(new Set(requestedModelStats.value.map((m) => m.model).filter(Boolean))).sort()
)

const handleUserClick = async (userId: number) => {
  try {
    const user = await adminAPI.users.getById(userId, true)
    balanceHistoryUser.value = user
    showBalanceHistoryModal.value = true
  } catch {
    appStore.showError(t('admin.usage.failedToLoadUser'))
  }
}

// Drill down from the per-user token ranking: scope the whole usage view to
// that user and jump to the usage-detail tab so the drill-down is visible.
const handleRankingSelectUser = (userId: number, email: string) => {
  filters.value = { ...filters.value, user_id: userId }
  usageFiltersRef.value?.setUserKeyword?.(email || '')
  activeTab.value = 'usage'
  applyFilters()
}

const granularityOptions = computed(() => [{ value: 'day', label: t('admin.dashboard.day') }, { value: 'hour', label: t('admin.dashboard.hour') }])
// Use local timezone to avoid UTC timezone issues
const formatLD = (d: Date) => {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLD(start),
    end: formatLD(end)
  }
}
const getGranularityForRange = (start: string, end: string): TrendGranularity => {
  const startTime = new Date(`${start}T00:00:00`).getTime()
  const endTime = new Date(`${end}T00:00:00`).getTime()
  const daysDiff = Math.ceil((endTime - startTime) / (1000 * 60 * 60 * 24))
  return daysDiff <= 1 ? 'hour' : 'day'
}
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start); const endDate = ref(defaultRange.end)
const filters = ref<AdminUsageQueryParams>({ user_id: undefined, model: undefined, request_type: undefined, native_compaction_v2: null, billing_type: null, start_date: startDate.value, end_date: endDate.value })
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const getSingleQueryValue = (value: string | null | Array<string | null> | undefined): string | undefined => {
  if (Array.isArray(value)) return value.find((item): item is string => typeof item === 'string' && item.length > 0)
  return typeof value === 'string' && value.length > 0 ? value : undefined
}

const getNumericQueryValue = (value: string | null | Array<string | null> | undefined): number | undefined => {
  const raw = getSingleQueryValue(value)
  if (!raw) return undefined
  const parsed = Number(raw)
  return Number.isFinite(parsed) ? parsed : undefined
}

const applyRouteQueryFilters = () => {
  const queryStartDate = getSingleQueryValue(route.query.start_date)
  const queryEndDate = getSingleQueryValue(route.query.end_date)
  const queryUserId = getNumericQueryValue(route.query.user_id)

  if (queryStartDate) {
    startDate.value = queryStartDate
  }
  if (queryEndDate) {
    endDate.value = queryEndDate
  }

  filters.value = {
    ...filters.value,
    user_id: queryUserId,
    start_date: startDate.value,
    end_date: endDate.value
  }
  granularity.value = getGranularityForRange(startDate.value, endDate.value)
}

const loadRouteUserFilterLabel = async () => {
  const requestedUserId = filters.value.user_id
  if (!requestedUserId) return
  const userSearchRevision = usageFiltersRef.value?.getUserSearchRevision?.()

  const routeUserFilterIsCurrent = () => (
    filters.value.user_id === requestedUserId
    && usageFiltersRef.value?.getUserSearchRevision?.() === userSearchRevision
  )

  try {
    const user = await adminAPI.users.getById(requestedUserId, true)
    if (!routeUserFilterIsCurrent()) return
    usageFiltersRef.value?.setUserKeyword?.(user.email || String(requestedUserId))
  } catch {
    if (!routeUserFilterIsCurrent()) return
    usageFiltersRef.value?.setUserKeyword?.(String(requestedUserId))
  }
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  filters.value = {
    ...filters.value,
    start_date: range.startDate,
    end_date: range.endDate
  }
  granularity.value = getGranularityForRange(range.startDate, range.endDate)
  applyFilters()
}

const buildUsageListParams = (
  page: number,
  pageSize: number,
  exactTotal: boolean
): AdminUsageQueryParams => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
  return {
    page,
    page_size: pageSize,
    exact_total: exactTotal,
    ...filters.value,
    stream: legacyStream === null ? undefined : legacyStream,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  }
}

const loadLogs = async () => {
  abortController?.abort(); const c = new AbortController(); abortController = c; loading.value = true
  try {
    const res = await adminAPI.usage.list(
      buildUsageListParams(pagination.page, pagination.page_size, false),
      { signal: c.signal }
    )
    if(!c.signal.aborted) { usageLogs.value = res.items; pagination.total = res.total }
  } catch (error: any) { if(error?.name !== 'AbortError') console.error('Failed to load usage logs:', error) } finally { if(abortController === c) loading.value = false }
}
const loadStats = async (force = false) => {
  const seq = ++statsReqSeq
  endpointStatsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const s = await adminAPI.usage.getStats({
      ...filters.value,
      stream: legacyStream === null ? undefined : legacyStream,
      ...(force ? { nocache: 1 } : {}),
    })
    if (seq !== statsReqSeq) return
    usageStats.value = s
    inboundEndpointStats.value = s.endpoints || []
    upstreamEndpointStats.value = s.upstream_endpoints || []
    endpointPathStats.value = s.endpoint_paths || []
  } catch (error) {
    if (seq !== statsReqSeq) return
    console.error('Failed to load usage stats:', error)
    inboundEndpointStats.value = []
    upstreamEndpointStats.value = []
    endpointPathStats.value = []
  } finally {
    if (seq === statsReqSeq) endpointStatsLoading.value = false
  }
}

// 失效模型统计缓存:仅标记需要重取,保留旧数据直到新数据到达(避免刷新时图表闪空)。
const invalidateModelStatsCache = () => {
  loadedModelSources.requested = false
  loadedModelSources.upstream = false
  loadedModelSources.mapping = false
}

const loadModelStats = async (source: ModelDistributionSource, force = false) => {
  if (!force && loadedModelSources[source]) {
    return
  }

  const seq = ++modelStatsReqSeq
  modelStatsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const baseParams = {
      start_date: filters.value.start_date || startDate.value,
      end_date: filters.value.end_date || endDate.value,
      user_id: filters.value.user_id,
      model: filters.value.model,
      api_key_id: filters.value.api_key_id,
      account_id: filters.value.account_id,
      request_type: requestType,
      stream: legacyStream === null ? undefined : legacyStream,
      native_compaction_v2: filters.value.native_compaction_v2,
      billing_type: filters.value.billing_type,
	  upstream_model_mismatch: filters.value.upstream_model_mismatch,
    }

    const response = await adminAPI.dashboard.getModelStats({ ...baseParams, model_source: source })

    if (seq !== modelStatsReqSeq) return

    const models = response.models || []
    if (source === 'requested') {
      requestedModelStats.value = models
    } else if (source === 'upstream') {
      upstreamModelStats.value = models
    } else {
      mappingModelStats.value = models
    }
    loadedModelSources[source] = true
  } catch (error) {
    if (seq !== modelStatsReqSeq) return
    console.error('Failed to load model stats:', error)
    if (source === 'requested') {
      requestedModelStats.value = []
    } else if (source === 'upstream') {
      upstreamModelStats.value = []
    } else {
      mappingModelStats.value = []
    }
    loadedModelSources[source] = false
  } finally {
    if (seq === modelStatsReqSeq) modelStatsLoading.value = false
  }
}

const loadChartData = async () => {
  const seq = ++chartReqSeq
  chartsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const snapshot = await adminAPI.dashboard.getSnapshotV2({
      start_date: filters.value.start_date || startDate.value,
      end_date: filters.value.end_date || endDate.value,
      granularity: granularity.value,
      user_id: filters.value.user_id,
      model: filters.value.model,
      api_key_id: filters.value.api_key_id,
      account_id: filters.value.account_id,
      request_type: requestType,
      stream: legacyStream === null ? undefined : legacyStream,
      native_compaction_v2: filters.value.native_compaction_v2,
      billing_type: filters.value.billing_type,
	  upstream_model_mismatch: filters.value.upstream_model_mismatch,
      include_stats: false,
      include_trend: true,
      include_model_stats: false,
      include_users_trend: false
    })
    if (seq !== chartReqSeq) return
    trendData.value = snapshot.trend || []
  } catch (error) { console.error('Failed to load chart data:', error) } finally { if (seq === chartReqSeq) chartsLoading.value = false }
}
const applyFilters = () => {
  pagination.page = 1
  invalidateModelStatsCache()
  loadLogs()
  loadStats()
  loadModelStats(modelDistributionSource.value, true)
  loadChartData()
  errPage.value = 1
  if (activeTab.value === 'errors') {
    loadAdminErrors()
  } else {
    errRows.value = []
  }
}
const refreshData = () => {
  invalidateModelStatsCache()
  loadLogs()
  loadStats(true)
  loadModelStats(modelDistributionSource.value, true)
  loadChartData()
  if (activeTab.value === 'errors') loadAdminErrors()
  if (rankingMounted.value) rankingRef.value?.reload()
}
const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = { start_date: startDate.value, end_date: endDate.value, request_type: undefined, native_compaction_v2: null, billing_type: null, billing_mode: undefined }
  granularity.value = getGranularityForRange(startDate.value, endDate.value)
  applyFilters()
}
const handlePageChange = (p: number) => { pagination.page = p; loadLogs() }
const handlePageSizeChange = (s: number) => { pagination.page_size = s; pagination.page = 1; loadLogs() }
const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadLogs()
}

const handleIpGeoBatchFailed = () => {
  appStore.showError(t('usage.ipGeo.batchFailed'))
}
const cancelExport = () => exportAbortController?.abort()
const openCleanupDialog = () => { cleanupDialogVisible.value = true }
const getRequestTypeLabel = (log: AdminUsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'cyber') return t('usage.cyber')
  if (requestType === 'live') return t('usage.live')
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

const exportToExcel = async () => {
  if (exporting.value) return; exporting.value = true; exportProgress.show = true
  const c = new AbortController(); exportAbortController = c
  try {
    let p = 1; let total = pagination.total; let exportedCount = 0
    const XLSX = await import('xlsx')
    const headers = [
      t('usage.time'), t('admin.usage.user'), t('usage.apiKeyFilter'),
      t('admin.usage.account'), t('usage.requestedModel'), t('usage.sentUpstreamModel'), t('usage.upstreamResponseModel'), t('usage.upstreamModelMismatch'), t('usage.requestedReasoningEffort'), t('usage.reasoningEffort'),
      t('usage.inboundEndpoint'), t('usage.upstreamEndpoint'),
      t('usage.type'),
      t('admin.usage.inputTokens'), t('admin.usage.outputTokens'),
      t('admin.usage.cacheReadTokens'), t('admin.usage.cacheCreationTokens'),
      t('admin.usage.inputCost'), t('admin.usage.outputCost'),
      t('admin.usage.cacheReadCost'), t('admin.usage.cacheCreationCost'),
      t('usage.rate'), t('usage.accountMultiplier'), t('usage.original'), t('usage.userBilled'), t('usage.accountBilled'),
      t('usage.firstToken'), t('usage.duration'),
      t('admin.usage.requestId'), t('admin.usage.upstreamRequestId'), t('usage.userAgent'), t('admin.usage.ipAddress')
    ]
    const ws = XLSX.utils.aoa_to_sheet([headers])
    while (true) {
      const res = await adminUsageAPI.list(
        buildUsageListParams(p, 100, true),
        { signal: c.signal }
      )
      if (c.signal.aborted) break; if (p === 1) { total = res.total; exportProgress.total = total }
      const rows = (res.items || []).map((log: AdminUsageLog) => [
        log.created_at, log.user?.email || '', log.api_key?.name || '', log.account?.name || '', log.model,
        log.upstream_model || log.model, log.upstream_response_model || '', log.upstream_model_mismatch == null ? '' : t(log.upstream_model_mismatch ? 'common.yes' : 'common.no'), formatReasoningEffort(log.reasoning_effort), formatReasoningEffort(log.upstream_reasoning_effort || log.reasoning_effort),
        log.inbound_endpoint || '', log.upstream_endpoint || '', getRequestTypeLabel(log),
        log.input_tokens, log.output_tokens, log.cache_read_tokens, log.cache_creation_tokens,
        log.input_cost?.toFixed(6) || '0.000000', log.output_cost?.toFixed(6) || '0.000000',
        log.cache_read_cost?.toFixed(6) || '0.000000', log.cache_creation_cost?.toFixed(6) || '0.000000',
        log.rate_multiplier?.toPrecision(4) || '1.00', (log.account_rate_multiplier ?? 1).toPrecision(4),
        log.total_cost?.toFixed(6) || '0.000000', log.actual_cost?.toFixed(6) || '0.000000',
        ((log.total_cost ?? 0) * (log.account_rate_multiplier ?? 1)).toFixed(6), log.first_token_ms ?? '', log.duration_ms,
        log.request_id || '', log.upstream_request_id || '', log.user_agent || '', log.ip_address || ''
      ])
      if (rows.length) {
        XLSX.utils.sheet_add_aoa(ws, rows, { origin: -1 })
      }
      exportedCount += rows.length
      exportProgress.current = exportedCount
      exportProgress.progress = total > 0 ? Math.min(100, Math.round(exportedCount / total * 100)) : 0
      if (exportedCount >= total || res.items.length < 100) break; p++
    }
    if(!c.signal.aborted) {
      const wb = XLSX.utils.book_new()
      XLSX.utils.book_append_sheet(wb, ws, 'Usage')
      saveAs(new Blob([XLSX.write(wb, { bookType: 'xlsx', type: 'array' })], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }), `usage_${filters.value.start_date}_to_${filters.value.end_date}.xlsx`)
      appStore.showSuccess(t('usage.exportSuccess'))
    }
  } catch (error) { console.error('Failed to export:', error); appStore.showError('Export Failed') }
  finally { if(exportAbortController === c) { exportAbortController = null; exporting.value = false; exportProgress.show = false } }
}

// Column visibility
const ALWAYS_VISIBLE = ['user', 'created_at']
const DEFAULT_HIDDEN_COLUMNS = ['reasoning_effort', 'request_id', 'upstream_request_id', 'user_agent']
const HIDDEN_COLUMNS_KEY = 'usage-hidden-columns'
const HIDDEN_COLUMNS_VERSION_KEY = 'usage-hidden-columns-version'
// 隐藏列版本链：每级只把当级新增列加入隐藏集，不重置用户已显式打开的列。
const HIDDEN_COLUMNS_PREV_VERSION = 'request-id-hidden-by-default'
const HIDDEN_COLUMNS_CURRENT_VERSION = 'upstream-request-id-hidden-by-default'

const allColumns = computed(() => [
  { key: 'user', label: t('admin.usage.user'), sortable: false },
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'account', label: t('admin.usage.account'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'latency', label: t('usage.latency'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'request_id', label: t('admin.usage.requestId'), sortable: false },
  { key: 'upstream_request_id', label: t('admin.usage.upstreamRequestId'), sortable: false },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
  { key: 'ip_address', label: t('admin.usage.ipAddress'), sortable: false }
])

const hiddenColumns = reactive<Set<string>>(new Set())

const toggleableColumns = computed(() =>
  allColumns.value.filter(col => !ALWAYS_VISIBLE.includes(col.key))
)

const visibleColumns = computed(() =>
  allColumns.value.filter(col =>
    ALWAYS_VISIBLE.includes(col.key) || !hiddenColumns.has(col.key)
  )
)

const isColumnVisible = (key: string) => !hiddenColumns.has(key)

const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
    localStorage.setItem(HIDDEN_COLUMNS_VERSION_KEY, HIDDEN_COLUMNS_CURRENT_VERSION)
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

// ---- 错误请求 tab 列设置(与用量明细同机制,独立存储) ----
const ERR_ALWAYS_VISIBLE = ['user', 'status', 'created_at', 'actions']
const ERR_DEFAULT_HIDDEN_COLUMNS = ['user_agent']
const ERR_HIDDEN_COLUMNS_KEY = 'usage-error-hidden-columns'

// key 集合须与 OpsErrorLogTable 内部 allColumns 一致
const errAllColumns = computed(() => [
  { key: 'user', label: t('admin.ops.errorLog.user') },
  { key: 'api_key', label: t('admin.ops.errorLog.apiKey') },
  { key: 'account', label: t('admin.ops.errorLog.account') },
  { key: 'platform', label: t('admin.ops.errorLog.platform') },
  { key: 'model', label: t('admin.ops.errorLog.model') },
  { key: 'endpoint', label: t('admin.ops.errorLog.endpoint') },
  { key: 'type', label: t('admin.ops.errorLog.type') },
  { key: 'category', label: t('usage.errors.category') },
  { key: 'status', label: t('admin.ops.errorLog.status') },
  { key: 'message', label: t('admin.ops.errorLog.message') },
  { key: 'created_at', label: t('admin.ops.errorLog.time') },
  { key: 'user_agent', label: t('usage.userAgent') },
  { key: 'client_ip', label: t('admin.ops.errorLog.ip') },
  { key: 'actions', label: t('admin.ops.errorLog.action') },
])

const errHiddenColumns = reactive<Set<string>>(new Set())

const errToggleableColumns = computed(() =>
  errAllColumns.value.filter(col => !ERR_ALWAYS_VISIBLE.includes(col.key))
)

const errVisibleColumnKeys = computed(() =>
  errAllColumns.value
    .filter(col => ERR_ALWAYS_VISIBLE.includes(col.key) || !errHiddenColumns.has(col.key))
    .map(col => col.key)
)

const toggleErrColumn = (key: string) => {
  if (errHiddenColumns.has(key)) {
    errHiddenColumns.delete(key)
  } else {
    errHiddenColumns.add(key)
  }
  try {
    localStorage.setItem(ERR_HIDDEN_COLUMNS_KEY, JSON.stringify([...errHiddenColumns]))
  } catch (e) {
    console.error('Failed to save error columns:', e)
  }
}

const loadSavedErrColumns = () => {
  try {
    const saved = localStorage.getItem(ERR_HIDDEN_COLUMNS_KEY)
    const keys = saved ? (JSON.parse(saved) as string[]) : ERR_DEFAULT_HIDDEN_COLUMNS
    keys.forEach((key) => errHiddenColumns.add(key))
  } catch {
    ERR_DEFAULT_HIDDEN_COLUMNS.forEach((key) => errHiddenColumns.add(key))
  }
}

// 列设置下拉按当前 tab 分发
const currentToggleableColumns = computed(() =>
  activeTab.value === 'errors' ? errToggleableColumns.value : toggleableColumns.value
)
const isCurrentColumnVisible = (key: string) =>
  activeTab.value === 'errors' ? !errHiddenColumns.has(key) : isColumnVisible(key)
const toggleCurrentColumn = (key: string) =>
  activeTab.value === 'errors' ? toggleErrColumn(key) : toggleColumn(key)

const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      (JSON.parse(saved) as string[]).forEach((key) => {
        hiddenColumns.add(key)
      })
      const savedVersion = localStorage.getItem(HIDDEN_COLUMNS_VERSION_KEY)
      if (savedVersion !== HIDDEN_COLUMNS_CURRENT_VERSION) {
        if (savedVersion !== HIDDEN_COLUMNS_PREV_VERSION) {
          hiddenColumns.add('request_id')
        }
        hiddenColumns.add('upstream_request_id')
        localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
        localStorage.setItem(HIDDEN_COLUMNS_VERSION_KEY, HIDDEN_COLUMNS_CURRENT_VERSION)
      }
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach((key) => {
        hiddenColumns.add(key)
      })
      localStorage.setItem(HIDDEN_COLUMNS_VERSION_KEY, HIDDEN_COLUMNS_CURRENT_VERSION)
    }
  } catch {
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => {
      hiddenColumns.add(key)
    })
  }
}

// 页签：明细（默认）· 错误 · 排行 · 分析
type DetailTab = 'usage' | 'errors' | 'ranking' | 'analysis'
const activeTab = ref<DetailTab>('usage')
const detailTabs = computed<SectionTab[]>(() => [
  { key: 'usage', label: t('admin.usage.tabs.records') },
  { key: 'errors', label: t('admin.usage.tabs.errors') },
  { key: 'ranking', label: t('admin.usage.tabs.ranking') },
  { key: 'analysis', label: t('admin.usage.tabs.analysis') },
])
const usageFiltersRef = ref<InstanceType<typeof UsageFilters> | null>(null)
const rankingMounted = ref(false)
const rankingRef = ref<InstanceType<typeof UserTokenRanking> | null>(null)

const switchTab = (tab: DetailTab) => {
  activeTab.value = tab
  if (tab === 'errors' && errRows.value.length === 0) loadAdminErrors()
  if (tab === 'ranking') rankingMounted.value = true
}
const onTabChange = (key: string) => switchTab(key as DetailTab)

// Error tab state
const errRows = ref<OpsErrorLog[]>([])
const errLoading = ref(false)
const errPage = ref(1)
const errPageSize = ref(20)
const errTotal = ref(0)
const errSortBy = ref('created_at')
const errSortOrder = ref<'asc' | 'desc'>('desc')
const showErrorModal = ref(false)
const selectedErrorId = ref<number | null>(null)

// 注意：'YYYY-MM-DDT00:00:00' 无时区后缀，按本地时区解析后再转 UTC——与页面其它日期处理语义一致，刻意如此，勿改成 'T00:00:00Z'
const toRFC3339 = (d: string | undefined, endOfDay = false): string | undefined =>
  d ? new Date(d + (endOfDay ? 'T23:59:59.999' : 'T00:00:00')).toISOString() : undefined

const loadAdminErrors = async () => {
  errLoading.value = true
  try {
    const resp = await listErrorLogs({
      page: errPage.value,
      page_size: errPageSize.value,
      view: 'all',
      start_time: toRFC3339(filters.value.start_date),
      end_time: toRFC3339(filters.value.end_date, true),
      user_id: filters.value.user_id ?? undefined,
      api_key_id: filters.value.api_key_id ?? undefined,
      account_id: filters.value.account_id ?? undefined,
      model: filters.value.model || undefined,
      phase: filters.value.error_phase || undefined,
      category: filters.value.error_category || undefined,
      status_codes: filters.value.status_code != null ? String(filters.value.status_code) : undefined,
      sort_by: errSortBy.value,
      sort_order: errSortOrder.value,
    })
    errRows.value = resp.items
    errTotal.value = resp.total
  } catch (error) {
    console.error('Failed to load admin errors:', error)
    appStore.showError(t('usage.errors.failedToLoad'))
  } finally {
    errLoading.value = false
  }
}

const onErrSort = (sortBy: string, sortOrder: 'asc' | 'desc') => {
  errSortBy.value = sortBy
  errSortOrder.value = sortOrder
  errPage.value = 1
  loadAdminErrors()
}
const onErrPage = (p: number) => { errPage.value = p; loadAdminErrors() }
const onErrPageSize = (s: number) => { errPageSize.value = s; errPage.value = 1; loadAdminErrors() }
const openError = (id: number) => { selectedErrorId.value = id; showErrorModal.value = true }

const showColumnDropdown = ref(false)
const columnDropdownRef = ref<HTMLElement | null>(null)

const handleColumnClickOutside = (event: MouseEvent) => {
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(event.target as HTMLElement)) {
    showColumnDropdown.value = false
  }
}

onMounted(() => {
  applyRouteQueryFilters()
  void loadRouteUserFilterLabel()
  loadLogs()
  loadStats()
  loadModelStats(modelDistributionSource.value, true)
  window.setTimeout(() => {
    void loadChartData()
  }, 120)
  loadSavedColumns()
  loadSavedErrColumns()
  document.addEventListener('click', handleColumnClickOutside)
})
onUnmounted(() => { abortController?.abort(); exportAbortController?.abort(); document.removeEventListener('click', handleColumnClickOutside) })

watch(modelDistributionSource, (source) => {
  void loadModelStats(source)
})

defineExpose({ requestedModelStats, refreshData })
</script>
