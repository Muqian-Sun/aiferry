<template>
  <!--
    调度 · 实时：每个模型一行，按调度器此刻的挑选顺序排出它绑定的渠道。
    #1 就是下一个没有粘性绑定的请求会落的渠道；排队 = 准入通过但并发已满；灰的是不会被选的，写原因和多久恢复。
    同一模型在不同入站协议上顺序可能不同（能直连的排前），多协议时可切换；卡片上的请求数只算当前协议。
  -->
  <SheetSection :title="t('admin.scheduling.models.title')">
    <template #actions>
      <div class="w-full sm:w-56">
        <SearchInput v-model="search" compact :placeholder="t('admin.scheduling.models.search')" />
      </div>
    </template>
    <p class="mb-4 text-xs text-af-ink-3">{{ t('admin.scheduling.models.hint') }}</p>

    <p v-if="visibleModels.length === 0" class="py-10 text-center text-sm text-af-ink-3">
      {{ overview.models.length === 0 ? t('admin.scheduling.models.empty') : t('admin.scheduling.models.noMatch') }}
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
            :options="model.lanes.map((lane) => ({ key: lane.protocol, label: protocolLabel(lane.protocol) }))"
            :label="t('admin.scheduling.protocols.label')"
            :test-id-prefix="`scheduling-lane-${model.entry_id}`"
            @update:model-value="(protocol: string) => (selectedLane[model.entry_id] = protocol)"
          />
          <span v-else-if="model.lanes.length === 1" class="text-xs text-af-ink-3">
            {{ t('admin.scheduling.protocols.only', { protocol: protocolLabel(model.lanes[0].protocol) }) }}
          </span>
        </div>

        <p v-if="!laneOf(model)" class="mt-3 text-xs text-af-ink-3">{{ t('admin.scheduling.models.noLane') }}</p>
        <ol v-else class="mt-4 flex flex-wrap items-stretch gap-2" data-testid="scheduling-lane">
          <template v-for="(item, index) in laneOf(model)!.items" :key="item.account_id">
            <li v-if="index > 0 && item.state === 'ready'" class="flex items-center text-af-ink-4" aria-hidden="true">→</li>
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
                <PlatformIcon v-if="channelOf(item.account_id)" :platform="channelOf(item.account_id)!.platform as AccountPlatform" size="xs" />
                <span class="truncate" :title="channelName(item.account_id)">{{ channelName(item.account_id) }}</span>
              </RouterLink>
              <p v-if="item.state === 'blocked'" class="mt-1 text-af-ink-3" :title="item.detail || undefined">{{ blockedText(item) }}</p>
              <p v-else class="mt-1 tabular-nums text-af-ink-3">{{ loadText(item) }}</p>
              <p class="mt-1 tabular-nums text-af-ink-3">{{ channelTrafficText(model, laneOf(model)!.protocol, item.account_id) }}</p>
            </li>
          </template>
        </ol>
      </li>
    </ul>
  </SheetSection>
</template>

<script setup lang="ts">
import { computed, reactive, ref, toRef } from 'vue'
import { RouterLink } from 'vue-router'
import type { SchedulingItem, SchedulingLane, SchedulingModel, SchedulingOverview, SchedulingProtocol } from '@/api/admin/scheduling'
import type { AccountPlatform } from '@/types'
import SearchInput from '@/components/common/SearchInput.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import { durationUntilWords } from '@/components/account/durationWords'
import { formatNumber } from '@/utils/format'
import { useSchedulingLabels } from './useSchedulingLabels'

const props = defineProps<{ overview: SchedulingOverview; now: Date }>()
const overviewRef = toRef(props, 'overview')
const { t, channelOf, channelName, reasonLabel, protocolLabel } = useSchedulingLabels(overviewRef)

const search = ref('')
const visibleModels = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  if (!keyword) return props.overview.models
  return props.overview.models.filter((model) => model.model_id.toLowerCase().includes(keyword) || model.display_name.toLowerCase().includes(keyword))
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

/** 卡片上的请求数只算当前协议线；占比的分母是这条线上选到了渠道的请求 */
function channelTrafficText(model: SchedulingModel, protocol: SchedulingProtocol, accountID: number): string {
  const channel = model.channels.find((c) => c.account_id === accountID)
  const counts = channel?.by_protocol[protocol]
  const requests = counts?.requests ?? 0
  const lane = model.by_protocol[protocol]
  const routed = (lane?.requests ?? 0) - (lane?.unrouted ?? 0)
  const parts = [
    routed > 0
      ? t('admin.scheduling.item.share', { count: formatNumber(requests), percent: Math.round((requests / routed) * 100) })
      : t('admin.scheduling.item.requests', { count: formatNumber(requests) })
  ]
  if (counts && counts.failures > 0) parts.push(t('admin.scheduling.traffic.failures', { count: formatNumber(counts.failures) }))
  if (channel && channel.sticky_sessions > 0) parts.push(t('admin.scheduling.item.sticky', { count: channel.sticky_sessions }))
  return parts.join(' · ')
}

function loadText(item: SchedulingItem): string {
  const channel = channelOf(item.account_id)
  if (!channel || channel.in_use === null) return t('admin.scheduling.item.loadUnknown')
  const parts = [t('admin.scheduling.item.load', { inUse: channel.in_use, max: channel.concurrency })]
  if (channel.waiting) parts.push(t('admin.scheduling.item.waiting', { count: channel.waiting }))
  return parts.join(' · ')
}

function blockedText(item: SchedulingItem): string {
  const label = reasonLabel(item.reason)
  const remaining = durationUntilWords(item.until, t, props.now)
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
</script>
