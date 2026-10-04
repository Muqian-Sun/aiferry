<template>
  <!-- 管理站侧栏：品牌 / 分组导航 / 主题与折叠。一张面 + hairline，激活项左侧一道品牌色。 -->
  <aside
    class="sidebar"
    :class="[
      sidebarCollapsed ? 'w-16' : 'w-60',
      { '-translate-x-full lg:translate-x-0': !mobileOpen }
    ]"
  >
    <!-- Logo/Brand -->
    <div class="sidebar-header" :class="{ 'sidebar-header-collapsed': sidebarCollapsed }">
      <!-- Custom Logo or Default Logo -->
      <router-link
        :to="homePath"
        class="sidebar-logo flex h-7 w-7 items-center justify-center overflow-hidden rounded-md transition-opacity hover:opacity-80"
        @click="handleMenuItemClick"
      >
        <BrandLogo v-if="settingsLoaded" :src="siteLogo" alt="Logo" class="h-full w-full text-af-ink" />
      </router-link>
      <div class="sidebar-brand" :class="{ 'sidebar-brand-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
        <router-link
          :to="homePath"
          class="sidebar-brand-title text-base font-semibold text-af-ink transition-colors hover:text-af-brand"
          @click="handleMenuItemClick"
        >
          {{ siteName }}
        </router-link>
        <!-- 版本号，纯文本 -->
        <slot name="version" />
      </div>
    </div>

    <!-- Navigation -->
    <nav ref="sidebarNavRef" class="sidebar-nav scrollbar-hide">
      <div v-for="section in sections" :key="section.key" class="sidebar-section">
        <div
          v-if="section.title"
          class="sidebar-section-title"
          :class="{ 'sidebar-section-title-collapsed': sidebarCollapsed }"
          :aria-hidden="sidebarCollapsed ? 'true' : 'false'"
        >
          <span class="sidebar-section-title-text" :class="{ 'sidebar-section-title-text-collapsed': sidebarCollapsed }">
            {{ section.title }}
          </span>
        </div>
        <template v-for="item in section.items" :key="item.path">
          <!-- 未开启的功能：灰色入口，点进去是「未开启 · 去设置打开」 -->
          <router-link
            v-if="item.disabled"
            :to="{ path: '/feature-off', query: { name: item.label } }"
            class="sidebar-link sidebar-link-disabled mb-1"
            :class="{ 'sidebar-link-collapsed': sidebarCollapsed }"
            :title="sidebarCollapsed ? `${item.label} · ${t('nav.featureOff')}` : undefined"
            :data-testid="`nav-disabled-${item.path}`"
            @click="handleMenuItemClick"
          >
            <component :is="item.icon" class="h-[18px] w-[18px] flex-shrink-0 text-af-ink-3" />
            <span class="sidebar-label sidebar-label-flex" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">
              <span class="min-w-0 truncate">{{ item.label }}</span>
              <span class="shrink-0 text-xs text-af-ink-3">{{ t('nav.featureOff') }}</span>
            </span>
          </router-link>
          <!-- Normal item (no children) -->
          <router-link
            v-else
            :to="item.path"
            class="sidebar-link mb-1"
            :class="{ 'sidebar-link-active': isActive(item.path), 'sidebar-link-collapsed': sidebarCollapsed }"
            :title="sidebarCollapsed ? item.label : undefined"
            @click="handleMenuItemClick"
          >
            <span v-if="item.iconSvg" class="h-[18px] w-[18px] flex-shrink-0 sidebar-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
            <component v-else :is="item.icon" class="h-[18px] w-[18px] flex-shrink-0 text-af-ink-3" />
            <span class="sidebar-label" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">{{ item.label }}</span>
          </router-link>
        </template>
      </div>
    </nav>

    <!-- Bottom Section -->
    <div class="mt-auto border-t border-af-hairline p-2">
      <!-- Theme Toggle -->
      <button
        @click="toggleTheme"
        class="sidebar-link mb-1 w-full"
        :class="{ 'sidebar-link-collapsed': sidebarCollapsed }"
        :title="sidebarCollapsed ? (isDark ? t('nav.lightMode') : t('nav.darkMode')) : undefined"
      >
        <SunIcon v-if="isDark" class="h-[18px] w-[18px] flex-shrink-0 text-af-ink-3" />
        <MoonIcon v-else class="h-[18px] w-[18px] flex-shrink-0 text-af-ink-3" />
        <span class="sidebar-label" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">{{
          isDark ? t('nav.lightMode') : t('nav.darkMode')
        }}</span>
      </button>

      <!-- Collapse Button -->
      <button
        @click="toggleSidebar"
        class="sidebar-link w-full"
        :class="{ 'sidebar-link-collapsed': sidebarCollapsed }"
        :title="sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
      >
        <ChevronDoubleLeftIcon v-if="!sidebarCollapsed" class="h-[18px] w-[18px] flex-shrink-0 text-af-ink-3" />
        <ChevronDoubleRightIcon v-else class="h-[18px] w-[18px] flex-shrink-0 text-af-ink-3" />
        <span class="sidebar-label" :class="{ 'sidebar-label-collapsed': sidebarCollapsed }" :aria-hidden="sidebarCollapsed ? 'true' : 'false'">{{ t('nav.collapse') }}</span>
      </button>
    </div>
  </aside>

  <!-- Mobile Overlay -->
  <transition name="fade">
    <div
      v-if="mobileOpen"
      class="fixed inset-0 z-30 bg-black/40 lg:hidden"
      @click="closeMobile"
    ></div>
  </transition>
</template>

<script setup lang="ts">
/**
 * 侧边栏外框：品牌、导航分区渲染、主题与折叠。导航内容由站点侧边栏
 * （UserSidebar / AdminSidebar）按各自规则构建后传入，本组件不含任何站点专属逻辑。
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useTheme } from '@/composables/useTheme'
import { sanitizeSvg } from '@/utils/sanitize'
import { sanitizeUrl } from '@/utils/url'
import BrandLogo from '@/components/common/BrandLogo.vue'
import type { NavSection } from './navTypes'
import {
  ChevronDoubleLeftIcon,
  ChevronDoubleRightIcon,
  MoonIcon,
  SunIcon
} from './navIcons'

const props = defineProps<{
  sections: NavSection[]
  homePath: string
}>()

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()

const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const mobileOpen = computed(() => appStore.mobileOpen)
const sidebarNavRef = ref<HTMLElement | null>(null)
const { isDark, toggleTheme } = useTheme()

const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

function toggleSidebar() {
  appStore.toggleSidebar()
}

function closeMobile() {
  appStore.setMobileOpen(false)
}

function handleMenuItemClick() {
  if (mobileOpen.value) {
    setTimeout(() => {
      appStore.setMobileOpen(false)
    }, 150)
  }
}

/**
 * 当前项：所有入口（含 activePaths）里与当前路径匹配得最长的那一个。
 * 避免 /orders/plans 同时点亮「订单」（前缀 /orders）与「订阅」（activePaths 含 /orders/plans）。
 */
const activeItemPath = computed(() => {
  let best = ''
  let bestLength = -1
  for (const section of props.sections) {
    for (const item of section.items) {
      for (const candidate of [item.path, ...(item.activePaths ?? [])]) {
        const matches = route.path === candidate || route.path.startsWith(`${candidate}/`)
        if (matches && candidate.length > bestLength) {
          best = item.path
          bestLength = candidate.length
        }
      }
    }
  }
  return best
})

function isActive(path: string): boolean {
  return activeItemPath.value === path
}

onMounted(() => {
  // Restore sidebar scroll position after route change re-mounts the component
  if (appStore.sidebarScrollTop > 0 && sidebarNavRef.value) {
    void nextTick(() => {
      if (sidebarNavRef.value) {
        sidebarNavRef.value.scrollTop = appStore.sidebarScrollTop
      }
    })
  }
})

onBeforeUnmount(() => {
  if (sidebarNavRef.value) {
    appStore.sidebarScrollTop = sidebarNavRef.value.scrollTop
  }
})
</script>

<style scoped>
.sidebar-logo {
  flex: 0 0 1.75rem;
  min-width: 1.75rem;
}

.sidebar-header-collapsed {
  gap: 0;
  padding-left: 1.125rem;
  padding-right: 1.125rem;
}

.sidebar-brand {
  min-width: 0;
  flex: 1 1 auto;
  white-space: nowrap;
  transition:
    max-width 0.22s ease,
    opacity 0.14s ease,
    transform 0.14s ease;
  max-width: 12rem;
}

.sidebar-brand-collapsed {
  max-width: 0;
  overflow: hidden;
  opacity: 0;
  transform: translateX(-4px);
  pointer-events: none;
}

.sidebar-brand-title {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sidebar-link-collapsed {
  gap: 0;
  padding-left: 0.8125rem;
  padding-right: 0.8125rem;
}

.sidebar-section-title {
  position: relative;
  display: flex;
  align-items: center;
  min-height: 1.25rem;
  overflow: hidden;
  white-space: nowrap;
}

.sidebar-section-title-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition:
    opacity 0.16s ease,
    transform 0.16s ease;
}

.sidebar-section-title::after {
  content: '';
  position: absolute;
  left: 0.75rem;
  right: 0.75rem;
  top: 50%;
  height: 1px;
  background: rgb(var(--af-hairline));
  opacity: 0;
  transform: translateY(-50%);
  transition: opacity 0.18s ease;
}

.sidebar-section-title-text-collapsed {
  opacity: 0;
  transform: translateX(-4px);
}

.sidebar-section-title-collapsed::after {
  opacity: 1;
  transition-delay: 0.08s;
}

.sidebar-label {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition:
    max-width 0.2s ease,
    opacity 0.12s ease,
    transform 0.12s ease;
  max-width: 12rem;
}

.sidebar-label-flex {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.sidebar-label-collapsed {
  max-width: 0;
  opacity: 0;
  transform: translateX(-4px);
  pointer-events: none;
}

/* Custom SVG icon in sidebar: constrain size without overriding uploaded SVG colors */
.sidebar-svg-icon {
  color: currentColor;
}

.sidebar-svg-icon :deep(svg) {
  display: block;
  width: 1.25rem;
  height: 1.25rem;
}
</style>
