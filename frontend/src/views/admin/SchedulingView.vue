<template>
  <!--
    调度（2026-10-18）：两个页签。
    实时：每个模型的渠道此刻按什么顺序被挑、哪些不会被选及原因（SchedulingRealtimePanel）。
    请求轨迹：每个请求每次选渠道考虑过哪些候选、为什么跳过、换渠道的过程（SchedulingTracesPanel）。
    页面在前台时每 10 秒刷新一次，切到后台暂停，回来立即刷新。
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

      <SegmentedControl v-model="tab" :options="tabOptions" :label="t('admin.scheduling.tabs.label')" test-id-prefix="scheduling-tab" />

      <SchedulingRealtimePanel v-if="tab === 'realtime'" :overview="data" :now="now" />
      <SchedulingTracesPanel v-else :overview="data" :time-window="timeWindow" :refresh-token="refreshToken" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDocumentVisibility, useIntervalFn } from '@vueuse/core'
import { getSchedulingOverview, type SchedulingOverview, type SchedulingWindow } from '@/api/admin/scheduling'
import AppLayout from '@/components/layout/AppLayout.vue'
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { StatItem } from '@/components/user/shell/types'
import SchedulingRealtimePanel from '@/components/admin/scheduling/SchedulingRealtimePanel.vue'
import SchedulingTracesPanel from '@/components/admin/scheduling/SchedulingTracesPanel.vue'
import { formatNumber } from '@/utils/format'

const { t } = useI18n()

const REFRESH_MS = 10_000
const WINDOWS: SchedulingWindow[] = ['15m', '1h', '24h']
type Tab = 'realtime' | 'traces'

const timeWindow = ref<SchedulingWindow>('1h')
const windowOptions = computed(() => WINDOWS.map((key) => ({ key, label: t(`admin.scheduling.window.${key}`) })))
const tab = ref<Tab>('realtime')
const tabOptions = computed(() => [
  { key: 'realtime' as const, label: t('admin.scheduling.tabs.realtime') },
  { key: 'traces' as const, label: t('admin.scheduling.tabs.traces') }
])

// ---------- 数据与刷新 ----------
const data = ref<SchedulingOverview | null>(null)
const loading = ref(false)
const loadError = ref(false)
const now = ref(new Date())
/** 每次定时刷新加一，请求轨迹页签据此跟着刷新 */
const refreshToken = ref(0)
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

function refresh() {
  refreshToken.value++
  if (!loading.value) void reload()
}

const visibility = useDocumentVisibility()
const visible = computed(() => visibility.value !== 'hidden')
const { pause, resume } = useIntervalFn(refresh, REFRESH_MS, { immediate: false })

watch(visible, (isVisible) => {
  if (isVisible) {
    refresh()
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

// ---------- 数字 ----------
const statItems = computed<StatItem[]>(() => {
  const overview = data.value
  const channels = overview?.channels ?? []
  const available = channels.filter((channel) => !channel.blocked_reason).length
  const loadKnown = channels.some((channel) => channel.in_use !== null)
  const sum = (pick: (channel: (typeof channels)[number]) => number | null) => channels.reduce((total, channel) => total + (pick(channel) ?? 0), 0)
  const models = overview?.models ?? []
  return [
    { key: 'models', label: t('admin.scheduling.stats.models'), value: formatNumber(models.length) },
    { key: 'channels', label: t('admin.scheduling.stats.channels'), value: `${available} / ${channels.length}` },
    { key: 'inUse', label: t('admin.scheduling.stats.inUse'), value: loadKnown ? formatNumber(sum((c) => c.in_use)) : '—' },
    { key: 'waiting', label: t('admin.scheduling.stats.waiting'), value: loadKnown ? formatNumber(sum((c) => c.waiting)) : '—' },
    { key: 'requests', label: t('admin.scheduling.stats.requests'), value: formatNumber(models.reduce((total, model) => total + model.requests, 0)) },
    { key: 'unrouted', label: t('admin.scheduling.stats.unrouted'), value: formatNumber(models.reduce((total, model) => total + model.unrouted, 0)) }
  ]
})
</script>
