<template>
  <!--
    用量（登录落地页）：账户带 / 今日带不看时间范围；趋势、模型用量、请求明细由头部的时间范围驱动。
    每个区块独立加载与重试，任一接口失败不把别的区块显示成零。
  -->
  <SiteShell>
    <template #actions>
      <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" @change="onDateRangeChange" />
      <button type="button" class="btn btn-secondary btn-md" :disabled="loading" data-testid="usage-refresh" @click="refreshData">
        {{ t('common.refresh') }}
      </button>
    </template>

    <div class="space-y-8">
      <!-- 账户带 + 今日带：来自 /usage/dashboard/stats，不随时间范围变 -->
      <section :aria-busy="dashboardLoading ? 'true' : undefined" class="space-y-6">
        <StatusState
          v-if="dashboardError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :description="t('userUi.usage.loadFailedHint')"
          :action-label="t('userUi.usage.retry')"
          @action="loadDashboardStats"
        />
        <template v-else>
          <StatRow :items="accountItems" data-testid="account-band" />
          <StatRow :items="todayItems" class="border-t border-af-hairline pt-6" data-testid="today-band" />
        </template>
      </section>

      <!-- 公告：最近三条，点开走全站同一个弹窗；没有公告整段不出现 -->
      <SheetSection v-if="recentAnnouncements.length" :title="t('userUi.usage.sections.announcements')">
        <template v-if="unreadAnnouncements > 0" #actions>
          <span class="text-13 text-af-ink-3">{{ t('userUi.usage.announcements.unread', { count: unreadAnnouncements }) }}</span>
        </template>
        <ul class="divide-y divide-af-hairline" data-testid="announcement-list">
          <li v-for="item in recentAnnouncements" :key="item.id">
            <button type="button" class="flex w-full items-baseline gap-3 py-3 text-left hover:bg-af-sunken" @click="openAnnouncement(item)">
              <span class="h-1.5 w-1.5 shrink-0 self-center rounded-full" :class="item.read_at ? 'bg-transparent' : 'bg-af-brand'" aria-hidden="true" />
              <span class="min-w-0 flex-1 truncate text-sm font-medium text-af-ink">{{ item.title }}</span>
              <time class="shrink-0 text-xs tabular-nums text-af-ink-4" :datetime="item.created_at">{{ formatDateOnly(item.created_at) }}</time>
            </button>
          </li>
        </ul>
      </SheetSection>

      <!-- 趋势：区间合计写在标题下；页签切 Token / 请求 / 费用 -->
      <SheetSection :title="t('userUi.usage.sections.trend')" :description="rangeSummary">
        <template #actions>
          <SectionTabs v-model="trendMetric" :tabs="trendMetricTabs" />
          <div class="w-28">
            <Select v-model="granularity" :options="granularityOptions" @change="loadChartData" />
          </div>
        </template>
        <StatusState
          v-if="chartsError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :description="t('userUi.usage.loadFailedHint')"
          :action-label="t('userUi.usage.retry')"
          @action="loadChartData"
        />
        <TokenUsageTrend v-else-if="trendMetric === 'tokens'" :trend-data="trendData" :loading="chartsLoading" bare />
        <UsageMetricTrend v-else :trend-data="trendData" :metric="trendMetric" :loading="chartsLoading" />
      </SheetSection>

      <!-- 模型用量 -->
      <SheetSection :title="t('userUi.usage.sections.models')">
        <StatusState
          v-if="modelStatsError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :description="t('userUi.usage.loadFailedHint')"
          :action-label="t('userUi.usage.retry')"
          @action="loadModelStats"
        />
        <StatusState v-else-if="modelStatsLoading && requestedModelStats.length === 0" kind="loading" :title="t('userUi.status.loading')" />
        <StatusState
          v-else-if="requestedModelStats.length === 0"
          kind="empty"
          :title="t('userUi.usage.empty')"
          :description="t('userUi.usage.emptyHint')"
        />
        <ModelUsageTable v-else :models="requestedModelStats" />
      </SheetSection>

      <!-- 请求明细 -->
      <SheetSection :title="t('userUi.usage.sections.records')">
        <template #actions>
          <button type="button" class="btn btn-ghost btn-sm" @click="resetFilters">{{ t('common.reset') }}</button>
          <div class="relative" ref="columnDropdownRef">
            <button
              type="button"
              data-testid="usage-column-settings"
              class="btn btn-secondary btn-sm"
              :title="t('admin.users.columnSettings')"
              @click="showColumnDropdown = !showColumnDropdown"
            >
              <Icon name="grid" size="sm" />
              <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
            </button>
            <div
              v-if="showColumnDropdown"
              class="absolute right-0 top-full z-50 mt-1 max-h-80 w-48 overflow-y-auto rounded-lg border border-af-hairline bg-af-sheet py-1 shadow-lg"
            >
              <button
                v-for="col in currentToggleableColumns"
                :key="col.key"
                type="button"
                :data-testid="`usage-column-toggle-${col.key}`"
                class="flex w-full items-center justify-between px-4 py-2 text-left text-sm text-af-ink-2 hover:bg-af-sunken"
                @click="toggleCurrentColumn(col.key)"
              >
                <span>{{ col.label }}</span>
                <Icon v-if="isCurrentColumnVisible(col.key)" name="check" size="sm" class="text-af-brand" />
              </button>
            </div>
          </div>
          <button v-if="activeTab !== 'errors'" type="button" class="btn btn-secondary btn-sm" :disabled="exporting" @click="exportToCSV">
            {{ exporting ? t('usage.exporting') : t('usage.exportCsv') }}
          </button>
        </template>

        <SectionTabs v-if="errorViewEnabled" v-model="activeTab" :tabs="recordTabs" class="mb-4" />

        <!-- 筛选：记录 / 错误各一组 -->
        <div v-if="activeTab === 'errors'" class="mb-4 grid grid-cols-2 gap-3 md:grid-cols-4">
          <div>
            <label class="input-label">{{ t('usage.errors.keyName') }}</label>
            <Select v-model="errorFilter.api_key_id" :options="errorKeyOptions" @change="applyErrorFilters" />
          </div>
          <div>
            <label class="input-label">{{ t('usage.errors.model') }}</label>
            <Select
              v-model="errorFilter.model"
              :options="errorModelOptions"
              searchable
              creatable
              clearable
              :placeholder="t('usage.errors.modelPlaceholder')"
              @change="applyErrorFilters"
            />
          </div>
          <div>
            <label class="input-label">{{ t('usage.errors.category') }}</label>
            <Select v-model="errorFilter.category" :options="errorCategoryOptions" @change="applyErrorFilters" />
          </div>
          <div>
            <label class="input-label">{{ t('usage.errors.status') }}</label>
            <Select v-model="errorFilter.status_code" :options="errorStatusOptions" @change="applyErrorFilters" />
          </div>
        </div>
        <div v-else class="mb-4 grid grid-cols-2 gap-3 md:grid-cols-4">
          <div>
            <label class="input-label">{{ t('usage.apiKeyFilter') }}</label>
            <Select v-model="filters.api_key_id" :options="apiKeyOptions" @change="applyFilters" />
          </div>
          <div>
            <label class="input-label">{{ t('usage.model') }}</label>
            <Select v-model="filters.model" :options="modelOptions" searchable @change="applyFilters" />
          </div>
          <div>
            <label class="input-label">{{ t('usage.type') }}</label>
            <Select v-model="filters.request_type" :options="requestTypeOptions" @change="applyFilters" />
          </div>
          <div>
            <label class="input-label">{{ t('usage.compactionFilter') }}</label>
            <Select v-model="filters.native_compaction_v2" :options="compactionOptions" @change="applyFilters" />
          </div>
          <div v-if="subscriptionFeatureEnabled">
            <label class="input-label">{{ t('admin.usage.billingType') }}</label>
            <Select v-model="filters.billing_type" :options="billingTypeOptions" @change="applyFilters" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.usage.billingMode') }}</label>
            <Select v-model="filters.billing_mode" :options="billingModeOptions" @change="applyFilters" />
          </div>
        </div>

        <template v-if="activeTab === 'usage'">
          <StatusState
            v-if="logsError"
            kind="error"
            :title="t('userUi.usage.loadFailed')"
            :description="t('userUi.usage.loadFailedHint')"
            :action-label="t('userUi.usage.retry')"
            @action="loadLogs"
          />
          <template v-else>
            <!-- 表格在容器内出血，让行分隔线贯通到页面边缘 -->
            <div class="-mx-6">
              <UsageTable
                :data="usageLogs"
                :loading="loading"
                :columns="visibleColumns"
                :server-side-sort="true"
                :show-account-billing="false"
                :show-upstream-endpoint="false"
                default-sort-key="created_at"
                default-sort-order="desc"
                @sort="handleSort"
                @ipGeoBatchFailed="handleIpGeoBatchFailed"
              />
              <Pagination
                v-if="pagination.total > 0"
                :page="pagination.page"
                :total="pagination.total"
                :page-size="pagination.page_size"
                @update:page="handlePageChange"
                @update:pageSize="handlePageSizeChange"
              />
            </div>
          </template>
        </template>

        <UserErrorRequestsTable
          v-else-if="errorViewEnabled"
          :rows="errorRows"
          :total="errorTotal"
          :loading="errorLoading"
          :page="errorPage"
          :page-size="errorPageSize"
          :visible-column-keys="errVisibleColumnKeys"
          @sort="onErrorSort"
          @update:page="onErrorPage"
          @update:pageSize="onErrorPageSize"
          @ipGeoBatchFailed="handleIpGeoBatchFailed"
        />
      </SheetSection>
    </div>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useAnnouncementStore } from '@/stores/announcements'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'
import { keysAPI, usageAPI } from '@/api'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab, StatItem } from '@/components/user/shell/types'
import Pagination from '@/components/common/Pagination.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageTable from '@/components/usage/UsageTable.vue'
import ModelUsageTable from '@/components/user/usage/ModelUsageTable.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import UsageMetricTrend, { type UsageTrendMetric } from '@/components/user/usage/UsageMetricTrend.vue'
import Icon from '@/components/icons/Icon.vue'
import UserErrorRequestsTable from '@/components/user/UserErrorRequestsTable.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatCurrency, formatDateOnly, formatNumber, formatReasoningEffort, formatTokensK } from '@/utils/format'
import { getBillingModeLabel, getDisplayBillingMode as resolveDisplayBillingMode } from '@/utils/billingMode'
import { resolveUsageRequestType, requestTypeToLegacyStream } from '@/utils/usageRequestType'
import type {
  ApiKey,
  ModelStat,
  TrendDataPoint,
  UsageLog,
  UsageQueryParams,
  UsageStatsResponse,
  UserAnnouncement,
  UserErrorRequest,
} from '@/types'
import type { UserDashboardStats } from '@/api/usage'
import type { Column } from '@/components/common/types'
import { COMMON_ERROR_STATUS_CODES } from '@/utils/errorBadges'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const announcementStore = useAnnouncementStore()

const usageStats = ref<UsageStatsResponse | null>(null)
const dashboardStats = ref<UserDashboardStats | null>(null)
const usageLogs = ref<UsageLog[]>([])
const trendData = ref<TrendDataPoint[]>([])
const requestedModelStats = ref<ModelStat[]>([])

const loading = ref(false)
const statsLoading = ref(false)
const chartsLoading = ref(false)
const modelStatsLoading = ref(false)
const exporting = ref(false)
// 每个区块独立的失败标记：任一接口失败只在自己的区块显示重试，不把别的区块显示成零用量
const statsError = ref(false)
const dashboardLoading = ref(false)
const dashboardError = ref(false)
const chartsError = ref(false)
const modelStatsError = ref(false)
const logsError = ref(false)

// 账户带：余额来自当前用户（simple mode 换成平均耗时），累计与当前速率来自 dashboard/stats
const paymentEnabled = computed(() => resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.payment))
const accountItems = computed<StatItem[]>(() => {
  const stats = dashboardStats.value
  const items: StatItem[] = []
  if (authStore.isSimpleMode) {
    items.push({
      key: 'latency',
      label: t('userUi.usage.stats.avgLatency'),
      value: `${Math.round(stats?.average_duration_ms ?? 0)} ms`
    })
  } else {
    items.push({
      key: 'balance',
      label: t('userUi.usage.stats.balance'),
      value: formatCurrency(Number(authStore.user?.balance ?? 0)),
      link: paymentEnabled.value ? { to: '/billing/recharge', label: t('userUi.usage.stats.recharge') } : undefined
    })
  }
  items.push(
    { key: 'total-cost', label: t('userUi.usage.stats.totalCost'), value: formatCurrency(stats?.total_actual_cost ?? 0) },
    { key: 'total-requests', label: t('userUi.usage.stats.totalRequests'), value: formatNumber(stats?.total_requests ?? 0) },
    {
      key: 'rate',
      label: t('userUi.usage.stats.rate'),
      value: `${formatNumber(stats?.rpm ?? 0)} RPM`,
      hint: `${formatTokensK(stats?.tpm ?? 0)} TPM`
    }
  )
  return items
})

// 今日带
const todayItems = computed<StatItem[]>(() => {
  const stats = dashboardStats.value
  return [
    {
      key: 'today-cost',
      label: t('userUi.usage.stats.todayCost'),
      value: formatCurrency(stats?.today_actual_cost ?? 0),
      hint:
        stats && stats.today_cost > stats.today_actual_cost
          ? `${t('userUi.usage.stats.standardCost')} ${formatCurrency(stats.today_cost)}`
          : undefined
    },
    { key: 'today-requests', label: t('userUi.usage.stats.todayRequests'), value: formatNumber(stats?.today_requests ?? 0) },
    { key: 'today-tokens', label: t('userUi.usage.stats.todayTokens'), value: formatTokensK(stats?.today_tokens ?? 0) }
  ]
})

// 区间合计：由当前时间范围驱动，写在趋势区块的标题下（统计接口失败时留空，不显示零）
const rangeSummary = computed(() => {
  const stats = usageStats.value
  if (statsError.value || !stats) return ''
  return t('userUi.usage.trend.rangeSummary', {
    requests: formatNumber(stats.total_requests),
    tokens: formatTokensK(stats.total_tokens),
    cost: formatCurrency(stats.total_actual_cost)
  })
})

const trendMetric = ref<'tokens' | UsageTrendMetric>('tokens')
const trendMetricTabs = computed<SectionTab[]>(() => [
  { key: 'tokens', label: t('userUi.usage.trend.tokens') },
  { key: 'requests', label: t('userUi.usage.trend.requests') },
  { key: 'cost', label: t('userUi.usage.trend.cost') }
])

// 公告：最近三条（App 壳登录后已拉取，这里只读），点开复用全站的公告弹窗
const RECENT_ANNOUNCEMENTS = 3
const recentAnnouncements = computed(() =>
  [...announcementStore.announcements]
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
    .slice(0, RECENT_ANNOUNCEMENTS)
)
const unreadAnnouncements = computed(() => announcementStore.unreadCount)
function openAnnouncement(item: UserAnnouncement) {
  announcementStore.currentPopup = item
}

const recordTabs = computed<SectionTab[]>(() => [
  { key: 'usage', label: t('usage.tabs.usage') },
  { key: 'errors', label: t('usage.tabs.errors') }
])
const errorRows = ref<UserErrorRequest[]>([])
const errorLoading = ref(false)
const errorPage = ref(1)
const errorPageSize = ref(20)
const errorSortBy = ref('created_at')
const errorSortOrder = ref<'asc' | 'desc'>('desc')
const errorTotal = ref(0)
const errorFilter = ref<{ model: string | null; category: string; api_key_id: number | null; status_code: number | null }>({
  model: '',
  category: '',
  api_key_id: null,
  status_code: null,
})

const errorKeyOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allKeys') },
  ...apiKeys.value.map((k) => ({ value: k.id, label: k.name })),
])

// 模型候选取自当前已加载错误中出现过的模型；creatable 允许输入任意片段做后端模糊。
const errorModelOptions = computed<SelectOption[]>(() => {
  const seen = new Set<string>()
  const opts: SelectOption[] = []
  for (const r of errorRows.value) {
    if (r.model && !seen.has(r.model)) {
      seen.add(r.model)
      opts.push({ value: r.model, label: r.model })
    }
  }
  return opts
})

const errorCategoryCodes = ['auth', 'rate_limit', 'quota', 'invalid_request', 'service_unavailable', 'upstream', 'internal', 'cyber']

const errorCategoryOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('usage.errors.allCategories') },
  ...errorCategoryCodes.map((c) => ({ value: c, label: t('usage.errors.categories.' + c) })),
])

// 状态码候选用固定常用列表(与管理端 UsageFilters 共用常量),不受当前页数据限制:
// 后端 status_code 过滤对全量生效,若只列当前页出现过的码,用户就选不到仅在后续页的码。
const errorStatusOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allStatuses') },
  ...COMMON_ERROR_STATUS_CODES.map((c) => ({ value: c, label: String(c) })),
])

const applyErrorFilters = () => {
  errorPage.value = 1
  void loadErrors()
}

let abortController: AbortController | null = null
let chartReqSeq = 0
let statsReqSeq = 0
let dashboardReqSeq = 0
let modelStatsReqSeq = 0

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

const getLast24HoursRangeDates = () => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const getGranularityForRange = (start: string, end: string): 'day' | 'hour' => {
  const startTime = new Date(`${start}T00:00:00`).getTime()
  const endTime = new Date(`${end}T00:00:00`).getTime()
  return Math.ceil((endTime - startTime) / (1000 * 60 * 60 * 24)) <= 1 ? 'hour' : 'day'
}

const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const granularity = ref<'day' | 'hour'>(getGranularityForRange(startDate.value, endDate.value))

const activeTab = ref<'usage' | 'errors'>('usage')
const errorViewEnabled = computed(() => appStore.cachedPublicSettings?.allow_user_view_error_requests ?? false)

const filters = ref<UsageQueryParams>({
  start_date: startDate.value,
  end_date: endDate.value,
  request_type: undefined,
  native_compaction_v2: null,
  billing_type: null,
  billing_mode: null,
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc',
})

const granularityOptions = computed<SelectOption[]>(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') },
])
const requestTypeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'live', label: t('usage.live') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
])
const compactionOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.allCompactionTypes') },
  { value: true, label: t('usage.compactionOnly') },
])
// 订阅功能关闭后只剩余额计费，「计费类型」筛选（余额/订阅）失去意义，整块隐藏。
const subscriptionFeatureEnabled = computed(() => resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.subscription))
const billingTypeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allBillingTypes') },
  { value: 0, label: t('admin.usage.billingTypeBalance') },
  { value: 1, label: t('admin.usage.billingTypeSubscription') },
])
const billingModeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allBillingModes') },
  { value: 'token', label: t('admin.usage.billingModeToken') },
  { value: 'per_request', label: t('admin.usage.billingModePerRequest') },
  { value: 'image', label: t('admin.usage.billingModeImage') },
  { value: 'video', label: t('admin.usage.billingModeVideo') },
])

const apiKeys = ref<ApiKey[]>([])
const modelOptionValues = ref<string[]>([])

const apiKeyOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.allApiKeys') },
  ...apiKeys.value.map((key) => ({ value: key.id, label: key.name })),
])
const modelOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allModels') },
  ...modelOptionValues.value.map((model) => ({ value: model, label: model })),
])

const normalizedFilters = computed<UsageQueryParams>(() => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
  return {
    ...filters.value,
    start_date: startDate.value,
    end_date: endDate.value,
    stream: legacyStream === null ? undefined : legacyStream,
  }
})

const buildUsageListParams = (page: number, pageSize: number): UsageQueryParams => ({
  page,
  page_size: pageSize,
  ...normalizedFilters.value,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order,
})

const loadLogs = async () => {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  loading.value = true
  logsError.value = false
  try {
    const res = await usageAPI.query(buildUsageListParams(pagination.page, pagination.page_size), {
      signal: controller.signal,
    })
    if (!controller.signal.aborted) {
      usageLogs.value = res.items
      pagination.total = res.total
    }
  } catch (error: any) {
    if (error?.name !== 'AbortError' && error?.code !== 'ERR_CANCELED') {
      logsError.value = true
      appStore.showError(t('usage.failedToLoad'))
    }
  } finally {
    if (abortController === controller) loading.value = false
  }
}

const loadDashboardStats = async () => {
  const seq = ++dashboardReqSeq
  dashboardLoading.value = true
  dashboardError.value = false
  try {
    const stats = await usageAPI.getDashboardStats()
    if (seq !== dashboardReqSeq) return
    dashboardStats.value = stats
  } catch (error) {
    if (seq !== dashboardReqSeq) return
    console.error('Failed to load dashboard stats:', error)
    dashboardError.value = true
  } finally {
    if (seq === dashboardReqSeq) dashboardLoading.value = false
  }
}

const loadStats = async () => {
  const seq = ++statsReqSeq
  statsLoading.value = true
  statsError.value = false
  try {
    const stats = await usageAPI.getStats(normalizedFilters.value)
    if (seq !== statsReqSeq) return
    usageStats.value = stats
  } catch (error) {
    if (seq !== statsReqSeq) return
    console.error('Failed to load usage stats:', error)
    statsError.value = true
  } finally {
    if (seq === statsReqSeq) statsLoading.value = false
  }
}

const loadModelStats = async () => {
  const seq = ++modelStatsReqSeq
  modelStatsLoading.value = true
  modelStatsError.value = false
  try {
    const response = await usageAPI.getDashboardModels({
      ...normalizedFilters.value,
      model_source: 'requested',
    })
    if (seq !== modelStatsReqSeq) return
    requestedModelStats.value = response.models || []
    refreshModelOptions(response.models || [])
  } catch (error) {
    if (seq !== modelStatsReqSeq) return
    console.error('Failed to load model stats:', error)
    modelStatsError.value = true
  } finally {
    if (seq === modelStatsReqSeq) modelStatsLoading.value = false
  }
}

const loadChartData = async () => {
  const seq = ++chartReqSeq
  chartsLoading.value = true
  chartsError.value = false
  try {
    const snapshot = await usageAPI.getDashboardSnapshotV2({
      ...normalizedFilters.value,
      granularity: granularity.value,
      include_trend: true,
      include_model_stats: false,
    })
    if (seq !== chartReqSeq) return
    trendData.value = snapshot.trend || []
  } catch (error) {
    if (seq !== chartReqSeq) return
    console.error('Failed to load chart data:', error)
    chartsError.value = true
  } finally {
    if (seq === chartReqSeq) chartsLoading.value = false
  }
}

const refreshModelOptions = (models: ModelStat[]) => {
  const current = filters.value.model
  const set = new Set(modelOptionValues.value)
  models.forEach((item) => {
    if (item.model) set.add(item.model)
  })
  if (current) set.add(current)
  modelOptionValues.value = Array.from(set).sort()
}

const applyFilters = () => {
  pagination.page = 1
  void loadLogs()
  void loadStats()
  void loadModelStats()
  void loadChartData()
  resetErrorRows()
}

const refreshData = () => {
  void loadDashboardStats()
  void loadLogs()
  void loadStats()
  void loadModelStats()
  void loadChartData()
  if (activeTab.value === 'errors') void loadErrors()
}

const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = {
    start_date: range.start,
    end_date: range.end,
    request_type: undefined,
    native_compaction_v2: null,
    billing_type: null,
    billing_mode: null,
  }
  granularity.value = getGranularityForRange(range.start, range.end)
  applyFilters()
  if (activeTab.value === 'errors') {
    errorFilter.value = { model: '', category: '', api_key_id: null, status_code: null }
    applyErrorFilters()
  }
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  filters.value.start_date = range.startDate
  filters.value.end_date = range.endDate
  granularity.value = getGranularityForRange(range.startDate, range.endDate)
  applyFilters()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  void loadLogs()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadLogs()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  void loadLogs()
}

const handleIpGeoBatchFailed = () => {
  appStore.showError(t('usage.ipGeo.batchFailed'))
}

const getRequestTypeExportText = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'cyber') return 'Cyber'
  if (requestType === 'live') return 'Live'
  if (requestType === 'ws_v2') return 'WS'
  if (requestType === 'stream') return 'Stream'
  if (requestType === 'sync') return 'Sync'
  return 'Unknown'
}

const getDisplayBillingMode = (
  row: Pick<UsageLog, 'billing_mode' | 'image_count'> | null | undefined
): string | null | undefined => resolveDisplayBillingMode(row)

const escapeCSVValue = (value: unknown): string => {
  if (value == null) return ''
  const str = String(value)
  const escaped = str.replace(/"/g, '""')
  if (/^[=+\-@\t\r]/.test(str)) return `"\'${escaped}"`
  if (/[,"\n\r]/.test(str)) return `"${escaped}"`
  return str
}

const exportToCSV = async () => {
  if (pagination.total === 0) {
    appStore.showWarning(t('usage.noDataToExport'))
    return
  }
  exporting.value = true
  appStore.showInfo(t('usage.preparingExport'))
  try {
    const allLogs: UsageLog[] = []
    const pageSize = 100
    const exportParams = buildUsageListParams(1, pageSize)
    const totalPages = Math.ceil(pagination.total / pageSize)
    for (let page = 1; page <= totalPages; page++) {
      const response = await usageAPI.query({ ...exportParams, page })
      allLogs.push(...response.items)
    }
    if (allLogs.length === 0) {
      appStore.showWarning(t('usage.noDataToExport'))
      return
    }
    const headers = [
      'Time',
      'API Key Name',
      'Model',
      'Reasoning Effort',
      'Inbound Endpoint',
      'IP Address',
      'Type',
      'Billing Mode',
      'Input Tokens',
      'Output Tokens',
      'Cache Read Tokens',
      'Cache Creation Tokens',
      'Rate Multiplier',
      'Billed Cost',
      'Original Cost',
      'First Token (ms)',
      'Duration (ms)',
    ]
    const rows = allLogs.map((log) => [
      log.created_at,
      log.api_key?.name || '',
      log.model,
      formatReasoningEffort(log.reasoning_effort),
      log.inbound_endpoint || '',
      log.ip_address || '',
      getRequestTypeExportText(log),
      getBillingModeLabel(getDisplayBillingMode(log), t),
      log.input_tokens,
      log.output_tokens,
      log.cache_read_tokens,
      log.cache_creation_tokens,
      log.rate_multiplier,
      log.actual_cost.toFixed(8),
      log.total_cost.toFixed(8),
      log.first_token_ms ?? '',
      log.duration_ms ?? '',
    ].map(escapeCSVValue))
    const csvContent = [
      headers.map(escapeCSVValue).join(','),
      ...rows.map((row) => row.join(',')),
    ].join('\n')
    const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `usage_${exportParams.start_date}_to_${exportParams.end_date}.csv`
    link.click()
    window.URL.revokeObjectURL(url)
    appStore.showSuccess(t('usage.exportSuccess'))
  } catch (error) {
    console.error('CSV Export failed:', error)
    appStore.showError(t('usage.exportFailed'))
  } finally {
    exporting.value = false
  }
}

const ALWAYS_VISIBLE = ['created_at']
const DEFAULT_HIDDEN_COLUMNS = ['user_agent']
const HIDDEN_COLUMNS_KEY = 'user-usage-hidden-columns'

const allColumns = computed<Column[]>(() => [
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'ip_address', label: 'IP', sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'latency', label: t('usage.latency'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
])

const hiddenColumns = reactive<Set<string>>(new Set())
const toggleableColumns = computed(() => allColumns.value.filter((col) => !ALWAYS_VISIBLE.includes(col.key)))
const visibleColumns = computed(() =>
  allColumns.value.filter((col) => ALWAYS_VISIBLE.includes(col.key) || !hiddenColumns.has(col.key))
)
const isColumnVisible = (key: string) => !hiddenColumns.has(key)
const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) hiddenColumns.delete(key)
  else hiddenColumns.add(key)
  localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
}
const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    const values = saved ? JSON.parse(saved) as string[] : DEFAULT_HIDDEN_COLUMNS
    values.forEach((key) => hiddenColumns.add(key))
  } catch {
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
  }
}

// 错误请求 tab 独立列设置(机制同用量列设置,存储互不影响)
const ERR_ALWAYS_VISIBLE = ['status', 'created_at']
const ERR_DEFAULT_HIDDEN_COLUMNS = ['user_agent']
const ERR_HIDDEN_COLUMNS_KEY = 'user-usage-error-hidden-columns'

// key 须与 UserErrorRequestsTable 的 allColumns 一致
const errAllColumns = computed<Column[]>(() => [
  { key: 'key_name', label: t('usage.errors.keyName') },
  { key: 'model', label: t('usage.errors.model') },
  { key: 'endpoint', label: t('usage.errors.endpoint') },
  { key: 'client_ip', label: 'IP' },
  { key: 'type', label: t('usage.type') },
  { key: 'platform', label: t('usage.errors.platform') },
  { key: 'category', label: t('usage.errors.category') },
  { key: 'status', label: t('usage.errors.status') },
  { key: 'message', label: t('usage.errors.message') },
  { key: 'created_at', label: t('usage.errors.time') },
  { key: 'user_agent', label: t('usage.userAgent') },
])

const errHiddenColumns = reactive<Set<string>>(new Set())
const errToggleableColumns = computed(() =>
  errAllColumns.value.filter((col) => !ERR_ALWAYS_VISIBLE.includes(col.key))
)
const errVisibleColumnKeys = computed(() =>
  errAllColumns.value
    .filter((col) => ERR_ALWAYS_VISIBLE.includes(col.key) || !errHiddenColumns.has(col.key))
    .map((col) => col.key)
)
const isErrColumnVisible = (key: string) => !errHiddenColumns.has(key)
const toggleErrColumn = (key: string) => {
  if (errHiddenColumns.has(key)) errHiddenColumns.delete(key)
  else errHiddenColumns.add(key)
  localStorage.setItem(ERR_HIDDEN_COLUMNS_KEY, JSON.stringify([...errHiddenColumns]))
}
const loadSavedErrColumns = () => {
  try {
    const saved = localStorage.getItem(ERR_HIDDEN_COLUMNS_KEY)
    const values = saved ? (JSON.parse(saved) as string[]) : ERR_DEFAULT_HIDDEN_COLUMNS
    values.forEach((key) => errHiddenColumns.add(key))
  } catch {
    ERR_DEFAULT_HIDDEN_COLUMNS.forEach((key) => errHiddenColumns.add(key))
  }
}

// 列设置下拉按当前 tab 分发
const currentToggleableColumns = computed(() =>
  activeTab.value === 'errors' ? errToggleableColumns.value : toggleableColumns.value
)
const isCurrentColumnVisible = (key: string) =>
  activeTab.value === 'errors' ? isErrColumnVisible(key) : isColumnVisible(key)
const toggleCurrentColumn = (key: string) => {
  if (activeTab.value === 'errors') toggleErrColumn(key)
  else toggleColumn(key)
}

const showColumnDropdown = ref(false)
const columnDropdownRef = ref<HTMLElement | null>(null)
const handleColumnClickOutside = (event: MouseEvent) => {
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(event.target as HTMLElement)) {
    showColumnDropdown.value = false
  }
}

const loadApiKeys = async () => {
  const firstPage = await keysAPI.list(1, 100)
  const keys = [...firstPage.items]
  for (let page = 2; page <= firstPage.pages && keys.length > 0; page++) {
    const response = await keysAPI.list(page, 100)
    if (response.items.length === 0) break
    keys.push(...response.items)
  }
  return keys
}

const loadFilterOptions = async () => {
  try {
    apiKeys.value = await loadApiKeys()
  } catch (error) {
    console.error('Failed to load usage filter options:', error)
  }
}

const resetErrorRows = () => {
  errorPage.value = 1
  if (activeTab.value === 'errors') {
    void loadErrors()
  } else {
    errorRows.value = []
    errorTotal.value = 0
  }
}

const loadErrors = async () => {
  errorLoading.value = true
  try {
    const resp = await usageAPI.listMyErrorRequests({
      page: errorPage.value,
      page_size: errorPageSize.value,
      start_date: startDate.value,
      end_date: endDate.value,
      model: (errorFilter.value.model ?? '').trim() || undefined,
      category: errorFilter.value.category || undefined,
      api_key_id: errorFilter.value.api_key_id ?? undefined,
      status_code: errorFilter.value.status_code ?? undefined,
      sort_by: errorSortBy.value,
      sort_order: errorSortOrder.value,
    })
    errorRows.value = resp.items
    errorTotal.value = resp.total
  } catch (error) {
    console.error('[UsageView] loadErrors failed:', error)
    appStore.showError(t('usage.errors.failedToLoad'))
  } finally {
    errorLoading.value = false
  }
}

const onErrorSort = (sortBy: string, sortOrder: 'asc' | 'desc') => {
  errorSortBy.value = sortBy
  errorSortOrder.value = sortOrder
  errorPage.value = 1
  void loadErrors()
}

const onErrorPage = (page: number) => {
  errorPage.value = page
  void loadErrors()
}

const onErrorPageSize = (pageSize: number) => {
  errorPageSize.value = pageSize
  errorPage.value = 1
  void loadErrors()
}

// 首次切到错误页签时才加载错误记录（页签由 SectionTabs 的 v-model 切换）
watch(activeTab, (tab) => {
  if (tab === 'errors' && errorRows.value.length === 0) void loadErrors()
})

onMounted(() => {
  loadSavedColumns()
  loadSavedErrColumns()
  document.addEventListener('click', handleColumnClickOutside)
  void loadFilterOptions()
  // 指标行的余额来自当前用户：进页时刷新一次（原概览页的行为）
  void authStore.refreshUser().catch(() => undefined)
  refreshData()
})

onUnmounted(() => {
  abortController?.abort()
  document.removeEventListener('click', handleColumnClickOutside)
})
</script>
