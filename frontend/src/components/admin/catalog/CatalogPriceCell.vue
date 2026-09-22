<template>
  <!--
    目录表格里的标价格：按 Token 的显示「输入 / 输出 $/M」，按次 / 图片 / 视频显示默认按次价与分档数；
    没配价的上架条目标红（上架必有价）。
  -->
  <div class="text-right tabular-nums" data-testid="model-catalog-price">
    <template v-if="isToken">
      <div :class="priced ? 'text-af-ink' : 'text-af-danger'">
        {{ formatPerMillion(entry.input_price) }} <span class="text-af-ink-4">/</span> {{ formatPerMillion(entry.output_price) }}
      </div>
      <div class="text-xs text-af-ink-4">{{ t('admin.modelCatalog.columns.perMillion') }}</div>
    </template>
    <template v-else>
      <div :class="priced ? 'text-af-ink' : 'text-af-danger'">{{ formatPerRequest(entry.per_request_price) }}</div>
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
import { formatCatalogPrice } from '@/components/modelPlaza/catalog'
import { hasPrice } from './entryRequest'

const props = defineProps<{ entry: ModelCatalogEntry }>()
const { t } = useI18n()

const isToken = computed(() => !props.entry.billing_mode || props.entry.billing_mode === 'token')
const priced = computed(() => hasPrice(props.entry))
const tierCount = computed(() => (props.entry.intervals ?? []).filter((iv) => iv.tier_label).length)

function formatPerMillion(value: number | null): string {
  return value == null ? '—' : formatCatalogPrice(value * 1_000_000)
}

function formatPerRequest(value: number | null): string {
  return value == null ? '—' : `$${Number(value.toFixed(6))}`
}
</script>
