<template>
  <!--
    控制台左侧分组栏（muqian 2026-09-23 定「左侧栏」）：主组无标题，账务 / 账户 / 更多各带小标题。
    整列一条右侧细线与内容隔开，不做底色、不做卡片；当前页浅底墨字，与顶栏页签同一套。
    随页面滚动吸顶（顶栏下方），条目多时自己滚。窄屏不渲染（由顶栏第二行替代）。
  -->
  <aside v-if="sections.length" class="w-52 shrink-0 border-r border-af-hairline" data-testid="console-sidebar">
    <nav
      class="sticky top-topbar max-h-[calc(100svh_-_var(--af-topbar-h))] space-y-7 overflow-y-auto py-8 pr-5 scrollbar-hide"
      :aria-label="t('userUi.nav.primaryNav')"
    >
      <div v-for="section in sections" :key="section.key" :data-testid="`sidebar-section-${section.key}`">
        <p v-if="section.label" class="mb-2 text-xs font-medium text-af-ink-4">{{ section.label }}</p>
        <ul class="-ml-3 space-y-0.5">
          <li v-for="item in section.items" :key="item.path">
            <RouterLink
              :to="item.path"
              :class="linkClass(isTabActive(item, currentPath))"
              :aria-current="isTabActive(item, currentPath) ? 'page' : undefined"
              :data-tour="item.dataTour"
            >
              <span v-if="item.iconSvg" class="h-4 w-4 shrink-0 [&>svg]:h-4 [&>svg]:w-4" v-html="sanitizeSvg(item.iconSvg)"></span>
              <span class="truncate">{{ item.label }}</span>
            </RouterLink>
          </li>
        </ul>
      </div>
    </nav>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { sanitizeSvg } from '@/utils/sanitize'
import { isTabActive } from './navItems'
import { useConsoleNav } from './useConsoleNav'

const { t } = useI18n()
const route = useRoute()
const currentPath = computed(() => route.path)
const { sections } = useConsoleNav()

function linkClass(active: boolean): string {
  return [
    'flex h-9 items-center gap-2 rounded-md px-3 text-sm transition-colors',
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-af-brand/40',
    active ? 'bg-af-sunken font-medium text-af-ink' : 'text-af-ink-2 hover:bg-af-sunken/70 hover:text-af-ink'
  ].join(' ')
}
</script>
