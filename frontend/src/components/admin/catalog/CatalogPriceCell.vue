<template>
  <!--
    目录表格里的标价格：按 Token 的第一行「$输入 / $输出」、第二行「输入 / 输出 · 每百万 Token」；
    按次 / 图片 / 视频第一行默认按次价、第二行「每次 / 每张 / 每秒 · N 档」。
    整列共用一个小数位数（decimals 由列表页按当前筛选结果算好传进来）。
    没配价写「未配价」，只有上架的才标红（上架必有价；未上架的没配价是常态）。
  -->
  <div class="text-right tabular-nums" data-testid="model-catalog-price">
    <div
      v-if="!priced"
      :class="entry.status === 'listed' ? 'text-af-danger' : 'text-af-ink-4'"
      data-testid="model-catalog-price-missing"
    >
      {{ t('admin.modelCatalog.columns.unpriced') }}
    </div>
    <template v-else-if="isToken">
      <div class="text-af-ink">
        {{ formatListPrice(prices[0], decimals) }} <span class="text-af-ink-4">/</span> {{ formatListPrice(prices[1], decimals) }}
      </div>
      <div class="text-xs text-af-ink-4">{{ t('admin.modelCatalog.columns.perMillion') }}</div>
    </template>
    <template v-else>
      <div class="text-af-ink">{{ formatListPrice(prices[0], decimals) }}</div>
      <div class="text-xs text-af-ink-4">
        {{ t(`admin.modelCatalog.columns.perUnit.${entry.billing_mode}`) }}
        <template v-if="tierCount"> · {{ t('admin.modelCatalog.columns.tiers', { count: tierCount }) }}</template>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import { hasPrice } from './entryRequest'
import { formatListPrice, listPriceValues } from './priceFormat'

const props = defineProps<{
  entry: ModelCatalogEntry
  /** 整列共用的小数位数（sharedPriceDecimals 算出） */
  decimals: number
}>()
const { t } = useI18n()

const isToken = computed(() => !props.entry.billing_mode || props.entry.billing_mode === 'token')
const priced = computed(() => hasPrice(props.entry))
const tierCount = computed(() => (props.entry.intervals ?? []).filter((iv) => iv.tier_label).length)
// 按 Token：[输入, 输出]（$ / 百万 Token）；其余计费：[默认按次价]
const prices = computed(() => listPriceValues(props.entry))
</script>
