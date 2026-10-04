<script setup lang="ts">
/**
 * 运维页「首字最慢的请求」（2026-10-04 重排）：排查变慢时直接看是哪个模型、哪个渠道慢；
 * 「查看全部」打开现有的请求明细（按首字延迟排序）。
 */
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { opsAPI, type OpsRequestDetail, type OpsRequestDetailsParams } from '@/api/admin/ops'
import { formatDurationMs } from '@/components/usage/usageRow'
import type { OpsChannelOption } from './OpsToolbar.vue'

const props = defineProps<{
  params: Pick<OpsRequestDetailsParams, 'time_range' | 'start_time' | 'end_time' | 'model' | 'account_id'>
  channels: OpsChannelOption[]
  refreshToken: number
}>()

const emit = defineEmits<{ (e: 'openAll'): void }>()

const { t } = useI18n()
const rows = ref<OpsRequestDetail[]>([])
const loading = ref(false)
const failed = ref(false)

let seq = 0
async function load() {
  const current = ++seq
  loading.value = true
  failed.value = false
  try {
    const res = await opsAPI.listRequestDetails({ ...props.params, kind: 'success', sort: 'ttft_desc', page: 1, page_size: 5 })
    if (current === seq) rows.value = (res.items || []).filter((r) => r.first_token_ms != null)
  } catch (err) {
    console.error('[OpsSlowRequests] failed to load', err)
    if (current === seq) failed.value = true
  } finally {
    if (current === seq) loading.value = false
  }
}

watch(() => props.refreshToken, load, { immediate: true })

function channelName(id?: number | null): string {
  if (!id) return '—'
  return props.channels.find((c) => c.id === id)?.name || t('admin.ops.page.channels.deleted')
}

function formatTime(value: string): string {
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString()
}

function ms(value?: number | null): string {
  return value == null ? '—' : formatDurationMs(value)
}
</script>

<template>
  <section class="border-t border-af-hairline py-4" data-testid="ops-slow-requests">
    <div class="mb-2 flex flex-wrap items-baseline justify-between gap-2">
      <h2 class="text-sm font-semibold text-af-ink">{{ t('admin.ops.page.slow.title') }}</h2>
      <button type="button" class="text-xs text-af-ink-3 hover:text-af-ink" @click="emit('openAll')">{{ t('admin.ops.page.viewAll') }} →</button>
    </div>
    <p v-if="failed" class="text-sm text-af-danger">{{ t('admin.ops.page.loadFailed') }}</p>
    <p v-else-if="!loading && !rows.length" class="text-sm text-af-ink-3">{{ t('admin.ops.page.noDataInRange') }}</p>
    <div v-else class="overflow-x-auto">
      <table class="w-full min-w-[560px] text-sm">
        <thead>
          <tr class="border-b border-af-hairline text-left text-xs text-af-ink-3">
            <th class="py-2 pr-3 font-medium">{{ t('admin.ops.page.failures.time') }}</th>
            <th class="py-2 pr-3 font-medium">{{ t('admin.ops.page.failures.model') }}</th>
            <th class="py-2 pr-3 font-medium">{{ t('admin.ops.page.failures.channel') }}</th>
            <th class="py-2 pr-3 text-right font-medium">{{ t('admin.ops.page.slow.ttft') }}</th>
            <th class="py-2 text-right font-medium">{{ t('admin.ops.page.slow.duration') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.request_id" class="border-b border-af-hairline">
            <td class="whitespace-nowrap py-2 pr-3 tabular-nums text-af-ink-2">{{ formatTime(row.created_at) }}</td>
            <td class="py-2 pr-3 font-mono text-xs text-af-ink">{{ row.model || '—' }}</td>
            <td class="py-2 pr-3 text-af-ink-2">{{ channelName(row.account_id) }}</td>
            <td class="py-2 pr-3 text-right tabular-nums text-af-ink">{{ ms(row.first_token_ms) }}</td>
            <td class="py-2 text-right tabular-nums text-af-ink-2">{{ ms(row.duration_ms) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
