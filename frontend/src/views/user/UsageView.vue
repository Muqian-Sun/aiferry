<template>
  <!--
    用量明细（muqian 2026-09-25 定）：回答「钱花在哪了、某一次请求怎么了」。
    区间摘要 → 费用分布（按实付，前 5 + 其他，点一行 = 加上这个模型筛选）→ 请求明细（请求 / 错误页签，筛选标签，点行开详情抽屉）。
    时间范围、密钥、模型、页签都写进地址栏：概览和密钥页带条件跳过来，刷新 / 返回也不丢。
    每个区块独立加载与重试，任一接口失败不把别的区块显示成零。
  -->
  <SiteShell>
    <template #actions>
      <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" @change="onDateRangeChange" />
      <button type="button" class="btn btn-ghost btn-md" :disabled="loading" data-testid="usage-refresh" @click="refreshData">
        <Icon name="refresh" size="sm" />
        {{ t('common.refresh') }}
      </button>
      <PopoverMenu width-class="w-44">
        <template #trigger="{ open }">
          <button
            type="button"
            class="btn btn-ghost btn-md px-2"
            :class="open ? 'bg-af-sunken' : ''"
            :aria-label="t('userUi.usage.moreActions')"
            :title="t('userUi.usage.moreActions')"
            data-testid="usage-more-menu"
          >
            <Icon name="more" size="sm" />
          </button>
        </template>
        <MenuItem icon="download" :disabled="exporting || activeTab === 'errors'" data-testid="usage-export" @click="exportToCSV">
          {{ exporting ? t('usage.exporting') : t('usage.exportCsv') }}
        </MenuItem>
      </PopoverMenu>
    </template>

    <div class="space-y-8">
      <!-- 区间数字摘要：跟随时间范围与筛选；统计接口失败就不出现，不显示零 -->
      <StatRow v-if="rangeItems" :items="rangeItems" data-testid="usage-range-summary" />

      <!-- 费用分布：这一页回答钱的问题，按实付排序与算占比 -->
      <SheetSection :title="t('userUi.usage.sections.spend')" data-testid="usage-spend">
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
        <ModelUsageTable v-else :models="requestedModelStats" :selected-model="filters.model || ''" @select="toggleModelFilter" />
      </SheetSection>

      <!-- 请求明细 -->
      <SheetSection :title="t('userUi.usage.sections.records')">
        <template #actions>
          <ColumnSettingsMenu :settings="activeTab === 'errors' ? errorColumnSettings : columnSettings" />
        </template>

        <SectionTabs v-if="errorViewEnabled" v-model="activeTab" :tabs="recordTabs" class="mb-4" />

        <!-- 筛选标签：没选是虚线、选了实心带 ✕；低频维度收在「更多筛选」后面，选了就一直露着 -->
        <div v-if="activeTab === 'errors'" class="mb-4 flex flex-wrap items-center gap-2" data-testid="usage-filters">
          <FilterChip v-model="errorKeyChip" :label="t('usage.errors.keyName')" :options="apiKeyOptions" :missing-label="missingKeyLabel" test-id="error-filter-key" @change="applyErrorFilters" />
          <FilterChip v-model="errorModelChip" :label="t('usage.errors.model')" :options="errorModelOptions" test-id="error-filter-model" @change="applyErrorFilters" />
          <FilterChip v-model="errorFilter.category" :label="t('usage.errors.category')" :options="errorCategoryOptions" test-id="error-filter-category" @change="applyErrorFilters" />
          <FilterChip v-model="errorStatusChip" :label="t('usage.errors.status')" :options="errorStatusOptions" test-id="error-filter-status" @change="applyErrorFilters" />
          <button v-if="errorFiltersActive" type="button" class="px-2 text-13 text-af-ink-3 hover:text-af-ink" data-testid="error-filters-clear" @click="clearErrorFilters">
            {{ t('userUi.usage.clearFilters') }}
          </button>
        </div>
        <div v-else class="mb-4 flex flex-wrap items-center gap-2" data-testid="usage-filters">
          <FilterChip v-model="keyChip" :label="t('usage.apiKeyFilter')" :options="apiKeyOptions" :missing-label="missingKeyLabel" test-id="usage-filter-key" @change="applyFilters" />
          <FilterChip v-model="modelChip" :label="t('usage.model')" :options="modelOptions" test-id="usage-filter-model" @change="applyFilters" />
          <template v-if="showMoreFilters">
            <FilterChip v-model="requestTypeChip" :label="t('usage.type')" :options="requestTypeOptions" test-id="usage-filter-type" @change="applyFilters" />
            <FilterChip v-model="compactionChip" :label="t('usage.compactionFilter')" :options="compactionOptions" test-id="usage-filter-compaction" @change="applyFilters" />
            <FilterChip
              v-if="subscriptionFeatureEnabled"
              v-model="billingTypeChip"
              :label="t('admin.usage.billingType')"
              :options="billingTypeOptions"
              test-id="usage-filter-billing-type"
              @change="applyFilters"
            />
            <FilterChip v-model="billingModeChip" :label="t('admin.usage.billingMode')" :options="billingModeOptions" test-id="usage-filter-billing-mode" @change="applyFilters" />
          </template>
          <button
            v-else
            type="button"
            class="inline-flex h-8 items-center gap-1 rounded-full px-2 text-13 text-af-ink-3 transition-colors hover:text-af-ink"
            data-testid="usage-more-filters"
            @click="moreFiltersOpen = true"
          >
            <Icon name="plus" size="xs" :stroke-width="2" />
            {{ t('userUi.usage.moreFilters') }}
          </button>
          <button v-if="usageFiltersActive" type="button" class="px-2 text-13 text-af-ink-3 hover:text-af-ink" data-testid="usage-filters-clear" @click="resetFilters">
            {{ t('userUi.usage.clearFilters') }}
          </button>
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
            <!-- 桌面表格在容器内出血，让行分隔线贯通到页面边缘；窄屏是卡片列表，留页边距 -->
            <div class="md:-mx-6">
              <UsageTable
                :data="usageLogs"
                :loading="loading"
                :columns="columnSettings.visibleColumns.value"
                :server-side-sort="true"
                :clickable-rows="true"
                default-sort-key="created_at"
                default-sort-order="desc"
                @sort="handleSort"
                @row-click="openDetail"
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
          :visible-column-keys="errorColumnSettings.visibleColumns.value.map((col) => col.key)"
          @sort="onErrorSort"
          @update:page="onErrorPage"
          @update:pageSize="onErrorPageSize"
          @ipGeoBatchFailed="handleIpGeoBatchFailed"
        />
      </SheetSection>
    </div>

    <!-- 请求详情：原来只能悬停看的 Token / 费用明细，加上报障要用的请求 ID、客户端信息 -->
    <DetailDrawer
      :show="detailLog !== null"
      :title="detailLog?.model || ''"
      :eyebrow="t('userUi.usage.detail.eyebrow')"
      :subtitle="detailLog ? formatDateTime(detailLog.created_at) : ''"
      @close="detailLog = null"
    >
      <div v-if="detailLog" class="space-y-6" data-testid="usage-detail">
        <dl class="grid grid-cols-3 divide-x divide-af-hairline">
          <div class="min-w-0 pr-4">
            <dt class="text-13 text-af-ink-3">{{ t('userUi.usage.stats.actualCost') }}</dt>
            <dd class="mt-1 truncate text-lg font-semibold text-af-ink">{{ formatCurrency(detailLog.actual_cost) }}</dd>
          </div>
          <div class="min-w-0 px-4">
            <dt class="text-13 text-af-ink-3">{{ t('userUi.usage.stats.tokens') }}</dt>
            <dd class="mt-1 truncate text-lg font-semibold text-af-ink">{{ formatTokensK(detailTotalTokens) }}</dd>
          </div>
          <div class="min-w-0 pl-4">
            <dt class="text-13 text-af-ink-3">{{ t('usage.latencyDuration') }}</dt>
            <dd class="mt-1 truncate text-lg font-semibold text-af-ink">{{ formatDuration(detailLog.duration_ms) }}</dd>
          </div>
        </dl>

        <section>
          <h3 class="mb-2 text-13 font-semibold text-af-ink">{{ t('userUi.usage.detail.request') }}</h3>
          <dl class="divide-y divide-af-hairline border-y border-af-hairline">
            <DetailField :label="t('usage.apiKeyFilter')" :value="detailLog.api_key?.name" />
            <DetailField v-if="detailLog.reasoning_effort" :label="t('usage.reasoningEffort')" :value="formatReasoningEffort(detailLog.reasoning_effort)" />
            <DetailField :label="t('usage.type')" :value="requestTypeLabel(detailLog)" />
            <DetailField :label="t('admin.usage.billingMode')" :value="getBillingModeLabel(getDisplayBillingMode(detailLog), t)" />
            <DetailField :label="t('usage.latencyFirstToken')" :value="formatDuration(detailLog.first_token_ms)" />
            <DetailField :label="t('usage.endpoint')" :value="detailLog.inbound_endpoint" />
            <DetailField label="IP">
              <template v-if="detailLog.ip_address">
                <span class="font-mono">{{ detailLog.ip_address }}</span>
                <IpGeoCell :ip="detailLog.ip_address" />
              </template>
              <template v-else>—</template>
            </DetailField>
            <DetailField :label="t('usage.userAgent')" :value="detailLog.user_agent" />
            <DetailField :label="t('userUi.usage.detail.requestId')">
              <span v-if="detailLog.request_id" class="flex items-center gap-2">
                <code class="min-w-0 truncate font-mono text-xs text-af-ink-2" :title="detailLog.request_id">{{ detailLog.request_id }}</code>
                <button
                  type="button"
                  class="shrink-0 text-13 text-af-ink-3 hover:text-af-ink"
                  data-testid="usage-detail-copy-request-id"
                  @click="copyToClipboard(detailLog.request_id, t('userUi.usage.detail.requestIdCopied'))"
                >
                  {{ t('userUi.usage.detail.copy') }}
                </button>
              </span>
              <template v-else>—</template>
            </DetailField>
          </dl>
        </section>

        <section class="text-13">
          <h3 class="mb-2 font-semibold text-af-ink">{{ t('usage.tokenDetails') }}</h3>
          <UsageTokenBreakdown :row="detailLog" :show-title="false" />
        </section>

        <section class="text-13">
          <h3 class="mb-2 font-semibold text-af-ink">{{ t('usage.costDetails') }}</h3>
          <UsageCostBreakdown :row="detailLog" :show-title="false" />
        </section>
      </div>
    </DetailDrawer>
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter, type LocationQuery } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { SITE_FEATURES } from '@/utils/siteFeatures'
import { keysAPI, usageAPI } from '@/api'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import SectionTabs from '@/components/user/shell/SectionTabs.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { SectionTab, StatItem } from '@/components/user/shell/types'
import Pagination from '@/components/common/Pagination.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import FilterChip from '@/components/common/FilterChip.vue'
import PopoverMenu from '@/components/common/PopoverMenu.vue'
import MenuItem from '@/components/common/MenuItem.vue'
import ColumnSettingsMenu from '@/components/common/ColumnSettingsMenu.vue'
import DetailDrawer from '@/components/common/DetailDrawer.vue'
import DetailField from '@/components/common/DetailField.vue'
import IpGeoCell from '@/components/common/IpGeoCell.vue'
import type { FilterOption } from '@/components/common/types'
import UsageTable from '@/components/usage/UsageTable.vue'
import UsageTokenBreakdown from '@/components/usage/UsageTokenBreakdown.vue'
import UsageCostBreakdown from '@/components/usage/UsageCostBreakdown.vue'
import ModelUsageTable from '@/components/user/usage/ModelUsageTable.vue'
import Icon from '@/components/icons/Icon.vue'
import UserErrorRequestsTable from '@/components/user/UserErrorRequestsTable.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useColumnSettings } from '@/composables/useColumnSettings'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatCurrency, formatDateTime, formatNumber, formatReasoningEffort, formatTokensK } from '@/utils/format'
import { getBillingModeLabel, getDisplayBillingMode as resolveDisplayBillingMode } from '@/utils/billingMode'
import { resolveUsageRequestType, requestTypeToLegacyStream } from '@/utils/usageRequestType'
import type {
  ApiKey,
  ModelStat,
  UsageLog,
  UsageQueryParams,
  UsageRequestType,
  UsageStatsResponse,
  UserErrorRequest,
} from '@/types'
import type { Column } from '@/components/common/types'
import { COMMON_ERROR_STATUS_CODES } from '@/utils/errorBadges'

const { t } = useI18n()
const appStore = useAppStore()
// 单测里不装路由：拿不到就当没有地址栏参数、也不回写
const route = useRoute() as ReturnType<typeof useRoute> | undefined
const router = useRouter() as ReturnType<typeof useRouter> | undefined
const { copyToClipboard } = useClipboard()

const usageStats = ref<UsageStatsResponse | null>(null)
const usageLogs = ref<UsageLog[]>([])
const requestedModelStats = ref<ModelStat[]>([])

const loading = ref(false)
const statsLoading = ref(false)
const modelStatsLoading = ref(false)
const exporting = ref(false)
// 每个区块独立的失败标记：任一接口失败只在自己的区块显示重试，不把别的区块显示成零用量
const statsError = ref(false)
const modelStatsError = ref(false)
const logsError = ref(false)

// ---------- 地址栏参数 ----------
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const queryString = (query: LocationQuery, key: string): string => {
  const value = query[key]
  return typeof value === 'string' ? value : ''
}
const initialQuery: LocationQuery = route?.query ?? {}

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

const getLast24HoursRangeDates = () => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const defaultRange = getLast24HoursRangeDates()
const queryStart = queryString(initialQuery, 'start')
const queryEnd = queryString(initialQuery, 'end')
const hasQueryRange = DATE_RE.test(queryStart) && DATE_RE.test(queryEnd) && queryStart <= queryEnd
const startDate = ref(hasQueryRange ? queryStart : defaultRange.start)
const endDate = ref(hasQueryRange ? queryEnd : defaultRange.end)

const queryKeyId = Number(queryString(initialQuery, 'key'))
const queryModel = queryString(initialQuery, 'model').trim()

const errorViewEnabled = computed(() => appStore.cachedPublicSettings?.allow_user_view_error_requests ?? false)
// 地址栏要错误页签时先记下：公开设置可能比页面晚到，到了且允许看错误才切过去
const wantsErrorTab = queryString(initialQuery, 'tab') === 'errors'
const activeTab = ref<'usage' | 'errors'>(wantsErrorTab && errorViewEnabled.value ? 'errors' : 'usage')

const filters = ref<UsageQueryParams>({
  start_date: startDate.value,
  end_date: endDate.value,
  api_key_id: Number.isInteger(queryKeyId) && queryKeyId > 0 ? queryKeyId : undefined,
  model: queryModel || undefined,
  request_type: undefined,
  native_compaction_v2: null,
  billing_type: null,
  billing_mode: null,
})

/** 把当前范围、密钥、模型、页签写回地址栏（replace，不堆历史）；地址栏里的其他参数原样保留 */
function syncQuery() {
  if (!route || !router) return
  const next: Record<string, string> = {}
  for (const [key, value] of Object.entries(route.query)) {
    if (typeof value === 'string' && !['start', 'end', 'key', 'model', 'tab'].includes(key)) next[key] = value
  }
  if (startDate.value !== defaultRange.start || endDate.value !== defaultRange.end) {
    next.start = startDate.value
    next.end = endDate.value
  }
  if (filters.value.api_key_id) next.key = String(filters.value.api_key_id)
  if (filters.value.model) next.model = filters.value.model
  if (activeTab.value === 'errors') next.tab = 'errors'
  void router.replace({ query: next }).catch(() => undefined)
}

// ---------- 区间摘要 ----------
const errorCount = ref<number | null>(null)

const percentText = (share: number) => `${(share * 100).toFixed(share > 0 && share < 0.1 ? 1 : 0)}%`

/** 毫秒数：1 秒内写 ms，1 分钟内写 1 位小数的秒，再长写分秒 */
function formatDuration(ms: number | null | undefined): string {
  if (ms == null) return '—'
  if (ms < 1000) return `${Math.round(ms)} ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`
  const totalSec = Math.round(ms / 1000)
  return `${Math.floor(totalSec / 60)}m ${totalSec % 60}s`
}

// 由当前时间范围与筛选驱动，放在页面最上方（统计接口失败时不出现，不显示零）
const rangeItems = computed<StatItem[] | null>(() => {
  const stats = usageStats.value
  if (statsError.value || !stats) return null
  const inputSide = (stats.total_input_tokens ?? 0) + (stats.total_cache_read_tokens ?? 0) + (stats.total_cache_creation_tokens ?? 0)
  const items: StatItem[] = [
    { key: 'range-requests', label: t('userUi.usage.stats.requests'), value: formatNumber(stats.total_requests) },
    { key: 'range-tokens', label: t('userUi.usage.stats.tokens'), value: formatTokensK(stats.total_tokens) },
    {
      key: 'range-cost',
      label: t('userUi.usage.stats.actualCost'),
      value: formatCurrency(stats.total_actual_cost),
      hint: stats.total_cost > stats.total_actual_cost ? `${t('userUi.usage.stats.standardCost')} ${formatCurrency(stats.total_cost)}` : undefined
    },
    {
      key: 'range-cache-hit',
      label: t('userUi.usage.stats.cacheHitRate'),
      value: inputSide > 0 ? percentText((stats.total_cache_read_tokens ?? 0) / inputSide) : '—'
    },
    { key: 'range-latency', label: t('userUi.usage.stats.avgLatency'), value: formatDuration(stats.average_duration_ms ?? 0) }
  ]
  if (errorViewEnabled.value && errorCount.value !== null) {
    items.push({
      key: 'range-failures',
      label: t('userUi.usage.stats.failures'),
      value: formatNumber(errorCount.value),
      action: errorCount.value > 0 && activeTab.value !== 'errors'
        ? { label: t('userUi.usage.stats.viewFailures'), onClick: () => { activeTab.value = 'errors' } }
        : undefined
    })
  }
  return items
})

// ---------- 错误页签 ----------
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
const errorFilter = ref<{ model: string; category: string; api_key_id: number | null; status_code: number | null }>({
  model: '',
  category: '',
  api_key_id: null,
  status_code: null,
})

/** 筛选标签的值：空串 = 全部；数字类维度在这里转回数字 / null */
const numberChip = (read: () => number | null | undefined, write: (value: number | null) => void) =>
  computed<string | number>({
    get: () => read() ?? '',
    set: (value) => write(value === '' ? null : Number(value)),
  })

const errorKeyChip = numberChip(() => errorFilter.value.api_key_id, (value) => { errorFilter.value.api_key_id = value })
const errorStatusChip = numberChip(() => errorFilter.value.status_code, (value) => { errorFilter.value.status_code = value })
const errorModelChip = computed<string | number>({
  get: () => errorFilter.value.model,
  set: (value) => { errorFilter.value.model = String(value) },
})

// 模型候选取自当前已加载错误与用量分布里出现过的模型
const errorModelOptions = computed<FilterOption[]>(() => {
  const seen = new Set<string>(modelOptionValues.value)
  for (const row of errorRows.value) if (row.model) seen.add(row.model)
  if (errorFilter.value.model) seen.add(errorFilter.value.model)
  return [...seen].sort().map((model) => ({ value: model, label: model }))
})

const errorCategoryCodes = ['auth', 'rate_limit', 'quota', 'invalid_request', 'service_unavailable', 'server', 'internal', 'cyber']

const errorCategoryOptions = computed<FilterOption[]>(() =>
  errorCategoryCodes.map((c) => ({ value: c, label: t('usage.errors.categories.' + c) }))
)

// 状态码候选用固定常用列表(与管理端 UsageFilters 共用常量),不受当前页数据限制:
// 后端 status_code 过滤对全量生效,若只列当前页出现过的码,用户就选不到仅在后续页的码。
const errorStatusOptions = computed<FilterOption[]>(() => COMMON_ERROR_STATUS_CODES.map((c) => ({ value: c, label: String(c) })))

const errorFiltersActive = computed(() =>
  Boolean(errorFilter.value.model || errorFilter.value.category || errorFilter.value.api_key_id || errorFilter.value.status_code)
)

const applyErrorFilters = () => {
  errorPage.value = 1
  void loadErrors()
}

const clearErrorFilters = () => {
  errorFilter.value = { model: '', category: '', api_key_id: null, status_code: null }
  applyErrorFilters()
}

let abortController: AbortController | null = null
let statsReqSeq = 0
let modelStatsReqSeq = 0
let errorCountSeq = 0

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc',
})

// ---------- 用量筛选 ----------
const requestTypeOptions = computed<FilterOption[]>(() => [
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'live', label: t('usage.live') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
])
const compactionOptions = computed<FilterOption[]>(() => [
  { value: 'only', label: t('usage.compactionOnly') },
])
// 订阅不显示时只剩余额计费，「计费类型」筛选（余额/订阅）失去意义，整块隐藏。
const subscriptionFeatureEnabled = SITE_FEATURES.subscription
const billingTypeOptions = computed<FilterOption[]>(() => [
  { value: 0, label: t('admin.usage.billingTypeBalance') },
  { value: 1, label: t('admin.usage.billingTypeSubscription') },
])
const billingModeOptions = computed<FilterOption[]>(() => [
  { value: 'token', label: t('admin.usage.billingModeToken') },
  { value: 'per_request', label: t('admin.usage.billingModePerRequest') },
  { value: 'image', label: t('admin.usage.billingModeImage') },
  { value: 'video', label: t('admin.usage.billingModeVideo') },
])

const apiKeys = ref<ApiKey[]>([])
const apiKeysLoaded = ref(false)
const modelOptionValues = ref<string[]>(queryModel ? [queryModel] : [])

const apiKeyOptions = computed<FilterOption[]>(() => apiKeys.value.map((key) => ({ value: key.id, label: key.name })))
// 从密钥抽屉跳来（/usage?key=…）时密钥清单可能还没到，或那把密钥已删：筛选标签不显示内部 ID
const missingKeyLabel = computed(() => (apiKeysLoaded.value ? t('usage.deletedKey') : t('common.loading')))
const modelOptions = computed<FilterOption[]>(() => modelOptionValues.value.map((model) => ({ value: model, label: model })))

const keyChip = numberChip(() => filters.value.api_key_id, (value) => { filters.value.api_key_id = value ?? undefined })
const billingTypeChip = numberChip(() => filters.value.billing_type, (value) => { filters.value.billing_type = value })
const modelChip = computed<string | number>({
  get: () => filters.value.model ?? '',
  set: (value) => { filters.value.model = value === '' ? undefined : String(value) },
})
const requestTypeChip = computed<string | number>({
  get: () => filters.value.request_type ?? '',
  set: (value) => { filters.value.request_type = value === '' ? undefined : (value as UsageRequestType) },
})
const compactionChip = computed<string | number>({
  get: () => (filters.value.native_compaction_v2 ? 'only' : ''),
  set: (value) => { filters.value.native_compaction_v2 = value === '' ? null : true },
})
const billingModeChip = computed<string | number>({
  get: () => filters.value.billing_mode ?? '',
  set: (value) => { filters.value.billing_mode = value === '' ? null : String(value) },
})

/** 低频维度（类型 / 压缩 / 计费类型 / 计费方式）有值时，「更多筛选」保持展开 */
const moreFiltersActive = computed(() =>
  Boolean(filters.value.request_type) ||
  filters.value.native_compaction_v2 === true ||
  (filters.value.billing_type !== null && filters.value.billing_type !== undefined) ||
  Boolean(filters.value.billing_mode)
)
const moreFiltersOpen = ref(false)
const showMoreFilters = computed(() => moreFiltersOpen.value || moreFiltersActive.value)
const usageFiltersActive = computed(() => Boolean(filters.value.api_key_id || filters.value.model) || moreFiltersActive.value)

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

// ---------- 加载 ----------
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

/** 区间内的失败请求数（只在管理员允许用户看错误时查）：同一时间范围、同一密钥 / 模型筛选，只要 total */
const loadErrorCount = async () => {
  if (!errorViewEnabled.value) return
  const seq = ++errorCountSeq
  try {
    const resp = await usageAPI.listMyErrorRequests({
      page: 1,
      page_size: 1,
      start_date: startDate.value,
      end_date: endDate.value,
      api_key_id: filters.value.api_key_id ?? undefined,
      model: filters.value.model || undefined,
    })
    if (seq === errorCountSeq) errorCount.value = resp.total
  } catch (error) {
    if (seq !== errorCountSeq) return
    console.error('Failed to load error count:', error)
    errorCount.value = null
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
  void loadErrorCount()
  resetErrorRows()
  syncQuery()
}

const refreshData = () => {
  void loadLogs()
  void loadStats()
  void loadModelStats()
  void loadErrorCount()
  if (activeTab.value === 'errors') void loadErrors()
}

/** 费用分布里点一行：加上这个模型筛选；再点同一行取消 */
const toggleModelFilter = (model: string) => {
  filters.value.model = filters.value.model === model ? undefined : model
  applyFilters()
}

/** 清掉维度筛选（时间范围在页头，不跟着重置） */
const resetFilters = () => {
  filters.value = {
    start_date: startDate.value,
    end_date: endDate.value,
    request_type: undefined,
    native_compaction_v2: null,
    billing_type: null,
    billing_mode: null,
  }
  moreFiltersOpen.value = false
  applyFilters()
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  filters.value.start_date = range.startDate
  filters.value.end_date = range.endDate
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

// ---------- 请求详情 ----------
const detailLog = ref<UsageLog | null>(null)
const openDetail = (row: UsageLog) => {
  detailLog.value = row
}
const detailTotalTokens = computed(() => {
  const row = detailLog.value
  if (!row) return 0
  return (row.input_tokens || 0) + (row.output_tokens || 0) + (row.cache_creation_tokens || 0) + (row.cache_read_tokens || 0)
})

const requestTypeLabel = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'cyber') return t('usage.cyber')
  if (requestType === 'live') return t('usage.live')
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

// ---------- 导出 ----------
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
    const blob = new Blob(['﻿' + csvContent], { type: 'text/csv;charset=utf-8;' })
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

// ---------- 列设置 ----------
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

// 端点 / IP / 计费方式 / UA 默认收起：点行的详情抽屉里都有（muqian 2026-09-25）
const columnSettings = useColumnSettings({
  storageKey: 'user-usage-columns',
  version: 1,
  columns: allColumns,
  defaultHidden: ['endpoint', 'ip_address', 'billing_mode', 'user_agent'],
  alwaysVisible: ['created_at'],
})

// key 须与 UserErrorRequestsTable 的 allColumns 一致
const errAllColumns = computed<Column[]>(() => [
  { key: 'key_name', label: t('usage.errors.keyName') },
  { key: 'model', label: t('usage.errors.model') },
  { key: 'endpoint', label: t('usage.errors.endpoint') },
  { key: 'client_ip', label: 'IP' },
  { key: 'type', label: t('usage.type') },
  { key: 'category', label: t('usage.errors.category') },
  { key: 'status', label: t('usage.errors.status') },
  { key: 'message', label: t('usage.errors.message') },
  { key: 'created_at', label: t('usage.errors.time') },
  { key: 'user_agent', label: t('usage.userAgent') },
])

const errorColumnSettings = useColumnSettings({
  storageKey: 'user-usage-error-columns',
  version: 1,
  columns: errAllColumns,
  defaultHidden: ['user_agent'],
  alwaysVisible: ['status', 'created_at'],
})

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
    apiKeysLoaded.value = true
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
      model: errorFilter.value.model.trim() || undefined,
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

// 切到错误页签时带上用量页签当前的密钥 / 模型（从「失败 N · 查看」过来时就是同一批请求）；首次切过去才加载
watch(activeTab, (tab) => {
  if (tab === 'errors') {
    if (!errorFiltersActive.value && (filters.value.api_key_id || filters.value.model)) {
      errorFilter.value.api_key_id = filters.value.api_key_id ?? null
      errorFilter.value.model = filters.value.model ?? ''
      applyErrorFilters()
    } else if (errorRows.value.length === 0) {
      void loadErrors()
    }
  }
  syncQuery()
})

// 公开设置晚到时补上失败请求数，并兑现地址栏里的错误页签
watch(errorViewEnabled, (enabled) => {
  if (!enabled) return
  void loadErrorCount()
  if (wantsErrorTab) activeTab.value = 'errors'
})

onMounted(() => {
  void loadFilterOptions()
  refreshData()
})

onUnmounted(() => {
  abortController?.abort()
})
</script>
