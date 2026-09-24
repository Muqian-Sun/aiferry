<template>
  <!-- 页内下划线页签。给 to 时走路由（aria-current），否则由 modelValue 控制（aria-selected）。 -->
  <div class="flex gap-1 overflow-x-auto border-b border-af-hairline scrollbar-hide" role="tablist" :aria-label="label">
    <template v-for="tab in tabs" :key="tab.key">
      <RouterLink
        v-if="tab.to"
        :to="tab.to"
        :class="tabClass(isActive(tab))"
        :aria-current="isActive(tab) ? 'page' : undefined"
        :data-testid="`section-tab-${tab.key}`"
      >
        {{ tab.label }}
        <span v-if="tab.count !== undefined" class="tabular-nums text-af-ink-4">{{ tab.count }}</span>
      </RouterLink>
      <button
        v-else
        type="button"
        role="tab"
        :class="tabClass(isActive(tab))"
        :aria-selected="isActive(tab)"
        :data-testid="`section-tab-${tab.key}`"
        @click="emit('update:modelValue', tab.key)"
      >
        {{ tab.label }}
        <span v-if="tab.count !== undefined" class="tabular-nums text-af-ink-4">{{ tab.count }}</span>
      </button>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import type { SectionTab } from './types'

const props = defineProps<{ tabs: SectionTab[]; modelValue?: string; label?: string }>()
const emit = defineEmits<{ 'update:modelValue': [key: string] }>()
const route = useRoute()

const matchesRoute = (to: string) => route.path === to || route.path.startsWith(`${to}/`)

/** 带路由的页签：匹配得最长的那个才算当前（同时有 /orders 与 /orders/dashboard 时只亮后者） */
const activeTo = computed(() =>
  props.tabs
    .filter((tab) => tab.to && matchesRoute(tab.to))
    .reduce<string | null>((best, tab) => (!best || tab.to!.length > best.length ? tab.to! : best), null)
)

function isActive(tab: SectionTab): boolean {
  if (tab.to) return tab.to === activeTo.value
  return props.modelValue === tab.key
}

function tabClass(active: boolean): string {
  return [
    'relative -mb-px inline-flex h-10 shrink-0 items-center gap-1.5 whitespace-nowrap px-3 text-sm font-medium transition-colors',
    'border-b-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-af-brand/40',
    active ? 'border-af-brand text-af-ink' : 'border-transparent text-af-ink-2 hover:text-af-ink'
  ].join(' ')
}
</script>
