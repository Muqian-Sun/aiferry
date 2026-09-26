<template>
  <!--
    操作日志（A4 列表模板）：标题右侧「⋯」（全部清理，确认 → TOTP 二次验证）；
    工具行 = 关键字搜索 + 方法 / 认证方式 / 结果 / 时间范围筛选标签 +「+ 更多筛选」（动作 / 操作者邮箱 / 客户端 IP，
    都是自由文本，挑出来才出现小输入框）+ 刷新；筛选即时生效。时间列保留精确时间；行尾「详情」图标。
  -->
  <AppLayout>
    <template #header-actions>
      <PopoverMenu width-class="w-48">
        <template #trigger="{ open }">
          <button
            type="button"
            class="btn btn-ghost btn-md px-2.5"
            :class="open ? 'bg-af-sunken text-af-ink' : ''"
            :title="t('common.more')"
            :aria-label="t('common.more')"
            data-testid="audit-tools"
          >
            <Icon name="more" size="md" />
          </button>
        </template>
        <MenuItem icon="trash" danger :disabled="checkingTotpStatus" data-testid="audit-clear" @click="openClearDialog">
          {{ t('admin.audit.clearAll') }}
        </MenuItem>
      </PopoverMenu>
    </template>

    <TablePageLayout>
      <template #filters>
        <ListToolbar>
          <SearchInput
            v-model="filters.q"
            compact
            class="w-full sm:w-64"
            :placeholder="t('admin.audit.filters.qPlaceholder')"
            data-testid="audit-search"
            @search="search"
          />
          <FilterChip
            v-model="filters.method"
            :label="t('admin.audit.filters.method')"
            :options="methodOptions"
            test-id="audit-filter-method"
            @change="search"
          />
          <FilterChip
            v-model="filters.auth_method"
            :label="t('admin.audit.filters.authMethod')"
            :options="authMethodOptions"
            test-id="audit-filter-auth-method"
            @change="search"
          />
          <FilterChip
            v-model="filters.success"
            :label="t('admin.audit.filters.result')"
            :options="resultOptions"
            test-id="audit-filter-result"
            @change="search"
          />
          <FilterChip
            :model-value="timeRangeChipValue"
            :label="t('admin.dashboard.timeRange')"
            :options="timeRangeOptions"
            test-id="audit-filter-time"
            @update:model-value="handleTimeRangeChange"
          />

          <!-- 自由文本筛选：从「+ 更多筛选」里挑出来的才显示 -->
          <template v-for="field in TEXT_FILTERS" :key="field.key">
            <input
              v-if="visibleTextFilters.has(field.key)"
              v-model="filters[field.key]"
              type="text"
              class="input h-8 w-full py-0 text-13 sm:w-40"
              :class="field.key === 'client_ip' || field.key === 'action' ? 'font-mono' : ''"
              :placeholder="t(field.label)"
              :title="t(field.label)"
              :aria-label="t(field.label)"
              :data-testid="`audit-filter-${field.key}`"
              @input="searchDebounced"
              @keyup.enter="search"
            />
          </template>
          <PopoverMenu align="start" width-class="w-48" :close-on-select="false">
            <template #trigger>
              <button
                type="button"
                class="inline-flex h-8 items-center gap-1 rounded-full px-2.5 text-13 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
                data-testid="audit-filter-more"
              >
                <Icon name="plus" size="xs" :stroke-width="2" />
                {{ t('admin.audit.filters.more') }}
              </button>
            </template>
            <MenuItem
              v-for="field in TEXT_FILTERS"
              :key="field.key"
              :checked="visibleTextFilters.has(field.key)"
              :data-testid="`audit-filter-more-${field.key}`"
              @click="toggleTextFilter(field.key)"
            >
              {{ t(field.label) }}
            </MenuItem>
          </PopoverMenu>
          <button
            v-if="hasActiveFilters"
            type="button"
            class="h-8 rounded-md px-2 text-13 text-af-ink-3 transition-colors hover:text-af-ink"
            data-testid="audit-filter-reset"
            @click="resetFilters"
          >
            {{ t('admin.audit.filters.reset') }}
          </button>

          <template #end>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="fetchLogs"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </template>
        </ListToolbar>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="logs" :loading="loading" row-key="id">
          <!-- 审计要精确时间，不写相对时间 -->
          <template #cell-created_at="{ value }">
            <span class="whitespace-nowrap tabular-nums text-af-ink-2">{{ formatTime(value) }}</span>
          </template>

          <template #cell-actor="{ row }">
            <div class="min-w-0 max-w-[220px]">
              <div class="truncate font-medium text-af-ink" :title="row.actor_email">
                {{ row.actor_email || '—' }}
              </div>
              <div class="mt-0.5 truncate text-xs text-af-ink-3">
                {{ row.actor_role }}<span v-if="row.auth_method"> · {{ authMethodLabel(row.auth_method) }}</span>
              </div>
            </div>
          </template>

          <!-- 列表只写动作；请求方法与路径（路径里带资源的内部 id）在详情里 -->
          <template #cell-action="{ row }">
            <div class="min-w-0 max-w-xs">
              <div class="truncate font-mono text-sm text-af-ink" :title="row.action">
                {{ row.action }}
              </div>
            </div>
          </template>

          <!-- 成功是常态：灰点；4xx 橙、5xx 红 -->
          <template #cell-status_code="{ row }">
            <div class="flex items-center gap-1.5 whitespace-nowrap">
              <span class="inline-block h-2 w-2 rounded-full" :class="statusDotClass(row.status_code)"></span>
              <span class="tabular-nums" :class="statusTextClass(row.status_code)">{{ row.status_code }}</span>
            </div>
          </template>

          <template #cell-latency_ms="{ value }">
            <span class="whitespace-nowrap tabular-nums text-af-ink-3">{{ value }} ms</span>
          </template>

          <template #cell-client_ip="{ value }">
            <span class="whitespace-nowrap font-mono text-af-ink-2">{{ value || '—' }}</span>
          </template>

          <template #cell-actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>

          <template #empty>
            <EmptyState :title="t('admin.audit.empty')">
              <template #icon>
                <Icon name="shield" size="xl" class="empty-state-icon h-10 w-10" />
              </template>
            </EmptyState>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="total > 0"
          :total="total"
          :page="page"
          :page-size="pageSize"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Detail dialog -->
    <BaseDialog
      :show="detailVisible"
      :title="t('admin.audit.detail.title')"
      width="wide"
      :close-on-click-outside="true"
      @close="detailVisible = false"
    >
      <div v-if="detailLoading" class="flex items-center justify-center py-16">
        <div class="flex flex-col items-center gap-3">
          <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-af-brand"></div>
          <div class="text-sm font-medium text-af-ink-3">{{ t('common.loading') }}</div>
        </div>
      </div>

      <div v-else-if="detail" class="space-y-5 py-2">
        <!-- 动作与结果 -->
        <div>
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span class="break-all font-mono text-base font-semibold text-af-ink">
              {{ detail.action }}
            </span>
            <span class="inline-flex items-center gap-1.5 text-sm">
              <span class="inline-block h-2 w-2 rounded-full" :class="statusDotClass(detail.status_code)"></span>
              <span class="tabular-nums" :class="statusTextClass(detail.status_code)">
                {{ detail.status_code }} {{ statusText(detail.status_code) }}
              </span>
            </span>
          </div>

          <div class="mt-2 break-all font-mono text-xs text-af-ink-2">
            <span class="font-semibold text-af-ink">{{ detail.method }}</span>
            {{ detail.path }}
          </div>

          <div class="mt-2 flex flex-wrap items-center gap-x-5 gap-y-1.5 text-xs text-af-ink-3">
            <span class="inline-flex items-center gap-1.5 tabular-nums">
              <Icon name="clock" size="xs" />
              {{ formatTime(detail.created_at) }}
            </span>
            <span class="tabular-nums">{{ t('admin.audit.detail.latency') }} {{ detail.latency_ms }} ms</span>
            <span v-if="detail.request_id" class="inline-flex items-center gap-1">
              {{ t('admin.audit.detail.requestId') }}
              <span class="break-all font-mono">{{ detail.request_id }}</span>
              <button
                type="button"
                class="shrink-0 rounded p-0.5 text-af-ink-4 transition-colors hover:bg-af-sunken hover:text-af-ink-2"
                :title="t('common.copy')"
                :aria-label="t('common.copy')"
                data-testid="audit-detail-copy-request-id"
                @click="copyToClipboard(detail.request_id, t('admin.usage.requestIdCopied'))"
              >
                <Icon name="copy" size="xs" />
              </button>
            </span>
          </div>
        </div>

        <!-- 操作者 / 认证 / 来源 -->
        <dl class="grid grid-cols-1 gap-4 border-y border-af-hairline py-4 sm:grid-cols-3">
          <div class="min-w-0">
            <dt class="text-xs text-af-ink-3">{{ t('admin.audit.columns.actor') }}</dt>
            <dd class="mt-1 break-all text-sm font-medium text-af-ink">{{ detail.actor_email || '—' }}</dd>
            <dd class="mt-0.5 text-xs text-af-ink-3">{{ detail.actor_role }}</dd>
          </div>

          <div class="min-w-0">
            <dt class="text-xs text-af-ink-3">{{ t('admin.audit.filters.authMethod') }}</dt>
            <dd class="mt-1 text-sm font-medium text-af-ink">{{ authMethodLabel(detail.auth_method) || '—' }}</dd>
            <dd v-if="detail.credential_masked" class="mt-0.5 break-all font-mono text-xs text-af-ink-3">
              {{ detail.credential_masked }}
            </dd>
          </div>

          <div class="min-w-0">
            <dt class="text-xs text-af-ink-3">{{ t('admin.audit.columns.clientIp') }}</dt>
            <dd class="mt-1 break-all font-mono text-sm font-medium text-af-ink">{{ detail.client_ip || '—' }}</dd>
          </div>
        </dl>

        <!-- User-Agent -->
        <section>
          <h4 class="mb-1.5 text-xs text-af-ink-3">{{ t('admin.audit.detail.userAgent') }}</h4>
          <div class="break-all rounded-lg bg-af-sunken p-3 font-mono text-xs leading-relaxed text-af-ink-2">
            {{ detail.user_agent || '—' }}
          </div>
        </section>

        <!-- Request body (redacted) -->
        <section v-if="detail.request_body">
          <h4 class="mb-1.5 text-xs text-af-ink-3">{{ t('admin.audit.detail.requestBody') }}</h4>
          <pre class="max-h-72 overflow-auto rounded-lg bg-af-sunken p-4 font-mono text-xs leading-relaxed text-af-ink-2">{{ prettyBody(detail.request_body) }}</pre>
        </section>

        <!-- Extra -->
        <section v-if="detail.extra && Object.keys(detail.extra).length">
          <h4 class="mb-1.5 text-xs text-af-ink-3">{{ t('admin.audit.detail.extra') }}</h4>
          <pre class="max-h-48 overflow-auto rounded-lg bg-af-sunken p-4 font-mono text-xs leading-relaxed text-af-ink-2">{{ JSON.stringify(detail.extra, null, 2) }}</pre>
        </section>
      </div>
    </BaseDialog>

    <!-- Custom time range dialog (与 /admin/ops 时间下拉一致的自定义范围，支持时分) -->
    <BaseDialog
      :show="showCustomTimeRangeDialog"
      :title="t('admin.ops.timeRange.custom')"
      width="narrow"
      @close="handleCustomTimeRangeCancel"
    >
      <div class="space-y-4 py-2">
        <div>
          <label class="input-label">{{ t('admin.ops.customTimeRange.startTime') }}</label>
          <input v-model="customStartTimeInput" type="datetime-local" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.ops.customTimeRange.endTime') }}</label>
          <input v-model="customEndTimeInput" type="datetime-local" class="input" />
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="handleCustomTimeRangeCancel">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!customStartTimeInput || !customEndTimeInput"
          @click="handleCustomTimeRangeConfirm"
        >
          {{ t('common.confirm') }}
        </button>
      </template>
    </BaseDialog>

    <!-- Clear confirmation → step-up TOTP -->
    <ConfirmDialog
      :show="clearConfirmVisible"
      :title="t('admin.audit.clearConfirm.title')"
      :message="t('admin.audit.clearConfirm.message')"
      :confirm-text="t('admin.audit.clearAll')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="onClearConfirmed"
      @cancel="clearConfirmVisible = false"
    />

    <!-- TOTP prompt for the clear operation -->
    <BaseDialog
      :show="clearTotpVisible"
      :title="t('admin.audit.clearConfirm.totpTitle')"
      width="narrow"
      :z-index="60"
      @close="cancelClearTotp"
    >
      <div class="py-2">
        <p class="text-sm text-af-ink-3">{{ t('admin.audit.clearConfirm.totpHint') }}</p>
        <input
          v-model.trim="clearTotpCode"
          type="text"
          inputmode="numeric"
          maxlength="6"
          autocomplete="one-time-code"
          class="input mt-4 text-center text-lg tracking-[0.5em]"
          placeholder="••••••"
          @keyup.enter="submitClear"
        />
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="clearing" @click="cancelClearTotp">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-danger"
          :disabled="clearing || clearTotpCode.length !== 6"
          @click="submitClear"
        >
          {{ clearing ? t('common.loading') : t('admin.audit.clearAll') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AuditLog } from '@/api/admin'
import { totpAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { FilterChip, ListToolbar, MenuItem, PopoverMenu, RowActions } from '@/components/admin/list'
import type { FilterOption, RowAction } from '@/components/admin/list'
import { useAppStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const logs = ref<AuditLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const filters = reactive({
  q: '',
  actor_email: '',
  action: '',
  client_ip: '',
  method: '',
  auth_method: '',
  success: ''
})

// 自由文本筛选（后端按包含 / 精确匹配）：默认不占工具行，从「+ 更多筛选」里挑出来才显示输入框
type TextFilterKey = 'action' | 'actor_email' | 'client_ip'
const TEXT_FILTERS: ReadonlyArray<{ key: TextFilterKey; label: string }> = [
  { key: 'action', label: 'admin.audit.filters.action' },
  { key: 'actor_email', label: 'admin.audit.filters.actorEmail' },
  { key: 'client_ip', label: 'admin.audit.filters.clientIp' }
]
const visibleTextFilters = reactive(new Set<TextFilterKey>())

async function toggleTextFilter(key: TextFilterKey) {
  if (visibleTextFilters.has(key)) {
    visibleTextFilters.delete(key)
    if (filters[key]) {
      filters[key] = ''
      search()
    }
    return
  }
  visibleTextFilters.add(key)
  await nextTick()
  document.querySelector<HTMLInputElement>(`[data-testid="audit-filter-${key}"]`)?.focus()
}

// 时间范围：预设窗口（同 /admin/ops 时间下拉）+ 自定义起止（datetime-local，支持时分）
const timeRange = ref('')
const customStartTime = ref('')
const customEndTime = ref('')
const showCustomTimeRangeDialog = ref(false)
const customStartTimeInput = ref('')
const customEndTimeInput = ref('')

const TIME_RANGE_MINUTES: Record<string, number> = {
  '30m': 30,
  '1h': 60,
  '6h': 6 * 60,
  '24h': 24 * 60,
  '7d': 7 * 24 * 60,
  '30d': 30 * 24 * 60
}

// 自定义范围生效时，标签显示起止时间（CUSTOM_ACTIVE 一项，已勾选）；「自定义…」一项始终在，
// 再点它可以改范围（筛选标签点当前值不会触发，所以两项分开）。
const CUSTOM_ACTIVE = 'custom:active'
const customActive = computed(() => timeRange.value === 'custom' && !!customStartTime.value && !!customEndTime.value)
const timeRangeChipValue = computed(() => (timeRange.value === 'custom' ? CUSTOM_ACTIVE : timeRange.value))
const timeRangeOptions = computed<FilterOption[]>(() => [
  { value: '30m', label: t('admin.ops.timeRange.30m') },
  { value: '1h', label: t('admin.ops.timeRange.1h') },
  { value: '6h', label: t('admin.ops.timeRange.6h') },
  { value: '24h', label: t('admin.ops.timeRange.24h') },
  { value: '7d', label: t('admin.ops.timeRange.7d') },
  { value: '30d', label: t('admin.ops.timeRange.30d') },
  ...(customActive.value
    ? [{ value: CUSTOM_ACTIVE, label: formatCustomTimeRangeLabel(customStartTime.value, customEndTime.value) }]
    : []),
  { value: 'custom', label: `${t('admin.ops.timeRange.custom')}…` }
])

// 标签里放得下：同一天只写一次日期（09-24 13:14 ~ 14:14）
function formatCustomTimeRangeLabel(startTime: string, endTime: string): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  const day = (d: Date) => `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  const clock = (d: Date) => `${pad(d.getHours())}:${pad(d.getMinutes())}`
  const start = new Date(startTime)
  const end = new Date(endTime)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return `${startTime} ~ ${endTime}`
  const sameDay = start.getFullYear() === end.getFullYear() && day(start) === day(end)
  return `${day(start)} ${clock(start)} ~ ${sameDay ? '' : `${day(end)} `}${clock(end)}`
}

function toDatetimeLocal(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function handleTimeRangeChange(val: string | number) {
  const value = String(val ?? '')
  if (value === CUSTOM_ACTIVE) return
  if (value === 'custom') {
    // 预填：已有自定义值沿用，否则默认最近1小时（本地时区）
    const now = new Date()
    customStartTimeInput.value = customStartTime.value || toDatetimeLocal(new Date(now.getTime() - 60 * 60 * 1000))
    customEndTimeInput.value = customEndTime.value || toDatetimeLocal(now)
    showCustomTimeRangeDialog.value = true
    return
  }
  timeRange.value = value
  search()
}

function handleCustomTimeRangeConfirm() {
  if (!customStartTimeInput.value || !customEndTimeInput.value) return
  customStartTime.value = customStartTimeInput.value
  customEndTime.value = customEndTimeInput.value
  timeRange.value = 'custom'
  showCustomTimeRangeDialog.value = false
  search()
}

function handleCustomTimeRangeCancel() {
  // 未确认不改变当前时间范围；筛选标签是受控的，展示值保持不变。
  showCustomTimeRangeDialog.value = false
}

const columns = computed<Column[]>(() => [
  { key: 'created_at', label: t('admin.audit.columns.time') },
  { key: 'actor', label: t('admin.audit.columns.actor') },
  { key: 'action', label: t('admin.audit.columns.action') },
  { key: 'status_code', label: t('admin.audit.columns.result') },
  { key: 'latency_ms', label: t('admin.audit.detail.latency') },
  { key: 'client_ip', label: t('admin.audit.columns.clientIp') },
  { key: 'actions', label: t('common.actions') }
])

const methodOptions: FilterOption[] = ['POST', 'PUT', 'PATCH', 'DELETE', 'GET'].map((method) => ({ value: method, label: method }))

const authMethodOptions: FilterOption[] = [
  { value: 'jwt', label: 'JWT' },
  { value: 'admin_api_key', label: 'Admin API Key' }
]

const resultOptions = computed<FilterOption[]>(() => [
  { value: 'true', label: t('admin.audit.filters.resultSuccess') },
  { value: 'false', label: t('admin.audit.filters.resultFailure') }
])

function authMethodLabel(method: string): string {
  return authMethodOptions.find((o) => o.value === method)?.label ?? method
}

// 行操作（A4）：只有「详情」，图标直接点
function rowActions(row: AuditLog): RowAction[] {
  return [{ key: 'detail', label: t('admin.audit.columns.detail'), icon: 'eye', primary: true, onSelect: () => openDetail(row.id) }]
}

function toRFC3339(local: string): string | undefined {
  if (!local) return undefined
  const d = new Date(local)
  if (Number.isNaN(d.getTime())) return undefined
  return d.toISOString()
}

function buildTimeRangeQuery(): { start_time?: string; end_time?: string } {
  if (timeRange.value === 'custom') {
    return {
      start_time: toRFC3339(customStartTime.value),
      end_time: toRFC3339(customEndTime.value)
    }
  }
  const minutes = TIME_RANGE_MINUTES[timeRange.value]
  if (!minutes) return {}
  return { start_time: new Date(Date.now() - minutes * 60 * 1000).toISOString() }
}

function buildQuery() {
  return {
    page: page.value,
    page_size: pageSize.value,
    q: filters.q.trim() || undefined,
    actor_email: filters.actor_email.trim() || undefined,
    action: filters.action.trim() || undefined,
    client_ip: filters.client_ip.trim() || undefined,
    method: filters.method || undefined,
    auth_method: filters.auth_method || undefined,
    success: filters.success || undefined,
    ...buildTimeRangeQuery()
  }
}

async function fetchLogs() {
  loading.value = true
  try {
    const res = await adminAPI.audit.list(buildQuery())
    logs.value = res.items
    total.value = res.total
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.loadFailed'))
  } finally {
    loading.value = false
  }
}

function search() {
  clearTimeout(searchTimer)
  page.value = 1
  fetchLogs()
}

// 自由文本输入：停手 300ms 再查（回车立即查）
let searchTimer: ReturnType<typeof setTimeout> | undefined
function searchDebounced() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(search, 300)
}

const hasActiveFilters = computed(
  () => Object.values(filters).some((value) => value.trim() !== '') || timeRange.value !== ''
)

function resetFilters() {
  filters.q = ''
  filters.actor_email = ''
  filters.action = ''
  filters.client_ip = ''
  filters.method = ''
  filters.auth_method = ''
  filters.success = ''
  visibleTextFilters.clear()
  timeRange.value = ''
  customStartTime.value = ''
  customEndTime.value = ''
  search()
}

function onPageChange(p: number) {
  page.value = p
  fetchLogs()
}

function onPageSizeChange(ps: number) {
  pageSize.value = ps
  page.value = 1
  fetchLogs()
}

// Detail dialog
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<AuditLog | null>(null)

async function openDetail(id: number) {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  try {
    detail.value = await adminAPI.audit.get(id)
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.loadFailed'))
    detailVisible.value = false
  } finally {
    detailLoading.value = false
  }
}

function prettyBody(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

// Clear-all flow: confirm → TOTP → clear
const clearConfirmVisible = ref(false)
const clearTotpVisible = ref(false)
const clearTotpCode = ref('')
const clearing = ref(false)
const checkingTotpStatus = ref(false)

// 与其他敏感操作一致：未启用 2FA 时直接提示去个人资料启用 TOTP，
// 而不是弹出一个无法完成的验证码输入框（后端会以 TOTP_NOT_SETUP 拒绝）。
async function openClearDialog() {
  if (checkingTotpStatus.value) return
  checkingTotpStatus.value = true
  try {
    const status = await totpAPI.getStatus()
    if (!status.enabled) {
      appStore.showError(t('stepUp.notEnabled'))
      return
    }
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.loadFailed'))
    return
  } finally {
    checkingTotpStatus.value = false
  }
  clearConfirmVisible.value = true
}

function onClearConfirmed() {
  clearConfirmVisible.value = false
  clearTotpCode.value = ''
  clearTotpVisible.value = true
}

function cancelClearTotp() {
  if (clearing.value) return
  clearTotpVisible.value = false
}

async function submitClear() {
  if (clearTotpCode.value.length !== 6) return
  clearing.value = true
  try {
    const res = await adminAPI.audit.clear(clearTotpCode.value)
    clearTotpVisible.value = false
    appStore.showSuccess(t('admin.audit.clearConfirm.success', { count: res.deleted }))
    search()
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.clearConfirm.failed'))
    clearTotpCode.value = ''
  } finally {
    clearing.value = false
  }
}

// Helpers
function formatTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

function statusText(status: number): string {
  return status < 400 ? t('admin.audit.filters.resultSuccess') : t('admin.audit.filters.resultFailure')
}

function statusDotClass(status: number): string {
  if (status >= 500) return 'bg-af-danger'
  if (status >= 400) return 'bg-af-warning'
  return 'bg-af-ink-4'
}

function statusTextClass(status: number): string {
  if (status >= 500) return 'text-af-danger'
  if (status >= 400) return 'text-af-warning'
  return 'text-af-ink-2'
}

onMounted(fetchLogs)
onUnmounted(() => clearTimeout(searchTimer))
</script>
