<template>
  <!--
    价格页按模型一块里的「售价」行（muqian 2026-10-06：每项单独填售价，没填的按官方价 × 默认售价比例）。
    五项与官方价同列；没单独定的格子灰字写实际按什么收（与后端计费同一规则，见 saleDefaults）。
    分段跟着官方价那一行的分段走（按下界对上），不能单独加删；展开后每段一行，同样可单独定或留空。
    「忙闲时」（muqian 2026-10-06：售价单独一套）：默认跟官方忙闲时，也可单独设（删光时段 = 全天一个价）。
    sale（v-model:sale）是父组件草稿里的对象，这里原地改它。
  -->
  <tr :data-testid="testId">
    <td class="py-2 pl-0 pr-3 align-middle">
      <div class="font-medium text-af-ink">{{ t('admin.pricing.sale.label') }}</div>
      <div class="text-xs text-af-ink-3">{{ t('admin.pricing.sale.hint', { ratio: ratioText }) }}</div>
    </td>
    <td class="px-2 py-2 align-middle"><span class="text-xs text-af-ink-3">{{ t('admin.pricing.sale.scope') }}</span></td>
    <td v-for="key in PRICE_KEYS" :key="key" class="px-2 py-2 text-right align-middle">
      <PriceInput
        v-model="sale.base[key]"
        compact
        :scale="PER_MILLION"
        :unit="unit"
        :label="t('admin.pricing.sale.cell', { item: t(`admin.pricing.columns.${key}`) })"
        :placeholder="formatPerMillion(defaults.base[key])"
        :test-id="`${testId}-${key}`"
      />
    </td>
    <td class="px-2 py-2 align-middle">
      <div class="flex flex-col items-start">
        <button
          v-if="segmentMins.length > 0"
          type="button"
          class="whitespace-nowrap rounded-md px-2 py-1 text-13 text-af-ink-2 transition-colors hover:bg-af-sunken hover:text-af-ink"
          :aria-expanded="expanded"
          :data-testid="`${testId}-segments-toggle`"
          @click="expanded = !expanded"
        >
          {{ t('admin.pricing.segmentsCount', { count: segmentMins.length + 1 }) }}
          <Icon :name="expanded ? 'chevronDown' : 'chevronRight'" size="xs" class="ml-0.5 inline text-af-ink-3" />
        </button>
        <button
          type="button"
          :class="['whitespace-nowrap rounded-md px-2 py-1 text-13 transition-colors hover:bg-af-sunken', peakInvalid ? 'text-af-danger' : 'text-af-ink-2 hover:text-af-ink']"
          :aria-expanded="peakExpanded"
          :data-testid="`${testId}-peak-toggle`"
          @click="peakExpanded = !peakExpanded"
        >
          {{ peakLabel }}
          <Icon :name="peakExpanded ? 'chevronDown' : 'chevronRight'" size="xs" class="ml-0.5 inline text-af-ink-3" />
        </button>
      </div>
    </td>
    <td class="px-3 py-2 text-right align-middle"><span class="text-af-ink-3">—</span></td>
    <td class="px-3 py-2 align-middle"></td>
    <td class="py-2 pl-3 pr-0 text-right align-middle">
      <button
        v-if="hasAny"
        type="button"
        class="whitespace-nowrap text-13 text-af-ink-3 transition-colors hover:text-af-ink"
        :data-testid="`${testId}-clear`"
        @click="clearAll"
      >
        {{ t('admin.pricing.sale.clear') }}
      </button>
    </td>
  </tr>
  <tr v-if="peakExpanded" :data-testid="`${testId}-peak`">
    <td colspan="2" class="py-2 pl-0 pr-3 align-top">
      <div class="pl-4 text-13 font-medium text-af-ink-2">{{ t('admin.pricing.peak.sale.title') }}</div>
      <div class="max-w-[16rem] pl-4 text-xs text-af-ink-3">{{ t('admin.pricing.peak.sale.note') }}</div>
    </td>
    <td :colspan="COLUMN_COUNT - 2" class="px-2 py-2 align-top">
      <div class="flex flex-col gap-2">
        <div class="flex flex-wrap items-center gap-x-5 gap-y-1.5 text-13 text-af-ink-2">
          <label class="flex items-center gap-1.5">
            <input
              type="radio"
              :checked="sale.peakFollowsOfficial"
              class="h-4 w-4 border-af-hairline"
              :data-testid="`${testId}-peak-follow`"
              @change="followOfficial"
            />
            {{ t('admin.pricing.peak.sale.follow', { summary: officialSummary }) }}
          </label>
          <label class="flex items-center gap-1.5">
            <input
              type="radio"
              :checked="!sale.peakFollowsOfficial"
              class="h-4 w-4 border-af-hairline"
              :data-testid="`${testId}-peak-custom`"
              @change="customize"
            />
            {{ t('admin.pricing.peak.sale.custom') }}
          </label>
        </div>
        <PeakEditor
          v-if="!sale.peakFollowsOfficial"
          v-model:peak="sale.peak"
          :issues="peakIssues"
          :dates-invalid="peakDatesInvalid(sale.peak)"
          :none-hint="t('admin.pricing.peak.sale.noneHint')"
          :official-peak="officialPeak"
          :test-id="testId"
        />
      </div>
    </td>
  </tr>
  <template v-if="expanded">
    <tr v-for="min in segmentMins" :key="min" :data-testid="`${testId}-segment`">
      <td colspan="2" class="py-2 pl-0 pr-3 align-middle">
        <span class="whitespace-nowrap pl-4 text-13 text-af-ink-2">{{ t('admin.pricing.sale.segmentAbove', { tokens: min }) }}</span>
      </td>
      <td v-for="key in PRICE_KEYS" :key="key" class="px-2 py-2 text-right align-middle">
        <PriceInput
          :model-value="sale.segments[min]?.[key] ?? null"
          compact
          :scale="PER_MILLION"
          :unit="unit"
          :label="t('admin.pricing.sale.cell', { item: t(`admin.pricing.columns.${key}`) })"
          :placeholder="formatPerMillion(defaults.segments[min]?.[key] ?? null)"
          @update:model-value="setSegment(min, key, $event)"
        />
      </td>
      <td colspan="4"></td>
    </tr>
  </template>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PriceInput from '@/components/admin/catalog/PriceInput.vue'
import PeakEditor from './PeakEditor.vue'
import {
  PRICE_KEYS,
  clonePeakForm,
  officialSegmentMins,
  peakDatesInvalid,
  peakErrors,
  peakMaxMultiplier,
  saleDefaults,
  salePeakInvalid,
  type PeakForm,
  type PriceKey,
  type PriceRow,
  type SaleRow
} from './pricingDraft'

/** 表格总列数：首列 + 上游模型名 + 五项价 + 分段 + 毛利 + 状态 + 操作 */
const COLUMN_COUNT = 11
const PER_MILLION = 1_000_000

const sale = defineModel<SaleRow>('sale', { required: true })

const props = defineProps<{
  /** 这一块（可能还没保存的）官方价：灰字默认值与分段都跟着它 */
  official: PriceRow
  /** 默认售价比例（没单独定的项 = 官方价 × 它） */
  ratio: number
  /** 这一块（可能还没保存的）官方忙闲时：售价默认跟它 */
  officialPeak: PeakForm | null
  testId: string
}>()

const { t } = useI18n()
const unit = computed(() => t('admin.modelCatalog.editor.units.perMillion'))
const expanded = ref(false)

const segmentMins = computed(() => officialSegmentMins(props.official))
const defaults = computed(() => saleDefaults(props.official, sale.value, props.ratio))
const ratioText = computed(() => (props.ratio > 0 ? `1/${Math.round(1 / props.ratio)}` : '—'))

const hasAny = computed(
  () =>
    !sale.value.peakFollowsOfficial ||
    PRICE_KEYS.some((key) => sale.value.base[key] != null) ||
    Object.values(sale.value.segments).some((prices) => PRICE_KEYS.some((key) => prices[key] != null))
)

// ---- 售价忙闲时
const peakExpanded = ref(false)
const peakIssues = computed(() => (sale.value.peakFollowsOfficial ? [] : peakErrors(sale.value.peak)))
const peakInvalid = computed(() => salePeakInvalid(sale.value))
watch(
  peakInvalid,
  (invalid) => {
    if (invalid) peakExpanded.value = true
  },
  { immediate: true }
)

/** 一个忙闲时的简述：不分忙闲时 / 忙时 ×N */
function peakSummary(form: PeakForm | null): string {
  const max = peakMaxMultiplier(form)
  return max == null ? t('admin.pricing.peak.none') : t('admin.pricing.peak.summary', { multiplier: max })
}
const officialSummary = computed(() => peakSummary(props.officialPeak))
const peakLabel = computed(() => {
  if (sale.value.peakFollowsOfficial) return t('admin.pricing.peak.sale.followShort', { summary: officialSummary.value })
  return sale.value.peak ? peakSummary(sale.value.peak) : t('admin.pricing.peak.sale.noneShort')
})

function followOfficial() {
  sale.value.peakFollowsOfficial = true
  sale.value.peak = null
}

/** 单独设：从官方忙闲时起步，改起来方便 */
function customize() {
  sale.value.peakFollowsOfficial = false
  sale.value.peak = clonePeakForm(props.officialPeak)
}

function setSegment(min: number, key: PriceKey, value: number | null) {
  const target =
    sale.value.segments[min] ??
    (sale.value.segments[min] = { input_price: null, output_price: null, cache_read_price: null, cache_write_price: null, cache_write_1h_price: null })
  target[key] = value
}

/** 全部清空 = 都按官方价 × 默认售价比例收，忙闲时跟官方 */
function clearAll() {
  for (const key of PRICE_KEYS) sale.value.base[key] = null
  sale.value.segments = {}
  followOfficial()
}

function formatPerMillion(value: number | null): string {
  return value == null ? '' : String(Number((value * PER_MILLION).toPrecision(6)))
}
</script>
