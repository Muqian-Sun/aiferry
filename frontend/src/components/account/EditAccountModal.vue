<template>
  <!-- /accounts/:id/edit 整页（A5 起不再有弹窗形态）：分区导航读下面的 FormSectionHeading -->
  <FormPageShell :show="show" :title="t('admin.accounts.editAccount')" @close="handleClose">
    <form
      v-if="account"
      id="edit-account-form"
      @submit.prevent="handleSubmit"
      class="space-y-5"
    >
      <FormSectionHeading section="basics" :title="t('admin.accounts.formPage.sections.basics')" />

      <div>
        <label class="input-label">{{ t('common.name') }}</label>
        <input v-model="form.name" type="text" required class="input" />
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.notes') }}</label>
        <textarea
          v-model="form.notes"
          rows="3"
          class="input"
          :placeholder="t('admin.accounts.notesPlaceholder')"
        ></textarea>
        <p class="input-hint">{{ t('admin.accounts.notesHint') }}</p>
      </div>

      <div class="border-t border-af-hairline pt-4">
        <div>
          <label class="input-label">{{ t('common.status') }}</label>
          <Select v-model="form.status" :options="statusOptions" />
        </div>

        <!-- 超量：Antigravity 成品号（OAuth）专属；第三方 key 按协议调度，没有这一项 -->
        <div v-if="account?.platform === 'antigravity' && account?.type === 'oauth'" class="flex items-center gap-2">
          <label class="flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              v-model="allowOverages"
              class="h-4 w-4 rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
            />
            <span class="text-sm font-medium text-af-ink-2">
              {{ t('admin.accounts.allowOverages') }}
            </span>
          </label>
          <div class="group relative">
            <span
              class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-af-hairline text-xs text-af-ink-3 hover:bg-af-ink-4"
            >
              ?
            </span>
            <div
              class="pointer-events-none absolute left-0 top-full z-[100] mt-1.5 w-72 rounded bg-af-ink px-3 py-2 text-xs text-af-on-brand opacity-0 transition-opacity group-hover:opacity-100"
            >
              {{ t('admin.accounts.allowOveragesTooltip') }}
              <div
                class="absolute bottom-full left-3 border-4 border-transparent border-b-af-ink-3"
              ></div>
            </div>
          </div>
        </div>
      </div>

      <!-- API Key 类型：计费方式、智谱团队版 ID、API Key -->
      <div v-if="account.type === 'apikey'" class="space-y-4">
        <!-- 按量 / Coding 套餐：与新建同一规则，地址分得出就不问；MiniMax 两种套餐同一个地址、智谱 Anthropic 同地址才要选 -->
        <div v-if="keyPlanNeedsChoice" data-testid="edit-key-plan-mode">
          <label class="input-label">{{ t('admin.accounts.cnProviders.accountMode.title') }}</label>
          <div class="mt-2 flex flex-wrap gap-2">
            <button
              v-for="mode in CN_PLAN_MODES"
              :key="mode"
              type="button"
              :class="[
                'rounded-lg border-2 px-3 py-1.5 text-xs transition-all',
                editAccountMode === mode
                  ? 'border-af-brand bg-af-brand-tint font-medium text-af-brand'
                  : 'border-af-hairline text-af-ink-2 hover:border-af-hairline-strong'
              ]"
              @click="editAccountMode = mode"
            >
              {{ t(`admin.accounts.cnProviders.accountMode.${mode}`) }}
            </button>
          </div>
          <p class="input-hint">{{ t(`admin.accounts.cnProviders.accountMode.${editAccountMode}Desc`) }}</p>
        </div>

        <!-- Zhipu 团队版 Coding Plan：组织/项目 ID（可选，填写后用量查询走团队版端点） -->
        <div v-if="keyVendor === 'zhipu' && keyAccountMode === 'coding'">
          <div class="flex items-center">
            <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.title') }}</label>
            <HelpTooltip trigger="click" width-class="w-80">
              <p class="mb-1 font-medium">{{ t('admin.accounts.cnProviders.zhipuTeam.help.title') }}</p>
              <ol class="list-decimal space-y-1 pl-4">
                <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step1') }}</li>
                <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step2') }}</li>
                <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step3') }}</li>
                <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step4') }}</li>
              </ol>
              <p class="mt-2 break-all rounded bg-black/20 p-1.5 font-mono text-[11px] leading-relaxed">
                {{ t('admin.accounts.cnProviders.zhipuTeam.help.example') }}
              </p>
            </HelpTooltip>
          </div>
          <div class="mt-2 grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.organization') }}</label>
              <input v-model="editZhipuOrganization" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.organizationPlaceholder')" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.project') }}</label>
              <input v-model="editZhipuProject" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.projectPlaceholder')" />
            </div>
          </div>
          <p class="input-hint mt-2">{{ t('admin.accounts.cnProviders.zhipuTeam.hint') }}</p>
        </div>

        <div>
          <label class="input-label">{{ t('admin.accounts.apiKey') }}</label>
          <input
            v-model="editApiKey"
            type="password"
            class="input font-mono"
            autocomplete="new-password"
            data-1p-ignore
            data-lpignore="true"
            data-bwignore="true"
            :placeholder="apiKeyValuePlaceholder"
          />
          <p class="input-hint">{{ t('admin.accounts.leaveEmptyToKeep') }}</p>
        </div>
      </div>

      <!-- Vertex Service Account：区域（Project ID 由后端从 Service Account JSON 里取） -->
      <div v-if="(account.platform === 'gemini' || account.platform === 'anthropic') && account.type === 'service_account'">
        <label class="input-label">Location</label>
        <select
          v-model="editVertexLocation"
          required
          class="input font-mono"
        >
          <optgroup
            v-for="group in VERTEX_LOCATION_OPTIONS"
            :key="group.label"
            :label="group.label"
          >
            <option
              v-for="option in group.options"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </optgroup>
        </select>
        <p class="input-hint">{{ t('admin.accounts.vertexLocationHint') }}</p>
        <p class="input-hint">{{ t('admin.accounts.vertexSaJsonEditHint') }}</p>
      </div>

      <!-- Bedrock 凭证（SigV4 与 API Key 两种模式） -->
      <div v-if="account.type === 'bedrock'" class="space-y-4">
        <!-- SigV4 fields -->
        <template v-if="!isBedrockAPIKeyMode">
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockAccessKeyId') }}</label>
            <input
              v-model="editBedrockAccessKeyId"
              type="text"
              class="input font-mono"
              placeholder="AKIA..."
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockSecretAccessKey') }}</label>
            <input
              v-model="editBedrockSecretAccessKey"
              type="password"
              class="input font-mono"
              :placeholder="t('admin.accounts.bedrockSecretKeyLeaveEmpty')"
            />
            <p class="input-hint">{{ t('admin.accounts.bedrockSecretKeyLeaveEmpty') }}</p>
          </div>
        </template>

        <!-- API Key field -->
        <div v-if="isBedrockAPIKeyMode">
          <label class="input-label">{{ t('admin.accounts.bedrockApiKeyInput') }}</label>
          <input
            v-model="editBedrockApiKeyValue"
            type="password"
            class="input font-mono"
            :placeholder="t('admin.accounts.bedrockApiKeyLeaveEmpty')"
          />
          <p class="input-hint">{{ t('admin.accounts.bedrockApiKeyLeaveEmpty') }}</p>
        </div>
      </div>

      <div
        v-if="account.platform === 'antigravity' && account.type === 'oauth'"
        class="border-t border-af-hairline pt-4"
      >
        <label class="input-label">{{ t('admin.accounts.antigravityProjectIdLabel') }}</label>
        <input
          v-model="antigravityProjectId"
          data-testid="antigravity-project-id-input"
          type="text"
          class="input font-mono"
          :placeholder="t('admin.accounts.antigravityProjectIdPlaceholder')"
        />
        <p class="input-hint">{{ t('admin.accounts.antigravityProjectIdHint') }}</p>
      </div>

      <div class="border-t border-af-hairline pt-4">
        <label class="input-label">{{ t('admin.accounts.expiresAt') }}</label>
        <input v-model="expiresAtInput" type="datetime-local" class="input" />
        <div class="mt-2 flex gap-2">
          <button type="button" class="btn btn-secondary btn-sm" @click="form.expires_at = getAccountExpiryTimestamp(1)">
            {{ t('payment.oneMonth') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="form.expires_at = getAccountExpiryTimestamp(12)">
            {{ t('payment.oneYear') }}
          </button>
        </div>
        <p class="input-hint">
          {{ t('admin.accounts.expiresAtHint') }}
          {{ t('admin.accounts.expiresAtTimezoneHint', { timezone: browserTimeZone }) }}
        </p>
      </div>

      <FormSectionHeading v-if="showEndpointSection" section="endpoint" :title="t('admin.accounts.formPage.sections.endpoint')" />

      <!-- API Key 类型的协议地址 / 预设 -->
      <div v-if="account.type === 'apikey'">
        <ProtocolEndpointsEditor
          v-model="editProtocolEndpoints"
          :protocols="UPSTREAM_PROTOCOLS"
          :official-endpoints="officialProtocolEndpoints"
          :defaults-load-failed="protocolDefaultsLoadFailed"
        />
        <CnBaseUrlPresets
          v-if="isCNApiKeyAccount && account.platform !== 'opencode_go'"
          class="mt-2"
          :platform="cnPresetPlatform"
          :mode="editAccountMode"
          @select="onCnPresetSelect"
        />
      </div>

      <div
        v-if="account?.type === 'apikey'"
        class="flex items-center justify-between gap-4 border-t border-af-hairline pt-4"
      >
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.upstreamBilling.autoProbe') }}</label>
          <p class="mt-1 text-xs text-af-ink-3">
            {{ t('admin.accounts.upstreamBilling.autoProbeHint') }}
          </p>
        </div>
        <Toggle
          :model-value="upstreamBillingAutoProbeEnabled"
          data-testid="upstream-billing-auto-probe"
          :aria-label="t('admin.accounts.upstreamBilling.autoProbe')"
          @update:model-value="handleUpstreamBillingAutoProbeChange"
        />
      </div>

      <!-- Bedrock 区域与全局推理 -->
      <div v-if="account.type === 'bedrock'" class="space-y-4">
        <!-- Shared: Region -->
        <div>
          <label class="input-label">{{ t('admin.accounts.bedrockRegion') }}</label>
          <input
            v-model="editBedrockRegion"
            type="text"
            class="input"
            placeholder="us-east-1"
          />
          <p class="input-hint">{{ t('admin.accounts.bedrockRegionHint') }}</p>
        </div>

        <!-- Shared: Force Global -->
        <div>
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="editBedrockForceGlobal"
              type="checkbox"
              class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
            />
            <span class="text-sm text-af-ink-2">{{ t('admin.accounts.bedrockForceGlobal') }}</span>
          </label>
          <p class="input-hint mt-1">{{ t('admin.accounts.bedrockForceGlobalHint') }}</p>
        </div>
      </div>

      <!-- 第三方 key 的 Anthropic 协议设置：配了 anthropic 协议地址才展示，不看平台标签 -->
      <div
        v-if="anthropicKeySettingsVisible"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.anthropic.apiKeyAuthScheme') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.anthropic.apiKeyAuthSchemeDesc') }}
            </p>
          </div>
          <select
            v-model="anthropicAPIKeyAuthScheme"
            data-testid="edit-anthropic-auth-scheme"
            class="input w-52 text-sm"
          >
            <option value="x_api_key">{{ t('admin.accounts.anthropic.apiKeyAuthSchemeXApiKey') }}</option>
            <option value="authorization_bearer">{{ t('admin.accounts.anthropic.apiKeyAuthSchemeBearer') }}</option>
          </select>
        </div>
      </div>

      <!-- Bedrock CC 兼容（Anthropic 协议上的 key 设置）：清理 Claude Code 专有字段并过滤 anthropic-beta，账号是唯一开关 -->
      <div
        v-if="anthropicKeySettingsVisible"
        data-testid="edit-bedrock-cc-compat"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.anthropic.bedrockCCCompat') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.anthropic.bedrockCCCompatDesc') }}
            </p>
          </div>
          <button
            type="button"
            data-testid="edit-bedrock-cc-compat-toggle"
            @click="bedrockCCCompatEnabled = !bedrockCCCompatEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              bedrockCCCompatEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                bedrockCCCompatEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <FormSectionHeading section="models" :title="t('admin.accounts.formPage.sections.models')" />

      <!-- 承接的模型（muqian 2026-09-25 渠道表单里直接绑定）：勾选变了，保存时整份写入绑定 -->
      <p
        v-if="catalogEntryIdsLoadFailed"
        class="text-sm text-af-warning"
        data-testid="edit-account-catalog-load-failed"
      >
        {{ t('admin.accounts.catalogEntries.loadBoundFailed') }}
      </p>
      <CatalogEntryPicker
        v-else-if="catalogEntryIdsLoaded"
        v-model="selectedCatalogEntryIds"
        :suggested-platform="account.platform"
      />

      <!-- 模型改名（可选）：只改名、不限定能接哪些模型，保存时带 model_mapping_rename_only（spark 影子账号除外） -->
      <ModelRenameEditor
        v-if="showModelRename"
        v-model="modelMappings"
        data-testid="edit-model-rename"
        class="border-t border-af-hairline pt-4"
        :presets="renamePresets"
        :extends-vendor-table="extendsVendorTable"
      >
        <template v-if="account.platform === 'antigravity'" #actions>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="isSyncingAntigravityUpstream || !account?.id"
            @click="syncAntigravityUpstreamModels"
          >
            {{ isSyncingAntigravityUpstream ? t('admin.accounts.syncUpstreamModelsLoading') : t('admin.accounts.syncUpstreamModels') }}
          </button>
        </template>
      </ModelRenameEditor>

      <FormSectionHeading section="limits" :title="t('admin.accounts.formPage.sections.limits')" />

      <div class="grid grid-cols-2 gap-4 lg:grid-cols-3">
        <div>
          <label class="input-label">{{ t('admin.accounts.concurrency') }}</label>
          <input v-model.number="form.concurrency" type="number" min="1" class="input"
            @input="form.concurrency = Math.max(1, form.concurrency || 1)" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.priority') }}</label>
          <input
            v-model.number="form.priority"
            type="number"
            min="1"
            class="input"
          />
          <p class="input-hint">{{ t('admin.accounts.priorityHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.billingRateMultiplier') }}</label>
          <input
            v-model.number="form.rate_multiplier"
            type="number"
            min="0"
            step="0.001"
            class="input disabled:cursor-not-allowed disabled:opacity-60"
            data-testid="account-rate-multiplier"
            :disabled="upstreamBillingRateSyncEnabled"
          />
          <p class="input-hint">
            {{
              t(
                upstreamBillingRateSyncEnabled
                  ? 'admin.accounts.upstreamBilling.syncRateManagedHint'
                  : 'admin.accounts.billingRateMultiplierHint'
              )
            }}
          </p>
        </div>
      </div>

      <!-- 同步上游倍率：原来塞在四列网格的倍率格里，说明被挤成窄条（A8）；改成网格下独占一行，左说明右开关 -->
      <div
        v-if="account?.type === 'apikey'"
        class="flex items-center justify-between gap-4"
      >
        <div class="min-w-0">
          <p class="text-sm font-medium text-af-ink">
            {{ t('admin.accounts.upstreamBilling.syncRate') }}
          </p>
          <p class="mt-1 text-xs text-af-ink-3">
            {{ t('admin.accounts.upstreamBilling.syncRateHint') }}
          </p>
        </div>
        <Toggle
          :model-value="upstreamBillingRateSyncEnabled"
          data-testid="upstream-billing-rate-sync"
          :aria-label="t('admin.accounts.upstreamBilling.syncRate')"
          @update:model-value="handleUpstreamBillingRateSyncChange"
        />
      </div>

      <!-- 配额控制 (Anthropic apikey/bedrock: 配额限制 + 亲和) -->
      <div
        v-if="account?.platform === 'anthropic' && (account?.type === 'apikey' || account?.type === 'bedrock')"
        class="border-t border-af-hairline pt-4 space-y-4"
      >
        <div class="mb-3">
          <h3 class="input-label mb-0 text-base font-semibold">{{ t('admin.accounts.quotaControl.title') }}</h3>
          <p class="mt-1 text-xs text-af-ink-3">
            {{ t('admin.accounts.quotaControl.hint') }}
          </p>
        </div>
        <QuotaLimitCard
          :totalLimit="editQuotaLimit"
          :dailyLimit="editQuotaDailyLimit"
          :weeklyLimit="editQuotaWeeklyLimit"
          @update:totalLimit="editQuotaLimit = $event"
          @update:dailyLimit="editQuotaDailyLimit = $event"
          @update:weeklyLimit="editQuotaWeeklyLimit = $event"
        />
      </div>
      <!-- 配额控制 (非 Anthropic apikey/bedrock) -->
      <div
        v-else-if="account?.type === 'apikey' || account?.type === 'bedrock'"
        class="border-t border-af-hairline pt-4 space-y-4"
      >
        <div class="mb-3">
          <h3 class="input-label mb-0 text-base font-semibold">{{ t('admin.accounts.quotaControl.title') }}</h3>
          <p class="mt-1 text-xs text-af-ink-3">
            {{ t('admin.accounts.quotaLimitHint') }}
          </p>
        </div>
        <QuotaLimitCard
          :totalLimit="editQuotaLimit"
          :dailyLimit="editQuotaDailyLimit"
          :weeklyLimit="editQuotaWeeklyLimit"
          @update:totalLimit="editQuotaLimit = $event"
          @update:dailyLimit="editQuotaDailyLimit = $event"
          @update:weeklyLimit="editQuotaWeeklyLimit = $event"
        />
      </div>

      <!-- 配额控制 (Anthropic OAuth/SetupToken: 会话 + RPM) -->
      <div
        v-if="account?.platform === 'anthropic' && (account?.type === 'oauth' || account?.type === 'setup-token')"
        class="border-t border-af-hairline pt-4 space-y-4"
      >
        <div class="mb-3">
          <h3 class="input-label mb-0 text-base font-semibold">{{ t('admin.accounts.quotaControl.title') }}</h3>
          <p class="mt-1 text-xs text-af-ink-3">
            {{ t('admin.accounts.quotaControl.hint') }}
          </p>
        </div>

        <!-- Session Limit -->
        <div class="rounded-lg border border-af-hairline p-4">
          <div class="mb-3 flex items-center justify-between">
            <div>
              <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.sessionLimit.label') }}</label>
              <p class="mt-1 text-xs text-af-ink-3">
                {{ t('admin.accounts.quotaControl.sessionLimit.hint') }}
              </p>
            </div>
            <button
              type="button"
              @click="sessionLimitEnabled = !sessionLimitEnabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                sessionLimitEnabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  sessionLimitEnabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="sessionLimitEnabled">
            <label class="input-label">{{ t('admin.accounts.quotaControl.sessionLimit.maxSessions') }}</label>
            <input
              v-model.number="maxSessions"
              type="number"
              min="1"
              step="1"
              class="input"
              :placeholder="t('admin.accounts.quotaControl.sessionLimit.maxSessionsPlaceholder')"
            />
            <p class="input-hint">{{ t('admin.accounts.quotaControl.sessionLimit.maxSessionsHint') }}</p>
          </div>
        </div>

        <!-- RPM Limit -->
        <div class="rounded-lg border border-af-hairline p-4">
          <div class="mb-3 flex items-center justify-between">
            <div>
              <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.rpmLimit.label') }}</label>
              <p class="mt-1 text-xs text-af-ink-3">
                {{ t('admin.accounts.quotaControl.rpmLimit.hint') }}
              </p>
            </div>
            <button
              type="button"
              @click="rpmLimitEnabled = !rpmLimitEnabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                rpmLimitEnabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  rpmLimitEnabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="rpmLimitEnabled" class="space-y-4">
            <div>
              <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpm') }}</label>
              <input
                v-model.number="baseRpm"
                type="number"
                min="1"
                max="1000"
                step="1"
                class="input"
                :placeholder="t('admin.accounts.quotaControl.rpmLimit.baseRpmPlaceholder')"
              />
              <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpmHint') }}</p>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="account?.platform === 'openai' && account?.type === 'oauth' && !isSparkShadow"
        class="space-y-4 border-t border-af-hairline pt-4"
        data-testid="auto-reset-credit-settings"
      >
        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <label class="input-label mb-0">{{ t('admin.accounts.autoResetCredit.title') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.autoResetCredit.hint') }}
            </p>
          </div>
          <button
            type="button"
            data-testid="auto-reset-credit-enabled"
            @click="autoResetCreditEnabled = !autoResetCreditEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              autoResetCreditEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                autoResetCreditEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <OllamaCloudUsageSettings
        v-if="account?.ollama_cloud_usage?.eligible"
        :account="account"
        @updated="handleOllamaCloudUsageUpdated"
      />

      <FormSectionHeading section="advanced" :title="t('admin.accounts.formPage.sections.advanced')" />

      <div v-if="!isSparkShadow">
        <label class="input-label">{{ t('admin.accounts.proxy') }}</label>
        <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
      </div>

      <!-- API Key 类型的池模式 -->
      <div v-if="account.type === 'apikey'" class="space-y-4">
        <!-- Pool Mode Section -->
        <div class="border-t border-af-hairline pt-4">
          <div class="mb-3 flex items-center justify-between">
            <div>
              <label class="input-label mb-0">{{ t('admin.accounts.poolMode') }}</label>
              <p class="mt-1 text-xs text-af-ink-3">
                {{ t('admin.accounts.poolModeHint') }}
              </p>
            </div>
            <button
              type="button"
              @click="poolModeEnabled = !poolModeEnabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                poolModeEnabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  poolModeEnabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          <div v-if="poolModeEnabled" class="rounded-lg bg-af-sunken p-3">
            <p class="text-xs text-af-ink-2">
              <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
              {{ t('admin.accounts.poolModeInfo') }}
            </p>
          </div>
        </div>
      </div>

      <!-- Header Override Section（任何第三方 key + Grok OAuth） -->
      <div
        v-if="headerOverrideCapable"
        data-testid="edit-header-override"
        class="border-t border-af-hairline pt-4"
      >
        <div class="mb-3 flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.headerOverride.title') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.headerOverride.hint') }}
            </p>
          </div>
        </div>

        <div class="space-y-3">
          <div class="rounded-lg bg-af-sunken p-3">
            <p class="text-xs text-af-ink-2">
              <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
              {{ t('admin.accounts.headerOverride.info') }}
            </p>
          </div>

          <HeaderOverrideEditor
            :rows="headerOverrideRows"
            @update:rows="headerOverrideRows = $event"
          />
        </div>
      </div>

      <!-- Intercept Warmup Requests (Anthropic/Antigravity) -->
      <div
        v-if="account?.platform === 'anthropic' || account?.platform === 'antigravity'"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{
              t('admin.accounts.interceptWarmupRequests')
            }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.interceptWarmupRequestsDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="interceptWarmupRequests = !interceptWarmupRequests"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              interceptWarmupRequests ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                interceptWarmupRequests ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

    </form>

    <template #footer>
      <div v-if="account" class="flex justify-end gap-3">
        <button @click="handleClose" type="button" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="edit-account-form"
          :disabled="submitting"
          class="btn btn-primary"
        >
          <svg
            v-if="submitting"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
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
          {{ submitting ? t('admin.accounts.updating') : t('common.update') }}
        </button>
      </div>
    </template>
  </FormPageShell>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'

import { adminAPI } from '@/api/admin'
import type {
  Account,
  Proxy,
  OllamaCloudUsageState,
  ProtocolEndpoints
} from '@/types'
import type { ProtocolDefaultsResponse } from '@/api/admin/accounts'
import FormPageShell from '@/components/admin/form/FormPageShell.vue'
import FormSectionHeading from '@/components/admin/form/FormSectionHeading.vue'
import Select from '@/components/common/Select.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import CatalogEntryPicker from '@/components/account/CatalogEntryPicker.vue'
import ModelRenameEditor from '@/components/account/ModelRenameEditor.vue'
import QuotaLimitCard from '@/components/account/QuotaLimitCard.vue'
import CnBaseUrlPresets from '@/components/account/CnBaseUrlPresets.vue'
import ProtocolEndpointsEditor from '@/components/account/ProtocolEndpointsEditor.vue'
import {
  UPSTREAM_PROTOCOLS,
  describeProtocolEndpointsIssue,
  currentProtocolOf,
  endpointsAfterDefaultsChange,
  hasAnthropicEndpoint,
  loadProtocolDefaults,
  protocolDefaultsFor,
  trimProtocolEndpoints,
  validateProtocolEndpoints
} from '@/components/account/protocolEndpoints'
import {
  VENDORS_WITH_CODING_PLAN,
  apiKeyPlaceholderFor,
  detectKeyVendor,
  keyAddressPresets,
  modeOfAddress
} from '@/components/account/keyAddress'
import HeaderOverrideEditor from '@/components/account/HeaderOverrideEditor.vue'
import OllamaCloudUsageSettings from '@/components/account/OllamaCloudUsageSettings.vue'
import {
  applyAntigravityProjectID,
  applyHeaderOverride,
  applyInterceptWarmup,
  isHeaderOverrideCapable,
  splitHeaderOverridesObject,
  validateHeaderOverrideRows,
  isCNProviderPlatform,
  HEADER_OVERRIDES_CREDENTIAL_KEY,
  type CnAccountMode,
  type CnBaseUrlPreset,
  type CnProviderPlatform,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import {
  formatDateTimeLocalInput,
  getBrowserTimeZone,
  parseDateTimeLocalInput
} from '@/utils/format'
import { getAccountExpiryTimestamp } from '@/components/account/accountExpiry'
import { VERTEX_LOCATION_OPTIONS } from '@/constants/account'
import {
  PLATFORMS_WITH_VENDOR_MODEL_TABLE,
  buildModelMappingObject,
  renamePresetsFor
} from '@/composables/useModelWhitelist'

interface Props {
  show: boolean
  account: Account | null
  proxies: Proxy[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  updated: [account: Account]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const browserTimeZone = getBrowserTimeZone()

// Spark 影子账号(parent_account_id 非空):代理恒继承母账号,不可独立编辑(外审 B/P1),
// 故隐藏代理选择器。
const isSparkShadow = computed(() => props.account?.parent_account_id != null)

const handleOllamaCloudUsageUpdated = (state: OllamaCloudUsageState) => {
  if (props.account) emit('updated', { ...props.account, ollama_cloud_usage: state })
}


// Model mapping type
interface ModelMapping {
  from: string
  to: string
}

// State
const submitting = ref(false)
const editApiKey = ref('')

// 国产厂商 / OpenCode 的第三方 key：地址下方给该厂商的常用地址预设（CnBaseUrlPresets）。
const isCNApiKeyAccount = computed(
  () =>
    props.account?.type === 'apikey' &&
    (isCNProviderPlatform(props.account.platform) || props.account.platform === 'opencode_go')
)
// CnBaseUrlPresets 的 platform prop 是平台字面量联合类型，模板里不能写
// `as` 断言（其中的 `|` 会被 eslint 误判为 Vue2 filter 语法），经此 computed 传递。
const cnPresetPlatform = computed<CnProviderPlatform>(() => {
  const platform = props.account?.platform
  if (isCNProviderPlatform(platform ?? '')) {
    return platform as CnProviderPlatform
  }
  return 'kimi'
})
// 地址分不出套餐时管理员选的计费方式（见下方 keyAccountMode）
const editAccountMode = ref<CnAccountMode>('payg')
// 智谱团队版 Coding Plan：组织/项目 ID，写入 credentials 供额度探测切换团队端点
const editZhipuOrganization = ref('')
const editZhipuProject = ref('')
// 回填窗口标志：syncFormFromAccount 会同步改写 editAccountMode 等字段，
// 而 watcher（pre-flush）在同步代码执行完之后才触发——若不抑制，会把刚恢复的
// 存储版协议地址（可能是中转地址）覆盖为官方地址并在下次保存时持久化。
// nextTick 后解除，此后管理员主动切换模式仍正常联动。
const syncingForm = ref(false)

// ── 第三方 key 协议地址 ──
// 初始值取账号已存的 protocol_endpoints。切换账号模式时只替换没改过的地址；
// 任何时候都可以点「填入官方地址」显式恢复。提交时不补任何默认地址。
const protocolDefaults = ref<ProtocolDefaultsResponse | null>(null)
const protocolDefaultsLoadFailed = ref(false)
const editProtocolEndpoints = ref<ProtocolEndpoints>({})

// ── 计费方式（credentials.account_mode）：与新建同一规则（2026-09-28 P5）──
// 厂商与套餐都按地址识别（官方域名表由后端下发，与后端 Account.Vendor 同口径）；地址分不出套餐
// （MiniMax 按量与套餐同地址、智谱 Anthropic 同地址）才让管理员选；OpenCode 的 Zen / Go 只看地址；中转不写。
const CN_PLAN_MODES: readonly CnAccountMode[] = ['payg', 'coding']
const keyPresets = computed(() => keyAddressPresets(protocolDefaults.value))
const keyVendor = computed(() =>
  props.account?.type === 'apikey'
    ? detectKeyVendor(editProtocolEndpoints.value, protocolDefaults.value?.vendor_hosts)
    : null
)
// API Key 占位跟着按地址识别出的厂商走，与新建同一规则（不看平台标签）
const apiKeyValuePlaceholder = computed(() => apiKeyPlaceholderFor(keyVendor.value))
const keyHasCodingPlan = computed(() => !!keyVendor.value && VENDORS_WITH_CODING_PLAN.has(keyVendor.value))
const keyPlanFromAddress = computed(() => {
  const vendor = keyVendor.value
  if (!vendor || !keyHasCodingPlan.value) return null
  const mode = modeOfAddress(keyPresets.value, vendor, editProtocolEndpoints.value)
  return mode === 'payg' || mode === 'coding' ? mode : null
})
const keyPlanNeedsChoice = computed(() => keyHasCodingPlan.value && keyPlanFromAddress.value === null)
const keyAccountMode = computed<string | undefined>(() => {
  const vendor = keyVendor.value
  if (!vendor) return undefined
  if (vendor === 'opencode_go') return modeOfAddress(keyPresets.value, vendor, editProtocolEndpoints.value) ?? 'zen'
  if (vendor === 'deepseek') return 'payg'
  return keyHasCodingPlan.value ? (keyPlanFromAddress.value ?? editAccountMode.value) : undefined
})

const protocolDefaultsMode = computed(() => {
  const platform = props.account?.platform ?? ''
  if (platform === 'opencode_go') return modeOfAddress(keyPresets.value, platform, editProtocolEndpoints.value) ?? 'zen'
  if (isCNProviderPlatform(platform)) return keyPlanFromAddress.value ?? editAccountMode.value
  return undefined
})
// 按平台标签取官方地址：后端官方地址表只有国产厂商与 OpenCode，中转 key 按协议归族的
// anthropic / openai / gemini 标签取到空表，不给「填入官方地址」（muqian 2026-09-29 删海外四家官方 Key）
const officialProtocolEndpoints = computed(() =>
  protocolDefaultsFor(protocolDefaults.value, props.account?.platform ?? '', protocolDefaultsMode.value)
)
watch(officialProtocolEndpoints, (next, previous) => {
  if (syncingForm.value) return
  editProtocolEndpoints.value = endpointsAfterDefaultsChange(
    editProtocolEndpoints.value,
    previous ?? {},
    next,
    // 编辑时平台不变，换模式保留当前协议；还没配协议时取 Chat Completions（国产厂商与 OpenCode 的默认协议）
    currentProtocolOf(editProtocolEndpoints.value) ?? 'chat_completions'
  )
})
async function ensureProtocolDefaults() {
  try {
    protocolDefaults.value = await loadProtocolDefaults()
    protocolDefaultsLoadFailed.value = false
  } catch {
    protocolDefaultsLoadFailed.value = true
  }
}
watch(
  () => props.show,
  (show) => {
    if (show) void ensureProtocolDefaults()
  },
  { immediate: true }
)
// 提交前校验协议地址，有问题直接提示并返回 null。
function validatedProtocolEndpoints(): ProtocolEndpoints | null {
  const issue = validateProtocolEndpoints(editProtocolEndpoints.value)
  if (issue) {
    appStore.showError(describeProtocolEndpointsIssue(issue, t))
    return null
  }
  return trimProtocolEndpoints(editProtocolEndpoints.value)
}
// 点击国产供应商预设：回填账号类型和该协议的地址。
function onCnPresetSelect(preset: CnBaseUrlPreset) {
  editAccountMode.value = preset.mode
  editProtocolEndpoints.value = { [preset.protocol]: preset.url }
}
// Bedrock credentials
const editBedrockAccessKeyId = ref('')
const editBedrockSecretAccessKey = ref('')
const editBedrockRegion = ref('')
const editBedrockForceGlobal = ref(false)
const editBedrockApiKeyValue = ref('')
const editVertexLocation = ref('us-central1')
const isBedrockAPIKeyMode = computed(() =>
  props.account?.type === 'bedrock' &&
  (props.account?.credentials as Record<string, unknown>)?.auth_mode === 'apikey'
)
const modelMappings = ref<ModelMapping[]>([])

// 承接的模型：按渠道读绑定（GET /admin/accounts/:id/catalog-entries），保存时勾选变了才整份写回。
const selectedCatalogEntryIds = ref<number[]>([])
const initialCatalogEntryIds = ref<number[]>([])
const catalogEntryIdsLoaded = ref(false)
const catalogEntryIdsLoadFailed = ref(false)
let catalogEntryIdsLoadSeq = 0
const loadCatalogEntryIds = async (accountID: number) => {
  const seq = ++catalogEntryIdsLoadSeq
  catalogEntryIdsLoaded.value = false
  catalogEntryIdsLoadFailed.value = false
  try {
    const ids = await adminAPI.modelCatalog.listAccountEntryIds(accountID)
    if (seq !== catalogEntryIdsLoadSeq) return
    initialCatalogEntryIds.value = [...ids]
    selectedCatalogEntryIds.value = [...ids]
    catalogEntryIdsLoaded.value = true
  } catch {
    if (seq === catalogEntryIdsLoadSeq) catalogEntryIdsLoadFailed.value = true
  }
}
// 换了渠道才重新读；同一渠道保存后回写账号（updated）不重置勾选
watch(
  () => props.account?.id,
  (accountID) => {
    if (accountID) void loadCatalogEntryIds(accountID)
  },
  { immediate: true }
)
const sameIdSet = (a: number[], b: number[]) => a.length === b.length && a.every((id) => b.includes(id))
// 渠道本身已保存；绑定写失败时提示并留在页面，再点保存会重试。
const persistCatalogEntries = async (accountID: number): Promise<boolean> => {
  if (!catalogEntryIdsLoaded.value || sameIdSet(selectedCatalogEntryIds.value, initialCatalogEntryIds.value)) {
    return true
  }
  try {
    const ids = await adminAPI.modelCatalog.replaceAccountEntries(accountID, selectedCatalogEntryIds.value)
    initialCatalogEntryIds.value = [...ids]
    selectedCatalogEntryIds.value = [...ids]
    return true
  } catch (error: any) {
    appStore.showError(t('admin.accounts.catalogEntries.saveFailed', {
      message: error?.response?.data?.message || error?.message || ''
    }))
    return false
  }
}
// 池模式同渠道重试次数与状态码写死在后端（channel_features.go），这里只有开关
const poolModeEnabled = ref(false)
const headerOverrideRows = ref<HeaderOverrideRow[]>([])

const headerOverrideCapable = computed(
  () => !!props.account && isHeaderOverrideCapable(props.account.platform, props.account.type)
)

const interceptWarmupRequests = ref(false)
const autoResetCreditEnabled = ref(false)
const upstreamBillingAutoProbeEnabled = ref(false)
const upstreamBillingRateSyncEnabled = ref(false)
const allowOverages = ref(false) // For antigravity accounts: enable AI Credits overages
const antigravityProjectId = ref('')
const isSyncingAntigravityUpstream = ref(false)


// Quota control state (Anthropic OAuth/SetupToken only)
// 空闲超时、RPM 策略 / 粘性缓冲、用户消息限速、TLS 指纹、会话 ID 伪装、缓存 TTL 替换已写死在后端
// （channel_features_anthropic.go），表单不再提供；库里的旧值不回填、不改写。
const sessionLimitEnabled = ref(false)
const maxSessions = ref<number | null>(null)
const rpmLimitEnabled = ref(false)
const baseRpm = ref<number | null>(null)

type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'
const anthropicAPIKeyAuthScheme = ref<AnthropicAPIKeyAuthScheme>('x_api_key')
const bedrockCCCompatEnabled = ref(false)
// Anthropic 协议上的 key 设置按编辑中的协议地址展示，不看平台标签。
const anthropicKeySettingsVisible = computed(
  () => props.account?.type === 'apikey' && hasAnthropicEndpoint(editProtocolEndpoints.value)
)
// 表单分区（A5-c）：「基本」「额度」「高级」总有字段；「地址与协议」「模型与映射」只在分区里有区块时才出标题，
// 条件与分区内各区块的 v-if 一一对应（改区块条件时这里一起改）。
const showEndpointSection = computed(() => {
  const account = props.account
  if (!account) return false
  return (
    account.type === 'apikey' ||
    account.type === 'bedrock' ||
    anthropicKeySettingsVisible.value
  )
})
// 模型改名：沿用原来有模型映射的类型（第三方 key、OpenAI / Grok 成品号、Vertex、Bedrock、Antigravity）
const showModelRename = computed(() => {
  const account = props.account
  if (!account) return false
  return (
    account.type === 'apikey' ||
    ((account.platform === 'openai' || account.platform === 'grok') && account.type === 'oauth') ||
    ((account.platform === 'gemini' || account.platform === 'anthropic') && account.type === 'service_account') ||
    account.type === 'bedrock' ||
    account.platform === 'antigravity'
  )
})

const editQuotaLimit = ref<number | null>(null)
const editQuotaDailyLimit = ref<number | null>(null)
const editQuotaWeeklyLimit = ref<number | null>(null)

// 改名快捷项（同名预设只对自带模型表的上游保留，见 renamePresetsFor）
const renamePresets = computed(() =>
  renamePresetsFor(props.account?.type === 'bedrock' ? 'bedrock' : (props.account?.platform || 'anthropic'))
)
const extendsVendorTable = computed(() => PLATFORMS_WITH_VENDOR_MODEL_TABLE.has(props.account?.platform ?? ''))
const form = reactive({
  name: '',
  notes: '',
  proxy_id: null as number | null,
  concurrency: 1,
  priority: 1,
  rate_multiplier: 1,
  status: 'active' as 'active' | 'inactive' | 'error',
  expires_at: null as number | null
})

const handleUpstreamBillingRateSyncChange = (enabled: boolean) => {
  upstreamBillingRateSyncEnabled.value = enabled
  if (enabled) {
    upstreamBillingAutoProbeEnabled.value = true
  }
}

const handleUpstreamBillingAutoProbeChange = (enabled: boolean) => {
  upstreamBillingAutoProbeEnabled.value = enabled
  if (!enabled) {
    upstreamBillingRateSyncEnabled.value = false
  }
}

const statusOptions = computed(() => {
  const options = [
    { value: 'active', label: t('common.active') },
    { value: 'inactive', label: t('common.inactive') }
  ]
  if (form.status === 'error') {
    options.push({ value: 'error', label: t('admin.accounts.status.error') })
  }
  return options
})

const expiresAtInput = computed({
  get: () => formatDateTimeLocal(form.expires_at),
  set: (value: string) => {
    form.expires_at = parseDateTimeLocal(value)
  }
})

// Watchers
// 映射整份按改名行展示：旧白名单留下的同名项也在——对承接没影响，但 Antigravity / xAI 这类自带模型表的
// 上游靠它扩表、批量生图也按映射列模型，不能静默丢掉，管理员可以自己删。
const loadModelRestrictionFromMapping = (rawMapping?: Record<string, unknown>) => {
  modelMappings.value = Object.entries(rawMapping ?? {}).flatMap(([from, to]) =>
    typeof to === 'string' && from.trim() && to.trim() ? [{ from: from.trim(), to: to.trim() }] : []
  )
}

// 写映射并打「只改名」标记（muqian 2026-09-25 去掉白名单）。spark 影子账号的映射是系统维护的模型集合，
// 后端也只放行 model_mapping 一个键，不打标记。
const writeRenameMapping = (credentials: Record<string, unknown>) => {
  const modelMapping = buildModelMappingObject('mapping', [], modelMappings.value)
  if (modelMapping) {
    credentials.model_mapping = modelMapping
  } else {
    delete credentials.model_mapping
  }
  if (modelMapping && !isSparkShadow.value) {
    credentials.model_mapping_rename_only = true
  } else {
    delete credentials.model_mapping_rename_only
  }
}

const syncFormFromAccount = (newAccount: Account | null) => {
  if (!newAccount) {
    return
  }
  // 进入回填窗口：抑制模式 watcher 与官方地址联动（见 syncingForm 注释）。
  syncingForm.value = true
  void nextTick(() => {
    syncingForm.value = false
  })
  editProtocolEndpoints.value = { ...(newAccount.protocol_endpoints ?? {}) }
  form.name = newAccount.name
  form.notes = newAccount.notes || ''
  form.proxy_id = newAccount.proxy_id
  form.concurrency = newAccount.concurrency
  form.priority = newAccount.priority
  form.rate_multiplier = newAccount.rate_multiplier ?? 1
  form.status = (newAccount.status === 'active' || newAccount.status === 'inactive' || newAccount.status === 'error')
    ? newAccount.status
    : 'active'
  form.expires_at = newAccount.expires_at ?? null

  // Load intercept warmup requests setting (applies to all account types)
  const credentials = newAccount.credentials as Record<string, unknown> | undefined
  interceptWarmupRequests.value = credentials?.intercept_warmup_requests === true
  editVertexLocation.value = 'us-central1'
  antigravityProjectId.value =
    newAccount.platform === 'antigravity' &&
    newAccount.type === 'oauth' &&
    typeof credentials?.antigravity_project_id === 'string'
      ? credentials.antigravity_project_id.trim()
      : ''

  allowOverages.value = false
	const extra = newAccount.extra as Record<string, unknown> | undefined
	allowOverages.value = extra?.allow_overages === true
	autoResetCreditEnabled.value = extra?.auto_reset_credit_enabled === true
	upstreamBillingAutoProbeEnabled.value = extra?.upstream_billing_probe_enabled === true
  upstreamBillingRateSyncEnabled.value =
    upstreamBillingAutoProbeEnabled.value && extra?.upstream_billing_rate_sync_enabled === true

  anthropicAPIKeyAuthScheme.value = 'x_api_key'
  bedrockCCCompatEnabled.value = false
  // 长上下文计费开关对任意标签的 key 都生效，按已存值回填；区块可见性另算。
  // 第三方 key 一律回填：地址行可在弹窗里增删，区块是否展示随地址变化
  if (newAccount.type === 'apikey') {
    anthropicAPIKeyAuthScheme.value = extra?.anthropic_apikey_auth_scheme === 'authorization_bearer'
      ? 'authorization_bearer'
      : 'x_api_key'
    bedrockCCCompatEnabled.value = extra?.bedrock_cc_compat === true
  }

  // Load quota limit for apikey/bedrock accounts (bedrock quota is also loaded in its own branch above)
  if (newAccount.type === 'apikey' || newAccount.type === 'bedrock') {
    const quotaVal = extra?.quota_limit as number | undefined
    editQuotaLimit.value = (quotaVal && quotaVal > 0) ? quotaVal : null
    const dailyVal = extra?.quota_daily_limit as number | undefined
    editQuotaDailyLimit.value = (dailyVal && dailyVal > 0) ? dailyVal : null
    const weeklyVal = extra?.quota_weekly_limit as number | undefined
    editQuotaWeeklyLimit.value = (weeklyVal && weeklyVal > 0) ? weeklyVal : null
  } else {
    editQuotaLimit.value = null
    editQuotaDailyLimit.value = null
    editQuotaWeeklyLimit.value = null
  }


  // Load quota control settings (Anthropic OAuth/SetupToken only)
  loadQuotaControlSettings(newAccount)

  // Load header override state for eligible account platforms/types
  headerOverrideRows.value = []
  if (newAccount.credentials && isHeaderOverrideCapable(newAccount.platform, newAccount.type)) {
    const overrideCreds = newAccount.credentials as Record<string, unknown>
    headerOverrideRows.value = splitHeaderOverridesObject(
      overrideCreds[HEADER_OVERRIDES_CREDENTIAL_KEY]
    )
  }

  // Initialize API Key fields for apikey type
  if (newAccount.type === 'apikey' && newAccount.credentials) {
    const credentials = newAccount.credentials as Record<string, unknown>
    // 计费方式：地址分不出套餐时回填已存的选择（分得出就跟地址走，见 keyAccountMode）
    editAccountMode.value = credentials.account_mode === 'coding' ? 'coding' : 'payg'
    // 智谱团队版 Coding Plan：回填组织/项目 ID（厂商按地址识别，是不是智谱看 keyVendor）
    editZhipuOrganization.value = typeof credentials.zhipu_organization === 'string' ? credentials.zhipu_organization : ''
    editZhipuProject.value = typeof credentials.zhipu_project === 'string' ? credentials.zhipu_project : ''
    // Load model mappings and detect mode
    loadModelRestrictionFromMapping(credentials.model_mapping as Record<string, unknown> | undefined)

    // Load pool mode（同渠道重试次数与状态码写死在后端，这里只有开关）
    poolModeEnabled.value = credentials.pool_mode === true
  } else if (newAccount.type === 'bedrock' && newAccount.credentials) {
    const bedrockCreds = newAccount.credentials as Record<string, unknown>
    const authMode = (bedrockCreds.auth_mode as string) || 'sigv4'
    editBedrockRegion.value = (bedrockCreds.aws_region as string) || ''
    editBedrockForceGlobal.value = (bedrockCreds.aws_force_global as string) === 'true'

    if (authMode === 'apikey') {
      editBedrockApiKeyValue.value = ''
    } else {
      editBedrockAccessKeyId.value = (bedrockCreds.aws_access_key_id as string) || ''
      editBedrockSecretAccessKey.value = ''
    }

    // Load quota limits for bedrock
    const bedrockExtra = (newAccount.extra as Record<string, unknown>) || {}
    editQuotaLimit.value = typeof bedrockExtra.quota_limit === 'number' ? bedrockExtra.quota_limit : null
    editQuotaDailyLimit.value = typeof bedrockExtra.quota_daily_limit === 'number' ? bedrockExtra.quota_daily_limit : null
    editQuotaWeeklyLimit.value = typeof bedrockExtra.quota_weekly_limit === 'number' ? bedrockExtra.quota_weekly_limit : null

    // Load model mappings for bedrock
    loadModelRestrictionFromMapping(bedrockCreds.model_mapping as Record<string, unknown> | undefined)
  } else if ((newAccount.platform === 'gemini' || newAccount.platform === 'anthropic') && newAccount.type === 'service_account' && newAccount.credentials) {
    const credentials = newAccount.credentials as Record<string, unknown>
    editVertexLocation.value = (credentials.location as string) || (credentials.vertex_location as string) || 'us-central1'

    // Load model mappings for service_account
    loadModelRestrictionFromMapping(credentials.model_mapping as Record<string, unknown> | undefined)
  } else {
    // Load model mappings for OpenAI/Grok OAuth accounts
    if ((newAccount.platform === 'openai' || newAccount.platform === 'grok') && newAccount.credentials) {
      const oauthCredentials = newAccount.credentials as Record<string, unknown>
      loadModelRestrictionFromMapping(oauthCredentials.model_mapping as Record<string, unknown> | undefined)
    } else if (newAccount.platform === 'antigravity') {
      const agCredentials = (newAccount.credentials as Record<string, unknown> | undefined) ?? {}
      const rawWhitelist = agCredentials.model_whitelist
      if (agCredentials.model_mapping && typeof agCredentials.model_mapping === 'object') {
        loadModelRestrictionFromMapping(agCredentials.model_mapping as Record<string, unknown>)
      } else if (Array.isArray(rawWhitelist)) {
        // 旧数据：model_whitelist 转成同名改名行，保存时迁到 model_mapping
        modelMappings.value = rawWhitelist
          .map((value) => String(value).trim())
          .filter((value) => value.length > 0)
          .map((model) => ({ from: model, to: model }))
      } else {
        modelMappings.value = []
      }
    } else {
      modelMappings.value = []
    }
    poolModeEnabled.value = false
  }
  editApiKey.value = ''
}

watch(
  [() => props.show, () => props.account],
  ([show, newAccount], [wasShow, previousAccount]) => {
    if (!show || !newAccount) {
      return
    }
    if (!wasShow || newAccount !== previousAccount) {
      syncFormFromAccount(newAccount)
    }
  },
  { immediate: true }
)

const syncAntigravityUpstreamModels = async () => {
  if (!props.account?.id || isSyncingAntigravityUpstream.value) return

  isSyncingAntigravityUpstream.value = true
  try {
    const result = await adminAPI.accounts.syncUpstreamModels(props.account.id)
    const upstreamModels = result.models.map((model) => model.trim()).filter(Boolean)
    if (upstreamModels.length === 0) {
      appStore.showInfo(t('admin.accounts.syncUpstreamModelsEmpty'))
      return
    }

    let addedCount = 0
    for (const model of upstreamModels) {
      const exists = modelMappings.value.some((mapping) => mapping.from === model)
      if (!exists) {
        modelMappings.value = [...modelMappings.value, { from: model, to: model }]
        addedCount += 1
      }
    }

    const warnings = result.warnings ?? []
    const hasPartialMetadata = warnings.some(
      (warning) => warning.code === 'upstream_model_metadata_partial'
    )
    const hasIncompleteMetadata = warnings.some(
      (warning) => warning.code === 'upstream_model_metadata_incomplete'
    )
    if (hasIncompleteMetadata) {
      appStore.showWarning(t('admin.accounts.syncUpstreamModelsMetadataIncomplete'))
      return
    }
    if (addedCount > 0) {
      appStore.showSuccess(t('admin.accounts.syncUpstreamModelsSuccess', { count: addedCount, total: upstreamModels.length }))
    } else {
      appStore.showInfo(t('admin.accounts.syncUpstreamModelsNoChanges', { count: upstreamModels.length }))
    }
    if (hasPartialMetadata) {
      appStore.showWarning(t('admin.accounts.syncUpstreamModelsMetadataPartial'))
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : t('admin.accounts.syncUpstreamModelsFailed')
    appStore.showError(t('admin.accounts.syncUpstreamModelsError', { message }))
  } finally {
    isSyncingAntigravityUpstream.value = false
  }
}

// Load quota control settings from account (Anthropic OAuth/SetupToken only)
function loadQuotaControlSettings(account: Account) {
  // Reset all quota control state first
  sessionLimitEnabled.value = false
  maxSessions.value = null
  rpmLimitEnabled.value = false
  baseRpm.value = null

  // Remaining quota control settings only apply to Anthropic accounts
  if (account.platform !== 'anthropic') {
    return
  }

  // Session / RPM limit only apply to Anthropic OAuth/SetupToken accounts
  if (account.type !== 'oauth' && account.type !== 'setup-token') {
    return
  }

  // Load from extra field (via backend DTO fields)
  if (account.max_sessions != null && account.max_sessions > 0) {
    sessionLimitEnabled.value = true
    maxSessions.value = account.max_sessions
  }

  // RPM limit
  if (account.base_rpm != null && account.base_rpm > 0) {
    rpmLimitEnabled.value = true
    baseRpm.value = account.base_rpm
  }
}

const formatDateTimeLocal = formatDateTimeLocalInput
const parseDateTimeLocal = parseDateTimeLocalInput

// Methods
const handleClose = () => {
  emit('close')
}

const submitUpdateAccount = async (accountID: number, updatePayload: Record<string, unknown>) => {
  submitting.value = true
  try {
    const updatedAccount = await adminAPI.accounts.update(accountID, updatePayload)
    const catalogSaved = await persistCatalogEntries(accountID)
    appStore.showSuccess(t('admin.accounts.accountUpdated'))
    emit('updated', updatedAccount)
    if (catalogSaved) handleClose()
  } catch (error: any) {
    appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
  } finally {
    submitting.value = false
  }
}

const handleSubmit = async () => {
  if (!props.account) return
  const accountID = props.account.id

  if (form.status !== 'active' && form.status !== 'inactive' && form.status !== 'error') {
    appStore.showError(t('admin.accounts.pleaseSelectStatus'))
    return
  }

  const updatePayload: Record<string, unknown> = { ...form }
  try {
    // 后端期望 proxy_id: 0 表示清除代理，而不是 null
    if (updatePayload.proxy_id === null) {
      updatePayload.proxy_id = 0
    }
    if (form.expires_at === null) {
      updatePayload.expires_at = 0
    }
    if (props.account.type === 'apikey') {
      updatePayload.upstream_billing_probe_enabled = upstreamBillingAutoProbeEnabled.value
      updatePayload.upstream_billing_rate_sync_enabled = upstreamBillingRateSyncEnabled.value
      if (upstreamBillingRateSyncEnabled.value) {
        delete updatePayload.rate_multiplier
      }
    }

    // For apikey type, handle credentials update
    if (props.account.type === 'apikey') {
      const apiKeyEndpoints = validatedProtocolEndpoints()
      if (!apiKeyEndpoints) {
        return
      }
      updatePayload.protocol_endpoints = apiKeyEndpoints
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}

      // Always update credentials for apikey type to handle model mapping changes
      const newCredentials: Record<string, unknown> = { ...currentCredentials }
      // 计费方式与新建同一规则：按地址识别出厂商才写 account_mode（决定额度/余额探测），地址指向中转就去掉。
      // 官方域名表没拉到时认不出厂商，保留已存的值，免得一次保存把套餐清掉。
      if (protocolDefaults.value) {
        if (keyAccountMode.value) newCredentials.account_mode = keyAccountMode.value
        else delete newCredentials.account_mode
      }
      // 智谱团队版 Coding Plan：组织/项目 ID 写入凭据（非空才写，清空即移除回落个人版路径）
      if (keyVendor.value === 'zhipu') {
        const org = editZhipuOrganization.value.trim()
        const project = editZhipuProject.value.trim()
        if (org) {
          newCredentials.zhipu_organization = org
          if (project) newCredentials.zhipu_project = project
          else delete newCredentials.zhipu_project
        } else {
          delete newCredentials.zhipu_organization
          delete newCredentials.zhipu_project
        }
      }

      // Handle API key
      // 后端响应已脱敏：currentCredentials 不会再包含 api_key 原文。
      // 用户填入新值则覆盖；留空时优先看 credentials_status.has_api_key；
      // 若后端尚未升级（无 credentials_status），回退读旧结构 currentCredentials.api_key。
      // 两者都无才报错。
      const hasExistingApiKey =
        props.account.credentials_status?.has_api_key ?? Boolean(currentCredentials.api_key)
      if (editApiKey.value.trim()) {
        newCredentials.api_key = editApiKey.value.trim()
      } else if (!hasExistingApiKey) {
        appStore.showError(t('admin.accounts.apiKeyIsRequired'))
        return
      }

      // Add model mapping if configured
      writeRenameMapping(newCredentials)

      // 池模式：同渠道重试次数与状态码写死在后端（channel_features.go），这里只写开关
      if (poolModeEnabled.value) {
        newCredentials.pool_mode = true
      } else {
        delete newCredentials.pool_mode
      }

      // 请求头覆写对任何第三方 key 开放，有条目就生效
      const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
      if (headerError) {
        appStore.showError(t(`admin.accounts.headerOverride.${headerError}`))
        return
      }
      applyHeaderOverride(newCredentials, headerOverrideRows.value, 'edit')

      // Add intercept warmup requests setting
      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

      updatePayload.credentials = newCredentials
    } else if ((props.account.platform === 'gemini' || props.account.platform === 'anthropic') && props.account.type === 'service_account') {
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      if (!editVertexLocation.value.trim()) {
        appStore.showError(t('admin.accounts.vertexLocationRequired'))
        return
      }

      // SA JSON 已脱敏不再随 credentials 返回，存在性优先读 credentials_status。
      // 若后端尚未升级（无 credentials_status），回退读旧结构 service_account_json / service_account。
      const credentialsStatus = props.account.credentials_status
      const hasExistingServiceAccountJson = credentialsStatus
        ? Boolean(
            credentialsStatus.has_service_account_json || credentialsStatus.has_service_account
          )
        : Boolean(currentCredentials.service_account_json || currentCredentials.service_account)
      if (!hasExistingServiceAccountJson) {
        appStore.showError(t('admin.accounts.vertexSaJsonRequired'))
        return
      }
      newCredentials.location = editVertexLocation.value.trim()
      newCredentials.tier_id = 'vertex'

      writeRenameMapping(newCredentials)

      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

      updatePayload.credentials = newCredentials
    } else if (props.account.type === 'bedrock') {
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      newCredentials.aws_region = editBedrockRegion.value.trim()
      if (editBedrockForceGlobal.value) {
        newCredentials.aws_force_global = 'true'
      } else {
        delete newCredentials.aws_force_global
      }

      if (isBedrockAPIKeyMode.value) {
        // API Key mode: only update api_key if user provided new value
        if (editBedrockApiKeyValue.value.trim()) {
          newCredentials.api_key = editBedrockApiKeyValue.value.trim()
        }
      } else {
        // SigV4 mode
        newCredentials.aws_access_key_id = editBedrockAccessKeyId.value.trim()
        if (editBedrockSecretAccessKey.value.trim()) {
          newCredentials.aws_secret_access_key = editBedrockSecretAccessKey.value.trim()
        }
      }

      writeRenameMapping(newCredentials)

      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

      updatePayload.credentials = newCredentials
    } else {
      // For oauth/setup-token types, only update intercept_warmup_requests if changed
      const currentCredentials = (props.account.credentials as Record<string, unknown>) || {}
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      applyInterceptWarmup(newCredentials, interceptWarmupRequests.value, 'edit')

      updatePayload.credentials = newCredentials
    }

    // OpenAI/Grok OAuth: persist model mapping to credentials
    if ((props.account.platform === 'openai' || props.account.platform === 'grok') && props.account.type === 'oauth') {
      const currentCredentials = isSparkShadow.value
        ? {}
        : (updatePayload.credentials as Record<string, unknown>) ||
          ((props.account.credentials as Record<string, unknown>) || {})
      const newCredentials: Record<string, unknown> = { ...currentCredentials }
      writeRenameMapping(newCredentials)

      updatePayload.credentials = newCredentials
    }

    // Grok OAuth: 请求头覆写。成品号只走官方地址，没有可配置的上游地址。
    if (props.account.platform === 'grok' && props.account.type === 'oauth') {
      const currentCredentials =
        (updatePayload.credentials as Record<string, unknown>) ||
        ((props.account.credentials as Record<string, unknown>) || {})
      const newCredentials: Record<string, unknown> = { ...currentCredentials }

      const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
      if (headerError) {
        appStore.showError(t(`admin.accounts.headerOverride.${headerError}`))
        return
      }
      applyHeaderOverride(newCredentials, headerOverrideRows.value, 'edit')

      updatePayload.credentials = newCredentials
    }

    // Antigravity: persist model mapping to credentials (applies to all antigravity types)
    // Antigravity 只支持映射模式
    if (props.account.platform === 'antigravity') {
      const currentCredentials = (updatePayload.credentials as Record<string, unknown>) ||
        ((props.account.credentials as Record<string, unknown>) || {})
      const newCredentials: Record<string, unknown> = { ...currentCredentials }
      if (props.account.type === 'oauth') {
        applyAntigravityProjectID(newCredentials, antigravityProjectId.value, 'edit')
      }

      // 移除旧字段；改名叠在 Antigravity 默认表之上（后端合并）
      delete newCredentials.model_whitelist
      writeRenameMapping(newCredentials)

      updatePayload.credentials = newCredentials
    }

    // 超量只属于 Antigravity 成品号；第三方 key 不写这个键
    if (props.account.platform === 'antigravity' && props.account.type === 'oauth') {
      const currentExtra = (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (allowOverages.value) {
        newExtra.allow_overages = true
      } else {
        delete newExtra.allow_overages
      }
      updatePayload.extra = newExtra
    }

    // For Anthropic OAuth/SetupToken accounts, handle quota control settings in extra
    if (props.account.platform === 'anthropic' && (props.account.type === 'oauth' || props.account.type === 'setup-token')) {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }

      // Session limit settings
      if (sessionLimitEnabled.value && maxSessions.value != null && maxSessions.value > 0) {
        newExtra.max_sessions = maxSessions.value
      } else {
        delete newExtra.max_sessions
      }

      // RPM limit settings
      if (rpmLimitEnabled.value) {
        const DEFAULT_BASE_RPM = 15
        newExtra.base_rpm = (baseRpm.value != null && baseRpm.value > 0)
          ? baseRpm.value
          : DEFAULT_BASE_RPM
      } else {
        delete newExtra.base_rpm
      }

      updatePayload.extra = newExtra
    }

    // 第三方 key 的 Anthropic 协议设置（认证方式 / Bedrock CC 兼容）写入 extra。
    // 区块隐藏（没有 anthropic 地址）时不写界面上的值，账号已存的值原样保留，与其他隐藏区块一致。
    if (anthropicKeySettingsVisible.value) {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (anthropicAPIKeyAuthScheme.value === 'authorization_bearer') {
        newExtra.anthropic_apikey_auth_scheme = 'authorization_bearer'
      } else {
        delete newExtra.anthropic_apikey_auth_scheme
      }
      if (bedrockCCCompatEnabled.value) {
        newExtra.bedrock_cc_compat = true
      } else {
        delete newExtra.bedrock_cc_compat
      }
      updatePayload.extra = newExtra
    }

    // 第三方 key 专属、按协议地址判定的 extra：不看平台标签。
    if (props.account.type === 'apikey') {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      // Responses 路由改由协议地址决定，已退役的探测标记与强制模式保存时清掉。
      delete newExtra.openai_responses_mode
      delete newExtra.openai_responses_supported
      updatePayload.extra = newExtra
    }

    // OpenAI 成品号：自动使用重置卡开关（阈值写死 100%，见后端 channel_features_openai.go）
    if (props.account.platform === 'openai' && (props.account.type === 'oauth' || props.account.type === 'setup-token' || props.account.type === 'apikey')) {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (props.account.type === 'oauth' && !isSparkShadow.value) {
        newExtra.auto_reset_credit_enabled = autoResetCreditEnabled.value
      }
      // 运行态只允许后端服务更新，账号编辑不得回写旧状态。
      delete newExtra.codex_auto_reset_credit_state
      updatePayload.extra = newExtra
    }

    // For apikey/bedrock accounts, handle quota_limit in extra
    if (props.account.type === 'apikey' || props.account.type === 'bedrock') {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) ||
        (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      // 上游倍率自动探测对全部 API-key 平台开放（sub2api 上游即可应答），
      // Bedrock 凭证无静态 Key 不参与。
      if (props.account.type === 'apikey') {
        delete newExtra.upstream_billing_probe_enabled
        delete newExtra.upstream_billing_rate_sync_enabled
      }
      // Total quota
      if (editQuotaLimit.value != null && editQuotaLimit.value > 0) {
        newExtra.quota_limit = editQuotaLimit.value
      } else {
        delete newExtra.quota_limit
      }
      // Daily quota
      if (editQuotaDailyLimit.value != null && editQuotaDailyLimit.value > 0) {
        newExtra.quota_daily_limit = editQuotaDailyLimit.value
      } else {
        delete newExtra.quota_daily_limit
        delete newExtra.quota_daily_used
        delete newExtra.quota_daily_start
      }
      // Weekly quota
      if (editQuotaWeeklyLimit.value != null && editQuotaWeeklyLimit.value > 0) {
        newExtra.quota_weekly_limit = editQuotaWeeklyLimit.value
      } else {
        delete newExtra.quota_weekly_limit
        delete newExtra.quota_weekly_used
        delete newExtra.quota_weekly_start
      }
      updatePayload.extra = newExtra
    }

    await submitUpdateAccount(accountID, updatePayload)
  } catch (error: any) {
    appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
  }
}
</script>
