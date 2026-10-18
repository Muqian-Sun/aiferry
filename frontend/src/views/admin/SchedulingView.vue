<template>
  <!--
    调度（2026-10-18）：每个模型一行，按调度器此刻的挑选顺序排出它绑定的渠道。
    #1 就是下一个没有粘性绑定的请求会落的渠道；排队 = 准入通过但并发已满；灰的是不会被选的，写原因和多久恢复。
    同一模型在不同入站协议上顺序可能不同（能直连的排前），多协议时可切换。页面在前台时每 10 秒刷新一次。
  -->
  <AppLayout>
    <template #header-actions>
      <span class="hidden text-xs text-af-ink-3 sm:inline" data-testid="scheduling-refresh-state">{{ refreshStateText }}</span>
      <SegmentedControl v-model="timeWindow" :options="windowOptions" :label="t('admin.scheduling.window.label')" test-id-prefix="scheduling-window" />
    </template>

    <StatusState
      v-if="loadError && !data"
      kind="error"
      :title="t('admin.scheduling.loadFailed')"
      :action-label="t('userUi.usage.retry')"
      @action="reload"
    />
    <StatusState v-else-if="!data" kind="loading" :title="t('common.loading')" />

    <div v-else class="space-y-10 transition-opacity" :class="loading ? 'opacity-60' : ''" :aria-busy="loading ? 'true' : undefined">
      <section class="space-y-3" data-testid="scheduling-summary">
        <StatRow :items="statItems" />
        <p v-for="warning in data.warnings" :key="warning" class="text-xs text-af-warning" data-testid="scheduling-warning">
          {{ t(`admin.scheduling.warnings.${warning}`) }}
        </p>
        <p v-if="loadError" class="text-xs text-af-danger">{{ t('admin.scheduling.refreshFailed') }}</p>
      </section>

      <SheetSection :title="t('admin.scheduling.models.title')">
        <template #actions>
          <div class="w-full sm:w-56">
            <SearchInput v-model="search" compact :placeholder="t('admin.scheduling.models.search')" />
          </div>
        </template>
        <p class="mb-4 text-xs text-af-ink-3">{{ t('admin.scheduling.models.hint') }}</p>

        <p v-if="visibleModels.length === 0" class="py-10 text-center text-sm text-af-ink-3">
          {{ data.models.length === 0 ? t('admin.scheduling.models.empty') : t('admin.scheduling.models.noMatch') }}
        </p>
        <ul v-else class="-mx-4 border-t border-af-hairline md:-mx-6" data-testid="scheduling-models">
          <li
            v-for="model in visibleModels"
            :key="model.entry_id"
            class="border-b border-af-hairline px-4 py-5 md:px-6"
            data-testid="scheduling-model"
          >
            <div class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2">
              <div class="min-w-0">
                <span class="text-sm font-medium text-af-ink">{{ model.display_name || model.model_id }}</span>
                <span v-if="model.display_name && model.display_name !== model.model_id" class="ml-2 text-xs text-af-ink-3">{{ model.model_id }}</span>
                <p class="mt-1 text-xs tabular-nums text-af-ink-3">{{ modelTrafficText(model) }}</p>
              </div>
              <SegmentedControl
                v-if="model.lanes.length > 1"
                :model-value="laneOf(model)?.protocol ?? ''"
                :options="model.lanes.map((lane) => ({ key: lane.protocol, label: t(`admin.scheduling.protocols.${lane.protocol}`) }))"
                :label="t('admin.scheduling.protocols.label')"
                :test-id-prefix="`scheduling-lane-${model.entry_id}`"
                @update:model-value="(protocol: string) => (selectedLane[model.entry_id] = protocol)"
              />
              <span v-else-if="model.lanes.length === 1" class="text-xs text-af-ink-3">
                {{ t('admin.scheduling.protocols.only', { protocol: t(`admin.scheduling.protocols.${model.lanes[0].protocol}`) }) }}
              </span>
            </div>

            <p v-if="!laneOf(model)" class="mt-3 text-xs text-af-ink-3">{{ t('admin.scheduling.models.noLane') }}</p>
            <ol v-else class="mt-4 flex flex-wrap items-stretch gap-2" data-testid="scheduling-lane">
              <template v-for="(item, index) in laneOf(model)!.items" :key="item.account_id">
                <li
                  v-if="index > 0 && item.state === 'ready'"
                  class="flex items-center text-af-ink-4"
                  aria-hidden="true"
                >→</li>
                <li
                  v-else-if="index > 0 && item.state !== laneOf(model)!.items[index - 1].state"
                  class="mx-1 w-px self-stretch bg-af-hairline"
                  aria-hidden="true"
                />
                <li
                  class="w-56 rounded-lg border px-3 py-2.5 text-xs"
                  :class="chipClass(item)"
                  :data-testid="`scheduling-item-${item.state}`"
                  :data-account-id="item.account_id"
                >
                  <div class="flex items-center gap-1.5">
                    <span class="shrink-0 font-medium tabular-nums" :class="badgeClass(item)">{{ badgeText(item) }}</span>
                    <span v-if="item.direct" class="shrink-0 text-af-ink-3">· {{ t('admin.scheduling.item.direct') }}</span>
                  </div>
                  <RouterLink
                    :to="`/accounts?edit=${item.account_id}`"
                    class="mt-1 flex min-w-0 items-center gap-1 text-13 font-medium hover:text-af-brand"
                    :class="item.state === 'blocked' ? 'text-af-ink-3' : 'text-af-ink'"
                  >
                    <PlatformIcon v-if="channelOf(item)" :platform="channelOf(item)!.platform as AccountPlatform" size="xs" />
                    <span class="truncate" :title="channelName(item.account_id)">{{ channelName(item.account_id) }}</span>
                  </RouterLink>
                  <p v-if="item.state === 'blocked'" class="mt-1 text-af-ink-3" :title="item.detail || undefined">
                    {{ blockedText(item) }}
                  </p>
                  <p v-else class="mt-1 tabular-nums text-af-ink-3">{{ loadText(item) }}</p>
                  <p class="mt-1 tabular-nums text-af-ink-3">{{ channelTrafficText(model, item.account_id) }}</p>
                </li>
              </template>
            </ol>
          </li>
        </ul>
      </SheetSection>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useDocumentVisibility, useIntervalFn } from '@vueuse/core'
import {
  getSchedulingOverview,
  type SchedulingChannel,
  type SchedulingItem,
  type SchedulingLane,
  type SchedulingModel,
  type SchedulingOverview,
  type SchedulingWindow
} from '@/api/admin/scheduling'
import type { AccountPlatform } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { StatItem } from '@/components/user/shell/types'
import { durationUntilWords } from '@/components/account/durationWords'
import { formatNumber } from '@/utils/format'

const { t, te } = useI18n()

const REFRESH_MS = 10_000
const WINDOWS: SchedulingWindow[] = ['15m', '1h', '24h']

const timeWindow = ref<SchedulingWindow>('1h')
const windowOptions = computed(() => WINDOWS.map((key) => ({ key, label: t(`admin.scheduling.window.${key}`) })))

// ---------- 数据与刷新 ----------
const data = ref<SchedulingOverview | null>(null)
const loading = ref(false)
const loadError = ref(false)
const now = ref(new Date())
let controller: AbortController | null = null

async function reload() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  try {
    const next = await getSchedulingOverview(timeWindow.value, request.signal)
    if (controller !== request) return
    data.value = next
    now.value = new Date()
    loadError.value = false
  } catch (error) {
    if (controller !== request) return
    const e = error as { name?: string; code?: string }
    if (e?.name === 'CanceledError' || e?.code === 'ERR_CANCELED') return
    console.error('Failed to load scheduling overview:', error)
    loadError.value = true
  } finally {
    if (controller === request) loading.value = false
  }
}

const visibility = useDocumentVisibility()
const visible = computed(() => visibility.value !== 'hidden')
const { pause, resume } = useIntervalFn(() => {
  if (!loading.value) void reload()
}, REFRESH_MS, { immediate: false })

watch(visible, (isVisible) => {
  if (isVisible) {
    void reload()
    resume()
  } else {
    pause()
  }
})
watch(timeWindow, () => void reload())
onMounted(() => {
  void reload()
  if (visible.value) resume()
})
onBeforeUnmount(() => {
  pause()
  controller?.abort()
})

const refreshStateText = computed(() => {
  if (!visible.value) return t('admin.scheduling.refresh.paused')
  if (!data.value) return ''
  const time = new Date(data.value.generated_at).toLocaleTimeString()
  return t('admin.scheduling.refresh.auto', { seconds: REFRESH_MS / 1000, time })
})

// ---------- 渠道 ----------
const channelsById = computed(() => new Map<number, SchedulingChannel>((data.value?.channels ?? []).map((channel) => [channel.id, channel])))
function channelOf(item: SchedulingItem): SchedulingChannel | undefined {
  return channelsById.value.get(item.account_id)
}
function channelName(accountID: number): string {
  return channelsById.value.get(accountID)?.name || `#${accountID}`
}

// ---------- 模型与调度线 ----------
const search = ref('')
const visibleModels = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  const models = data.value?.models ?? []
  if (!keyword) return models
  return models.filter((model) => model.model_id.toLowerCase().includes(keyword) || model.display_name.toLowerCase().includes(keyword))
})

const selectedLane = reactive<Record<number, string>>({})
function laneOf(model: SchedulingModel): SchedulingLane | undefined {
  return model.lanes.find((lane) => lane.protocol === selectedLane[model.entry_id]) ?? model.lanes[0]
}

function modelTrafficText(model: SchedulingModel): string {
  const parts = [t('admin.scheduling.traffic.requests', { count: formatNumber(model.requests) })]
  if (model.failures > 0) parts.push(t('admin.scheduling.traffic.failures', { count: formatNumber(model.failures) }))
  if (model.unrouted > 0) parts.push(t('admin.scheduling.traffic.unrouted', { count: formatNumber(model.unrouted) }))
  return parts.join(' · ')
}

function channelTrafficText(model: SchedulingModel, accountID: number): string {
  const channel = model.channels.find((c) => c.account_id === accountID)
  const requests = channel?.requests ?? 0
  const routed = model.requests - model.unrouted
  const parts = [
    routed > 0
      ? t('admin.scheduling.item.share', { count: formatNumber(requests), percent: Math.round((requests / routed) * 100) })
      : t('admin.scheduling.item.requests', { count: formatNumber(requests) })
  ]
  if (channel && channel.failures > 0) parts.push(t('admin.scheduling.traffic.failures', { count: formatNumber(channel.failures) }))
  if (channel && channel.sticky_sessions > 0) parts.push(t('admin.scheduling.item.sticky', { count: channel.sticky_sessions }))
  return parts.join(' · ')
}

function loadText(item: SchedulingItem): string {
  const channel = channelOf(item)
  if (!channel || channel.in_use === null) return t('admin.scheduling.item.loadUnknown')
  const parts = [t('admin.scheduling.item.load', { inUse: channel.in_use, max: channel.concurrency })]
  if (channel.waiting) parts.push(t('admin.scheduling.item.waiting', { count: channel.waiting }))
  return parts.join(' · ')
}

function reasonLabel(reason: string | undefined): string {
  const key = `admin.scheduling.reasons.${reason}`
  return reason && te(key) ? t(key) : t('admin.scheduling.reasons.other', { reason: reason || '-' })
}

function blockedText(item: SchedulingItem): string {
  const label = reasonLabel(item.reason)
  const remaining = durationUntilWords(item.until, t, now.value)
  return remaining ? t('admin.scheduling.item.recoversIn', { reason: label, duration: remaining }) : label
}

function badgeText(item: SchedulingItem): string {
  if (item.state === 'ready') return `#${item.rank}`
  if (item.state === 'queue') return t('admin.scheduling.item.queue')
  return t('admin.scheduling.item.blocked')
}

function chipClass(item: SchedulingItem): string {
  if (item.state === 'ready') return item.rank === 1 ? 'border-af-brand bg-af-sheet' : 'border-af-hairline bg-af-sheet'
  if (item.state === 'queue') return 'border-af-warning/40 bg-af-sheet'
  return 'border-dashed border-af-hairline bg-af-sunken/60'
}

function badgeClass(item: SchedulingItem): string {
  if (item.state === 'ready') return item.rank === 1 ? 'text-af-brand' : 'text-af-ink-2'
  if (item.state === 'queue') return 'text-af-warning'
  return 'text-af-ink-3'
}

// ---------- 数字 ----------
const statItems = computed<StatItem[]>(() => {
  const overview = data.value
  const channels = overview?.channels ?? []
  const available = channels.filter((channel) => !channel.blocked_reason).length
  const loadKnown = channels.some((channel) => channel.in_use !== null)
  const sum = (pick: (channel: SchedulingChannel) => number | null) => channels.reduce((total, channel) => total + (pick(channel) ?? 0), 0)
  const models = overview?.models ?? []
  return [
    { key: 'models', label: t('admin.scheduling.stats.models'), value: formatNumber(models.length) },
    { key: 'channels', label: t('admin.scheduling.stats.channels'), value: `${available} / ${channels.length}` },
    { key: 'inUse', label: t('admin.scheduling.stats.inUse'), value: loadKnown ? formatNumber(sum((c) => c.in_use)) : '—' },
    { key: 'waiting', label: t('admin.scheduling.stats.waiting'), value: loadKnown ? formatNumber(sum((c) => c.waiting)) : '—' },
    {
      key: 'requests',
      label: t('admin.scheduling.stats.requests'),
      value: formatNumber(models.reduce((total, model) => total + model.requests, 0)),
      hint: overview?.data_through ? t('admin.scheduling.stats.dataThrough', { time: new Date(overview.data_through).toLocaleTimeString() }) : undefined
    },
    { key: 'unrouted', label: t('admin.scheduling.stats.unrouted'), value: formatNumber(models.reduce((total, model) => total + model.unrouted, 0)) }
  ]
})
</script>
