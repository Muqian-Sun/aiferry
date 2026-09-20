<template>
  <!--
    首页「模型与官方参考价」：模型页同一张表的前几行，纯展示。价格是 USD / 百万 Token，与 /model-plaza 同源。
  -->
  <div class="-mx-6 overflow-x-auto">
    <table class="w-full min-w-[640px] text-sm" data-testid="home-catalog">
      <thead>
        <tr class="border-b border-af-hairline text-left text-13 text-af-ink-3">
          <th class="py-2 pl-6 pr-4 font-medium">{{ t('userUi.models.columns.model') }}</th>
          <th class="py-2 pr-4 font-medium">{{ t('userUi.models.columns.vendor') }}</th>
          <th class="py-2 pr-4 text-right font-medium">{{ t('userUi.models.columns.input') }}</th>
          <th class="py-2 pr-4 text-right font-medium">{{ t('userUi.models.columns.output') }}</th>
          <th class="py-2 pr-6 text-right font-medium">{{ t('userUi.models.columns.cacheRead') }}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-af-hairline">
        <tr v-for="entry in entries" :key="entry.id" class="h-11 hover:bg-af-sunken" data-testid="home-catalog-row">
          <td class="pl-6 pr-4 font-mono font-medium text-af-ink">{{ entry.id }}</td>
          <td class="pr-4 text-af-ink-2">{{ entry.platforms.map(platformLabel).join(' / ') }}</td>
          <td class="pr-4 text-right tabular-nums text-af-ink">{{ formatPrice(entry.official?.input) }}</td>
          <td class="pr-4 text-right tabular-nums text-af-ink">{{ formatPrice(entry.official?.output) }}</td>
          <td class="pr-6 text-right tabular-nums text-af-ink-2">{{ formatPrice(entry.official?.cacheRead) }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatCatalogPrice as formatPrice, type CatalogModel } from '@/components/modelPlaza/catalog'
import { platformLabel } from '@/utils/platformColors'

defineProps<{ entries: CatalogModel[] }>()
const { t } = useI18n()
</script>
