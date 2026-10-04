<script setup lang="ts">
/**
 * 运维页页头：标题、刷新时间，以及筛选（时间范围 / 模型 / 渠道）与入口（刷新、告警规则、设置、全屏）。
 * 2026-10-04 运维页重排（方案页 8ARyR9…）：平台筛选换成按模型、按渠道。
 */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'

export interface OpsChannelOption {
  id: number
  name: string
}

const props = defineProps<{
  timeRange: string
  customStartTime?: string | null
  customEndTime?: string | null
  model: string
  accountId: number | null
  modelOptions: string[]
  channelOptions: OpsChannelOption[]
  loading: boolean
  lastUpdated: Date | null
  autoRefreshEnabled: boolean
  autoRefreshCountdown: number
  fullscreen: boolean
}>()

const emit = defineEmits<{
  (e: 'update:timeRange', value: string): void
  (e: 'update:customTimeRange', startTime: string, endTime: string): void
  (e: 'update:model', value: string): void
  (e: 'update:accountId', value: number | null): void
  (e: 'refresh'): void
  (e: 'openAlertRules'): void
  (e: 'openSettings'): void
  (e: 'enterFullscreen'): void
  (e: 'exitFullscreen'): void
}>()

const { t } = useI18n()

const showCustomDialog = ref(false)
const customStartInput = ref('')
const customEndInput = ref('')

function formatCustomLabel(startTime: string, endTime: string): string {
  const fmt = (d: Date) =>
    `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  return `${fmt(new Date(startTime))} ~ ${fmt(new Date(endTime))}`
}

const timeRangeOptions = computed(() => [
  { value: '5m', label: t('admin.ops.timeRange.5m') },
  { value: '30m', label: t('admin.ops.timeRange.30m') },
  { value: '1h', label: t('admin.ops.timeRange.1h') },
  { value: '6h', label: t('admin.ops.timeRange.6h') },
  { value: '24h', label: t('admin.ops.timeRange.24h') },
  {
    value: 'custom',
    label:
      props.timeRange === 'custom' && props.customStartTime && props.customEndTime
        ? `${t('admin.ops.timeRange.custom')} (${formatCustomLabel(props.customStartTime, props.customEndTime)})`
        : t('admin.ops.timeRange.custom')
  }
])

const modelSelectOptions = computed(() => [
  { value: '', label: t('admin.ops.page.allModels') },
  ...props.modelOptions.map((model) => ({ value: model, label: model }))
])

const channelSelectOptions = computed(() => [
  { value: 0, label: t('admin.ops.page.allChannels') },
  ...props.channelOptions.map((channel) => ({ value: channel.id, label: channel.name }))
])

function onTimeRangeChange(value: string | number | boolean | null) {
  const next = String(value || '1h')
  if (next !== 'custom') {
    emit('update:timeRange', next)
    return
  }
  const now = new Date()
  const toLocalInput = (d: Date) => new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
  customStartInput.value = toLocalInput(new Date(now.getTime() - 60 * 60 * 1000))
  customEndInput.value = toLocalInput(now)
  showCustomDialog.value = true
}

function confirmCustomRange() {
  if (!customStartInput.value || !customEndInput.value) return
  // 先给自定义时间、再切到 custom：父组件响应 timeRange 变化时就能拼出正确的参数
  emit('update:customTimeRange', new Date(customStartInput.value).toISOString(), new Date(customEndInput.value).toISOString())
  emit('update:timeRange', 'custom')
  showCustomDialog.value = false
}

const lastUpdatedLabel = computed(() => (props.lastUpdated ? props.lastUpdated.toLocaleTimeString() : t('common.unknown')))
</script>

<template>
  <div class="flex flex-wrap items-end justify-between gap-x-6 gap-y-3 pb-4" data-testid="ops-toolbar">
    <div class="min-w-0">
      <h1 class="text-xl font-semibold text-af-ink">{{ t('nav.ops') }}</h1>
      <p class="mt-1 text-xs text-af-ink-3">
        {{ props.loading ? t('admin.ops.loadingText') : t('admin.ops.page.updatedAt', { time: lastUpdatedLabel }) }}
        <template v-if="props.autoRefreshEnabled">· {{ t('admin.ops.autoRefreshRemaining', { seconds: props.autoRefreshCountdown }) }}</template>
      </p>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <template v-if="!props.fullscreen">
        <Select
          :model-value="props.model"
          :options="modelSelectOptions"
          searchable
          class="w-full sm:w-[180px]"
          data-testid="ops-filter-model"
          @update:model-value="(v) => emit('update:model', String(v || ''))"
        />
        <Select
          :model-value="props.accountId ?? 0"
          :options="channelSelectOptions"
          searchable
          class="w-full sm:w-[180px]"
          data-testid="ops-filter-channel"
          @update:model-value="(v) => emit('update:accountId', Number(v) > 0 ? Number(v) : null)"
        />
      </template>
      <Select
        :model-value="props.timeRange"
        :options="timeRangeOptions"
        class="w-full sm:w-[150px]"
        data-testid="ops-filter-time"
        @update:model-value="onTimeRangeChange"
      />
      <button type="button" class="btn btn-secondary btn-sm" :disabled="props.loading" @click="emit('refresh')">
        {{ t('common.refresh') }}
      </button>
      <template v-if="!props.fullscreen">
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('openAlertRules')">{{ t('admin.ops.alertRules.manage') }}</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('openSettings')">{{ t('common.settings') }}</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('enterFullscreen')">{{ t('admin.ops.fullscreen.enter') }}</button>
      </template>
      <button v-else type="button" class="btn btn-secondary btn-sm" @click="emit('exitFullscreen')">{{ t('admin.ops.fullscreen.exit') }}</button>
    </div>

    <BaseDialog :show="showCustomDialog" :title="t('admin.ops.timeRange.custom')" width="narrow" @close="showCustomDialog = false">
      <div class="space-y-4">
        <div>
          <label class="input-label" for="ops-custom-start">{{ t('admin.ops.customTimeRange.startTime') }}</label>
          <input id="ops-custom-start" v-model="customStartInput" type="datetime-local" class="input" />
        </div>
        <div>
          <label class="input-label" for="ops-custom-end">{{ t('admin.ops.customTimeRange.endTime') }}</label>
          <input id="ops-custom-end" v-model="customEndInput" type="datetime-local" class="input" />
        </div>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="showCustomDialog = false">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" @click="confirmCustomRange">{{ t('common.confirm') }}</button>
        </div>
      </div>
    </BaseDialog>
  </div>
</template>
