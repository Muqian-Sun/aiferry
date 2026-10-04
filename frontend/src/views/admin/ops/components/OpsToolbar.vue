<script setup lang="ts">
/**
 * 运维页页头：标题、刷新时间，以及筛选（时间范围 / 模型 / 渠道）与入口（刷新、告警规则、设置、全屏）。
 * 2026-10-04 运维页重排（方案页 8ARyR9…）：平台筛选换成按模型、按渠道。
 * 2026-10-04 muqian「这些框很丑，像卡片似的」：与各列表页同一套工具行——模型 / 渠道用筛选标签，
 * 时间范围用分段切换，刷新是图标按钮，其余入口是无框文字按钮。
 */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import SegmentedControl from '@/components/common/SegmentedControl.vue'
import Icon from '@/components/icons/Icon.vue'
import { FilterChip } from '@/components/admin/list'

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
const customRangeError = ref('')

function formatCustomLabel(startTime: string, endTime: string): string {
  const fmt = (d: Date) =>
    `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  return `${fmt(new Date(startTime))} ~ ${fmt(new Date(endTime))}`
}

const timeRangeOptions = computed(() => [
  { key: '5m', label: t('admin.ops.timeRange.5m') },
  { key: '30m', label: t('admin.ops.timeRange.30m') },
  { key: '1h', label: t('admin.ops.timeRange.1h') },
  { key: '6h', label: t('admin.ops.timeRange.6h') },
  { key: '24h', label: t('admin.ops.timeRange.24h') },
  {
    key: 'custom',
    label:
      props.timeRange === 'custom' && props.customStartTime && props.customEndTime
        ? `${t('admin.ops.timeRange.custom')} (${formatCustomLabel(props.customStartTime, props.customEndTime)})`
        : t('admin.ops.timeRange.custom')
  }
])

// 筛选标签不选就是全部（自带清除），不放「全部」项
const modelChipOptions = computed(() => props.modelOptions.map((model) => ({ value: model, label: model })))
const channelChipOptions = computed(() => props.channelOptions.map((channel) => ({ value: channel.id, label: channel.name })))

function onTimeRangeChange(value: string) {
  const next = String(value || '1h')
  if (next !== 'custom') {
    emit('update:timeRange', next)
    return
  }
  const now = new Date()
  const toLocalInput = (d: Date) => new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
  // 已经是自定义时沿用当前起止（原来每次都重置成近 1 小时）
  const hasCustom = props.customStartTime && props.customEndTime
  customStartInput.value = toLocalInput(hasCustom ? new Date(props.customStartTime as string) : new Date(now.getTime() - 60 * 60 * 1000))
  customEndInput.value = toLocalInput(hasCustom ? new Date(props.customEndTime as string) : now)
  customRangeError.value = ''
  showCustomDialog.value = true
}

function confirmCustomRange() {
  if (!customStartInput.value || !customEndInput.value) return
  // 开始不能晚于结束（原来照样发请求，页面只报「加载运维数据失败」、旧数据还留着）
  if (new Date(customStartInput.value).getTime() >= new Date(customEndInput.value).getTime()) {
    customRangeError.value = t('admin.ops.page.customRangeInvalid')
    return
  }
  customRangeError.value = ''
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
        <FilterChip
          :model-value="props.model"
          :label="t('admin.ops.page.modelFilter')"
          :options="modelChipOptions"
          test-id="ops-filter-model"
          @update:model-value="(v) => emit('update:model', String(v || ''))"
        />
        <FilterChip
          :model-value="props.accountId ?? ''"
          :label="t('admin.ops.page.channelFilter')"
          :options="channelChipOptions"
          test-id="ops-filter-channel"
          @update:model-value="(v) => emit('update:accountId', Number(v) > 0 ? Number(v) : null)"
        />
      </template>
      <SegmentedControl
        :model-value="props.timeRange"
        :options="timeRangeOptions"
        :label="t('admin.ops.page.timeRangeLabel')"
        test-id-prefix="ops-filter-time"
        @update:model-value="onTimeRangeChange"
      />
      <button
        type="button"
        class="btn btn-ghost btn-md px-2.5"
        :disabled="props.loading"
        :title="t('common.refresh')"
        :aria-label="t('common.refresh')"
        @click="emit('refresh')"
      >
        <Icon name="refresh" size="md" :class="props.loading ? 'animate-spin' : ''" />
      </button>
      <template v-if="!props.fullscreen">
        <button type="button" class="btn btn-ghost btn-sm" @click="emit('openAlertRules')">
          <Icon name="bell" size="sm" />{{ t('admin.ops.alertRules.manage') }}
        </button>
        <button type="button" class="btn btn-ghost btn-sm" @click="emit('openSettings')">
          <Icon name="cog" size="sm" />{{ t('common.settings') }}
        </button>
        <button type="button" class="btn btn-ghost btn-sm" @click="emit('enterFullscreen')">{{ t('admin.ops.fullscreen.enter') }}</button>
      </template>
      <button v-else type="button" class="btn btn-ghost btn-sm" @click="emit('exitFullscreen')">{{ t('admin.ops.fullscreen.exit') }}</button>
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
        <p v-if="customRangeError" class="text-sm text-af-danger" role="alert">{{ customRangeError }}</p>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="showCustomDialog = false">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" @click="confirmCustomRange">{{ t('common.confirm') }}</button>
        </div>
      </div>
    </BaseDialog>
  </div>
</template>
