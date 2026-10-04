<script setup lang="ts">
/**
 * 运维页「未恢复的告警」（2026-10-04 重排）：只列还在触发的告警，没有就整段不出现；
 * 历史告警事件挪进「告警规则」弹窗。
 */
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { opsAPI, type AlertEvent } from '@/api/admin/ops'

const props = defineProps<{ refreshToken: number }>()
const emit = defineEmits<{ (e: 'openAlertRules'): void }>()

const { t } = useI18n()
const events = ref<AlertEvent[]>([])

async function load() {
  try {
    events.value = await opsAPI.listAlertEvents({ status: 'firing', limit: 20 })
  } catch (err) {
    console.error('[OpsActiveAlerts] failed to load firing alerts', err)
    events.value = []
  }
}

watch(() => props.refreshToken, load)

function formatTime(value: string): string {
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString()
}

function formatValue(v?: number): string {
  return v == null ? '—' : Number.isInteger(v) ? String(v) : v.toFixed(2)
}
</script>

<template>
  <section v-if="events.length" class="border-t border-af-hairline py-4" data-testid="ops-active-alerts">
    <div class="mb-2 flex flex-wrap items-baseline justify-between gap-2">
      <h2 class="text-sm font-semibold text-af-danger">{{ t('admin.ops.page.alerts.title', { count: events.length }) }}</h2>
      <button type="button" class="text-xs text-af-ink-3 hover:text-af-ink" @click="emit('openAlertRules')">{{ t('admin.ops.page.alerts.history') }} →</button>
    </div>
    <div class="overflow-x-auto">
      <table class="w-full min-w-[520px] text-sm">
        <thead>
          <tr class="border-b border-af-hairline text-left text-xs text-af-ink-3">
            <th class="py-2 pr-4 font-medium">{{ t('admin.ops.page.alerts.firedAt') }}</th>
            <th class="py-2 pr-4 font-medium">{{ t('admin.ops.page.alerts.name') }}</th>
            <th class="py-2 pr-4 text-right font-medium">{{ t('admin.ops.page.alerts.value') }}</th>
            <th class="py-2 text-right font-medium">{{ t('admin.ops.page.alerts.threshold') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="ev in events" :key="ev.id" class="border-b border-af-hairline">
            <td class="py-2 pr-4 tabular-nums text-af-ink-2">{{ formatTime(ev.fired_at) }}</td>
            <td class="py-2 pr-4 text-af-ink">{{ ev.title || ev.description || '—' }}</td>
            <td class="py-2 pr-4 text-right tabular-nums text-af-danger">{{ formatValue(ev.metric_value) }}</td>
            <td class="py-2 text-right tabular-nums text-af-ink-2">{{ formatValue(ev.threshold_value) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
