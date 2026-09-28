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
            :placeholder="
              account.platform === 'openai'
                ? 'sk-proj-...'
                : account.platform === 'gemini'
                  ? 'AIza...'
                  : account.platform === 'antigravity'
                    ? 'sk-...'
                    : account.platform === 'grok'
                      ? 'xai-...'
                      : 'sk-ant-...'
            "
          />
          <p class="input-hint">{{ t('admin.accounts.leaveEmptyToKeep') }}</p>
        </div>
      </div>

      <!-- Vertex Service Account：项目与区域 -->
      <div v-if="(account.platform === 'gemini' || account.platform === 'anthropic') && account.type === 'service_account'" class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <label class="input-label">Project ID</label>
          <input
            v-model="editVertexProjectId"
            type="text"
            class="input font-mono"
            readonly
            :placeholder="t('admin.accounts.vertexProjectIdPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.vertexSaJsonEditHint') }}</p>
        </div>
        <div>
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
        </div>
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
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockSessionToken') }}</label>
            <input
              v-model="editBedrockSessionToken"
              type="password"
              class="input font-mono"
              :placeholder="t('admin.accounts.bedrockSecretKeyLeaveEmpty')"
            />
            <p class="input-hint">{{ t('admin.accounts.bedrockSessionTokenHint') }}</p>
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

      <!-- OpenAI 订阅档位手动覆盖（Plus/Pro/Free），仅 OAuth 非影子账号 -->
      <div
        v-if="account?.platform === 'openai' && account?.type === 'oauth' && !isSparkShadow"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <label class="input-label mb-0">{{ t('admin.accounts.openai.planType') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.openai.planTypeDesc') }}
            </p>
          </div>
          <div class="w-44 flex-shrink-0">
            <Select v-model="editPlanType" :options="planTypeOptions" />
          </div>
        </div>
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
        <GrokBaseUrlPresets
          v-if="account.platform === 'grok'"
          class="mt-2"
          @select="applyGrokPreset"
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

      <!-- OpenAI 自动透传开关：OpenAI 成品号；第三方 key 配了 responses / chat_completions 地址才展示，不看平台标签 -->
      <div
        v-if="openAIResponsesSettingsVisible"
        data-testid="edit-openai-passthrough"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.oauthPassthrough') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.openai.oauthPassthroughDesc') }}
            </p>
            <p
              v-if="account?.type === 'apikey'"
              data-testid="edit-openai-key-protocol-hint"
              class="mt-1 text-xs text-af-warning"
            >
              {{ t('admin.accounts.openai.keyProtocolSettingsHint') }}
            </p>
          </div>
          <button
            type="button"
            data-testid="edit-openai-passthrough-toggle"
            @click="openaiPassthroughEnabled = !openaiPassthroughEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              openaiPassthroughEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                openaiPassthroughEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <!-- OpenAI Codex namespace 工具摊平（兼容开关，仅 OAuth） -->
      <div
        v-if="account?.platform === 'openai' && account?.type === 'oauth'"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.flattenNamespaces') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.openai.flattenNamespacesDesc') }}
            </p>
          </div>
          <button
            type="button"
            data-testid="edit-openai-flatten-namespaces-toggle"
            @click="openaiFlattenNamespacesEnabled = !openaiFlattenNamespacesEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              openaiFlattenNamespacesEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                openaiFlattenNamespacesEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <!-- OpenAI Codex hosted image_generation bridge policy -->
      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'setup-token' || account?.type === 'apikey')"
        class="border-t border-af-hairline pt-4"
      >
        <div class="overflow-hidden rounded-lg border border-af-hairline bg-af-sunken/60">
          <div class="flex items-start gap-3 px-4 py-3">
            <div class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-af-sheet text-af-ink-2 ring-1 ring-af-hairline">
              <Icon name="sparkles" size="sm" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <label class="input-label mb-0">{{ t('admin.accounts.openai.codexImageTool') }}</label>
                <span
                  class="rounded-full px-2 py-0.5 text-[11px] font-medium"
                  :class="codexImageToolBadgeClass"
                >
                  {{ codexImageToolBadgeLabel }}
                </span>
              </div>
              <p class="mt-1 text-xs leading-5 text-af-ink-2">
                {{ t('admin.accounts.openai.codexImageToolDesc') }}
              </p>
            </div>
          </div>
          <div class="border-t border-af-hairline bg-af-sheet/70 p-2">
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <button
                v-for="option in codexImageToolOptions"
                :key="option.value"
                type="button"
                :data-testid="`codex-image-tool-${option.value}`"
                @click="codexImageToolMode = option.value"
                :class="[
                  'group flex min-h-[62px] items-start gap-2 rounded-md border px-3 py-2 text-left transition-all',
                  codexImageToolMode === option.value
                    ? option.selectedCardClass
                    : 'border-transparent bg-transparent text-af-ink-2 hover:border-af-hairline hover:bg-af-sunken'
                ]"
              >
                <span
                  :class="[
                    'mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border transition-colors',
                    codexImageToolMode === option.value
                      ? option.selectedDotClass
                      : 'border-af-hairline-strong text-transparent group-hover:border-af-ink-4'
                  ]"
                >
                  <Icon name="check" size="xs" :stroke-width="2" />
                </span>
                <span class="min-w-0">
                  <span class="block text-sm font-medium">{{ option.label }}</span>
                  <span class="mt-0.5 block text-xs leading-4 text-af-ink-3">{{ option.description }}</span>
                </span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- OpenAI WS Mode（off/ctx_pool/passthrough/http_bridge），展示条件同自动透传 -->
      <div
        v-if="openAIResponsesSettingsVisible"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.wsMode') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.openai.wsModeDesc') }}
            </p>
            <p v-if="openAIWSModeHintKey" class="mt-1 text-xs text-af-ink-3">
              {{ t(openAIWSModeHintKey) }}
            </p>
          </div>
          <div class="w-52">
            <Select v-model="openaiResponsesWebSocketV2Mode" data-testid="edit-openai-ws-mode-select" :options="openAIWSModeOptions" />
          </div>
        </div>
      </div>

      <!-- OpenAI APIKey endpoint capabilities -->
      <div
        v-if="openAIKeySettingsVisible"
        class="space-y-4 border-t border-af-hairline pt-4"
      >
        <div>
          <label class="input-label mb-2 block">{{ t('admin.accounts.openai.endpointCapabilities') }}</label>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <label
              v-for="option in openAIEndpointCapabilityOptions"
              :key="option.value"
              class="flex cursor-pointer items-center gap-2 rounded-lg border border-af-hairline px-3 py-2 text-sm"
            >
              <input
                type="checkbox"
                class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
                :data-testid="`openai-endpoint-capability-${option.value}`"
                :checked="openAIEndpointCapabilities.includes(option.value)"
                @change="toggleOpenAIEndpointCapability(option.value, $event)"
              />
              <span class="text-af-ink-2">{{ option.label }}</span>
            </label>
          </div>
          <p class="input-hint">{{ t('admin.accounts.openai.endpointCapabilitiesDesc') }}</p>
        </div>
      </div>

      <!-- OpenAI APIKey images: backfill b64_json from url -->
      <div
        v-if="openAIKeySettingsVisible"
        class="flex items-center justify-between gap-4 border-t border-af-hairline pt-4"
      >
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.openai.imagesUrlToB64Json') }}</label>
          <p class="mt-1 text-xs text-af-ink-3">
            {{ t('admin.accounts.openai.imagesUrlToB64JsonDesc') }}
          </p>
        </div>
        <button
          type="button"
          data-testid="openai-images-url-to-b64-json-toggle"
          role="switch"
          :aria-checked="openAIImagesUrlToB64JsonEnabled"
          @click="openAIImagesUrlToB64JsonEnabled = !openAIImagesUrlToB64JsonEnabled"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
            openAIImagesUrlToB64JsonEnabled ? 'bg-af-brand' : 'bg-af-hairline'
          ]"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
              openAIImagesUrlToB64JsonEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>

      <!-- 第三方 key 的 Anthropic 协议设置：配了 anthropic 协议地址才展示，不看平台标签 -->
      <div
        v-if="anthropicKeySettingsVisible"
        data-testid="edit-anthropic-passthrough"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.anthropic.apiKeyPassthrough') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.anthropic.apiKeyPassthroughDesc') }}
            </p>
          </div>
          <button
            type="button"
            data-testid="edit-anthropic-passthrough-toggle"
            @click="anthropicPassthroughEnabled = !anthropicPassthroughEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              anthropicPassthroughEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                anthropicPassthroughEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

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

      <!-- Web Search Emulation（Anthropic 协议上的 key 设置，全局关闭时隐藏） -->
      <div
        v-if="anthropicKeySettingsVisible && webSearchGlobalEnabled"
        data-testid="edit-web-search-emulation"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.anthropic.webSearchEmulation') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.anthropic.webSearchEmulationDesc') }}
            </p>
          </div>
          <button
            type="button"
            data-testid="edit-web-search-emulation-toggle"
            @click="webSearchEmulationEnabled = !webSearchEmulationEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              webSearchEmulationEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                webSearchEmulationEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
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
        :disabled="isOpenAIModelRestrictionDisabled"
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

      <!-- OpenAI Compact 模式与专属模型映射，展示条件同自动透传 -->
      <div
        v-if="openAIResponsesSettingsVisible"
        data-testid="edit-openai-compact"
        class="border-t border-af-hairline pt-4 space-y-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.compactMode') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.openai.compactModeDesc') }}
            </p>
          </div>
          <div class="w-44">
            <Select v-model="openAICompactMode" data-testid="edit-openai-compact-mode-select" :options="openAICompactModeOptions" />
          </div>
        </div>
        <div class="rounded-lg bg-af-sunken px-3 py-2 text-xs text-af-ink-2">
          <span class="font-medium">{{ t(openAICompactStatusKey) }}</span>
          <span
            v-if="account?.extra?.openai_compact_checked_at"
            class="ml-2 text-af-ink-3"
          >
            {{ t('admin.accounts.openai.compactLastChecked') }}:
            {{ formatDateTime(new Date(String(account.extra.openai_compact_checked_at))) }}
          </span>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.openai.compactModelMapping') }}</label>
          <p class="input-hint">{{ t('admin.accounts.openai.compactModelMappingDesc') }}</p>
          <div v-if="openAICompactModelMappings.length > 0" class="mb-3 space-y-2">
            <div
              v-for="(mapping, index) in openAICompactModelMappings"
              :key="getOpenAICompactModelMappingKey(mapping)"
              class="flex items-center gap-2"
            >
              <input
                v-model="mapping.from"
                type="text"
                class="input flex-1"
                :placeholder="t('admin.accounts.fromModel')"
              />
              <span class="text-af-ink-3">→</span>
              <input
                v-model="mapping.to"
                type="text"
                class="input flex-1"
                :placeholder="t('admin.accounts.toModel')"
              />
              <button type="button" @click="removeOpenAICompactModelMapping(index)" class="text-af-danger hover:text-af-danger">
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>
          <button type="button" @click="addOpenAICompactModelMapping" class="btn btn-secondary text-sm">
            + {{ t('admin.accounts.addMapping') }}
          </button>
        </div>
      </div>

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

      <!-- 配额控制 (Anthropic OAuth/SetupToken: 窗口费用 + 会话 + RPM) -->
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

        <!-- Window Cost Limit -->
        <div class="rounded-lg border border-af-hairline p-4">
          <div class="mb-3 flex items-center justify-between">
            <div>
              <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.windowCost.label') }}</label>
              <p class="mt-1 text-xs text-af-ink-3">
                {{ t('admin.accounts.quotaControl.windowCost.hint') }}
              </p>
            </div>
            <button
              type="button"
              @click="windowCostEnabled = !windowCostEnabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                windowCostEnabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  windowCostEnabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="windowCostEnabled" class="grid grid-cols-2 gap-4">
            <div>
              <label class="input-label">{{ t('admin.accounts.quotaControl.windowCost.limit') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-af-ink-3">$</span>
                <input
                  v-model.number="windowCostLimit"
                  type="number"
                  min="0"
                  step="1"
                  class="input pl-7"
                  :placeholder="t('admin.accounts.quotaControl.windowCost.limitPlaceholder')"
                />
              </div>
              <p class="input-hint">{{ t('admin.accounts.quotaControl.windowCost.limitHint') }}</p>
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.quotaControl.windowCost.stickyReserve') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-af-ink-3">$</span>
                <input
                  v-model.number="windowCostStickyReserve"
                  type="number"
                  min="0"
                  step="1"
                  class="input pl-7"
                  :placeholder="t('admin.accounts.quotaControl.windowCost.stickyReservePlaceholder')"
                />
              </div>
              <p class="input-hint">{{ t('admin.accounts.quotaControl.windowCost.stickyReserveHint') }}</p>
            </div>
          </div>
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

          <div v-if="sessionLimitEnabled" class="grid grid-cols-2 gap-4">
            <div>
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
            <div>
              <label class="input-label">{{ t('admin.accounts.quotaControl.sessionLimit.idleTimeout') }}</label>
              <div class="relative">
                <input
                  v-model.number="sessionIdleTimeout"
                  type="number"
                  min="1"
                  step="1"
                  class="input pr-12"
                  :placeholder="t('admin.accounts.quotaControl.sessionLimit.idleTimeoutPlaceholder')"
                />
                <span class="absolute right-3 top-1/2 -translate-y-1/2 text-af-ink-3">{{ t('common.minutes') }}</span>
              </div>
              <p class="input-hint">{{ t('admin.accounts.quotaControl.sessionLimit.idleTimeoutHint') }}</p>
            </div>
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

            <div>
              <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.strategy') }}</label>
              <div class="flex gap-2">
                <button
                  type="button"
                  @click="rpmStrategy = 'tiered'"
                  :class="[
                    'flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all',
                    rpmStrategy === 'tiered'
                      ? 'bg-af-brand-tint text-af-brand'
                      : 'bg-af-sunken text-af-ink-2 hover:bg-af-hairline'
                  ]"
                >
                  <div class="text-center">
                    <div>{{ t('admin.accounts.quotaControl.rpmLimit.strategyTiered') }}</div>
                    <div class="mt-0.5 text-[10px] opacity-70">{{ t('admin.accounts.quotaControl.rpmLimit.strategyTieredHint') }}</div>
                  </div>
                </button>
                <button
                  type="button"
                  @click="rpmStrategy = 'sticky_exempt'"
                  :class="[
                    'flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all',
                    rpmStrategy === 'sticky_exempt'
                      ? 'bg-af-brand-tint text-af-brand'
                      : 'bg-af-sunken text-af-ink-2 hover:bg-af-hairline'
                  ]"
                >
                  <div class="text-center">
                    <div>{{ t('admin.accounts.quotaControl.rpmLimit.strategyStickyExempt') }}</div>
                    <div class="mt-0.5 text-[10px] opacity-70">{{ t('admin.accounts.quotaControl.rpmLimit.strategyStickyExemptHint') }}</div>
                  </div>
                </button>
              </div>
            </div>

            <div v-if="rpmStrategy === 'tiered'">
              <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.stickyBuffer') }}</label>
              <input
                v-model.number="rpmStickyBuffer"
                type="number"
                min="1"
                step="1"
                class="input"
                :placeholder="t('admin.accounts.quotaControl.rpmLimit.stickyBufferPlaceholder')"
              />
              <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.stickyBufferHint') }}</p>
            </div>

          </div>

          <!-- 用户消息限速模式（独立于 RPM 开关，始终可见） -->
          <div class="mt-4">
            <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.userMsgQueue') }}</label>
            <p class="mt-1 text-xs text-af-ink-3 mb-2">
              {{ t('admin.accounts.quotaControl.rpmLimit.userMsgQueueHint') }}
            </p>
            <div class="flex space-x-2">
              <button type="button" v-for="opt in umqModeOptions" :key="opt.value"
                @click="userMsgQueueMode = opt.value"
                :class="[
                  'px-3 py-1.5 text-sm rounded-md border transition-colors',
                  userMsgQueueMode === opt.value
                    ? 'bg-af-brand text-af-on-brand border-af-brand'
                    : 'bg-af-sheet text-af-ink-2 border-af-hairline-strong hover:bg-af-sunken'
                ]">
                {{ opt.label }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="account?.platform === 'openai'"
        class="border-t border-af-hairline pt-4 space-y-4"
      >
        <div class="space-y-2">
          <div class="flex items-center justify-between gap-4">
            <label class="input-label mb-0">{{ t('admin.accounts.autoPause5hDisabled') }}</label>
            <button
              type="button"
              @click="autoPause5hDisabled = !autoPause5hDisabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                autoPause5hDisabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
              data-testid="auto-pause-5h-disabled"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  autoPause5hDisabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          <p class="input-hint">{{ t('admin.accounts.autoPauseDisabledHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.autoPause5hThreshold') }}</label>
          <input
            v-model.number="autoPause5hThreshold"
            type="number"
            min="0"
            max="100"
            step="0.1"
            class="input"
            :disabled="autoPause5hDisabled"
            data-testid="auto-pause-5h-threshold"
          />
          <p class="input-hint">{{ t('admin.accounts.autoPauseThresholdHint') }}</p>
        </div>
        <div class="space-y-2">
          <div class="flex items-center justify-between gap-4">
            <label class="input-label mb-0">{{ t('admin.accounts.autoPause7dDisabled') }}</label>
            <button
              type="button"
              @click="autoPause7dDisabled = !autoPause7dDisabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                autoPause7dDisabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
              data-testid="auto-pause-7d-disabled"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  autoPause7dDisabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          <p class="input-hint">{{ t('admin.accounts.autoPauseDisabledHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.autoPause7dThreshold') }}</label>
          <input
            v-model.number="autoPause7dThreshold"
            type="number"
            min="0"
            max="100"
            step="0.1"
            class="input"
            :disabled="autoPause7dDisabled"
            data-testid="auto-pause-7d-threshold"
          />
          <p class="input-hint">{{ t('admin.accounts.autoPauseThresholdHint') }}</p>
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
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.accounts.autoResetCredit.threshold5h') }}</label>
            <input
              v-model.number="autoResetCredit5hThreshold"
              type="number"
              min="0.1"
              max="100"
              step="0.1"
              class="input"
              :disabled="!autoResetCreditEnabled"
              data-testid="auto-reset-credit-5h-threshold"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.autoResetCredit.threshold7d') }}</label>
            <input
              v-model.number="autoResetCredit7dThreshold"
              type="number"
              min="0.1"
              max="100"
              step="0.1"
              class="input"
              :disabled="!autoResetCreditEnabled"
              data-testid="auto-reset-credit-7d-threshold"
            />
          </div>
        </div>
        <p class="input-hint">{{ t('admin.accounts.autoResetCredit.thresholdHint') }}</p>
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

      <!-- Bedrock 的池模式 -->
      <!-- Pool Mode Section for Bedrock -->
      <div v-if="account.type === 'bedrock'" class="border-t border-af-hairline pt-4">
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
        <div v-if="poolModeEnabled" class="mt-3">
          <label class="input-label">{{ t('admin.accounts.poolModeRetryCount') }}</label>
          <input
            v-model.number="poolModeRetryCount"
            type="number"
            min="0"
            :max="MAX_POOL_MODE_RETRY_COUNT"
            step="1"
            class="input"
          />
          <p class="mt-1 text-xs text-af-ink-3">
            {{
              t('admin.accounts.poolModeRetryCountHint', {
                default: DEFAULT_POOL_MODE_RETRY_COUNT,
                max: MAX_POOL_MODE_RETRY_COUNT
              })
            }}
          </p>
        </div>
        <div v-if="poolModeEnabled" class="mt-3">
          <label class="input-label">{{ t('admin.accounts.poolModeRetryStatusCodes') }}</label>
          <input
            v-model="poolModeRetryStatusCodesInput"
            type="text"
            class="input"
            :placeholder="DEFAULT_POOL_MODE_RETRY_STATUS_CODES.join(', ')"
          />
          <p class="mt-1 text-xs text-af-ink-3">
            {{ t('admin.accounts.poolModeRetryStatusCodesHint', { default: DEFAULT_POOL_MODE_RETRY_STATUS_CODES.join(', ') }) }}
          </p>
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

      <!-- Anthropic OAuth/SetupToken：TLS 指纹、会话 ID 伪装、缓存 TTL 覆盖 -->
      <div
        v-if="account?.platform === 'anthropic' && (account?.type === 'oauth' || account?.type === 'setup-token')"
        class="border-t border-af-hairline pt-4 space-y-4"
      >
        <!-- TLS Fingerprint -->
        <div class="rounded-lg border border-af-hairline p-4">
          <div class="flex items-center justify-between gap-4">
            <div>
              <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.tlsFingerprint.label') }}</label>
              <p class="mt-1 text-xs text-af-ink-3">
                {{ t('admin.accounts.quotaControl.tlsFingerprint.hint') }}
              </p>
            </div>
            <button
              type="button"
              @click="tlsFingerprintEnabled = !tlsFingerprintEnabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                tlsFingerprintEnabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  tlsFingerprintEnabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          <!-- Profile selector -->
          <div v-if="tlsFingerprintEnabled" class="mt-3">
            <select v-model="tlsFingerprintProfileId" class="input">
              <option :value="null">{{ t('admin.accounts.quotaControl.tlsFingerprint.defaultProfile') }}</option>
              <option v-if="tlsFingerprintProfiles.length > 0" :value="-1">{{ t('admin.accounts.quotaControl.tlsFingerprint.randomProfile') }}</option>
              <option v-for="p in tlsFingerprintProfiles" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </div>
        </div>

        <!-- Session ID Masking -->
        <div class="rounded-lg border border-af-hairline p-4">
          <div class="flex items-center justify-between gap-4">
            <div>
              <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.sessionIdMasking.label') }}</label>
              <p class="mt-1 text-xs text-af-ink-3">
                {{ t('admin.accounts.quotaControl.sessionIdMasking.hint') }}
              </p>
            </div>
            <button
              type="button"
              @click="sessionIdMaskingEnabled = !sessionIdMaskingEnabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                sessionIdMaskingEnabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  sessionIdMaskingEnabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
        </div>

        <!-- Cache TTL Override -->
        <div class="rounded-lg border border-af-hairline p-4">
          <div class="flex items-center justify-between gap-4">
            <div>
              <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.cacheTTLOverride.label') }}</label>
              <p class="mt-1 text-xs text-af-ink-3">
                {{ t('admin.accounts.quotaControl.cacheTTLOverride.hint') }}
              </p>
            </div>
            <button
              type="button"
              @click="cacheTTLOverrideEnabled = !cacheTTLOverrideEnabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                cacheTTLOverrideEnabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  cacheTTLOverrideEnabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          <div v-if="cacheTTLOverrideEnabled" class="mt-3">
            <label class="input-label text-xs">{{ t('admin.accounts.quotaControl.cacheTTLOverride.target') }}</label>
            <select
              v-model="cacheTTLOverrideTarget"
              class="mt-1 block w-full rounded-md border border-af-hairline-strong bg-af-sheet px-3 py-2 text-sm focus:border-af-brand focus:outline-none focus:ring-1 focus:ring-af-brand"
            >
              <option value="5m">5m</option>
              <option value="1h">1h</option>
            </select>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.quotaControl.cacheTTLOverride.targetHint') }}
            </p>
          </div>
        </div>
      </div>

      <!-- Grok OAuth client-tool prompt cache opt-in -->
      <div
        v-if="account.platform === 'grok' && account.type === 'oauth'"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <label class="input-label mb-0">{{ t('admin.accounts.grokClientToolCache.title') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.grokClientToolCache.hint') }}
            </p>
          </div>
          <Toggle
            v-model="grokClientToolCacheEnabled"
            data-testid="grok-client-tool-cache-toggle"
            :aria-label="t('admin.accounts.grokClientToolCache.title')"
          />
        </div>
      </div>

      <!-- Grok OAuth media generation eligibility override -->
      <div
        v-if="isGrokOAuthAccount"
        class="border-t border-af-hairline pt-4"
        data-testid="grok-media-eligibility-card"
      >
        <div class="space-y-3">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.grokMediaEligibility.title') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.grokMediaEligibility.hint') }}
            </p>
          </div>
          <select
            v-model="grokMediaEligibilityMode"
            class="input"
            data-testid="grok-media-eligibility-mode"
            :disabled="grokMediaEligibilityLoading"
          >
            <option value="auto">{{ t('admin.accounts.grokMediaEligibility.auto') }}</option>
            <option value="enabled">{{ t('admin.accounts.grokMediaEligibility.enabled') }}</option>
            <option value="disabled">{{ t('admin.accounts.grokMediaEligibility.disabled') }}</option>
          </select>
          <p v-if="grokMediaEligibilityLoading" class="text-xs text-af-ink-3">
            {{ t('admin.accounts.grokMediaEligibility.loading') }}
          </p>
          <p v-else-if="grokMediaEligibilityError" class="text-xs text-af-danger">
            {{ grokMediaEligibilityError }}
          </p>
          <div v-else-if="grokMediaEligibilityState" class="rounded-lg bg-af-sunken p-3 text-xs">
            <span class="font-medium">{{ t('admin.accounts.grokMediaEligibility.current') }}</span>
            <span class="ml-1" data-testid="grok-media-eligibility-status">
              {{ grokMediaEligibilityState.eligible ? t('admin.accounts.grokMediaEligibility.eligible') : t('admin.accounts.grokMediaEligibility.ineligible') }}
              · {{ t(`admin.accounts.grokMediaEligibility.reasons.${grokMediaEligibilityState.reason}`) }}
            </span>
          </div>
          <div
            v-if="grokMediaEligibilityMode === 'enabled'"
            class="rounded-lg bg-af-warning-tint p-3"
          >
            <p class="text-xs text-af-warning">
              <Icon name="exclamationTriangle" size="sm" class="mr-1 inline" :stroke-width="2" />
              {{ t('admin.accounts.grokMediaEligibility.forceEnableWarning') }}
            </p>
          </div>
          <p v-else-if="grokMediaEligibilityMode === 'auto'" class="text-xs text-af-ink-3">
            {{ t('admin.accounts.grokMediaEligibility.autoHint') }}
          </p>
        </div>
      </div>

      <div
        v-if="account?.platform === 'openai' && (account?.type === 'oauth' || account?.type === 'setup-token')"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLIOnly') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.openai.codexCLIOnlyDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="codexCLIOnlyEnabled = !codexCLIOnlyEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              codexCLIOnlyEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                codexCLIOnlyEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
        <div
          v-if="codexCLIOnlyEnabled"
          class="mt-4 flex items-center justify-between border-l-2 border-af-hairline pl-4"
        >
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.codexCLIOnlyAppServer') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.openai.codexCLIOnlyAppServerDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="codexCLIOnlyAppServerEnabled = !codexCLIOnlyAppServerEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              codexCLIOnlyAppServerEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                codexCLIOnlyAppServerEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <!-- Codex 指纹收敛模式（仅 OpenAI OAuth） -->
      <div
        v-if="account?.platform === 'openai' && account?.type === 'oauth'"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <label class="input-label mb-0">{{ t('admin.accounts.openai.codexFingerprintMode') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.openai.codexFingerprintModeDesc') }}
            </p>
          </div>
          <div class="w-52 flex-shrink-0">
            <Select v-model="codexFingerprintMode" data-testid="edit-codex-fingerprint-mode-select" :options="codexFingerprintModeOptions" />
          </div>
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
  OpenAICompactMode,
  OpenAIEndpointCapability,
  OllamaCloudUsageState,
  GrokMediaEligibilityMode,
  GrokMediaEligibilityState,
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
import GrokBaseUrlPresets from '@/components/account/GrokBaseUrlPresets.vue'
import CnBaseUrlPresets from '@/components/account/CnBaseUrlPresets.vue'
import ProtocolEndpointsEditor from '@/components/account/ProtocolEndpointsEditor.vue'
import {
  UPSTREAM_PROTOCOLS,
  applyPresetUrl,
  describeProtocolEndpointsIssue,
  currentProtocolOf,
  endpointsAfterDefaultsChange,
  preferredProtocolFor,
  hasAnthropicEndpoint,
  hasOpenAIEndpoint,
  loadProtocolDefaults,
  protocolDefaultsFor,
  trimProtocolEndpoints,
  validateProtocolEndpoints
} from '@/components/account/protocolEndpoints'
import {
  VENDORS_WITH_CODING_PLAN,
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
  applyPlanType,
  buildPlanTypeOptions,
  readPlanType,
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
  formatDateTime,
  formatDateTimeLocalInput,
  getBrowserTimeZone,
  parseDateTimeLocalInput
} from '@/utils/format'
import { createStableObjectKeyResolver } from '@/utils/stableObjectKey'
import { getAccountExpiryTimestamp } from '@/components/account/accountExpiry'
import { VERTEX_LOCATION_OPTIONS } from '@/constants/account'
import {
  OPENAI_WS_MODE_CTX_POOL,
  OPENAI_WS_MODE_OFF,
  OPENAI_WS_MODE_PASSTHROUGH,
  OPENAI_WS_MODE_HTTP_BRIDGE,
  isOpenAIWSModeEnabled,
  resolveOpenAIWSModeHintKey,
  type OpenAIWSMode,
  resolveOpenAIWSModeFromExtra
} from '@/utils/openaiWsMode'
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
const officialProtocolEndpoints = computed(() =>
  protocolDefaultsFor(protocolDefaults.value, props.account?.platform ?? '', protocolDefaultsMode.value)
)
watch(officialProtocolEndpoints, (next, previous) => {
  if (syncingForm.value) return
  editProtocolEndpoints.value = endpointsAfterDefaultsChange(
    editProtocolEndpoints.value,
    previous ?? {},
    next,
    // 编辑时平台不变，换模式保留当前协议
    currentProtocolOf(editProtocolEndpoints.value) ?? preferredProtocolFor(props.account?.platform ?? '')
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
// Grok 预设地址同时服务 Chat Completions 与 Responses。
function applyGrokPreset(url: string) {
  editProtocolEndpoints.value = applyPresetUrl(editProtocolEndpoints.value, ['chat_completions', 'responses'], url)
}
// Bedrock credentials
const editBedrockAccessKeyId = ref('')
const editBedrockSecretAccessKey = ref('')
const editBedrockSessionToken = ref('')
const editBedrockRegion = ref('')
const editBedrockForceGlobal = ref(false)
const editBedrockApiKeyValue = ref('')
const editVertexProjectId = ref('')
const editVertexClientEmail = ref('')
const editVertexLocation = ref('us-central1')
const isBedrockAPIKeyMode = computed(() =>
  props.account?.type === 'bedrock' &&
  (props.account?.credentials as Record<string, unknown>)?.auth_mode === 'apikey'
)
const modelMappings = ref<ModelMapping[]>([])
const openAICompactModelMappings = ref<ModelMapping[]>([])

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
const DEFAULT_POOL_MODE_RETRY_COUNT = 3
const MAX_POOL_MODE_RETRY_COUNT = 10
const DEFAULT_POOL_MODE_RETRY_STATUS_CODES = [401, 403, 429]
const GROK_CLIENT_TOOL_CACHE_EXTRA_KEY = 'grok_client_tool_cache_enabled'
const poolModeEnabled = ref(false)
const poolModeRetryCount = ref(DEFAULT_POOL_MODE_RETRY_COUNT)
const poolModeRetryStatusCodesInput = ref('')

function parsePoolModeRetryStatusCodes(input: string): number[] {
  if (!input || !input.trim()) return []
  const seen = new Set<number>()
  const out: number[] = []
  for (const token of input.split(/[,\s]+/)) {
    const trimmed = token.trim()
    if (!trimmed) continue
    const n = Number(trimmed)
    if (!Number.isFinite(n) || !Number.isInteger(n)) continue
    if (n < 100 || n > 599) continue
    if (seen.has(n)) continue
    seen.add(n)
    out.push(n)
  }
  return out.sort((a, b) => a - b)
}

function formatPoolModeRetryStatusCodes(value: unknown): string {
  if (!Array.isArray(value)) return ''
  const out: number[] = []
  const seen = new Set<number>()
  for (const v of value) {
    const n = typeof v === 'string' ? Number(v.trim()) : Number(v)
    if (!Number.isFinite(n) || !Number.isInteger(n)) continue
    if (n < 100 || n > 599) continue
    if (seen.has(n)) continue
    seen.add(n)
    out.push(n)
  }
  return out.sort((a, b) => a - b).join(', ')
}
const headerOverrideRows = ref<HeaderOverrideRow[]>([])

const headerOverrideCapable = computed(
  () => !!props.account && isHeaderOverrideCapable(props.account.platform, props.account.type)
)

// Grok Free OAuth accounts use client-tool prompt caching by default. Keep an
// explicit false in the account extra as the opt-out signal.
const grokClientToolCacheEnabled = ref(true)
const isGrokOAuthAccount = computed(
  () => props.account?.platform === 'grok' && props.account?.type === 'oauth'
)
const grokMediaEligibilityMode = ref<GrokMediaEligibilityMode>('auto')
const grokMediaEligibilityInitialMode = ref<GrokMediaEligibilityMode>('auto')
const grokMediaEligibilityState = ref<GrokMediaEligibilityState | null>(null)
const grokMediaEligibilityLoading = ref(false)
const grokMediaEligibilityError = ref('')
let grokMediaEligibilityRequestVersion = 0

const modeFromGrokMediaExtra = (extra: Record<string, unknown> | undefined): GrokMediaEligibilityMode => {
  if (extra?.grok_media_eligible === true) return 'enabled'
  if (extra?.grok_media_eligible === false) return 'disabled'
  return 'auto'
}

const loadGrokMediaEligibility = async (accountID: number): Promise<GrokMediaEligibilityState | null> => {
  if (!isGrokOAuthAccount.value || typeof adminAPI.accounts.getGrokMediaEligibility !== 'function') {
    return null
  }
  const requestVersion = ++grokMediaEligibilityRequestVersion
  grokMediaEligibilityLoading.value = true
  grokMediaEligibilityError.value = ''
  try {
    const state = await adminAPI.accounts.getGrokMediaEligibility(accountID)
    if (requestVersion !== grokMediaEligibilityRequestVersion) return null
    grokMediaEligibilityState.value = state
    grokMediaEligibilityMode.value = state.mode
    grokMediaEligibilityInitialMode.value = state.mode
    return state
  } catch (error: any) {
    if (requestVersion !== grokMediaEligibilityRequestVersion) return null
    grokMediaEligibilityError.value = error?.message || t('admin.accounts.grokMediaEligibility.loadFailed')
    return null
  } finally {
    if (requestVersion === grokMediaEligibilityRequestVersion) {
      grokMediaEligibilityLoading.value = false
    }
  }
}

const interceptWarmupRequests = ref(false)
const autoPause5hThreshold = ref<number | null>(null)
const autoPause7dThreshold = ref<number | null>(null)
const autoPause5hDisabled = ref(false)
const autoPause7dDisabled = ref(false)
const autoResetCreditEnabled = ref(false)
const autoResetCredit5hThreshold = ref(100)
const autoResetCredit7dThreshold = ref(100)
const upstreamBillingAutoProbeEnabled = ref(false)
const upstreamBillingRateSyncEnabled = ref(false)
const allowOverages = ref(false) // For antigravity accounts: enable AI Credits overages
const antigravityProjectId = ref('')
const isSyncingAntigravityUpstream = ref(false)
const getOpenAICompactModelMappingKey = createStableObjectKeyResolver<ModelMapping>('edit-openai-compact-model-mapping')


// Quota control state (Anthropic OAuth/SetupToken only)
const windowCostEnabled = ref(false)
const windowCostLimit = ref<number | null>(null)
const windowCostStickyReserve = ref<number | null>(null)
const sessionLimitEnabled = ref(false)
const maxSessions = ref<number | null>(null)
const sessionIdleTimeout = ref<number | null>(null)
const rpmLimitEnabled = ref(false)
const baseRpm = ref<number | null>(null)
const rpmStrategy = ref<'tiered' | 'sticky_exempt'>('tiered')
const rpmStickyBuffer = ref<number | null>(null)
const userMsgQueueMode = ref('')
const umqModeOptions = computed(() => [
  { value: '', label: t('admin.accounts.quotaControl.rpmLimit.umqModeOff') },
  { value: 'throttle', label: t('admin.accounts.quotaControl.rpmLimit.umqModeThrottle') },
  { value: 'serialize', label: t('admin.accounts.quotaControl.rpmLimit.umqModeSerialize') },
])
const tlsFingerprintEnabled = ref(false)
const tlsFingerprintProfileId = ref<number | null>(null)
const tlsFingerprintProfiles = ref<{ id: number; name: string }[]>([])
const sessionIdMaskingEnabled = ref(false)
const cacheTTLOverrideEnabled = ref(false)
const cacheTTLOverrideTarget = ref<string>('5m')

// OpenAI 自动透传开关（OAuth/API Key）
const openaiPassthroughEnabled = ref(false)
// OpenAI Codex namespace 工具摊平兼容开关（仅 OAuth），缺省关闭即原样保留
const openaiFlattenNamespacesEnabled = ref(false)
// OpenAI 订阅档位（Plus / Pro 20x / Pro 5x / Business Standard / Business Premium / Free）手动覆盖值,
// 存于 credentials.plan_type;'' 表示清空/自动识别
const editPlanType = ref<string>('')
const openAICompactMode = ref<OpenAICompactMode>('auto')
// Images 非流式响应缺 b64_json 时由网关下载 url 回填（仅 OpenAI API Key）。
const openAIImagesUrlToB64JsonEnabled = ref(false)
const openAIEndpointCapabilities = ref<OpenAIEndpointCapability[]>(['chat_completions', 'embeddings'])
const openaiOAuthResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
const openaiAPIKeyResponsesWebSocketV2Mode = ref<OpenAIWSMode>(OPENAI_WS_MODE_OFF)
const codexCLIOnlyEnabled = ref(false)
const codexCLIOnlyAppServerEnabled = ref(false)
type CodexFingerprintMode = 'off' | 'device' | 'session' | 'full'
const codexFingerprintMode = ref<CodexFingerprintMode>('off')
type CodexImageToolMode = 'inherit' | 'enabled' | 'disabled' | 'block'
const codexImageToolMode = ref<CodexImageToolMode>('inherit')
type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'
const anthropicPassthroughEnabled = ref(false)
const anthropicAPIKeyAuthScheme = ref<AnthropicAPIKeyAuthScheme>('x_api_key')
const webSearchEmulationEnabled = ref(false)
const bedrockCCCompatEnabled = ref(false)
const webSearchGlobalEnabled = ref(false)
// Anthropic 协议上的 key 设置按编辑中的协议地址展示，不看平台标签。
const anthropicKeySettingsVisible = computed(
  () => props.account?.type === 'apikey' && hasAnthropicEndpoint(editProtocolEndpoints.value)
)
// OpenAI Responses 协议设置（自动透传、WS mode、Compact）：成品号沿用 OpenAI 平台的规则；
// 第三方 key 按编辑中的协议地址（responses 或 chat_completions）展示，不看平台标签。
const openAIResponsesSettingsVisible = computed(() => {
  const account = props.account
  if (!account) return false
  if (account.type === 'apikey') return hasOpenAIEndpoint(editProtocolEndpoints.value)
  return account.platform === 'openai' && (account.type === 'oauth' || account.type === 'setup-token')
})

// 端点能力与生图结果转 base64 是第三方 key 专属设置：后端对任意标签的 key 都生效，
// 按编辑中的协议地址展示，不看平台标签。
const openAIKeySettingsVisible = computed(
  () => props.account?.type === 'apikey' && hasOpenAIEndpoint(editProtocolEndpoints.value)
)

// 表单分区（A5-c）：「基本」「额度」「高级」总有字段；「地址与协议」「模型与映射」只在分区里有区块时才出标题，
// 条件与分区内各区块的 v-if 一一对应（改区块条件时这里一起改）。
const showEndpointSection = computed(() => {
  const account = props.account
  if (!account) return false
  return (
    account.type === 'apikey' ||
    account.type === 'bedrock' ||
    (account.platform === 'openai' && (account.type === 'oauth' || account.type === 'setup-token')) ||
    openAIResponsesSettingsVisible.value ||
    openAIKeySettingsVisible.value ||
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
// Load global feature states once
adminAPI.settings.getWebSearchEmulationConfig().then(cfg => {
  webSearchGlobalEnabled.value = cfg?.enabled === true && (cfg?.providers?.length ?? 0) > 0
}).catch(() => { webSearchGlobalEnabled.value = false })

const editQuotaLimit = ref<number | null>(null)
const editQuotaDailyLimit = ref<number | null>(null)
const editQuotaWeeklyLimit = ref<number | null>(null)
const codexFingerprintModeOptions = computed(() => [
  { value: 'off' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintOff') },
  { value: 'device' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintDevice') },
  { value: 'session' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintSession') },
  { value: 'full' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintFull') },
])

const openAIWSModeOptions = computed(() => [
  { value: OPENAI_WS_MODE_OFF, label: t('admin.accounts.openai.wsModeOff') },
  { value: OPENAI_WS_MODE_CTX_POOL, label: t('admin.accounts.openai.wsModeCtxPool') },
  { value: OPENAI_WS_MODE_PASSTHROUGH, label: t('admin.accounts.openai.wsModePassthrough') },
  { value: OPENAI_WS_MODE_HTTP_BRIDGE, label: t('admin.accounts.openai.wsModeHttpBridge') }
])
const openaiResponsesWebSocketV2Mode = computed({
  get: () => {
    if (props.account?.type === 'apikey') {
      return openaiAPIKeyResponsesWebSocketV2Mode.value
    }
    return openaiOAuthResponsesWebSocketV2Mode.value
  },
  set: (mode: OpenAIWSMode) => {
    if (props.account?.type === 'apikey') {
      openaiAPIKeyResponsesWebSocketV2Mode.value = mode
      return
    }
    openaiOAuthResponsesWebSocketV2Mode.value = mode
  }
})
const openAIWSModeHintKey = computed(() =>
  resolveOpenAIWSModeHintKey(openaiResponsesWebSocketV2Mode.value)
)
const codexImageToolOptions = computed<Array<{
  value: CodexImageToolMode
  label: string
  description: string
  selectedCardClass: string
  selectedDotClass: string
}>>(() => [
  {
    value: 'inherit',
    label: t('admin.accounts.openai.codexImageToolInherit'),
    description: t('admin.accounts.openai.codexImageToolInheritDesc'),
    selectedCardClass: 'border-af-hairline bg-af-sunken text-af-ink-2 ring-1 ring-af-hairline',
    selectedDotClass: 'border-af-ink-3 bg-af-ink text-af-on-brand'
  },
  {
    value: 'enabled',
    label: t('admin.accounts.openai.codexImageToolEnabled'),
    description: t('admin.accounts.openai.codexImageToolEnabledDesc'),
    selectedCardClass: 'border-af-success/30 bg-af-success-tint text-af-success ring-1 ring-af-success/30',
    selectedDotClass: 'border-af-success bg-af-success text-af-on-brand'
  },
  {
    value: 'disabled',
    label: t('admin.accounts.openai.codexImageToolDisabled'),
    description: t('admin.accounts.openai.codexImageToolDisabledDesc'),
    selectedCardClass: 'border-af-warning/30 bg-af-warning-tint text-af-warning ring-1 ring-af-warning/30',
    selectedDotClass: 'border-af-warning bg-af-warning text-af-on-brand'
  },
  {
    value: 'block',
    label: t('admin.accounts.openai.codexImageToolBlock'),
    description: t('admin.accounts.openai.codexImageToolBlockDesc'),
    selectedCardClass: 'border-af-danger/30 bg-af-danger-tint text-af-danger ring-1 ring-af-danger/30',
    selectedDotClass: 'border-af-danger bg-af-danger text-af-on-brand'
  }
])
const codexImageToolBadgeLabel = computed(() => {
  switch (codexImageToolMode.value) {
    case 'enabled':
      return t('admin.accounts.openai.codexImageToolBadgeEnabled')
    case 'disabled':
      return t('admin.accounts.openai.codexImageToolBadgeDisabled')
    case 'block':
      return t('admin.accounts.openai.codexImageToolBadgeBlock')
    default:
      return t('admin.accounts.openai.codexImageToolBadgeInherit')
  }
})
const codexImageToolBadgeClass = computed(() => {
  switch (codexImageToolMode.value) {
    case 'enabled':
      return 'bg-af-success-tint text-af-success'
    case 'disabled':
      return 'bg-af-warning-tint text-af-warning'
    case 'block':
      return 'bg-af-danger-tint text-af-danger'
    default:
      return 'bg-af-sunken text-af-ink-2'
  }
})
const openAICompactModeOptions = computed(() => [
  { value: 'auto', label: t('admin.accounts.openai.compactModeAuto') },
  { value: 'force_on', label: t('admin.accounts.openai.compactModeForceOn') },
  { value: 'force_off', label: t('admin.accounts.openai.compactModeForceOff') }
])
// OpenAI 订阅档位手动覆盖选项(清空 + Plus/Pro/Free;别名/自定义值友好显示且保留 canonical)
const planTypeOptions = computed(() =>
  buildPlanTypeOptions(editPlanType.value, t('admin.accounts.openai.planTypeClear'))
)
const openAIEndpointCapabilityOptions = computed<{ value: OpenAIEndpointCapability; label: string }[]>(() => [
  { value: 'chat_completions', label: t('admin.accounts.openai.capabilityText') },
  { value: 'embeddings', label: t('admin.accounts.openai.capabilityEmbeddings') }
])

const normalizeOpenAIEndpointCapabilities = (values: OpenAIEndpointCapability[]) => {
  const allowed: OpenAIEndpointCapability[] = ['chat_completions', 'embeddings']
  const selected = allowed.filter((value) => values.includes(value))
  return selected.length > 0 ? selected : allowed
}

const readOpenAIEndpointCapabilities = (credentials?: Record<string, unknown>): OpenAIEndpointCapability[] => {
  const raw = credentials?.openai_capabilities
  if (Array.isArray(raw)) {
    return normalizeOpenAIEndpointCapabilities(
      raw.filter((value): value is OpenAIEndpointCapability =>
        value === 'chat_completions' || value === 'embeddings'
      )
    )
  }
  if (raw !== null && typeof raw === 'object') {
    const capabilityMap = raw as Record<string, unknown>
    return normalizeOpenAIEndpointCapabilities(
      openAIEndpointCapabilityOptions.value
        .map((option) => option.value)
        .filter((value) => capabilityMap[value] === true)
    )
  }
  return ['chat_completions', 'embeddings']
}

const toggleOpenAIEndpointCapability = (capability: OpenAIEndpointCapability, event?: Event) => {
  if (openAIEndpointCapabilities.value.includes(capability)) {
    if (openAIEndpointCapabilities.value.length <= 1) {
      const input = event?.target as HTMLInputElement | null
      if (input) input.checked = true
      return
    }
    openAIEndpointCapabilities.value = openAIEndpointCapabilities.value.filter(
      (value) => value !== capability
    )
    return
  }
  openAIEndpointCapabilities.value = normalizeOpenAIEndpointCapabilities([
    ...openAIEndpointCapabilities.value,
    capability
  ])
}

const applyOpenAIEndpointCapabilities = (credentials: Record<string, unknown>) => {
  const capabilities = normalizeOpenAIEndpointCapabilities(openAIEndpointCapabilities.value)
  if (capabilities.length === 2) {
    delete credentials.openai_capabilities
    return
  }
  credentials.openai_capabilities = capabilities
}
// 自动透传会跳过模型改写：透传区块可见且开启时，模型限制不再可编辑。
const isOpenAIModelRestrictionDisabled = computed(() =>
  openAIResponsesSettingsVisible.value && openaiPassthroughEnabled.value
)
const openAICompactStatusKey = computed(() => {
  const extra = props.account?.extra as Record<string, unknown> | undefined
  if (!props.account) return ''
  const mode = typeof extra?.openai_compact_mode === 'string' ? extra.openai_compact_mode : 'auto'
  if (mode === 'force_on') return 'admin.accounts.openai.compactSupported'
  if (mode === 'force_off') return 'admin.accounts.openai.compactUnsupported'
  if (typeof extra?.openai_compact_supported === 'boolean') {
    return extra.openai_compact_supported
      ? 'admin.accounts.openai.compactSupported'
      : 'admin.accounts.openai.compactUnsupported'
  }
  return 'admin.accounts.openai.compactAuto'
})

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
const normalizePoolModeRetryCount = (value: number) => {
  if (!Number.isFinite(value)) {
    return DEFAULT_POOL_MODE_RETRY_COUNT
  }
  const normalized = Math.trunc(value)
  if (normalized < 0) {
    return 0
  }
  if (normalized > MAX_POOL_MODE_RETRY_COUNT) {
    return MAX_POOL_MODE_RETRY_COUNT
  }
  return normalized
}

// 映射整份按改名行展示：旧白名单留下的同名项也在——对承接没影响，但 Antigravity / xAI 这类自带模型表的
// 上游靠它扩表、批量生图也按映射列模型，不能静默丢掉，管理员可以自己删。
const loadModelRestrictionFromMapping = (rawMapping?: Record<string, unknown>) => {
  modelMappings.value = Object.entries(rawMapping ?? {}).flatMap(([from, to]) =>
    typeof to === 'string' && from.trim() && to.trim() ? [{ from: from.trim(), to: to.trim() }] : []
  )
}

// 写映射并打「只改名」标记（muqian 2026-09-25 去掉白名单）。spark 影子账号的映射是系统维护的模型集合，
// 后端也只放行 model_mapping / compact_model_mapping 两个键，不打标记。
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

const applyOpenAIModelMappingCredentials = (credentials: Record<string, unknown>) => {
  const shouldApplyModelMapping = !openaiPassthroughEnabled.value

  if (shouldApplyModelMapping) {
    writeRenameMapping(credentials)
  } else if (!credentials.model_mapping) {
    delete credentials.model_mapping
  }

  const compactModelMapping = buildModelMappingObject('mapping', [], openAICompactModelMappings.value)
  if (compactModelMapping) {
    credentials.compact_model_mapping = compactModelMapping
  } else {
    delete credentials.compact_model_mapping
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
  editVertexProjectId.value = ''
  editVertexClientEmail.value = ''
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
	openAIImagesUrlToB64JsonEnabled.value = extra?.images_url_to_b64_json === true
	autoPause5hThreshold.value = typeof extra?.auto_pause_5h_threshold === 'number' ? extra.auto_pause_5h_threshold * 100 : null
	autoPause7dThreshold.value = typeof extra?.auto_pause_7d_threshold === 'number' ? extra.auto_pause_7d_threshold * 100 : null
	autoPause5hDisabled.value = extra?.auto_pause_5h_disabled === true
	autoPause7dDisabled.value = extra?.auto_pause_7d_disabled === true
	autoResetCreditEnabled.value = extra?.auto_reset_credit_enabled === true
	autoResetCredit5hThreshold.value =
		typeof extra?.auto_reset_credit_5h_threshold === 'number' ? extra.auto_reset_credit_5h_threshold * 100 : 100
	autoResetCredit7dThreshold.value =
		typeof extra?.auto_reset_credit_7d_threshold === 'number' ? extra.auto_reset_credit_7d_threshold * 100 : 100
	upstreamBillingAutoProbeEnabled.value = extra?.upstream_billing_probe_enabled === true
  upstreamBillingRateSyncEnabled.value =
    upstreamBillingAutoProbeEnabled.value && extra?.upstream_billing_rate_sync_enabled === true

  // Load OpenAI passthrough toggle (OpenAI OAuth/SetupToken/API Key)
  openaiPassthroughEnabled.value = false
  openaiFlattenNamespacesEnabled.value = false
  editPlanType.value = ''
  openAICompactMode.value = 'auto'
  openAIEndpointCapabilities.value = ['chat_completions', 'embeddings']
  openAICompactModelMappings.value = []
  openaiOAuthResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
  openaiAPIKeyResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
  codexCLIOnlyEnabled.value = false
  codexCLIOnlyAppServerEnabled.value = false
  codexFingerprintMode.value = 'off'
  codexImageToolMode.value = 'inherit'
  anthropicPassthroughEnabled.value = false
  anthropicAPIKeyAuthScheme.value = 'x_api_key'
  webSearchEmulationEnabled.value = false
  bedrockCCCompatEnabled.value = false
  // OpenAI Responses 协议设置（自动透传 / WS mode / Compact）：OpenAI 成品号与所有第三方 key 都回填，
  // key 的区块随协议地址行显隐
  if (
    newAccount.type === 'apikey' ||
    (newAccount.platform === 'openai' && (newAccount.type === 'oauth' || newAccount.type === 'setup-token'))
  ) {
    openaiPassthroughEnabled.value = extra?.openai_passthrough === true || extra?.openai_oauth_passthrough === true
    openAICompactMode.value = (extra?.openai_compact_mode as OpenAICompactMode) || 'auto'
    openaiOAuthResponsesWebSocketV2Mode.value = resolveOpenAIWSModeFromExtra(extra, {
      modeKey: 'openai_oauth_responses_websockets_v2_mode',
      enabledKey: 'openai_oauth_responses_websockets_v2_enabled',
      fallbackEnabledKeys: ['responses_websockets_v2_enabled', 'openai_ws_enabled'],
      defaultMode: OPENAI_WS_MODE_OFF
    })
    openaiAPIKeyResponsesWebSocketV2Mode.value = resolveOpenAIWSModeFromExtra(extra, {
      modeKey: 'openai_apikey_responses_websockets_v2_mode',
      enabledKey: 'openai_apikey_responses_websockets_v2_enabled',
      fallbackEnabledKeys: ['responses_websockets_v2_enabled', 'openai_ws_enabled'],
      defaultMode: OPENAI_WS_MODE_OFF
    })
    const compactMappings = credentials?.compact_model_mapping as Record<string, string> | undefined
    if (compactMappings && typeof compactMappings === 'object') {
      openAICompactModelMappings.value = Object.entries(compactMappings).map(([from, to]) => ({ from, to }))
    }
  }
  // 端点能力是第三方 key 专属设置，与平台标签无关：任何标签的 key 都要回填，
  // 否则保存时会把已存的能力限制覆盖成默认值。必须放在上面的默认值重置之后。
  if (newAccount.type === 'apikey') {
    openAIEndpointCapabilities.value = readOpenAIEndpointCapabilities(
      newAccount.credentials as Record<string, unknown> | undefined
    )
  }
  // 长上下文计费开关对任意标签的 key 都生效，按已存值回填；区块可见性另算。
  // OpenAI 平台专属设置（成品号语义；openai 标签的 key 仍沿用，待协议化）
  if (newAccount.platform === 'openai' && (newAccount.type === 'oauth' || newAccount.type === 'setup-token' || newAccount.type === 'apikey')) {
    openaiFlattenNamespacesEnabled.value =
      newAccount.type === 'oauth' && extra?.openai_responses_flatten_namespaces === true
    // plan_type 手动覆盖仅 OAuth 有实际调度语义(IsOpenAIChatGPTSubscription 要求 oauth),故只对 oauth 回填
    editPlanType.value = newAccount.type === 'oauth'
      ? readPlanType(newAccount.credentials as Record<string, unknown> | undefined)
      : ''
    const codexImageGenerationBridgeValue = typeof extra?.codex_image_generation_bridge === 'boolean'
      ? extra.codex_image_generation_bridge
      : extra?.codex_image_generation_bridge_enabled
    if (extra?.codex_image_generation_explicit_tool_policy === 'strip') {
      codexImageToolMode.value = 'block'
    } else if (codexImageGenerationBridgeValue === true) {
      codexImageToolMode.value = 'enabled'
    } else if (codexImageGenerationBridgeValue === false) {
      codexImageToolMode.value = 'disabled'
    }
    if (newAccount.type === 'oauth' || newAccount.type === 'setup-token') {
      codexCLIOnlyEnabled.value = extra?.codex_cli_only === true
      codexCLIOnlyAppServerEnabled.value =
        extra?.codex_cli_only_allow_app_server === true
    }
    if (newAccount.type === 'oauth') {
      const fpMode = extra?.codex_fingerprint_mode as string | undefined
      // 缺省/非法值按 off 呈现，与后端 GetCodexFingerprintMode 的 opt-in 语义一致（#5610）
      codexFingerprintMode.value = (['off', 'device', 'session', 'full'].includes(fpMode || '')
        ? fpMode as CodexFingerprintMode
        : 'off')
    }
  }
  // 第三方 key 一律回填：地址行可在弹窗里增删，区块是否展示随地址变化
  if (newAccount.type === 'apikey') {
    anthropicPassthroughEnabled.value = extra?.anthropic_passthrough === true
    anthropicAPIKeyAuthScheme.value = extra?.anthropic_apikey_auth_scheme === 'authorization_bearer'
      ? 'authorization_bearer'
      : 'x_api_key'
    // 开关写 bool；历史字符串只有 "enabled" 算开（与后端读法一致），保存后落成 bool
    const wsVal = extra?.web_search_emulation
    webSearchEmulationEnabled.value = wsVal === true || wsVal === 'enabled'
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

  const grokClientToolCacheSetting =
    newAccount.platform === 'grok' && newAccount.type === 'oauth'
      ? newAccount.extra?.[GROK_CLIENT_TOOL_CACHE_EXTRA_KEY]
      : undefined
  grokClientToolCacheEnabled.value =
    newAccount.platform === 'grok' &&
    newAccount.type === 'oauth' &&
    (grokClientToolCacheSetting === undefined || grokClientToolCacheSetting === true)
  grokMediaEligibilityMode.value = modeFromGrokMediaExtra(extra)
  grokMediaEligibilityInitialMode.value = grokMediaEligibilityMode.value
  grokMediaEligibilityState.value = null
  grokMediaEligibilityError.value = ''
  if (newAccount.platform === 'grok' && newAccount.type === 'oauth') {
    void loadGrokMediaEligibility(newAccount.id)
  } else {
    grokMediaEligibilityRequestVersion++
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
      editBedrockSessionToken.value = ''
    }

    // Load pool mode for bedrock
    poolModeEnabled.value = bedrockCreds.pool_mode === true
    const retryCount = bedrockCreds.pool_mode_retry_count
    poolModeRetryCount.value = (typeof retryCount === 'number' && retryCount >= 0) ? retryCount : DEFAULT_POOL_MODE_RETRY_COUNT
    poolModeRetryStatusCodesInput.value = formatPoolModeRetryStatusCodes(bedrockCreds.pool_mode_retry_status_codes)

    // Load quota limits for bedrock
    const bedrockExtra = (newAccount.extra as Record<string, unknown>) || {}
    editQuotaLimit.value = typeof bedrockExtra.quota_limit === 'number' ? bedrockExtra.quota_limit : null
    editQuotaDailyLimit.value = typeof bedrockExtra.quota_daily_limit === 'number' ? bedrockExtra.quota_daily_limit : null
    editQuotaWeeklyLimit.value = typeof bedrockExtra.quota_weekly_limit === 'number' ? bedrockExtra.quota_weekly_limit : null

    // Load model mappings for bedrock
    loadModelRestrictionFromMapping(bedrockCreds.model_mapping as Record<string, unknown> | undefined)
  } else if ((newAccount.platform === 'gemini' || newAccount.platform === 'anthropic') && newAccount.type === 'service_account' && newAccount.credentials) {
    const credentials = newAccount.credentials as Record<string, unknown>
    editVertexProjectId.value = (credentials.project_id as string) || ''
    editVertexClientEmail.value = (credentials.client_email as string) || ''
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
    poolModeRetryCount.value = DEFAULT_POOL_MODE_RETRY_COUNT
    poolModeRetryStatusCodesInput.value = ''
  }
  editApiKey.value = ''
}

async function loadTLSProfiles() {
  try {
    const profiles = await adminAPI.tlsFingerprintProfiles.list()
    tlsFingerprintProfiles.value = profiles.map(p => ({ id: p.id, name: p.name }))
  } catch {
    tlsFingerprintProfiles.value = []
  }
}

watch(
  [() => props.show, () => props.account],
  ([show, newAccount], [wasShow, previousAccount]) => {
    if (!show || !newAccount) {
      return
    }
    if (!wasShow || newAccount !== previousAccount) {
      syncFormFromAccount(newAccount)
      loadTLSProfiles()
    }
  },
  { immediate: true }
)

// Model mapping helpers
const addOpenAICompactModelMapping = () => {
  openAICompactModelMappings.value.push({ from: '', to: '' })
}

const removeOpenAICompactModelMapping = (index: number) => {
  openAICompactModelMappings.value.splice(index, 1)
}

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
  windowCostEnabled.value = false
  windowCostLimit.value = null
  windowCostStickyReserve.value = null
  sessionLimitEnabled.value = false
  maxSessions.value = null
  sessionIdleTimeout.value = null
  rpmLimitEnabled.value = false
  baseRpm.value = null
  rpmStrategy.value = 'tiered'
  rpmStickyBuffer.value = null
  userMsgQueueMode.value = ''
  tlsFingerprintEnabled.value = false
  tlsFingerprintProfileId.value = null
  sessionIdMaskingEnabled.value = false
  cacheTTLOverrideEnabled.value = false
  cacheTTLOverrideTarget.value = '5m'

  // Remaining quota control settings only apply to Anthropic accounts
  if (account.platform !== 'anthropic') {
    return
  }

  // Window cost / session limit only apply to Anthropic OAuth/SetupToken accounts
  if (account.type !== 'oauth' && account.type !== 'setup-token') {
    return
  }

  // Load from extra field (via backend DTO fields)
  if (account.window_cost_limit != null && account.window_cost_limit > 0) {
    windowCostEnabled.value = true
    windowCostLimit.value = account.window_cost_limit
    windowCostStickyReserve.value = account.window_cost_sticky_reserve ?? 10
  }

  if (account.max_sessions != null && account.max_sessions > 0) {
    sessionLimitEnabled.value = true
    maxSessions.value = account.max_sessions
    sessionIdleTimeout.value = account.session_idle_timeout_minutes ?? 5
  }

  // RPM limit
  if (account.base_rpm != null && account.base_rpm > 0) {
    rpmLimitEnabled.value = true
    baseRpm.value = account.base_rpm
    rpmStrategy.value = (account.rpm_strategy as 'tiered' | 'sticky_exempt') || 'tiered'
    rpmStickyBuffer.value = account.rpm_sticky_buffer ?? null
  }

  // UMQ mode（独立于 RPM 加载，防止编辑无 RPM 账号时丢失已有配置）
  userMsgQueueMode.value = account.user_msg_queue_mode ?? ''

  // Load TLS fingerprint setting
  if (account.enable_tls_fingerprint === true) {
    tlsFingerprintEnabled.value = true
  }
  tlsFingerprintProfileId.value = account.tls_fingerprint_profile_id ?? null

  // Load session ID masking setting
  if (account.session_id_masking_enabled === true) {
    sessionIdMaskingEnabled.value = true
  }

  // Load cache TTL override setting
  if (account.cache_ttl_override_enabled === true) {
    cacheTTLOverrideEnabled.value = true
    cacheTTLOverrideTarget.value = account.cache_ttl_override_target || '5m'
  }
}

const formatDateTimeLocal = formatDateTimeLocalInput
const parseDateTimeLocal = parseDateTimeLocalInput

// Methods
const handleClose = () => {
  emit('close')
}

const persistGrokMediaEligibility = async (accountID: number, updatedAccount: Account): Promise<Account> => {
  if (
    !isGrokOAuthAccount.value ||
    grokMediaEligibilityMode.value === grokMediaEligibilityInitialMode.value ||
    typeof adminAPI.accounts.updateGrokMediaEligibility !== 'function'
  ) {
    return updatedAccount
  }

  try {
    const state = await adminAPI.accounts.updateGrokMediaEligibility(accountID, grokMediaEligibilityMode.value)
    grokMediaEligibilityState.value = state
    grokMediaEligibilityInitialMode.value = state.mode
    const nextExtra = { ...((updatedAccount.extra as Record<string, unknown> | undefined) || {}) }
    if (state.mode === 'auto') {
      delete nextExtra.grok_media_eligible
    } else {
      nextExtra.grok_media_eligible = state.mode === 'enabled'
    }
    updatedAccount.extra = nextExtra
  } catch (error: any) {
    appStore.showError(t('admin.accounts.grokMediaEligibility.partialSave'))
    try {
      const state = await loadGrokMediaEligibility(accountID)
      if (state) {
        const nextExtra = { ...((updatedAccount.extra as Record<string, unknown> | undefined) || {}) }
        if (state.mode === 'auto') delete nextExtra.grok_media_eligible
        else nextExtra.grok_media_eligible = state.mode === 'enabled'
        updatedAccount.extra = nextExtra
      }
    } catch {
      // The original save result remains useful even when the refresh fails.
    }
  }
  return updatedAccount
}

const submitUpdateAccount = async (accountID: number, updatePayload: Record<string, unknown>) => {
  submitting.value = true
  try {
    let updatedAccount = await adminAPI.accounts.update(accountID, updatePayload)
    updatedAccount = await persistGrokMediaEligibility(accountID, updatedAccount)
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
	if (autoResetCreditEnabled.value) {
		const thresholds = [autoResetCredit5hThreshold.value, autoResetCredit7dThreshold.value]
		if (thresholds.some((value) => !Number.isFinite(value) || value < 0.1 || value > 100)) {
			appStore.showError(t('admin.accounts.autoResetCredit.thresholdInvalid'))
			return
		}
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
      const shouldApplyModelMapping = !isOpenAIModelRestrictionDisabled.value

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

      // Add model mapping if configured（OpenAI 开启自动透传时保留现有映射，不再编辑）
      if (shouldApplyModelMapping) {
        writeRenameMapping(newCredentials)
      } else if (currentCredentials.model_mapping) {
        newCredentials.model_mapping = currentCredentials.model_mapping
      }
      if (openAIKeySettingsVisible.value) {
        applyOpenAIEndpointCapabilities(newCredentials)
      }
      // Compact 专属模型映射与 Compact 模式同区块，区块隐藏时保留已存值
      if (openAIResponsesSettingsVisible.value) {
        const compactModelMapping = buildModelMappingObject('mapping', [], openAICompactModelMappings.value)
        if (compactModelMapping) {
          newCredentials.compact_model_mapping = compactModelMapping
        } else {
          delete newCredentials.compact_model_mapping
        }
      }

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

      if (!editVertexProjectId.value.trim()) {
        appStore.showError(t('admin.accounts.vertexSaJsonMissingProjectId'))
        return
      }
      if (!editVertexClientEmail.value.trim()) {
        appStore.showError(t('admin.accounts.vertexSaJsonMissingClientEmail'))
        return
      }
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
      newCredentials.project_id = editVertexProjectId.value.trim()
      newCredentials.client_email = editVertexClientEmail.value.trim()
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
        if (editBedrockSessionToken.value.trim()) {
          newCredentials.aws_session_token = editBedrockSessionToken.value.trim()
        }
      }

      // Pool mode
      if (poolModeEnabled.value) {
        newCredentials.pool_mode = true
        newCredentials.pool_mode_retry_count = normalizePoolModeRetryCount(poolModeRetryCount.value)
        const parsedRetryStatusCodes = parsePoolModeRetryStatusCodes(poolModeRetryStatusCodesInput.value)
        if (parsedRetryStatusCodes.length > 0) {
          newCredentials.pool_mode_retry_status_codes = parsedRetryStatusCodes
        } else {
          delete newCredentials.pool_mode_retry_status_codes
        }
      } else {
        delete newCredentials.pool_mode
        delete newCredentials.pool_mode_retry_count
        delete newCredentials.pool_mode_retry_status_codes
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
      if (props.account.platform === 'openai') {
        applyOpenAIModelMappingCredentials(newCredentials)
      } else {
        writeRenameMapping(newCredentials)
      }

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

      const newExtra: Record<string, unknown> = {
        ...((props.account.extra as Record<string, unknown>) || {})
      }
      // Persist both states so a disabled account remains opted out when the
      // backend applies the default-enabled policy to missing values.
      newExtra[GROK_CLIENT_TOOL_CACHE_EXTRA_KEY] = grokClientToolCacheEnabled.value
      updatePayload.extra = newExtra
    }

    // OpenAI: 手动覆盖订阅档位 plan_type（Plus / Pro 20x / Pro 5x / Business Standard / Business Premium / Free）。
    // 仅 OAuth 非影子账号：
    // 影子账号凭据由母账号管理(且后端会 sanitize),setup-token 无订阅调度语义。
    if (props.account.platform === 'openai' && props.account.type === 'oauth' && !isSparkShadow.value) {
      const currentCredentials = (updatePayload.credentials as Record<string, unknown>) ||
        ((props.account.credentials as Record<string, unknown>) || {})
      updatePayload.credentials = applyPlanType({ ...currentCredentials }, editPlanType.value)
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

      // Window cost limit settings
      if (windowCostEnabled.value && windowCostLimit.value != null && windowCostLimit.value > 0) {
        newExtra.window_cost_limit = windowCostLimit.value
        newExtra.window_cost_sticky_reserve = windowCostStickyReserve.value ?? 10
      } else {
        delete newExtra.window_cost_limit
        delete newExtra.window_cost_sticky_reserve
      }

      // Session limit settings
      if (sessionLimitEnabled.value && maxSessions.value != null && maxSessions.value > 0) {
        newExtra.max_sessions = maxSessions.value
        newExtra.session_idle_timeout_minutes = sessionIdleTimeout.value ?? 5
      } else {
        delete newExtra.max_sessions
        delete newExtra.session_idle_timeout_minutes
      }

      // RPM limit settings
      if (rpmLimitEnabled.value) {
        const DEFAULT_BASE_RPM = 15
        newExtra.base_rpm = (baseRpm.value != null && baseRpm.value > 0)
          ? baseRpm.value
          : DEFAULT_BASE_RPM
        newExtra.rpm_strategy = rpmStrategy.value
        if (rpmStickyBuffer.value != null && rpmStickyBuffer.value > 0) {
          newExtra.rpm_sticky_buffer = rpmStickyBuffer.value
        } else {
          delete newExtra.rpm_sticky_buffer
        }
      } else {
        delete newExtra.base_rpm
        delete newExtra.rpm_strategy
        delete newExtra.rpm_sticky_buffer
      }

      // UMQ mode（独立于 RPM 保存）
      if (userMsgQueueMode.value) {
        newExtra.user_msg_queue_mode = userMsgQueueMode.value
      } else {
        delete newExtra.user_msg_queue_mode
      }
      delete newExtra.user_msg_queue_enabled  // 清理旧字段

      // TLS fingerprint setting
      if (tlsFingerprintEnabled.value) {
        newExtra.enable_tls_fingerprint = true
        if (tlsFingerprintProfileId.value) {
          newExtra.tls_fingerprint_profile_id = tlsFingerprintProfileId.value
        } else {
          delete newExtra.tls_fingerprint_profile_id
        }
      } else {
        delete newExtra.enable_tls_fingerprint
        delete newExtra.tls_fingerprint_profile_id
      }

      // Session ID masking setting
      if (sessionIdMaskingEnabled.value) {
        newExtra.session_id_masking_enabled = true
      } else {
        delete newExtra.session_id_masking_enabled
      }

      // Cache TTL override setting
      if (cacheTTLOverrideEnabled.value) {
        newExtra.cache_ttl_override_enabled = true
        newExtra.cache_ttl_override_target = cacheTTLOverrideTarget.value
      } else {
        delete newExtra.cache_ttl_override_enabled
        delete newExtra.cache_ttl_override_target
      }

      updatePayload.extra = newExtra
    }

    // 第三方 key 的 Anthropic 协议设置（透传 / 认证方式 / web search 模拟）写入 extra。
    // 区块隐藏（没有 anthropic 地址）时不写界面上的值，账号已存的值原样保留，与其他隐藏区块一致。
    if (anthropicKeySettingsVisible.value) {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (anthropicPassthroughEnabled.value) {
        newExtra.anthropic_passthrough = true
      } else {
        delete newExtra.anthropic_passthrough
      }
      if (anthropicAPIKeyAuthScheme.value === 'authorization_bearer') {
        newExtra.anthropic_apikey_auth_scheme = 'authorization_bearer'
      } else {
        delete newExtra.anthropic_apikey_auth_scheme
      }
      if (webSearchEmulationEnabled.value) {
        newExtra.web_search_emulation = true
      } else {
        delete newExtra.web_search_emulation
      }
      if (bedrockCCCompatEnabled.value) {
        newExtra.bedrock_cc_compat = true
      } else {
        delete newExtra.bedrock_cc_compat
      }
      updatePayload.extra = newExtra
    }

    // OpenAI Responses 协议设置（自动透传 / WS mode / Compact 模式）写入 extra。
    // 区块隐藏（第三方 key 没有 responses / chat_completions 地址）时不写界面值，已存值原样保留。
    if (openAIResponsesSettingsVisible.value) {
      // 接着前面区块写好的 extra 改：key 可能同时配了 anthropic 地址，从账号原 extra 重来会把
      // 上面写入的 Anthropic 协议设置冲掉。
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      if (props.account.type === 'apikey') {
        newExtra.openai_apikey_responses_websockets_v2_mode = openaiAPIKeyResponsesWebSocketV2Mode.value
        newExtra.openai_apikey_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiAPIKeyResponsesWebSocketV2Mode.value)
      } else {
        newExtra.openai_oauth_responses_websockets_v2_mode = openaiOAuthResponsesWebSocketV2Mode.value
        newExtra.openai_oauth_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiOAuthResponsesWebSocketV2Mode.value)
      }
      delete newExtra.responses_websockets_v2_enabled
      delete newExtra.openai_ws_enabled
      if (openaiPassthroughEnabled.value) {
        newExtra.openai_passthrough = true
      } else {
        delete newExtra.openai_passthrough
        delete newExtra.openai_oauth_passthrough
      }
      if (openAICompactMode.value === 'auto') {
        delete newExtra.openai_compact_mode
      } else {
        newExtra.openai_compact_mode = openAICompactMode.value
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
      // 生图结果转 base64 与端点能力同区块：区块隐藏时保留已存值，不按界面值改写。
      if (openAIKeySettingsVisible.value) {
        if (openAIImagesUrlToB64JsonEnabled.value) {
          newExtra.images_url_to_b64_json = true
        } else {
          delete newExtra.images_url_to_b64_json
        }
      }
      updatePayload.extra = newExtra
    }

    // OpenAI 平台专属设置（成品号语义；openai 标签的 key 仍沿用，待协议化）
    if (props.account.platform === 'openai' && (props.account.type === 'oauth' || props.account.type === 'setup-token' || props.account.type === 'apikey')) {
      const currentExtra = (updatePayload.extra as Record<string, unknown>) || (props.account.extra as Record<string, unknown>) || {}
      const newExtra: Record<string, unknown> = { ...currentExtra }
      const hadCodexCLIOnlyEnabled = currentExtra.codex_cli_only === true
      // 缺省即保留 namespace，不写空值，避免 extra 里堆积默认项
      if (props.account.type === 'oauth' && openaiFlattenNamespacesEnabled.value) {
        newExtra.openai_responses_flatten_namespaces = true
      } else {
        delete newExtra.openai_responses_flatten_namespaces
      }
		if (autoPause5hThreshold.value != null && autoPause5hThreshold.value > 0) {
			newExtra.auto_pause_5h_threshold = autoPause5hThreshold.value / 100
		} else {
			delete newExtra.auto_pause_5h_threshold
		}
		if (autoPause7dThreshold.value != null && autoPause7dThreshold.value > 0) {
			newExtra.auto_pause_7d_threshold = autoPause7dThreshold.value / 100
		} else {
			delete newExtra.auto_pause_7d_threshold
		}
		if (autoPause5hDisabled.value) {
			newExtra.auto_pause_5h_disabled = true
		} else {
			delete newExtra.auto_pause_5h_disabled
		}
		if (autoPause7dDisabled.value) {
			newExtra.auto_pause_7d_disabled = true
		} else {
			delete newExtra.auto_pause_7d_disabled
		}
		if (props.account.type === 'oauth' && !isSparkShadow.value) {
			newExtra.auto_reset_credit_enabled = autoResetCreditEnabled.value
			newExtra.auto_reset_credit_5h_threshold = autoResetCredit5hThreshold.value / 100
			newExtra.auto_reset_credit_7d_threshold = autoResetCredit7dThreshold.value / 100
		}
		// 运行态只允许后端服务更新，账号编辑不得回写旧状态。
		delete newExtra.codex_auto_reset_credit_state

		delete newExtra.codex_image_generation_bridge_enabled
      switch (codexImageToolMode.value) {
        case 'enabled':
        case 'disabled':
          newExtra.codex_image_generation_bridge = codexImageToolMode.value === 'enabled'
          delete newExtra.codex_image_generation_explicit_tool_policy
          break
        case 'block':
          newExtra.codex_image_generation_explicit_tool_policy = 'strip'
          delete newExtra.codex_image_generation_bridge
          break
        default:
          delete newExtra.codex_image_generation_bridge
          delete newExtra.codex_image_generation_explicit_tool_policy
      }

      if (props.account.type === 'oauth' || props.account.type === 'setup-token') {
        if (codexCLIOnlyEnabled.value) {
          newExtra.codex_cli_only = true
        } else if (hadCodexCLIOnlyEnabled) {
          // 关闭时显式写 false，避免 extra 为空被后端忽略导致旧值无法清除
          newExtra.codex_cli_only = false
        } else {
          delete newExtra.codex_cli_only
        }
        // Claude Code 插件放行已迁移到全局 codex_cli_only_whitelist，编辑时清理废弃账号级快捷字段。
        delete newExtra.codex_cli_only_allowed_clients
        if (codexCLIOnlyEnabled.value && codexCLIOnlyAppServerEnabled.value) {
          newExtra.codex_cli_only_allow_app_server = true
        } else {
          delete newExtra.codex_cli_only_allow_app_server
        }
      }

      // 指纹收敛模式：默认 off（不写入）；device/session/full 是显式 opt-in，
      // 必须落键，否则管理员的选择会被后端当作"未设置"而回落到 off（#5610）。
      if (props.account.type === 'oauth') {
        if (codexFingerprintMode.value !== 'off') {
          newExtra.codex_fingerprint_mode = codexFingerprintMode.value
        } else {
          delete newExtra.codex_fingerprint_mode
        }
      }

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
