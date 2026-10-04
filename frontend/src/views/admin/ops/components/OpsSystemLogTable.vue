<script setup lang="ts">
/**
 * 运维页「系统日志」（2026-10-04 重排）：先给关键词搜索和级别，其余筛选收进「更多筛选」；
 * 时间跟页头的时间范围走，不再单独选；运行时日志配置挪进「日志配置」弹窗；清理收进更多筛选、页面内确认。
 */
import { computed, reactive, ref, watch } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { opsAPI, type OpsSystemLog, type OpsSystemLogQuery, type OpsSystemLogSinkHealth } from '@/api/admin/ops'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import EntityPicker from '@/components/admin/form/EntityPicker.vue'
import OpsRuntimeLogConfigDialog from './OpsRuntimeLogConfigDialog.vue'
import { parseTimeRangeMinutes } from '../utils/opsFormatters'

const { t } = useI18n()

// 与 DataTable 一致：< 768px 切换为卡片视图，避免宽表在移动端被截断。
const isDesktopViewport = useMediaQuery('(min-width: 768px)')

const props = withDefaults(defineProps<{
  /** 页头选的时间范围（time_range 或 start_time / end_time） */
  timeParams: { time_range?: string; start_time?: string; end_time?: string }
  refreshToken?: number
}>(), {
  refreshToken: 0
})

const loading = ref(false)
const logs = ref<OpsSystemLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const showMoreFilters = ref(false)
const showRuntimeConfig = ref(false)
const confirmCleanup = ref(false)

const health = ref<OpsSystemLogSinkHealth>({
  queue_depth: 0,
  queue_capacity: 0,
  dropped_count: 0,
  write_failed_count: 0,
  written_count: 0,
  avg_write_delay_ms: 0
})

// 用户 / 密钥 / 渠道按名字选（EntityPicker），存的是选中对象的 id，不让人手填
const filters = reactive({
  q: '',
  level: '',
  host: '',
  component: '',
  request_id: '',
  client_request_id: '',
  user_id: undefined as number | undefined,
  api_key_id: undefined as number | undefined,
  account_id: undefined as number | undefined,
  model: ''
})

// 换了用户，原来选的密钥不一定属于新用户，一并清掉
const onUserFilterChange = (userId: number | undefined) => {
  filters.user_id = userId
  filters.api_key_id = undefined
}

const filterLevelOptions = computed(() => [
  { value: '', label: t('admin.ops.systemLogs.all') },
  { value: 'debug', label: 'debug' },
  { value: 'info', label: 'info' },
  { value: 'warn', label: 'warn' },
  { value: 'error', label: 'error' }
])

const moreFilterCount = computed(
  () =>
    [filters.host, filters.component, filters.request_id, filters.client_request_id, filters.model].filter((v) => v.trim()).length +
    [filters.user_id, filters.api_key_id, filters.account_id].filter(Boolean).length
)

const levelBadgeClass = (level: string) => {
  const v = String(level || '').toLowerCase()
  if (v === 'error' || v === 'fatal') return 'bg-af-danger-tint text-af-danger'
  if (v === 'warn' || v === 'warning') return 'bg-af-warning-tint text-af-warning'
  if (v === 'debug') return 'bg-af-sunken text-af-ink-2'
  return 'bg-af-sunken text-af-ink-2'
}

const formatTime = (value: string) => {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

const getExtraString = (extra: Record<string, any> | undefined, key: string) => {
  if (!extra) return ''
  const v = extra[key]
  if (v == null) return ''
  if (typeof v === 'string') return v.trim()
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  return ''
}

const formatSystemLogDetail = (row: OpsSystemLog) => {
  const parts: string[] = []
  const msg = String(row.message || '').trim()
  if (msg) parts.push(msg)

  const extra = row.extra || {}
  const statusCode = getExtraString(extra, 'status_code')
  const latencyMs = getExtraString(extra, 'latency_ms')
  const method = getExtraString(extra, 'method')
  const path = getExtraString(extra, 'path')
  const clientIP = getExtraString(extra, 'client_ip')
  const protocol = getExtraString(extra, 'protocol')

  const accessParts: string[] = []
  if (statusCode) accessParts.push(`status=${statusCode}`)
  if (latencyMs) accessParts.push(`latency_ms=${latencyMs}`)
  if (method) accessParts.push(`method=${method}`)
  if (path) accessParts.push(`path=${path}`)
  if (clientIP) accessParts.push(`ip=${clientIP}`)
  if (protocol) accessParts.push(`proto=${protocol}`)
  if (accessParts.length > 0) parts.push(accessParts.join(' '))

  // 只留请求 ID（排查时按它搜）；日志里没有用户 / 密钥 / 渠道的名字，内部 id 不显示
  const corrParts: string[] = []
  if (row.request_id) corrParts.push(`req=${row.request_id}`)
  if (row.client_request_id) corrParts.push(`client_req=${row.client_request_id}`)
  if (row.platform) corrParts.push(`platform=${row.platform}`)
  if (row.model) corrParts.push(`model=${row.model}`)
  if (corrParts.length > 0) parts.push(corrParts.join(' '))

  const errors = getExtraString(extra, 'errors')
  if (errors) parts.push(`errors=${errors}`)
  const err = getExtraString(extra, 'err') || getExtraString(extra, 'error')
  if (err) parts.push(`error=${err}`)

  // 用空格拼接，交给 CSS 自动换行，尽量“填满再换行”。
  return parts.join('  ')
}

const toRFC3339 = (value: string) => {
  if (!value) return undefined
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return undefined
  return d.toISOString()
}

const filterPayload = () => ({
  host: filters.host.trim() || undefined,
  level: filters.level.trim() || undefined,
  component: filters.component.trim() || undefined,
  request_id: filters.request_id.trim() || undefined,
  client_request_id: filters.client_request_id.trim() || undefined,
  user_id: filters.user_id,
  api_key_id: filters.api_key_id,
  account_id: filters.account_id,
  model: filters.model.trim() || undefined,
  q: filters.q.trim() || undefined
})

const fetchLogs = async () => {
  loading.value = true
  try {
    const res = await opsAPI.listSystemLogs({
      page: page.value,
      page_size: pageSize.value,
      time_range: props.timeParams.time_range as OpsSystemLogQuery['time_range'],
      start_time: props.timeParams.start_time,
      end_time: props.timeParams.end_time,
      ...filterPayload()
    })
    logs.value = res.items || []
    total.value = res.total || 0
  } catch (err: any) {
    console.error('[OpsSystemLogTable] Failed to fetch logs', err)
  } finally {
    loading.value = false
  }
}

const fetchHealth = async () => {
  try {
    health.value = await opsAPI.getSystemLogSinkHealth()
  } catch {
    // 忽略健康数据读取失败，不影响主流程。
  }
}

const cleanupCurrentFilter = async () => {
  confirmCleanup.value = false
  try {
    // 清理接口只认具体的起止时间：页头是「近 1 小时」这类相对范围时先换算，免得删到范围外的日志
    let start = props.timeParams.start_time
    let end = props.timeParams.end_time
    if (!start || !end) {
      const now = new Date()
      end = now.toISOString()
      start = new Date(now.getTime() - parseTimeRangeMinutes(props.timeParams.time_range || '1h') * 60_000).toISOString()
    }
    await opsAPI.cleanupSystemLogs({ ...filterPayload(), start_time: toRFC3339(start), end_time: toRFC3339(end) })
    page.value = 1
    await Promise.all([fetchLogs(), fetchHealth()])
  } catch (err: any) {
    console.error('[OpsSystemLogTable] Failed to cleanup logs', err)
  }
}

const resetFilters = () => {
  filters.q = ''
  filters.level = ''
  filters.host = ''
  filters.component = ''
  filters.request_id = ''
  filters.client_request_id = ''
  filters.user_id = undefined
  filters.api_key_id = undefined
  filters.account_id = undefined
  filters.model = ''
  page.value = 1
  fetchLogs()
}

watch(
  () => [props.refreshToken, props.timeParams.time_range, props.timeParams.start_time, props.timeParams.end_time],
  () => {
    page.value = 1
    fetchLogs()
    fetchHealth()
  }
)

const onPageChange = (next: number) => {
  page.value = next
  fetchLogs()
}

const onPageSizeChange = (next: number) => {
  pageSize.value = next
  page.value = 1
  fetchLogs()
}

const applyFilters = () => {
  page.value = 1
  fetchLogs()
}

const hasData = computed(() => logs.value.length > 0)

// 不在挂载时自己加载：页面每次刷新递增 refreshToken，第一次刷新就会触发上面的 watch（避免首屏请求两次）
</script>

<template>
  <section class="border-t border-af-hairline py-4" data-testid="ops-system-logs">
    <div class="mb-3 flex flex-wrap items-baseline justify-between gap-2">
      <h2 class="text-sm font-semibold text-af-ink">{{ t('admin.ops.systemLogs.title') }}</h2>
      <div class="flex flex-wrap items-center gap-3 text-xs text-af-ink-3">
        <span class="tabular-nums">{{ t('admin.ops.systemLogs.queue') }} {{ health.queue_depth }}/{{ health.queue_capacity }}</span>
        <span class="tabular-nums">{{ t('admin.ops.systemLogs.written') }} {{ health.written_count }}</span>
        <span class="tabular-nums" :class="health.dropped_count > 0 ? 'text-af-warning' : ''">{{ t('admin.ops.systemLogs.dropped') }} {{ health.dropped_count }}</span>
        <span class="tabular-nums" :class="health.write_failed_count > 0 ? 'text-af-danger' : ''">{{ t('admin.ops.systemLogs.failed') }} {{ health.write_failed_count }}</span>
        <button type="button" class="text-af-ink-3 hover:text-af-ink" @click="showRuntimeConfig = true">{{ t('admin.ops.page.logs.config') }}</button>
      </div>
    </div>
    <p v-if="health.last_error" class="mb-2 text-xs text-af-danger">{{ t('admin.ops.systemLogs.latestWriteError') }} {{ health.last_error }}</p>

    <form class="mb-3 flex flex-wrap items-center gap-2" @submit.prevent="applyFilters">
      <input
        v-model="filters.q"
        type="search"
        class="input w-full sm:w-80"
        :placeholder="t('admin.ops.page.logs.searchPlaceholder')"
        data-testid="ops-logs-search"
      />
      <Select v-model="filters.level" class="w-full sm:w-36" :options="filterLevelOptions" @change="applyFilters" />
      <button type="button" class="btn btn-primary btn-sm" @click="applyFilters">{{ t('admin.ops.systemLogs.search') }}</button>
      <button type="button" class="btn btn-secondary btn-sm" @click="showMoreFilters = !showMoreFilters">
        {{ t('admin.ops.page.logs.moreFilters') }}<template v-if="moreFilterCount"> · {{ moreFilterCount }}</template>
      </button>
    </form>

    <div v-if="showMoreFilters" class="mb-3 border-y border-af-hairline py-3">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.user') }}
          <EntityPicker :model-value="filters.user_id" kind="user" class="mt-1" @update:model-value="onUserFilterChange" />
        </div>
        <div class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.apiKey') }}
          <EntityPicker v-model="filters.api_key_id" kind="apiKey" class="mt-1" :user-id="filters.user_id" />
        </div>
        <div class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.account') }}
          <EntityPicker v-model="filters.account_id" kind="channel" class="mt-1" />
        </div>
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.model') }}
          <input v-model="filters.model" type="text" class="input mt-1" />
        </label>
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.requestId') }}
          <input v-model="filters.request_id" type="text" class="input mt-1" />
        </label>
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.clientRequestId') }}
          <input v-model="filters.client_request_id" type="text" class="input mt-1" />
        </label>
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.component') }}
          <input v-model="filters.component" type="text" class="input mt-1" :placeholder="t('admin.ops.systemLogs.componentPlaceholder')" />
        </label>
        <label class="text-xs text-af-ink-2">
          {{ t('admin.ops.systemLogs.host') }}
          <input v-model="filters.host" type="text" class="input mt-1" />
        </label>
      </div>
      <div class="mt-3 flex flex-wrap justify-between gap-2">
        <div class="flex gap-2">
          <button type="button" class="btn btn-primary btn-sm" @click="applyFilters">{{ t('admin.ops.systemLogs.search') }}</button>
          <button type="button" class="btn btn-secondary btn-sm" @click="resetFilters">{{ t('common.reset') }}</button>
        </div>
        <button type="button" class="btn btn-danger btn-sm" @click="confirmCleanup = true">{{ t('admin.ops.systemLogs.cleanCurrentFilters') }}</button>
      </div>
    </div>

    <div>
      <div v-if="loading" class="px-4 py-8 text-center text-sm text-af-ink-3">{{ t('common.loading') }}</div>
      <div v-else-if="!hasData" class="px-4 py-8 text-center text-sm text-af-ink-3">{{ t('admin.ops.systemLogs.empty') }}</div>
      <div v-else-if="!isDesktopViewport" class="divide-y divide-af-hairline">
        <div v-for="row in logs" :key="row.id" class="space-y-1.5 p-3">
          <div class="flex items-center justify-between gap-2">
            <span class="inline-flex rounded-full px-2 py-0.5 text-xs font-semibold" :class="levelBadgeClass(row.level)">
              {{ row.level }}
            </span>
            <span class="text-xs text-af-ink-3">{{ formatTime(row.created_at) }}</span>
          </div>
          <div v-if="row.host" class="truncate text-xs text-af-ink-3" :title="row.host">
            {{ row.host }}
          </div>
          <div class="whitespace-normal break-all text-xs text-af-ink-2">
            {{ formatSystemLogDetail(row) }}
          </div>
        </div>
      </div>
      <div v-else class="overflow-auto">
        <table class="min-w-full table-fixed divide-y divide-af-hairline">
          <thead class="bg-af-sunken">
            <tr>
              <th class="w-[170px] px-3 py-2 text-left text-[11px] font-semibold text-af-ink-3">{{ t('admin.ops.systemLogs.time') }}</th>
              <th class="w-[160px] px-3 py-2 text-left text-[11px] font-semibold text-af-ink-3">{{ t('admin.ops.systemLogs.host') }}</th>
              <th class="w-[80px] px-3 py-2 text-left text-[11px] font-semibold text-af-ink-3">{{ t('admin.ops.systemLogs.level') }}</th>
              <th class="px-3 py-2 text-left text-[11px] font-semibold text-af-ink-3">{{ t('admin.ops.systemLogs.logDetails') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-af-hairline">
            <tr v-for="row in logs" :key="row.id" class="align-top">
              <td class="px-3 py-2 text-xs text-af-ink-2">{{ formatTime(row.created_at) }}</td>
              <td class="px-3 py-2 text-xs text-af-ink-2">
                <span class="block truncate" :title="row.host || '-'">{{ row.host || '-' }}</span>
              </td>
              <td class="px-3 py-2 text-xs">
                <span class="inline-flex rounded-full px-2 py-0.5 font-semibold" :class="levelBadgeClass(row.level)">
                  {{ row.level }}
                </span>
              </td>
              <td class="px-3 py-2 text-xs text-af-ink-2 whitespace-normal break-all">
                {{ formatSystemLogDetail(row) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <Pagination
        :total="total"
        :page="page"
        :page-size="pageSize"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </div>

    <OpsRuntimeLogConfigDialog :show="showRuntimeConfig" @close="showRuntimeConfig = false" @saved="fetchHealth" />
    <ConfirmDialog
      :show="confirmCleanup"
      :title="t('admin.ops.systemLogs.cleanCurrentFilters')"
      :message="t('admin.ops.systemLogs.cleanupConfirm')"
      danger
      @confirm="cleanupCurrentFilter"
      @cancel="confirmCleanup = false"
    />
  </section>
</template>
