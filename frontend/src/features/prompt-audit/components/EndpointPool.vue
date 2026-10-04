<template>
  <!--
    守卫节点（2026-10-05）：节点写在部署配置里（prompt_audit.guard_endpoints / PROMPT_AUDIT_GUARD_ENDPOINTS），
    这里只读、按顺序列出，可逐个探测；改节点要改配置并重启。探测结果只在这里显示一次。
  -->
  <section aria-labelledby="prompt-pool-title" class="border-b border-af-hairline py-6">
    <div>
      <h2 id="prompt-pool-title" class="text-base font-semibold text-af-ink">{{ t('admin.promptAudit.pool.title') }}</h2>
      <p class="mt-1 text-sm text-af-ink-3">{{ t('admin.promptAudit.pool.description') }}</p>
    </div>

    <p v-if="endpoints.length === 0" class="mt-4 text-sm text-af-ink-3" data-test="endpoint-pool-empty">
      {{ t('admin.promptAudit.pool.empty') }}
    </p>
    <ul v-else class="mt-4 divide-y divide-af-hairline border-y border-af-hairline">
      <li
        v-for="(endpoint, index) in endpoints"
        :key="endpoint.id"
        :data-test="`endpoint-${endpoint.id}`"
        class="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between sm:gap-6"
      >
        <div class="min-w-0">
          <div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-1">
            <span class="text-xs tabular-nums text-af-ink-3">{{ index + 1 }}</span>
            <p class="truncate text-sm font-medium text-af-ink">{{ endpoint.name }}</p>
            <span class="text-xs text-af-ink-3">{{ endpoint.has_token ? t('admin.promptAudit.pool.configured') : t('admin.promptAudit.pool.missing') }}</span>
          </div>
          <p class="mt-0.5 truncate font-mono text-xs text-af-ink-3" :title="`${endpoint.base_url} · ${endpoint.model}`">
            {{ endpoint.base_url }} · {{ endpoint.model }}
          </p>
          <p v-if="probingIds.includes(endpoint.id)" class="mt-1 text-xs text-af-ink-3">{{ t('admin.promptAudit.pool.probing') }}</p>
          <p
            v-else-if="probeResults[endpoint.id]"
            class="mt-1 inline-flex items-center gap-1.5 text-xs"
            :class="probeResults[endpoint.id].ok ? 'text-af-ink-2' : 'text-af-danger'"
            data-test="endpoint-probe-result"
          >
            <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="probeResults[endpoint.id].ok ? 'bg-af-success' : 'bg-af-danger'" aria-hidden="true" />
            {{ probeText(probeResults[endpoint.id]) }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm shrink-0 self-start sm:self-center"
          :disabled="probingIds.includes(endpoint.id)"
          :aria-label="t('admin.promptAudit.pool.probeNode', { name: endpoint.name })"
          @click="$emit('probe', endpoint.id)"
        >
          {{ t('admin.promptAudit.pool.probe') }}
        </button>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { PromptAuditEndpoint, PromptProbeResult } from '../types'
import { guardErrorText } from '../viewModel'

defineProps<{
  endpoints: PromptAuditEndpoint[]
  /** 节点 id → 最近一次探测结果（服务端记下的 + 本页刚测的） */
  probeResults: Record<string, PromptProbeResult>
  probingIds: string[]
}>()
defineEmits<{ (event: 'probe', endpointId: string): void }>()
const { t, locale } = useI18n()

function probeText(result: PromptProbeResult): string {
  const parts: string[] = []
  if (result.ok) {
    parts.push(t('admin.promptAudit.pool.probeOk'))
  } else {
    parts.push(guardErrorText(t, result.error_code || ''))
    if (result.http_status > 0) parts.push(`HTTP ${result.http_status}`)
  }
  parts.push(`${result.latency_ms} ms`)
  if (result.checked_at) {
    parts.push(new Intl.DateTimeFormat(locale.value, { timeStyle: 'medium' }).format(new Date(result.checked_at)))
  }
  return parts.join(' · ')
}
</script>
