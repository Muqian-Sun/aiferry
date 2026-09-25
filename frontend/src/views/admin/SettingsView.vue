<template>
  <!--
    系统设置（A6）：二级导航 + 小节页，每节一个地址 /settings/<小节>。
    所有小节都渲染、v-show 切换（与原来的页签一致，切换不丢未保存的输入）；每节是自己的表单，底部是这一节的保存栏。
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
          @submit.prevent="saveSettings"
        >
          <header class="mb-6">
            <h2 class="text-xl font-semibold text-af-ink">{{ t(`admin.settings.sections.${section.key}.title`) }}</h2>
            <p class="mt-1 text-sm text-af-ink-3">{{ t(`admin.settings.sections.${section.key}.description`) }}</p>
          </header>

          <component :is="SECTION_COMPONENTS[section.key]" />

          <div class="settings-save-bar">
            <button
              type="submit"
              :disabled="saving || loadFailed"
              class="btn btn-primary"
              :data-testid="`settings-save-${section.key}`"
            >
              {{ saving ? t("admin.settings.saving") : t("admin.settings.saveSettings") }}
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
      <!-- 关闭 step-up 开关等敏感保存操作触发的 TOTP 二次验证 -->
      <TotpStepUpDialog :controller="settingsStepUp" />
  </AppLayout>
</template>

<script setup lang="ts">
// 系统设置页（A6 拆小节）：状态与逻辑在 settings/useSettingsPage.ts，provide 给 settings/sections/*Section.vue；
// 本页只剩二级导航、小节外壳（表单 + 保存栏）与弹窗。当前小节来自路由 /settings/:section。
import { computed, provide } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import PaymentProviderDialog from '@/components/admin/payment/providers/PaymentProviderDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import SettingsNav from './settings/SettingsNav.vue'
import { SETTINGS_SECTIONS, SECTION_COMPONENTS, resolveSettingsSection, type SettingsSectionKey } from './settings/sections'
import { SETTINGS_PAGE_KEY, useSettingsPage } from './settings/useSettingsPage'

const page = useSettingsPage()
provide(SETTINGS_PAGE_KEY, page)
const {
  allPaymentTypes,
  editingProvider,
  enabledProviderKeyOptions,
  handleDeleteProvider,
  handleSaveProvider,
  loadFailed,
  loading,
  providerKeyOptions,
  providerDialogRef,
  providerSaving,
  saveSettings,
  saving,
  settingsStepUp,
  showDeleteProviderDialog,
  showProviderDialog,
  t
} = page

const route = useRoute()
const router = useRouter()
const currentSection = computed(() => resolveSettingsSection(route.params.section))

function goSection(key: SettingsSectionKey) {
  if (key !== currentSection.value) void router.push({ name: 'AdminSettings', params: { section: key } })
}
</script>

<style scoped>
.settings-save-bar {
  @apply mt-8 flex justify-end border-t border-af-hairline pt-5;
}
</style>
