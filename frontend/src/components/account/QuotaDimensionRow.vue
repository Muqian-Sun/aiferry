<script setup lang="ts">
import { useI18n } from 'vue-i18n'

// 限额一律滚动窗口、提醒写死用到 80%（2026-09-28 P5），这里只填限额。
const { t } = useI18n()

defineProps<{
  label: string
  limit: number | null
  hint: string
}>()

const emit = defineEmits<{
  'update:limit': [value: number | null]
}>()

const onLimitInput = (e: Event) => {
  const raw = (e.target as HTMLInputElement).valueAsNumber
  emit('update:limit', Number.isNaN(raw) ? null : raw)
}
</script>

<template>
  <div>
    <label class="text-xs font-medium text-af-ink-2 mb-1 block">{{ label }}</label>
    <div class="relative">
      <span class="absolute left-2.5 top-1/2 -translate-y-1/2 text-af-ink-3 text-sm">$</span>
      <input :value="limit" @input="onLimitInput" type="number" min="0" step="0.01" class="input pl-6 py-1.5 text-sm" :placeholder="t('admin.accounts.quotaLimitPlaceholder')" />
    </div>
    <p class="input-hint mb-0 text-[11px]">{{ hint }}</p>
  </div>
</template>
