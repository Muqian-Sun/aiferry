<template>
  <!--
    系统设置（A6）：二级导航 + 小节页，每节一个地址 /settings/<小节>。
    所有小节都渲染、v-show 切换；每节是自己的表单，底部吸底保存栏只保存这一节。
    有未保存的改动时切走会先问「放弃 / 留下」，所以同一时间只有当前小节可能有改动（见 useSettingsPage 的 A6-2）。
    状态与逻辑在 settings/useSettingsPage.ts；小节模板在 settings/sections/*Section.vue。
  -->
  <AppLayout>
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-af-brand"></div>
    </div>

    <div v-else class="flex flex-col gap-8 lg:flex-row lg:items-start">
      <SettingsNav :current="currentSection" @select="goSection" />

      <div class="min-w-0 flex-1">
        <form
          v-for="section in SETTINGS_SECTIONS"
          v-show="section.key === currentSection"
          :key="section.key"
          :data-testid="`settings-section-${section.key}`"
          novalidate
          @submit.prevent="saveSection(section.key)"
        >
          <header class="mb-6">
            <h2 class="text-xl font-semibold text-af-ink">{{ t(`admin.settings.sections.${section.key}.title`) }}</h2>
            <p class="mt-1 text-sm text-af-ink-3">{{ t(`admin.settings.sections.${section.key}.description`) }}</p>
          </header>

          <component :is="SECTION_COMPONENTS[section.key]" />

          <div class="settings-save-bar">
            <span v-if="isSectionDirty(section.key)" class="mr-auto text-13 text-af-ink-3">
              {{ t('admin.settings.unsavedHint') }}
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

    <!-- Provider dialogs placed outside the settings form to prevent form submission bubbling -->
    <PaymentProviderDialog
      ref="providerDialogRef"
      :show="showProviderDialog"
      :saving="providerSaving"
      :editing="editingProvider"
      :all-key-options="providerKeyOptions"
      :enabled-key-options="enabledProviderKeyOptions"
      :all-payment-types="allPaymentTypes"
      :redirect-label="t('admin.settings.payment.easypayRedirect')"
      @close="showProviderDialog = false"
      @save="handleSaveProvider"
    />
    <ConfirmDialog
      :show="showDeleteProviderDialog"
      :title="t('admin.settings.payment.deleteProvider')"
      :message="t('admin.settings.payment.deleteProviderConfirm')"
      :confirm-text="t('common.delete')"
      danger
      @confirm="handleDeleteProvider"
      @cancel="showDeleteProviderDialog = false"
    />
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
    <!-- 关闭 step-up 开关等敏感保存操作触发的 TOTP 二次验证 -->
    <TotpStepUpDialog :controller="settingsStepUp" />
  </AppLayout>
</template>

<script setup lang="ts">
// 系统设置页（A6 拆小节）：状态与逻辑在 settings/useSettingsPage.ts，provide 给 settings/sections/*Section.vue；
// 本页只剩二级导航、小节外壳（表单 + 保存栏）、离开提醒与弹窗。当前小节来自路由 /settings/:section。
import { computed, onBeforeUnmount, onMounted, provide, reactive } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import PaymentProviderDialog from '@/components/admin/payment/providers/PaymentProviderDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import SettingsNav from './settings/SettingsNav.vue'
import { SETTINGS_SECTIONS, SECTION_COMPONENTS, resolveSettingsSection, type SettingsSectionKey } from './settings/sections'
import { SETTINGS_PAGE_KEY, useSettingsPage } from './settings/useSettingsPage'

const route = useRoute()
const router = useRouter()
const currentSection = computed(() => resolveSettingsSection(route.params.section))

const page = useSettingsPage(currentSection)
provide(SETTINGS_PAGE_KEY, page)
const {
  allPaymentTypes,
  discardSection,
  editingProvider,
  enabledProviderKeyOptions,
  handleDeleteProvider,
  handleSaveProvider,
  isSectionDirty,
  loadFailed,
  loading,
  providerKeyOptions,
  providerSaving,
  saveSection,
  sectionSaving,
  settingsStepUp,
  showDeleteProviderDialog,
  showProviderDialog,
  t,
  providerDialogRef
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
/* 吸底保存栏：长小节滚到哪都能保存 */
.settings-save-bar {
  @apply sticky bottom-0 z-10 mt-8 flex items-center justify-end gap-2 border-t border-af-hairline bg-af-sheet py-4;
}
</style>
