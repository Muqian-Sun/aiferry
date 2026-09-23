<template>
  <!-- 顶栏页签（公开站与控制台同一组）与窄屏第二行；管理员自定义页在控制台侧栏的「更多」组，不在这里 -->
  <nav class="flex h-full min-w-0 items-center gap-1" :aria-label="t('userUi.nav.primaryNav')">
    <template v-for="tab in tabs" :key="tab.path">
      <a
        v-if="tab.external"
        :href="tab.path"
        target="_blank"
        rel="noopener noreferrer"
        :class="tabClass(false)"
      >
        {{ tab.label }}
      </a>
      <RouterLink
        v-else
        :to="tab.path"
        :class="tabClass(tab.path === activePath)"
        :aria-current="tab.path === activePath ? 'page' : undefined"
        :data-tour="tab.dataTour"
      >
        <span v-if="tab.iconSvg" class="h-4 w-4 shrink-0 [&>svg]:h-4 [&>svg]:w-4" v-html="sanitizeSvg(tab.iconSvg)"></span>
        {{ tab.label }}
      </RouterLink>
    </template>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { sanitizeSvg } from '@/utils/sanitize'
import { pickActivePath, type NavTab } from './navItems'

const props = defineProps<{ tabs: NavTab[] }>()

const { t } = useI18n()
const route = useRoute()
const currentPath = computed(() => route.path)
const activePath = computed(() => pickActivePath(props.tabs, currentPath.value))

/* 页签：胶囊式，当前页一块浅底 + 墨色字，其余悬停出浅底 */
function tabClass(active: boolean): string {
  return [
    'inline-flex h-9 shrink-0 items-center gap-1 whitespace-nowrap rounded-full px-3.5 text-sm font-medium transition-colors',
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-af-brand/40',
    active ? 'bg-af-sunken text-af-ink' : 'text-af-ink-2 hover:bg-af-sunken/70 hover:text-af-ink'
  ].join(' ')
}
</script>
