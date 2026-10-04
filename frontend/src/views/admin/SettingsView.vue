<template>
  <!--
    系统设置（A6）：二级导航 + 小节页，每节一个地址 /settings/<小节>。
    所有小节都渲染、v-show 切换；每节是自己的表单，底部吸底保存栏只保存这一节。
    有未保存的改动时切走会先问「放弃 / 留下」，所以同一时间只有当前小节可能有改动（见 useSettingsPage 的 A6-2）。
    状态与逻辑在 settings/useSettingsPage.ts；小节模板在 settings/sections/*Section.vue。
  -->
  <AppLayout>
    <div v-if="loading" class="flex items-center justify-center py-12">
      <LoadingSpinner />
    </div>

    <!-- 加载失败：说清楚并给重试；保存按钮在失败时是禁用的，不说明会让人以为页面坏了 -->
    <div v-if="!loading && loadFailed" class="mb-6 flex flex-wrap items-center gap-3" data-testid="settings-load-error">
      <FormError :message="loadError" class="!mt-0" />
      <button type="button" class="btn btn-secondary btn-sm" @click="reload">{{ t('admin.settings.retry') }}</button>
    </div>

    <div v-if="!loading" class="flex flex-col gap-8 lg:flex-row lg:items-start">
      <SettingsNav :current="currentSection" @select="goSection" />

      <div class="min-w-0 flex-1">
        <form
          v-for="section in SETTINGS_SECTIONS"
          v-show="section.key === currentSection"
          :key="section.key"
          :data-testid="`settings-section-${section.key}`"
          novalidate
          @submit.prevent="saveSection()"
        >
          <header class="mb-6">
            <h2 class="text-xl font-semibold text-af-ink">{{ t(`admin.settings.sections.${section.key}.title`) }}</h2>
            <p class="mt-1 text-sm text-af-ink-3">{{ t(`admin.settings.sections.${section.key}.description`) }}</p>
          </header>

          <div class="settings-blocks">
            <component :is="SECTION_COMPONENTS[section.key]" />
          </div>

          <div class="settings-save-bar">
            <!-- 保存栏左侧：失败原因 > 未保存 > 刚保存成功（2.5 秒后收起） -->
            <span class="mr-auto inline-flex items-center gap-1.5 text-13" role="status" :data-testid="`settings-status-${section.key}`">
              <template v-if="section.key === currentSection && saveError">
                <span class="text-af-danger">{{ saveError }}</span>
              </template>
              <template v-else-if="isSectionDirty(section.key)">
                <span class="text-af-ink-3">{{ t('admin.settings.unsavedHint') }}</span>
              </template>
              <template v-else-if="section.key === currentSection && justSaved">
                <Icon name="check" size="sm" class="text-af-success" />
                <span class="text-af-success">{{ t('admin.settings.settingsSaved') }}</span>
              </template>
            </span>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="sectionSaving || !isSectionDirty(section.key)"
              :data-testid="`settings-discard-${section.key}`"
              @click="discardSection(section.key)"
            >
              {{ t('admin.settings.discard') }}
            </button>
            <button
              type="submit"
              class="btn btn-primary"
              :disabled="sectionSaving || loadFailed || !isSectionDirty(section.key)"
              :data-testid="`settings-save-${section.key}`"
            >
              {{ sectionSaving ? t('admin.settings.saving') : t('admin.settings.saveSection') }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- 有未保存的改动时切小节 / 离开设置页 -->
    <ConfirmDialog
      :show="leaveDialog.show"
      :title="t('admin.settings.leaveTitle')"
      :message="t('admin.settings.leaveMessage')"
      :confirm-text="t('admin.settings.discard')"
      :cancel-text="t('admin.settings.stay')"
      danger
      @confirm="resolveLeave(true)"
      @cancel="resolveLeave(false)"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
// 系统设置页（A6 拆小节）：状态与逻辑在 settings/useSettingsPage.ts，provide 给 settings/sections/*Section.vue；
// 本页只剩二级导航、小节外壳（表单 + 保存栏）、离开提醒与弹窗。当前小节来自路由 /settings/:section。
import { computed, onBeforeUnmount, onMounted, provide, reactive } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Icon from '@/components/icons/Icon.vue'
import SettingsNav from './settings/SettingsNav.vue'
import { SETTINGS_SECTIONS, SECTION_COMPONENTS, resolveSettingsSection, type SettingsSectionKey } from './settings/sections'
import { SETTINGS_PAGE_KEY, useSettingsPage } from './settings/useSettingsPage'

const route = useRoute()
const router = useRouter()
const currentSection = computed(() => resolveSettingsSection(route.params.section))

const page = useSettingsPage(currentSection)
provide(SETTINGS_PAGE_KEY, page)
const {
  discardSection,
  isSectionDirty,
  justSaved,
  loadError,
  loadFailed,
  loading,
  reload,
  saveError,
  saveSection,
  sectionSaving,
  t
} = page

function goSection(key: SettingsSectionKey) {
  if (key !== currentSection.value) void router.push({ name: 'AdminSettings', params: { section: key } })
}

// ---- 离开提醒：当前小节有改动时，切小节 / 离开设置页先问；放弃就恢复成已保存的值
const leaveDialog = reactive<{ show: boolean; resolve: ((leave: boolean) => void) | null }>({ show: false, resolve: null })

function resolveLeave(leave: boolean) {
  leaveDialog.show = false
  leaveDialog.resolve?.(leave)
  leaveDialog.resolve = null
}

async function guardUnsaved(): Promise<boolean> {
  const key = currentSection.value
  if (!isSectionDirty(key)) return true
  const leave = await new Promise<boolean>((resolve) => {
    leaveDialog.resolve = resolve
    leaveDialog.show = true
  })
  if (leave) discardSection(key)
  return leave
}

onBeforeRouteUpdate(guardUnsaved)
onBeforeRouteLeave(guardUnsaved)

// 刷新 / 关标签页：只能用浏览器自带的提示
function onBeforeUnload(event: BeforeUnloadEvent) {
  if (SETTINGS_SECTIONS.some((section) => isSectionDirty(section.key))) event.preventDefault()
}
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', onBeforeUnload))
</script>

<style scoped>
/*
 * 两栏布局（A6-3，同用户站「基本信息」）：小节里每张卡片 = 标题头 + 正文。
 * 卡片框去掉，区块之间只用 hairline 分隔；宽屏标题头在左（这一项是干什么的），正文在右；窄屏单栏。
 * 小节模板是从原设置页原样搬来的，结构统一（见各 *Section.vue），所以在外壳上一处改样式即可。
 */
.settings-blocks > :deep(div > .card) {
  @apply mt-0 grid gap-x-10 gap-y-4 rounded-none border-0 border-t border-af-hairline bg-transparent py-7 shadow-none lg:grid-cols-[15rem_minmax(0,1fr)];
}

/* 只有一块内容的卡片（提示条）不分栏 */
.settings-blocks > :deep(div > .card:has(> :only-child)) {
  @apply block;
}

/* 标题头；带按钮的在左栏里上下排，不带的 flex-col 不起作用 */
.settings-blocks > :deep(div > .card > :first-child) {
  @apply flex-col items-start gap-3 border-0 p-0;
}

.settings-blocks > :deep(div > .card > :first-child h2) {
  @apply text-base;
}

.settings-blocks > :deep(div > .card > :not(:first-child)) {
  @apply min-w-0 p-0;
}

/* 吸底保存栏：长小节滚到哪都能保存 */
.settings-save-bar {
  /* sticky 不用 @apply：Tailwind 会把全局里带 .sticky 的规则一起改写过来 */
  position: sticky;
  bottom: 0;
  @apply z-10 mt-8 flex items-center justify-end gap-2 border-t border-af-hairline bg-af-sheet py-4;
}
</style>
