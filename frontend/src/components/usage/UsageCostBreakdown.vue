<template>
  <!--
    一条用量记录的费用明细（各项费用、单价、图片计费与实付）。
    只有用户站用：用量表的悬停提示和请求详情抽屉共用这一份（管理站的成本 / 利润在管理端详情抽屉）。单价原来写的是 text-af-on-brand（白底上看不见），统一成墨色。
    倍率与官方价不给用户看（muqian 2026-09-30）：用户站接口给的各项费用已由后端折成实付口径（2026-10-04 D1），这里原样显示；
    联网搜索费按原价收、不乘倍率，单列一行。各项加起来就是实付；单价同样按实付口径算。
  -->
  <div class="space-y-1.5">
    <!-- Cost Breakdown -->
    <div class="mb-2 border-b border-af-hairline-strong pb-1.5">
      <div v-if="showTitle" class="text-xs font-semibold text-af-ink-3 mb-1">{{ t('usage.costDetails') }}</div>
      <p v-if="row?.web_search_delegated" class="mb-1 max-w-[16rem] text-xs text-af-ink-3" data-testid="usage-cost-delegated-note">{{ t('usage.webSearchDelegatedHint') }}</p>
      <div v-if="row && row.input_cost > 0" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('admin.usage.inputCost') }}</span>
        <span class="font-medium text-af-ink">${{ row.input_cost.toFixed(8) }}</span>
      </div>
      <div v-if="row && hasImageInputCost(row)" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('usage.imageInputCost') }}</span>
        <span class="font-medium text-af-ink-2">${{ row.image_input_cost.toFixed(8) }}</span>
      </div>
      <div v-if="row && row.output_cost > 0" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('admin.usage.outputCost') }}</span>
        <span class="font-medium text-af-ink">${{ row.output_cost.toFixed(8) }}</span>
      </div>
      <div v-if="row && hasImageOutputCost(row)" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('usage.imageOutputCost') }}</span>
        <span class="font-medium text-af-ink-2">${{ row.image_output_cost.toFixed(8) }}</span>
      </div>
      <!-- Token billing: show unit prices per 1M tokens -->
      <template v-if="row && !isImageUsage(row) && (!row.billing_mode || row.billing_mode === BILLING_MODE_TOKEN)">
        <div v-if="row && textInputTokens(row) > 0" class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.inputTokenPrice') }}</span>
          <span class="font-medium text-af-ink">{{ formatTokenPricePerMillion(row.input_cost, textInputTokens(row)) }} {{ t('usage.perMillionTokens') }}</span>
        </div>
        <div v-if="row && hasImageInputTokens(row)" class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageInputTokenPrice') }}</span>
          <span class="font-medium text-af-ink-2">{{ formatTokenPricePerMillion(row.image_input_cost, row.image_input_tokens) }} {{ t('usage.perMillionTokens') }}</span>
        </div>
        <div v-if="row && row.output_cost > 0 && textOutputTokens(row) > 0" class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.outputTokenPrice') }}</span>
          <span class="font-medium text-af-ink">{{ formatTokenPricePerMillion(row.output_cost, textOutputTokens(row)) }} {{ t('usage.perMillionTokens') }}</span>
        </div>
        <div v-if="row && hasImageOutputTokens(row)" class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageOutputTokenPrice') }}</span>
          <span class="font-medium text-af-ink-2">{{ formatTokenPricePerMillion(row.image_output_cost, row.image_output_tokens) }} {{ t('usage.perMillionTokens') }}</span>
        </div>
      </template>
      <template v-else-if="row && isImageUsage(row)">
        <div class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageCount') }}</span>
          <span class="font-medium text-af-ink">{{ row.image_count }}{{ t('usage.imageUnit') }}</span>
        </div>
        <div class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageBillingSize') }}</span>
          <span class="font-medium text-af-ink">{{ formatImageBillingSize(row, t) }}</span>
        </div>
        <div class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageSizeSource') }}</span>
          <span class="font-medium text-af-ink">{{ formatImageSizeSource(row, t) }}</span>
        </div>
        <div class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageInputSize') }}</span>
          <span class="font-medium text-af-ink">{{ formatImageInputSize(row, t) }}</span>
        </div>
        <div class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageOutputSize') }}</span>
          <span class="font-medium text-af-ink">{{ formatImageOutputSize(row, t) }}</span>
        </div>
        <div v-if="formatImageSizeBreakdown(row)" class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageSizeBreakdown') }}</span>
          <span class="font-medium text-af-ink">{{ formatImageSizeBreakdown(row) }}</span>
        </div>
        <div class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageUnitPrice') }}</span>
          <span class="font-medium text-af-ink">${{ imageUnitPrice(row).toFixed(8) }}</span>
        </div>
        <div class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('usage.imageTotalPrice') }}</span>
          <span class="font-medium text-af-ink">${{ row.actual_cost?.toFixed(8) || '0.00000000' }}</span>
        </div>
      </template>
      <div v-else class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('usage.unitPrice') }}</span>
        <span class="font-medium text-af-ink">${{ row?.actual_cost?.toFixed(8) || '0.00000000' }}</span>
      </div>
      <div v-if="row && row.cache_creation_cost > 0" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('admin.usage.cacheCreationCost') }}</span>
        <span class="font-medium text-af-ink">${{ row.cache_creation_cost.toFixed(8) }}</span>
      </div>
      <div v-if="row && row.cache_read_cost > 0" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('admin.usage.cacheReadCost') }}</span>
        <span class="font-medium text-af-ink">${{ row.cache_read_cost.toFixed(8) }}</span>
      </div>
      <div v-if="row && row.web_search_count > 0" class="flex items-center justify-between gap-4" data-testid="usage-cost-web-search">
        <span class="text-af-ink-3">{{ t('usage.webSearch') }} · {{ t('usage.webSearchTimes', { count: row.web_search_count }) }}</span>
        <span class="font-medium text-af-ink">${{ (row.web_search_cost ?? 0).toFixed(8) }}</span>
      </div>
    </div>
    <!-- Summary -->
    <div class="flex items-center justify-between gap-6">
      <span class="text-af-ink-3">{{ t('usage.serviceTier') }}</span>
      <span class="font-semibold text-af-ink">{{ getUsageServiceTierLabel(row?.service_tier, t) }}</span>
    </div>
    <div class="flex items-center justify-between gap-6">
      <span class="text-af-ink-3">{{ t('usage.userBilled') }}</span>
      <span class="font-semibold text-af-ink">${{ row?.actual_cost?.toFixed(8) || '0.00000000' }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatTokenPricePerMillion } from '@/utils/usagePricing'
import { getUsageServiceTierLabel } from '@/utils/usageServiceTier'
import { BILLING_MODE_TOKEN, imageUnitPrice, isImageUsage } from '@/utils/billingMode'
import {
  formatImageBillingSize,
  formatImageInputSize,
  formatImageOutputSize,
  formatImageSizeBreakdown,
  formatImageSizeSource,
  hasImageInputCost,
  hasImageInputTokens,
  hasImageOutputCost,
  hasImageOutputTokens,
  textInputTokens,
  textOutputTokens
} from '@/utils/imageUsage'
import type { UsageLog } from '@/types'

/** showTitle：悬停提示里自带小标题；抽屉里由外面的分节标题代替 */
withDefaults(defineProps<{ row: UsageLog; showTitle?: boolean }>(), { showTitle: true })

const { t } = useI18n()

</script>
