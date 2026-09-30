<template>
  <!--
    价格页表格里的一行价（官方价或一条承接关系的上游价）：五项价 + 「分段」开关，展开后每段一行、价格列对齐。
    prices（v-model:prices）是父组件草稿里的对象，这里原地改它（与模型编辑页的分段行同一套 TokenSegmentForm）。
    首列、上游模型名、毛利、状态、操作由父组件经插槽给出；refs 给了就在每格下面标官方价作参考（按渠道视图用）。
  -->
  <tr :class="rowClass" :data-testid="testId">
    <td class="px-3 py-2 align-middle"><slot name="lead" /></td>
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
        <span v-if="refs" class="mt-0.5 text-[11px] tabular-nums text-af-ink-4">
          {{ refs[key] == null ? t('admin.pricing.officialUnset') : t('admin.pricing.officialRef', { price: formatPerMillion(refs[key]) }) }}
        </span>
      </div>
    </td>
    <td class="px-2 py-2 align-middle">
      <button
        type="button"
        :class="['whitespace-nowrap rounded-md px-2 py-1 text-13 transition-colors hover:bg-af-sunken', segmentsInvalid ? 'text-af-danger' : 'text-af-ink-2 hover:text-af-ink']"
        :aria-expanded="expanded"
        :data-testid="testId ? `${testId}-segments-toggle` : undefined"
        @click="expanded = !expanded"
      >
        {{ prices.segments.length ? t('admin.pricing.segmentsCount', { count: prices.segments.length }) : t('admin.pricing.segmentsNone') }}
        <Icon :name="expanded ? 'chevronDown' : 'chevronRight'" size="xs" class="ml-0.5 inline text-af-ink-3" />
      </button>
    </td>
    <td class="px-3 py-2 text-right align-middle"><slot name="margin" /></td>
    <td class="px-3 py-2 align-middle"><slot name="status" /></td>
    <td class="px-3 py-2 text-right align-middle"><slot name="actions" /></td>
  </tr>
  <template v-if="expanded">
    <tr
      v-for="(segment, index) in prices.segments"
      :key="index"
      class="bg-af-sunken/60"
      :data-testid="testId ? `${testId}-segment` : undefined"
    >
      <td colspan="2" class="px-3 py-2 align-middle">
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
      <td class="px-3 py-2 text-right align-middle">
        <button type="button" class="text-13 text-af-ink-3 transition-colors hover:text-af-ink" @click="prices.segments.splice(index, 1)">
          {{ t('admin.pricing.remove') }}
        </button>
      </td>
    </tr>
    <tr class="bg-af-sunken/60">
      <td :colspan="COLUMN_COUNT" class="px-3 py-2">
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
import { PRICE_KEYS, type PriceKey, type PriceRow, type RowIssues } from './pricingDraft'

/** 表格总列数：首列 + 上游模型名 + 五项价 + 分段 + 毛利 + 状态 + 操作 */
const COLUMN_COUNT = 11
const PER_MILLION = 1_000_000

const prices = defineModel<PriceRow>('prices', { required: true })

const props = defineProps<{
  issues: RowIssues
  /** 每格下面标的官方价（$/token） */
  refs?: Record<PriceKey, number | null>
  rowClass?: string
  testId?: string
}>()

const { t } = useI18n()
const unit = computed(() => t('admin.modelCatalog.editor.units.perMillion'))
const expanded = ref(false)
const segmentsInvalid = computed(() => props.issues.segments.some((error) => error != null))

// 分段有问题时自动展开，免得保存按钮灰着却看不到哪里错
watch(segmentsInvalid, (invalid) => {
  if (invalid) expanded.value = true
})

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
