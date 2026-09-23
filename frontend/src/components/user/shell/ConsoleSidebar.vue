<template>
  <!--
    控制台左侧分组栏（muqian 2026-09-23 定「左侧栏」）：主组无标题，账务 / 账户 / 更多各带小标题。
    整列一条右侧细线与内容隔开，不做底色、不做卡片、不做高亮色块（灰底块被否「丑死了」）：
    条目是纯文字，当前页墨色加粗 + 左侧一道 2px 墨色短竖线（与顶栏 logo 左缘对齐），其余淡墨、悬停变深。
    随页面滚动吸顶（顶栏下方），条目多时自己滚——nav 是 overflow 容器，所以条目不能用负外边距往外探（会被裁掉）。
  -->
  <aside v-if="sections.length" class="w-52 shrink-0 border-r border-af-hairline" data-testid="console-sidebar">
    <nav
      class="sticky top-topbar max-h-[calc(100svh_-_var(--af-topbar-h))] space-y-7 overflow-y-auto py-8 pr-6 scrollbar-hide"
      :aria-label="t('userUi.nav.primaryNav')"
    >
      <div v-for="section in sections" :key="section.key" :data-testid="`sidebar-section-${section.key}`">
        <p v-if="section.label" class="mb-1.5 pl-3.5 text-xs text-af-ink-4">{{ section.label }}</p>
        <ul class="space-y-0.5">
          <li v-for="item in section.items" :key="item.path">
            <RouterLink
              :to="item.path"
              :class="linkClass(item.path === activePath)"
              :aria-current="item.path === activePath ? 'page' : undefined"
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
import { pickActivePath } from './navItems'
import { useConsoleNav } from './useConsoleNav'

const { t } = useI18n()
const route = useRoute()
const currentPath = computed(() => route.path)
const { sections, flatItems } = useConsoleNav()
/** 整栏只亮一项：所有条目里最长的匹配路径 */
const activePath = computed(() => pickActivePath(flatItems.value, currentPath.value))

function linkClass(active: boolean): string {
  return [
    'relative flex h-8 items-center gap-2 pl-3.5 text-sm transition-colors',
    'focus:outline-none focus-visible:underline',
    // 当前页：左侧 2px 墨色短竖线（伪元素，不占位）
    "before:absolute before:left-0 before:top-1/2 before:h-4 before:w-0.5 before:-translate-y-1/2 before:rounded-full before:content-['']",
    active ? 'font-medium text-af-ink before:bg-af-ink' : 'text-af-ink-3 before:bg-transparent hover:text-af-ink'
  ].join(' ')
}
</script>
