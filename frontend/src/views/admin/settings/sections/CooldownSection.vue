<template>
  <div class="space-y-6">
    <!-- Overload Cooldown (529) Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.overloadCooldown.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.overloadCooldown.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div
          v-if="overloadCooldownLoading"
          class="flex items-center gap-2 text-af-ink-3"
        >
          <div
            class="h-4 w-4 animate-spin rounded-full border-b-2 border-af-brand"
          ></div>
          {{ t("common.loading") }}
        </div>

        <template v-else>
          <div class="flex items-center justify-between">
            <div>
              <label class="font-medium text-af-ink">{{
                t("admin.settings.overloadCooldown.enabled")
              }}</label>
              <p class="text-sm text-af-ink-3">
                {{ t("admin.settings.overloadCooldown.enabledHint") }}
              </p>
            </div>
            <Toggle v-model="overloadCooldownForm.enabled" />
          </div>

          <div
            v-if="overloadCooldownForm.enabled"
            class="space-y-4 border-t border-af-hairline pt-4"
          >
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.overloadCooldown.cooldownMinutes") }}
              </label>
              <input
                v-model.number="overloadCooldownForm.cooldown_minutes"
                type="number"
                min="1"
                max="120"
                class="input w-32"
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{
                  t("admin.settings.overloadCooldown.cooldownMinutesHint")
                }}
              </p>
            </div>
          </div>

          <div
            class="flex justify-end border-t border-af-hairline pt-4"
          >
            <button
              type="button"
              @click="saveOverloadCooldownSettings"
              :disabled="overloadCooldownSaving"
              class="btn btn-primary btn-sm"
            >
              <svg
                v-if="overloadCooldownSaving"
                class="mr-1 h-4 w-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              {{
                overloadCooldownSaving
                  ? t("common.saving")
                  : t("common.save")
              }}
            </button>
          </div>
        </template>
      </div>
    </div>

    <!-- Rate Limit Cooldown (429) Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.rateLimit429Cooldown.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.rateLimit429Cooldown.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div
          v-if="rateLimit429CooldownLoading"
          class="flex items-center gap-2 text-af-ink-3"
        >
          <div
            class="h-4 w-4 animate-spin rounded-full border-b-2 border-af-brand"
          ></div>
          {{ t("common.loading") }}
        </div>

        <template v-else>
          <div class="flex items-center justify-between">
            <div>
              <label class="font-medium text-af-ink">{{
                t("admin.settings.rateLimit429Cooldown.enabled")
              }}</label>
              <p class="text-sm text-af-ink-3">
                {{ t("admin.settings.rateLimit429Cooldown.enabledHint") }}
              </p>
            </div>
            <Toggle v-model="rateLimit429CooldownForm.enabled" />
          </div>

          <div
            v-if="rateLimit429CooldownForm.enabled"
            class="space-y-4 border-t border-af-hairline pt-4"
          >
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{
                  t(
                    "admin.settings.rateLimit429Cooldown.cooldownSeconds",
                  )
                }}
              </label>
              <input
                v-model.number="rateLimit429CooldownForm.cooldown_seconds"
                type="number"
                min="1"
                max="7200"
                class="input w-32"
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{
                  t(
                    "admin.settings.rateLimit429Cooldown.cooldownSecondsHint",
                  )
                }}
              </p>
            </div>
          </div>

          <div
            class="flex justify-end border-t border-af-hairline pt-4"
          >
            <button
              type="button"
              @click="saveRateLimit429CooldownSettings"
              :disabled="rateLimit429CooldownSaving"
              class="btn btn-primary btn-sm"
            >
              <svg
                v-if="rateLimit429CooldownSaving"
                class="mr-1 h-4 w-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              {{
                rateLimit429CooldownSaving
                  ? t("common.saving")
                  : t("common.save")
              }}
            </button>
          </div>
        </template>
      </div>
    </div>

    <!-- Stream Timeout Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.streamTimeout.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.streamTimeout.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <!-- Loading State -->
        <div
          v-if="streamTimeoutLoading"
          class="flex items-center gap-2 text-af-ink-3"
        >
          <div
            class="h-4 w-4 animate-spin rounded-full border-b-2 border-af-brand"
          ></div>
          {{ t("common.loading") }}
        </div>

        <template v-else>
          <!-- Enable Stream Timeout -->
          <div class="flex items-center justify-between">
            <div>
              <label class="font-medium text-af-ink">{{
                t("admin.settings.streamTimeout.enabled")
              }}</label>
              <p class="text-sm text-af-ink-3">
                {{ t("admin.settings.streamTimeout.enabledHint") }}
              </p>
            </div>
            <Toggle v-model="streamTimeoutForm.enabled" />
          </div>

          <!-- Settings - Only show when enabled -->
          <div
            v-if="streamTimeoutForm.enabled"
            class="space-y-4 border-t border-af-hairline pt-4"
          >
            <!-- Action -->
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.streamTimeout.action") }}
              </label>
              <select
                v-model="streamTimeoutForm.action"
                class="input w-64"
              >
                <option value="temp_unsched">
                  {{
                    t("admin.settings.streamTimeout.actionTempUnsched")
                  }}
                </option>
                <option value="error">
                  {{ t("admin.settings.streamTimeout.actionError") }}
                </option>
                <option value="none">
                  {{ t("admin.settings.streamTimeout.actionNone") }}
                </option>
              </select>
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{ t("admin.settings.streamTimeout.actionHint") }}
              </p>
            </div>

            <!-- Temp Unsched Minutes (only show when action is temp_unsched) -->
            <div v-if="streamTimeoutForm.action === 'temp_unsched'">
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.streamTimeout.tempUnschedMinutes") }}
              </label>
              <input
                v-model.number="streamTimeoutForm.temp_unsched_minutes"
                type="number"
                min="1"
                max="60"
                class="input w-32"
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{
                  t("admin.settings.streamTimeout.tempUnschedMinutesHint")
                }}
              </p>
            </div>

            <!-- Threshold Count -->
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.streamTimeout.thresholdCount") }}
              </label>
              <input
                v-model.number="streamTimeoutForm.threshold_count"
                type="number"
                min="1"
                max="10"
                class="input w-32"
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{ t("admin.settings.streamTimeout.thresholdCountHint") }}
              </p>
            </div>

            <!-- Threshold Window Minutes -->
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{
                  t("admin.settings.streamTimeout.thresholdWindowMinutes")
                }}
              </label>
              <input
                v-model.number="
                  streamTimeoutForm.threshold_window_minutes
                "
                type="number"
                min="1"
                max="60"
                class="input w-32"
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{
                  t(
                    "admin.settings.streamTimeout.thresholdWindowMinutesHint",
                  )
                }}
              </p>
            </div>
          </div>

          <!-- Save Button -->
          <div
            class="flex justify-end border-t border-af-hairline pt-4"
          >
            <button
              type="button"
              @click="saveStreamTimeoutSettings"
              :disabled="streamTimeoutSaving"
              class="btn btn-primary btn-sm"
            >
              <svg
                v-if="streamTimeoutSaving"
                class="mr-1 h-4 w-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              {{
                streamTimeoutSaving
                  ? t("common.saving")
                  : t("common.save")
              }}
            </button>
          </div>
        </template>
      </div>
    </div>

    <!-- Gateway Scheduling Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.scheduling.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.scheduling.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div>
          <div class="mb-3">
            <label class="font-medium text-af-ink">
              {{
                t(
                  "admin.settings.scheduling.accountSchedulingThresholdsTitle",
                )
              }}
            </label>
            <p class="mt-1 text-sm text-af-ink-3">
              {{
                t(
                  "admin.settings.scheduling.accountSchedulingThresholdsDescription",
                )
              }}
            </p>
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{
                t(
                  "admin.settings.scheduling.accountSchedulingThresholdsGlobalHint",
                )
              }}
            </p>
            <p class="mt-0.5 text-xs text-af-warning">
              {{
                t(
                  "admin.settings.scheduling.accountSchedulingThresholdsDisabledHint",
                )
              }}
            </p>
          </div>
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            <div
              v-for="platform in schedulingThresholdPlatforms"
              :key="platform"
              class="rounded-lg border border-af-hairline p-4"
            >
              <div class="flex items-start justify-between gap-3">
                <div>
                  <label
                    class="font-mono text-sm font-medium text-af-ink"
                  >
                    {{ platform }}
                  </label>
                  <p class="mt-0.5 text-xs text-af-ink-3">
                    {{
                      t(
                        "admin.settings.scheduling.accountSchedulingThresholdsRangeHint",
                      )
                    }}
                  </p>
                </div>
                <span
                  class="rounded bg-af-sunken px-2 py-0.5 text-[11px] font-medium text-af-ink-2"
                >
                  %
                </span>
              </div>
              <input
                v-model.number="form.account_scheduling_thresholds[platform]"
                type="number"
                min="1"
                max="100"
                step="1"
                class="input mt-3"
                :data-testid="`account-scheduling-threshold-${platform}`"
                placeholder="100"
              />
            </div>
          </div>
        </div>

        <!-- 利润门（全站一档；原来在分组上） -->
        <div class="border-t border-af-hairline pt-6">
          <div class="flex items-center justify-between">
            <div>
              <label class="text-sm font-medium text-af-ink-2">
                {{ t("admin.settings.profitControl.title") }}
              </label>
              <p class="text-xs text-af-ink-3">
                {{ t("admin.settings.profitControl.description") }}
              </p>
            </div>
            <Toggle
              v-model="form.profit_control_enabled"
              data-testid="profit-control-enabled"
            />
          </div>
          <div v-if="form.profit_control_enabled" class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t("admin.settings.profitControl.minMargin") }}</label>
              <input
                v-model.number="form.profit_min_margin"
                type="number"
                min="0"
                max="0.99"
                step="0.01"
                class="input"
                data-testid="profit-control-min-margin"
              />
            </div>
            <div>
              <label class="input-label">{{ t("admin.settings.profitControl.safetyBuffer") }}</label>
              <input
                v-model.number="form.profit_safety_buffer"
                type="number"
                min="0"
                max="0.99"
                step="0.01"
                class="input"
                data-testid="profit-control-safety-buffer"
              />
            </div>
            <p class="text-xs text-af-ink-3 sm:col-span-2">
              {{ t("admin.settings.profitControl.hint") }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 系统设置 › cooldown（A6 从 SettingsView 拆出，卡片模板原样搬来；状态与逻辑在 useSettingsPage）
import Toggle from '@/components/common/Toggle.vue'
import { useSettingsPageContext } from '../useSettingsPage'

const {
  form,
  overloadCooldownForm,
  overloadCooldownLoading,
  overloadCooldownSaving,
  rateLimit429CooldownForm,
  rateLimit429CooldownLoading,
  rateLimit429CooldownSaving,
  saveOverloadCooldownSettings,
  saveRateLimit429CooldownSettings,
  saveStreamTimeoutSettings,
  schedulingThresholdPlatforms,
  streamTimeoutForm,
  streamTimeoutLoading,
  streamTimeoutSaving,
  t
} = useSettingsPageContext()
</script>
