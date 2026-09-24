<template>
  <div
    class="stat-card !min-h-[6.5rem] !rounded-lg !border-0 !p-4 ring-1 ring-af-hairline"
    :title="title || undefined"
  >
    <div
      v-if="resolvedState"
      class="mt-1 h-2 w-2 shrink-0 rounded-full"
      :class="dotClass"
      aria-hidden="true"
    ></div>
    <div class="min-w-0 flex-1">
      <span class="stat-label text-[10px] font-bold uppercase tracking-wider text-af-ink-3">{{ label }}</span>
      <strong
        class="stat-value mt-1 block overflow-visible text-xl tabular-nums leading-tight !text-clip !whitespace-normal"
        :class="stateClass"
      >{{ value }}</strong>
      <div
        v-if="detailParts.length > 1"
        class="mt-1.5 flex flex-wrap gap-x-2 gap-y-0.5 text-[11px] leading-snug text-af-ink-3"
      >
        <span
          v-for="(part, index) in detailParts"
          :key="`${index}:${part}`"
          class="whitespace-nowrap tabular-nums"
        >{{ part }}</span>
      </div>
      <small
        v-else-if="detail"
        class="mt-1.5 block text-[11px] leading-snug text-af-ink-3"
      >{{ detail }}</small>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { HealthState } from '@/api/channelMonitorV2'

const props = defineProps<{
  label: string
  value: string
  detail: string
  state?: HealthState
  /** Exact numeric tooltip (e.g. uncompacted RPM/TPM). */
  title?: string
}>()

/** Split "AVG 475ms · P90 800ms" into chips so nothing is ellipsized. */
const detailParts = computed(() => {
  const raw = (props.detail || '').trim()
  if (!raw || raw === '-') return []
  return raw
    .split(/\s*[·|]\s*/)
    .map((part) => part.trim())
    .filter(Boolean)
})

const missingValue = computed(() => {
  const value = (props.value || '').trim()
  return value === '' || value === '-' || value === '—'
})

const resolvedState = computed(() => (missingValue.value ? undefined : props.state))

const stateClass = computed(() => {
  if (!resolvedState.value) return missingValue.value ? 'text-af-ink-3' : 'text-af-ink'
  if (resolvedState.value === 'healthy') return 'text-af-success'
  if (resolvedState.value === 'warning') return 'text-af-warning'
  if (resolvedState.value === 'critical') return 'text-af-danger'
  return 'text-af-ink-3'
})

const dotClass = computed(() => {
  if (resolvedState.value === 'healthy') return 'bg-af-success'
  if (resolvedState.value === 'warning') return 'bg-af-warning'
  if (resolvedState.value === 'critical') return 'bg-af-danger'
  return 'bg-af-ink-4'
})
</script>
