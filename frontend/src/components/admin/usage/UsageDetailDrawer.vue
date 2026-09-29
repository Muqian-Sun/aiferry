<template>
  <!--
    用量明细详情抽屉：明细表点行打开。表格只留 7 列，其余都在这里——
    费用构成（收入 / 成本 / 利润，单笔精确金额）、请求（端点、请求 ID、IP、UA…）、Token 明细、耗时。
    「标准价」不出现：成本、收入都已按各自倍率算好，倍率单列在费用构成下面。
  -->
  <DetailDrawer
    :show="!!log"
    :title="log?.model ?? ''"
    :eyebrow="log ? formatDateTime(log.created_at) : ''"
    :subtitle="log ? (log.user?.email || t('common.deletedUser')) : ''"
    @close="emit('close')"
  >
    <div v-if="log" class="space-y-8" data-testid="usage-detail">
      <!-- 费用构成 -->
      <section data-testid="usage-detail-money">
        <h3 class="text-13 font-medium text-af-ink-3">{{ t('admin.usage.detail.sections.money') }}</h3>
        <dl class="mt-2 grid grid-cols-3 gap-4">
          <div v-for="item in moneyItems" :key="item.key" :data-testid="`usage-detail-${item.key}`">
            <dt class="text-13 text-af-ink-3" :title="item.hint">{{ item.label }}</dt>
            <dd class="mt-1 text-xl font-semibold tabular-nums" :class="item.valueClass">{{ formatMoneyExact(item.value) }}</dd>
          </div>
        </dl>
        <dl class="mt-4 divide-y divide-af-hairline border-t border-af-hairline">
          <!-- 用户倍率 0 = 免费（库里 NOT NULL DEFAULT 1），不能用 || 把 0 当成没记 -->
          <DetailField :label="t('admin.usage.detail.userRate')" :value="`${formatMultiplier(log.rate_multiplier ?? 1)}x`" />
          <DetailField :label="t('admin.usage.detail.accountRate')" :value="`${formatMultiplier(log.account_rate_multiplier ?? 1)}x`" />
          <DetailField
            :label="t('admin.usage.billingType')"
            :value="log.billing_type === 1 ? t('admin.usage.billingTypeSubscription') : t('admin.usage.billingTypeBalance')"
          />
          <DetailField :label="t('admin.usage.billingMode')" :value="getBillingModeLabel(getDisplayBillingMode(log), t)" />
          <DetailField :label="t('usage.serviceTier')" :value="getUsageServiceTierLabel(log.service_tier, t)" />
          <DetailField
            v-if="log.long_context_billing_applied"
            :label="t('admin.usage.detail.longContext')"
            :value="t('admin.usage.detail.longContextApplied')"
          />
        </dl>
      </section>

      <!-- 请求 -->
      <section data-testid="usage-detail-request">
        <h3 class="text-13 font-medium text-af-ink-3">{{ t('admin.usage.detail.sections.request') }}</h3>
        <dl class="divide-y divide-af-hairline">
          <DetailField :label="t('admin.usage.user')">
            <template v-if="log.user?.email">
              {{ log.user.email }}
              <span v-if="log.user.deleted_at" class="ml-1 inline-flex items-center rounded bg-af-danger-tint px-1 py-px text-[10px] font-medium leading-tight text-af-danger">
                {{ t('admin.usage.userDeletedBadge') }}
              </span>
            </template>
            <template v-else>{{ t('common.deletedUser') }}</template>
          </DetailField>
          <!-- 名字查不到就是已删除（不露内部 id） -->
          <DetailField :label="t('usage.apiKeyFilter')" :value="log.api_key?.name || t('common.deletedKey')" />
          <DetailField :label="t('admin.usage.account')" :value="log.account?.name || t('common.deletedChannel')" />
          <DetailField :label="t('usage.model')">
            <div class="space-y-0.5">
              <div class="break-all">{{ log.model }}</div>
              <div v-if="sentUpstreamModel(log) !== log.model" class="break-all text-af-ink-3">
                ↳ {{ t('usage.sentUpstreamModel') }}{{ t('common.labelSeparator') }}{{ sentUpstreamModel(log) }}
              </div>
              <div
                v-if="log.upstream_model_mismatch === true && log.upstream_response_model"
                class="break-all"
                :class="isLikelyModelVariant(log) ? 'text-af-warning' : 'text-af-danger'"
                data-testid="usage-detail-model-mismatch"
              >
                ↳ {{ t('usage.upstreamResponseModel') }}{{ t('common.labelSeparator') }}{{ log.upstream_response_model }}
                <span class="ml-1 text-xs font-medium">{{ isLikelyModelVariant(log) ? t('usage.modelVariant') : t('usage.modelMismatch') }}</span>
              </div>
            </div>
          </DetailField>
          <DetailField
            v-if="log.reasoning_effort || log.upstream_reasoning_effort"
            :label="t('usage.reasoningEffort')"
            :value="hasReasoningEffortMapping(log)
              ? `${formatReasoningEffort(log.reasoning_effort)} → ${formatReasoningEffort(log.upstream_reasoning_effort)}`
              : formatReasoningEffort(log.reasoning_effort || log.upstream_reasoning_effort)"
          />
          <DetailField
            :label="t('usage.type')"
            :value="log.native_compaction_v2 ? `${requestTypeLabel(log, t)} · ${t('usage.nativeCompactionV2')}` : requestTypeLabel(log, t)"
          />
          <DetailField :label="t('usage.inboundEndpoint')" :value="log.inbound_endpoint?.trim()" />
          <DetailField :label="t('usage.upstreamEndpoint')" :value="log.upstream_endpoint?.trim()" />
          <DetailField v-for="id in idFields" :key="id.key" :label="id.label">
            <span v-if="id.value" class="inline-flex max-w-full items-center gap-1.5">
              <span class="break-all font-mono text-xs">{{ id.value }}</span>
              <button
                type="button"
                class="shrink-0 rounded p-0.5 text-af-ink-4 transition-colors hover:bg-af-sunken hover:text-af-ink-2"
                :title="t('keys.copyToClipboard')"
                :aria-label="t('keys.copyToClipboard')"
                :data-testid="`usage-detail-copy-${id.key}`"
                @click="copyToClipboard(id.value)"
              >
                <Icon name="copy" size="sm" />
              </button>
            </span>
            <template v-else>—</template>
          </DetailField>
          <DetailField :label="t('admin.usage.ipAddress')">
            <template v-if="log.ip_address">
              <span class="font-mono">{{ log.ip_address }}</span>
              <IpGeoCell :ip="log.ip_address" />
            </template>
            <template v-else>—</template>
          </DetailField>
          <DetailField :label="t('usage.userAgent')" :value="log.user_agent" />
        </dl>
      </section>

      <!-- Token -->
      <section data-testid="usage-detail-tokens">
        <h3 class="text-13 font-medium text-af-ink-3">{{ t('admin.usage.detail.sections.tokens') }}</h3>
        <dl class="divide-y divide-af-hairline">
          <DetailField v-for="line in tokenLines" :key="line.key" :label="line.label" :value="line.value" />
        </dl>
      </section>

      <!-- 耗时 -->
      <section data-testid="usage-detail-timing">
        <h3 class="text-13 font-medium text-af-ink-3">{{ t('admin.usage.detail.sections.timing') }}</h3>
        <dl class="divide-y divide-af-hairline">
          <DetailField :label="t('usage.latencyFirstToken')" :value="log.first_token_ms != null ? formatDurationMs(log.first_token_ms) : null" />
          <DetailField :label="t('usage.latencyDuration')" :value="log.duration_ms != null ? formatDurationMs(log.duration_ms) : null" />
        </dl>
      </section>
    </div>
  </DetailDrawer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { DetailDrawer, DetailField } from '@/components/admin/list'
import IpGeoCell from '@/components/common/IpGeoCell.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { formatDateTime, formatReasoningEffort } from '@/utils/format'
import { formatMultiplier } from '@/utils/formatters'
import { formatMoneyExact, profitOf, profitTextClass } from '@/utils/money'
import { getBillingModeLabel, getDisplayBillingMode, isImageUsage } from '@/utils/billingMode'
import { formatImageBillingSize, textInputTokens, textOutputTokens } from '@/utils/imageUsage'
import { getUsageServiceTierLabel } from '@/utils/usageServiceTier'
import {
  formatDurationMs,
  hasReasoningEffortMapping,
  isLikelyModelVariant,
  requestTypeLabel,
  rowAccountCost,
  sentUpstreamModel,
  totalTokens
} from '@/components/usage/usageRow'
import type { AdminUsageLog } from '@/types'

const props = defineProps<{ log: AdminUsageLog | null }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

/** 请求 ID / 上游 ID：等宽全文 + 复制按钮。 */
const idFields = computed(() => [
  {
    key: 'request_id',
    label: t('admin.usage.requestId'),
    value: props.log?.request_id || ''
  },
  {
    key: 'upstream_request_id',
    label: t('admin.usage.upstreamRequestId'),
    value: props.log?.upstream_request_id || ''
  }
])

const moneyItems = computed(() => {
  const log = props.log
  if (!log) return []
  const revenue = log.actual_cost ?? 0
  const cost = rowAccountCost(log)
  const profit = profitOf(revenue, cost)
  return [
    { key: 'revenue', label: t('common.money.revenue'), hint: t('common.money.revenueHint'), value: revenue, valueClass: 'text-af-ink' },
    { key: 'cost', label: t('common.money.cost'), hint: t('common.money.costHint'), value: cost, valueClass: 'text-af-ink' },
    { key: 'profit', label: t('common.money.profit'), hint: t('common.money.profitHint'), value: profit, valueClass: profitTextClass(profit) || 'text-af-ink' }
  ]
})

const tokenLines = computed(() => {
  const log = props.log
  if (!log) return []
  // input / output_tokens 已含图片 Token，下面图片 Token 单列，这里只写文字部分，各行加起来才等于合计
  const lines: Array<{ key: string; label: string; value: string }> = [
    { key: 'input', label: t('admin.usage.inputTokens'), value: textInputTokens(log).toLocaleString() },
    { key: 'output', label: t('admin.usage.outputTokens'), value: textOutputTokens(log).toLocaleString() }
  ]
  if (log.image_input_tokens > 0) {
    lines.push({ key: 'imageInput', label: t('usage.imageInputTokens'), value: log.image_input_tokens.toLocaleString() })
  }
  if (log.image_output_tokens > 0) {
    lines.push({ key: 'imageOutput', label: t('usage.imageOutputTokens'), value: log.image_output_tokens.toLocaleString() })
  }
  if (log.cache_read_tokens > 0) {
    lines.push({ key: 'cacheRead', label: t('admin.usage.cacheReadTokens'), value: log.cache_read_tokens.toLocaleString() })
  }
  if (log.cache_creation_5m_tokens > 0 || log.cache_creation_1h_tokens > 0) {
    if (log.cache_creation_5m_tokens > 0) {
      lines.push({ key: 'cacheCreation5m', label: t('admin.usage.detail.cacheCreation5m'), value: log.cache_creation_5m_tokens.toLocaleString() })
    }
    if (log.cache_creation_1h_tokens > 0) {
      lines.push({ key: 'cacheCreation1h', label: t('admin.usage.detail.cacheCreation1h'), value: log.cache_creation_1h_tokens.toLocaleString() })
    }
  } else if (log.cache_creation_tokens > 0) {
    lines.push({ key: 'cacheCreation', label: t('admin.usage.cacheCreationTokens'), value: log.cache_creation_tokens.toLocaleString() })
  }
  if (log.cache_ttl_overridden) {
    lines.push({
      key: 'cacheTtl',
      label: t('admin.usage.detail.cacheTtlOverridden'),
      value: log.cache_creation_1h_tokens > 0 ? t('admin.usage.detail.ttlBilled1h') : t('admin.usage.detail.ttlBilled5m')
    })
  }
  if (isImageUsage(log)) {
    lines.push({
      key: 'images',
      label: t('admin.usage.detail.images'),
      value: t('admin.usage.detail.imagesValue', { count: log.image_count, size: formatImageBillingSize(log, t) })
    })
  }
  lines.push({ key: 'total', label: t('usage.totalTokens'), value: totalTokens(log).toLocaleString() })
  return lines
})
</script>
