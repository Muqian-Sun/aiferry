<template>
  <!--
    调度 · 请求轨迹：最近的请求，每行画成渠道链（失败的尝试带状态码，最后一段是结果），点开看每次选渠道的候选与原因。
    第一页且没打开详情时跟着页面刷新；翻页或看详情时不动，免得内容跳。
  -->
  <SheetSection :title="t('admin.scheduling.trace.title')">
    <template #actions>
      <FilterChip v-model="entryFilter" :label="t('admin.scheduling.trace.filters.model')" :options="modelOptions" test-id="scheduling-trace-filter-model" />
      <FilterChip v-model="protocolFilter" :label="t('admin.scheduling.trace.filters.protocol')" :options="protocolOptions" test-id="scheduling-trace-filter-protocol" />
      <SegmentedControl v-model="outcomeFilter" :options="outcomeOptions" :label="t('admin.scheduling.trace.filters.outcome')" test-id-prefix="scheduling-trace-outcome" />
    </template>

    <p v-if="loadError" class="mb-3 text-xs text-af-danger">{{ t('admin.scheduling.trace.loadFailed') }}</p>
    <StatusState v-if="!page && loading" kind="loading" :title="t('common.loading')" />
    <p v-else-if="page && page.items.length === 0" class="py-10 text-center text-sm text-af-ink-3">{{ t('admin.scheduling.trace.empty') }}</p>
    <template v-else-if="page">
      <ul class="-mx-4 border-t border-af-hairline md:-mx-6" data-testid="scheduling-traces">
        <li v-for="trace in page.items" :key="trace.id" class="border-b border-af-hairline">
          <button
            type="button"
            class="block w-full px-4 py-3 text-left transition-colors hover:bg-af-sunken/60 md:px-6"
            data-testid="scheduling-trace-row"
            @click="selected = trace"
          >
            <div class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 text-xs text-af-ink-3">
              <span class="tabular-nums">{{ new Date(trace.created_at).toLocaleTimeString() }} · {{ trace.model || '-' }} · {{ protocolLabel(trace.inbound_protocol) }}</span>
              <span class="tabular-nums">{{ formatMs(trace.duration_ms) }} · <span :class="OUTCOME_TONE[trace.outcome]">{{ labelOr('admin.scheduling.trace.outcomes', trace.outcome) }}</span></span>
            </div>
            <p class="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-1 text-13" data-testid="scheduling-trace-chain">
              <template v-for="(segment, index) in chainOf(trace)" :key="index">
                <span v-if="index > 0" class="text-af-ink-4" aria-hidden="true">→</span>
                <span :class="SEGMENT_TONE[segment.tone]">{{ segment.text }}</span>
              </template>
              <span v-if="trace.sticky_hit" class="ml-1 text-xs text-af-ink-3">· {{ t('admin.scheduling.trace.stickyHit') }}</span>
            </p>
          </button>
        </li>
      </ul>
      <div class="mt-4 flex items-center justify-between text-xs text-af-ink-3">
        <span>{{ t('admin.scheduling.trace.total', { count: page.total }) }}</span>
        <div class="flex items-center gap-3">
          <button type="button" class="hover:text-af-ink disabled:opacity-40" :disabled="pageNumber <= 1" data-testid="scheduling-trace-prev" @click="pageNumber--">
            {{ t('admin.scheduling.trace.prev') }}
          </button>
          <span class="tabular-nums">{{ pageNumber }} / {{ pageCount }}</span>
          <button type="button" class="hover:text-af-ink disabled:opacity-40" :disabled="pageNumber >= pageCount" data-testid="scheduling-trace-next" @click="pageNumber++">
            {{ t('admin.scheduling.trace.next') }}
          </button>
        </div>
      </div>
    </template>
  </SheetSection>

  <SchedulingTraceDrawer :trace="selected" :overview="overview" @close="selected = null" />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, toRef, watch } from 'vue'
import {
  getSchedulingTraces,
  type SchedulingOverview,
  type SchedulingProtocol,
  type SchedulingTrace,
  type SchedulingTraceOutcome,
  type SchedulingTraceOutcomeFilter,
  type SchedulingTracePage,
  type SchedulingWindow
} from '@/api/admin/scheduling'
import FilterChip from '@/components/common/FilterChip.vue'
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import SchedulingTraceDrawer from './SchedulingTraceDrawer.vue'
import { useSchedulingLabels } from './useSchedulingLabels'

const props = defineProps<{ overview: SchedulingOverview | null; timeWindow: SchedulingWindow; refreshToken: number }>()
const { t, channelName, labelOr, protocolLabel } = useSchedulingLabels(toRef(props, 'overview'))

const PAGE_SIZE = 20
const PROTOCOLS: SchedulingProtocol[] = ['anthropic', 'chat_completions', 'responses', 'gemini']
const OUTCOMES: SchedulingTraceOutcomeFilter[] = ['', 'switched', 'failed', 'no_channel']
const OUTCOME_TONE: Record<SchedulingTraceOutcome, string> = {
  success: 'text-af-ink-2',
  failed: 'text-af-danger',
  no_channel: 'text-af-danger',
  canceled: 'text-af-ink-3'
}
type SegmentTone = 'ok' | 'fail' | 'muted'
const SEGMENT_TONE: Record<SegmentTone, string> = { ok: 'text-af-ink', fail: 'text-af-danger', muted: 'text-af-ink-3' }

// ---------- 筛选 ----------
const entryFilter = ref('')
const protocolFilter = ref('')
const outcomeFilter = ref<SchedulingTraceOutcomeFilter>('')
const pageNumber = ref(1)
const modelOptions = computed(() => (props.overview?.models ?? []).map((model) => ({ value: String(model.entry_id), label: model.model_id })))
const protocolOptions = computed(() => PROTOCOLS.map((protocol) => ({ value: protocol, label: protocolLabel(protocol) })))
const outcomeOptions = computed(() => OUTCOMES.map((key) => ({ key, label: t(`admin.scheduling.trace.outcomeFilters.${key || 'all'}`) })))

// ---------- 数据 ----------
const page = ref<SchedulingTracePage | null>(null)
const loading = ref(false)
const loadError = ref(false)
const selected = ref<SchedulingTrace | null>(null)
const pageCount = computed(() => Math.max(1, Math.ceil((page.value?.total ?? 0) / PAGE_SIZE)))
let controller: AbortController | null = null

async function reload() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  try {
    const next = await getSchedulingTraces(
      {
        window: props.timeWindow,
        entry_id: entryFilter.value ? Number(entryFilter.value) : undefined,
        protocol: protocolFilter.value as SchedulingProtocol | '',
        outcome: outcomeFilter.value,
        page: pageNumber.value,
        page_size: PAGE_SIZE
      },
      request.signal
    )
    if (controller !== request) return
    page.value = next
    loadError.value = false
  } catch (error) {
    if (controller !== request) return
    const e = error as { name?: string; code?: string }
    if (e?.name === 'CanceledError' || e?.code === 'ERR_CANCELED') return
    console.error('Failed to load scheduling traces:', error)
    loadError.value = true
  } finally {
    if (controller === request) loading.value = false
  }
}

watch([() => props.timeWindow, entryFilter, protocolFilter, outcomeFilter], () => {
  if (pageNumber.value !== 1) pageNumber.value = 1
  else void reload()
})
watch(pageNumber, () => void reload())
watch(
  () => props.refreshToken,
  () => {
    if (pageNumber.value === 1 && !selected.value && !loading.value) void reload()
  }
)
void reload()
onBeforeUnmount(() => controller?.abort())

// ---------- 渠道链 ----------
function formatMs(ms: number): string {
  return ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`
}

/** 每次选到的渠道一段：中途失败的带那次尝试的状态码，最后一段是请求结果；没选到渠道单独一段 */
function chainOf(trace: SchedulingTrace): Array<{ text: string; tone: SegmentTone }> {
  return trace.picks.map((pick, index) => {
    if (!pick.account_id) return { text: t('admin.scheduling.trace.noChannel'), tone: 'fail' as const }
    const name = channelName(pick.account_id)
    const isLast = trace.picks.slice(index + 1).every((later) => !later.account_id)
    if (isLast) {
      if (trace.outcome === 'success') return { text: `${name} ✓`, tone: 'ok' as const }
      if (trace.outcome === 'canceled') return { text: `${name} · ${labelOr('admin.scheduling.trace.outcomes', 'canceled')}`, tone: 'muted' as const }
      return { text: `${name} ${trace.status_code || '✗'}`, tone: 'fail' as const }
    }
    const nextAt = trace.picks[index + 1]?.at_ms ?? Number.MAX_SAFE_INTEGER
    const failed = trace.attempts.filter((a) => a.account_id === pick.account_id && a.at_ms >= pick.at_ms && a.at_ms < nextAt).at(-1)
    return { text: `${name} ${failed?.status_code || '✗'}`, tone: 'fail' as const }
  })
}
</script>
