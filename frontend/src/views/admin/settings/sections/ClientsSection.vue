<template>
  <div class="space-y-6">
    <!-- Claude Code Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.claudeCode.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.claudeCode.description") }}
        </p>
      </div>
      <div class="p-6">
        <div>
          <label
            class="mb-2 block text-sm font-medium text-af-ink-2"
          >
            {{ t("admin.settings.claudeCode.minVersion") }}
          </label>
          <input
            v-model="form.min_claude_code_version"
            type="text"
            class="input max-w-xs font-mono text-sm"
            :placeholder="
              t('admin.settings.claudeCode.minVersionPlaceholder')
            "
          />
          <p class="mt-1.5 text-xs text-af-ink-3">
            {{ t("admin.settings.claudeCode.minVersionHint") }}
          </p>
        </div>
        <div class="mt-4">
          <label
            class="mb-2 block text-sm font-medium text-af-ink-2"
          >
            {{ t("admin.settings.claudeCode.maxVersion") }}
          </label>
          <input
            v-model="form.max_claude_code_version"
            type="text"
            class="input max-w-xs font-mono text-sm"
            :placeholder="
              t('admin.settings.claudeCode.maxVersionPlaceholder')
            "
          />
          <p class="mt-1.5 text-xs text-af-ink-3">
            {{ t("admin.settings.claudeCode.maxVersionHint") }}
          </p>
        </div>
      </div>
    </div>

    <!-- Codex Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.gatewayForwarding.codexHardeningTitle") }}
        </h2>
      </div>
      <div class="p-6 space-y-4">
          <div>
            <h3 class="text-base font-semibold text-af-ink">
              {{ t("admin.settings.gatewayForwarding.codexClientRestrictionTitle") }}
            </h3>
            <p class="mt-1 text-sm text-af-ink-3">
              {{ t("admin.settings.gatewayForwarding.codexHardeningDesc") }}
            </p>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.gatewayForwarding.minCodexVersion") }}
              </label>
              <input
                v-model="form.min_codex_version"
                type="text"
                class="input w-full font-mono text-sm"
                :placeholder="
                  t(
                    'admin.settings.gatewayForwarding.minCodexVersionPlaceholder',
                  )
                "
              />
            </div>
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.gatewayForwarding.maxCodexVersion") }}
              </label>
              <input
                v-model="form.max_codex_version"
                type="text"
                class="input w-full font-mono text-sm"
                :placeholder="
                  t(
                    'admin.settings.gatewayForwarding.maxCodexVersionPlaceholder',
                  )
                "
              />
            </div>
          </div>
          <p class="text-xs text-af-ink-3">
            {{ t("admin.settings.gatewayForwarding.codexVersionHint") }}
          </p>

          <div>
            <label class="block text-sm font-medium text-af-ink-2">
              {{ t("admin.settings.gatewayForwarding.codexFingerprintSignals") }}
            </label>
            <p class="mb-2 mt-1 text-xs text-af-ink-3">
              {{ t("admin.settings.gatewayForwarding.codexFingerprintSignalsDesc") }}
            </p>
            <div
              v-for="(row, i) in codexFingerprintRows"
              :key="`codex-fp-${i}`"
              class="mb-2 flex items-center gap-2"
            >
              <select v-model="row.type" class="input w-32 text-sm">
                <option value="header_exact">{{ t("admin.settings.gatewayForwarding.codexFpTypeHeaderExact") }}</option>
                <option value="header_prefix">{{ t("admin.settings.gatewayForwarding.codexFpTypeHeaderPrefix") }}</option>
                <option value="body_path">{{ t("admin.settings.gatewayForwarding.codexFpTypeBodyPath") }}</option>
              </select>
              <input
                v-model="row.match"
                type="text"
                class="input flex-1 font-mono text-sm"
                :placeholder="t('admin.settings.gatewayForwarding.codexFpMatchPlaceholder')"
              />
              <label class="flex shrink-0 items-center gap-1 text-xs text-af-ink-2">
                <input v-model="row.required" type="checkbox" />
                {{ t("admin.settings.gatewayForwarding.codexFpRequired") }}
              </label>
              <button
                type="button"
                class="btn btn-secondary btn-sm shrink-0 text-af-danger hover:text-af-danger"
                @click="removeCodexFingerprintRow(i)"
              >
                {{ t("admin.settings.gatewayForwarding.codexRemoveRow") }}
              </button>
            </div>
            <button type="button" class="btn btn-secondary btn-sm" @click="addCodexFingerprintRow">
              {{ t("admin.settings.gatewayForwarding.codexAddRow") }}
            </button>
            <p
              v-if="codexFingerprintNoRequired"
              class="mt-2 text-xs text-af-warning"
            >
              {{ t("admin.settings.gatewayForwarding.codexFingerprintNoRequiredWarn") }}
            </p>
          </div>

          <div class="flex items-center justify-between">
            <div class="pr-4">
              <label
                class="block text-sm font-medium text-af-ink-2"
              >
                {{
                  t("admin.settings.gatewayForwarding.codexAllowAppServer")
                }}
              </label>
              <p class="mt-1 text-xs text-af-ink-3">
                {{
                  t(
                    "admin.settings.gatewayForwarding.codexAllowAppServerDesc",
                  )
                }}
              </p>
            </div>
            <Toggle
              v-model="form.codex_cli_only_allow_app_server_clients"
            />
          </div>

          <div>
            <label
              class="block text-sm font-medium text-af-ink-2"
            >
              {{ t("admin.settings.gatewayForwarding.codexBlacklist") }}
            </label>
            <p class="mb-2 mt-1 text-xs text-af-ink-3">
              {{ t("admin.settings.gatewayForwarding.codexBlacklistDesc") }}
            </p>
            <div
              v-for="(row, i) in codexBlacklistRows"
              :key="`codex-bl-${i}`"
              class="mb-2 flex gap-2"
            >
              <input
                v-model="row.originator"
                type="text"
                class="input w-1/3 font-mono text-sm"
                :placeholder="
                  t(
                    'admin.settings.gatewayForwarding.codexOriginatorPlaceholder',
                  )
                "
              />
              <input
                v-model="row.uaContains"
                type="text"
                class="input flex-1 font-mono text-sm"
                :placeholder="
                  t(
                    'admin.settings.gatewayForwarding.codexUaContainsPlaceholder',
                  )
                "
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm shrink-0 text-af-danger hover:text-af-danger"
                @click="removeCodexBlacklistRow(i)"
              >
                {{ t("admin.settings.gatewayForwarding.codexRemoveRow") }}
              </button>
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click="addCodexBlacklistRow"
            >
              {{ t("admin.settings.gatewayForwarding.codexAddRow") }}
            </button>
          </div>

          <div>
            <label
              class="block text-sm font-medium text-af-ink-2"
            >
              {{ t("admin.settings.gatewayForwarding.codexWhitelist") }}
            </label>
            <p class="mb-2 mt-1 text-xs text-af-ink-3">
              {{ t("admin.settings.gatewayForwarding.codexWhitelistDesc") }}
            </p>
            <div
              v-for="(row, i) in codexWhitelistRows"
              :key="`codex-wl-${i}`"
              class="mb-2 flex gap-2"
            >
              <input
                v-model="row.originator"
                type="text"
                class="input w-1/3 font-mono text-sm"
                :placeholder="
                  t(
                    'admin.settings.gatewayForwarding.codexOriginatorPlaceholder',
                  )
                "
              />
              <input
                v-model="row.uaContains"
                type="text"
                class="input flex-1 font-mono text-sm"
                :placeholder="
                  t(
                    'admin.settings.gatewayForwarding.codexUaContainsPlaceholder',
                  )
                "
              />
              <label
                class="flex shrink-0 items-center gap-1 text-xs text-af-ink-2"
                :title="
                  t(
                    'admin.settings.gatewayForwarding.codexWhitelistSkipFingerprintTooltip',
                  )
                "
              >
                <input
                  v-model="row.skipEngineFingerprint"
                  type="checkbox"
                />
                {{
                  t(
                    'admin.settings.gatewayForwarding.codexWhitelistSkipFingerprint',
                  )
                }}
              </label>
              <button
                type="button"
                class="btn btn-secondary btn-sm shrink-0 text-af-danger hover:text-af-danger"
                @click="removeCodexWhitelistRow(i)"
              >
                {{ t("admin.settings.gatewayForwarding.codexRemoveRow") }}
              </button>
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click="addCodexWhitelistRow"
            >
              {{ t("admin.settings.gatewayForwarding.codexAddRow") }}
            </button>
          </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 系统设置 › clients（A6 从 SettingsView 拆出，卡片模板原样搬来；状态与逻辑在 useSettingsPage）
import Toggle from '@/components/common/Toggle.vue'
import { useSettingsPageContext } from '../useSettingsPage'

const {
  addCodexBlacklistRow,
  addCodexFingerprintRow,
  addCodexWhitelistRow,
  codexBlacklistRows,
  codexFingerprintNoRequired,
  codexFingerprintRows,
  codexWhitelistRows,
  form,
  removeCodexBlacklistRow,
  removeCodexFingerprintRow,
  removeCodexWhitelistRow,
  t
} = useSettingsPageContext()
</script>
