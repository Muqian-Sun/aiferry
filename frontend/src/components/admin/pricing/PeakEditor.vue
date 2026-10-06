<template>
  <!--
    忙闲时编辑（官方价、售价、承接行共用）：时区、只在工作日、各时段（开始 / 结束 / 倍数）、节假日。
    peak（v-model:peak）null = 不分忙闲时；删掉最后一个时段也变成 null。officialPeak 给了就有「按官方忙闲时」一键带入。
  -->
  <div class="flex flex-col gap-2">
    <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-13">
      <button
        v-if="officialPeak"
        type="button"
        class="font-medium text-af-ink-2 transition-colors hover:text-af-ink"
        :data-testid="testId ? `${testId}-peak-official` : undefined"
        @click="peak = clonePeakForm(officialPeak)"
      >
        {{ t('admin.pricing.peak.useOfficial') }}
      </button>
      <button
        v-if="peak"
        type="button"
        class="font-medium text-af-ink-2 transition-colors hover:text-af-ink"
        :data-testid="testId ? `${testId}-peak-clear` : undefined"
        @click="peak = null"
      >
        {{ t('admin.pricing.peak.clear') }}
      </button>
      <template v-if="peak">
        <label class="flex items-center gap-1.5 text-af-ink-2">
          {{ t('admin.pricing.peak.timezone') }}
          <select v-model="peak.timezone" class="input h-8 w-auto px-2 py-1 text-13">
            <option v-for="zone in timezoneOptions" :key="zone" :value="zone">{{ timezoneLabel(zone) }}</option>
          </select>
        </label>
        <label class="flex items-center gap-1.5 text-af-ink-2">
          <input v-model="peak.weekdaysOnly" type="checkbox" class="h-4 w-4 rounded border-af-hairline" />
          {{ t('admin.pricing.peak.weekdaysOnly') }}
        </label>
      </template>
      <span v-else class="text-xs text-af-ink-3">{{ noneHint }}</span>
    </div>
    <div
      v-for="(period, index) in peak?.periods ?? []"
      :key="index"
      class="flex flex-wrap items-center gap-2 text-13 text-af-ink-2"
      :data-testid="testId ? `${testId}-peak-period` : undefined"
    >
      <input
        v-model="period.start"
        type="time"
        :aria-label="t('admin.pricing.peak.start')"
        :class="['input h-8 w-28 px-2 py-1 text-13 tabular-nums', issues[index] ? 'border-af-danger' : '']"
      />
      <span>–</span>
      <input
        v-model="period.end"
        type="time"
        :aria-label="t('admin.pricing.peak.end')"
        :class="['input h-8 w-28 px-2 py-1 text-13 tabular-nums', issues[index] ? 'border-af-danger' : '']"
      />
      <span>×</span>
      <input
        v-model="period.multiplier"
        type="text"
        inputmode="decimal"
        autocomplete="off"
        :aria-label="t('admin.pricing.peak.multiplier')"
        :class="['input h-8 w-16 px-2 py-1 text-right text-13 tabular-nums', issues[index] === 'multiplier' ? 'border-af-danger' : '']"
      />
      <button type="button" class="whitespace-nowrap text-af-ink-3 transition-colors hover:text-af-ink" @click="removePeriod(index)">
        {{ t('admin.pricing.remove') }}
      </button>
      <span v-if="issues[index]" class="text-xs text-af-danger">{{ t(`admin.pricing.peak.errors.${issues[index]}`) }}</span>
    </div>
    <label v-if="peak" class="flex flex-wrap items-center gap-2 text-13 text-af-ink-2">
      {{ t('admin.pricing.peak.excludeDates') }}
      <input
        v-model="peak.excludeDates"
        type="text"
        autocomplete="off"
        spellcheck="false"
        :placeholder="t('admin.pricing.peak.excludeDatesPlaceholder')"
        :aria-label="t('admin.pricing.peak.excludeDates')"
        :class="['input h-8 min-w-0 flex-1 px-2 py-1 font-mono text-13 sm:max-w-xl', datesInvalid ? 'border-af-danger' : '']"
        :data-testid="testId ? `${testId}-peak-exclude-dates` : undefined"
      />
      <span class="text-xs" :class="datesInvalid ? 'text-af-danger' : 'text-af-ink-3'">
        {{ datesInvalid ? t('admin.pricing.peak.excludeDatesInvalid') : t('admin.pricing.peak.excludeDatesCount', { count: excludeDateCount }) }}
      </span>
    </label>
    <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
      <button
        type="button"
        class="text-13 font-medium text-af-ink-2 transition-colors hover:text-af-ink"
        :data-testid="testId ? `${testId}-peak-add` : undefined"
        @click="addPeriod"
      >
        {{ t('admin.pricing.peak.add') }}
      </button>
      <span class="text-xs text-af-ink-3">{{ t('admin.pricing.peak.endHint') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { clonePeakForm, parseExcludeDates, type PeakForm, type PeakPeriodError } from './pricingDraft'

const peak = defineModel<PeakForm | null>('peak', { default: null })

withDefaults(
  defineProps<{
    /** 各时段的问题，与 periods 一一对应 */
    issues: Array<PeakPeriodError | null>
    datesInvalid: boolean
    /** 不分忙闲时的说明 */
    noneHint: string
    /** 一键「按官方忙闲时」：这个模型（可能还没保存的）官方忙闲时 */
    officialPeak?: PeakForm | null
    testId?: string
  }>(),
  { officialPeak: null, testId: undefined }
)

const { t } = useI18n()
const excludeDateCount = computed(() => (peak.value ? parseExcludeDates(peak.value.excludeDates).length : 0))

/** 常用时区；已存的不在其中时也列上 */
const COMMON_TIMEZONES = ['Asia/Shanghai', 'UTC', 'America/Los_Angeles']
const timezoneOptions = computed(() => {
  const current = peak.value?.timezone
  return current && !COMMON_TIMEZONES.includes(current) ? [...COMMON_TIMEZONES, current] : COMMON_TIMEZONES
})
function timezoneLabel(zone: string): string {
  const key = { 'Asia/Shanghai': 'beijing', UTC: 'utc', 'America/Los_Angeles': 'pacific' }[zone]
  return key ? t(`admin.pricing.peak.zones.${key}`) : zone
}

function addPeriod() {
  const period = { start: '', end: '', multiplier: '2' }
  if (peak.value) peak.value.periods.push(period)
  else peak.value = { timezone: 'Asia/Shanghai', weekdaysOnly: true, periods: [period], excludeDates: '' }
}

/** 删掉最后一个时段 = 不分忙闲时 */
function removePeriod(index: number) {
  if (!peak.value) return
  peak.value.periods.splice(index, 1)
  if (peak.value.periods.length === 0) peak.value = null
}
</script>
