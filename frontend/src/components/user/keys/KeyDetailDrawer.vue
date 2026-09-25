<template>
  <!--
    密钥详情抽屉（muqian 2026-09-25）：点密钥行打开，不离开列表。
    「概览」：今日 / 近 30 天 / 最近使用 → 近 30 天逐日趋势（单色，费用 / Token / 请求）→ 限额（额度与三档速率，带重置）→ 其他信息 → 去用量明细。
    「使用方法」：客户端配置（原「使用密钥」对话框的正文）。改设置点右上「编辑」，仍走对话框。
  -->
  <DetailDrawer
    :show="show && apiKey !== null"
    :title="apiKey?.name || ''"
    :eyebrow="t('keys.detail.eyebrow')"
    :subtitle="apiKey ? maskApiKey(apiKey.key) : ''"
    :tabs="tabs"
    :tab="tab"
    width="lg"
    @update:tab="emit('update:tab', $event as KeyDrawerTab)"
    @close="emit('close')"
  >
    <template #actions>
      <button v-if="apiKey" type="button" class="btn btn-secondary btn-sm" data-testid="key-drawer-edit" @click="emit('edit', apiKey)">
        {{ t('common.edit') }}
      </button>
    </template>

    <div v-if="apiKey && tab === 'overview'" class="space-y-8" data-testid="key-drawer-overview">
      <dl class="grid grid-cols-3 divide-x divide-af-hairline">
        <div class="min-w-0 pr-4">
          <dt class="text-13 text-af-ink-3">{{ t('keys.today') }}</dt>
          <dd class="mt-1 truncate text-lg font-semibold text-af-ink">{{ usage ? formatCurrency(usage.today_actual_cost) : '—' }}</dd>
        </div>
        <div class="min-w-0 px-4">
          <dt class="text-13 text-af-ink-3">{{ t('keys.total') }}</dt>
          <dd class="mt-1 truncate text-lg font-semibold text-af-ink">{{ usage ? formatCurrency(usage.total_actual_cost) : '—' }}</dd>
        </div>
        <div class="min-w-0 pl-4">
          <dt class="text-13 text-af-ink-3">{{ t('keys.lastUsedAt') }}</dt>
          <dd class="mt-1 truncate text-lg font-semibold text-af-ink" :title="apiKey.last_used_at ? formatDateTime(apiKey.last_used_at) : undefined">
            {{ apiKey.last_used_at ? formatRelativeTime(apiKey.last_used_at) : t('keys.detail.neverUsed') }}
          </dd>
        </div>
      </dl>

      <section data-testid="key-drawer-trend">
        <div class="mb-3 flex items-center justify-between gap-4">
          <h3 class="text-13 font-semibold text-af-ink">{{ t('keys.detail.trendTitle') }}</h3>
          <div class="inline-flex rounded-lg bg-af-sunken p-1" role="tablist" :aria-label="t('keys.detail.trendTitle')">
            <button
              v-for="option in metricOptions"
              :key="option.key"
              type="button"
              role="tab"
              class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
              :class="metric === option.key ? 'bg-af-sheet text-af-ink' : 'text-af-ink-3 hover:text-af-ink-2'"
              :aria-selected="metric === option.key"
              @click="metric = option.key"
            >
              {{ option.label }}
            </button>
          </div>
        </div>
        <StatusState
          v-if="trendError"
          kind="error"
          :title="t('userUi.usage.loadFailed')"
          :action-label="t('userUi.usage.retry')"
          @action="loadTrend"
        />
        <UsageMetricTrend v-else :trend-data="trend" :metric="metric" :loading="trendLoading" />
      </section>

      <section data-testid="key-drawer-limits">
        <div class="mb-3 flex items-center justify-between gap-4">
          <h3 class="text-13 font-semibold text-af-ink">{{ t('keys.detail.limitsTitle') }}</h3>
          <div class="flex items-center gap-3 text-13">
            <button
              v-if="apiKey.quota > 0 && apiKey.quota_used > 0"
              type="button"
              class="text-af-ink-3 hover:text-af-ink"
              data-testid="key-drawer-reset-quota"
              @click="emit('resetQuota', apiKey)"
            >
              {{ t('keys.detail.resetQuota') }}
            </button>
            <button
              v-if="hasRateUsage"
              type="button"
              class="text-af-ink-3 hover:text-af-ink"
              data-testid="key-drawer-reset-rate"
              @click="emit('resetRateLimit', apiKey)"
            >
              {{ t('keys.detail.resetRate') }}
            </button>
          </div>
        </div>
        <ul v-if="meters.length" class="space-y-4">
          <li v-for="meter in meters" :key="meter.kind" :data-testid="`key-drawer-limit-${meter.kind}`">
            <div class="flex items-baseline justify-between gap-4 text-13">
              <span class="text-af-ink-2">{{ limitLabel(meter.kind) }}</span>
              <span class="tabular-nums" :class="LEVEL_TEXT[limitLevel(meter.ratio)]">
                {{ formatCurrency(meter.used) }} / {{ formatCurrency(meter.limit) }}
              </span>
            </div>
            <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-af-hairline">
              <div class="h-full rounded-full" :class="LEVEL_BAR[limitLevel(meter.ratio)]" :style="{ width: `${Math.min(meter.ratio, 1) * 100}%` }" />
            </div>
            <p v-if="meter.resetAt && resetText(meter.resetAt)" class="mt-1 text-xs text-af-ink-4">
              {{ t('keys.detail.resetsIn', { time: resetText(meter.resetAt) }) }}
            </p>
          </li>
        </ul>
        <p v-else class="text-13 text-af-ink-3">
          {{ t('keys.detail.noLimits') }}
          <button type="button" class="ml-1 font-medium text-af-ink hover:underline" @click="emit('edit', apiKey)">{{ t('keys.detail.setLimits') }}</button>
        </p>
      </section>

      <section>
        <h3 class="mb-1 text-13 font-semibold text-af-ink">{{ t('keys.detail.infoTitle') }}</h3>
        <dl class="divide-y divide-af-hairline">
          <DetailField :label="t('common.status')">
            <span class="inline-flex items-center gap-1.5">
              <span class="h-1.5 w-1.5 rounded-full" :class="STATUS_DOT[apiKey.status] ?? 'bg-af-ink-4'" aria-hidden="true" />
              {{ t('keys.status.' + apiKey.status) }}
            </span>
          </DetailField>
          <DetailField :label="t('keys.expiresAt')" :value="apiKey.expires_at ? formatDateTime(apiKey.expires_at) : t('keys.noExpiration')" />
          <DetailField :label="t('keys.ipWhitelist')" :value="apiKey.ip_whitelist?.join(', ')" />
          <DetailField :label="t('keys.ipBlacklist')" :value="apiKey.ip_blacklist?.join(', ')" />
          <DetailField :label="t('keys.lastUsedIP')" :value="apiKey.last_used_ip" />
          <DetailField :label="t('keys.currentConcurrency')" :value="apiKey.current_concurrency ?? 0" />
          <DetailField :label="t('keys.created')" :value="formatDateTime(apiKey.created_at)" />
          <DetailField :label="t('keys.id')" :value="`#${apiKey.id}`" />
        </dl>
      </section>

      <RouterLink
        :to="{ path: '/usage', query: { key: String(apiKey.id) } }"
        class="inline-flex items-center gap-1 text-13 font-medium text-af-ink hover:underline"
        data-testid="key-drawer-usage-link"
      >
        {{ t('keys.detail.viewUsage') }}
        <Icon name="chevronRight" size="sm" />
      </RouterLink>
    </div>

    <UseKeyModal
      v-if="apiKey"
      layout="inline"
      :show="tab === 'use'"
      :api-key="apiKey.key"
      :base-url="baseUrl"
      :site-name="siteName"
    />
  </DetailDrawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usageAPI } from '@/api'
import type { BatchApiKeyUsageStats } from '@/api/usage'
import DetailDrawer from '@/components/common/DetailDrawer.vue'
import DetailField from '@/components/common/DetailField.vue'
import Icon from '@/components/icons/Icon.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab } from '@/components/user/shell/types'
import UsageMetricTrend, { type UsageTrendMetric } from '@/components/user/usage/UsageMetricTrend.vue'
import UseKeyModal from '@/components/keys/UseKeyModal.vue'
import { formatCurrency, formatDateTime, formatRelativeTime } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import { fillTrendBuckets, formatLocalDate, trendBucketKeys } from '@/utils/trendBuckets'
import type { ApiKey, TrendDataPoint } from '@/types'
import { keyLimitMeters, limitLevel, type KeyLimitKind } from './keyAttention'

export type KeyDrawerTab = 'overview' | 'use'

const props = defineProps<{
  show: boolean
  apiKey: ApiKey | null
  usage?: BatchApiKeyUsageStats
  tab: KeyDrawerTab
  baseUrl: string
  siteName?: string
  /** 页面每分钟刷新一次的「现在」，重置倒计时跟着走 */
  now: Date
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:tab', tab: KeyDrawerTab): void
  (e: 'edit', key: ApiKey): void
  (e: 'resetQuota', key: ApiKey): void
  (e: 'resetRateLimit', key: ApiKey): void
}>()

const { t } = useI18n()

const TREND_DAYS = 30

const STATUS_DOT: Record<string, string> = {
  active: 'bg-af-ink',
  quota_exhausted: 'bg-af-warning',
  expired: 'bg-af-danger',
  inactive: 'bg-af-ink-4'
}
const LEVEL_BAR = { normal: 'bg-af-ink', warning: 'bg-af-warning', danger: 'bg-af-danger' } as const
const LEVEL_TEXT = { normal: 'text-af-ink', warning: 'text-af-warning', danger: 'text-af-danger' } as const

const tabs = computed<SectionTab[]>(() => [
  { key: 'overview', label: t('keys.detail.tabOverview') },
  { key: 'use', label: t('keys.detail.tabUse') }
])

const meters = computed(() => (props.apiKey ? keyLimitMeters(props.apiKey, props.now) : []))
const hasRateUsage = computed(() => meters.value.some((meter) => meter.kind !== 'quota' && meter.used > 0))

function limitLabel(kind: KeyLimitKind): string {
  return kind === 'quota' ? t('keys.quota') : t(`keys.detail.window.${kind}`)
}

function resetText(resetAt: string): string {
  const diff = new Date(resetAt).getTime() - props.now.getTime()
  if (diff <= 0) return ''
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const mins = Math.max(1, Math.floor((diff % 3600000) / 60000))
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

// ---------- 近 30 天逐日用量 ----------
const metric = ref<UsageTrendMetric>('cost')
const metricOptions = computed<Array<{ key: UsageTrendMetric; label: string }>>(() => [
  { key: 'cost', label: t('userUi.usage.trend.cost') },
  { key: 'tokens', label: t('userUi.usage.trend.tokens') },
  { key: 'requests', label: t('userUi.usage.trend.requests') }
])

const trend = ref<TrendDataPoint[]>([])
const trendLoading = ref(false)
const trendError = ref(false)
let trendSeq = 0

async function loadTrend() {
  const key = props.apiKey
  if (!key) return
  const seq = ++trendSeq
  trendLoading.value = true
  trendError.value = false
  try {
    const response = await usageAPI.getMyApiKeyDailyUsage(key.id, TREND_DAYS)
    if (seq !== trendSeq) return
    const end = new Date()
    const start = new Date(end)
    start.setDate(end.getDate() - (TREND_DAYS - 1))
    const points: TrendDataPoint[] = response.items.map((item) => ({
      date: item.date,
      requests: item.requests,
      input_tokens: item.input_tokens,
      output_tokens: item.output_tokens,
      cache_creation_tokens: item.cache_write_tokens,
      cache_read_tokens: item.cache_read_tokens,
      total_tokens: item.total_tokens,
      cost: item.cost,
      actual_cost: item.actual_cost
    }))
    trend.value = fillTrendBuckets(points, trendBucketKeys(formatLocalDate(start), formatLocalDate(end), 'day'))
  } catch (error) {
    if (seq !== trendSeq) return
    console.error('Failed to load key daily usage:', error)
    trendError.value = true
  } finally {
    if (seq === trendSeq) trendLoading.value = false
  }
}

watch(
  () => (props.show ? props.apiKey?.id : undefined),
  (id, previous) => {
    if (id !== undefined && id !== previous) {
      trend.value = []
      void loadTrend()
    }
  },
  { immediate: true }
)
</script>
