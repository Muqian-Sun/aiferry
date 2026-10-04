<template>
  <!--
    控制台页头（与首页同一套字阶，muqian 2026-09-23）：28px 标题 + 一行说明，右侧放页面级操作；下方可选页签。
    不画底线——页头与第一块内容之间靠留白分开，内容区块之间才用 hairline。
  -->
  <div class="mb-8">
    <div class="flex flex-wrap items-end justify-between gap-x-6 gap-y-3">
      <div class="min-w-0">
        <h1 class="text-28 font-semibold leading-tight tracking-[-0.015em] text-af-ink">{{ title }}</h1>
        <p v-if="description" class="mt-1.5 text-sm text-af-ink-3">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="flex max-w-full shrink-0 flex-wrap items-center gap-2">
        <slot name="actions" />
      </div>
    </div>
    <div v-if="$slots.tabs" class="mt-6">
      <slot name="tabs" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>
