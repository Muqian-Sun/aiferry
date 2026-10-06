<template>
  <!--
    价格页按模型一块里的「售价」行（muqian 2026-10-06：每项单独填售价，没填的按官方价 × 默认售价比例）。
    五项与官方价同列；没单独定的格子灰字写实际按什么收（与后端计费同一规则，见 saleDefaults）。
    分段跟着官方价那一行的分段走（按下界对上），不能单独加删；展开后每段一行，同样可单独定或留空。
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
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PriceInput from '@/components/admin/catalog/PriceInput.vue'
import { PRICE_KEYS, officialSegmentMins, saleDefaults, type PriceKey, type PriceRow, type SaleRow } from './pricingDraft'

const PER_MILLION = 1_000_000

const sale = defineModel<SaleRow>('sale', { required: true })

const props = defineProps<{
  /** 这一块（可能还没保存的）官方价：灰字默认值与分段都跟着它 */
  official: PriceRow
  /** 默认售价比例（没单独定的项 = 官方价 × 它） */
  ratio: number
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
    PRICE_KEYS.some((key) => sale.value.base[key] != null) ||
    Object.values(sale.value.segments).some((prices) => PRICE_KEYS.some((key) => prices[key] != null))
)

function setSegment(min: number, key: PriceKey, value: number | null) {
  const target =
    sale.value.segments[min] ??
    (sale.value.segments[min] = { input_price: null, output_price: null, cache_read_price: null, cache_write_price: null, cache_write_1h_price: null })
  target[key] = value
}

/** 全部清空 = 都按官方价 × 默认售价比例收 */
function clearAll() {
  for (const key of PRICE_KEYS) sale.value.base[key] = null
  sale.value.segments = {}
}

function formatPerMillion(value: number | null): string {
  return value == null ? '' : String(Number((value * PER_MILLION).toPrecision(6)))
}
</script>
