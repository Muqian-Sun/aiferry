<template>
  <!-- /accounts/new 整页（A5 起不再有弹窗形态）：分区导航读下面的 FormSectionHeading -->
  <FormPageShell :show="show" :title="t('admin.accounts.createAccount')" @close="handleClose">
    <!-- Step Indicator for OAuth accounts -->
    <div v-if="isOAuthFlow" class="mb-6 flex items-center justify-center">
      <div class="flex items-center space-x-4">
        <div class="flex items-center">
          <div
            :class="[
              'flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold',
              step >= 1 ? 'bg-af-ink text-af-on-brand' : 'bg-af-hairline text-af-ink-3'
            ]"
          >
            1
          </div>
          <span class="ml-2 text-sm font-medium text-af-ink-2">{{
            t('admin.accounts.oauth.authMethod')
          }}</span>
        </div>
        <div class="h-0.5 w-8 bg-af-ink-4" />
        <div class="flex items-center">
          <div
            :class="[
              'flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold',
              step >= 2 ? 'bg-af-ink text-af-on-brand' : 'bg-af-hairline text-af-ink-3'
            ]"
          >
            2
          </div>
          <span class="ml-2 text-sm font-medium text-af-ink-2">{{
            oauthStepTitle
          }}</span>
        </div>
      </div>
    </div>

    <!-- Step 1: Basic Info -->
    <form
      v-if="step === 1"
      id="create-account-form"
      @submit.prevent="handleSubmit"
      class="space-y-5"
    >
      <!-- 先选接入方式与来源（muqian 2026-09-25）：第三方 key 不选平台，成品号只选哪家的账号 -->
      <AccessSourcePicker v-model="accessSourceId" />

      <div>
        <label class="input-label">{{ t('admin.accounts.accountName') }}</label>
        <input
          v-model="form.name"
          type="text"
          :required="!isGrokSSOInputMethod"
          class="input"
          :placeholder="t('admin.accounts.enterAccountName')"
        />
      </div>

      <div
        v-if="form.platform === 'anthropic' && accountCategory === 'service_account'"
        class="rounded-lg border border-af-hairline bg-af-sunken px-3 py-2 text-xs text-af-ink-2"
      >
        <p>{{ t('admin.accounts.vertexAnthropicHint') }}</p>
      </div>

      <!-- Add Method (only for Anthropic OAuth-based type) -->
      <div v-if="form.platform === 'anthropic' && isOAuthFlow">
        <label class="input-label">{{ t('admin.accounts.addMethod') }}</label>
        <div class="mt-2 flex gap-4">
          <label class="flex cursor-pointer items-center">
            <input
              v-model="addMethod"
              type="radio"
              value="oauth"
              class="mr-2 text-af-brand focus:ring-af-brand"
            />
            <span class="text-sm text-af-ink-2">{{ t('admin.accounts.types.oauth') }}</span>
          </label>
          <label class="flex cursor-pointer items-center">
            <input
              v-model="addMethod"
              type="radio"
              value="setup-token"
              class="mr-2 text-af-brand focus:ring-af-brand"
            />
            <span class="text-sm text-af-ink-2">{{
              t('admin.accounts.setupTokenLongLived')
            }}</span>
          </label>
        </div>
      </div>

      <!-- Gemini 成品号的附加选项（key 的档位在地址下面，按地址识别出 Gemini 才显示） -->
      <div v-if="form.platform === 'gemini'">
        <div
          v-if="accountCategory === 'service_account'"
          class="mt-3 rounded-lg border border-af-hairline bg-af-sunken px-3 py-2 text-xs text-af-ink-2"
        >
          <p>{{ t('admin.accounts.vertexGeminiHint') }}</p>
        </div>

        <!-- OAuth Type Selection (only show when oauth-based is selected) -->
        <div v-if="accountCategory === 'oauth-based'" class="mt-4">
          <div class="flex items-center justify-between gap-4">
            <label class="input-label">{{ t('admin.accounts.oauth.gemini.oauthTypeLabel') }}</label>
            <button
              type="button"
              class="flex items-center gap-1 rounded px-2 py-1 text-xs text-af-ink-2 hover:bg-af-sunken"
              @click="showGeminiHelpDialog = true"
            >
              <Icon name="questionCircle" size="sm" />
              {{ t('admin.accounts.gemini.helpButton') }}
            </button>
          </div>
          <div class="mt-2 grid grid-cols-2 gap-3">
            <!-- Google One OAuth -->
            <button
              type="button"
              @click="handleSelectGeminiOAuthType('google_one')"
              :class="[
                'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
                geminiOAuthType === 'google_one'
                  ? 'border-af-brand bg-af-brand-tint'
                  : 'border-af-hairline hover:border-af-hairline-strong'
              ]"
            >
              <div
                :class="[
                  'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
                  geminiOAuthType === 'google_one'
                    ? 'bg-af-ink text-af-on-brand'
                    : 'bg-af-sunken text-af-ink-3'
                ]"
              >
                <Icon name="user" size="sm" />
              </div>
              <div class="min-w-0">
                <span class="block text-sm font-medium text-af-ink">
                  Google One
                </span>
                <span class="text-xs text-af-ink-3">
                  {{ t('admin.accounts.gemini.oauthType.googleOneDesc') }}
                </span>
                <div class="mt-2 flex flex-wrap gap-1">
                  <span
                    class="rounded bg-af-sunken px-2 py-0.5 text-[10px] font-semibold text-af-ink-2"
                  >
                    {{ t('admin.accounts.gemini.oauthType.badges.individuals') }}
                  </span>
                  <span
                    class="rounded bg-af-sunken px-2 py-0.5 text-[10px] font-semibold text-af-ink-2"
                  >
                    {{ t('admin.accounts.gemini.oauthType.badges.noGcp') }}
                  </span>
                </div>
              </div>
            </button>

            <!-- GCP Code Assist OAuth -->
            <button
              type="button"
              @click="handleSelectGeminiOAuthType('code_assist')"
              :class="[
                'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
                geminiOAuthType === 'code_assist'
                  ? 'border-af-brand bg-af-brand-tint'
                  : 'border-af-hairline hover:border-af-hairline-strong'
              ]"
            >
              <div
                :class="[
                  'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
                  geminiOAuthType === 'code_assist'
                    ? 'bg-af-ink text-af-on-brand'
                    : 'bg-af-sunken text-af-ink-3'
                ]"
              >
                <Icon name="cloud" size="sm" />
              </div>
              <div class="min-w-0">
                <span class="block text-sm font-medium text-af-ink">
                  GCP Code Assist
                </span>
                <span class="text-xs text-af-ink-3">
                  {{ t('admin.accounts.gemini.oauthType.codeAssistDesc') }}
                </span>
                <div class="mt-1 text-xs text-af-ink-3">
                  {{ t('admin.accounts.gemini.oauthType.codeAssistRequirement') }}
                  <a
                    :href="geminiHelpLinks.gcpProject"
                    class="ml-1 text-af-ink-2 hover:underline"
                    target="_blank"
                    rel="noreferrer"
                  >
                    {{ t('admin.accounts.gemini.oauthType.gcpProjectLink') }}
                  </a>
                </div>
                <div class="mt-2 flex flex-wrap gap-1">
                  <span
                    class="rounded bg-af-sunken px-2 py-0.5 text-[10px] font-semibold text-af-ink-2"
                  >
                    {{ t('admin.accounts.gemini.oauthType.badges.enterprise') }}
                  </span>
                  <span
                    class="rounded bg-af-sunken px-2 py-0.5 text-[10px] font-semibold text-af-ink-2"
                  >
                    {{ t('admin.accounts.gemini.oauthType.badges.highConcurrency') }}
                  </span>
                </div>
              </div>
            </button>
          </div>

          <!-- Advanced Options Toggle -->
          <div class="mt-3">
            <button
              type="button"
              @click="showAdvancedOAuth = !showAdvancedOAuth"
              class="flex items-center gap-2 text-sm text-af-ink-2 hover:text-af-ink"
            >
              <svg
                :class="['h-4 w-4 transition-transform', showAdvancedOAuth ? 'rotate-90' : '']"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
              </svg>
              <span>
                {{
                  showAdvancedOAuth
                    ? t('admin.accounts.gemini.oauthType.hideAdvanced')
                    : t('admin.accounts.gemini.oauthType.showAdvanced')
                }}
              </span>
            </button>
          </div>

          <!-- Custom OAuth Client (Advanced) -->
          <div v-if="showAdvancedOAuth" class="mt-3 group relative">
            <button
              type="button"
              :disabled="!geminiAIStudioOAuthEnabled"
              @click="handleSelectGeminiOAuthType('ai_studio')"
              :class="[
                'flex w-full items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
                !geminiAIStudioOAuthEnabled ? 'cursor-not-allowed opacity-60' : '',
                geminiOAuthType === 'ai_studio'
                  ? 'border-af-brand bg-af-brand-tint'
                  : 'border-af-hairline hover:border-af-hairline-strong'
              ]"
            >
              <div
                :class="[
                  'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
                  geminiOAuthType === 'ai_studio'
                    ? 'bg-af-ink text-af-on-brand'
                    : 'bg-af-sunken text-af-ink-3'
                ]"
              >
                <svg
                  class="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  stroke-width="1.5"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z"
                  />
                </svg>
              </div>
              <div class="min-w-0">
                <span class="block text-sm font-medium text-af-ink">
                  {{ t('admin.accounts.gemini.oauthType.customTitle') }}
                </span>
                <span class="text-xs text-af-ink-3">
                  {{ t('admin.accounts.gemini.oauthType.customDesc') }}
                </span>
                <div class="mt-1 text-xs text-af-ink-3">
                  {{ t('admin.accounts.gemini.oauthType.customRequirement') }}
                </div>
                <div class="mt-2 flex flex-wrap gap-1">
                  <span
                    class="rounded bg-af-warning-tint px-2 py-0.5 text-[10px] font-semibold text-af-warning"
                  >
                    {{ t('admin.accounts.gemini.oauthType.badges.orgManaged') }}
                  </span>
                  <span
                    class="rounded bg-af-warning-tint px-2 py-0.5 text-[10px] font-semibold text-af-warning"
                  >
                    {{ t('admin.accounts.gemini.oauthType.badges.adminRequired') }}
                  </span>
                </div>
              </div>
              <span
                v-if="!geminiAIStudioOAuthEnabled"
                class="ml-auto shrink-0 rounded bg-af-warning-tint px-2 py-0.5 text-xs text-af-warning"
              >
                {{ t('admin.accounts.oauth.gemini.aiStudioNotConfiguredShort') }}
              </span>
            </button>

            <div
              v-if="!geminiAIStudioOAuthEnabled"
              class="pointer-events-none absolute right-0 top-full z-50 mt-2 w-80 rounded-md border border-af-warning/30 bg-af-warning-tint px-3 py-2 text-xs text-af-warning opacity-0 shadow-lg transition-opacity group-hover:opacity-100"
            >
              {{ t('admin.accounts.oauth.gemini.aiStudioNotConfiguredTip') }}
            </div>
          </div>
        </div>

      </div>

      <!-- Vertex Service Account -->
      <div v-if="(form.platform === 'gemini' || form.platform === 'anthropic') && accountCategory === 'service_account'" class="space-y-4">
        <div>
          <label class="input-label">Service Account JSON</label>
          <input
            ref="vertexServiceAccountFileInput"
            type="file"
            accept="application/json,.json"
            class="hidden"
            @change="handleVertexServiceAccountFile"
          />
          <div
            :class="[
              'rounded-lg border-2 border-dashed px-4 py-5 transition-colors',
              vertexServiceAccountDragActive
                ? 'border-af-brand bg-af-brand-tint'
                : 'border-af-hairline-strong bg-af-sunken hover:border-af-ink-3 hover:bg-af-sunken/60'
            ]"
            @dragenter.prevent="vertexServiceAccountDragActive = true"
            @dragover.prevent="vertexServiceAccountDragActive = true"
            @dragleave.prevent="vertexServiceAccountDragActive = false"
            @drop.prevent="handleVertexServiceAccountDrop"
          >
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div class="min-w-0">
                <div class="flex items-center gap-2 text-sm font-medium text-af-ink">
                  <Icon name="upload" size="sm" />
                  <span>{{ vertexServiceAccountJson ? t('admin.accounts.vertexSaJsonLoaded') : t('admin.accounts.vertexSaJsonDrop') }}</span>
                </div>
                <p class="mt-1 text-xs text-af-ink-3">
                  {{ vertexServiceAccountJson ? t('admin.accounts.vertexSaJsonKeyHidden') : t('admin.accounts.vertexSaJsonDropHint') }}
                </p>
              </div>
              <button
                type="button"
                class="btn btn-secondary shrink-0"
                @click="vertexServiceAccountFileInput?.click()"
              >
                <Icon name="upload" size="sm" />
                {{ t('admin.accounts.vertexSaJsonSelectBtn') }}
              </button>
            </div>
          </div>
          <p class="input-hint">{{ t('admin.accounts.vertexSaJsonUploadHint') }}</p>
        </div>

        <div>
          <label class="input-label">Location</label>
          <select
            v-model="vertexLocation"
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

      <!-- Bedrock 凭证（仅 Anthropic Bedrock 类型） -->
      <div v-if="form.platform === 'anthropic' && accountCategory === 'bedrock'" class="space-y-4">
        <!-- Auth Mode Radio -->
        <div>
          <label class="input-label">{{ t('admin.accounts.bedrockAuthMode') }}</label>
          <div class="mt-2 flex gap-4">
            <label class="flex cursor-pointer items-center">
              <input
                v-model="bedrockAuthMode"
                type="radio"
                value="sigv4"
                class="mr-2 text-af-brand focus:ring-af-brand"
              />
              <span class="text-sm text-af-ink-2">{{ t('admin.accounts.bedrockAuthModeSigv4') }}</span>
            </label>
            <label class="flex cursor-pointer items-center">
              <input
                v-model="bedrockAuthMode"
                type="radio"
                value="apikey"
                class="mr-2 text-af-brand focus:ring-af-brand"
              />
              <span class="text-sm text-af-ink-2">{{ t('admin.accounts.bedrockAuthModeApikey') }}</span>
            </label>
          </div>
        </div>

        <!-- SigV4 fields -->
        <template v-if="bedrockAuthMode === 'sigv4'">
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockAccessKeyId') }}</label>
            <input
              v-model="bedrockAccessKeyId"
              type="text"
              required
              class="input font-mono"
              placeholder="AKIA..."
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.bedrockSecretAccessKey') }}</label>
            <input
              v-model="bedrockSecretAccessKey"
              type="password"
              required
              class="input font-mono"
            />
          </div>
        </template>

        <!-- API Key field -->
        <div v-if="bedrockAuthMode === 'apikey'">
          <label class="input-label">{{ t('admin.accounts.bedrockApiKeyInput') }}</label>
          <input
            v-model="bedrockApiKeyValue"
            type="password"
            required
            class="input font-mono"
          />
        </div>
      </div>

      <!-- Bedrock 区域与全局推理 -->
      <div v-if="form.platform === 'anthropic' && accountCategory === 'bedrock'" class="space-y-4">
        <!-- Shared: Region -->
        <div>
          <label class="input-label">{{ t('admin.accounts.bedrockRegion') }}</label>
          <select v-model="bedrockRegion" class="input">
            <optgroup label="US">
              <option value="us-east-1">us-east-1 (N. Virginia)</option>
              <option value="us-east-2">us-east-2 (Ohio)</option>
              <option value="us-west-1">us-west-1 (N. California)</option>
              <option value="us-west-2">us-west-2 (Oregon)</option>
              <option value="us-gov-east-1">us-gov-east-1 (GovCloud US-East)</option>
              <option value="us-gov-west-1">us-gov-west-1 (GovCloud US-West)</option>
            </optgroup>
            <optgroup label="Europe">
              <option value="eu-west-1">eu-west-1 (Ireland)</option>
              <option value="eu-west-2">eu-west-2 (London)</option>
              <option value="eu-west-3">eu-west-3 (Paris)</option>
              <option value="eu-central-1">eu-central-1 (Frankfurt)</option>
              <option value="eu-central-2">eu-central-2 (Zurich)</option>
              <option value="eu-south-1">eu-south-1 (Milan)</option>
              <option value="eu-south-2">eu-south-2 (Spain)</option>
              <option value="eu-north-1">eu-north-1 (Stockholm)</option>
            </optgroup>
            <optgroup label="Asia Pacific">
              <option value="ap-northeast-1">ap-northeast-1 (Tokyo)</option>
              <option value="ap-northeast-2">ap-northeast-2 (Seoul)</option>
              <option value="ap-northeast-3">ap-northeast-3 (Osaka)</option>
              <option value="ap-south-1">ap-south-1 (Mumbai)</option>
              <option value="ap-south-2">ap-south-2 (Hyderabad)</option>
              <option value="ap-southeast-1">ap-southeast-1 (Singapore)</option>
              <option value="ap-southeast-2">ap-southeast-2 (Sydney)</option>
            </optgroup>
            <optgroup label="Canada">
              <option value="ca-central-1">ca-central-1 (Canada)</option>
            </optgroup>
            <optgroup label="South America">
              <option value="sa-east-1">sa-east-1 (São Paulo)</option>
            </optgroup>
          </select>
          <p class="input-hint">{{ t('admin.accounts.bedrockRegionHint') }}</p>
        </div>

        <!-- Shared: Force Global -->
        <div>
          <label class="flex items-center gap-2 cursor-pointer">
            <input
              v-model="bedrockForceGlobal"
              type="checkbox"
              class="rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
            />
            <span class="text-sm text-af-ink-2">{{ t('admin.accounts.bedrockForceGlobal') }}</span>
          </label>
          <p class="input-hint mt-1">{{ t('admin.accounts.bedrockForceGlobalHint') }}</p>
        </div>
      </div>

      <!--
        第三方 key 的协议地址（muqian 2026-09-25：key 不选平台）：可从常用官方地址里选一条填入，也可直接填中转地址；
        厂商按地址识别，识别出的厂商才有它的专属选项（Coding 套餐、智谱团队版、Gemini 档位）。
      -->
      <div v-if="form.type === 'apikey'" class="space-y-4">
        <div>
          <KeyAddressPresetMenu
            v-if="keyPresets.length > 0"
            class="mb-3"
            :presets="keyPresets"
            @select="applyKeyAddressPreset"
          />
          <ProtocolEndpointsEditor
            v-model="protocolEndpoints"
            :protocols="UPSTREAM_PROTOCOLS"
            :official-endpoints="officialProtocolEndpoints"
            :defaults-load-failed="protocolDefaultsLoadFailed"
          />
          <p v-if="keyVendor" class="input-hint" data-testid="key-vendor-detected">
            {{ t('admin.accounts.keyAddress.detected', { vendor: keyVendorLabel }) }}
          </p>
          <p v-else-if="hasKeyAddress" class="input-hint" data-testid="key-vendor-relay">
            {{ t('admin.accounts.keyAddress.relay') }}
          </p>
        </div>

        <!-- 按量 / Coding 套餐：地址分得出就不问（识别提示里带上）；MiniMax 两种套餐同一个地址，要管理员选 -->
        <div v-if="keyPlanNeedsChoice" data-testid="key-plan-mode">
          <label class="input-label">{{ t('admin.accounts.cnProviders.accountMode.title') }}</label>
          <div class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <button
              v-for="mode in CN_PLAN_MODES"
              :key="mode.value"
              type="button"
              :data-testid="`key-plan-mode-${mode.value}`"
              :class="[
                'flex items-center gap-3 rounded-lg border p-3 text-left transition-colors',
                keyPlanMode === mode.value ? 'border-af-brand bg-af-brand-tint' : 'border-af-hairline hover:border-af-hairline-strong'
              ]"
              @click="keyPlanMode = mode.value"
            >
              <span
                :class="[
                  'flex h-8 w-8 shrink-0 items-center justify-center rounded-md',
                  keyPlanMode === mode.value ? 'bg-af-ink text-af-on-brand' : 'bg-af-sunken text-af-ink-3'
                ]"
              >
                <Icon :name="mode.icon" size="sm" />
              </span>
              <span>
                <span class="block text-sm font-medium text-af-ink">{{ t(`admin.accounts.cnProviders.accountMode.${mode.value}`) }}</span>
                <span class="text-xs text-af-ink-3">{{ t(`admin.accounts.cnProviders.accountMode.${mode.value}Desc`) }}</span>
              </span>
            </button>
          </div>
        </div>
      </div>

      <!-- 第三方 key 的 API Key -->
      <div v-if="form.type === 'apikey'">
        <label class="input-label">{{ t('admin.accounts.apiKeyRequired') }}</label>
        <input
          v-model="apiKeyValue"
          type="password"
          required
          class="input font-mono"
          :placeholder="apiKeyValuePlaceholder"
        />
        <p v-if="apiKeyHint" class="input-hint">{{ apiKeyHint }}</p>
      </div>

      <!-- 承接的模型：默认勾上识别出的厂商已上架的对话模型（muqian 2026-09-25），收成一行，点「修改」展开 -->
      <CatalogEntryPicker
        v-model="selectedCatalogEntryIds"
        collapsible
        :suggested-platform="catalogSuggestedPlatform"
        @update:model-value="catalogSelectionTouched = true"
        @loaded="catalogEntries = $event"
      />

      <!-- 更多设置：不点开就按默认值建（muqian 2026-09-25「还是太繁琐」：默认只露必填项） -->
      <div class="border-t border-af-hairline pt-4">
        <button
          type="button"
          class="flex w-full items-center gap-2 text-left text-sm font-medium text-af-ink-2 transition-colors hover:text-af-ink"
          :aria-expanded="showMoreSettings ? 'true' : 'false'"
          data-testid="create-more-settings-toggle"
          @click="showMoreSettings = !showMoreSettings"
        >
          <Icon name="chevronRight" size="sm" :class="['transition-transform', showMoreSettings ? 'rotate-90' : '']" />
          {{ t('admin.accounts.moreSettings.title') }}
          <span class="font-normal text-af-ink-3">{{ t('admin.accounts.moreSettings.hint') }}</span>
        </button>
      </div>

      <template v-if="showMoreSettings">
      <FormSectionHeading section="basics" :title="t('admin.accounts.formPage.sections.basics')" />

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

      <div>
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{
              t('admin.accounts.autoPauseOnExpired')
            }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.autoPauseOnExpiredDesc') }}
            </p>
          </div>
          <button
            type="button"
            @click="autoPauseOnExpired = !autoPauseOnExpired"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              autoPauseOnExpired ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                autoPauseOnExpired ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>
      </div>

      <div v-if="form.platform === 'antigravity'">
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

      <!-- 超量：Antigravity 成品号（OAuth）专属；第三方 key 按协议调度，没有这一项。条件放在外层，别的平台不留一条空分隔线 -->
      <div v-if="form.platform === 'antigravity'" class="border-t border-af-hairline pt-4">
        <div class="flex items-center gap-2">
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

      <!-- Gemini 成品号档位（自动识别失败时的兜底） -->
      <div v-if="form.platform === 'gemini'">
        <div v-if="accountCategory === 'oauth-based'" class="mt-4">
          <label class="input-label">{{ t('admin.accounts.gemini.tier.label') }}</label>
          <div class="mt-2">
            <select
              v-if="geminiOAuthType === 'google_one'"
              v-model="geminiTierGoogleOne"
              class="input"
            >
              <option value="google_one_free">{{ t('admin.accounts.gemini.tier.googleOne.free') }}</option>
              <option value="google_ai_pro">{{ t('admin.accounts.gemini.tier.googleOne.pro') }}</option>
              <option value="google_ai_ultra">{{ t('admin.accounts.gemini.tier.googleOne.ultra') }}</option>
            </select>

            <select
              v-else-if="geminiOAuthType === 'code_assist'"
              v-model="geminiTierGcp"
              class="input"
            >
              <option value="gcp_standard">{{ t('admin.accounts.gemini.tier.gcp.standard') }}</option>
              <option value="gcp_enterprise">{{ t('admin.accounts.gemini.tier.gcp.enterprise') }}</option>
            </select>

            <select
              v-else
              v-model="geminiTierAIStudio"
              class="input"
            >
              <option value="aistudio_free">{{ t('admin.accounts.gemini.tier.aiStudio.free') }}</option>
              <option value="aistudio_paid">{{ t('admin.accounts.gemini.tier.aiStudio.paid') }}</option>
            </select>
          </div>
          <p class="input-hint">{{ t('admin.accounts.gemini.tier.hint') }}</p>
        </div>
      </div>

      <FormSectionHeading v-if="showEndpointSection" section="endpoint" :title="t('admin.accounts.formPage.sections.endpoint')" />

      <!-- 第三方 key 的其余设置：智谱团队版、Gemini 档位（按识别出的厂商显示）、上游倍率探测 -->
      <div v-if="form.type === 'apikey'" class="space-y-4">
        <!-- 智谱团队版 Coding Plan：组织/项目 ID（可选，填写后额度探测走团队版端点） -->
        <div v-if="keyVendor === 'zhipu' && keyPlanMode === 'coding'">
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
              <input v-model="zhipuOrganization" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.organizationPlaceholder')" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.project') }}</label>
              <input v-model="zhipuProject" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.projectPlaceholder')" />
            </div>
          </div>
          <p class="input-hint mt-2">{{ t('admin.accounts.cnProviders.zhipuTeam.hint') }}</p>
        </div>

        <!-- 上游倍率自动探测：全部 API-key 平台可用（所在区块已限定 apikey 类型） -->
        <div
          class="flex items-center justify-between gap-4 border-t border-af-hairline pt-4"
        >
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.upstreamBilling.autoProbe') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.upstreamBilling.autoProbeHint') }}
            </p>
          </div>
          <Toggle
            v-model="upstreamBillingAutoProbeEnabled"
            data-testid="upstream-billing-auto-probe"
            :aria-label="t('admin.accounts.upstreamBilling.autoProbe')"
          />
        </div>
      </div>

      <!-- OpenAI 自动透传开关：OpenAI 成品号；第三方 key 配了 responses / chat_completions 地址才展示，不看平台标签 -->
      <div
        v-if="openAIResponsesSettingsVisible"
        data-testid="create-openai-passthrough"
        class="border-t border-af-hairline pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.openai.oauthPassthrough') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.openai.oauthPassthroughDesc') }}
            </p>
            <p
              v-if="form.type === 'apikey'"
              data-testid="create-openai-key-protocol-hint"
              class="mt-1 text-xs text-af-warning"
            >
              {{ t('admin.accounts.openai.keyProtocolSettingsHint') }}
            </p>
          </div>
          <button
            type="button"
            data-testid="create-openai-passthrough-toggle"
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
        v-if="form.platform === 'openai' && form.type === 'oauth'"
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
            data-testid="create-openai-flatten-namespaces-toggle"
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

      <!-- OpenAI WS Mode（off/ctx_pool/passthrough/http_bridge），展示条件同自动透传 -->
      <div
        v-if="openAIResponsesSettingsVisible"
        data-testid="create-openai-ws-mode"
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
            <Select v-model="openaiResponsesWebSocketV2Mode" :options="openAIWSModeOptions" />
          </div>
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
            data-testid="create-anthropic-auth-scheme"
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
        data-testid="create-bedrock-cc-compat"
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
            data-testid="create-bedrock-cc-compat-toggle"
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

      <FormSectionHeading v-if="showModelMoreSection" section="models" :title="t('admin.accounts.formPage.sections.models')" />

      <!-- 模型改名（可选）：只改名，不限定能接哪些模型（那由上面的勾选决定），提交时带 model_mapping_rename_only -->
      <ModelRenameEditor
        v-if="showModelRename"
        v-model="modelMappings"
        data-testid="create-model-rename"
        class="border-t border-af-hairline pt-4"
        :presets="renamePresets"
        :disabled="isOpenAIModelRestrictionDisabled"
        :extends-vendor-table="extendsVendorTable"
      />

      <!-- OpenAI Compact 能力配置，展示条件同自动透传 -->
      <div
        v-if="openAIResponsesSettingsVisible"
        data-testid="create-openai-compact"
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
            <Select v-model="openAICompactMode" :options="openAICompactModeOptions" />
          </div>
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
              <input v-model="mapping.from" type="text" class="input flex-1" :placeholder="t('admin.accounts.fromModel')" />
              <span class="text-af-ink-3">→</span>
              <input v-model="mapping.to" type="text" class="input flex-1" :placeholder="t('admin.accounts.toModel')" />
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

      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.concurrency') }}</label>
          <input v-model.number="form.concurrency" type="number" min="1" class="input"
            @input="form.concurrency = Math.max(1, form.concurrency || 1)" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.loadFactor') }}</label>
          <input v-model.number="form.load_factor" type="number" min="1"
            class="input" :placeholder="String(form.concurrency || 1)"
            @input="form.load_factor = (form.load_factor &amp;&amp; form.load_factor >= 1) ? form.load_factor : null" />
          <p class="input-hint">{{ t('admin.accounts.loadFactorHint') }}</p>
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
          <input v-model.number="form.rate_multiplier" type="number" min="0" step="0.001" class="input" />
          <p class="input-hint">{{ t('admin.accounts.billingRateMultiplierHint') }}</p>
        </div>
      </div>

      <!-- 配额控制 (Anthropic apikey/bedrock: 配额限制 + 亲和) -->
      <div
        v-if="form.platform === 'anthropic' && (form.type === 'apikey' || form.type === 'bedrock')"
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
          :quotaNotifyGlobalEnabled="quotaNotifyGlobalEnabled"
          :quotaNotifyDailyEnabled="quotaNotifyState.daily.enabled"
          :quotaNotifyDailyThreshold="quotaNotifyState.daily.threshold"
          :quotaNotifyDailyThresholdType="quotaNotifyState.daily.thresholdType"
          :quotaNotifyWeeklyEnabled="quotaNotifyState.weekly.enabled"
          :quotaNotifyWeeklyThreshold="quotaNotifyState.weekly.threshold"
          :quotaNotifyWeeklyThresholdType="quotaNotifyState.weekly.thresholdType"
          :quotaNotifyTotalEnabled="quotaNotifyState.total.enabled"
          :quotaNotifyTotalThreshold="quotaNotifyState.total.threshold"
          :quotaNotifyTotalThresholdType="quotaNotifyState.total.thresholdType"
          :dailyResetMode="editDailyResetMode"
          :dailyResetHour="editDailyResetHour"
          :weeklyResetMode="editWeeklyResetMode"
          :weeklyResetDay="editWeeklyResetDay"
          :weeklyResetHour="editWeeklyResetHour"
          :resetTimezone="editResetTimezone"
          @update:totalLimit="editQuotaLimit = $event"
          @update:dailyLimit="editQuotaDailyLimit = $event"
          @update:weeklyLimit="editQuotaWeeklyLimit = $event"
          @update:quotaNotifyDailyEnabled="quotaNotifyState.daily.enabled = $event"
          @update:quotaNotifyDailyThreshold="quotaNotifyState.daily.threshold = $event"
          @update:quotaNotifyDailyThresholdType="quotaNotifyState.daily.thresholdType = $event"
          @update:quotaNotifyWeeklyEnabled="quotaNotifyState.weekly.enabled = $event"
          @update:quotaNotifyWeeklyThreshold="quotaNotifyState.weekly.threshold = $event"
          @update:quotaNotifyWeeklyThresholdType="quotaNotifyState.weekly.thresholdType = $event"
          @update:quotaNotifyTotalEnabled="quotaNotifyState.total.enabled = $event"
          @update:quotaNotifyTotalThreshold="quotaNotifyState.total.threshold = $event"
          @update:quotaNotifyTotalThresholdType="quotaNotifyState.total.thresholdType = $event"
          @update:dailyResetMode="editDailyResetMode = $event"
          @update:dailyResetHour="editDailyResetHour = $event"
          @update:weeklyResetMode="editWeeklyResetMode = $event"
          @update:weeklyResetDay="editWeeklyResetDay = $event"
          @update:weeklyResetHour="editWeeklyResetHour = $event"
          @update:resetTimezone="editResetTimezone = $event"
        />
      </div>

      <!-- 配额控制 (非 Anthropic apikey/bedrock) -->
      <div
        v-else-if="form.type === 'apikey' || form.type === 'bedrock'"
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
          :quotaNotifyGlobalEnabled="quotaNotifyGlobalEnabled"
          :quotaNotifyDailyEnabled="quotaNotifyState.daily.enabled"
          :quotaNotifyDailyThreshold="quotaNotifyState.daily.threshold"
          :quotaNotifyDailyThresholdType="quotaNotifyState.daily.thresholdType"
          :quotaNotifyWeeklyEnabled="quotaNotifyState.weekly.enabled"
          :quotaNotifyWeeklyThreshold="quotaNotifyState.weekly.threshold"
          :quotaNotifyWeeklyThresholdType="quotaNotifyState.weekly.thresholdType"
          :quotaNotifyTotalEnabled="quotaNotifyState.total.enabled"
          :quotaNotifyTotalThreshold="quotaNotifyState.total.threshold"
          :quotaNotifyTotalThresholdType="quotaNotifyState.total.thresholdType"
          :dailyResetMode="editDailyResetMode"
          :dailyResetHour="editDailyResetHour"
          :weeklyResetMode="editWeeklyResetMode"
          :weeklyResetDay="editWeeklyResetDay"
          :weeklyResetHour="editWeeklyResetHour"
          :resetTimezone="editResetTimezone"
          @update:totalLimit="editQuotaLimit = $event"
          @update:dailyLimit="editQuotaDailyLimit = $event"
          @update:weeklyLimit="editQuotaWeeklyLimit = $event"
          @update:quotaNotifyDailyEnabled="quotaNotifyState.daily.enabled = $event"
          @update:quotaNotifyDailyThreshold="quotaNotifyState.daily.threshold = $event"
          @update:quotaNotifyDailyThresholdType="quotaNotifyState.daily.thresholdType = $event"
          @update:quotaNotifyWeeklyEnabled="quotaNotifyState.weekly.enabled = $event"
          @update:quotaNotifyWeeklyThreshold="quotaNotifyState.weekly.threshold = $event"
          @update:quotaNotifyWeeklyThresholdType="quotaNotifyState.weekly.thresholdType = $event"
          @update:quotaNotifyTotalEnabled="quotaNotifyState.total.enabled = $event"
          @update:quotaNotifyTotalThreshold="quotaNotifyState.total.threshold = $event"
          @update:quotaNotifyTotalThresholdType="quotaNotifyState.total.thresholdType = $event"
          @update:dailyResetMode="editDailyResetMode = $event"
          @update:dailyResetHour="editDailyResetHour = $event"
          @update:weeklyResetMode="editWeeklyResetMode = $event"
          @update:weeklyResetDay="editWeeklyResetDay = $event"
          @update:weeklyResetHour="editWeeklyResetHour = $event"
          @update:resetTimezone="editResetTimezone = $event"
        />
      </div>

      <!-- 配额控制 (Anthropic OAuth/SetupToken: 会话 + RPM) -->
      <div
        v-if="form.platform === 'anthropic' && accountCategory === 'oauth-based'"
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

      <FormSectionHeading section="advanced" :title="t('admin.accounts.formPage.sections.advanced')" />

      <div>
        <label class="input-label">{{ t('admin.accounts.proxy') }}</label>
        <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
      </div>

      <!-- API Key 类型的池模式与自定义错误码 -->
      <div v-if="form.type === 'apikey' && form.platform !== 'antigravity'" class="space-y-4">
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

        <!-- Custom Error Codes Section -->
        <div class="border-t border-af-hairline pt-4">
          <div class="mb-3 flex items-center justify-between">
            <div>
              <label class="input-label mb-0">{{ t('admin.accounts.customErrorCodes') }}</label>
              <p class="mt-1 text-xs text-af-ink-3">
                {{ t('admin.accounts.customErrorCodesHint') }}
              </p>
            </div>
            <button
              type="button"
              @click="customErrorCodesEnabled = !customErrorCodesEnabled"
              :class="[
                'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                customErrorCodesEnabled ? 'bg-af-brand' : 'bg-af-hairline'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                  customErrorCodesEnabled ? 'translate-x-5' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="customErrorCodesEnabled" class="space-y-3">
            <div class="rounded-lg bg-af-warning-tint p-3">
              <p class="text-xs text-af-warning">
                <Icon name="exclamationTriangle" size="sm" class="mr-1 inline" :stroke-width="2" />
                {{ t('admin.accounts.customErrorCodesWarning') }}
              </p>
            </div>

            <!-- Error Code Buttons -->
            <div class="flex flex-wrap gap-2">
              <button
                v-for="code in commonErrorCodes"
                :key="code.value"
                type="button"
                @click="toggleErrorCode(code.value)"
                :class="[
                  'rounded-lg px-3 py-1.5 text-sm font-medium transition-colors',
                  selectedErrorCodes.includes(code.value)
                    ? 'bg-af-danger-tint text-af-danger ring-1 ring-af-danger'
                    : 'bg-af-sunken text-af-ink-2 hover:bg-af-hairline'
                ]"
              >
                {{ code.value }} {{ code.label }}
              </button>
            </div>

            <!-- Manual input -->
            <div class="flex items-center gap-2">
              <input
                v-model.number="customErrorCodeInput"
                type="number"
                min="100"
                max="599"
                class="input flex-1"
                :placeholder="t('admin.accounts.enterErrorCode')"
                @keyup.enter="addCustomErrorCode"
              />
              <button type="button" @click="addCustomErrorCode" class="btn btn-secondary px-3">
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
              </button>
            </div>

            <!-- Selected codes summary -->
            <div class="flex flex-wrap gap-1.5">
              <span
                v-for="code in selectedErrorCodes.sort((a, b) => a - b)"
                :key="code"
                class="inline-flex items-center gap-1 rounded-full bg-af-danger-tint px-2.5 py-0.5 text-sm font-medium text-af-danger"
              >
                {{ code }}
                <button
                  type="button"
                  @click="removeErrorCode(code)"
                  class="hover:text-af-danger"
                >
                  <Icon name="x" size="sm" :stroke-width="2" />
                </button>
              </span>
              <span v-if="selectedErrorCodes.length === 0" class="text-xs text-af-ink-3">
                {{ t('admin.accounts.noneSelectedUsesDefault') }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- 请求头覆写：任何第三方 key（含 Antigravity 上游 key）与 Grok OAuth -->
      <div
        v-if="isHeaderOverrideCapable(form.platform, form.type)"
        data-testid="create-header-override"
        class="border-t border-af-hairline pt-4"
      >
        <div class="mb-3 flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.headerOverride.title') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.headerOverride.hint') }}
            </p>
          </div>
          <button
            type="button"
            data-testid="create-header-override-toggle"
            @click="headerOverrideEnabled = !headerOverrideEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              headerOverrideEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                headerOverrideEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>

        <div v-if="headerOverrideEnabled" class="space-y-3">
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

      <!-- Temp Unschedulable Rules -->
      <div class="border-t border-af-hairline pt-4 space-y-4">
        <div class="mb-3 flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.tempUnschedulable.title') }}</label>
            <p class="mt-1 text-xs text-af-ink-3">
              {{ t('admin.accounts.tempUnschedulable.hint') }}
            </p>
          </div>
          <button
            type="button"
            @click="tempUnschedEnabled = !tempUnschedEnabled"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
              tempUnschedEnabled ? 'bg-af-brand' : 'bg-af-hairline'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                tempUnschedEnabled ? 'translate-x-5' : 'translate-x-0'
              ]"
            />
          </button>
        </div>

        <div v-if="tempUnschedEnabled" class="space-y-3">
          <div class="rounded-lg bg-af-sunken p-3">
              <p class="text-xs text-af-ink-2">
                <Icon name="exclamationTriangle" size="sm" class="mr-1 inline" :stroke-width="2" />
                {{ t('admin.accounts.tempUnschedulable.notice') }}
              </p>
            </div>

          <div class="flex flex-wrap gap-2">
            <button
              v-for="preset in tempUnschedPresets"
              :key="preset.label"
              type="button"
              @click="addTempUnschedRule(preset.rule)"
              class="rounded-lg bg-af-sunken px-3 py-1.5 text-xs font-medium text-af-ink-2 transition-colors hover:bg-af-hairline"
            >
              + {{ preset.label }}
            </button>
          </div>

          <div v-if="tempUnschedRules.length > 0" class="space-y-3">
            <div
              v-for="(rule, index) in tempUnschedRules"
              :key="getTempUnschedRuleKey(rule)"
              class="rounded-lg border border-af-hairline p-3"
            >
              <div class="mb-2 flex items-center justify-between">
                <span class="text-xs font-medium text-af-ink-3">
                  {{ t('admin.accounts.tempUnschedulable.ruleIndex', { index: index + 1 }) }}
                </span>
                <div class="flex items-center gap-2">
                  <button
                    type="button"
                    :disabled="index === 0"
                    @click="moveTempUnschedRule(index, -1)"
                    class="rounded p-1 text-af-ink-3 transition-colors hover:text-af-ink-2 disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    <Icon name="chevronUp" size="sm" :stroke-width="2" />
                  </button>
                  <button
                    type="button"
                    :disabled="index === tempUnschedRules.length - 1"
                    @click="moveTempUnschedRule(index, 1)"
                    class="rounded p-1 text-af-ink-3 transition-colors hover:text-af-ink-2 disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    @click="removeTempUnschedRule(index)"
                    class="rounded p-1 text-af-danger transition-colors hover:text-af-danger"
                  >
                    <Icon name="x" size="sm" :stroke-width="2" />
                  </button>
                </div>
              </div>

              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label class="input-label">{{ t('admin.accounts.tempUnschedulable.errorCode') }}</label>
                  <input
                    v-model.number="rule.error_code"
                    type="number"
                    min="100"
                    max="599"
                    class="input"
                    :placeholder="t('admin.accounts.tempUnschedulable.errorCodePlaceholder')"
                  />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.accounts.tempUnschedulable.durationMinutes') }}</label>
                  <input
                    v-model.number="rule.duration_minutes"
                    type="number"
                    min="1"
                    class="input"
                    :placeholder="t('admin.accounts.tempUnschedulable.durationPlaceholder')"
                  />
                </div>
                <div class="sm:col-span-2">
                  <label class="input-label">{{ t('admin.accounts.tempUnschedulable.keywords') }}</label>
                  <input
                    v-model="rule.keywords"
                    type="text"
                    class="input"
                    :placeholder="t('admin.accounts.tempUnschedulable.keywordsPlaceholder')"
                  />
                  <p class="input-hint">{{ t('admin.accounts.tempUnschedulable.keywordsHint') }}</p>
                </div>
                <div class="sm:col-span-2">
                  <label class="input-label">{{ t('admin.accounts.tempUnschedulable.description') }}</label>
                  <input
                    v-model="rule.description"
                    type="text"
                    class="input"
                    :placeholder="t('admin.accounts.tempUnschedulable.descriptionPlaceholder')"
                  />
                </div>
              </div>
            </div>
          </div>

          <button
            type="button"
            @click="addTempUnschedRule()"
            class="w-full rounded-lg border-2 border-dashed border-af-hairline-strong px-4 py-2 text-sm text-af-ink-2 transition-colors hover:border-af-ink-4 hover:text-af-ink"
          >
            <svg
              class="mr-1 inline h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            {{ t('admin.accounts.tempUnschedulable.addRule') }}
          </button>
        </div>
      </div>

      <!-- Intercept Warmup Requests (Anthropic/Antigravity) -->
      <div
        v-if="form.platform === 'anthropic' || form.platform === 'antigravity'"
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

      <div
        v-if="form.platform === 'openai' && accountCategory === 'oauth-based'"
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
        v-if="form.platform === 'openai' && accountCategory === 'oauth-based'"
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
            <Select v-model="codexFingerprintMode" data-testid="create-codex-fingerprint-mode-select" :options="codexFingerprintModeOptions" />
          </div>
        </div>
      </div>

      <UpstreamRequestIdHeaderField
        v-model="upstreamRequestIdHeader"
        :platform="form.platform"
        :type="form.type"
      />
      </template>
    </form>

    <!-- Step 2: OAuth Authorization -->
    <div v-else class="space-y-5">
      <OAuthAuthorizationFlow
        ref="oauthFlowRef"
        :add-method="form.platform === 'anthropic' ? addMethod : 'oauth'"
        :auth-url="currentAuthUrl"
        :session-id="currentSessionId"
        :loading="currentOAuthLoading"
        :error="currentOAuthError"
        :show-help="form.platform === 'anthropic'"
        :show-proxy-warning="form.platform !== 'openai' && form.platform !== 'grok' && !!form.proxy_id"
        :allow-multiple="form.platform === 'anthropic'"
        :show-cookie-option="form.platform === 'anthropic'"
        :show-refresh-token-option="form.platform === 'openai' || form.platform === 'antigravity' || form.platform === 'grok'"
        :show-mobile-refresh-token-option="form.platform === 'openai'"
        :show-session-token-option="false"
        :show-access-token-option="false"
        :show-codex-session-import-option="form.platform === 'openai'"
        :show-agent-identity-option="form.platform === 'openai'"
        :show-codex-pat-option="form.platform === 'openai'"
        :show-sso-option="form.platform === 'grok'"
        :show-email-password-option="false"
        :show-manual-option="true"
        :initial-input-method="'manual'"
        :platform="form.platform"
        :show-project-id="geminiOAuthType === 'code_assist'"
        @generate-url="handleGenerateUrl"
        @cookie-auth="handleCookieAuth"
        @validate-refresh-token="handleValidateRefreshToken"
        @validate-mobile-refresh-token="handleOpenAIValidateMobileRT"
        @validate-session-token="handleValidateSessionToken"
        @import-codex-session="handleOpenAIImportCodexSession"
        @import-codex-pat="handleOpenAIImportCodexPAT"
        @import-sso="handleGrokImportSSO"
        @authorize-password="handleGrokAuthorizePassword"
      />

    </div>

    <template #footer>
      <div v-if="step === 1" class="flex justify-end gap-3">
        <button @click="handleClose" type="button" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="create-account-form"
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
          {{
            isOAuthFlow
              ? t('common.next')
              : submitting
                ? t('admin.accounts.creating')
                : t('common.create')
          }}
        </button>
      </div>
      <div v-else class="flex justify-between gap-3">
        <button type="button" class="btn btn-secondary" @click="goBackToBasicInfo">
          {{ t('common.back') }}
        </button>
        <button
          v-if="isManualInputMethod"
          type="button"
          :disabled="!canExchangeCode"
          class="btn btn-primary"
          @click="handleExchangeCode"
        >
          <svg
            v-if="currentOAuthLoading"
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
          {{
            currentOAuthLoading
              ? t('admin.accounts.oauth.verifying')
              : t('admin.accounts.oauth.completeAuth')
          }}
        </button>
      </div>
    </template>
  </FormPageShell>

  <!-- Gemini Help Dialog -->
  <BaseDialog
    :show="showGeminiHelpDialog"
    :title="t('admin.accounts.gemini.helpDialog.title')"
    width="wide"
    @close="showGeminiHelpDialog = false"
  >
    <div class="space-y-6">
      <!-- Setup Guide Section -->
      <div>
        <h3 class="mb-3 text-sm font-semibold text-af-ink">
          {{ t('admin.accounts.gemini.setupGuide.title') }}
        </h3>
        <div class="space-y-4">
          <div>
            <p class="mb-2 text-sm font-medium text-af-ink-2">
              {{ t('admin.accounts.gemini.setupGuide.checklistTitle') }}
            </p>
            <ul class="list-inside list-disc space-y-1 text-sm text-af-ink-2">
              <li>{{ t('admin.accounts.gemini.setupGuide.checklistItems.usIp') }}</li>
              <li>{{ t('admin.accounts.gemini.setupGuide.checklistItems.age') }}</li>
            </ul>
          </div>
          <div>
            <p class="mb-2 text-sm font-medium text-af-ink-2">
              {{ t('admin.accounts.gemini.setupGuide.activationTitle') }}
            </p>
            <ul class="list-inside list-disc space-y-1 text-sm text-af-ink-2">
              <li>{{ t('admin.accounts.gemini.setupGuide.activationItems.geminiWeb') }}</li>
              <li>{{ t('admin.accounts.gemini.setupGuide.activationItems.gcpProject') }}</li>
            </ul>
            <div class="mt-2 flex flex-wrap gap-2">
              <a
                href="https://policies.google.com/terms"
                target="_blank"
                rel="noreferrer"
                class="text-sm text-af-ink-2 hover:underline"
              >
                {{ t('admin.accounts.gemini.setupGuide.links.countryCheck') }}
              </a>
              <span class="text-af-ink-3">·</span>
              <a
                href="https://policies.google.com/country-association-form"
                target="_blank"
                rel="noreferrer"
                class="text-sm text-af-ink-2 hover:underline"
              >
                {{ t('admin.accounts.gemini.setupGuide.links.countryChange') }}
              </a>
              <span class="text-af-ink-3">·</span>
              <a
                href="https://gemini.google.com/gems/create?hl=en-US&pli=1"
                target="_blank"
                rel="noreferrer"
                class="text-sm text-af-ink-2 hover:underline"
              >
                {{ t('admin.accounts.gemini.setupGuide.links.geminiWebActivation') }}
              </a>
              <span class="text-af-ink-3">·</span>
              <a
                href="https://console.cloud.google.com"
                target="_blank"
                rel="noreferrer"
                class="text-sm text-af-ink-2 hover:underline"
              >
                {{ t('admin.accounts.gemini.setupGuide.links.gcpProject') }}
              </a>
            </div>
          </div>
        </div>
      </div>

      <!-- Quota Policy Section -->
      <div class="border-t border-af-hairline pt-6">
        <h3 class="mb-3 text-sm font-semibold text-af-ink">
          {{ t('admin.accounts.gemini.quotaPolicy.title') }}
        </h3>
        <p class="mb-4 text-xs text-af-warning">
          {{ t('admin.accounts.gemini.quotaPolicy.note') }}
        </p>
        <div class="overflow-x-auto">
          <table class="w-full text-xs">
            <thead class="bg-af-sunken">
              <tr>
                <th class="px-3 py-2 text-left font-medium text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.columns.channel') }}
                </th>
                <th class="px-3 py-2 text-left font-medium text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.columns.account') }}
                </th>
                <th class="px-3 py-2 text-left font-medium text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.columns.limits') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-af-hairline">
              <tr>
                <td class="px-3 py-2 text-af-ink">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.googleOne.channel') }}
                </td>
                <td class="px-3 py-2 text-af-ink-2">Free</td>
                <td class="px-3 py-2 text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsFree') }}
                </td>
              </tr>
              <tr>
                <td class="px-3 py-2 text-af-ink"></td>
                <td class="px-3 py-2 text-af-ink-2">Pro</td>
                <td class="px-3 py-2 text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsPro') }}
                </td>
              </tr>
              <tr>
                <td class="px-3 py-2 text-af-ink"></td>
                <td class="px-3 py-2 text-af-ink-2">Ultra</td>
                <td class="px-3 py-2 text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsUltra') }}
                </td>
              </tr>
              <tr>
                <td class="px-3 py-2 text-af-ink">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.gcp.channel') }}
                </td>
                <td class="px-3 py-2 text-af-ink-2">Standard</td>
                <td class="px-3 py-2 text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsStandard') }}
                </td>
              </tr>
              <tr>
                <td class="px-3 py-2 text-af-ink"></td>
                <td class="px-3 py-2 text-af-ink-2">Enterprise</td>
                <td class="px-3 py-2 text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsEnterprise') }}
                </td>
              </tr>
              <tr>
                <td class="px-3 py-2 text-af-ink">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.channel') }}
                </td>
                <td class="px-3 py-2 text-af-ink-2">Free</td>
                <td class="px-3 py-2 text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsFree') }}
                </td>
              </tr>
              <tr>
                <td class="px-3 py-2 text-af-ink"></td>
                <td class="px-3 py-2 text-af-ink-2">Paid</td>
                <td class="px-3 py-2 text-af-ink-2">
                  {{ t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsPaid') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mt-4 flex flex-wrap gap-3">
          <a
            :href="geminiQuotaDocs.codeAssist"
            target="_blank"
            rel="noreferrer"
            class="text-sm text-af-ink-2 hover:underline"
          >
            {{ t('admin.accounts.gemini.quotaPolicy.docs.codeAssist') }}
          </a>
          <a
            :href="geminiQuotaDocs.aiStudio"
            target="_blank"
            rel="noreferrer"
            class="text-sm text-af-ink-2 hover:underline"
          >
            {{ t('admin.accounts.gemini.quotaPolicy.docs.aiStudio') }}
          </a>
          <a
            :href="geminiQuotaDocs.vertex"
            target="_blank"
            rel="noreferrer"
            class="text-sm text-af-ink-2 hover:underline"
          >
            {{ t('admin.accounts.gemini.quotaPolicy.docs.vertex') }}
          </a>
        </div>
      </div>

      <!-- API Key Links Section -->
      <div class="border-t border-af-hairline pt-6">
        <h3 class="mb-3 text-sm font-semibold text-af-ink">
          {{ t('admin.accounts.gemini.helpDialog.apiKeySection') }}
        </h3>
        <div class="flex flex-wrap gap-3">
          <a
            :href="geminiHelpLinks.apiKey"
            target="_blank"
            rel="noreferrer"
            class="text-sm text-af-ink-2 hover:underline"
          >
            {{ t('admin.accounts.gemini.accountType.apiKeyLink') }}
          </a>
          <a
            :href="geminiHelpLinks.aiStudioPricing"
            target="_blank"
            rel="noreferrer"
            class="text-sm text-af-ink-2 hover:underline"
          >
            {{ t('admin.accounts.gemini.accountType.quotaLink') }}
          </a>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button @click="showGeminiHelpDialog = false" type="button" class="btn btn-primary">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'

import {
  PLATFORMS_WITH_VENDOR_MODEL_TABLE,
  commonErrorCodes,
  buildModelMappingObject,
  renamePresetsFor
} from '@/composables/useModelWhitelist'
import { adminAPI } from '@/api/admin'
import { useQuotaNotifyState } from '@/composables/useQuotaNotifyState'
import {
  useAccountOAuth,
  type AddMethod,
  type AuthInputMethod
} from '@/composables/useAccountOAuth'
import { useOpenAIOAuth } from '@/composables/useOpenAIOAuth'
import { useGeminiOAuth } from '@/composables/useGeminiOAuth'
import { useAntigravityOAuth } from '@/composables/useAntigravityOAuth'
import { useGrokOAuth } from '@/composables/useGrokOAuth'
import type {
  Account,
  Proxy,
  AccountPlatform,
  AccountType,
  CreateAccountRequest,
  CodexSessionImportMessage,
  OpenAICompactMode,
  OpenAIEndpointCapability,
  ProtocolEndpoints
} from '@/types'
import type { ProtocolDefaultsResponse } from '@/api/admin/accounts'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormPageShell from '@/components/admin/form/FormPageShell.vue'
import FormSectionHeading from '@/components/admin/form/FormSectionHeading.vue'
import Select from '@/components/common/Select.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import UpstreamRequestIdHeaderField from '@/components/account/UpstreamRequestIdHeaderField.vue'
import Icon from '@/components/icons/Icon.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import AccessSourcePicker from '@/components/account/AccessSourcePicker.vue'
import CatalogEntryPicker from '@/components/account/CatalogEntryPicker.vue'
import ModelRenameEditor from '@/components/account/ModelRenameEditor.vue'
import {
  DEFAULT_ACCESS_SOURCE_ID,
  findAccessSource
} from '@/components/account/accessSources'
import QuotaLimitCard from '@/components/account/QuotaLimitCard.vue'
import Toggle from '@/components/common/Toggle.vue'
import KeyAddressPresetMenu from '@/components/account/KeyAddressPresetMenu.vue'
import {
  VENDORS_WITH_CODING_PLAN,
  detectKeyVendor,
  keyAddressPresets,
  modeOfAddress,
  type KeyAddressPreset
} from '@/components/account/keyAddress'
import { platformLabel } from '@/utils/platformLabel'
import ProtocolEndpointsEditor from '@/components/account/ProtocolEndpointsEditor.vue'
import {
  UPSTREAM_PROTOCOLS,
  describeProtocolEndpointsIssue,
  currentProtocolOf,
  endpointsAfterDefaultsChange,
  hasAnthropicEndpoint,
  hasOpenAIEndpoint,
  loadProtocolDefaults,
  protocolDefaultsFor,
  trimProtocolEndpoints,
  validateProtocolEndpoints
} from '@/components/account/protocolEndpoints'
import HeaderOverrideEditor from '@/components/account/HeaderOverrideEditor.vue'
import {
  applyAntigravityProjectID,
  applyHeaderOverride,
  applyInterceptWarmup,
  isHeaderOverrideCapable,
  validateHeaderOverrideRows,
  type CnAccountMode,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import {
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
  type OpenAIWSMode
} from '@/utils/openaiWsMode'
import OAuthAuthorizationFlow from './OAuthAuthorizationFlow.vue'

// Type for exposed OAuthAuthorizationFlow component
// Note: defineExpose automatically unwraps refs, so we use the unwrapped types
interface OAuthFlowExposed {
  authCode: string
  oauthState: string
  projectId: string
  sessionKey: string
  refreshToken: string
  sessionToken: string
  codexSession: string
  codexPAT: string
  ssoCookie: string
  inputMethod: AuthInputMethod
  reset: () => void
}

const { t } = useI18n()
const browserTimeZone = getBrowserTimeZone()

const oauthStepTitle = computed(() => {
  if (form.platform === 'openai') return t('admin.accounts.oauth.openai.title')
  if (form.platform === 'gemini') return t('admin.accounts.oauth.gemini.title')
  if (form.platform === 'antigravity') return t('admin.accounts.oauth.antigravity.title')
  if (form.platform === 'grok') return t('admin.accounts.oauth.grok.title')
  return t('admin.accounts.oauth.title')
})

// Platform-specific hints for API Key type
// 上游ID：直接上游声明请求标识的响应头名，留空不记录。
const upstreamRequestIdHeader = ref('')
const withUpstreamRequestIdHeader = <T extends Record<string, unknown> | undefined>(extra: T): T | Record<string, unknown> => {
  const name = upstreamRequestIdHeader.value.trim()
  if (!name) return extra
  return { ...(extra || {}), upstream_request_id_header: name }
}

// API Key 的提示与占位跟着按地址识别出的厂商走；中转用通用说法
const apiKeyHint = computed(() => {
  switch (keyVendor.value) {
    case 'anthropic':
      return t('admin.accounts.apiKeyHint')
    case 'openai':
      return t('admin.accounts.openai.apiKeyHint')
    case 'gemini':
      return t('admin.accounts.gemini.apiKeyHint')
    default:
      return t('admin.accounts.upstream.apiKeyHint')
  }
})

const apiKeyValuePlaceholder = computed(() => {
  switch (keyVendor.value) {
    case 'anthropic':
      return 'sk-ant-...'
    case 'openai':
      return 'sk-proj-...'
    case 'gemini':
      return 'AIza...'
    case 'grok':
      return 'xai-...'
    case 'zhipu':
      return '<api-key>.<secret>'
    default:
      return 'sk-...'
  }
})

interface Props {
  show: boolean
  proxies: Proxy[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  created: []
}>()

const appStore = useAppStore()

// OAuth composables
const oauth = useAccountOAuth() // For Anthropic OAuth
const openaiOAuth = useOpenAIOAuth() // For OpenAI OAuth
const geminiOAuth = useGeminiOAuth() // For Gemini OAuth
const antigravityOAuth = useAntigravityOAuth() // For Antigravity OAuth
const grokOAuth = useGrokOAuth() // For Grok OAuth

// Computed: current OAuth state for template binding
const currentAuthUrl = computed(() => {
  if (form.platform === 'openai') return openaiOAuth.authUrl.value
  if (form.platform === 'gemini') return geminiOAuth.authUrl.value
  if (form.platform === 'antigravity') return antigravityOAuth.authUrl.value
  if (form.platform === 'grok') return grokOAuth.authUrl.value
  return oauth.authUrl.value
})

const currentSessionId = computed(() => {
  if (form.platform === 'openai') return openaiOAuth.sessionId.value
  if (form.platform === 'gemini') return geminiOAuth.sessionId.value
  if (form.platform === 'antigravity') return antigravityOAuth.sessionId.value
  if (form.platform === 'grok') return grokOAuth.sessionId.value
  return oauth.sessionId.value
})

const currentOAuthLoading = computed(() => {
  if (form.platform === 'openai') return openaiOAuth.loading.value
  if (form.platform === 'gemini') return geminiOAuth.loading.value
  if (form.platform === 'antigravity') return antigravityOAuth.loading.value
  if (form.platform === 'grok') return grokOAuth.loading.value
  return oauth.loading.value
})

const currentOAuthError = computed(() => {
  if (form.platform === 'openai') return openaiOAuth.error.value
  if (form.platform === 'gemini') return geminiOAuth.error.value
  if (form.platform === 'antigravity') return antigravityOAuth.error.value
  if (form.platform === 'grok') return grokOAuth.error.value
  return oauth.error.value
})

// Refs
const oauthFlowRef = ref<OAuthFlowExposed | null>(null)

// Model mapping type
interface ModelMapping {
  from: string
  to: string
}

interface TempUnschedRuleForm {
  error_code: number | null
  keywords: string
  duration_minutes: number | null
  description: string
}

// State
const step = ref(1)
const submitting = ref(false)
const accountCategory = ref<'oauth-based' | 'apikey' | 'bedrock' | 'service_account'>('oauth-based') // UI selection for account category
const addMethod = ref<AddMethod>('oauth') // For oauth-based: 'oauth' or 'setup-token'
const apiKeyValue = ref('')
const upstreamBillingAutoProbeEnabled = ref(true)

// 智谱团队版 Coding Plan：组织/项目 ID，写入 credentials 供额度探测切换团队端点
const zhipuOrganization = ref('')
const zhipuProject = ref('')

// ── 接入方式（新建渠道第一步，见 accessSources.ts） ──
const accessSourceId = ref(DEFAULT_ACCESS_SOURCE_ID)
const accessSource = computed(() => findAccessSource(accessSourceId.value))
// 第三方 key 不选平台：form.platform 只是表单内部占位，提交时不带，厂商按地址识别（keyVendor）
const isKeyMode = computed(() => accessSource.value.kind === 'key')
const selectedCatalogEntryIds = ref<number[]>([])
// 默认只露必填项，其余在「更多设置」里（muqian 2026-09-25「还是太繁琐，要填的东西太多了」）
const showMoreSettings = ref(false)

// 选接入方式 / 成品号的厂商 = 切到它的平台与类别。先换平台并等平台 watcher 跑完（它会重置平台相关字段，
// 部分平台还会把类别复位成成品号），再定类别。
async function applyAccessSource(sourceId: string) {
  const source = findAccessSource(sourceId)
  if (form.platform !== source.platform) {
    form.platform = source.platform
    await nextTick()
  }
  accountCategory.value = source.category
}
watch(accessSourceId, (sourceId) => {
  void applyAccessSource(sourceId)
})

// ── 第三方 key 协议地址（muqian 2026-09-25：key 不选平台） ──
// 存库的就是管理员填的显式地址：可从常用官方地址里选一条填入，也可直接填中转地址。
const protocolDefaults = ref<ProtocolDefaultsResponse | null>(null)
const protocolDefaultsLoadFailed = ref(false)
const protocolEndpoints = ref<ProtocolEndpoints>({})
const keyPresets = computed(() => keyAddressPresets(protocolDefaults.value))
// 按地址识别出的厂商（官方域名表由后端下发，与后端 Account.Vendor 同口径）；认不出的是中转
const keyVendor = computed(() =>
  isKeyMode.value ? detectKeyVendor(protocolEndpoints.value, protocolDefaults.value?.vendor_hosts) : null
)
const hasKeyAddress = computed(() => Object.values(protocolEndpoints.value).some((url) => !!url?.trim()))
// 按量 / Coding 套餐：只有 Kimi / 智谱 / MiniMax 有。地址能分出来就跟地址走，分不出来（MiniMax 同地址）由管理员选
const CN_PLAN_MODES = [
  { value: 'payg', icon: 'creditCard' },
  { value: 'coding', icon: 'bolt' }
] as const
const keyPlanMode = ref<CnAccountMode>('payg')
const keyHasCodingPlan = computed(() => !!keyVendor.value && VENDORS_WITH_CODING_PLAN.has(keyVendor.value))
// 地址本身定得了套餐就不问（识别提示里带上）；MiniMax 两种套餐同地址、智谱 Anthropic 同地址才要选
const keyPlanFromAddress = computed(() => {
  const vendor = keyVendor.value
  if (!vendor || !keyHasCodingPlan.value) return null
  const mode = modeOfAddress(keyPresets.value, vendor, protocolEndpoints.value)
  return mode === 'payg' || mode === 'coding' ? mode : null
})
const keyPlanNeedsChoice = computed(() => keyHasCodingPlan.value && keyPlanFromAddress.value === null)
const keyVendorLabel = computed(() => {
  const vendor = keyVendor.value
  if (!vendor) return ''
  const plan = keyPlanFromAddress.value
  return plan ? `${platformLabel(vendor)} · ${t(`admin.accounts.cnProviders.accountMode.${plan}`)}` : platformLabel(vendor)
})
// 写进 credentials.account_mode 的模式：国产厂商按量 / 套餐，OpenCode 的 Zen / Go 由地址定；中转不写
const keyAccountMode = computed<string | undefined>(() => {
  const vendor = keyVendor.value
  if (!vendor) return undefined
  if (vendor === 'opencode_go') return modeOfAddress(keyPresets.value, vendor, protocolEndpoints.value) ?? 'zen'
  if (vendor === 'deepseek') return 'payg'
  return keyHasCodingPlan.value ? keyPlanMode.value : undefined
})
watch(keyPlanFromAddress, (mode) => {
  if (mode) keyPlanMode.value = mode
})
// 换套餐时，地址还是上一个套餐的官方地址（没改过）就换成新套餐同协议的官方地址
watch(keyPlanMode, (mode, previous) => {
  const vendor = keyVendor.value
  const current = currentProtocolOf(protocolEndpoints.value)
  if (!vendor || !current || mode === previous) return
  // 新套餐在这个协议上没有官方地址就不动，换套餐不能把地址清掉
  const next = protocolDefaultsFor(protocolDefaults.value, vendor, mode)
  if (!next[current]) return
  protocolEndpoints.value = endpointsAfterDefaultsChange(
    protocolEndpoints.value,
    protocolDefaultsFor(protocolDefaults.value, vendor, previous),
    next,
    current
  )
})
// 识别出的厂商在当前套餐下的官方地址：给地址编辑器的「填入官方地址」与换协议用
const officialProtocolEndpoints = computed<ProtocolEndpoints>(() => {
  const vendor = keyVendor.value
  if (!vendor) return {}
  const mode = vendor === 'opencode_go' ? keyAccountMode.value : keyHasCodingPlan.value ? keyPlanMode.value : undefined
  return protocolDefaultsFor(protocolDefaults.value, vendor, mode)
})
function applyKeyAddressPreset(preset: KeyAddressPreset) {
  protocolEndpoints.value = { [preset.protocol]: preset.url }
  if (preset.mode === 'payg' || preset.mode === 'coding') keyPlanMode.value = preset.mode
}
// 承接的模型：成品号的厂商、或 key 按地址识别出的厂商排在最前；中转没有
const catalogEntries = ref<ModelCatalogEntry[]>([])
const catalogSelectionTouched = ref(false)
const catalogSuggestedPlatform = computed(() =>
  isKeyMode.value ? (keyVendor.value ?? undefined) : accessSource.value.platform
)
// 承接模型的默认勾选：识别出的厂商（成品号即它的平台）已上架的对话模型；管理员动过就不再改。
// 生图 / 视频 / 向量走扩展端点，另有承接条件（key 要有 Chat Completions 地址），默认不勾，免得整批绑定被拒
watch(
  () => [catalogEntries.value, catalogSuggestedPlatform.value] as const,
  ([entries, platform]) => {
    if (catalogSelectionTouched.value) return
    selectedCatalogEntryIds.value = platform
      ? entries
          .filter((entry) => entry.status === 'listed' && entry.vendor_platform === platform && !entry.extension_endpoints)
          .map((entry) => entry.id)
      : []
  }
)
async function ensureProtocolDefaults() {
  try {
    protocolDefaults.value = await loadProtocolDefaults()
    protocolDefaultsLoadFailed.value = false
  } catch {
    protocolDefaultsLoadFailed.value = true
  }
}
// 提交前校验协议地址，有问题直接提示并返回 null。
function validatedProtocolEndpoints(): ProtocolEndpoints | null {
  const issue = validateProtocolEndpoints(protocolEndpoints.value)
  if (issue) {
    appStore.showError(describeProtocolEndpointsIssue(issue, t))
    return null
  }
  return trimProtocolEndpoints(protocolEndpoints.value)
}

const editQuotaLimit = ref<number | null>(null)
const editQuotaDailyLimit = ref<number | null>(null)
const editQuotaWeeklyLimit = ref<number | null>(null)
const editDailyResetMode = ref<'rolling' | 'fixed' | null>(null)
const editDailyResetHour = ref<number | null>(null)
const editWeeklyResetMode = ref<'rolling' | 'fixed' | null>(null)
const editWeeklyResetDay = ref<number | null>(null)
const editWeeklyResetHour = ref<number | null>(null)
const editResetTimezone = ref<string | null>(null)
// 模型改名（可选）：只改名、不兼任白名单，写入时带 model_mapping_rename_only（见 withRenameOnlyMapping）
const modelMappings = ref<ModelMapping[]>([])
const buildRenameMapping = () => buildModelMappingObject('mapping', [], modelMappings.value)
const openAICompactModelMappings = ref<ModelMapping[]>([])
const DEFAULT_POOL_MODE_RETRY_COUNT = 3
const MAX_POOL_MODE_RETRY_COUNT = 10
const DEFAULT_POOL_MODE_RETRY_STATUS_CODES = [401, 403, 429]
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
const customErrorCodesEnabled = ref(false)
const selectedErrorCodes = ref<number[]>([])
const customErrorCodeInput = ref<number | null>(null)
const headerOverrideEnabled = ref(false)
const headerOverrideRows = ref<HeaderOverrideRow[]>([])

// 请求头覆写的前置校验，失败时提示并返回 false。
// Grok OAuth 三条创建路径（授权码/RT 批量/SSO 批量）必须在兑换 code 之前调用，
// 避免校验失败时白白消耗一次性授权码。
const validateHeaderOverrideForm = (): boolean => {
  if (headerOverrideEnabled.value) {
    const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
    if (headerError) {
      appStore.showError(t(`admin.accounts.headerOverride.${headerError}`))
      return false
    }
  }
  return true
}

// 把已通过校验的请求头覆写写入 credentials（Grok OAuth 成品号只走官方地址，只有这一项上游配置）
const applyGrokOAuthUpstreamConfig = (credentials: Record<string, unknown>) => {
  applyHeaderOverride(credentials, headerOverrideEnabled.value, headerOverrideRows.value, 'create')
}

// 第三方 key（含 Antigravity 上游 key）的请求头覆写：校验通过才写入 credentials。
const applyKeyHeaderOverride = (credentials: Record<string, unknown>): boolean => {
  if (!validateHeaderOverrideForm()) {
    return false
  }
  applyHeaderOverride(credentials, headerOverrideEnabled.value, headerOverrideRows.value, 'create')
  return true
}
const interceptWarmupRequests = ref(false)
const autoPauseOnExpired = ref(true)
const openaiPassthroughEnabled = ref(false)
// OpenAI Codex namespace 工具摊平兼容开关（仅 OAuth），缺省关闭即原样保留
const openaiFlattenNamespacesEnabled = ref(false)
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
const codexFingerprintModeOptions = computed(() => [
  { value: 'off' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintOff') },
  { value: 'device' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintDevice') },
  { value: 'session' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintSession') },
  { value: 'full' as CodexFingerprintMode, label: t('admin.accounts.openai.codexFingerprintFull') },
])
type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'
const anthropicAPIKeyAuthScheme = ref<AnthropicAPIKeyAuthScheme>('x_api_key')
const bedrockCCCompatEnabled = ref(false)

const {
  globalEnabled: quotaNotifyGlobalEnabled,
  state: quotaNotifyState,
  writeToExtra: writeQuotaNotifyToExtra,
} = useQuotaNotifyState()

const allowOverages = ref(false) // For antigravity accounts: enable AI Credits overages
const antigravityProjectId = ref('')

// Bedrock credentials
const bedrockAuthMode = ref<'sigv4' | 'apikey'>('sigv4')
const bedrockAccessKeyId = ref('')
const bedrockSecretAccessKey = ref('')
const bedrockRegion = ref('us-east-1')
const bedrockForceGlobal = ref(false)
const bedrockApiKeyValue = ref('')
const vertexServiceAccountFileInput = ref<HTMLInputElement | null>(null)
const vertexServiceAccountJson = ref('')
const vertexLocation = ref('global')
const vertexServiceAccountDragActive = ref(false)
const tempUnschedEnabled = ref(false)
const tempUnschedRules = ref<TempUnschedRuleForm[]>([])
const getOpenAICompactModelMappingKey = createStableObjectKeyResolver<ModelMapping>('create-openai-compact-model-mapping')
const getTempUnschedRuleKey = createStableObjectKeyResolver<TempUnschedRuleForm>('create-temp-unsched-rule')
const geminiOAuthType = ref<'code_assist' | 'google_one' | 'ai_studio'>('google_one')
const geminiAIStudioOAuthEnabled = ref(false)
const openAICompactModeOptions = computed(() => [
  { value: 'auto', label: t('admin.accounts.openai.compactModeAuto') },
  { value: 'force_on', label: t('admin.accounts.openai.compactModeForceOn') },
  { value: 'force_off', label: t('admin.accounts.openai.compactModeForceOff') }
])
const openAIEndpointCapabilityOptions = computed<{ value: OpenAIEndpointCapability; label: string }[]>(() => [
  { value: 'chat_completions', label: t('admin.accounts.openai.capabilityText') },
  { value: 'embeddings', label: t('admin.accounts.openai.capabilityEmbeddings') }
])

const normalizeOpenAIEndpointCapabilities = (values: OpenAIEndpointCapability[]) => {
  const allowed: OpenAIEndpointCapability[] = ['chat_completions', 'embeddings']
  const selected = allowed.filter((value) => values.includes(value))
  return selected.length > 0 ? selected : allowed
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

function buildAntigravityExtra(): Record<string, unknown> | undefined {
  const extra: Record<string, unknown> = {}
  if (allowOverages.value) extra.allow_overages = true
  return Object.keys(extra).length > 0 ? extra : undefined
}

const buildOpenAICompactModelMapping = () =>
  buildModelMappingObject('mapping', [], openAICompactModelMappings.value)

// 第三方 key 的 Compact 专属模型映射：与 Compact 模式同区块，区块可见才写入。
const applyKeyCompactModelMapping = (credentials: Record<string, unknown>) => {
  if (!openAIResponsesSettingsVisible.value) {
    return
  }
  const compactModelMapping = buildOpenAICompactModelMapping()
  if (compactModelMapping) {
    credentials.compact_model_mapping = compactModelMapping
  }
}

const showAdvancedOAuth = ref(false)
const showGeminiHelpDialog = ref(false)

// Quota control state (Anthropic OAuth/SetupToken only)
// 空闲超时、RPM 策略 / 粘性缓冲、用户消息限速、TLS 指纹、会话 ID 伪装、缓存 TTL 替换已写死在后端
// （channel_features_anthropic.go），表单不再提供。
const sessionLimitEnabled = ref(false)
const maxSessions = ref<number | null>(null)
const rpmLimitEnabled = ref(false)
const baseRpm = ref<number | null>(null)

// Gemini tier selection (used as fallback when auto-detection is unavailable/fails)
const geminiTierGoogleOne = ref<'google_one_free' | 'google_ai_pro' | 'google_ai_ultra'>('google_one_free')
const geminiTierGcp = ref<'gcp_standard' | 'gcp_enterprise'>('gcp_standard')
const geminiTierAIStudio = ref<'aistudio_free' | 'aistudio_paid'>('aistudio_free')

const geminiSelectedTier = computed(() => {
  if (form.platform !== 'gemini') return ''
  if (accountCategory.value === 'apikey') return geminiTierAIStudio.value
  switch (geminiOAuthType.value) {
    case 'google_one':
      return geminiTierGoogleOne.value
    case 'code_assist':
      return geminiTierGcp.value
    default:
      return geminiTierAIStudio.value
  }
})

const openAIWSModeOptions = computed(() => [
  { value: OPENAI_WS_MODE_OFF, label: t('admin.accounts.openai.wsModeOff') },
  { value: OPENAI_WS_MODE_CTX_POOL, label: t('admin.accounts.openai.wsModeCtxPool') },
  { value: OPENAI_WS_MODE_PASSTHROUGH, label: t('admin.accounts.openai.wsModePassthrough') },
  { value: OPENAI_WS_MODE_HTTP_BRIDGE, label: t('admin.accounts.openai.wsModeHttpBridge') }
])

const openaiResponsesWebSocketV2Mode = computed({
  get: () => {
    if (form.type === 'apikey') {
      return openaiAPIKeyResponsesWebSocketV2Mode.value
    }
    return openaiOAuthResponsesWebSocketV2Mode.value
  },
  set: (mode: OpenAIWSMode) => {
    if (form.type === 'apikey') {
      openaiAPIKeyResponsesWebSocketV2Mode.value = mode
      return
    }
    openaiOAuthResponsesWebSocketV2Mode.value = mode
  }
})

const openAIWSModeHintKey = computed(() =>
  resolveOpenAIWSModeHintKey(openaiResponsesWebSocketV2Mode.value)
)

const geminiQuotaDocs = {
  codeAssist: 'https://developers.google.com/gemini-code-assist/resources/quotas',
  aiStudio: 'https://ai.google.dev/pricing',
  vertex: 'https://cloud.google.com/vertex-ai/generative-ai/docs/quotas'
}

const geminiHelpLinks = {
  apiKey: 'https://aistudio.google.com/app/apikey',
  aiStudioPricing: 'https://ai.google.dev/pricing',
  gcpProject: 'https://console.cloud.google.com/welcome/new',
  geminiWebActivation: 'https://gemini.google.com/gems/create?hl=en-US&pli=1',
  countryCheck: 'https://policies.google.com/terms',
  countryChange: 'https://policies.google.com/country-association-form'
}

// 改名快捷项（同名预设只对自带模型表的上游保留，见 renamePresetsFor）
const renamePresets = computed(() => {
  if (isKeyMode.value) return keyVendor.value ? renamePresetsFor(keyVendor.value) : []
  return renamePresetsFor(accountCategory.value === 'bedrock' ? 'bedrock' : form.platform)
})
const extendsVendorTable = computed(() =>
  PLATFORMS_WITH_VENDOR_MODEL_TABLE.has(isKeyMode.value ? (keyVendor.value ?? '') : form.platform)
)
const tempUnschedPresets = computed(() => [
  {
    label: t('admin.accounts.tempUnschedulable.presets.overloadLabel'),
    rule: {
      error_code: 529,
      keywords: 'overloaded, too many',
      duration_minutes: 60,
      description: t('admin.accounts.tempUnschedulable.presets.overloadDesc')
    }
  },
  {
    label: t('admin.accounts.tempUnschedulable.presets.rateLimitLabel'),
    rule: {
      error_code: 429,
      keywords: 'rate limit, too many requests',
      duration_minutes: 10,
      description: t('admin.accounts.tempUnschedulable.presets.rateLimitDesc')
    }
  },
  {
    label: t('admin.accounts.tempUnschedulable.presets.unavailableLabel'),
    rule: {
      error_code: 503,
      keywords: 'unavailable, maintenance',
      duration_minutes: 30,
      description: t('admin.accounts.tempUnschedulable.presets.unavailableDesc')
    }
  }
])

const form = reactive({
  name: '',
  notes: '',
  platform: 'anthropic' as AccountPlatform,
  type: 'oauth' as AccountType, // Will be 'oauth', 'setup-token', or 'apikey'
  credentials: {} as Record<string, unknown>,
  proxy_id: null as number | null,
  concurrency: 10,
  load_factor: null as number | null,
  priority: 1,
  rate_multiplier: 1,
  expires_at: null as number | null
})

// Helper to check if current type needs OAuth flow
const isOAuthFlow = computed(() => {
  // Bedrock 类型不需要 OAuth 流程
  if (form.platform === 'anthropic' && accountCategory.value === 'bedrock') {
    return false
  }
  return accountCategory.value === 'oauth-based'
})

// 第三方 key 的 Anthropic 协议设置按编辑中的协议地址展示，不看平台标签。
// 区块隐藏（换成成品号、删掉 anthropic 地址行）时提交不写入（见 buildAnthropicExtra）；切换平台时清空。
const anthropicKeySettingsVisible = computed(
  () => form.type === 'apikey' && hasAnthropicEndpoint(protocolEndpoints.value)
)

// OpenAI Responses 协议设置（自动透传、WS mode、Compact）：OpenAI 成品号沿用平台规则；
// 第三方 key 按编辑中的协议地址（responses 或 chat_completions）展示，不看平台标签。
// 区块隐藏时提交不写入（见 buildOpenAIExtra）；切换平台时清空。
const openAIResponsesSettingsVisible = computed(() => {
  if (form.type === 'apikey') return hasOpenAIEndpoint(protocolEndpoints.value)
  return form.platform === 'openai' && accountCategory.value === 'oauth-based'
})

// 端点能力与生图结果转 base64 是第三方 key 专属设置：后端对任意标签的 key 都生效，
// 按填写中的协议地址展示，不看平台标签。
const openAIKeySettingsVisible = computed(
  () => form.type === 'apikey' && hasOpenAIEndpoint(protocolEndpoints.value)
)

// 自动透传会跳过模型改写：透传区块可见且开启时，模型限制不再可编辑。
const isOpenAIModelRestrictionDisabled = computed(() =>
  openAIResponsesSettingsVisible.value && openaiPassthroughEnabled.value
)

// 表单分区（A5-c）：「基本」「模型与映射」「额度」「高级」总有字段（承接的模型所有接入方式都有）；
// 「地址与协议」只在分区里有区块时才出标题，条件与分区内各区块的 v-if 一一对应（改区块条件时这里一起改）。
const showEndpointSection = computed(() =>
  form.type === 'apikey' ||
  (form.platform === 'openai' && form.type === 'oauth') ||
  openAIResponsesSettingsVisible.value ||
  anthropicKeySettingsVisible.value ||
  openAIKeySettingsVisible.value
)
// 模型改名：沿用原来有模型映射的接入方式（第三方 key、Bedrock、Antigravity、OpenAI / Grok 成品号）
// 「更多设置」里的模型分区：改名或 Compact 有一个就出标题（承接的模型在前面，不在这个分区）
const showModelMoreSection = computed(() => showModelRename.value || openAIResponsesSettingsVisible.value)
const showModelRename = computed(() =>
  form.platform === 'antigravity' ||
  form.type === 'apikey' ||
  (form.platform === 'anthropic' && accountCategory.value === 'bedrock') ||
  ((form.platform === 'openai' || form.platform === 'grok') && isOAuthFlow.value)
)

const isGrokSSOInputMethod = computed(() => form.platform === 'grok' && oauthFlowRef.value?.inputMethod === 'sso_cookie')

const isManualInputMethod = computed(() => {
  return oauthFlowRef.value?.inputMethod === 'manual'
})

const expiresAtInput = computed({
  get: () => formatDateTimeLocal(form.expires_at),
  set: (value: string) => {
    form.expires_at = parseDateTimeLocal(value)
  }
})

const canExchangeCode = computed(() => {
  const authCode = oauthFlowRef.value?.authCode || ''
  if (form.platform === 'openai') {
    return authCode.trim() && openaiOAuth.sessionId.value && !openaiOAuth.loading.value
  }
  if (form.platform === 'gemini') {
    return authCode.trim() && geminiOAuth.sessionId.value && !geminiOAuth.loading.value
  }
  if (form.platform === 'antigravity') {
    return authCode.trim() && antigravityOAuth.sessionId.value && !antigravityOAuth.loading.value
  }
  if (form.platform === 'grok') {
    return authCode.trim() && grokOAuth.sessionId.value && !grokOAuth.loading.value
  }
  return authCode.trim() && oauth.sessionId.value && !oauth.loading.value
})

// Watchers
watch(
  () => props.show,
  (newVal) => {
    if (!newVal) {
      resetForm()
    }
  }
)

// Sync form.type based on accountCategory, addMethod, and platform-specific type
watch(
  [accountCategory, addMethod, () => form.platform],
  ([category, method]) => {
    // Bedrock 类型
    if (form.platform === 'anthropic' && category === 'bedrock') {
      form.type = 'bedrock' as AccountType
      return
    }
    if ((form.platform === 'gemini' || form.platform === 'anthropic') && category === 'service_account') {
      form.type = 'service_account' as AccountType
    } else if (category === 'oauth-based') {
      form.type = form.platform === 'anthropic' ? method as AccountType : 'oauth'
    } else {
      form.type = 'apikey'
    }
  },
  { immediate: true }
)

// 弹窗打开时加载官方地址（含挂载时即打开的情况）。
watch(
  () => props.show,
  (show) => {
    if (show) void ensureProtocolDefaults()
  },
  { immediate: true }
)

// Reset platform-specific settings when platform changes
watch(
  () => form.platform,
  (newPlatform) => {
    // 改名是按平台的模型名写的，换平台清空（Antigravity 的默认表由后端叠加，不再预填）
    modelMappings.value = []
    if (newPlatform === 'antigravity') {
      accountCategory.value = 'oauth-based'
    } else {
      allowOverages.value = false
      antigravityProjectId.value = ''
    }
    if (newPlatform === 'grok') {
      accountCategory.value = 'oauth-based'
      addMethod.value = 'oauth'
      form.concurrency = 1
      form.load_factor = null
    }
    if (newPlatform !== 'gemini' && newPlatform !== 'anthropic' && accountCategory.value === 'service_account') {
      accountCategory.value = 'oauth-based'
    }
    if (newPlatform !== 'anthropic' && accountCategory.value === 'bedrock') {
      accountCategory.value = 'oauth-based'
    }
    // Reset Bedrock fields when switching platforms
    bedrockAccessKeyId.value = ''
    bedrockSecretAccessKey.value = ''
    bedrockRegion.value = 'us-east-1'
    bedrockForceGlobal.value = false
    bedrockAuthMode.value = 'sigv4'
    bedrockApiKeyValue.value = ''
    vertexServiceAccountJson.value = ''
    vertexLocation.value = 'global'
    // Reset Anthropic/Antigravity-specific settings when switching to other platforms
    if (newPlatform !== 'anthropic' && newPlatform !== 'antigravity') {
      interceptWarmupRequests.value = false
    }
    if (newPlatform !== 'openai') {
      openaiFlattenNamespacesEnabled.value = false
      openAIEndpointCapabilities.value = ['chat_completions', 'embeddings']
      openaiOAuthResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
      codexCLIOnlyEnabled.value = false
      codexCLIOnlyAppServerEnabled.value = false
    }
    // 第三方 key 也能配的协议设置（OpenAI 自动透传 / key 的 WS mode / Compact，Anthropic
    // 认证方式 / Bedrock CC 兼容）：切换平台一律清空（不看切到哪个平台），与请求头覆写一致；
    // 同一平台内删掉地址行导致的隐藏，由提交时的可见性判断保证不写入。
    openaiPassthroughEnabled.value = false
    openaiAPIKeyResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
    openAICompactMode.value = 'auto'
    openAICompactModelMappings.value = []
    anthropicAPIKeyAuthScheme.value = 'x_api_key'
    bedrockCCCompatEnabled.value = false
    // 请求头覆写为平台相关配置（常用头集合不同），切换平台时清空，
    // 避免上一平台的配置行被提交到新平台账号
    headerOverrideEnabled.value = false
    headerOverrideRows.value = []
    openAIImagesUrlToB64JsonEnabled.value = false
    // Reset OAuth states
    oauth.resetState()
    openaiOAuth.resetState()

    geminiOAuth.resetState()
    antigravityOAuth.resetState()
    grokOAuth.resetState()
  }
)

// Gemini AI Studio OAuth availability (requires operator-configured OAuth client)
watch(
  [accountCategory, () => form.platform],
  ([category, platform]) => {
    if (platform === 'openai' && category !== 'oauth-based') {
      codexCLIOnlyEnabled.value = false
      codexCLIOnlyAppServerEnabled.value = false
    }
  }
)

watch(
  [() => props.show, () => form.platform, accountCategory],
  async ([show, platform, category]) => {
    if (!show || platform !== 'gemini' || category !== 'oauth-based') {
      geminiAIStudioOAuthEnabled.value = false
      return
    }
    const caps = await geminiOAuth.getCapabilities()
    geminiAIStudioOAuthEnabled.value = !!caps?.ai_studio_oauth_enabled
    if (!geminiAIStudioOAuthEnabled.value && geminiOAuthType.value === 'ai_studio') {
      geminiOAuthType.value = 'code_assist'
    }
  },
  { immediate: true }
)

const handleSelectGeminiOAuthType = (oauthType: 'code_assist' | 'google_one' | 'ai_studio') => {
  if (oauthType === 'ai_studio' && !geminiAIStudioOAuthEnabled.value) {
    appStore.showError(t('admin.accounts.oauth.gemini.aiStudioNotConfigured'))
    return
  }
  geminiOAuthType.value = oauthType
}

// Model mapping helpers
const addOpenAICompactModelMapping = () => {
  openAICompactModelMappings.value.push({ from: '', to: '' })
}

const removeOpenAICompactModelMapping = (index: number) => {
  openAICompactModelMappings.value.splice(index, 1)
}

// Error code toggle helper
const toggleErrorCode = (code: number) => {
  const index = selectedErrorCodes.value.indexOf(code)
  if (index === -1) {
    // Adding code - check for 429/529 warning
    if (code === 429) {
      if (!confirm(t('admin.accounts.customErrorCodes429Warning'))) {
        return
      }
    } else if (code === 529) {
      if (!confirm(t('admin.accounts.customErrorCodes529Warning'))) {
        return
      }
    }
    selectedErrorCodes.value.push(code)
  } else {
    selectedErrorCodes.value.splice(index, 1)
  }
}

// Add custom error code from input
const addCustomErrorCode = () => {
  const code = customErrorCodeInput.value
  if (code === null || code < 100 || code > 599) {
    appStore.showError(t('admin.accounts.invalidErrorCode'))
    return
  }
  if (selectedErrorCodes.value.includes(code)) {
    appStore.showInfo(t('admin.accounts.errorCodeExists'))
    return
  }
  // Check for 429/529 warning
  if (code === 429) {
    if (!confirm(t('admin.accounts.customErrorCodes429Warning'))) {
      return
    }
  } else if (code === 529) {
    if (!confirm(t('admin.accounts.customErrorCodes529Warning'))) {
      return
    }
  }
  selectedErrorCodes.value.push(code)
  customErrorCodeInput.value = null
}

// Remove error code
const removeErrorCode = (code: number) => {
  const index = selectedErrorCodes.value.indexOf(code)
  if (index !== -1) {
    selectedErrorCodes.value.splice(index, 1)
  }
}

const addTempUnschedRule = (preset?: TempUnschedRuleForm) => {
  if (preset) {
    tempUnschedRules.value.push({ ...preset })
    return
  }
  tempUnschedRules.value.push({
    error_code: null,
    keywords: '',
    duration_minutes: 30,
    description: ''
  })
}

const removeTempUnschedRule = (index: number) => {
  tempUnschedRules.value.splice(index, 1)
}

const moveTempUnschedRule = (index: number, direction: number) => {
  const target = index + direction
  if (target < 0 || target >= tempUnschedRules.value.length) return
  const rules = tempUnschedRules.value
  const current = rules[index]
  rules[index] = rules[target]
  rules[target] = current
}

const buildTempUnschedRules = (rules: TempUnschedRuleForm[]) => {
  const out: Array<{
    error_code: number
    keywords: string[]
    duration_minutes: number
    description: string
  }> = []

  for (const rule of rules) {
    const errorCode = Number(rule.error_code)
    const duration = Number(rule.duration_minutes)
    const keywords = splitTempUnschedKeywords(rule.keywords)
    if (!Number.isFinite(errorCode) || errorCode < 100 || errorCode > 599) {
      continue
    }
    if (!Number.isFinite(duration) || duration <= 0) {
      continue
    }
    if (keywords.length === 0) {
      continue
    }
    out.push({
      error_code: Math.trunc(errorCode),
      keywords,
      duration_minutes: Math.trunc(duration),
      description: rule.description.trim()
    })
  }

  return out
}

const applyTempUnschedConfig = (credentials: Record<string, unknown>) => {
  if (!tempUnschedEnabled.value) {
    delete credentials.temp_unschedulable_enabled
    delete credentials.temp_unschedulable_rules
    return true
  }

  const rules = buildTempUnschedRules(tempUnschedRules.value)
  if (rules.length === 0) {
    appStore.showError(t('admin.accounts.tempUnschedulable.rulesInvalid'))
    return false
  }

  credentials.temp_unschedulable_enabled = true
  credentials.temp_unschedulable_rules = rules
  return true
}

const splitTempUnschedKeywords = (value: string) => {
  return value
    .split(/[,;]/)
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
}

// 映射只改名（muqian 2026-09-25 去掉白名单）：写了 model_mapping 就一并打标记，渠道承接哪些模型看目录绑定。
const withRenameOnlyMapping = (credentials: Record<string, unknown>): Record<string, unknown> => {
  const out = { ...credentials }
  if (out.model_mapping) out.model_mapping_rename_only = true
  else delete out.model_mapping_rename_only
  return out
}

// 把勾选的目录模型写成这些渠道的绑定。绑定失败不回滚建号，提示到编辑页再勾。
const bindSelectedCatalogEntries = async (accountIds: number[]) => {
  const entryIds = [...selectedCatalogEntryIds.value]
  if (entryIds.length === 0) return
  for (const accountId of accountIds) {
    try {
      await adminAPI.modelCatalog.replaceAccountEntries(accountId, entryIds)
    } catch (error: any) {
      appStore.showWarning(t('admin.accounts.catalogEntries.bindFailed', {
        message: error?.response?.data?.message || error?.message || ''
      }))
    }
  }
}

// 所有单个建号都走这里：第三方 key 不带平台（后端按地址认厂商，认不出的中转按协议归族），
// 映射打「只改名」标记，建好后写入承接的模型。
const createAccountRecord = async (payload: CreateAccountRequest): Promise<Account> => {
  const body: CreateAccountRequest = { ...payload, credentials: withRenameOnlyMapping(payload.credentials) }
  if (body.type === 'apikey') delete body.platform
  const account = await adminAPI.accounts.create(body)
  await bindSelectedCatalogEntries([account.id])
  return account
}

const submitCreateAccount = async (payload: CreateAccountRequest) => {
  submitting.value = true
  try {
    const account = await createAccountRecord(payload)
    const modelMapping = payload.credentials.model_mapping
    const hasConcreteMappedTarget = payload.type === 'apikey' &&
      typeof modelMapping === 'object' &&
      modelMapping !== null &&
      Object.values(modelMapping).some((target) =>
        typeof target === 'string' && target.trim() !== '' && !target.includes('*')
      )
    if (hasConcreteMappedTarget) {
      try {
        const result = await adminAPI.accounts.syncUpstreamModels(account.id)
        const warnings = result.warnings ?? []
        if (warnings.some(warning => warning.code === 'upstream_model_metadata_incomplete')) {
          appStore.showWarning(t('admin.accounts.syncUpstreamModelsMetadataIncomplete'))
        } else if (warnings.some(warning => warning.code === 'upstream_model_metadata_partial')) {
          appStore.showWarning(t('admin.accounts.syncUpstreamModelsMetadataPartial'))
        }
      } catch {
        appStore.showWarning(t('admin.accounts.syncUpstreamModelsFailed'))
      }
    }
    if (
      payload.type === 'apikey' &&
      payload.upstream_billing_probe_enabled === true
    ) {
      try {
        await adminAPI.accounts.probeUpstreamBilling(account.id)
      } catch {
        appStore.showWarning(t('admin.accounts.upstreamBilling.probeFailed'))
      }
    }
    appStore.showSuccess(t('admin.accounts.accountCreated'))
    emit('created')
    handleClose()
  } catch (error: any) {
    appStore.showError(error.response?.data?.message || error.response?.data?.detail || t('admin.accounts.failedToCreate'))
  } finally {
    submitting.value = false
  }
}

// Methods
const resetForm = () => {
  step.value = 1
  form.name = ''
  form.notes = ''
  form.platform = 'anthropic'
  form.type = 'oauth'
  form.credentials = {}
  form.proxy_id = null
  form.concurrency = 10
  form.load_factor = null
  form.priority = 1
  form.rate_multiplier = 1
  form.expires_at = null
  accountCategory.value = 'oauth-based'
  addMethod.value = 'oauth'
  keyPlanMode.value = 'payg'
  protocolEndpoints.value = {}
  zhipuOrganization.value = ''
  zhipuProject.value = ''
  apiKeyValue.value = ''
  upstreamRequestIdHeader.value = ''
  upstreamBillingAutoProbeEnabled.value = true
  editQuotaLimit.value = null
  editQuotaDailyLimit.value = null
  editQuotaWeeklyLimit.value = null
  editDailyResetMode.value = null
  editDailyResetHour.value = null
  editWeeklyResetMode.value = null
  editWeeklyResetDay.value = null
  editWeeklyResetHour.value = null
  editResetTimezone.value = null
  modelMappings.value = []
  openAICompactModelMappings.value = []
  accessSourceId.value = DEFAULT_ACCESS_SOURCE_ID
  selectedCatalogEntryIds.value = []
  catalogSelectionTouched.value = false
  showMoreSettings.value = false
  poolModeEnabled.value = false
  poolModeRetryCount.value = DEFAULT_POOL_MODE_RETRY_COUNT
  poolModeRetryStatusCodesInput.value = ''
  customErrorCodesEnabled.value = false
  selectedErrorCodes.value = []
  customErrorCodeInput.value = null
  headerOverrideEnabled.value = false
  headerOverrideRows.value = []
  openAIImagesUrlToB64JsonEnabled.value = false
  interceptWarmupRequests.value = false
  autoPauseOnExpired.value = true
  openaiPassthroughEnabled.value = false
  openaiFlattenNamespacesEnabled.value = false
  openAICompactMode.value = 'auto'
  openAIEndpointCapabilities.value = ['chat_completions', 'embeddings']
  openaiOAuthResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
  openaiAPIKeyResponsesWebSocketV2Mode.value = OPENAI_WS_MODE_OFF
  codexCLIOnlyEnabled.value = false
  codexCLIOnlyAppServerEnabled.value = false
  codexFingerprintMode.value = 'off'
  anthropicAPIKeyAuthScheme.value = 'x_api_key'
  bedrockCCCompatEnabled.value = false
  // Reset quota control state
  sessionLimitEnabled.value = false
  maxSessions.value = null
  rpmLimitEnabled.value = false
  baseRpm.value = null
  allowOverages.value = false
  antigravityProjectId.value = ''
  vertexServiceAccountJson.value = ''
  vertexLocation.value = 'global'
  tempUnschedEnabled.value = false
  tempUnschedRules.value = []
  geminiOAuthType.value = 'code_assist'
  geminiTierGoogleOne.value = 'google_one_free'
  geminiTierGcp.value = 'gcp_standard'
  geminiTierAIStudio.value = 'aistudio_free'
  oauth.resetState()
  openaiOAuth.resetState()
  geminiOAuth.resetState()
  antigravityOAuth.resetState()
  grokOAuth.resetState()
  oauthFlowRef.value?.reset()
}

const handleClose = () => {
  emit('close')
}

const buildOpenAIExtra = (base?: Record<string, unknown>): Record<string, unknown> | undefined => {
  const responsesSettingsVisible = openAIResponsesSettingsVisible.value
  const openaiPlatform = form.platform === 'openai'
  if (!responsesSettingsVisible && !openaiPlatform) {
    return base
  }

  const extra: Record<string, unknown> = { ...(base || {}) }
  // OpenAI Responses 协议设置（自动透传 / WS mode / Compact）：只在区块可见时写入
  if (responsesSettingsVisible) {
    if (form.type === 'apikey') {
      extra.openai_apikey_responses_websockets_v2_mode = openaiAPIKeyResponsesWebSocketV2Mode.value
      extra.openai_apikey_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiAPIKeyResponsesWebSocketV2Mode.value)
    } else {
      extra.openai_oauth_responses_websockets_v2_mode = openaiOAuthResponsesWebSocketV2Mode.value
      extra.openai_oauth_responses_websockets_v2_enabled = isOpenAIWSModeEnabled(openaiOAuthResponsesWebSocketV2Mode.value)
    }
    // 清理兼容旧键，统一改用分类型开关。
    delete extra.responses_websockets_v2_enabled
    delete extra.openai_ws_enabled
    if (openaiPassthroughEnabled.value) {
      extra.openai_passthrough = true
    } else {
      delete extra.openai_passthrough
      delete extra.openai_oauth_passthrough
    }
    if (openAICompactMode.value !== 'auto') {
      extra.openai_compact_mode = openAICompactMode.value
    } else {
      delete extra.openai_compact_mode
    }
  }
  // 生图结果转 base64 与端点能力同区块：第三方 key 按协议地址判定，不看平台标签，
  // 必须在 openai 平台专属段落之前写入。
  if (openAIKeySettingsVisible.value && openAIImagesUrlToB64JsonEnabled.value) {
    extra.images_url_to_b64_json = true
  } else {
    delete extra.images_url_to_b64_json
  }
  if (!openaiPlatform) {
    return Object.keys(extra).length > 0 ? extra : undefined
  }

  // 以下是 OpenAI 成品号专属设置
  // 缺省即保留 namespace，不写空值，避免 extra 里堆积默认项
  if (form.type === 'oauth' && openaiFlattenNamespacesEnabled.value) {
    extra.openai_responses_flatten_namespaces = true
  } else {
    delete extra.openai_responses_flatten_namespaces
  }

  if (accountCategory.value === 'oauth-based' && codexCLIOnlyEnabled.value) {
    extra.codex_cli_only = true
  } else {
    delete extra.codex_cli_only
  }
  delete extra.codex_cli_only_allowed_clients
  if (
    accountCategory.value === 'oauth-based' &&
    codexCLIOnlyEnabled.value &&
    codexCLIOnlyAppServerEnabled.value
  ) {
    extra.codex_cli_only_allow_app_server = true
  } else {
    delete extra.codex_cli_only_allow_app_server
  }
  // 收敛是显式 opt-in：off 即默认值，不落键；device/session/full 必须显式写入，
  // 否则管理员的选择会被当成默认而丢失（#5610）。区块只对 OpenAI 成品号露出，
  // 换成 key 后不写残留值。
  if (accountCategory.value === 'oauth-based' && codexFingerprintMode.value !== 'off') {
    extra.codex_fingerprint_mode = codexFingerprintMode.value
  } else {
    delete extra.codex_fingerprint_mode
  }

  return Object.keys(extra).length > 0 ? extra : undefined
}

const buildOpenAICodexImportExtra = (): Record<string, unknown> | undefined => {
  const extra = buildOpenAIExtra()
  if (!extra) {
    return undefined
  }
  return Object.keys(extra).length > 0 ? extra : undefined
}

const buildAnthropicExtra = (base?: Record<string, unknown>): Record<string, unknown> | undefined => {
  if (!anthropicKeySettingsVisible.value) {
    return base
  }

  const extra: Record<string, unknown> = { ...(base || {}) }
  if (anthropicAPIKeyAuthScheme.value === 'authorization_bearer') {
    extra.anthropic_apikey_auth_scheme = 'authorization_bearer'
  } else {
    delete extra.anthropic_apikey_auth_scheme
  }
  if (bedrockCCCompatEnabled.value) {
    extra.bedrock_cc_compat = true
  } else {
    delete extra.bedrock_cc_compat
  }

  return Object.keys(extra).length > 0 ? extra : undefined
}

const doCreateAccount = async (payload: CreateAccountRequest) => {
  await submitCreateAccount(payload)
}

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

// Service Account JSON 原样交给后端，Project ID / Client Email 由后端从 JSON 里取（不另存副本）；
// 这里只校验三个必需字段在不在。
const applyVertexServiceAccountJson = (value: string) => {
  const raw = value.trim()
  if (!raw) {
    return false
  }
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const projectId = typeof parsed.project_id === 'string' ? parsed.project_id.trim() : ''
    const clientEmail = typeof parsed.client_email === 'string' ? parsed.client_email.trim() : ''
    const privateKey = typeof parsed.private_key === 'string' ? parsed.private_key.trim() : ''
    if (!projectId || !clientEmail || !privateKey) {
      appStore.showError(t('admin.accounts.vertexSaJsonMissingFields'))
      return false
    }
    vertexServiceAccountJson.value = JSON.stringify(parsed)
    return true
  } catch {
    appStore.showError(t('admin.accounts.vertexSaJsonInvalid'))
    return false
  }
}

const parseVertexServiceAccountJson = () => applyVertexServiceAccountJson(vertexServiceAccountJson.value)

const handleVertexServiceAccountFile = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    applyVertexServiceAccountJson(await file.text())
  } finally {
    input.value = ''
  }
}

const handleVertexServiceAccountDrop = async (event: DragEvent) => {
  vertexServiceAccountDragActive.value = false
  const file = event.dataTransfer?.files?.[0]
  if (!file) return
  applyVertexServiceAccountJson(await file.text())
}

const handleSubmit = async () => {
  // For OAuth-based type, handle OAuth flow (goes to step 2)
  if (isOAuthFlow.value) {
    if (!isGrokSSOInputMethod.value && !form.name.trim()) {
      appStore.showError(t('admin.accounts.pleaseEnterAccountName'))
      return
    }
    step.value = 2
    return
  }

  // For Bedrock type, create directly
  if (form.platform === 'anthropic' && accountCategory.value === 'bedrock') {
    if (!form.name.trim()) {
      appStore.showError(t('admin.accounts.pleaseEnterAccountName'))
      return
    }

    const credentials: Record<string, unknown> = {
      auth_mode: bedrockAuthMode.value,
      aws_region: bedrockRegion.value.trim() || 'us-east-1',
    }

    if (bedrockAuthMode.value === 'sigv4') {
      if (!bedrockAccessKeyId.value.trim()) {
        appStore.showError(t('admin.accounts.bedrockAccessKeyIdRequired'))
        return
      }
      if (!bedrockSecretAccessKey.value.trim()) {
        appStore.showError(t('admin.accounts.bedrockSecretAccessKeyRequired'))
        return
      }
      credentials.aws_access_key_id = bedrockAccessKeyId.value.trim()
      credentials.aws_secret_access_key = bedrockSecretAccessKey.value.trim()
    } else {
      if (!bedrockApiKeyValue.value.trim()) {
        appStore.showError(t('admin.accounts.bedrockApiKeyRequired'))
        return
      }
      credentials.api_key = bedrockApiKeyValue.value.trim()
    }

    if (bedrockForceGlobal.value) {
      credentials.aws_force_global = 'true'
    }

    // Model mapping
    const modelMapping = buildRenameMapping()
    if (modelMapping) {
      credentials.model_mapping = modelMapping
    }

    applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')

    await createAccountAndFinish('anthropic', 'bedrock' as AccountType, credentials)
    return
  }

  if ((form.platform === 'gemini' || form.platform === 'anthropic') && accountCategory.value === 'service_account') {
    if (!form.name.trim()) {
      appStore.showError(t('admin.accounts.pleaseEnterAccountName'))
      return
    }
    if (!parseVertexServiceAccountJson()) {
      return
    }
    if (!vertexLocation.value.trim()) {
      appStore.showError(t('admin.accounts.vertexLocationRequired'))
      return
    }
    const credentials: Record<string, unknown> = {
      service_account_json: vertexServiceAccountJson.value.trim(),
      location: vertexLocation.value.trim(),
      tier_id: 'vertex'
    }
    await createAccountAndFinish(form.platform, 'service_account' as AccountType, credentials)
    return
  }

  // For apikey type, create directly
  if (!apiKeyValue.value.trim()) {
    appStore.showError(t('admin.accounts.pleaseEnterApiKey'))
    return
  }

  // 第三方 key 的上游地址只认协议映射，不在这里补任何默认地址。
  const apiKeyEndpoints = validatedProtocolEndpoints()
  if (!apiKeyEndpoints) {
    return
  }

  // Build credentials with optional model mapping
  const credentials: Record<string, unknown> = {
    api_key: apiKeyValue.value.trim()
  }

  // 国产厂商 / OpenCode：账号模式写入凭据，后端按 account_mode 路由额度 / 余额探测；
  // 厂商按地址识别，中转不写。转发协议由协议地址决定。
  if (keyAccountMode.value) {
    credentials.account_mode = keyAccountMode.value
    // 智谱团队版 Coding Plan：组织/项目 ID 写入凭据（非空才写）
    if (keyVendor.value === 'zhipu' && keyPlanMode.value === 'coding') {
      if (zhipuOrganization.value.trim()) credentials.zhipu_organization = zhipuOrganization.value.trim()
      if (zhipuProject.value.trim()) credentials.zhipu_project = zhipuProject.value.trim()
    }
  }

  // Add model mapping if configured（OpenAI 开启自动透传时不应用）
  if (!isOpenAIModelRestrictionDisabled.value) {
    const modelMapping = buildRenameMapping()
    if (modelMapping) {
      credentials.model_mapping = modelMapping
    }
  }
  if (openAIKeySettingsVisible.value) {
    applyOpenAIEndpointCapabilities(credentials)
  }
  applyKeyCompactModelMapping(credentials)

  // Add pool mode if enabled
  if (poolModeEnabled.value) {
    credentials.pool_mode = true
    credentials.pool_mode_retry_count = normalizePoolModeRetryCount(poolModeRetryCount.value)
    const parsedRetryStatusCodes = parsePoolModeRetryStatusCodes(poolModeRetryStatusCodesInput.value)
    if (parsedRetryStatusCodes.length > 0) {
      credentials.pool_mode_retry_status_codes = parsedRetryStatusCodes
    }
  }

  // Add custom error codes if enabled
  if (customErrorCodesEnabled.value) {
    credentials.custom_error_codes_enabled = true
    credentials.custom_error_codes = [...selectedErrorCodes.value]
  }

  // 请求头覆写对任何第三方 key 开放
  if (!applyKeyHeaderOverride(credentials)) {
    return
  }

  applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')
  if (!applyTempUnschedConfig(credentials)) {
    return
  }

  form.credentials = credentials
  const extra = buildAnthropicExtra(buildOpenAIExtra())

  await doCreateAccount({
    ...form,
    protocol_endpoints: apiKeyEndpoints,
    extra: withQuotaExtra(withUpstreamRequestIdHeader(extra)),
    upstream_billing_probe_enabled: upstreamBillingAutoProbeEnabled.value,
    auto_pause_on_expired: autoPauseOnExpired.value
  })
}

const goBackToBasicInfo = () => {
  step.value = 1
  oauth.resetState()
  openaiOAuth.resetState()
  geminiOAuth.resetState()
  antigravityOAuth.resetState()
  grokOAuth.resetState()
  oauthFlowRef.value?.reset()
}

const handleGenerateUrl = async () => {
  if (form.platform === 'openai') {
    await openaiOAuth.generateAuthUrl(form.proxy_id)
  } else if (form.platform === 'gemini') {
    await geminiOAuth.generateAuthUrl(
      form.proxy_id,
      oauthFlowRef.value?.projectId,
      geminiOAuthType.value,
      geminiSelectedTier.value
    )
  } else if (form.platform === 'antigravity') {
    await antigravityOAuth.generateAuthUrl(form.proxy_id)
  } else if (form.platform === 'grok') {
    await grokOAuth.generateAuthUrl(form.proxy_id)
  } else {
    await oauth.generateAuthUrl(addMethod.value, form.proxy_id)
  }
}

const handleValidateRefreshToken = (rt: string) => {
  if (form.platform === 'openai') {
    handleOpenAIValidateRT(rt)
  } else if (form.platform === 'antigravity') {
    handleAntigravityValidateRT(rt)
  } else if (form.platform === 'grok') {
    handleGrokValidateRT(rt)
  }
}

const handleValidateSessionToken = (_sessionToken: string) => {
  // Session token validation removed
}

const formatDateTimeLocal = formatDateTimeLocalInput
const parseDateTimeLocal = parseDateTimeLocalInput

// 限额（日 / 周 / 总）、重置方式与提醒写进 extra。第三方 key 的提交分支和 Bedrock / Vertex 共用，
// 任何一条漏调，界面上填的限额就会被静默丢掉。
const withQuotaExtra = (extra?: Record<string, unknown>): Record<string, unknown> | undefined => {
  const quotaExtra: Record<string, unknown> = { ...(extra || {}) }
  if (editQuotaLimit.value != null && editQuotaLimit.value > 0) {
    quotaExtra.quota_limit = editQuotaLimit.value
  }
  if (editQuotaDailyLimit.value != null && editQuotaDailyLimit.value > 0) {
    quotaExtra.quota_daily_limit = editQuotaDailyLimit.value
  }
  if (editQuotaWeeklyLimit.value != null && editQuotaWeeklyLimit.value > 0) {
    quotaExtra.quota_weekly_limit = editQuotaWeeklyLimit.value
  }
  // Quota reset mode config
  if (editDailyResetMode.value === 'fixed') {
    quotaExtra.quota_daily_reset_mode = 'fixed'
    quotaExtra.quota_daily_reset_hour = editDailyResetHour.value ?? 0
  }
  if (editWeeklyResetMode.value === 'fixed') {
    quotaExtra.quota_weekly_reset_mode = 'fixed'
    quotaExtra.quota_weekly_reset_day = editWeeklyResetDay.value ?? 1
    quotaExtra.quota_weekly_reset_hour = editWeeklyResetHour.value ?? 0
  }
  if (editDailyResetMode.value === 'fixed' || editWeeklyResetMode.value === 'fixed') {
    quotaExtra.quota_reset_timezone = editResetTimezone.value || 'UTC'
  }
  // Quota notify config
  writeQuotaNotifyToExtra(quotaExtra, 'create')
  return Object.keys(quotaExtra).length > 0 ? quotaExtra : extra
}

// Create account and handle success/failure
const createAccountAndFinish = async (
  platform: AccountPlatform,
  type: AccountType,
  credentials: Record<string, unknown>,
  extra?: Record<string, unknown>,
  protocolEndpointsForKey?: ProtocolEndpoints
) => {
  if (!applyTempUnschedConfig(credentials)) {
    return
  }
  // Inject quota limits for apikey/bedrock accounts
  let finalExtra = withUpstreamRequestIdHeader(extra)
  if (type === 'apikey' || type === 'bedrock') {
    finalExtra = withQuotaExtra(finalExtra)
  }
  // 端点能力按协议地址判定，不看平台标签。
  if (openAIKeySettingsVisible.value) {
    applyOpenAIEndpointCapabilities(credentials)
  }
  if (platform === 'openai') {
    const compactModelMapping = buildOpenAICompactModelMapping()
    if (compactModelMapping) {
      credentials.compact_model_mapping = compactModelMapping
    } else {
      delete credentials.compact_model_mapping
    }
  }
  if (platform === 'grok') {
    const modelMapping = buildRenameMapping()
    if (modelMapping) {
      credentials.model_mapping = modelMapping
    } else {
      delete credentials.model_mapping
    }
  }
  await doCreateAccount({
    name: form.name,
    notes: form.notes,
    platform,
    type,
    credentials,
    protocol_endpoints: protocolEndpointsForKey,
    extra: finalExtra,
    proxy_id: form.proxy_id,
    concurrency: form.concurrency,
    load_factor: form.load_factor ?? undefined,
    priority: form.priority,
    rate_multiplier: form.rate_multiplier,
    expires_at: form.expires_at,
    // 上游倍率探测对全部 API-key 平台开放（antigravity upstream 走本 helper）；
    // 非 apikey 类型（bedrock/oauth）不传，后端不动作。
    upstream_billing_probe_enabled: type === 'apikey' ? upstreamBillingAutoProbeEnabled.value : undefined,
    auto_pause_on_expired: autoPauseOnExpired.value
  })
}

// Grok 手动 RT 批量验证和创建
const handleGrokValidateRT = async (refreshTokenInput: string) => {
  if (!refreshTokenInput.trim()) return

  const refreshTokens = refreshTokenInput
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    grokOAuth.error.value = t('admin.accounts.oauth.grok.pleaseEnterRefreshToken')
    return
  }
  if (!validateHeaderOverrideForm()) return

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const tokenInfo = await grokOAuth.validateRefreshToken(refreshTokens[i], form.proxy_id)
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${grokOAuth.error.value || 'Validation failed'}`)
          grokOAuth.error.value = ''
          continue
        }

        const credentials = grokOAuth.buildCredentials(tokenInfo)
        applyGrokOAuthUpstreamConfig(credentials)
        const extra = grokOAuth.buildExtraInfo(tokenInfo)
        const accountName = refreshTokens.length > 1 ? `${form.name || tokenInfo.email || 'Grok OAuth Account'} #${i + 1}` : (form.name || tokenInfo.email || 'Grok OAuth Account')

        const modelMapping = buildRenameMapping()
        if (modelMapping) {
          credentials.model_mapping = modelMapping
        }
        if (!applyTempUnschedConfig(credentials)) {
          return
        }

        await createAccountRecord({
          name: accountName,
          notes: form.notes,
          platform: 'grok',
          type: 'oauth',
          credentials,
          extra: withUpstreamRequestIdHeader(extra),
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0) {
      appStore.showWarning(t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount }))
      grokOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    grokOAuth.loading.value = false
  }
}

const handleGrokImportSSO = async (ssoInput: string) => {
  // Align with OpenAI/Grok RT batch import: one token per line, no client-side dedupe.
  const ssoTokens = ssoInput
    .split('\n')
    .map((token) => token.trim())
    .filter((token) => token)
  if (ssoTokens.length === 0) return
  if (!validateHeaderOverrideForm()) return

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  const credentials: Record<string, unknown> = {}
  applyGrokOAuthUpstreamConfig(credentials)
  const modelMapping = buildRenameMapping()
  if (modelMapping) {
    credentials.model_mapping = modelMapping
  }
  if (!applyTempUnschedConfig(credentials)) {
    grokOAuth.loading.value = false
    return
  }

  try {
    const result = await adminAPI.grok.createFromSSO({
      sso_tokens: ssoTokens,
      name: form.name || undefined,
      notes: form.notes || undefined,
      proxy_id: form.proxy_id,
      credentials: withRenameOnlyMapping(credentials),
      concurrency: form.concurrency,
      load_factor: form.load_factor ?? undefined,
      priority: form.priority,
      rate_multiplier: form.rate_multiplier,
      expires_at: form.expires_at,
      auto_pause_on_expired: autoPauseOnExpired.value
    })
    await bindSelectedCatalogEntries(
      (result.created ?? []).flatMap((item) => (item.account ? [item.account.id] : []))
    )

    const successCount = result.created?.length || 0
    const failedCount = result.failed?.length || 0
    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        ssoTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      // Same as OpenAI/Grok RT: keep input, show failures, refresh list.
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      grokOAuth.error.value = (result.failed || [])
        .map((item) => `#${item.index}: ${item.error || 'Unknown error'}`)
        .join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = (result.failed || [])
        .map((item) => `#${item.index}: ${item.error || 'Unknown error'}`)
        .join('\n') || t('admin.accounts.oauth.grok.failedToConvertSSO')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } catch (error: any) {
    grokOAuth.error.value = error.response?.data?.detail || error.message || t('admin.accounts.oauth.grok.failedToConvertSSO')
    appStore.showError(grokOAuth.error.value)
  } finally {
    grokOAuth.loading.value = false
  }
}

/**
 * Grok password login: each line is email----password.
 * Password is only used for the authorize API call; buildCredentials never stores it.
 */
const handleGrokAuthorizePassword = async (emailPasswordInput: string) => {
  if (!emailPasswordInput.trim()) return
  if (!validateHeaderOverrideForm()) return

  const lines = emailPasswordInput
    .split('\n')
    // Keep the password portion byte-for-byte; trim is only for determining
    // whether this textarea line is blank.
    .filter((line) => line.trim() && line.includes('----'))

  if (lines.length === 0) {
    grokOAuth.error.value = t(
      'admin.accounts.oauth.grok.pleaseEnterPassword',
      'Please enter email----password (one per line)'
    )
    return
  }

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < lines.length; i++) {
      try {
        const tokenInfo = await grokOAuth.authorizePassword(lines[i], form.proxy_id)
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${grokOAuth.error.value || 'Authorization failed'}`)
          grokOAuth.error.value = ''
          continue
        }

        const credentials = grokOAuth.buildCredentials(tokenInfo)
        applyGrokOAuthUpstreamConfig(credentials)
        const extra = grokOAuth.buildExtraInfo(tokenInfo)
        const accountName =
          lines.length > 1
            ? `${form.name || tokenInfo.email || 'Grok OAuth Account'} #${i + 1}`
            : form.name || tokenInfo.email || 'Grok OAuth Account'

        const modelMapping = buildRenameMapping()
        if (modelMapping) {
          credentials.model_mapping = modelMapping
        }
        if (!applyTempUnschedConfig(credentials)) {
          return
        }

        await createAccountRecord({
          name: accountName,
          notes: form.notes,
          platform: 'grok',
          type: 'oauth',
          credentials,
          extra: withUpstreamRequestIdHeader(extra),
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        lines.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', {
          success: successCount,
          failed: failedCount
        })
      )
      grokOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    grokOAuth.loading.value = false
  }
}

// OpenAI OAuth 授权码兑换
const handleOpenAIExchange = async (authCode: string) => {
  const oauthClient = openaiOAuth
  if (!authCode.trim() || !oauthClient.sessionId.value) return

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  try {
    const stateToUse = (oauthFlowRef.value?.oauthState || oauthClient.oauthState.value || '').trim()
    if (!stateToUse) {
      oauthClient.error.value = t('admin.accounts.oauth.authFailed')
      appStore.showError(oauthClient.error.value)
      return
    }

    const tokenInfo = await oauthClient.exchangeAuthCode(
      authCode.trim(),
      oauthClient.sessionId.value,
      stateToUse,
      form.proxy_id
    )
    if (!tokenInfo) return

    const credentials = oauthClient.buildCredentials(tokenInfo)
    const oauthExtra = oauthClient.buildExtraInfo(tokenInfo) as Record<string, unknown> | undefined
    const extra = buildOpenAIExtra(oauthExtra)
    const shouldCreateOpenAI = form.platform === 'openai'

    // Add model mapping for OpenAI OAuth accounts（透传模式下不应用）
    if (shouldCreateOpenAI && !isOpenAIModelRestrictionDisabled.value) {
      const modelMapping = buildRenameMapping()
      if (modelMapping) {
        credentials.model_mapping = modelMapping
      }
    }
    if (shouldCreateOpenAI) {
      const compactModelMapping = buildOpenAICompactModelMapping()
      if (compactModelMapping) {
        credentials.compact_model_mapping = compactModelMapping
      }
    }

    // 应用临时不可调度配置
    if (!applyTempUnschedConfig(credentials)) {
      return
    }

    if (shouldCreateOpenAI) {
      await createAccountRecord({
        name: form.name,
        notes: form.notes,
        platform: 'openai',
        type: 'oauth',
        credentials,
        extra: withUpstreamRequestIdHeader(extra),
        proxy_id: form.proxy_id,
        concurrency: form.concurrency,
        load_factor: form.load_factor ?? undefined,
        priority: form.priority,
        rate_multiplier: form.rate_multiplier,
        expires_at: form.expires_at,
        auto_pause_on_expired: autoPauseOnExpired.value
      })
      appStore.showSuccess(t('admin.accounts.accountCreated'))
    }

    emit('created')
    handleClose()
  } catch (error: any) {
    oauthClient.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(oauthClient.error.value)
  } finally {
    oauthClient.loading.value = false
  }
}

// OpenAI 手动 RT 批量验证和创建
// OpenAI Mobile RT client_id
const OPENAI_MOBILE_RT_CLIENT_ID = 'app_LlGpXReQgckcGGUo2JrYvtJK'

const buildOpenAICodexImportCredentialExtras = (): Record<string, unknown> | null => {
  const credentials: Record<string, unknown> = {}
  if (!isOpenAIModelRestrictionDisabled.value) {
    const modelMapping = buildRenameMapping()
    if (modelMapping) {
      credentials.model_mapping = modelMapping
    }
  }

  const compactModelMapping = buildOpenAICompactModelMapping()
  if (compactModelMapping) {
    credentials.compact_model_mapping = compactModelMapping
  }

  if (!applyTempUnschedConfig(credentials)) {
    return null
  }
  return credentials
}

const formatCodexImportMessages = (messages?: CodexSessionImportMessage[]) => {
  return (messages || [])
    .map((item) => {
      const name = item.name ? ` ${item.name}` : ''
      return `#${item.index}${name}: ${item.message}`
    })
    .join('\n')
}

const isAgentIdentityImportContent = (content: string) => {
  const isAgentIdentityValue = (value: unknown): boolean => {
    if (Array.isArray(value)) return value.length > 0 && value.every(isAgentIdentityValue)
    if (!value || typeof value !== 'object') return false
    const record = value as Record<string, unknown>
    const authMode = record.auth_mode ?? record.authMode
    const agentIdentity = record.agent_identity ?? record.agentIdentity
    return (typeof authMode === 'string' && authMode.toLowerCase() === 'agentidentity')
      || (!!agentIdentity && typeof agentIdentity === 'object')
  }

  try {
    return isAgentIdentityValue(JSON.parse(content))
  } catch {
    const lines = content.split('\n').map((line) => line.trim()).filter(Boolean)
    if (lines.length === 0) return false
    try {
      return lines.every((line) => isAgentIdentityValue(JSON.parse(line)))
    } catch {
      return false
    }
  }
}

const handleOpenAIImportCodexSession = async (content: string) => {
  const oauthClient = openaiOAuth
  const trimmed = content.trim()
  if (!trimmed) {
    oauthClient.error.value = t('admin.accounts.oauth.openai.codexSessionEmpty')
    return
  }
  if (oauthFlowRef.value?.inputMethod === 'agent_identity' && !isAgentIdentityImportContent(trimmed)) {
    oauthClient.error.value = t('admin.accounts.oauth.openai.agentIdentityInvalid')
    return
  }

  const credentialExtras = buildOpenAICodexImportCredentialExtras()
  if (credentialExtras === null) {
    return
  }

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  try {
    const extra = buildOpenAICodexImportExtra()
    const result = await adminAPI.accounts.importCodexSession({
      content: trimmed,
      name: form.name,
      notes: form.notes || null,
      proxy_id: form.proxy_id,
      concurrency: form.concurrency,
      load_factor: form.load_factor ?? undefined,
      priority: form.priority,
      rate_multiplier: form.rate_multiplier,
      expires_at: form.expires_at,
      auto_pause_on_expired: autoPauseOnExpired.value,
      credential_extras: Object.keys(credentialExtras).length > 0 ? credentialExtras : undefined,
      extra: withUpstreamRequestIdHeader(extra),
      update_existing: true
    })

    const successCount = result.created + result.updated
    const params = {
      created: result.created,
      updated: result.updated,
      skipped: result.skipped,
      failed: result.failed
    }

    if (successCount > 0 && result.failed === 0) {
      appStore.showSuccess(t('admin.accounts.oauth.openai.codexSessionImportSuccess', params))
      emit('created')
      handleClose()
      return
    }

    const errorText = formatCodexImportMessages(result.errors)
    const warningText = formatCodexImportMessages(result.warnings)
    oauthClient.error.value = [errorText, warningText].filter(Boolean).join('\n')

    if (result.failed === 0) {
      appStore.showWarning(t('admin.accounts.oauth.openai.codexSessionImportSuccess', params))
      return
    }

    if (successCount > 0) {
      appStore.showWarning(t('admin.accounts.oauth.openai.codexSessionImportPartial', params))
      emit('created')
      return
    }

    appStore.showError(t('admin.accounts.oauth.openai.codexSessionImportFailed'))
  } catch (error: any) {
    oauthClient.error.value =
      error.response?.data?.detail ||
      error.response?.data?.message ||
      error.message ||
      t('admin.accounts.oauth.openai.codexSessionImportFailed')
    appStore.showError(oauthClient.error.value)
  } finally {
    oauthClient.loading.value = false
  }
}

const handleOpenAIImportCodexPAT = async (accessToken: string) => {
  const oauthClient = openaiOAuth
  const trimmed = accessToken.trim()
  if (!trimmed) {
    oauthClient.error.value = t('admin.accounts.oauth.openai.codexPatEmpty')
    return
  }

  const credentialExtras = buildOpenAICodexImportCredentialExtras()
  if (credentialExtras === null) {
    return
  }

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  try {
    const extra = buildOpenAICodexImportExtra()
    await adminAPI.accounts.createOpenAICodexPAT({
      access_token: trimmed,
      name: form.name,
      notes: form.notes || null,
      proxy_id: form.proxy_id,
      concurrency: form.concurrency,
      load_factor: form.load_factor ?? undefined,
      priority: form.priority,
      rate_multiplier: form.rate_multiplier,
      expires_at: form.expires_at,
      auto_pause_on_expired: autoPauseOnExpired.value,
      credential_extras: Object.keys(credentialExtras).length > 0 ? credentialExtras : undefined,
      extra: withUpstreamRequestIdHeader(extra)
    })

    appStore.showSuccess(t('admin.accounts.accountCreated'))
    emit('created')
    handleClose()
  } catch (error: any) {
    oauthClient.error.value =
      error.response?.data?.detail ||
      error.response?.data?.message ||
      error.message ||
      t('admin.accounts.oauth.openai.codexPatImportFailed')
    appStore.showError(oauthClient.error.value)
  } finally {
    oauthClient.loading.value = false
  }
}

// OpenAI RT 批量验证和创建（共享逻辑）
const handleOpenAIBatchRT = async (refreshTokenInput: string, clientId?: string) => {
  const oauthClient = openaiOAuth
  if (!refreshTokenInput.trim()) return

  const refreshTokens = refreshTokenInput
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    oauthClient.error.value = t('admin.accounts.oauth.openai.pleaseEnterRefreshToken')
    return
  }

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []
  const shouldCreateOpenAI = form.platform === 'openai'

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const tokenInfo = await oauthClient.validateRefreshToken(
          refreshTokens[i],
          form.proxy_id,
          clientId
        )
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${oauthClient.error.value || 'Validation failed'}`)
          oauthClient.error.value = ''
          continue
        }

        const credentials = oauthClient.buildCredentials(tokenInfo)
        if (clientId) {
          credentials.client_id = clientId
        }
        const oauthExtra = oauthClient.buildExtraInfo(tokenInfo) as Record<string, unknown> | undefined
        const extra = buildOpenAIExtra(oauthExtra)

        // Add model mapping for OpenAI OAuth accounts（透传模式下不应用）
        if (shouldCreateOpenAI && !isOpenAIModelRestrictionDisabled.value) {
          const modelMapping = buildRenameMapping()
          if (modelMapping) {
            credentials.model_mapping = modelMapping
          }
        }
        if (shouldCreateOpenAI) {
          const compactModelMapping = buildOpenAICompactModelMapping()
          if (compactModelMapping) {
            credentials.compact_model_mapping = compactModelMapping
          }
        }

        // Generate account name; fallback to email if name is empty (ent schema requires NotEmpty)
        const baseName = form.name || tokenInfo.email || 'OpenAI OAuth Account'
        const accountName = refreshTokens.length > 1 ? `${baseName} #${i + 1}` : baseName

        if (shouldCreateOpenAI) {
          await createAccountRecord({
            name: accountName,
            notes: form.notes,
            platform: 'openai',
            type: 'oauth',
            credentials,
            extra: withUpstreamRequestIdHeader(extra),
            proxy_id: form.proxy_id,
            concurrency: form.concurrency,
            load_factor: form.load_factor ?? undefined,
            priority: form.priority,
            rate_multiplier: form.rate_multiplier,
            expires_at: form.expires_at,
            auto_pause_on_expired: autoPauseOnExpired.value
          })
        }

        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    // Show results
    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      oauthClient.error.value = errors.join('\n')
      emit('created')
    } else {
      oauthClient.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    oauthClient.loading.value = false
  }
}

// 手动输入 RT（Codex CLI client_id，默认）
const handleOpenAIValidateRT = (rt: string) => handleOpenAIBatchRT(rt)

// 手动输入 Mobile RT
const handleOpenAIValidateMobileRT = (rt: string) => handleOpenAIBatchRT(rt, OPENAI_MOBILE_RT_CLIENT_ID)

// Antigravity 手动 RT 批量验证和创建
const handleAntigravityValidateRT = async (refreshTokenInput: string) => {
  if (!refreshTokenInput.trim()) return

  // Parse multiple refresh tokens (one per line)
  const refreshTokens = refreshTokenInput
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt)

  if (refreshTokens.length === 0) {
    antigravityOAuth.error.value = t('admin.accounts.oauth.antigravity.pleaseEnterRefreshToken')
    return
  }

  antigravityOAuth.loading.value = true
  antigravityOAuth.error.value = ''

  let successCount = 0
  let failedCount = 0
  const errors: string[] = []

  try {
    for (let i = 0; i < refreshTokens.length; i++) {
      try {
        const tokenInfo = await antigravityOAuth.validateRefreshToken(
          refreshTokens[i],
          form.proxy_id
        )
        if (!tokenInfo) {
          failedCount++
          errors.push(`#${i + 1}: ${antigravityOAuth.error.value || 'Validation failed'}`)
          antigravityOAuth.error.value = ''
          continue
        }

        const credentials = antigravityOAuth.buildCredentials(tokenInfo, refreshTokens[i])
        applyAntigravityProjectID(credentials, antigravityProjectId.value, 'create')
        
        // Generate account name with index for batch
        const accountName = refreshTokens.length > 1 ? `${form.name} #${i + 1}` : form.name

        // Note: Antigravity doesn't have buildExtraInfo, so we pass empty extra or rely on credentials
        const createPayload: CreateAccountRequest = {
          name: accountName,
          notes: form.notes,
          platform: 'antigravity',
          type: 'oauth',
          credentials,
          extra: withUpstreamRequestIdHeader({}),
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        }
        await createAccountRecord(createPayload)
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = error.response?.data?.detail || error.message || 'Unknown error'
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    // Show results
    if (successCount > 0 && failedCount === 0) {
      appStore.showSuccess(
        refreshTokens.length > 1
          ? t('admin.accounts.oauth.batchSuccess', { count: successCount })
          : t('admin.accounts.accountCreated')
      )
      emit('created')
      handleClose()
    } else if (successCount > 0 && failedCount > 0) {
      appStore.showWarning(
        t('admin.accounts.oauth.batchPartialSuccess', { success: successCount, failed: failedCount })
      )
      antigravityOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      antigravityOAuth.error.value = errors.join('\n')
      appStore.showError(t('admin.accounts.oauth.batchFailed'))
    }
  } finally {
    antigravityOAuth.loading.value = false
  }
}

// Gemini OAuth 授权码兑换
const handleGeminiExchange = async (authCode: string) => {
  if (!authCode.trim() || !geminiOAuth.sessionId.value) return

  geminiOAuth.loading.value = true
  geminiOAuth.error.value = ''

  try {
    const stateFromInput = oauthFlowRef.value?.oauthState || ''
    const stateToUse = stateFromInput || geminiOAuth.state.value
    if (!stateToUse) {
      geminiOAuth.error.value = t('admin.accounts.oauth.authFailed')
      appStore.showError(geminiOAuth.error.value)
      return
    }

    const tokenInfo = await geminiOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId: geminiOAuth.sessionId.value,
      state: stateToUse,
      proxyId: form.proxy_id,
      oauthType: geminiOAuthType.value,
      tierId: geminiSelectedTier.value
    })
    if (!tokenInfo) return

    const credentials = geminiOAuth.buildCredentials(tokenInfo)
    const extra = geminiOAuth.buildExtraInfo(tokenInfo)
    await createAccountAndFinish('gemini', 'oauth', credentials, extra)
  } catch (error: any) {
    geminiOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(geminiOAuth.error.value)
  } finally {
    geminiOAuth.loading.value = false
  }
}

// Antigravity OAuth 授权码兑换
const handleAntigravityExchange = async (authCode: string) => {
  if (!authCode.trim() || !antigravityOAuth.sessionId.value) return

  antigravityOAuth.loading.value = true
  antigravityOAuth.error.value = ''

  try {
    const stateFromInput = oauthFlowRef.value?.oauthState || ''
    const stateToUse = stateFromInput || antigravityOAuth.state.value
    if (!stateToUse) {
      antigravityOAuth.error.value = t('admin.accounts.oauth.authFailed')
      appStore.showError(antigravityOAuth.error.value)
      return
    }

    const tokenInfo = await antigravityOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId: antigravityOAuth.sessionId.value,
      state: stateToUse,
      proxyId: form.proxy_id
    })
		if (!tokenInfo) return

		const credentials = antigravityOAuth.buildCredentials(tokenInfo)
		applyAntigravityProjectID(credentials, antigravityProjectId.value, 'create')
		applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')
		// 改名叠在 Antigravity 默认表之上（后端合并），不填就是默认表
		const antigravityModelMapping = buildRenameMapping()
		if (antigravityModelMapping) {
			credentials.model_mapping = antigravityModelMapping
		}
		const extra = buildAntigravityExtra()
		await createAccountAndFinish('antigravity', 'oauth', credentials, extra)
  } catch (error: any) {
    antigravityOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(antigravityOAuth.error.value)
  } finally {
    antigravityOAuth.loading.value = false
  }
}

// Grok OAuth 授权码兑换
const handleGrokExchange = async (authCode: string) => {
  if (!authCode.trim() || !grokOAuth.sessionId.value) return
  if (!validateHeaderOverrideForm()) return

  grokOAuth.loading.value = true
  grokOAuth.error.value = ''

  try {
    const stateFromInput = oauthFlowRef.value?.oauthState || ''
    const stateToUse = stateFromInput || grokOAuth.state.value
    if (!stateToUse) {
      grokOAuth.error.value = t('admin.accounts.oauth.authFailed')
      appStore.showError(grokOAuth.error.value)
      return
    }

    const tokenInfo = await grokOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId: grokOAuth.sessionId.value,
      state: stateToUse,
      proxyId: form.proxy_id
    })
    if (!tokenInfo) return

    const credentials = grokOAuth.buildCredentials(tokenInfo)
    applyGrokOAuthUpstreamConfig(credentials)
    const extra = grokOAuth.buildExtraInfo(tokenInfo)
    await createAccountAndFinish('grok', 'oauth', credentials, extra)
  } catch (error: any) {
    grokOAuth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(grokOAuth.error.value)
  } finally {
    grokOAuth.loading.value = false
  }
}

// Anthropic OAuth 授权码兑换
const handleAnthropicExchange = async (authCode: string) => {
  if (!authCode.trim() || !oauth.sessionId.value) return

  oauth.loading.value = true
  oauth.error.value = ''

  try {
    const proxyConfig = form.proxy_id ? { proxy_id: form.proxy_id } : {}
    const endpoint =
      addMethod.value === 'oauth'
        ? '/admin/accounts/exchange-code'
        : '/admin/accounts/exchange-setup-token-code'

    const tokenInfo = await adminAPI.accounts.exchangeCode(endpoint, {
      session_id: oauth.sessionId.value,
      code: authCode.trim(),
      ...proxyConfig
    })

    // Build extra with quota control settings
    const baseExtra = oauth.buildExtraInfo(tokenInfo) || {}
    const extra: Record<string, unknown> = { ...baseExtra }

    // Add session limit settings
    if (sessionLimitEnabled.value && maxSessions.value != null && maxSessions.value > 0) {
      extra.max_sessions = maxSessions.value
    }

    // Add RPM limit settings
    if (rpmLimitEnabled.value) {
      const DEFAULT_BASE_RPM = 15
      extra.base_rpm = (baseRpm.value != null && baseRpm.value > 0)
        ? baseRpm.value
        : DEFAULT_BASE_RPM
    }

    const credentials: Record<string, unknown> = { ...tokenInfo }
    applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')
    await createAccountAndFinish(form.platform, addMethod.value as AccountType, credentials, extra)
  } catch (error: any) {
    oauth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
    appStore.showError(oauth.error.value)
  } finally {
    oauth.loading.value = false
  }
}

// 主入口：根据平台路由到对应处理函数
const handleExchangeCode = async () => {
  const authCode = oauthFlowRef.value?.authCode || ''

  switch (form.platform) {
    case 'openai':
      return handleOpenAIExchange(authCode)
    case 'gemini':
      return handleGeminiExchange(authCode)
    case 'antigravity':
      return handleAntigravityExchange(authCode)
    case 'grok':
      return handleGrokExchange(authCode)
    default:
      return handleAnthropicExchange(authCode)
  }
}

const handleCookieAuth = async (sessionKey: string) => {
  oauth.loading.value = true
  oauth.error.value = ''

  try {
    const proxyConfig = form.proxy_id ? { proxy_id: form.proxy_id } : {}
    const keys = oauth.parseSessionKeys(sessionKey)

    if (keys.length === 0) {
      oauth.error.value = t('admin.accounts.oauth.pleaseEnterSessionKey')
      return
    }

    const tempUnschedPayload = tempUnschedEnabled.value
      ? buildTempUnschedRules(tempUnschedRules.value)
      : []
    if (tempUnschedEnabled.value && tempUnschedPayload.length === 0) {
      appStore.showError(t('admin.accounts.tempUnschedulable.rulesInvalid'))
      return
    }

    const endpoint =
      addMethod.value === 'oauth'
        ? '/admin/accounts/cookie-auth'
        : '/admin/accounts/setup-token-cookie-auth'

    let successCount = 0
    let failedCount = 0
    const errors: string[] = []

    for (let i = 0; i < keys.length; i++) {
      try {
        const tokenInfo = await adminAPI.accounts.exchangeCode(endpoint, {
          session_id: '',
          code: keys[i],
          ...proxyConfig
        })

        // Build extra with quota control settings
        const baseExtra = oauth.buildExtraInfo(tokenInfo) || {}
        const extra: Record<string, unknown> = { ...baseExtra }

        // Add session limit settings
        if (sessionLimitEnabled.value && maxSessions.value != null && maxSessions.value > 0) {
          extra.max_sessions = maxSessions.value
        }

        // Add RPM limit settings
        if (rpmLimitEnabled.value) {
          const DEFAULT_BASE_RPM = 15
          extra.base_rpm = (baseRpm.value != null && baseRpm.value > 0)
            ? baseRpm.value
            : DEFAULT_BASE_RPM
        }

        const accountName = keys.length > 1 ? `${form.name} #${i + 1}` : form.name

        const credentials: Record<string, unknown> = { ...tokenInfo }
        applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')
        if (tempUnschedEnabled.value) {
          credentials.temp_unschedulable_enabled = true
          credentials.temp_unschedulable_rules = tempUnschedPayload
        }

        await createAccountRecord({
          name: accountName,
          notes: form.notes,
          platform: form.platform,
          type: addMethod.value, // Use addMethod as type: 'oauth' or 'setup-token'
          credentials,
          extra: withUpstreamRequestIdHeader(extra),
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          load_factor: form.load_factor ?? undefined,
          priority: form.priority,
          rate_multiplier: form.rate_multiplier,
          expires_at: form.expires_at,
          auto_pause_on_expired: autoPauseOnExpired.value
        })

        successCount++
      } catch (error: any) {
        failedCount++
        errors.push(
          t('admin.accounts.oauth.keyAuthFailed', {
            index: i + 1,
            error: error.response?.data?.detail || t('admin.accounts.oauth.authFailed')
          })
        )
      }
    }

    if (successCount > 0) {
      appStore.showSuccess(t('admin.accounts.oauth.successCreated', { count: successCount }))
      if (failedCount === 0) {
        emit('created')
        handleClose()
      } else {
        emit('created')
      }
    }

    if (failedCount > 0) {
      oauth.error.value = errors.join('\n')
    }
  } catch (error: any) {
    oauth.error.value = error.response?.data?.detail || t('admin.accounts.oauth.cookieAuthFailed')
  } finally {
    oauth.loading.value = false
  }
}
</script>
