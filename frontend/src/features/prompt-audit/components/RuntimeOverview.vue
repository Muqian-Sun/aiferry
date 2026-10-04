<template>
  <!-- 运行概览（2026-10-05）：数字用数字行，不做瓷砖；节点探测结果只在下方「守卫节点」里显示 -->
  <section aria-labelledby="prompt-runtime-title" class="border-b border-af-hairline py-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 id="prompt-runtime-title" class="text-base font-semibold text-af-ink">
          {{ t('admin.promptAudit.runtime.title') }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t('admin.promptAudit.runtime.description') }}
        </p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="$emit('refresh')">
        {{ t('admin.promptAudit.actions.refresh') }}
      </button>
    </div>

    <FormError v-if="error" class="mt-4" :message="error" />
    <p v-else-if="loading && !runtime" class="mt-4 text-sm text-af-ink-3" aria-busy="true">{{ t('common.loading') }}</p>
    <div v-else-if="runtime" class="mt-5 space-y-6">
      <StatRow :items="statusItems" data-test="prompt-runtime-status" />

      <div>
        <h3 class="mb-3 text-sm font-medium text-af-ink">{{ t('admin.promptAudit.runtime.guardMetrics') }}</h3>
        <StatRow :items="guardMetricItems" data-test="prompt-runtime-metrics" />
      </div>

      <div class="space-y-1 text-xs leading-5 text-af-ink-3">
        <p>
          {{ t('admin.promptAudit.runtime.queueBreakdown', {
            queued: runtime.queue.queued,
            processing: runtime.queue.processing,
            retry: runtime.queue.retry,
            done: runtime.queue.done,
            failed: runtime.queue.failed,
          }) }}
          <span class="mx-1.5">·</span>
          {{ t('admin.promptAudit.runtime.deliveryTotals', { enqueued: runtime.enqueued_total, dropped: runtime.dropped_total, processed: runtime.processed_total, failed: runtime.failed_total }) }}
        </p>
        <p>
          {{ t('admin.promptAudit.runtime.lastProcessed', { time: runtime.last_processed_at ? formatDate(runtime.last_processed_at) : t('admin.promptAudit.common.never') }) }}
          <span v-if="runtime.last_error_code" class="text-af-danger" data-test="prompt-runtime-last-error">
            <span class="mx-1.5 text-af-ink-3">·</span>{{ guardErrorText(t, runtime.last_error_code) }}
          </span>
        </p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import FormError from '@/components/common/FormError.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import type { PromptAuditRuntime } from '../types'
import { guardErrorText } from '../viewModel'

const props = defineProps<{
  runtime: PromptAuditRuntime | null
  loading: boolean
  error: string
}>()
defineEmits<{ (event: 'refresh'): void }>()
const { t, locale } = useI18n()

const statusItems = computed<StatItem[]>(() => {
  const runtime = props.runtime
  if (!runtime) return []
  return [
    { key: 'process', label: t('admin.promptAudit.runtime.process'), value: t(`admin.promptAudit.status.${runtime.process_status}`), valueClass: statusClass(runtime.process_status) },
    { key: 'mode', label: t('admin.promptAudit.runtime.mode'), value: t(`admin.promptAudit.mode.${runtime.effective_mode}`) },
    { key: 'workers', label: t('admin.promptAudit.runtime.workers'), value: `${runtime.worker_active} / ${runtime.worker_total}` },
    { key: 'queue', label: t('admin.promptAudit.runtime.queue'), value: `${runtime.queue.active} / ${runtime.queue_capacity}` },
    { key: 'version', label: t('admin.promptAudit.runtime.version'), value: `${runtime.active_config_version} / ${runtime.expected_config_version}` },
    { key: 'dependencies', label: t('admin.promptAudit.runtime.dependencies'), value: `DB ${runtime.database_status} · Redis ${runtime.redis_status}` },
  ]
})

const guardMetricItems = computed<StatItem[]>(() => {
  const metrics = props.runtime?.guard_metrics
  if (!metrics) return []
  return [
    { key: 'total', label: t('admin.promptAudit.metrics.total'), value: String(metrics.total) },
    { key: 'allowed', label: t('admin.promptAudit.metrics.allowed'), value: String(metrics.allowed) },
    { key: 'flagged', label: t('admin.promptAudit.metrics.flagged'), value: String(metrics.flagged) },
    { key: 'blocked', label: t('admin.promptAudit.metrics.blocked'), value: String(metrics.blocked) },
    { key: 'unavailable', label: t('admin.promptAudit.metrics.unavailable'), value: String(metrics.unavailable) },
    { key: 'timeouts', label: t('admin.promptAudit.metrics.timeouts'), value: String(metrics.timeouts) },
    { key: 'failovers', label: t('admin.promptAudit.metrics.failovers'), value: String(metrics.failovers) },
    { key: 'p95', label: 'P95', value: metrics.latency_p95_ms != null ? `${metrics.latency_p95_ms} ms` : '—' },
  ]
})

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
}

// 进程状态只在出问题时上色；运行中 / 未启用是常态，用正文色
function statusClass(status: string): string | undefined {
  if (status === 'degraded') return 'text-af-warning'
  if (status === 'error') return 'text-af-danger'
  return undefined
}
</script>
