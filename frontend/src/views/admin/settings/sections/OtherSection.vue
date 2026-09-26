<template>
  <div class="space-y-6">
    <!-- 利润门（全站一档；P4 从「重试与冷却」的调度卡片里单独拿出来） -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.profitControl.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.profitControl.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div class="flex items-center justify-between">
          <label class="text-sm font-medium text-af-ink-2">
            {{ t("admin.settings.profitControl.enabled") }}
          </label>
          <Toggle
            v-model="form.profit_control_enabled"
            data-testid="profit-control-enabled"
          />
        </div>
        <div v-if="form.profit_control_enabled" class="grid grid-cols-1 gap-4 sm:grid-cols-2">
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

    <!-- Web Search Emulation -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.webSearchEmulation.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.webSearchEmulation.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <!-- Global Toggle -->
        <div class="flex items-center justify-between">
          <div>
            <label
              class="text-sm font-medium text-af-ink-2"
            >
              {{ t("admin.settings.webSearchEmulation.enabled") }}
            </label>
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{ t("admin.settings.webSearchEmulation.enabledHint") }}
            </p>
          </div>
          <Toggle v-model="webSearchConfig.enabled" />
        </div>

        <!-- Providers -->
        <div v-if="webSearchConfig.enabled" class="space-y-4">
          <div class="flex items-center justify-between">
            <label
              class="text-sm font-medium text-af-ink-2"
            >
              {{ t("admin.settings.webSearchEmulation.providers") }}
            </label>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click="addWebSearchProvider"
            >
              {{ t("admin.settings.webSearchEmulation.addProvider") }}
            </button>
          </div>

          <div
            v-if="webSearchConfig.providers.length === 0"
            class="rounded-lg border border-dashed border-af-hairline-strong p-4 text-center text-sm text-af-ink-3"
          >
            {{ t("admin.settings.webSearchEmulation.noProviders") }}
          </div>

          <div
            v-for="(provider, pIdx) in webSearchConfig.providers"
            :key="pIdx"
            class="rounded-lg border border-af-hairline"
          >
            <!-- Collapsible header -->
            <div
              class="flex cursor-pointer items-center justify-between px-4 py-3"
              @click="toggleProviderExpand(pIdx)"
            >
              <div class="flex items-center gap-3">
                <svg
                  class="h-4 w-4 text-af-ink-3 transition-transform"
                  :class="{ 'rotate-90': expandedProviders[pIdx] }"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 5l7 7-7 7"
                  />
                </svg>
                <Select
                  v-model="provider.type"
                  :options="[
                    { value: 'brave', label: 'Brave Search' },
                    { value: 'tavily', label: 'Tavily' },
                  ]"
                  class="w-36"
                  @click.stop
                />
                <!-- Quota summary (always visible) -->
                <span class="text-xs text-af-ink-3">
                  {{ provider.quota_used ?? 0 }} /
                  {{
                    provider.quota_limit != null &&
                    provider.quota_limit > 0
                      ? provider.quota_limit
                      : "∞"
                  }}
                </span>
                <span
                  v-if="
                    !expandedProviders[pIdx] &&
                    provider.api_key_configured
                  "
                  class="text-xs text-af-success"
                >
                  {{
                    t(
                      "admin.settings.webSearchEmulation.apiKeyConfigured",
                    )
                  }}
                </span>
              </div>
              <button
                type="button"
                class="text-af-danger hover:text-af-danger text-xs"
                @click.stop="removeWebSearchProvider(pIdx)"
              >
                {{
                  t("admin.settings.webSearchEmulation.removeProvider")
                }}
              </button>
            </div>

            <!-- Expanded content -->
            <div
              v-if="expandedProviders[pIdx]"
              class="space-y-3 border-t border-af-hairline px-4 pb-4 pt-3"
            >
              <!-- API Key with inline show/copy -->
              <div>
                <label class="text-xs text-af-ink-3">{{
                  t("admin.settings.webSearchEmulation.apiKey")
                }}</label>
                <div class="relative">
                  <input
                    v-model="provider.api_key"
                    :type="apiKeyVisible[pIdx] ? 'text' : 'password'"
                    class="input w-full text-sm"
                    :class="
                      provider.api_key || provider.api_key_configured
                        ? 'pr-16'
                        : ''
                    "
                    :placeholder="
                      provider.api_key_configured
                        ? '••••••••'
                        : t(
                            'admin.settings.webSearchEmulation.apiKeyPlaceholder',
                          )
                    "
                  />
                  <div
                    v-if="provider.api_key || provider.api_key_configured"
                    class="absolute inset-y-0 right-0 flex items-center pr-1.5"
                  >
                    <button
                      type="button"
                      class="rounded p-1 text-af-ink-3 hover:text-af-ink-2"
                      :title="
                        apiKeyVisible[pIdx]
                          ? t(
                              'admin.settings.webSearchEmulation.hideApiKey',
                            )
                          : t(
                              'admin.settings.webSearchEmulation.showApiKey',
                            )
                      "
                      @click="apiKeyVisible[pIdx] = !apiKeyVisible[pIdx]"
                    >
                      <svg
                        v-if="!apiKeyVisible[pIdx]"
                        class="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                        />
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                        />
                      </svg>
                      <svg
                        v-else
                        class="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21"
                        />
                      </svg>
                    </button>
                    <button
                      type="button"
                      class="rounded p-1 text-af-ink-3 hover:text-af-ink-2"
                      :class="{
                        'opacity-30 cursor-not-allowed':
                          !provider.api_key,
                      }"
                      :title="
                        t('admin.settings.webSearchEmulation.copyApiKey')
                      "
                      :disabled="!provider.api_key"
                      @click="copyApiKey(pIdx)"
                    >
                      <svg
                        class="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
              </div>

              <!-- Quota + Subscription in compact row -->
              <div class="grid grid-cols-2 gap-3">
                <div>
                  <label class="text-xs text-af-ink-3">{{
                    t("admin.settings.webSearchEmulation.quotaLimit")
                  }}</label>
                  <input
                    v-model="provider.quota_limit"
                    type="number"
                    min="1"
                    class="input text-sm"
                    :placeholder="'∞'"
                  />
                  <p class="mt-0.5 text-xs text-af-ink-3">
                    {{
                      t(
                        "admin.settings.webSearchEmulation.quotaLimitHint",
                      )
                    }}
                  </p>
                </div>
                <div>
                  <label class="text-xs text-af-ink-3">{{
                    t("admin.settings.webSearchEmulation.subscribedAt")
                  }}</label>
                  <input
                    :value="formatSubscribedAt(provider.subscribed_at)"
                    type="date"
                    class="input text-sm"
                    @input="
                      provider.subscribed_at = parseSubscribedAt(
                        ($event.target as HTMLInputElement).value,
                      )
                    "
                  />
                  <p class="mt-0.5 text-xs text-af-ink-3">
                    {{
                      t(
                        "admin.settings.webSearchEmulation.subscribedAtHint",
                      )
                    }}
                  </p>
                </div>
              </div>

              <!-- Usage display -->
              <div class="flex items-center gap-2">
                <span class="text-xs text-af-ink-3"
                  >{{
                    t("admin.settings.webSearchEmulation.quotaUsage")
                  }}:</span
                >
                <div
                  v-if="
                    provider.quota_limit != null &&
                    provider.quota_limit > 0
                  "
                  class="flex-1 rounded-full bg-af-hairline"
                  style="height: 6px"
                >
                  <div
                    class="h-full rounded-full transition-all"
                    :class="
                      quotaPercentage(provider) > 90
                        ? 'bg-af-danger'
                        : quotaPercentage(provider) > 70
                          ? 'bg-af-warning'
                          : 'bg-af-success'
                    "
                    :style="{
                      width:
                        Math.min(quotaPercentage(provider), 100) + '%',
                    }"
                  />
                </div>
                <div v-else class="flex-1" />
                <span class="text-xs text-af-ink-3"
                  >{{ provider.quota_used ?? 0 }} /
                  {{
                    provider.quota_limit != null &&
                    provider.quota_limit > 0
                      ? provider.quota_limit
                      : "∞"
                  }}</span
                >
                <button
                  v-if="(provider.quota_used ?? 0) > 0"
                  type="button"
                  class="text-xs text-af-brand hover:text-af-brand-hover"
                  @click="resetWebSearchUsage(pIdx)"
                >
                  {{ t("admin.settings.webSearchEmulation.resetUsage") }}
                </button>
              </div>

              <!-- Proxy + Test on same row -->
              <div class="flex items-end gap-3">
                <div class="flex-1">
                  <label class="text-xs text-af-ink-3">{{
                    t("admin.settings.webSearchEmulation.proxy")
                  }}</label>
                  <ProxySelector
                    v-model="provider.proxy_id"
                    :proxies="webSearchProxies"
                  />
                </div>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm whitespace-nowrap"
                  @click="openTestDialog()"
                >
                  {{ t("admin.settings.webSearchEmulation.test") }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Web Search Test Dialog -->
    <div
      v-if="wsTestDialogOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="wsTestDialogOpen = false"
    >
      <div
        class="mx-4 w-full max-w-lg rounded-xl bg-af-sheet p-6 shadow-xl"
      >
        <h3
          class="mb-4 text-lg font-semibold text-af-ink"
        >
          {{ t("admin.settings.webSearchEmulation.testResultTitle") }}
        </h3>
        <div class="flex items-center gap-2">
          <input
            v-model="wsTestQuery"
            type="text"
            class="input flex-1 text-sm"
            :placeholder="
              t('admin.settings.webSearchEmulation.testDefaultQuery')
            "
            @keyup.enter="testWebSearchProvider()"
          />
          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="wsTestLoading"
            @click="testWebSearchProvider()"
          >
            {{
              wsTestLoading
                ? t("admin.settings.webSearchEmulation.testing")
                : t("admin.settings.webSearchEmulation.test")
            }}
          </button>
        </div>
        <!-- Test results -->
        <div
          v-if="wsTestResult"
          class="mt-4 max-h-80 overflow-y-auto rounded-lg bg-af-sunken p-4"
        >
          <p
            class="mb-2 text-sm font-medium text-af-ink-2"
          >
            {{
              t("admin.settings.webSearchEmulation.testResultProvider")
            }}: {{ wsTestResult.provider }}
          </p>
          <div
            v-if="wsTestResult.results.length === 0"
            class="text-sm text-af-ink-3"
          >
            {{ t("admin.settings.webSearchEmulation.testNoResults") }}
          </div>
          <div
            v-for="(r, rIdx) in wsTestResult.results"
            :key="rIdx"
            class="mt-2 border-t border-af-hairline pt-2 first:mt-0 first:border-0 first:pt-0"
          >
            <a
              :href="r.url"
              target="_blank"
              class="text-sm font-medium text-af-ink-2 hover:underline"
              >{{ r.title }}</a
            >
            <p class="mt-0.5 text-xs text-af-ink-3">
              {{ r.snippet }}
            </p>
          </div>
        </div>
        <div class="mt-4 flex justify-end">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            @click="wsTestDialogOpen = false"
          >
            {{ t("common.close") }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 系统设置 › 其它（上线收口 P4，2026-09-27）：网关的冷却、转发、客户端、上游探测都写进后端代码，
// 这里只剩两张卡片——利润门（原在「重试与冷却」）与联网搜索模拟（原在「转发行为」），模板原样搬来；
// 状态与逻辑在 useSettingsPage：利润门随总表单保存，联网搜索模拟走自己的接口、在总表单保存后一起保存。
import ProxySelector from '@/components/common/ProxySelector.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useSettingsPageContext } from '../useSettingsPage'

const {
  addWebSearchProvider,
  apiKeyVisible,
  copyApiKey,
  expandedProviders,
  form,
  formatSubscribedAt,
  openTestDialog,
  parseSubscribedAt,
  quotaPercentage,
  removeWebSearchProvider,
  resetWebSearchUsage,
  t,
  testWebSearchProvider,
  toggleProviderExpand,
  webSearchConfig,
  webSearchProxies,
  wsTestDialogOpen,
  wsTestLoading,
  wsTestQuery,
  wsTestResult
} = useSettingsPageContext()
</script>
