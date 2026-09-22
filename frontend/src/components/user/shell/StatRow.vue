<template>
  <!--
    指标行：行内数字，竖 hairline 分隔，不做瓷砖。
    lg 以上一行 N 列；以下两列网格、行间 hairline。数字用 tabular-nums 对齐。
  -->
  <dl class="grid grid-cols-2 gap-y-4 lg:flex lg:divide-x lg:divide-af-hairline" data-testid="stat-row">
    <div
      v-for="(item, index) in items"
      :key="item.key ?? item.label"
      class="min-w-0 pr-6 lg:flex-1 lg:px-6 lg:first:pl-0 lg:last:pr-0"
      :class="index >= 2 ? 'border-t border-af-hairline pt-4 lg:border-t-0 lg:pt-0' : ''"
      :data-testid="item.key ? `stat-${item.key}` : undefined"
    >
      <dt class="truncate text-13 text-af-ink-3">{{ item.label }}</dt>
      <dd class="mt-1 flex items-baseline gap-2">
        <span class="text-xl font-semibold tabular-nums text-af-ink">{{ item.value }}</span>
        <span v-if="item.hint" class="truncate text-xs text-af-ink-3">{{ item.hint }}</span>
        <RouterLink v-if="item.link" :to="item.link.to" class="text-xs font-medium text-af-brand hover:text-af-brand-hover">
          {{ item.link.label }}
        </RouterLink>
      </dd>
    </div>
  </dl>
</template>

<script setup lang="ts">
import type { StatItem } from './types'

defineProps<{ items: StatItem[] }>()
</script>
