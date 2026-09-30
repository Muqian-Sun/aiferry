<template>
  <!--
    控制台左侧分组栏（muqian 2026-09-23 定「左侧栏」）：主组无标题，账务 / 账户 / 更多各带小标题。
    整列一条右侧细线与内容隔开，不做底色、不做卡片、不做高亮色块（灰底块被否「丑死了」）：
    条目是图标 + 文字，当前页墨色加粗 + 左侧一道 2px 墨色短竖线（贴屏幕左缘），其余淡墨、悬停变深。
    整列贴屏幕左缘（muqian 2026-09-30：左边空白太大），条目左内边距 20px：图标与顶栏 logo 左缘对齐，收起时正好在 56px 图标栏正中。
    底部「收起」把整列收成图标栏（每项靠图标 + 悬停提示认页面），偏好记在本浏览器（muqian：下面加一个收缩的）。
    整列吸顶（顶栏下方）、占满视口高：条目区自己滚，收起按钮固定在底部。
    条目区是 overflow 容器，条目不能用负外边距往外探（会被裁掉）。窄屏不渲染（由顶栏第二行替代）。
  -->
  <aside
    v-if="sections.length"
    class="shrink-0 border-r border-af-hairline transition-[width] duration-200"
    :class="collapsed ? 'w-14' : 'w-52'"
    data-testid="console-sidebar"
    :data-collapsed="collapsed ? 'true' : undefined"
  >
    <div class="sticky top-topbar flex h-[calc(100svh_-_var(--af-topbar-h))] flex-col">
      <nav class="min-h-0 flex-1 space-y-6 overflow-y-auto overflow-x-hidden pb-4 pt-5 scrollbar-hide" :class="collapsed ? '' : 'pr-6'" :aria-label="t('userUi.nav.primaryNav')">
        <div v-for="section in sections" :key="section.key" :data-testid="`sidebar-section-${section.key}`">
          <template v-if="section.label">
            <p v-if="!collapsed" class="mb-1.5 pl-5 text-xs text-af-ink-4">{{ section.label }}</p>
            <div v-else class="mb-3 ml-5 h-px w-4 bg-af-hairline-strong" aria-hidden="true" />
          </template>
          <ul class="space-y-0.5">
            <li v-for="item in section.items" :key="item.path">
              <RouterLink
                :to="item.path"
                :class="linkClass(item.path === activePath)"
                :aria-current="item.path === activePath ? 'page' : undefined"
                :aria-label="collapsed ? item.label : undefined"
                :title="collapsed ? item.label : undefined"
                :data-tour="item.dataTour"
              >
                <span v-if="item.iconSvg" class="h-4 w-4 shrink-0 [&>svg]:h-4 [&>svg]:w-4" v-html="sanitizeSvg(item.iconSvg)"></span>
                <Icon v-else-if="item.icon" :name="item.icon" size="sm" class="shrink-0" />
                <span v-if="!collapsed" class="truncate">{{ item.label }}</span>
              </RouterLink>
            </li>
          </ul>
        </div>
      </nav>

      <div class="border-t border-af-hairline py-3">
        <button
          type="button"
          class="flex h-8 w-full items-center gap-2.5 pl-5 text-13 text-af-ink-3 transition-colors hover:text-af-ink focus:outline-none focus-visible:underline"
          :aria-label="collapsed ? t('userUi.nav.expandSidebar') : t('userUi.nav.collapseSidebar')"
          :title="collapsed ? t('userUi.nav.expandSidebar') : undefined"
          :aria-expanded="!collapsed"
          data-testid="sidebar-toggle"
          @click="toggle"
        >
          <Icon :name="collapsed ? 'chevronRight' : 'chevronLeft'" size="sm" class="shrink-0" />
          <span v-if="!collapsed">{{ t('userUi.nav.collapseSidebar') }}</span>
        </button>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeSvg } from '@/utils/sanitize'
import { pickActivePath } from './navItems'
import { useConsoleNav } from './useConsoleNav'
import { useSidebarCollapsed } from './useSidebarCollapsed'

const { t } = useI18n()
const route = useRoute()
const currentPath = computed(() => route.path)
const { sections, flatItems } = useConsoleNav()
const { collapsed, toggle } = useSidebarCollapsed()
/** 整栏只亮一项：所有条目里最长的匹配路径 */
const activePath = computed(() => pickActivePath(flatItems.value, currentPath.value))

function linkClass(active: boolean): string {
  return [
    'relative flex h-8 items-center gap-2.5 pl-5 text-sm transition-colors',
    'focus:outline-none focus-visible:underline',
    // 当前页：左侧 2px 墨色短竖线（伪元素，不占位）
    "before:absolute before:left-0 before:top-1/2 before:h-4 before:w-0.5 before:-translate-y-1/2 before:rounded-full before:content-['']",
    active ? 'font-medium text-af-ink before:bg-af-ink' : 'text-af-ink-3 before:bg-transparent hover:text-af-ink'
  ].join(' ')
}
</script>
