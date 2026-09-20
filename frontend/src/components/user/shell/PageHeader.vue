<template>
  <!-- 页标题行：标题 + 说明 + 右侧至多一个主操作；下方可选页签；底 hairline -->
  <div class="border-b border-af-hairline" :class="$slots.tabs ? 'pb-0' : 'pb-4'">
    <div class="flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
      <div class="min-w-0">
        <h1 class="text-xl font-semibold leading-7 text-af-ink">{{ title }}</h1>
        <p v-if="description" class="mt-1 text-13 text-af-ink-3">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="flex shrink-0 items-center gap-2">
        <slot name="actions" />
      </div>
    </div>
    <div v-if="$slots.tabs" class="mt-3">
      <slot name="tabs" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>
