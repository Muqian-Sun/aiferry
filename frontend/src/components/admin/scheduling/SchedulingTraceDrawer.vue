<template>
  <DetailDrawer
    :show="!!trace"
    :title="trace?.model || '-'"
    :eyebrow="trace ? new Date(trace.created_at).toLocaleString() : ''"
    :subtitle="trace ? subtitle(trace) : ''"
    @close="emit('close')"
  >
    <div v-if="trace" class="space-y-8 text-13" data-testid="scheduling-trace-drawer">
      <dl class="grid grid-cols-2 gap-x-6 gap-y-3 text-xs">
        <div><dt class="text-af-ink-3">{{ t('admin.scheduling.trace.fields.outcome') }}</dt><dd class="mt-0.5 text-af-ink">{{ labelOr('admin.scheduling.trace.outcomes', trace.outcome) }} · {{ trace.status_code || '-' }}</dd></div>
        <div><dt class="text-af-ink-3">{{ t('admin.scheduling.trace.fields.duration') }}</dt><dd class="mt-0.5 tabular-nums text-af-ink">{{ formatMs(trace.duration_ms) }}</dd></div>
        <div><dt class="text-af-ink-3">{{ t('admin.scheduling.trace.fields.finalChannel') }}</dt><dd class="mt-0.5 text-af-ink">{{ channelName(trace.final_account_id) }}</dd></div>
        <div><dt class="text-af-ink-3">{{ t('admin.scheduling.trace.fields.switches') }}</dt><dd class="mt-0.5 tabular-nums text-af-ink">{{ trace.switch_count }}</dd></div>
        <div class="col-span-2"><dt class="text-af-ink-3">{{ t('admin.scheduling.trace.fields.requestId') }}</dt><dd class="mt-0.5 break-all font-mono text-af-ink">{{ trace.request_id || '-' }}</dd></div>
      </dl>

      <section v-for="(pick, index) in trace.picks" :key="index" data-testid="scheduling-trace-pick">
        <h3 class="text-13 font-medium text-af-ink">
          {{ t('admin.scheduling.trace.pickTitle', { n: index + 1, at: formatMs(pick.at_ms) }) }}
          <span class="ml-1 font-normal text-af-ink-3">
            {{ pick.account_id
              ? t('admin.scheduling.trace.pickResult', { channel: channelName(pick.account_id), layer: labelOr('admin.scheduling.trace.layers', pick.layer) })
              : labelOr('admin.scheduling.trace.errors', pick.error) }}
          </span>
        </h3>
        <p v-if="pick.sticky" class="mt-1 text-xs text-af-ink-3">{{ stickyText(pick) }}</p>
        <table v-if="pick.candidates.length" class="mt-3 w-full text-xs">
          <thead class="text-left text-af-ink-3">
            <tr>
              <th class="pb-1.5 font-normal">{{ t('admin.scheduling.trace.columns.channel') }}</th>
              <th class="pb-1.5 font-normal">{{ t('admin.scheduling.trace.columns.decision') }}</th>
              <th class="pb-1.5 text-right font-normal">{{ t('admin.scheduling.trace.columns.load') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="candidate in pick.candidates" :key="candidate.account_id" class="border-t border-af-hairline" :data-decision="candidate.decision">
              <td class="py-1.5 pr-3 text-af-ink">{{ channelName(candidate.account_id) }}</td>
              <td class="py-1.5 pr-3" :class="candidate.decision === 'picked' || candidate.decision === 'queued' ? 'font-medium text-af-ink' : 'text-af-ink-3'">
                {{ labelOr('admin.scheduling.trace.decisions', candidate.decision) }}<template v-if="candidate.reason">（{{ reasonLabel(candidate.reason) }}）</template>
              </td>
              <td class="py-1.5 text-right tabular-nums text-af-ink-3">{{ candidate.load_rate != null ? `${candidate.load_rate}%` : '—' }}</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section v-if="trace.attempts.length" data-testid="scheduling-trace-attempts">
        <h3 class="text-13 font-medium text-af-ink">{{ t('admin.scheduling.trace.attemptsTitle') }}</h3>
        <ul class="mt-2 space-y-2 text-xs">
          <li v-for="(attempt, index) in trace.attempts" :key="index" class="text-af-ink-3">
            <span class="tabular-nums">+{{ formatMs(attempt.at_ms) }}</span>
            · <span class="text-af-ink">{{ channelName(attempt.account_id) }}</span>
            · {{ attempt.status_code || '-' }}
            <span v-if="attempt.kind"> · {{ attempt.kind }}</span>
            <p v-if="attempt.message" class="mt-0.5 break-all">{{ attempt.message }}</p>
          </li>
        </ul>
      </section>
    </div>
  </DetailDrawer>
</template>

<script setup lang="ts">
import { toRef } from 'vue'
import type { SchedulingOverview, SchedulingTrace, SchedulingTracePick } from '@/api/admin/scheduling'
import DetailDrawer from '@/components/common/DetailDrawer.vue'
import { useSchedulingLabels } from './useSchedulingLabels'

const props = defineProps<{ trace: SchedulingTrace | null; overview: SchedulingOverview | null }>()
const emit = defineEmits<{ close: [] }>()
const { t, channelName, reasonLabel, labelOr, protocolLabel } = useSchedulingLabels(toRef(props, 'overview'))

function formatMs(ms: number): string {
  return ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`
}

function subtitle(trace: SchedulingTrace): string {
  const parts = [protocolLabel(trace.inbound_protocol)]
  if (trace.user_id) parts.push(t('admin.scheduling.trace.user', { id: trace.user_id }))
  if (trace.api_key_id) parts.push(t('admin.scheduling.trace.key', { id: trace.api_key_id }))
  return parts.join(' · ')
}

function stickyText(pick: SchedulingTracePick): string {
  const sticky = pick.sticky!
  const channel = channelName(sticky.account_id)
  if (sticky.outcome === 'miss') return t('admin.scheduling.trace.sticky.miss', { channel, reason: reasonLabel(sticky.reason) })
  return t(`admin.scheduling.trace.sticky.${sticky.outcome === 'wait' ? 'wait' : 'hit'}`, { channel })
}
</script>
