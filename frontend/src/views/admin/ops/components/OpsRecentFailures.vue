<script setup lang="ts">
/**
 * 运维页「最近失败的请求」（2026-10-04 重排）：原来要点「明细」进弹窗才看得到，现在直接列在页面上。
 * - 失败 = 用户收到错误、不是业务限制的（错误列表 errors 视图；没选到渠道也在里面）
 * - 换渠道恢复 = 上游出错、换渠道后成功的（上游错误列表里状态码小于 400 的行），用户没受影响
 * 点一行打开现有的错误详情；「查看全部」打开现有的错误列表。
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { opsAPI, type OpsErrorListQueryParams, type OpsErrorLog } from '@/api/admin/ops'

type Kind = 'all' | 'failed' | 'recovered' | 'routing'

const props = defineProps<{
  params: Pick<OpsErrorListQueryParams, 'time_range' | 'start_time' | 'end_time' | 'model' | 'account_id'>
  refreshToken: number
}>()

const emit = defineEmits<{
  (e: 'openError', id: number, type: 'request' | 'upstream'): void
  (e: 'openAll', type: 'request' | 'upstream'): void
}>()

const { t } = useI18n()

const LIMIT = 10
const kind = ref<Kind>('all')
const loading = ref(false)
const failed = ref(false)
const rows = ref<OpsErrorLog[]>([])

const kinds = computed(() => [
  { key: 'all' as Kind, label: t('admin.ops.page.failures.kinds.all') },
  { key: 'failed' as Kind, label: t('admin.ops.page.failures.kinds.failed') },
  { key: 'recovered' as Kind, label: t('admin.ops.page.failures.kinds.recovered') },
  { key: 'routing' as Kind, label: t('admin.ops.page.failures.kinds.routing') }
])

function isRecovered(row: OpsErrorLog): boolean {
  return row.status_code > 0 && row.status_code < 400
}

async function listFailed(phase?: string): Promise<OpsErrorLog[]> {
  const res = await opsAPI.listErrorLogs({ ...props.params, view: 'errors', phase, page: 1, page_size: LIMIT })
  return res.items || []
}

async function listRecovered(): Promise<OpsErrorLog[]> {
  // 上游错误列表里既有失败的、也有换渠道后成功的：多取一些，只留状态码小于 400 的
  const res = await opsAPI.listUpstreamErrors({ ...props.params, view: 'all', page: 1, page_size: 100 })
  return (res.items || []).filter(isRecovered).slice(0, LIMIT)
}

let seq = 0
async function load() {
  const current = ++seq
  loading.value = true
  failed.value = false
  try {
    let list: OpsErrorLog[]
    if (kind.value === 'failed') list = await listFailed()
    else if (kind.value === 'routing') list = await listFailed('routing')
    else if (kind.value === 'recovered') list = await listRecovered()
    else {
      const [a, b] = await Promise.all([listFailed(), listRecovered()])
      list = [...a, ...b].sort((x, y) => (x.created_at < y.created_at ? 1 : -1)).slice(0, LIMIT)
    }
    if (current === seq) rows.value = list
  } catch (err) {
    console.error('[OpsRecentFailures] failed to load', err)
    if (current === seq) failed.value = true
  } finally {
    if (current === seq) loading.value = false
  }
}

watch(() => [props.refreshToken, kind.value], load)

function phaseLabel(row: OpsErrorLog): string {
  switch (row.phase) {
    case 'upstream':
    case 'account_auth':
      return t('admin.ops.page.phase.upstream')
    case 'routing':
      return t('admin.ops.page.phase.routing')
    case 'request':
    case 'auth':
      return t('admin.ops.page.phase.client')
    default:
      return t('admin.ops.page.phase.other')
  }
}

function formatTime(value: string): string {
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString()
}

function open(row: OpsErrorLog) {
  emit('openError', row.id, isRecovered(row) ? 'upstream' : 'request')
}
</script>

<template>
  <section class="border-t border-af-hairline py-4" data-testid="ops-recent-failures">
    <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
      <h2 class="text-sm font-semibold text-af-ink">{{ t('admin.ops.page.failures.title') }}</h2>
      <div class="flex flex-wrap items-center gap-1">
        <button
          v-for="item in kinds"
          :key="item.key"
          type="button"
          class="rounded px-2 py-0.5 text-xs"
          :class="kind === item.key ? 'bg-af-ink text-af-sheet' : 'text-af-ink-3 hover:text-af-ink'"
          @click="kind = item.key"
        >
          {{ item.label }}
        </button>
        <button type="button" class="ml-2 text-xs text-af-ink-3 hover:text-af-ink" @click="emit('openAll', kind === 'recovered' ? 'upstream' : 'request')">
          {{ t('admin.ops.page.viewAll') }} →
        </button>
      </div>
    </div>
    <p v-if="failed" class="text-sm text-af-danger">{{ t('admin.ops.page.loadFailed') }}</p>
    <p v-else-if="!loading && !rows.length" class="text-sm text-af-ink-3">{{ t(`admin.ops.page.failures.emptyBy.${kind}`) }}</p>
    <div v-else class="overflow-x-auto">
      <table class="w-full min-w-[760px] text-sm">
        <thead>
          <tr class="border-b border-af-hairline text-left text-xs text-af-ink-3">
            <th class="py-2 pr-3 font-medium">{{ t('admin.ops.page.failures.time') }}</th>
            <th class="py-2 pr-3 font-medium">{{ t('admin.ops.page.failures.result') }}</th>
            <th class="py-2 pr-3 font-medium">{{ t('admin.ops.page.failures.user') }}</th>
            <th class="py-2 pr-3 font-medium">{{ t('admin.ops.page.failures.model') }}</th>
            <th class="py-2 pr-3 font-medium">{{ t('admin.ops.page.failures.channel') }}</th>
            <th class="py-2 pr-3 font-medium">{{ t('admin.ops.page.failures.phase') }}</th>
            <th class="py-2 pr-3 text-right font-medium">{{ t('admin.ops.page.failures.status') }}</th>
            <th class="py-2 font-medium">{{ t('admin.ops.page.failures.message') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="row in rows"
            :key="row.id"
            class="cursor-pointer border-b border-af-hairline align-top hover:bg-af-sunken"
            @click="open(row)"
          >
            <td class="whitespace-nowrap py-2 pr-3 tabular-nums text-af-ink-2">{{ formatTime(row.created_at) }}</td>
            <td class="py-2 pr-3">
              <span
                class="inline-block rounded px-1.5 text-xs"
                :class="isRecovered(row) ? 'bg-af-warning-tint text-af-warning' : 'bg-af-danger-tint text-af-danger'"
              >
                {{ isRecovered(row) ? t('admin.ops.page.failures.recoveredTag') : t('admin.ops.page.failures.failedTag') }}
              </span>
            </td>
            <td class="py-2 pr-3 text-af-ink-2">{{ row.user_email || '—' }}</td>
            <td class="py-2 pr-3 font-mono text-xs text-af-ink">{{ row.requested_model || row.model || '—' }}</td>
            <td class="py-2 pr-3 text-af-ink-2">{{ row.account_name || '—' }}</td>
            <td class="py-2 pr-3 text-af-ink-2">{{ phaseLabel(row) }}</td>
            <td class="py-2 pr-3 text-right tabular-nums text-af-ink-2">{{ isRecovered(row) ? '—' : row.status_code || '—' }}</td>
            <td class="max-w-[320px] truncate py-2 text-af-ink-3" :title="row.message">{{ row.message || '—' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
