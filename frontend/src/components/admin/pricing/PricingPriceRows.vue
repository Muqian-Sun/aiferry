<template>
  <!--
    价格页表格里的一行价（官方价或一条承接关系的上游价）：五项价 + 「分段」开关，展开后每段一行、价格列对齐；
    有官方搜索工具的模型另有「联网搜索」开关，展开后填搜索价（存 $/次、$/条，按每千次 / 千条 / 千个显示）。
    peak 给了就有「忙闲时」开关（v-model:peak，null = 不分忙闲时）：官方价那一行是目录条目的分时（向用户收钱整单乘倍数），
    承接行是上游忙闲时（渠道成本整单乘倍数）；承接行可一键「按官方忙闲时」。
    prices（v-model:prices）是父组件草稿里的对象，这里原地改它（与模型编辑页的分段行同一套 TokenSegmentForm）。
    首列、上游模型名、毛利、状态、操作由父组件经插槽给出；refs 给了就在每格下面标官方价作参考（按渠道视图用）。
  -->
  <tr :class="rowClass" :data-testid="testId">
    <td class="py-2 pl-0 pr-3 align-middle"><slot name="lead" /></td>
    <td class="px-2 py-2 align-middle"><slot name="upstream" /></td>
    <td v-for="key in PRICE_KEYS" :key="key" class="px-2 py-2 text-right align-middle">
      <div class="inline-flex flex-col items-end">
        <PriceInput
          v-model="prices[key]"
          compact
          :scale="PER_MILLION"
          :unit="unit"
          :label="t(`admin.pricing.columns.${key}`)"
          :required="issues.missing.includes(key)"
          :placeholder="issues.missing.includes(key) ? t('admin.pricing.required') : ''"
          :test-id="testId ? `${testId}-${key}` : undefined"
        />
        <span v-if="refs" class="mt-0.5 text-xs tabular-nums text-af-ink-3">
          {{ refs[key] == null ? t('admin.pricing.officialUnset') : t('admin.pricing.officialRef', { price: formatPerMillion(refs[key]) }) }}
        </span>
      </div>
    </td>
    <td class="px-2 py-2 align-middle">
      <div class="flex flex-col items-start">
        <button
          type="button"
          :class="['whitespace-nowrap rounded-md px-2 py-1 text-13 transition-colors hover:bg-af-sunken', segmentsInvalid ? 'text-af-danger' : 'text-af-ink-2 hover:text-af-ink']"
          :aria-expanded="expanded"
          :data-testid="testId ? `${testId}-segments-toggle` : undefined"
          @click="expanded = !expanded"
        >
          <!-- 段数含第 1 段（基础价），与模型页「分 N 段」同一口径 -->
          {{ prices.segments.length ? t('admin.pricing.segmentsCount', { count: prices.segments.length + 1 }) : t('admin.pricing.segmentsNone') }}
          <Icon :name="expanded ? 'chevronDown' : 'chevronRight'" size="xs" class="ml-0.5 inline text-af-ink-3" />
        </button>
        <button
          v-if="searchKeys.length > 0"
          type="button"
          :class="['whitespace-nowrap rounded-md px-2 py-1 text-13 transition-colors hover:bg-af-sunken', searchInvalid ? 'text-af-danger' : 'text-af-ink-2 hover:text-af-ink']"
          :aria-expanded="searchExpanded"
          :data-testid="testId ? `${testId}-search-toggle` : undefined"
          @click="searchExpanded = !searchExpanded"
        >
          {{ t('admin.pricing.search.toggle') }}
          <Icon :name="searchExpanded ? 'chevronDown' : 'chevronRight'" size="xs" class="ml-0.5 inline text-af-ink-3" />
        </button>
        <button
          v-if="peakEditable"
          type="button"
          :class="['whitespace-nowrap rounded-md px-2 py-1 text-13 transition-colors hover:bg-af-sunken', peakInvalid ? 'text-af-danger' : 'text-af-ink-2 hover:text-af-ink']"
          :aria-expanded="peakExpanded"
          :data-testid="testId ? `${testId}-peak-toggle` : undefined"
          @click="peakExpanded = !peakExpanded"
        >
          {{ peakLabel }}
          <Icon :name="peakExpanded ? 'chevronDown' : 'chevronRight'" size="xs" class="ml-0.5 inline text-af-ink-3" />
        </button>
      </div>
    </td>
    <td class="px-3 py-2 text-right align-middle"><slot name="margin" /></td>
    <td class="px-3 py-2 align-middle"><slot name="status" /></td>
    <td class="py-2 pl-3 pr-0 text-right align-middle"><slot name="actions" /></td>
  </tr>
  <tr v-if="searchExpanded && searchKeys.length > 0" :data-testid="testId ? `${testId}-search` : undefined">
    <td colspan="2" class="py-2 pl-0 pr-3 align-top">
      <div class="pl-4 text-13 font-medium text-af-ink-2">{{ t('admin.pricing.search.title') }}</div>
      <div v-if="searchNote" class="max-w-[16rem] pl-4 text-xs text-af-ink-3">{{ searchNote }}</div>
    </td>
    <td :colspan="COLUMN_COUNT - 2" class="px-2 py-2 align-top">
      <div class="flex flex-wrap items-start gap-x-6 gap-y-2">
        <div v-for="key in searchKeys" :key="key" class="w-52">
          <label class="mb-1 block text-xs text-af-ink-3">{{ t(`admin.pricing.columns.${key}`) }}</label>
          <PriceInput
            v-model="prices[key]"
            :scale="PER_THOUSAND"
            :unit="t(`admin.pricing.search.units.${key}`)"
            :label="t(`admin.pricing.columns.${key}`)"
            :required="issues.missing.includes(key)"
            :placeholder="issues.missing.includes(key) ? t('admin.pricing.required') : (searchPlaceholders?.[key] ?? '')"
            :test-id="testId ? `${testId}-${key}` : undefined"
          />
          <p v-if="searchHints?.[key]" class="mt-0.5 text-xs tabular-nums text-af-ink-3">{{ searchHints[key] }}</p>
        </div>
      </div>
    </td>
  </tr>
  <tr v-if="peakEditable && peakExpanded" :data-testid="testId ? `${testId}-peak` : undefined">
    <td colspan="2" class="py-2 pl-0 pr-3 align-top">
      <div class="pl-4 text-13 font-medium text-af-ink-2">{{ t(`admin.pricing.peak.${peakEditable}.title`) }}</div>
      <div class="max-w-[16rem] pl-4 text-xs text-af-ink-3">{{ t(`admin.pricing.peak.${peakEditable}.note`) }}</div>
    </td>
    <td :colspan="COLUMN_COUNT - 2" class="px-2 py-2 align-top">
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
          <span v-else class="text-xs text-af-ink-3">{{ t(`admin.pricing.peak.${peakEditable}.noneHint`) }}</span>
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
            :class="['input h-8 w-28 px-2 py-1 text-13 tabular-nums', peakIssues[index] ? 'border-af-danger' : '']"
          />
          <span>–</span>
          <input
            v-model="period.end"
            type="time"
            :aria-label="t('admin.pricing.peak.end')"
            :class="['input h-8 w-28 px-2 py-1 text-13 tabular-nums', peakIssues[index] ? 'border-af-danger' : '']"
          />
          <span>×</span>
          <input
            v-model="period.multiplier"
            type="text"
            inputmode="decimal"
            autocomplete="off"
            :aria-label="t('admin.pricing.peak.multiplier')"
            :class="['input h-8 w-16 px-2 py-1 text-right text-13 tabular-nums', peakIssues[index] === 'multiplier' ? 'border-af-danger' : '']"
          />
          <button type="button" class="whitespace-nowrap text-af-ink-3 transition-colors hover:text-af-ink" @click="removePeakPeriod(index)">
            {{ t('admin.pricing.remove') }}
          </button>
          <span v-if="peakIssues[index]" class="text-xs text-af-danger">{{ t(`admin.pricing.peak.errors.${peakIssues[index]}`) }}</span>
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
            :class="['input h-8 min-w-0 flex-1 px-2 py-1 font-mono text-13 sm:max-w-xl', issues.peakDatesInvalid ? 'border-af-danger' : '']"
            :data-testid="testId ? `${testId}-peak-exclude-dates` : undefined"
          />
          <span class="text-xs" :class="issues.peakDatesInvalid ? 'text-af-danger' : 'text-af-ink-3'">
            {{ issues.peakDatesInvalid ? t('admin.pricing.peak.excludeDatesInvalid') : t('admin.pricing.peak.excludeDatesCount', { count: excludeDateCount }) }}
          </span>
        </label>
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1">
          <button
            type="button"
            class="text-13 font-medium text-af-ink-2 transition-colors hover:text-af-ink"
            :data-testid="testId ? `${testId}-peak-add` : undefined"
            @click="addPeakPeriod"
          >
            {{ t('admin.pricing.peak.add') }}
          </button>
          <span class="text-xs text-af-ink-3">{{ t('admin.pricing.peak.endHint') }}</span>
        </div>
      </div>
    </td>
  </tr>
  <template v-if="expanded">
    <tr
      v-for="(segment, index) in prices.segments"
      :key="index"
      :data-testid="testId ? `${testId}-segment` : undefined"
    >
      <td colspan="2" class="py-2 pl-0 pr-3 align-middle">
        <label class="flex items-center gap-1.5 whitespace-nowrap pl-4 text-13 text-af-ink-2">
          {{ t('admin.pricing.segmentAbove') }}
          <input
            v-model="segment.above"
            type="text"
            inputmode="numeric"
            autocomplete="off"
            :placeholder="t('admin.modelCatalog.segments.abovePlaceholder')"
            :class="['input h-8 w-24 px-2 py-1 text-right text-13 tabular-nums', issues.segments[index] ? 'border-af-danger' : '']"
            :data-testid="testId ? `${testId}-segment-above` : undefined"
          />
          Token
        </label>
      </td>
      <td v-for="key in PRICE_KEYS" :key="key" class="px-2 py-2 text-right align-middle">
        <PriceInput
          v-model="segment[key]"
          compact
          :scale="PER_MILLION"
          :unit="unit"
          :label="t(`admin.pricing.columns.${key}`)"
          :placeholder="t('admin.pricing.segmentInherit')"
        />
      </td>
      <td colspan="3" class="px-3 py-2 align-middle">
        <span v-if="issues.segments[index]" class="text-xs text-af-danger">
          {{ t(`admin.modelCatalog.segments.errors.${issues.segments[index]}`) }}
        </span>
      </td>
      <td class="py-2 pl-3 pr-0 text-right align-middle">
        <button type="button" class="whitespace-nowrap text-13 text-af-ink-3 transition-colors hover:text-af-ink" @click="prices.segments.splice(index, 1)">
          {{ t('admin.pricing.remove') }}
        </button>
      </td>
    </tr>
    <tr>
      <td :colspan="COLUMN_COUNT" class="py-2 pl-0 pr-3">
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1 pl-4">
          <button
            type="button"
            class="text-13 font-medium text-af-ink-2 transition-colors hover:text-af-ink"
            :data-testid="testId ? `${testId}-segment-add` : undefined"
            @click="addSegment"
          >
            {{ t('admin.pricing.segmentAdd') }}
          </button>
          <span class="text-xs text-af-ink-3">{{ t('admin.pricing.segmentHint') }}</span>
        </div>
      </td>
    </tr>
  </template>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PriceInput from '@/components/admin/catalog/PriceInput.vue'
import {
  PRICE_KEYS,
  SEARCH_KEYS,
  clonePeakForm,
  parseExcludeDates,
  peakMaxMultiplier,
  type PeakForm,
  type PriceKey,
  type PriceRow,
  type RowIssues,
  type SearchKey
} from './pricingDraft'

/** 表格总列数：首列 + 上游模型名 + 五项价 + 分段 + 毛利 + 状态 + 操作 */
const COLUMN_COUNT = 11
const PER_MILLION = 1_000_000
const PER_THOUSAND = 1_000

const prices = defineModel<PriceRow>('prices', { required: true })
/** 忙闲时（peak-editable 给了才显示）；null = 不分忙闲时 */
const peak = defineModel<PeakForm | null>('peak', { default: null })

const props = withDefaults(
  defineProps<{
    issues: RowIssues
    /** 每格下面标的官方价（$/token） */
    refs?: Record<PriceKey, number | null>
    rowClass?: string
    testId?: string
    /** 这一行能填的搜索价；空 = 这个模型的厂商没有官方搜索工具，不显示「联网搜索」 */
    searchKeys?: SearchKey[]
    /** 搜索价输入框的占位（官方价那一行写厂商公开价） */
    searchPlaceholders?: Partial<Record<SearchKey, string>>
    /** 搜索价输入框下面的参考（承接行写官方价） */
    searchHints?: Partial<Record<SearchKey, string>>
    /** 「联网搜索」那一行的说明 */
    searchNote?: string
    /** 显示「忙闲时」开关：official = 官方价那一行（目录条目的分时），upstream = 承接行（上游忙闲时） */
    peakEditable?: 'official' | 'upstream'
    /** 承接行的快捷选项「按官方忙闲时」：这个模型（可能还没保存的）官方忙闲时 */
    officialPeak?: PeakForm | null
  }>(),
  {
    refs: undefined,
    rowClass: undefined,
    testId: undefined,
    searchKeys: () => [],
    searchPlaceholders: undefined,
    searchHints: undefined,
    searchNote: undefined,
    peakEditable: undefined,
    officialPeak: null
  }
)

const { t } = useI18n()
const unit = computed(() => t('admin.modelCatalog.editor.units.perMillion'))
const expanded = ref(false)
const segmentsInvalid = computed(() => props.issues.segments.some((error) => error != null))
const searchExpanded = ref(false)
const searchInvalid = computed(() =>
  SEARCH_KEYS.some((key) => props.issues.missing.includes(key) || props.issues.invalid.includes(key))
)

// 分段 / 搜索价有问题时自动展开，免得保存按钮灰着却看不到哪里错
watch(segmentsInvalid, (invalid) => {
  if (invalid) expanded.value = true
})
watch(
  searchInvalid,
  (invalid) => {
    if (invalid) searchExpanded.value = true
  },
  { immediate: true }
)

// ---- 上游忙闲时
const peakExpanded = ref(false)
const peakIssues = computed(() => props.issues.peak ?? [])
const peakInvalid = computed(() => peakIssues.value.some((error) => error != null) || props.issues.peakDatesInvalid === true)
const excludeDateCount = computed(() => (peak.value ? parseExcludeDates(peak.value.excludeDates).length : 0))
watch(
  peakInvalid,
  (invalid) => {
    if (invalid) peakExpanded.value = true
  },
  { immediate: true }
)
const peakLabel = computed(() => {
  if (!peak.value || peak.value.periods.length === 0) return t('admin.pricing.peak.none')
  const max = peakMaxMultiplier(peak.value)
  return max == null ? t('admin.pricing.peak.toggle') : t('admin.pricing.peak.summary', { multiplier: max })
})

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

function addPeakPeriod() {
  const period = { start: '', end: '', multiplier: '2' }
  if (peak.value) peak.value.periods.push(period)
  else peak.value = { timezone: 'Asia/Shanghai', weekdaysOnly: true, periods: [period], excludeDates: '' }
}

/** 删掉最后一个时段 = 不分忙闲时 */
function removePeakPeriod(index: number) {
  if (!peak.value) return
  peak.value.periods.splice(index, 1)
  if (peak.value.periods.length === 0) peak.value = null
}

function addSegment() {
  prices.value.segments.push({
    above: '',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_write_1h_price: null,
    cache_read_price: null
  })
  expanded.value = true
}

function formatPerMillion(value: number | null): string {
  return value == null ? '' : String(Number((value * PER_MILLION).toPrecision(6)))
}
</script>
