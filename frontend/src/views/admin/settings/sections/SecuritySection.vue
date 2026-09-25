<template>
  <div class="space-y-6">
    <!-- Admin API Key Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.adminApiKey.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.adminApiKey.description") }}
        </p>
      </div>
      <div class="space-y-4 p-6">
        <!-- Security Warning -->
        <div
          class="rounded-lg border border-af-warning/30 bg-af-warning-tint p-4"
        >
          <div class="flex items-start">
            <Icon
              name="exclamationTriangle"
              size="md"
              class="mt-0.5 flex-shrink-0 text-af-warning"
            />
            <p class="ml-3 text-sm text-af-warning">
              {{ t("admin.settings.adminApiKey.securityWarning") }}
            </p>
          </div>
        </div>

        <!-- Loading State -->
        <div
          v-if="adminApiKeyLoading"
          class="flex items-center gap-2 text-af-ink-3"
        >
          <div
            class="h-4 w-4 animate-spin rounded-full border-b-2 border-af-brand"
          ></div>
          {{ t("common.loading") }}
        </div>

        <!-- No Key Configured -->
        <div
          v-else-if="!adminApiKeyExists"
          class="flex items-center justify-between"
        >
          <span class="text-af-ink-3">
            {{ t("admin.settings.adminApiKey.notConfigured") }}
          </span>
          <button
            type="button"
            @click="createAdminApiKey"
            :disabled="adminApiKeyOperating"
            class="btn btn-primary btn-sm"
          >
            <svg
              v-if="adminApiKeyOperating"
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
              adminApiKeyOperating
                ? t("admin.settings.adminApiKey.creating")
                : t("admin.settings.adminApiKey.create")
            }}
          </button>
        </div>

        <!-- Key Exists -->
        <div v-else class="space-y-4">
          <div class="flex items-center justify-between">
            <div>
              <label
                class="mb-1 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.adminApiKey.currentKey") }}
              </label>
              <code
                class="rounded bg-af-sunken px-2 py-1 font-mono text-sm text-af-ink"
              >
                {{ adminApiKeyMasked }}
              </code>
            </div>
            <div class="flex gap-2">
              <button
                type="button"
                @click="regenerateAdminApiKey"
                :disabled="adminApiKeyOperating"
                class="btn btn-secondary btn-sm"
              >
                {{
                  adminApiKeyOperating
                    ? t("admin.settings.adminApiKey.regenerating")
                    : t("admin.settings.adminApiKey.regenerate")
                }}
              </button>
              <button
                type="button"
                @click="deleteAdminApiKey"
                :disabled="adminApiKeyOperating"
                class="btn btn-secondary btn-sm text-af-danger hover:text-af-danger"
              >
                {{ t("admin.settings.adminApiKey.delete") }}
              </button>
            </div>
          </div>

          <!-- Newly Generated Key Display -->
          <div
            v-if="newAdminApiKey"
            class="space-y-3 rounded-lg border border-af-success/30 bg-af-success-tint p-4"
          >
            <p
              class="text-sm font-medium text-af-success"
            >
              {{ t("admin.settings.adminApiKey.keyWarning") }}
            </p>
            <div class="flex items-center gap-2">
              <code
                class="flex-1 select-all break-all rounded border border-af-success/30 bg-af-sheet px-3 py-2 font-mono text-sm"
              >
                {{ newAdminApiKey }}
              </code>
              <button
                type="button"
                @click="copyNewKey"
                class="btn btn-primary btn-sm flex-shrink-0"
              >
                {{ t("admin.settings.adminApiKey.copyKey") }}
              </button>
            </div>
            <p class="text-xs text-af-success">
              {{ t("admin.settings.adminApiKey.usage") }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- API Key IP ACL Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.apiKeyAcl.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.apiKeyAcl.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="font-medium text-af-ink">
              {{ t("admin.settings.apiKeyAcl.trustForwardedIp") }}
            </label>
            <p class="text-sm text-af-ink-3">
              {{ t("admin.settings.apiKeyAcl.trustForwardedIpHint") }}
            </p>
          </div>
          <Toggle v-model="form.api_key_acl_trust_forwarded_ip" />
        </div>

        <div
          v-if="form.api_key_acl_trust_forwarded_ip"
          class="border-t border-af-hairline pt-4"
        >
          <label
            for="forwarded-client-ip-headers"
            class="font-medium text-af-ink"
          >
            {{ t("admin.settings.apiKeyAcl.forwardedClientIpHeaders") }}
          </label>
          <p class="mt-1 text-sm text-af-ink-3">
            {{ t("admin.settings.apiKeyAcl.forwardedClientIpHeadersHint") }}
          </p>
          <div
            class="mt-3 rounded-lg border border-af-hairline-strong bg-af-sheet p-2"
          >
            <div class="flex flex-wrap items-center gap-2">
              <span
                v-for="header in form.forwarded_client_ip_headers"
                :key="header"
                data-testid="forwarded-client-ip-header-tag"
                class="inline-flex items-center gap-1 rounded bg-af-sunken px-2 py-1 text-xs font-mono text-af-ink-2"
              >
                <span>{{ header }}</span>
                <button
                  type="button"
                  class="rounded-full text-af-ink-3 hover:bg-af-hairline hover:text-af-ink-2"
                  :aria-label="t('admin.settings.apiKeyAcl.removeForwardedClientIpHeader', { header })"
                  @click="removeForwardedClientIpHeader(header)"
                >
                  <Icon
                    name="x"
                    size="xs"
                    class="h-3.5 w-3.5"
                    :stroke-width="2"
                  />
                </button>
              </span>
              <div
                class="flex min-w-[220px] flex-1 items-center gap-1 rounded border border-transparent px-2 py-1 focus-within:border-af-hairline-strong"
              >
                <input
                  id="forwarded-client-ip-headers"
                  v-model="forwardedClientIpHeaderDraft"
                  data-testid="forwarded-client-ip-headers-input"
                  type="text"
                  class="w-full bg-transparent text-sm font-mono text-af-ink outline-none placeholder:text-af-ink-3"
                  :placeholder="t('admin.settings.apiKeyAcl.forwardedClientIpHeadersPlaceholder')"
                  @keydown="handleForwardedClientIpHeaderKeydown"
                  @blur="commitForwardedClientIpHeaderDraft"
                  @paste="handleForwardedClientIpHeaderPaste"
                />
              </div>
            </div>
          </div>
          <p class="mt-2 text-xs text-af-ink-3">
            {{ t("admin.settings.apiKeyAcl.forwardedClientIpHeadersRiskHint") }}
          </p>
        </div>
      </div>
    </div>

    <!-- Panel API Rate Limit Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <div class="flex items-center gap-2">
          <Icon
            name="shield"
            size="md"
            class="text-af-brand"
          />
          <h2 class="text-lg font-semibold text-af-ink">
            {{ t("admin.settings.panelRateLimit.title") }}
          </h2>
        </div>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.panelRateLimit.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div
          v-if="panelRateLimitLoading"
          class="flex items-center gap-2 text-af-ink-3"
        >
          <div
            class="h-4 w-4 animate-spin rounded-full border-b-2 border-af-brand"
          ></div>
          {{ t("common.loading") }}
        </div>

        <template v-else>
          <!-- 计数维度说明：按账号计数，反代部署无误伤 -->
          <div
            class="rounded-lg border border-af-hairline bg-af-sunken p-4"
          >
            <div class="flex items-start">
              <Icon
                name="infoCircle"
                size="md"
                class="mt-0.5 flex-shrink-0 text-af-ink-2"
              />
              <p class="ml-3 text-sm text-af-ink-2">
                {{ t("admin.settings.panelRateLimit.proxySafeNote") }}
              </p>
            </div>
          </div>

          <div class="flex items-center justify-between">
            <div>
              <label class="font-medium text-af-ink">{{
                t("admin.settings.panelRateLimit.enabled")
              }}</label>
              <p class="text-sm text-af-ink-3">
                {{ t("admin.settings.panelRateLimit.enabledHint") }}
              </p>
            </div>
            <Toggle v-model="panelRateLimitForm.enabled" />
          </div>

          <div
            v-if="panelRateLimitForm.enabled"
            class="space-y-5 border-t border-af-hairline pt-4"
          >
            <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
              <div>
                <label
                  class="mb-2 block text-sm font-medium text-af-ink-2"
                >
                  {{ t("admin.settings.panelRateLimit.userRpm") }}
                </label>
                <div class="flex items-center gap-2">
                  <input
                    v-model.number="panelRateLimitForm.user_rpm"
                    data-testid="panel-rate-limit-user-rpm"
                    type="number"
                    min="0"
                    max="100000"
                    class="input w-32"
                  />
                  <span class="text-sm text-af-ink-3">
                    {{ t("admin.settings.panelRateLimit.perMinute") }}
                  </span>
                </div>
                <p class="mt-1.5 text-xs text-af-ink-3">
                  {{ t("admin.settings.panelRateLimit.userRpmHint") }}
                </p>
              </div>

              <div>
                <label
                  class="mb-2 block text-sm font-medium text-af-ink-2"
                >
                  {{ t("admin.settings.panelRateLimit.heavyRpm") }}
                </label>
                <div class="flex items-center gap-2">
                  <input
                    v-model.number="panelRateLimitForm.heavy_rpm"
                    type="number"
                    min="0"
                    max="100000"
                    class="input w-32"
                  />
                  <span class="text-sm text-af-ink-3">
                    {{ t("admin.settings.panelRateLimit.perMinute") }}
                  </span>
                </div>
                <p class="mt-1.5 text-xs text-af-ink-3">
                  {{ t("admin.settings.panelRateLimit.heavyRpmHint") }}
                </p>
              </div>

              <div>
                <label
                  class="mb-2 block text-sm font-medium text-af-ink-2"
                >
                  {{ t("admin.settings.panelRateLimit.publicIpRpm") }}
                </label>
                <div class="flex items-center gap-2">
                  <input
                    v-model.number="panelRateLimitForm.public_ip_rpm"
                    type="number"
                    min="0"
                    max="100000"
                    class="input w-32"
                  />
                  <span class="text-sm text-af-ink-3">
                    {{ t("admin.settings.panelRateLimit.perMinute") }}
                  </span>
                </div>
                <p class="mt-1.5 text-xs text-af-ink-3">
                  {{ t("admin.settings.panelRateLimit.publicIpRpmHint") }}
                </p>
              </div>
            </div>

            <div
              class="flex items-center justify-between border-t border-af-hairline pt-4"
            >
              <div>
                <label class="font-medium text-af-ink">{{
                  t("admin.settings.panelRateLimit.exemptAdmin")
                }}</label>
                <p class="text-sm text-af-ink-3">
                  {{ t("admin.settings.panelRateLimit.exemptAdminHint") }}
                </p>
              </div>
              <Toggle v-model="panelRateLimitForm.exempt_admin" />
            </div>
          </div>

          <div
            class="flex justify-end border-t border-af-hairline pt-4"
          >
            <button
              type="button"
              data-testid="panel-rate-limit-save"
              @click="savePanelRateLimitSettings"
              :disabled="panelRateLimitSaving"
              class="btn btn-primary btn-sm"
            >
              <svg
                v-if="panelRateLimitSaving"
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
                panelRateLimitSaving
                  ? t("common.saving")
                  : t("common.save")
              }}
            </button>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 系统设置 › security（A6 从 SettingsView 拆出，卡片模板原样搬来；状态与逻辑在 useSettingsPage）
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useSettingsPageContext } from '../useSettingsPage'

const {
  adminApiKeyExists,
  adminApiKeyLoading,
  adminApiKeyMasked,
  adminApiKeyOperating,
  commitForwardedClientIpHeaderDraft,
  copyNewKey,
  createAdminApiKey,
  deleteAdminApiKey,
  form,
  forwardedClientIpHeaderDraft,
  handleForwardedClientIpHeaderKeydown,
  handleForwardedClientIpHeaderPaste,
  newAdminApiKey,
  panelRateLimitForm,
  panelRateLimitLoading,
  panelRateLimitSaving,
  regenerateAdminApiKey,
  removeForwardedClientIpHeader,
  savePanelRateLimitSettings,
  t
} = useSettingsPageContext()
</script>
