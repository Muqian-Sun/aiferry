<template>
  <!--
    系统设置（2026-10-05）：后台只剩两项，去掉二级导航与小节，一页两行（两栏设置式，同用户站「基本信息」）、一个保存栏。
    状态与逻辑在 settings/useSettingsPage.ts。
  -->
  <AppLayout>
    <div v-if="loading" class="flex items-center justify-center py-12">
      <LoadingSpinner />
    </div>

    <!-- 加载失败：说清楚并给重试；保存按钮在失败时是禁用的，不说明会让人以为页面坏了 -->
    <div v-if="!loading && loadFailed" class="mb-6 flex flex-wrap items-center gap-3" data-testid="settings-load-error">
      <FormError :message="loadError" class="!mt-0" />
      <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('admin.settings.retry') }}</button>
    </div>

    <form v-if="!loading" novalidate data-testid="settings-form" @submit.prevent="save">
      <div class="-mt-8 divide-y divide-af-hairline">
        <SettingsRow :title="t('admin.settings.profitControl.title')" :description="t('admin.settings.profitControl.description')">
          <div class="flex flex-wrap items-center gap-3">
            <input
              v-model.number="form.profit_min_margin"
              type="number"
              min="0"
              max="0.99"
              step="0.01"
              class="input w-32"
              :aria-label="t('admin.settings.profitControl.title')"
              data-testid="profit-control-min-margin"
            />
            <span class="text-13 text-af-ink-3">{{ t('admin.settings.profitControl.hint') }}</span>
          </div>
        </SettingsRow>

        <SettingsRow :title="t('admin.settings.features.riskControl.title')" :description="t('admin.settings.features.riskControl.description')">
          <div class="flex flex-wrap items-center gap-4">
            <Toggle v-model="form.risk_control_enabled" :aria-label="t('admin.settings.features.riskControl.title')" />
            <RouterLink to="/risk-control" class="text-13 text-af-brand hover:text-af-brand-hover">
              {{ t('admin.settings.features.riskControl.configureLink') }} →
            </RouterLink>
          </div>
        </SettingsRow>
      </div>

      <!--
        保存栏只在有事可说时出现：有改动 / 正在保存 / 保存失败 / 刚保存成功（2.5 秒后收起）；
        没改动时不摆两个灰掉的按钮（2026-10-05 走查）
      -->
      <div v-if="dirty || saving || saveError || justSaved" class="settings-save-bar">
        <!-- 保存栏左侧：失败原因 > 未保存 > 刚保存成功 -->
        <span class="mr-auto inline-flex items-center gap-1.5 text-13" role="status" data-testid="settings-status">
          <span v-if="saveError" class="text-af-danger">{{ saveError }}</span>
          <span v-else-if="dirty" class="text-af-ink-3">{{ t('admin.settings.unsavedHint') }}</span>
          <template v-else-if="justSaved">
            <Icon name="check" size="sm" class="text-af-success" />
            <span class="text-af-success">{{ t('admin.settings.settingsSaved') }}</span>
          </template>
        </span>
        <template v-if="dirty || saving">
          <button type="button" class="btn btn-secondary" :disabled="saving || !dirty" data-testid="settings-discard" @click="discard">
            {{ t('admin.settings.discard') }}
          </button>
          <button type="submit" class="btn btn-primary" :disabled="saving || loadFailed || !dirty" data-testid="settings-save">
            {{ saving ? t('admin.settings.saving') : t('common.save') }}
          </button>
        </template>
      </div>
    </form>

    <!-- 有未保存的改动时离开设置页 -->
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
import { onBeforeUnmount, onMounted, reactive } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import SettingsRow from '@/components/user/shell/SettingsRow.vue'
import { useSettingsPage } from './settings/useSettingsPage'

const { dirty, discard, form, justSaved, load, loadError, loadFailed, loading, save, saveError, saving, t } = useSettingsPage()

// ---- 离开提醒：有改动时离开设置页先问；放弃就恢复成已保存的值
const leaveDialog = reactive<{ show: boolean; resolve: ((leave: boolean) => void) | null }>({ show: false, resolve: null })

function resolveLeave(leave: boolean) {
  leaveDialog.show = false
  leaveDialog.resolve?.(leave)
  leaveDialog.resolve = null
}

onBeforeRouteLeave(async () => {
  if (!dirty.value) return true
  const leave = await new Promise<boolean>((resolve) => {
    leaveDialog.resolve = resolve
    leaveDialog.show = true
  })
  if (leave) discard()
  return leave
})

// 刷新 / 关标签页：只能用浏览器自带的提示
function onBeforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value) event.preventDefault()
}
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', onBeforeUnload))
</script>

<style scoped>
/* 吸底保存栏 */
.settings-save-bar {
  /* sticky 不用 @apply：Tailwind 会把全局里带 .sticky 的规则一起改写过来 */
  position: sticky;
  bottom: 0;
  @apply z-10 mt-2 flex items-center justify-end gap-2 border-t border-af-hairline bg-af-sheet py-4;
}
</style>
