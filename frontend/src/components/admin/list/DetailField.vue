<template>
  <!-- 抽屉里的一行「标签 · 值」。放在 <dl class="divide-y divide-af-hairline"> 里；值为空时显示「—」。 -->
  <div class="grid grid-cols-[7.5rem_minmax(0,1fr)] gap-x-4 py-2.5">
    <dt class="text-13 text-af-ink-3">{{ label }}</dt>
    <dd class="min-w-0 break-words text-sm text-af-ink">
      <slot>{{ value === undefined || value === null || value === '' ? '—' : value }}</slot>
    </dd>
  </div>
</template>

<script setup lang="ts">
defineProps<{ label: string; value?: string | number | null }>()
</script>
