<template>
  <div class="space-y-6">
    <div class="card">
      <div class="border-b border-af-hairline px-6 py-4">
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t('admin.settings.features.riskControl.title') }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t('admin.settings.features.riskControl.description') }}
        </p>
        <p class="mt-1.5 text-xs">
          <router-link
            to="/risk-control"
            class="inline-flex items-center gap-1 text-af-brand hover:underline"
          >
            {{ t('admin.settings.features.riskControl.configureLink') }}
            <span aria-hidden="true">→</span>
          </router-link>
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div class="flex items-center justify-between">
          <div>
            <label class="text-sm font-medium text-af-ink-2">
              {{ t('admin.settings.features.riskControl.enabled') }}
            </label>
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{ t('admin.settings.features.riskControl.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.risk_control_enabled" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 系统设置 › features（A6 从 SettingsView 拆出，卡片模板原样搬来；状态与逻辑在 useSettingsPage）
// 2026-09-27：渠道监控写死开、运维监控只认部署配置 OPS_ENABLED、模型广场说明写进代码，这里只剩风控开关
import Toggle from '@/components/common/Toggle.vue'
import { useSettingsPageContext } from '../useSettingsPage'

const {
  form,
  t
} = useSettingsPageContext()
</script>
