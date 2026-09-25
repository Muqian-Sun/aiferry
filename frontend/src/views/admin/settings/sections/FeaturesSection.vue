<template>
  <div class="space-y-6">
    <div class="card">
      <div class="border-b border-af-hairline px-6 py-4">
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t('admin.settings.features.channelMonitor.title') }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t('admin.settings.features.channelMonitor.description') }}
        </p>
        <p class="mt-1.5 text-xs">
          <router-link
            to="/channels/monitor"
            class="inline-flex items-center gap-1 text-af-brand hover:underline"
          >
            {{ t('admin.settings.features.channelMonitor.configureLink') }}
            <span aria-hidden="true">→</span>
          </router-link>
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div class="flex items-center justify-between">
          <div>
            <label class="text-sm font-medium text-af-ink-2">
              {{ t('admin.settings.features.channelMonitor.enabled') }}
            </label>
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{ t('admin.settings.features.channelMonitor.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.channel_monitor_enabled" />
        </div>
      </div>
    </div>

    <!-- 运维监控开关（A3）：关掉后侧栏入口变灰、运维页不可用，只能从这里再打开 -->
    <div class="card" data-testid="settings-ops-monitoring">
      <div class="border-b border-af-hairline px-6 py-4">
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t('admin.settings.opsMonitoring.title') }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t('admin.settings.opsMonitoring.description') }}
        </p>
      </div>
      <div class="p-6">
        <div class="flex items-center justify-between">
          <div>
            <label class="text-sm font-medium text-af-ink-2">
              {{ t('admin.settings.opsMonitoring.enabled') }}
            </label>
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{ t('admin.settings.opsMonitoring.enabledHint') }}
            </p>
          </div>
          <Toggle v-model="form.ops_monitoring_enabled" />
        </div>
      </div>
    </div>

    <div class="card">
      <div class="border-b border-af-hairline px-6 py-4">
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t('admin.settings.features.modelPlaza.title') }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t('admin.settings.features.modelPlaza.description') }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div>
          <label class="text-sm font-medium text-af-ink-2">
            {{ t('admin.settings.features.modelPlaza.priceDescription') }}
          </label>
          <p class="mb-2 mt-0.5 text-xs text-af-ink-3">
            {{ t('admin.settings.features.modelPlaza.priceDescriptionHint') }}
          </p>
          <textarea
            v-model="form.model_plaza_description"
            rows="6"
            class="input font-mono text-sm"
          ></textarea>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="border-b border-af-hairline px-6 py-4">
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t('admin.settings.features.siteBillingMode.title') }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t('admin.settings.features.siteBillingMode.description') }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <label class="text-sm font-medium text-af-ink-2">
              {{ t('admin.settings.features.siteBillingMode.label') }}
            </label>
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{ siteBillingModeHint }}
            </p>
          </div>
          <div class="w-full shrink-0 sm:w-56">
            <Select
              :modelValue="siteBillingMode"
              :options="siteBillingModeOptions"
              @update:modelValue="siteBillingMode = $event as SiteBillingMode"
            />
          </div>
        </div>
      </div>
    </div>

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
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import type { SiteBillingMode } from '@/utils/siteBillingMode'
import { useSettingsPageContext } from '../useSettingsPage'

const {
  form,
  siteBillingMode,
  siteBillingModeHint,
  siteBillingModeOptions,
  t
} = useSettingsPageContext()
</script>
