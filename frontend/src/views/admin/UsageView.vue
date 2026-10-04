<template>
  <!--
    用量：区间数字摘要 + 页签（明细 · 错误）。「排行」「分析」两个页签和概览重复，已删（站长 2026-09-25 拍板）。
    时间范围、刷新、导出 / 清理在页头，作用于整页。筛选第一行只露用户 / 密钥 / 模型 / 渠道，其余在「更多筛选」。
    明细默认 7 列（时间、用户、模型、渠道、Token、收入、耗时），另 4 列在列设置里；点行开详情抽屉看其余一切
    （请求 ID、上游请求 ID 只在抽屉里、可复制；列表不放内部编号）。
  -->
  <AppLayout>
    <template #header-actions>
      <DateRangePicker
        v-model:start-date="startDate"
        v-model:end-date="endDate"
        :preset="datePreset"
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
      <UsageSummary v-if="usageStats" :stats="usageStats" />

      <div>
        <SectionTabs :model-value="activeTab" :tabs="detailTabs" :label="t('nav.usage')" @update:model-value="onTabChange" />

        <UsageFilters v-model="filters" ref="usageFiltersRef" flat :mode="activeTab" :start-date="startDate" :end-date="endDate" :model-options="modelFilterOptions" @change="applyFilters" @reset="resetFilters">
          <template #after-reset>
            <ColumnSettingsMenu :settings="activeTab === 'errors' ? errorColumnSettings : usageColumnSettings" />
          </template>
        </UsageFilters>

        <div v-show="activeTab === 'usage'" class="border-t border-af-hairline">
          <UsageTable
            mode="admin"
            :data="usageLogs"
            :loading="loading"
            :columns="usageColumnSettings.visibleColumns.value"
            :server-side-sort="true"
            :default-sort-key="'created_at'"
            :default-sort-order="'desc'"
            @sort="handleSort"
            @userClick="handleUserClick"
            @rowClick="openDetail"
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
      </div>
      <OpsErrorDetailModal v-model:show="showErrorModal" :error-id="selectedErrorId" :error-type="'request'" />
    </div>
  </AppLayout>
  <UsageDetailDrawer :log="selectedLog" @close="selectedLog = null" />
  <UsageExportProgress :show="exportProgress.show" :progress="exportProgress.progress" :current="exportProgress.current" :total="exportProgress.total" :estimated-time="exportProgress.estimatedTime" @cancel="cancelExport" />
  <UsageCleanupDialog
    :show="cleanupDialogVisible"
    :request="cleanupRequest"
    :conditions="cleanupConditions"
    @close="closeCleanupDialog"
    @submitted="cleanupSubmitted = true"
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
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import { useRoute } from 'vue-router'
import { adminAPI } from '@/api/admin'; import { adminUsageAPI } from '@/api/admin/usage'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useColumnSettings } from '@/composables/useColumnSettings'
import { formatReasoningEffort, getBrowserTimeZone } from '@/utils/format'
import { formatMultiplier } from '@/utils/formatters'
import { formatMoneyExact, profitOf } from '@/utils/money'
import { requestTypeToLegacyStream } from '@/utils/usageRequestType'
import { LAST_24_HOURS_PRESET, isReversedDateRange, rangeParams, windowForPreset } from '@/utils/dateRange'
import AppLayout from '@/components/layout/AppLayout.vue'; import Pagination from '@/components/common/Pagination.vue'; import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageFilters from '@/components/admin/usage/UsageFilters.vue'
import UsageSummary from '@/components/admin/usage/UsageSummary.vue'
import UsageDetailDrawer from '@/components/admin/usage/UsageDetailDrawer.vue'
import UsageTable from '@/components/usage/UsageTable.vue'; import UsageExportProgress from '@/components/admin/usage/UsageExportProgress.vue'
import { requestTypeLabel, rowAccountCost } from '@/components/usage/usageRow'
import UsageCleanupDialog from '@/components/admin/usage/UsageCleanupDialog.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import OpsErrorLogTable from '@/views/admin/ops/components/OpsErrorLogTable.vue'
import OpsErrorDetailModal from '@/views/admin/ops/components/OpsErrorDetailModal.vue'
import { listErrorLogs } from '@/api/admin/ops'
import type { OpsErrorLog } from '@/api/admin/ops'
import Icon from '@/components/icons/Icon.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import type { SectionTab } from '@/components/user/shell/types'
import type { Column } from '@/components/common/types'
import { ColumnSettingsMenu, MenuItem, PopoverMenu } from '@/components/admin/list'
import type { AdminUsageLog, AdminUser } from '@/types'; import type { AdminUsageStatsResponse, AdminUsageQueryParams, UsageCleanupRequest } from '@/api/admin/usage'

const { t } = useI18n()
const route = useRoute()
const usageStats = ref<AdminUsageStatsResponse | null>(null); const usageLogs = ref<AdminUsageLog[]>([]); const loading = ref(false); const exporting = ref(false)
let abortController: AbortController | null = null; let exportAbortController: AbortController | null = null
let statsReqSeq = 0
let modelOptionsReqSeq = 0
const exportProgress = reactive({ show: false, progress: 0, current: 0, total: 0, estimatedTime: '' })
const cleanupDialogVisible = ref(false)
// Balance history modal state
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)
// 明细详情抽屉
const selectedLog = ref<AdminUsageLog | null>(null)
const openDetail = (log: AdminUsageLog) => { selectedLog.value = log }

// ---------- 模型筛选的候选 ----------
// 只按时间范围取「这段时间用过的模型」，不带其它筛选：带上当前的模型筛选会把候选收窄成只剩选中的那一个。
const modelNames = ref<string[]>([])
const modelFilterOptions = computed(() => {
  const names = new Set(modelNames.value)
  if (filters.value.model) names.add(filters.value.model)
  return [...names].sort()
})
const loadModelOptions = async () => {
  const seq = ++modelOptionsReqSeq
  try {
    const res = await adminAPI.dashboard.getModelStats({ ...rangeQuery.value, model_source: 'requested' })
    if (seq !== modelOptionsReqSeq) return
    modelNames.value = (res.models || []).map((m) => m.model).filter(Boolean)
  } catch (error) {
    if (seq === modelOptionsReqSeq) console.error('Failed to load model options:', error)
  }
}

const handleUserClick = async (userId: number) => {
  try {
    const user = await adminAPI.users.getById(userId, true)
    balanceHistoryUser.value = user
    showBalanceHistoryModal.value = true
  } catch (error) {
    console.error(t('admin.usage.failedToLoadUser'), error)
  }
}

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
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start); const endDate = ref(defaultRange.end)
// 默认近 24 小时：按精确时刻查（startDate / endDate 只给日期选择器显示用）。窗口在应用筛选、刷新时按此刻重算，翻页和排序沿用同一个窗口
const datePreset = ref<string | null>(LAST_24_HOURS_PRESET)
const timeWindow = ref(windowForPreset(datePreset.value))
const renewWindow = () => { timeWindow.value = windowForPreset(datePreset.value) }
/** 发给列表 / 统计 / 模型候选的时间范围；铺在 filters 后面，盖掉 filters 里跟着日期选择器走的 start_date / end_date */
const rangeQuery = computed(() => rangeParams(startDate.value, endDate.value, timeWindow.value))
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

  // 概览点用户带来的是按天的范围（概览当时是近 24 小时就不带，这里沿用默认的近 24 小时）；起止颠倒的地址不认
  if (queryStartDate && queryEndDate && !isReversedDateRange(queryStartDate, queryEndDate)) {
    startDate.value = queryStartDate
    endDate.value = queryEndDate
    datePreset.value = null
    timeWindow.value = null
  }

  filters.value = {
    ...filters.value,
    user_id: queryUserId,
    start_date: startDate.value,
    end_date: endDate.value
  }
}

const loadRouteUserFilterLabel = async () => {
  const requestedUserId = filters.value.user_id
  if (!requestedUserId) return
  const userSearchRevision = usageFiltersRef.value?.getUserSearchRevision?.()

  const routeUserFilterIsCurrent = () => (
    filters.value.user_id === requestedUserId
    && usageFiltersRef.value?.getUserSearchRevision?.() === userSearchRevision
  )

  // 连已删用户一起查（include_deleted），仍查不到就是已被彻底删除：写「已删除用户」，不回填数字 id
  try {
    const user = await adminAPI.users.getById(requestedUserId, true)
    if (!routeUserFilterIsCurrent()) return
    usageFiltersRef.value?.setUserKeyword?.(user.email)
  } catch {
    if (!routeUserFilterIsCurrent()) return
    usageFiltersRef.value?.setUserKeyword?.(t('common.deletedUser'))
  }
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  datePreset.value = range.preset
  filters.value = {
    ...filters.value,
    start_date: range.startDate,
    end_date: range.endDate
  }
  applyFilters()
  loadModelOptions()
}

// 总数一律要精确值（exact_total）：不要时后端只回「本页之前的条数 + 本页条数 + 1」表示还有下一页，
// 第 1 页显示「共 21 条」、翻到第 2 页变成「共 27 条」（2026-10-04 走查）。精确计数是同一 WHERE 的 COUNT(*)，
// 同一页的区间统计接口本来就在同一 WHERE 上做 COUNT / SUM / AVG，列表再数一次不改变量级。
/** 列表、区间统计、清理共用的查询条件：页面上的筛选 + 时间范围（铺在后面，盖掉 filters 里按天的 start_date / end_date） */
const currentUsageQuery = (): AdminUsageQueryParams => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
  return {
    ...filters.value,
    ...rangeQuery.value,
    stream: legacyStream === null ? undefined : legacyStream
  }
}

const buildUsageListParams = (page: number, pageSize: number): AdminUsageQueryParams => ({
  page,
  page_size: pageSize,
  exact_total: true,
  ...currentUsageQuery(),
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})

const loadLogs = async () => {
  abortController?.abort(); const c = new AbortController(); abortController = c; loading.value = true
  try {
    const res = await adminAPI.usage.list(
      buildUsageListParams(pagination.page, pagination.page_size),
      { signal: c.signal }
    )
    if(!c.signal.aborted) { usageLogs.value = res.items; pagination.total = res.total }
  } catch (error: any) { if(error?.name !== 'AbortError') console.error('Failed to load usage logs:', error) } finally { if(abortController === c) loading.value = false }
}
const loadStats = async (force = false) => {
  const seq = ++statsReqSeq
  try {
    const s = await adminAPI.usage.getStats({
      ...currentUsageQuery(),
      ...(force ? { nocache: 1 } : {}),
    })
    if (seq !== statsReqSeq) return
    usageStats.value = s
  } catch (error) {
    if (seq !== statsReqSeq) return
    console.error('Failed to load usage stats:', error)
  }
}

const applyFilters = () => {
  renewWindow()
  pagination.page = 1
  loadLogs()
  loadStats()
  errPage.value = 1
  if (activeTab.value === 'errors') {
    loadAdminErrors()
  } else {
    errRows.value = []
  }
}
const refreshData = () => {
  renewWindow()
  loadLogs()
  loadStats(true)
  loadModelOptions()
  if (activeTab.value === 'errors') loadAdminErrors()
}
const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  datePreset.value = LAST_24_HOURS_PRESET
  filters.value = { start_date: startDate.value, end_date: endDate.value, request_type: undefined, native_compaction_v2: null, billing_type: null, billing_mode: undefined }
  applyFilters()
  loadModelOptions()
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
  console.error(t('usage.ipGeo.batchFailed'))
}
const cancelExport = () => exportAbortController?.abort()
// 清理删的就是页面上列着的这批：与列表同一份时间范围和筛选（2026-10-04 D8）。打开时定格，弹窗显示的、预览数的、最后删的是同一份
const cleanupRequest = ref<UsageCleanupRequest | null>(null)
const cleanupConditions = ref<Array<{ label: string; value: string }>>([])
const cleanupSubmitted = ref(false)
const openCleanupDialog = () => {
  const q = currentUsageQuery()
  cleanupRequest.value = {
    start_time: q.start_time,
    end_time: q.end_time,
    start_date: q.start_date,
    end_date: q.end_date,
    timezone: getBrowserTimeZone(),
    user_id: q.user_id,
    api_key_id: q.api_key_id,
    account_id: q.account_id,
    model: q.model,
    request_type: q.request_type,
    stream: q.stream,
    native_compaction_v2: q.native_compaction_v2,
    billing_type: q.billing_type,
    billing_mode: q.billing_mode,
    upstream_model_mismatch: q.upstream_model_mismatch
  }
  cleanupConditions.value = usageFiltersRef.value?.describeUsageConditions() ?? []
  cleanupSubmitted.value = false
  cleanupDialogVisible.value = true
}
// 提交过清理就刷新一遍，列表里不再留着已删的记录
const closeCleanupDialog = () => {
  cleanupDialogVisible.value = false
  if (cleanupSubmitted.value) refreshData()
}

// 导出：金额只导收入 / 成本 / 利润三个数（单笔精确金额），不再导标准价与它的分项。
// 请求 ID、上游请求 ID 两列保留（对账、给上游提工单用）；名字查不到的写「已删除…」，不导内部 id
const exportToExcel = async () => {
  if (exporting.value) return; exporting.value = true; exportProgress.show = true
  const c = new AbortController(); exportAbortController = c
  // 文件名按开始导出时的范围取：导出过程中改了范围也不影响这一份
  const fileName = `usage_${startDate.value}_to_${endDate.value}.xlsx`
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
      t('usage.webSearchCount'), t('usage.webSearchCost'),
      t('common.money.revenue'), t('common.money.cost'), t('common.money.profit'),
      t('admin.usage.detail.userRate'),
      t('usage.firstToken'), t('usage.duration'),
      t('admin.usage.requestId'), t('admin.usage.upstreamRequestId'), t('usage.userAgent'), t('admin.usage.ipAddress')
    ]
    const ws = XLSX.utils.aoa_to_sheet([headers])
    while (true) {
      const res = await adminUsageAPI.list(
        buildUsageListParams(p, 100),
        { signal: c.signal }
      )
      if (c.signal.aborted) break; if (p === 1) { total = res.total; exportProgress.total = total }
      const rows = (res.items || []).map((log: AdminUsageLog) => {
        const revenue = log.actual_cost ?? 0
        const cost = rowAccountCost(log)
        return [
          log.created_at, log.user?.email || t('common.deletedUser'), log.api_key?.name || t('common.deletedKey'), log.account?.name || t('common.deletedChannel'), log.model,
          log.upstream_model || log.model, log.upstream_response_model || '', log.upstream_model_mismatch == null ? '' : t(log.upstream_model_mismatch ? 'common.yes' : 'common.no'), formatReasoningEffort(log.reasoning_effort), formatReasoningEffort(log.upstream_reasoning_effort || log.reasoning_effort),
          log.inbound_endpoint || '', log.upstream_endpoint || '', requestTypeLabel(log, t),
          log.input_tokens, log.output_tokens, log.cache_read_tokens, log.cache_creation_tokens,
          log.web_search_count ?? 0, formatMoneyExact(log.web_search_cost ?? 0),
          formatMoneyExact(revenue), formatMoneyExact(cost), formatMoneyExact(profitOf(revenue, cost)),
          formatMultiplier(log.rate_multiplier ?? 1),
          log.first_token_ms ?? '', log.duration_ms,
          log.request_id || '', log.upstream_request_id || '', log.user_agent || '', log.ip_address || ''
        ]
      })
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
      saveAs(new Blob([XLSX.write(wb, { bookType: 'xlsx', type: 'array' })], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }), fileName)
    }
  } catch (error) { console.error('Failed to export:', error); }
  finally { if(exportAbortController === c) { exportAbortController = null; exporting.value = false; exportProgress.show = false } }
}

// ---------- 列设置（A4 共用实现） ----------
// 明细：默认 7 列；API 密钥、推理强度、类型、计费模式在列设置里（默认关）；端点、请求 ID、上游请求 ID、IP、UA 在详情抽屉
const usageColumns = computed<Column[]>(() => [
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user', label: t('admin.usage.user') },
  { key: 'api_key', label: t('usage.apiKeyFilter') },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort') },
  { key: 'account', label: t('admin.usage.account') },
  { key: 'stream', label: t('usage.type') },
  { key: 'billing_mode', label: t('admin.usage.billingMode') },
  { key: 'tokens', label: t('usage.tokens') },
  { key: 'cost', label: t('common.money.revenue') },
  { key: 'latency', label: t('usage.duration') },
])
const usageColumnSettings = useColumnSettings({
  storageKey: 'admin-usage-columns',
  // 2：上游请求 ID 列从列表里拿掉（只在详情抽屉），本机存过的旧设置作废
  version: 2,
  columns: usageColumns,
  defaultHidden: ['api_key', 'reasoning_effort', 'stream', 'billing_mode'],
  alwaysVisible: ['created_at', 'user']
})

// 错误页签：key 集合须与 OpsErrorLogTable 内部 allColumns 一致
const errorColumns = computed<Column[]>(() => [
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
const errorColumnSettings = useColumnSettings({
  storageKey: 'admin-usage-error-columns',
  version: 1,
  columns: errorColumns,
  defaultHidden: ['user_agent'],
  alwaysVisible: ['user', 'status', 'created_at', 'actions']
})
const errVisibleColumnKeys = computed(() => errorColumnSettings.visibleColumns.value.map((col) => col.key))

// 页签：明细（默认）· 错误
type DetailTab = 'usage' | 'errors'
const activeTab = ref<DetailTab>('usage')
const detailTabs = computed<SectionTab[]>(() => [
  { key: 'usage', label: t('admin.usage.tabs.records') },
  { key: 'errors', label: t('admin.usage.tabs.errors') },
])
const usageFiltersRef = ref<InstanceType<typeof UsageFilters> | null>(null)

const switchTab = (tab: DetailTab) => {
  activeTab.value = tab
  if (tab === 'errors' && errRows.value.length === 0) loadAdminErrors()
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
      // 近 24 小时用同一个精确窗口；按天的范围换成本地零点到当天结束
      start_time: timeWindow.value?.start_time ?? toRFC3339(startDate.value),
      end_time: timeWindow.value?.end_time ?? toRFC3339(endDate.value, true),
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

onMounted(() => {
  applyRouteQueryFilters()
  void loadRouteUserFilterLabel()
  loadLogs()
  loadStats()
  loadModelOptions()
})
onUnmounted(() => { abortController?.abort(); exportAbortController?.abort() })

defineExpose({ refreshData })
</script>
