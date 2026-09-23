<template>
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
        :class="tabClass(isTabActive(tab, currentPath))"
        :aria-current="isTabActive(tab, currentPath) ? 'page' : undefined"
        :data-tour="tab.dataTour"
      >
        {{ tab.label }}
      </RouterLink>
    </template>

    <!-- 管理员配置的自定义页收进「更多」 -->
    <div v-if="more.length" ref="moreRef" class="relative flex items-center">
      <button
        type="button"
        :class="tabClass(moreActive)"
        :aria-expanded="moreOpen"
        aria-haspopup="menu"
        data-testid="nav-more"
        @click="moreOpen = !moreOpen"
      >
        {{ t('userUi.nav.more') }}
        <Icon name="chevronDown" size="xs" :class="['transition-transform', moreOpen && 'rotate-180']" />
      </button>
      <div
        v-if="moreOpen"
        class="absolute left-0 top-full z-50 mt-1 w-56 rounded-lg border border-af-hairline bg-af-sheet py-1 shadow-lg"
        role="menu"
      >
        <RouterLink
          v-for="item in more"
          :key="item.path"
          :to="item.path"
          class="dropdown-item"
          role="menuitem"
          @click="moreOpen = false"
        >
          <span v-if="item.iconSvg" class="h-4 w-4 shrink-0 [&>svg]:h-4 [&>svg]:w-4" v-html="sanitizeSvg(item.iconSvg)"></span>
          <span class="truncate">{{ item.label }}</span>
        </RouterLink>
      </div>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeSvg } from '@/utils/sanitize'
import { isTabActive, type NavTab } from './navItems'

const props = withDefaults(defineProps<{ tabs: NavTab[]; more?: NavTab[] }>(), { more: () => [] })

const { t } = useI18n()
const route = useRoute()
const currentPath = computed(() => route.path)
const moreOpen = ref(false)
const moreRef = ref<HTMLElement | null>(null)
const moreActive = computed(() => props.more.some((item) => isTabActive(item, currentPath.value)))

/* 页签：胶囊式，当前页一块浅底 + 墨色字，其余悬停出浅底 */
function tabClass(active: boolean): string {
  return [
    'inline-flex h-9 shrink-0 items-center gap-1 whitespace-nowrap rounded-full px-3.5 text-sm font-medium transition-colors',
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-af-brand/40',
    active ? 'bg-af-sunken text-af-ink' : 'text-af-ink-2 hover:bg-af-sunken/70 hover:text-af-ink'
  ].join(' ')
}

function onDocumentClick(event: MouseEvent) {
  if (moreRef.value && !moreRef.value.contains(event.target as Node)) moreOpen.value = false
}

watch(currentPath, () => (moreOpen.value = false))
onMounted(() => document.addEventListener('click', onDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))
</script>
