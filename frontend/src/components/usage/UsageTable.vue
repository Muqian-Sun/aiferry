<template>
  <!-- 无外框：外观由调用方决定（用户站随页滚动、管理端套在 card 里） -->
  <div>
    <!--
      只有真有待查归属地的 IP 时才出现（原来 IP 列一显示就常驻一条空条）。一行小字 + 墨色文字链接、无底线。
      只有用户站有 IP 列；管理站的 IP 在明细详情抽屉里，逐条「获取地区」。
    -->
    <div
      v-if="showIpGeoToolbar && (pendingIpCount > 0 || ipGeoBatchLoading)"
      class="flex items-center justify-end gap-3 px-6 pb-3 text-xs"
    >
      <span v-if="pendingIpCount > 0" class="text-af-ink-3">
        {{ t('usage.ipGeo.pending', { count: pendingIpCount }) }}
      </span>
      <button
        type="button"
        class="font-medium text-af-ink underline-offset-4 transition-colors hover:underline disabled:cursor-not-allowed disabled:opacity-50"
        :disabled="ipGeoBatchLoading || pendingIpCount === 0"
        @click="handleBatchFetchIpGeo"
      >
        {{ ipGeoBatchLoading ? t('usage.ipGeo.batchFetching') : t('usage.ipGeo.batchFetch') }}
      </button>
    </div>
    <div class="overflow-auto">
      <DataTable
        :columns="columns"
        :data="data"
        :loading="loading"
        :server-side-sort="serverSideSort"
        :default-sort-key="defaultSortKey"
        :default-sort-order="defaultSortOrder"
        :clickable-rows="isAdmin || clickableRows"
        @sort="(key, order) => $emit('sort', key, order)"
        @row-click="(row: AdminUsageLog) => $emit('rowClick', row)"
      >
        <template #cell-user="{ row }">
          <div class="text-sm">
            <button
              v-if="row.user?.email"
              class="font-medium text-af-brand underline decoration-dashed underline-offset-2 transition-colors hover:text-af-brand-hover"
              @click.stop="$emit('userClick', row.user_id, row.user?.email)"
              :title="t('admin.usage.clickToViewBalance')"
            >
              {{ row.user.email }}
            </button>
            <!-- 用户列只有管理站有；查不到用户（已被彻底删除）时写「已删除用户」，不露内部 id -->
            <span v-else class="font-medium text-af-ink-3">{{ t('common.deletedUser') }}</span>
            <span v-if="row.user?.deleted_at" class="ml-1 inline-flex items-center rounded px-1 py-px text-xs font-medium leading-tight bg-af-danger-tint text-af-danger ring-1 ring-inset ring-af-danger/30">
              {{ t('admin.usage.userDeletedBadge') }}
            </span>
          </div>
        </template>

        <!-- 管理站：密钥 / 渠道查不到名字就是已删除；用户站保持原样 -->
        <template #cell-api_key="{ row }">
          <span class="text-sm text-af-ink">{{ row.api_key?.name || (isAdmin ? t('common.deletedKey') : '-') }}</span>
        </template>

        <template #cell-account="{ row }">
          <span class="text-sm text-af-ink">{{ row.account?.name || (isAdmin ? t('common.deletedChannel') : '-') }}</span>
        </template>

        <template #cell-model="{ row }">
          <div class="space-y-0.5 text-xs">
            <div v-if="row.upstream_model && row.upstream_model !== row.model" class="space-y-0.5">
              <div class="break-all font-medium text-af-ink">
                {{ row.model }}
              </div>
              <div class="break-all text-af-ink-3">
                <span class="mr-0.5">↳</span>{{ row.upstream_model }}
              </div>
            </div>
            <span v-else class="font-medium text-af-ink">{{ row.model }}</span>
            <span
              v-if="row.web_search_delegated"
              class="inline-flex rounded bg-af-sunken px-1.5 py-px text-xs font-medium text-af-ink-2"
              :title="isAdmin ? t('usage.webSearchDelegatedAdminHint') : t('usage.webSearchDelegatedHint')"
              data-testid="usage-web-search-delegated"
            >{{ t('usage.webSearch') }}</span>
            <div
              v-if="row.upstream_model_mismatch === true && row.upstream_response_model"
              class="break-all pl-3 text-xs"
              :class="isLikelyModelVariant(row) ? 'text-af-warning' : 'text-af-danger'"
              :title="modelAuditTitle(row)"
            >
              <span class="mr-1">↳ {{ t('usage.upstreamResponseModel') }}:</span>{{ row.upstream_response_model }}
              <span
                class="ml-1 inline-flex rounded px-1 py-px text-xs font-medium ring-1 ring-inset"
                :class="isLikelyModelVariant(row)
                  ? 'bg-af-warning-tint text-af-warning ring-af-warning/30'
                  : 'bg-af-danger-tint text-af-danger ring-af-danger/30'"
              >
                {{ isLikelyModelVariant(row) ? t('usage.modelVariant') : t('usage.modelMismatch') }}
              </span>
            </div>
          </div>
        </template>

        <template #cell-reasoning_effort="{ row }">
          <div v-if="hasReasoningEffortMapping(row)" data-testid="reasoning-effort-cell" class="space-y-0.5 text-xs">
            <div class="font-medium text-af-ink">
              {{ formatReasoningEffort(row.reasoning_effort) }}
            </div>
            <div class="text-af-ink-3">
              <span class="mr-0.5">↳</span>{{ formatReasoningEffort(row.upstream_reasoning_effort) }}
            </div>
          </div>
          <span v-else data-testid="reasoning-effort-cell" class="text-sm text-af-ink">
            {{ formatReasoningEffort(row.reasoning_effort) }}
          </span>
        </template>

        <template #cell-endpoint="{ row }">
          <div class="max-w-[320px] space-y-1 text-xs">
            <div class="break-all text-af-ink-2">
              <span class="font-medium text-af-ink-3">{{ t('usage.inbound') }}:</span>
              <span class="ml-1">{{ row.inbound_endpoint?.trim() || '-' }}</span>
            </div>
          </div>
        </template>

        <template #cell-stream="{ row }">
          <div class="flex flex-wrap items-center gap-1">
            <span data-testid="request-type-badge" class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="getRequestTypeBadgeClass(row)">
              {{ requestTypeLabel(row, t) }}
            </span>
            <span
              v-if="row.native_compaction_v2"
              data-testid="native-compaction-badge"
              class="inline-flex items-center rounded bg-af-brand-tint px-2 py-0.5 text-xs font-medium text-af-brand"
            >
              {{ t('usage.nativeCompactionV2') }}
            </span>
          </div>
        </template>

        <template #cell-billing_mode="{ row }">
          <span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-af-sunken text-af-ink-2">
            {{ getBillingModeLabel(getDisplayBillingMode(row), t) }}
          </span>
        </template>

        <template #cell-tokens="{ row }">
          <!-- 管理站：一格只写一个数（按次计费的图片请求写张数），悬停看输入 / 输出 / 缓存 -->
          <span
            v-if="isAdmin"
            class="cursor-help text-sm font-medium tabular-nums text-af-ink"
            data-testid="usage-token-total"
            @mouseenter="showTokenTooltip($event, row)"
            @mouseleave="hideTokenTooltip"
          >{{ isImageUsage(row) ? `${row.image_count}${t('usage.imageUnit')}` : totalTokens(row).toLocaleString() }}</span>
          <!-- 图片生成请求（仅按次计费时显示图片格式） -->
          <div v-else-if="isImageUsage(row)" class="flex items-center gap-1.5">
            <svg class="h-4 w-4 text-af-brand" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
            <span class="font-medium text-af-ink">{{ row.image_count }}{{ t('usage.imageUnit') }}</span>
            <span class="text-af-ink-3">({{ formatImageBillingSize(row, t) }})</span>
          </div>
          <!-- Token 请求 -->
          <div v-else class="flex items-center gap-1.5">
            <div class="space-y-1 text-sm">
              <div class="flex items-center gap-2">
                <div class="inline-flex items-center gap-1">
                  <Icon name="arrowDown" size="sm" class="h-3.5 w-3.5 text-af-ink-3" />
                  <span class="font-medium text-af-ink">{{ row.input_tokens?.toLocaleString() || 0 }}</span>
                </div>
                <div class="inline-flex items-center gap-1">
                  <Icon name="arrowUp" size="sm" class="h-3.5 w-3.5 text-af-ink-3" />
                  <span class="font-medium text-af-ink">{{ row.output_tokens?.toLocaleString() || 0 }}</span>
                </div>
              </div>
              <div v-if="row.cache_read_tokens > 0 || row.cache_creation_tokens > 0" class="flex items-center gap-2">
                <div v-if="row.cache_read_tokens > 0" class="inline-flex items-center gap-1">
                  <svg class="h-3.5 w-3.5 text-af-ink-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" /></svg>
                  <span class="font-medium text-af-ink-2">{{ formatCacheTokens(row.cache_read_tokens) }}</span>
                </div>
                <div v-if="row.cache_creation_tokens > 0" class="inline-flex items-center gap-1">
                  <svg class="h-3.5 w-3.5 text-af-ink-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
                  <span class="font-medium text-af-ink-2">{{ formatCacheTokens(row.cache_creation_tokens) }}</span>
                  <span v-if="row.cache_creation_1h_tokens > 0" class="inline-flex items-center rounded px-1 py-px text-xs font-medium leading-tight bg-af-sunken text-af-ink-2">1h</span>
                  <span v-if="row.cache_ttl_overridden" :title="t('usage.cacheTtlOverriddenHint')" class="inline-flex items-center rounded px-1 py-px text-xs font-medium leading-tight bg-af-danger-tint text-af-danger ring-1 ring-inset ring-af-danger/30 cursor-help">R</span>
                </div>
              </div>
              <div v-if="hasImageInputTokens(row)" class="flex items-center gap-2">
                <div class="inline-flex items-center gap-1">
                  <svg class="h-3.5 w-3.5 text-af-ink-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>
                  <span class="font-medium text-af-ink-2">{{ row.image_input_tokens.toLocaleString() }}</span>
                </div>
              </div>
              <div v-if="hasImageOutputTokens(row)" class="flex items-center gap-2">
                <div class="inline-flex items-center gap-1">
                  <svg class="h-3.5 w-3.5 text-af-ink-3" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>
                  <span class="font-medium text-af-ink-2">{{ row.image_output_tokens.toLocaleString() }}</span>
                </div>
              </div>
              <div v-if="row.web_search_count > 0" class="inline-flex items-center gap-1 text-af-ink-2" data-testid="usage-web-search">
                <Icon name="search" size="sm" class="h-3.5 w-3.5 text-af-ink-3" />
                <span class="font-medium">{{ t('usage.webSearchTimes', { count: row.web_search_count }) }}</span>
              </div>
            </div>
            <!-- Token Detail Tooltip -->
            <div
              class="group relative"
              @mouseenter="showTokenTooltip($event, row)"
              @mouseleave="hideTokenTooltip"
            >
              <div class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-af-sunken transition-colors group-hover:bg-af-brand-tint">
                <Icon name="infoCircle" size="xs" class="text-af-ink-3 group-hover:text-af-brand" />
              </div>
            </div>
          </div>
        </template>

        <template #cell-cost="{ row }">
          <!-- 管理站：这一列是收入，按方案两位小数（不足一分写 <$0.01）；单笔精确金额、成本、利润在详情抽屉 -->
          <span v-if="isAdmin" class="text-sm font-medium tabular-nums text-af-ink" :title="formatMoneyExact(row.actual_cost)">{{ formatMoney(row.actual_cost) }}</span>
          <div v-else class="text-sm">
            <div class="flex items-center gap-1.5">
              <span class="font-medium tabular-nums text-af-ink">${{ row.actual_cost?.toFixed(6) || '0.000000' }}</span>
              <!-- Cost Detail Tooltip -->
              <div
                class="group relative"
                @mouseenter="showTooltip($event, row)"
                @mouseleave="hideTooltip"
              >
                <div class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-af-sunken transition-colors group-hover:bg-af-brand-tint">
                  <Icon name="infoCircle" size="xs" class="text-af-ink-3 group-hover:text-af-brand" />
                </div>
              </div>
            </div>
          </div>
        </template>

        <!-- 合并首字/总耗时的健康度列：左侧色条上半随首字档、下半随总耗时档，便于纵向扫视整体健康状况 -->
        <template #cell-latency="{ row }">
          <!-- 管理站：只写总耗时（按档着色），首字耗时在 title 与详情抽屉里 -->
          <span
            v-if="isAdmin"
            class="text-sm font-medium tabular-nums"
            :class="LATENCY_TEXT_CLASSES[durationSeverity(row.duration_ms ?? 0)]"
            :title="row.first_token_ms != null ? `${t('usage.latencyFirstToken')} ${formatDuration(row.first_token_ms)}` : undefined"
          >{{ formatDuration(row.duration_ms) }}</span>
          <div v-else class="flex items-stretch gap-2">
            <span class="flex w-1 shrink-0 flex-col overflow-hidden rounded-full" aria-hidden="true">
              <span class="flex-1" :class="LATENCY_BAR_CLASSES[row.first_token_ms != null ? firstTokenSeverity(row.first_token_ms) : durationSeverity(row.duration_ms ?? 0)]"></span>
              <span class="flex-1" :class="LATENCY_BAR_CLASSES[durationSeverity(row.duration_ms ?? 0)]"></span>
            </span>
            <div class="grid grid-cols-[max-content_max-content] items-baseline gap-x-2 gap-y-0.5 text-xs">
              <span class="text-af-ink-3">{{ t('usage.latencyFirstToken') }}</span>
              <span v-if="row.first_token_ms != null" class="font-medium tabular-nums" :class="LATENCY_TEXT_CLASSES[firstTokenSeverity(row.first_token_ms)]">{{ formatDuration(row.first_token_ms) }}</span>
              <span v-else class="text-af-ink-3">-</span>
              <span class="text-af-ink-3">{{ t('usage.latencyDuration') }}</span>
              <span class="font-medium tabular-nums" :class="LATENCY_TEXT_CLASSES[durationSeverity(row.duration_ms ?? 0)]">{{ formatDuration(row.duration_ms) }}</span>
            </div>
          </div>
        </template>

        <template #cell-created_at="{ value }">
          <span class="text-sm text-af-ink-2">{{ formatDateTime(value) }}</span>
        </template>

        <template #cell-user_agent="{ row }">
          <span v-if="row.user_agent" class="text-sm text-af-ink-2 block max-w-[320px] truncate" :title="row.user_agent">{{ row.user_agent }}</span>
          <span v-else class="text-sm text-af-ink-3">-</span>
        </template>

        <template #cell-ip_address="{ row }">
          <div v-if="row.ip_address">
            <span class="text-sm font-mono text-af-ink-2">{{ row.ip_address }}</span>
            <IpGeoCell :ip="row.ip_address" />
          </div>
          <span v-else class="text-sm text-af-ink-3">-</span>
        </template>

        <template #empty><EmptyState :message="t('usage.noRecords')" /></template>
      </DataTable>
    </div>
  </div>

  <!-- Token Tooltip Portal -->
  <Teleport to="body">
    <div
      v-if="tokenTooltipVisible"
      class="fixed z-[9999] pointer-events-none -translate-y-1/2"
      :style="{
        left: tokenTooltipPosition.x + 'px',
        top: tokenTooltipPosition.y + 'px'
      }"
    >
      <div class="whitespace-nowrap rounded-lg border border-af-hairline-strong bg-af-sheet px-3 py-2.5 text-xs text-af-ink shadow-xl">
        <!-- 管理站：只列输入 / 输出 / 缓存（5 分钟 / 1 小时缓存、图片 Token 在详情抽屉） -->
        <div v-if="isAdmin && tokenTooltipData" class="space-y-1.5" data-testid="usage-token-tooltip-admin">
          <div>
            <div class="text-xs font-semibold text-af-ink-3 mb-1">{{ t('usage.tokenDetails') }}</div>
            <div v-for="line in adminTokenLines(tokenTooltipData)" :key="line.key" class="flex items-center justify-between gap-4">
              <span class="text-af-ink-3">{{ line.label }}</span>
              <span class="font-medium tabular-nums text-af-ink">{{ line.value.toLocaleString() }}</span>
            </div>
          </div>
          <div class="flex items-center justify-between gap-6 border-t border-af-hairline-strong pt-1.5">
            <span class="text-af-ink-3">{{ t('usage.totalTokens') }}</span>
            <span class="font-semibold text-af-brand">{{ totalTokens(tokenTooltipData).toLocaleString() }}</span>
          </div>
        </div>
        <!-- 用户站：和请求详情抽屉共用一份明细 -->
        <UsageTokenBreakdown v-else-if="tokenTooltipData" :row="tokenTooltipData" />
        <div class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-af-hairline-strong border-t-transparent"></div>
      </div>
    </div>
  </Teleport>

  <!-- Cost Tooltip Portal -->
  <Teleport to="body">
    <div
      v-if="tooltipVisible"
      class="fixed z-[9999] pointer-events-none -translate-y-1/2"
      :style="{
        left: tooltipPosition.x + 'px',
        top: tooltipPosition.y + 'px'
      }"
    >
      <div class="whitespace-nowrap rounded-lg border border-af-hairline-strong bg-af-sheet px-3 py-2.5 text-xs text-af-ink shadow-xl">
        <!-- 只有用户站有费用悬浮框（管理站这一列是收入，明细在详情抽屉） -->
        <UsageCostBreakdown v-if="tooltipData" :row="tooltipData" />
        <div class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-af-hairline-strong border-t-transparent"></div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatMoney, formatMoneyExact } from '@/utils/money'
import { formatDateTime, formatReasoningEffort } from '@/utils/format'
import { formatCacheTokens } from '@/utils/formatters'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import {
  LATENCY_BAR_CLASSES,
  LATENCY_TEXT_CLASSES,
  durationSeverity,
  firstTokenSeverity,
} from '@/utils/latencyHealth'
import { getBillingModeLabel, isImageUsage, getDisplayBillingMode } from '@/utils/billingMode'
import { formatImageBillingSize, hasImageOutputTokens, hasImageInputTokens } from '@/utils/imageUsage'

import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import IpGeoCell from '@/components/common/IpGeoCell.vue'
import Icon from '@/components/icons/Icon.vue'
import UsageTokenBreakdown from './UsageTokenBreakdown.vue'
import UsageCostBreakdown from './UsageCostBreakdown.vue'
import { fetchBatch, getEntry } from '@/utils/ipGeoLookup'
import {
  formatDurationMs,
  hasReasoningEffortMapping,
  isLikelyModelVariant,
  requestTypeLabel,
  sentUpstreamModel,
  totalTokens,
} from './usageRow'
import type { AdminUsageLog } from '@/types'
import type { Column } from '@/components/common/types'

interface Props {
  data: AdminUsageLog[]
  loading?: boolean
  columns: Column[]
  serverSideSort?: boolean
  defaultSortKey?: string
  defaultSortOrder?: 'asc' | 'desc'
  /**
   * user：用户站（Token 分行、费用列带悬浮明细、延迟列两行）。
   * admin：管理站（Token 一个数、这一列是收入、耗时一个数；点行 emit rowClick，由页面打开详情抽屉）。
   */
  mode?: 'user' | 'admin'
  /** 用户站点行打开请求详情（管理站 mode=admin 时总是可点）；行里的提示图标、复制按钮都已 stop，不会误触 */
  clickableRows?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  serverSideSort: false,
  defaultSortKey: '',
  defaultSortOrder: 'asc',
  mode: 'user',
  clickableRows: false
})
const emit = defineEmits<{
  userClick: [userID: number, email?: string]
  rowClick: [row: AdminUsageLog]
  sort: [key: string, order: 'asc' | 'desc']
  ipGeoBatchFailed: []
}>()
const { t } = useI18n()
const isAdmin = computed(() => props.mode === 'admin')
const ipGeoBatchLoading = ref(false)

const showIpGeoToolbar = computed(() => props.columns.some((col) => col.key === 'ip_address'))

const modelAuditTitle = (row: AdminUsageLog): string => [
  `${t('usage.requestedModel')}: ${row.model || '-'}`,
  `${t('usage.sentUpstreamModel')}: ${sentUpstreamModel(row) || '-'}`,
  `${t('usage.upstreamResponseModel')}: ${row.upstream_response_model || '-'}`,
].join('\n')

const currentPageIps = computed(() =>
  Array.from(new Set(props.data.map((row) => row.ip_address).filter((ip): ip is string => Boolean(ip))))
)

const pendingIpCount = computed(() => {
  if (!showIpGeoToolbar.value) return 0
  return currentPageIps.value.filter((ip) => {
    const status = getEntry(ip).status
    return status === 'idle' || status === 'error'
  }).length
})

const handleBatchFetchIpGeo = async () => {
  ipGeoBatchLoading.value = true
  try {
    const ok = await fetchBatch(currentPageIps.value)
    if (!ok) emit('ipGeoBatchFailed')
  } finally {
    ipGeoBatchLoading.value = false
  }
}

// Tooltip state - cost
const tooltipVisible = ref(false)
const tooltipPosition = ref({ x: 0, y: 0 })
const tooltipData = ref<AdminUsageLog | null>(null)

// Tooltip state - token
const tokenTooltipVisible = ref(false)
const tokenTooltipPosition = ref({ x: 0, y: 0 })
const tokenTooltipData = ref<AdminUsageLog | null>(null)

/** 请求类型标签：只有 Cyber（被安全策略拦下）标红，其余一律中性（控制台单色为主，muqian 2026-09-23） */
const getRequestTypeBadgeClass = (row: AdminUsageLog): string =>
  resolveUsageRequestType(row) === 'cyber' ? 'bg-af-danger-tint text-af-danger' : 'bg-af-sunken text-af-ink-2'

/** 管理站 Token 悬浮框：输入 / 输出一定列出，缓存只在有时列出。 */
const adminTokenLines = (row: AdminUsageLog): Array<{ key: string; label: string; value: number }> => [
  { key: 'input', label: t('admin.usage.inputTokens'), value: row.input_tokens || 0 },
  { key: 'output', label: t('admin.usage.outputTokens'), value: row.output_tokens || 0 },
  ...(row.cache_read_tokens > 0 ? [{ key: 'cacheRead', label: t('admin.usage.cacheReadTokens'), value: row.cache_read_tokens }] : []),
  ...(row.cache_creation_tokens > 0 ? [{ key: 'cacheCreation', label: t('admin.usage.cacheCreationTokens'), value: row.cache_creation_tokens }] : []),
  ...(row.web_search_count > 0 ? [{ key: 'webSearch', label: t('usage.webSearchCount'), value: row.web_search_count }] : []),
]

const formatDuration = (ms: number | null | undefined): string => (ms == null ? '-' : formatDurationMs(ms))

// Cost tooltip functions
const showTooltip = (event: MouseEvent, row: AdminUsageLog) => {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  tooltipData.value = row
  tooltipPosition.value.x = rect.right + 8
  tooltipPosition.value.y = rect.top + rect.height / 2
  tooltipVisible.value = true
}

const hideTooltip = () => {
  tooltipVisible.value = false
  tooltipData.value = null
}

// Token tooltip functions
const showTokenTooltip = (event: MouseEvent, row: AdminUsageLog) => {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  tokenTooltipData.value = row
  tokenTooltipPosition.value.x = rect.right + 8
  tokenTooltipPosition.value.y = rect.top + rect.height / 2
  tokenTooltipVisible.value = true
}

const hideTokenTooltip = () => {
  tokenTooltipVisible.value = false
  tokenTooltipData.value = null
}
</script>
