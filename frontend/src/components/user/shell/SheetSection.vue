<template>
  <!-- 页内区块：可选标题行 + 右侧控件；兄弟区块之间只用一条 hairline 分隔，不做卡片 -->
  <section class="border-t border-af-hairline pt-6 first:border-t-0 first:pt-0" :aria-labelledby="title ? headingId : undefined">
    <div v-if="title || $slots.actions" class="mb-4 flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
      <div class="min-w-0">
        <h2 v-if="title" :id="headingId" class="text-base font-semibold text-af-ink">{{ title }}</h2>
        <p v-if="description" class="mt-0.5 text-13 text-af-ink-3">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="flex shrink-0 items-center gap-2">
        <slot name="actions" />
      </div>
    </div>
    <slot />
  </section>
</template>

<script setup lang="ts">
let counter = 0
defineProps<{ title?: string; description?: string }>()
const headingId = `sheet-section-${++counter}`
</script>
