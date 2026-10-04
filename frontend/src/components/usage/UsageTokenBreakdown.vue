<template>
  <!--
    一条用量记录的 Token 明细（输入 / 输出 / 图片 / 缓存写 5m·1h / 缓存读 / 合计）。
    用量表的悬停提示和用户站的请求详情抽屉共用这一份，两处口径不会分叉。
  -->
  <div class="space-y-1.5">
    <div>
      <div v-if="showTitle" class="text-xs font-semibold text-af-ink-3 mb-1">{{ t('usage.tokenDetails') }}</div>
      <div v-if="row && row.input_tokens > 0 && !hasImageInputTokens(row)" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('admin.usage.inputTokens') }}</span>
        <span class="font-medium text-af-ink">{{ row.input_tokens.toLocaleString() }}</span>
      </div>
      <div v-if="row && hasImageInputTokens(row) && textInputTokens(row) > 0" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('admin.usage.inputTokens') }}</span>
        <span class="font-medium text-af-ink">{{ textInputTokens(row).toLocaleString() }}</span>
      </div>
      <div v-if="row && hasImageInputTokens(row)" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('usage.imageInputTokens') }}</span>
        <span class="font-medium text-af-ink-2">{{ row.image_input_tokens.toLocaleString() }}</span>
      </div>
      <div v-if="row && row.output_tokens > 0 && !hasImageOutputTokens(row)" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('admin.usage.outputTokens') }}</span>
        <span class="font-medium text-af-ink">{{ row.output_tokens.toLocaleString() }}</span>
      </div>
      <div v-if="row && hasImageOutputTokens(row) && textOutputTokens(row) > 0" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('admin.usage.outputTokens') }}</span>
        <span class="font-medium text-af-ink">{{ textOutputTokens(row).toLocaleString() }}</span>
      </div>
      <div v-if="row && hasImageOutputTokens(row)" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('usage.imageOutputTokens') }}</span>
        <span class="font-medium text-af-ink-2">{{ row.image_output_tokens.toLocaleString() }}</span>
      </div>
      <div v-if="row && row.cache_creation_tokens > 0">
        <!-- 有 5m/1h 明细时，展开显示 -->
        <template v-if="row.cache_creation_5m_tokens > 0 || row.cache_creation_1h_tokens > 0">
          <div v-if="row.cache_creation_5m_tokens > 0" class="flex items-center justify-between gap-4">
            <span class="text-af-ink-3 flex items-center gap-1.5">
              {{ t('admin.usage.cacheCreation5mTokens') }}
              <span class="inline-flex items-center rounded px-1 py-px text-xs font-medium leading-tight bg-af-sunken text-af-ink-2">5m</span>
            </span>
            <span class="font-medium text-af-ink">{{ row.cache_creation_5m_tokens.toLocaleString() }}</span>
          </div>
          <div v-if="row.cache_creation_1h_tokens > 0" class="flex items-center justify-between gap-4">
            <span class="text-af-ink-3 flex items-center gap-1.5">
              {{ t('admin.usage.cacheCreation1hTokens') }}
              <span class="inline-flex items-center rounded px-1 py-px text-xs font-medium leading-tight bg-af-sunken text-af-ink-2">1h</span>
            </span>
            <span class="font-medium text-af-ink">{{ row.cache_creation_1h_tokens.toLocaleString() }}</span>
          </div>
        </template>
        <!-- 无明细时，只显示聚合值 -->
        <div v-else class="flex items-center justify-between gap-4">
          <span class="text-af-ink-3">{{ t('admin.usage.cacheCreationTokens') }}</span>
          <span class="font-medium text-af-ink">{{ row.cache_creation_tokens.toLocaleString() }}</span>
        </div>
      </div>
      <div v-if="row && row.cache_ttl_overridden" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3 flex items-center gap-1.5">
          {{ t('usage.cacheTtlOverriddenLabel') }}
          <span class="inline-flex items-center rounded px-1 py-px text-xs font-medium leading-tight bg-af-danger/20 text-af-danger ring-1 ring-inset ring-af-danger/30">R-{{ row.cache_creation_1h_tokens > 0 ? '5m' : '1H' }}</span>
        </span>
        <span class="font-medium text-af-danger">{{ row.cache_creation_1h_tokens > 0 ? t('usage.cacheTtlOverridden1h') : t('usage.cacheTtlOverridden5m') }}</span>
      </div>
      <div v-if="row && row.cache_read_tokens > 0" class="flex items-center justify-between gap-4">
        <span class="text-af-ink-3">{{ t('admin.usage.cacheReadTokens') }}</span>
        <span class="font-medium text-af-ink">{{ row.cache_read_tokens.toLocaleString() }}</span>
      </div>
    </div>
    <div class="flex items-center justify-between gap-6 border-t border-af-hairline-strong pt-1.5">
      <span class="text-af-ink-3">{{ t('usage.totalTokens') }}</span>
      <span class="font-semibold text-af-brand">{{ ((row?.input_tokens || 0) + (row?.output_tokens || 0) + (row?.cache_creation_tokens || 0) + (row?.cache_read_tokens || 0)).toLocaleString() }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { hasImageInputTokens, hasImageOutputTokens, textInputTokens, textOutputTokens } from '@/utils/imageUsage'
import type { AdminUsageLog } from '@/types'

/** showTitle：悬停提示里自带小标题；抽屉里由外面的分节标题代替 */
withDefaults(defineProps<{ row: AdminUsageLog; showTitle?: boolean }>(), { showTitle: true })

const { t } = useI18n()
</script>
