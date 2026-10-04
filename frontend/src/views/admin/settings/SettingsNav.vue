<template>
  <!--
    系统设置二级导航（A6）：组标题 + 小节。宽屏是左侧一列，窄屏收成一行横向滚动。
    样式同侧栏：当前项文字加粗 + 左侧竖线，不用底色块。
  -->
  <nav class="settings-nav" :aria-label="t('admin.settings.navLabel')">
    <div v-for="group in SETTINGS_SECTION_GROUPS" :key="group.key" class="settings-nav-group">
      <p class="settings-nav-title">{{ t(`admin.settings.sectionGroups.${group.key}`) }}</p>
      <button
        v-for="key in group.sections"
        :key="key"
        type="button"
        class="settings-nav-link"
        :class="{ 'settings-nav-link-active': key === current }"
        :aria-current="key === current ? 'page' : undefined"
        :data-testid="`settings-nav-${key}`"
        @click="emit('select', key)"
      >
        {{ t(`admin.settings.sections.${key}.title`) }}
      </button>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { SETTINGS_SECTION_GROUPS, type SettingsSectionKey } from './sections'

defineProps<{ current: SettingsSectionKey }>()
const emit = defineEmits<{ select: [key: SettingsSectionKey] }>()
const { t } = useI18n()
</script>

<style scoped>
.settings-nav {
  @apply -mx-4 flex gap-6 overflow-x-auto px-4 pb-2 lg:sticky lg:top-20 lg:mx-0 lg:w-44 lg:shrink-0 lg:flex-col lg:gap-5 lg:overflow-visible lg:px-0 lg:pb-0;
}

.settings-nav-group {
  @apply flex shrink-0 items-center gap-1 lg:flex-col lg:items-stretch lg:gap-0.5;
}

.settings-nav-title {
  @apply hidden text-xs text-af-ink-3 lg:mb-1 lg:block;
}

.settings-nav-link {
  @apply relative whitespace-nowrap rounded-md px-2.5 py-1.5 text-left text-sm text-af-ink-2 transition-colors hover:text-af-ink lg:rounded-none lg:border-l-2 lg:border-transparent lg:py-1;
}

.settings-nav-link-active {
  @apply font-semibold text-af-ink lg:border-af-ink;
}
</style>
