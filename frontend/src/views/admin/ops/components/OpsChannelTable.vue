<script setup lang="ts">
/**
 * 运维页「渠道」（2026-10-04 重排）：一张表同时给出「能不能用」（渠道可用性：出错停调 / 限流 / 过载）和
 * 「忙不忙」（并发占用、排队）。原来按平台分组的并发卡与「需要处理」里的渠道合到这里；
 * 渠道的历史可用率、首字延迟在「渠道状态」页，这里不重复。
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { opsAPI, type AccountAvailability, type AccountConcurrencyInfo } from '@/api/admin/ops'

const props = defineProps<{
  accountId: number | null
  refreshToken: number
}>()

const emit = defineEmits<{
  (e: 'loaded', channels: Array<{ id: number; name: string }>): void
}>()

const { t } = useI18n()

const VISIBLE_NORMAL = 5

const loading = ref(false)
const failed = ref(false)
const expanded = ref(false)
const availability = ref<Record<string, AccountAvailability>>({})
const concurrency = ref<Record<string, AccountConcurrencyInfo>>({})

type State = 'error' | 'paused' | 'rateLimited' | 'overloaded' | 'unavailable' | 'normal'

interface Row {
  id: number
  name: string
  state: State
  reason: string
  inUse: number | null
  max: number | null
  waiting: number | null
}

const STATE_ORDER: Record<State, number> = { error: 0, paused: 1, rateLimited: 2, overloaded: 3, unavailable: 4, normal: 5 }

function remaining(sec?: number): string {
  if (!sec || sec <= 0) return ''
  if (sec < 60) return t('admin.ops.page.channels.inSeconds', { n: sec })
  return t('admin.ops.page.channels.inMinutes', { n: Math.ceil(sec / 60) })
}

// 全部渠道（不看筛选）：给页头的渠道下拉做候选
const allIds = computed(() => {
  const ids = new Set<number>()
  Object.values(availability.value).forEach((a) => a && ids.add(a.account_id))
  Object.values(concurrency.value).forEach((c) => c && ids.add(c.account_id))
  return [...ids]
})

function channelName(id: number): string {
  return availability.value[String(id)]?.account_name || concurrency.value[String(id)]?.account_name || t('admin.ops.page.channels.deleted')
}

const rows = computed<Row[]>(() => {
  const list: Row[] = []
  allIds.value.forEach((id) => {
    if (props.accountId && id !== props.accountId) return
    const a = availability.value[String(id)]
    const c = concurrency.value[String(id)]
    let state: State = 'normal'
    let reason = ''
    if (a?.has_error) {
      state = 'error'
      reason = a.error_message || ''
    } else if (a?.is_rate_limited) {
      state = 'rateLimited'
      reason = remaining(a.rate_limit_remaining_sec)
    } else if (a?.is_overloaded) {
      state = 'overloaded'
      reason = remaining(a.overload_remaining_sec)
    } else if (a?.temp_unschedulable_until && Date.parse(a.temp_unschedulable_until) > Date.now()) {
      // 出错后按规则临时停调（原来落到「不可用」并直接显示原始 status「active」，2026-10-04 UI E2E）
      state = 'paused'
      reason = remaining(Math.ceil((Date.parse(a.temp_unschedulable_until) - Date.now()) / 1000))
    } else if (a && !a.is_available) {
      state = 'unavailable'
      reason = a.status === 'disabled' ? t('admin.ops.page.channels.disabled') : ''
    }
    list.push({
      id,
      name: channelName(id),
      state,
      reason,
      inUse: c ? c.current_in_use : null,
      max: c ? c.max_capacity : null,
      waiting: c ? c.waiting_in_queue : null
    })
  })
  const load = (r: Row) => (r.inUse != null && r.max ? r.inUse / r.max : 0)
  return list.sort((x, y) => STATE_ORDER[x.state] - STATE_ORDER[y.state] || load(y) - load(x) || x.name.localeCompare(y.name))
})

const problemRows = computed(() => rows.value.filter((r) => r.state !== 'normal'))
const normalRows = computed(() => rows.value.filter((r) => r.state === 'normal'))
const visibleRows = computed(() =>
  expanded.value ? rows.value : [...problemRows.value, ...normalRows.value.slice(0, VISIBLE_NORMAL)]
)
const hiddenCount = computed(() => (expanded.value ? 0 : Math.max(normalRows.value.length - VISIBLE_NORMAL, 0)))

const totals = computed(() => {
  let inUse = 0
  let max = 0
  let waiting = 0
  rows.value.forEach((r) => {
    inUse += r.inUse ?? 0
    max += r.max ?? 0
    waiting += r.waiting ?? 0
  })
  return { inUse, max, waiting }
})

function stateLabel(state: State): string {
  return t(`admin.ops.page.channels.state.${state}`)
}

function stateClass(state: State): string {
  if (state === 'error') return 'bg-af-danger-tint text-af-danger'
  if (state === 'normal') return 'text-af-ink-3'
  return 'bg-af-warning-tint text-af-warning'
}

async function load() {
  loading.value = true
  failed.value = false
  try {
    const [avail, conc] = await Promise.all([opsAPI.getAccountAvailabilityStats(), opsAPI.getConcurrencyStats()])
    availability.value = avail.account || {}
    concurrency.value = conc.account || {}
    emit(
      'loaded',
      allIds.value.map((id) => ({ id, name: channelName(id) })).sort((x, y) => x.name.localeCompare(y.name))
    )
  } catch (err) {
    console.error('[OpsChannelTable] failed to load channels', err)
    failed.value = true
  } finally {
    loading.value = false
  }
}

watch(() => props.refreshToken, load)
</script>

<template>
  <section class="border-t border-af-hairline py-4" data-testid="ops-channel-table">
    <div class="mb-2 flex flex-wrap items-baseline justify-between gap-2">
      <h2 class="text-sm font-semibold text-af-ink">{{ t('admin.ops.page.channels.title') }}</h2>
      <router-link to="/channels/status" class="text-xs text-af-ink-3 hover:text-af-ink">{{ t('admin.ops.page.channels.history') }} →</router-link>
    </div>
    <p v-if="failed" class="text-sm text-af-danger">{{ t('admin.ops.page.loadFailed') }}</p>
    <p v-else-if="!loading && !rows.length" class="text-sm text-af-ink-3">{{ t('admin.ops.page.channels.empty') }}</p>
    <div v-else class="overflow-x-auto">
      <table class="w-full min-w-[520px] text-sm">
        <thead>
          <tr class="border-b border-af-hairline text-left text-xs text-af-ink-3">
            <th class="py-2 pr-4 font-medium">{{ t('admin.ops.page.channels.name') }}</th>
            <th class="py-2 pr-4 font-medium">{{ t('admin.ops.page.channels.status') }}</th>
            <th class="py-2 pr-4 text-right font-medium">{{ t('admin.ops.page.channels.inUse') }}</th>
            <th class="py-2 text-right font-medium">{{ t('admin.ops.page.channels.waiting') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in visibleRows" :key="row.id" class="border-b border-af-hairline align-top">
            <td class="py-2 pr-4">
              <router-link :to="`/accounts?edit=${row.id}`" class="text-af-ink hover:underline">{{ row.name }}</router-link>
            </td>
            <td class="py-2 pr-4">
              <span class="inline-block rounded px-1.5 text-xs" :class="stateClass(row.state)">{{ stateLabel(row.state) }}</span>
              <span v-if="row.reason" class="ml-2 break-all text-xs text-af-ink-3">{{ row.reason }}</span>
            </td>
            <td class="py-2 pr-4 text-right tabular-nums text-af-ink-2">{{ row.inUse == null ? '—' : `${row.inUse} / ${row.max}` }}</td>
            <td class="py-2 text-right tabular-nums" :class="(row.waiting ?? 0) > 0 ? 'text-af-warning' : 'text-af-ink-2'">{{ row.waiting ?? '—' }}</td>
          </tr>
          <tr v-if="hiddenCount > 0 || (expanded && normalRows.length > VISIBLE_NORMAL)">
            <td colspan="4" class="py-2">
              <button type="button" class="text-xs text-af-ink-3 hover:text-af-ink" @click="expanded = !expanded">
                {{ expanded ? t('admin.ops.page.channels.showLess') : t('admin.ops.page.channels.showMore', { count: hiddenCount }) }}
              </button>
            </td>
          </tr>
          <tr v-if="!props.accountId" class="text-af-ink-2">
            <td class="py-2 pr-4 font-medium">{{ t('admin.ops.page.channels.total') }}</td>
            <td class="py-2 pr-4"></td>
            <td class="py-2 pr-4 text-right font-medium tabular-nums">{{ totals.inUse }} / {{ totals.max }}</td>
            <td class="py-2 text-right font-medium tabular-nums">{{ totals.waiting }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
