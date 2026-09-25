<template>
  <div class="space-y-6">
    <!-- Upstream Billing Probe Settings -->
    <div class="card" data-testid="upstream-billing-probe-settings">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.upstreamBillingProbe.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.upstreamBillingProbe.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div
          v-if="upstreamBillingProbeLoading"
          class="flex items-center gap-2 text-af-ink-3"
        >
          <div
            class="h-4 w-4 animate-spin rounded-full border-b-2 border-af-brand"
          ></div>
          {{ t("common.loading") }}
        </div>

        <template v-else>
          <div class="flex items-center justify-between gap-4">
            <div>
              <label class="font-medium text-af-ink">
                {{ t("admin.settings.upstreamBillingProbe.enabled") }}
              </label>
              <p class="text-sm text-af-ink-3">
                {{ t("admin.settings.upstreamBillingProbe.enabledHint") }}
              </p>
            </div>
            <Toggle
              v-model="upstreamBillingProbeForm.enabled"
              :aria-label="t('admin.settings.upstreamBillingProbe.enabled')"
              data-testid="upstream-billing-probe-enabled"
            />
          </div>

          <div
            v-if="upstreamBillingProbeForm.enabled"
            class="border-t border-af-hairline pt-4"
          >
            <label
              class="mb-2 block text-sm font-medium text-af-ink-2"
              for="upstream-billing-probe-interval"
            >
              {{ t("admin.settings.upstreamBillingProbe.intervalMinutes") }}
            </label>
            <input
              id="upstream-billing-probe-interval"
              v-model.number="upstreamBillingProbeForm.interval_minutes"
              type="number"
              min="5"
              max="1440"
              class="input w-32"
              data-testid="upstream-billing-probe-interval"
              @keydown.enter.prevent="saveUpstreamBillingProbeSettings"
            />
            <p class="mt-1.5 text-xs text-af-ink-3">
              {{ t("admin.settings.upstreamBillingProbe.intervalHint") }}
            </p>
          </div>

          <div
            class="flex justify-end border-t border-af-hairline pt-4"
          >
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="upstreamBillingProbeSaving"
              data-testid="upstream-billing-probe-save"
              @click="saveUpstreamBillingProbeSettings"
            >
              {{
                upstreamBillingProbeSaving
                  ? t("common.saving")
                  : t("common.save")
              }}
            </button>
          </div>
        </template>
      </div>
    </div>

    <!-- Ollama Cloud Usage Settings -->
    <div class="card" data-testid="ollama-cloud-usage-global-settings">
      <div class="border-b border-af-hairline px-6 py-4">
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.ollamaCloudUsage.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.ollamaCloudUsage.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div v-if="ollamaCloudUsageLoading" class="flex items-center gap-2 text-af-ink-3">
          <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-af-brand"></div>
          {{ t("common.loading") }}
        </div>
        <template v-else>
          <div class="flex items-center justify-between gap-4">
            <div>
              <label class="font-medium text-af-ink">
                {{ t("admin.settings.ollamaCloudUsage.enabled") }}
              </label>
              <p class="text-sm text-af-ink-3">
                {{ t("admin.settings.ollamaCloudUsage.enabledHint") }}
              </p>
            </div>
            <Toggle
              v-model="ollamaCloudUsageForm.enabled"
              :aria-label="t('admin.settings.ollamaCloudUsage.enabled')"
              data-testid="ollama-cloud-usage-global-enabled"
            />
          </div>
          <div v-if="ollamaCloudUsageForm.enabled" class="space-y-4 border-t border-af-hairline pt-4">
            <div>
              <label class="mb-2 block text-sm font-medium text-af-ink-2" for="ollama-cloud-usage-debounce">
                {{ t("admin.settings.ollamaCloudUsage.debounceMinutes") }}
              </label>
              <input
                id="ollama-cloud-usage-debounce"
                v-model.number="ollamaCloudUsageForm.debounce_minutes"
                type="number"
                min="1"
                max="60"
                class="input w-32"
                data-testid="ollama-cloud-usage-global-debounce"
                @keydown.enter.prevent="saveOllamaCloudUsageSettings"
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{ t("admin.settings.ollamaCloudUsage.debounceHint") }}
              </p>
            </div>
            <div>
              <label class="mb-2 block text-sm font-medium text-af-ink-2" for="ollama-cloud-usage-interval">
                {{ t("admin.settings.ollamaCloudUsage.intervalMinutes") }}
              </label>
              <input
                id="ollama-cloud-usage-interval"
                v-model.number="ollamaCloudUsageForm.interval_minutes"
                type="number"
                min="15"
                max="1440"
                class="input w-32"
                data-testid="ollama-cloud-usage-global-interval"
                @keydown.enter.prevent="saveOllamaCloudUsageSettings"
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{ t("admin.settings.ollamaCloudUsage.intervalHint") }}
              </p>
            </div>
          </div>
          <div class="flex justify-end border-t border-af-hairline pt-4">
            <button
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="ollamaCloudUsageSaving"
              data-testid="ollama-cloud-usage-global-save"
              @click="saveOllamaCloudUsageSettings"
            >
              {{ ollamaCloudUsageSaving ? t("common.saving") : t("common.save") }}
            </button>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 系统设置 › upstream（A6 从 SettingsView 拆出，卡片模板原样搬来；状态与逻辑在 useSettingsPage）
import Toggle from '@/components/common/Toggle.vue'
import { useSettingsPageContext } from '../useSettingsPage'

const {
  ollamaCloudUsageForm,
  ollamaCloudUsageLoading,
  ollamaCloudUsageSaving,
  saveOllamaCloudUsageSettings,
  saveUpstreamBillingProbeSettings,
  t,
  upstreamBillingProbeForm,
  upstreamBillingProbeLoading,
  upstreamBillingProbeSaving
} = useSettingsPageContext()
</script>
