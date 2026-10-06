<template>
  <!--
    新建渠道弹窗（2026-10-03 由整页改回弹窗）：与编辑同一外壳、同一分区顺序 ——
    上游 / 调度与限额 / 高级（默认收起）/ 备注；成品号点「下一步」进第二步授权。
  -->
  <BaseDialog :show="show" :title="t('admin.accounts.createAccount')" :width="step === 3 ? 'extra-wide' : 'wide'" @close="handleClose">
    <!-- 步骤：第三方 key 是「连上游 → 承接模型」，成品号中间多一步授权 -->
    <ol class="mb-5 flex flex-wrap items-center gap-2 text-13" data-testid="create-account-steps">
      <li v-for="(item, index) in stepItems" :key="item.step" class="flex items-center gap-2">
        <span v-if="index > 0" class="h-px w-6 bg-af-hairline-strong" aria-hidden="true"></span>
        <span
          :class="[
            'flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold',
            step >= item.step ? 'bg-af-ink text-af-on-brand' : 'bg-af-hairline text-af-ink-3'
          ]"
        >
          {{ index + 1 }}
        </span>
        <span :class="step === item.step ? 'font-medium text-af-ink' : 'text-af-ink-3'">{{ item.label }}</span>
      </li>
    </ol>

    <!-- Step 1: Basic Info -->
    <form
      v-if="step === 1"
      id="create-account-form"
      @submit.prevent="handleSubmit"
      class="space-y-5"
    >
      <ChannelFormSection section="upstream" :title="t('admin.accounts.dialog.sections.upstream')">
      <!-- 先选接入方式与来源（muqian 2026-09-25）：第三方 key 不选平台，成品号只选哪家的账号 -->
      <AccessSourcePicker v-model="accessSourceId" />

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

      <!-- Gemini 成品号的附加选项 -->
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
                    class="rounded bg-af-sunken px-2 py-0.5 text-xs font-semibold text-af-ink-2"
                  >
                    {{ t('admin.accounts.gemini.oauthType.badges.individuals') }}
                  </span>
                  <span
                    class="rounded bg-af-sunken px-2 py-0.5 text-xs font-semibold text-af-ink-2"
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
                    class="rounded bg-af-sunken px-2 py-0.5 text-xs font-semibold text-af-ink-2"
                  >
                    {{ t('admin.accounts.gemini.oauthType.badges.enterprise') }}
                  </span>
                  <span
                    class="rounded bg-af-sunken px-2 py-0.5 text-xs font-semibold text-af-ink-2"
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
                    class="rounded bg-af-warning-tint px-2 py-0.5 text-xs font-semibold text-af-warning"
                  >
                    {{ t('admin.accounts.gemini.oauthType.badges.orgManaged') }}
                  </span>
                  <span
                    class="rounded bg-af-warning-tint px-2 py-0.5 text-xs font-semibold text-af-warning"
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
        厂商按地址识别，识别出的厂商才有它的专属选项（Coding 套餐、智谱团队版）。常用官方地址只有国产厂商与 OpenCode，
        Anthropic / OpenAI / Gemini / Grok 的官方地址手填也不拦，一律按中转处理（muqian 2026-09-29）。
      -->
      <div v-if="form.type === 'apikey'" class="space-y-4">
        <div>
          <ProtocolEndpointsEditor
            v-model="protocolEndpoints"
            v-model:draft-url="keyAddressDraft"
            :protocols="UPSTREAM_PROTOCOLS"
            :official-endpoints="officialProtocolEndpoints"
            :defaults-load-failed="protocolDefaultsLoadFailed"
          >
            <template #url-suffix>
              <KeyAddressPresetMenu
                v-if="keyPresets.length > 0"
                class="sm:w-48 sm:shrink-0"
                :presets="keyPresets"
                @select="applyKeyAddressPreset"
              />
            </template>
          </ProtocolEndpointsEditor>
          <p v-if="keyVendor" class="input-hint" data-testid="key-vendor-detected">
            {{ t('admin.accounts.keyAddress.detected', { vendor: keyVendorLabel }) }}
          </p>
          <p v-else-if="hasKeyAddress" class="input-hint" data-testid="key-vendor-relay">
            {{ t('admin.accounts.keyAddress.relay') }}
          </p>
        </div>

        <!-- 按量 / Coding 套餐：地址分得出就不问（识别提示里带上）；MiniMax 两种套餐同一个地址，要管理员选 -->
        <KeyPlanModePicker v-if="keyPlanNeedsChoice" v-model="keyPlanMode" test-id="key-plan-mode" />
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
        <p class="input-hint">{{ t('admin.accounts.upstream.apiKeyHint') }}</p>
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.proxy') }}</label>
        <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
      </div>

      <!-- 检测上游（2026-10-03）：一次查协议与模型；查到的模型在下一步「承接模型」里预填。检测也走上面的代理 -->
      <UpstreamDetect
        v-if="form.type === 'apikey'"
        :protocol-endpoints="protocolEndpoints"
        :draft-url="keyAddressDraft"
        :api-key="apiKeyValue"
        :proxy-id="form.proxy_id"
        @select="applyProbedProtocol"
        @models="detected = $event"
      />

      <!-- 渠道名称放在最后：第三方 key 按「上游 · 协议」自动建议（改过就不再跟着变） -->
      <div>
        <label class="input-label">{{ t('admin.accounts.accountName') }}</label>
        <input
          v-model="form.name"
          type="text"
          :required="!isGrokSSOInputMethod"
          class="input"
          :placeholder="t('admin.accounts.enterAccountName')"
          data-testid="channel-name"
        />
      </div>

      </ChannelFormSection>

      <ChannelFormSection section="scheduling" :title="t('admin.accounts.dialog.sections.scheduling')">
        <ChannelLimitsFields
          v-model:priority="form.priority"
          v-model:concurrency="form.concurrency"
          v-model:expires-at="form.expires_at"
        />

        <ChannelQuotaFields
          v-if="form.type === 'apikey' || form.type === 'bedrock'"
          v-model:total-limit="editQuotaLimit"
          v-model:daily-limit="editQuotaDailyLimit"
          v-model:weekly-limit="editQuotaWeeklyLimit"
        />

        <AnthropicSubscriptionLimits
          v-if="form.platform === 'anthropic' && accountCategory === 'oauth-based'"
          v-model:session-limit-enabled="sessionLimitEnabled"
          v-model:max-sessions="maxSessions"
          v-model:rpm-limit-enabled="rpmLimitEnabled"
          v-model:base-rpm="baseRpm"
        />

        <!-- 超量：Antigravity 成品号（OAuth）专属；第三方 key 按协议调度，没有这一项 -->
        <ChannelSettingToggle
          v-if="form.platform === 'antigravity'"
          v-model="allowOverages"
          :label="t('admin.accounts.allowOverages')"
          :description="t('admin.accounts.allowOveragesTooltip')"
          test-id="allow-overages"
        />
      </ChannelFormSection>

      <ChannelAdvancedSection v-if="advancedItems.length > 0" :summary="advancedItems.join(' · ')">
        <!-- 请求头覆写：任何第三方 key 与 Grok 成品号 -->
        <HeaderOverrideField
          v-if="headerOverrideCapable"
          v-model:rows="headerOverrideRows"
          test-id="create-header-override"
        />

        <!-- 池模式：同渠道重试次数与状态码写死在后端（channel_features.go），这里只有开关 -->
        <ChannelSettingToggle
          v-if="poolModeCapable"
          v-model="poolModeEnabled"
          :label="t('admin.accounts.poolMode')"
          :description="t('admin.accounts.poolModeHint')"
          test-id="pool-mode"
        >
          <p class="rounded-lg bg-af-sunken p-3 text-xs text-af-ink-2">
            <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
            {{ t('admin.accounts.poolModeInfo') }}
          </p>
        </ChannelSettingToggle>

        <!-- 第三方 key 的 Anthropic 协议设置：配了 anthropic 协议地址才展示，不看平台标签 -->
        <AnthropicKeySettings
          v-if="anthropicKeySettingsVisible"
          v-model:auth-scheme="anthropicAPIKeyAuthScheme"
          v-model:bedrock-cc-compat="bedrockCCCompatEnabled"
          test-id-prefix="create"
        />

        <ChannelSettingToggle
          v-if="interceptWarmupCapable"
          v-model="interceptWarmupRequests"
          :label="t('admin.accounts.interceptWarmupRequests')"
          :description="t('admin.accounts.interceptWarmupRequestsDesc')"
          test-id="intercept-warmup"
        />

        <!-- 智谱团队版 Coding Plan：组织 / 项目 ID（可选，填写后额度探测走团队版端点） -->
        <ZhipuTeamFields
          v-if="zhipuTeamCapable"
          v-model:organization="zhipuOrganization"
          v-model:project="zhipuProject"
        />

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
      </ChannelAdvancedSection>

      <ChannelFormSection section="notes" :title="t('admin.accounts.dialog.sections.notes')">
        <div>
          <textarea
            v-model="form.notes"
            rows="3"
            class="input"
            :aria-label="t('admin.accounts.notes')"
            :placeholder="t('admin.accounts.notesPlaceholder')"
          ></textarea>
          <p class="input-hint">{{ t('admin.accounts.notesHint') }}</p>
        </div>
      </ChannelFormSection>
    </form>

    <!-- Step 2: OAuth Authorization -->
    <div v-else-if="step === 2" class="space-y-5">
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
        :show-codex-session-import-option="form.platform === 'openai'"
        :show-agent-identity-option="form.platform === 'openai'"
        :show-codex-pat-option="form.platform === 'openai'"
        :show-sso-option="form.platform === 'grok'"
        :show-manual-option="true"
        :initial-input-method="'manual'"
        :platform="form.platform"
        :show-project-id="geminiOAuthType === 'code_assist'"
        @generate-url="handleGenerateUrl"
        @cookie-auth="handleCookieAuth"
        @validate-refresh-token="handleValidateRefreshToken"
        @validate-mobile-refresh-token="handleOpenAIValidateMobileRT"
        @import-codex-session="handleOpenAIImportCodexSession"
        @import-codex-pat="handleOpenAIImportCodexPAT"
        @import-sso="handleGrokImportSSO"
      />

    </div>

    <!--
      第 3 步「承接模型」：上面是上游名单（2026-10-06：按检测到的上游名单承接，不再列整个目录），
      下面就是价格页「按渠道」里这个渠道的那一块（同组件、同保存接口），填上游价后保存。
      名单里目录有的默认勾上；目录没有的先看是不是官方模型 ID——是的加进目录，不是的映射到目录里的模型。
    -->
    <div v-else class="space-y-4" data-testid="create-account-bind">
      <p class="text-13 text-af-ink-3">{{ detected ? t('admin.accounts.dialog.bind.hint') : t('admin.accounts.dialog.bind.hintNoDetect') }}</p>
      <FormError v-if="bindLoadError" :message="bindLoadError" />
      <p v-else-if="!bindAccount || !channelState || !overview" class="flex items-center gap-2 text-13 text-af-ink-3">
        <Icon name="refresh" size="sm" class="animate-spin" />
        {{ t('admin.accounts.dialog.bind.loading') }}
      </p>
      <template v-else>
        <UpstreamBindPanel
          v-if="detected && detected.names.length > 0"
          :protocol-label="bindProtocolLabel"
          :names="detected.names"
          :catalog="catalogEntries"
          :bindable-ids="bindableEntryIds"
          :rows="channelState.draft.rows"
          :lookup="officialLookup"
          :lookup-state="officialLookupState"
          :busy="addingOfficial"
          @toggle="onToggleDetected"
          @map="onMapUpstream"
          @unmap="onUnmapUpstream"
          @add-official="onAddOfficial"
          @add-all-official="onAddAllOfficial"
        />
        <FormError :message="addOfficialError" />
        <PricingChannelBlock
          :account="bindAccount"
          :state="channelState"
          :accounts="overview.accounts"
          :entries="overview.entries"
          :default-sale-ratio="overview.default_sale_ratio"
          :min-margin="overview.min_margin"
          @saved="onBindSaved"
        />
      </template>
    </div>

    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-3">
        <FormError class="mr-auto min-w-0 flex-1" :message="submitError" />
        <template v-if="step === 1">
          <button type="button" class="btn btn-secondary" @click="handleClose">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="create-account-form" :disabled="submitting" class="btn btn-primary">
            <Icon v-if="submitting" name="refresh" size="sm" class="-ml-1 mr-2 animate-spin" />
            {{
              isOAuthFlow
                ? t('common.next')
                : submitting
                  ? t('admin.accounts.creating')
                  : t('common.create')
            }}
          </button>
        </template>
        <template v-else-if="step === 2">
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
            <Icon v-if="currentOAuthLoading" name="refresh" size="sm" class="-ml-1 mr-2 animate-spin" />
            {{
              currentOAuthLoading
                ? t('admin.accounts.oauth.verifying')
                : t('admin.accounts.oauth.completeAuth')
            }}
          </button>
        </template>
        <button v-else type="button" class="btn btn-primary" data-testid="create-account-done" @click="handleClose">
          {{ t('admin.accounts.dialog.bind.done') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="showDiscardConfirm"
    :title="t('admin.accounts.dialog.discard.title')"
    :message="t('admin.accounts.dialog.discard.message')"
    :confirm-text="t('admin.accounts.dialog.discard.confirm')"
    :cancel-text="t('admin.accounts.dialog.discard.keepEditing')"
    danger
    @confirm="confirmDiscard"
    @cancel="showDiscardConfirm = false"
  />

  <!-- 承接模型那一步里「目录里没有」的模型：叠一层新建模型弹窗，预填标识 -->
  <ModelCreateDialog
    :show="creatingModelId !== ''"
    :initial-model-id="creatingModelId"
    :vendor-options="catalogVendors"
    :existing-model-ids="catalogModelIds"
    :z-index="60"
    @saved="createdModelId = $event.id"
    @close="onModelDialogClose"
  />

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

import { adminAPI } from '@/api/admin'
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
  ProtocolEndpoints,
  UpstreamProtocol
} from '@/types'
import type { ProtocolDefaultsResponse } from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import FormError from '@/components/common/FormError.vue'
import Icon from '@/components/icons/Icon.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import AccessSourcePicker from '@/components/account/AccessSourcePicker.vue'
import UpstreamDetect from '@/components/account/channel/UpstreamDetect.vue'
import {
  classifyUpstreamModels,
  upstreamModelFor,
  type DetectedMatch,
  type DetectedModels,
  type OfficialCandidate
} from '@/components/account/channel/upstreamModels'
import UpstreamBindPanel from '@/components/account/channel/UpstreamBindPanel.vue'
import { entryToRequest } from '@/components/admin/catalog/entryRequest'
import { suggestChannelName, upstreamHostLabel } from '@/components/account/channel/channelName'
import PricingChannelBlock from '@/components/admin/pricing/PricingChannelBlock.vue'
import ModelCreateDialog from '@/components/admin/catalog/ModelCreateDialog.vue'
import { catalogVendorChoices } from '@/components/admin/catalog/vendorLabel'
import {
  channelDraftChanges,
  channelDraftFrom,
  cloneChannelDraft,
  cloneKeyedRows,
  emptyPriceRow,
  priceRowFrom,
  siblingBindingOf,
  type BlockState,
  type ChannelDraft
} from '@/components/admin/pricing/pricingDraft'
import type { PricingOverview } from '@/api/admin/pricing'
import type { ModelCatalogEntry, OfficialModelLookupResult } from '@/api/admin/modelCatalog'
import {
  DEFAULT_ACCESS_SOURCE_ID,
  findAccessSource
} from '@/components/account/accessSources'
import KeyAddressPresetMenu from '@/components/account/KeyAddressPresetMenu.vue'
import {
  VENDORS_WITH_CODING_PLAN,
  apiKeyPlaceholderFor,
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
  loadProtocolDefaults,
  protocolDefaultsFor,
  trimProtocolEndpoints,
  validateProtocolEndpoints
} from '@/components/account/protocolEndpoints'
import ChannelFormSection from '@/components/account/channel/ChannelFormSection.vue'
import ChannelLimitsFields from '@/components/account/channel/ChannelLimitsFields.vue'
import ChannelQuotaFields from '@/components/account/channel/ChannelQuotaFields.vue'
import ChannelSettingToggle from '@/components/account/channel/ChannelSettingToggle.vue'
import ChannelAdvancedSection from '@/components/account/channel/ChannelAdvancedSection.vue'
import AnthropicSubscriptionLimits from '@/components/account/channel/AnthropicSubscriptionLimits.vue'
import AnthropicKeySettings from '@/components/account/channel/AnthropicKeySettings.vue'
import HeaderOverrideField from '@/components/account/channel/HeaderOverrideField.vue'
import KeyPlanModePicker from '@/components/account/channel/KeyPlanModePicker.vue'
import ZhipuTeamFields from '@/components/account/channel/ZhipuTeamFields.vue'
import {
  applyAntigravityProjectID,
  applyHeaderOverride,
  applyInterceptWarmup,
  isHeaderOverrideCapable,
  validateHeaderOverrideRows,
  type CnAccountMode,
  type HeaderOverrideRow
} from '@/components/account/credentialsBuilder'
import { extractApiErrorMessage } from '@/utils/apiError'
import { VERTEX_LOCATION_OPTIONS } from '@/constants/account'
import OAuthAuthorizationFlow from './OAuthAuthorizationFlow.vue'

// Type for exposed OAuthAuthorizationFlow component
// Note: defineExpose automatically unwraps refs, so we use the unwrapped types
interface OAuthFlowExposed {
  authCode: string
  oauthState: string
  projectId: string
  sessionKey: string
  refreshToken: string
  codexSession: string
  codexPAT: string
  ssoCookie: string
  inputMethod: AuthInputMethod
  reset: () => void
}

const { t } = useI18n()
// 打开弹窗时的接入方式：第三方 key（2026-10-03 起，原来是 Claude 成品号）
const DEFAULT_ACCESS_SOURCE = findAccessSource(DEFAULT_ACCESS_SOURCE_ID)

const oauthStepTitle = computed(() => {
  if (form.platform === 'openai') return t('admin.accounts.oauth.openai.title')
  if (form.platform === 'gemini') return t('admin.accounts.oauth.gemini.title')
  if (form.platform === 'antigravity') return t('admin.accounts.oauth.antigravity.title')
  if (form.platform === 'grok') return t('admin.accounts.oauth.grok.title')
  return t('admin.accounts.oauth.title')
})

// 步骤条：第三方 key「连上游 → 承接模型」，成品号中间多一步授权
const stepItems = computed(() => [
  { step: 1, label: t('admin.accounts.dialog.steps.upstream') },
  ...(isOAuthFlow.value ? [{ step: 2, label: oauthStepTitle.value }] : []),
  { step: 3, label: t('admin.accounts.dialog.steps.bind') }
])

// API Key 的占位跟着按地址识别出的厂商走（与编辑同一规则）；提示一律用通用说法
const apiKeyValuePlaceholder = computed(() => apiKeyPlaceholderFor(keyVendor.value))

interface Props {
  show: boolean
  proxies: Proxy[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  created: []
}>()

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

// State
const step = ref(1)
const submitting = ref(false)
// 第一步校验不过 / 建号失败的提示，显示在底部按钮左边（第二步授权的错误由授权组件自己显示）
const submitError = ref('')
const accountCategory = ref<'oauth-based' | 'apikey' | 'bedrock' | 'service_account'>(DEFAULT_ACCESS_SOURCE.category) // UI selection for account category
const addMethod = ref<AddMethod>('oauth') // For oauth-based: 'oauth' or 'setup-token'
const apiKeyValue = ref('')

// 智谱团队版 Coding Plan：组织/项目 ID，写入 credentials 供额度探测切换团队端点
const zhipuOrganization = ref('')
const zhipuProject = ref('')

// ── 接入方式（新建渠道第一步，见 accessSources.ts） ──
const accessSourceId = ref(DEFAULT_ACCESS_SOURCE_ID)
const accessSource = computed(() => findAccessSource(accessSourceId.value))
// 第三方 key 不选平台：form.platform 只是表单内部占位，提交时不带，厂商按地址识别（keyVendor）
const isKeyMode = computed(() => accessSource.value.kind === 'key')

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
  // 换了接入方式，上一次提交的报错已经对不上了
  submitError.value = ''
  void applyAccessSource(sourceId)
})

// ── 第三方 key 协议地址（muqian 2026-09-25：key 不选平台） ──
// 存库的就是管理员填的显式地址：可从常用官方地址里选一条填入，也可直接填中转地址。
const protocolDefaults = ref<ProtocolDefaultsResponse | null>(null)
const protocolDefaultsLoadFailed = ref(false)
const protocolEndpoints = ref<ProtocolEndpoints>({})
// 还没选协议时地址栏里先填的地址（好用「检测上游」），选了协议就并进 protocolEndpoints
const keyAddressDraft = ref('')
const keyPresets = computed(() => keyAddressPresets(protocolDefaults.value))
// 按地址识别出的厂商（官方域名表由后端下发，与后端 Account.Vendor 同口径）；认不出的是中转
const keyVendor = computed(() =>
  isKeyMode.value ? detectKeyVendor(protocolEndpoints.value, protocolDefaults.value?.vendor_hosts) : null
)
const hasKeyAddress = computed(() => Object.values(protocolEndpoints.value).some((url) => !!url?.trim()))
// 按量 / Coding 套餐：只有 Kimi / 智谱 / MiniMax 有。地址能分出来就跟地址走，分不出来（MiniMax 同地址）由管理员选
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
// 渠道名建议（2026-10-03）：第三方 key 按「上游 · 协议」（识别出厂商用厂商名，中转用地址里的上游名）；
// 名称空着或还是上一次的建议时才填，管理员改过就不再覆盖
let nameSuggestion = ''
const suggestedName = computed(() => {
  if (!isKeyMode.value) return ''
  const protocol = currentProtocolOf(protocolEndpoints.value)
  const url = protocol ? protocolEndpoints.value[protocol]?.trim() : ''
  if (!protocol || !url) return ''
  const label = keyVendor.value ? platformLabel(keyVendor.value) : upstreamHostLabel(url)
  return suggestChannelName(label, protocol)
})
watch(suggestedName, (next) => {
  if (!form.name.trim() || form.name === nameSuggestion) form.name = next
  nameSuggestion = next
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
// 检测上游里选中一个协议：协议与地址填进表单，草稿作废
function applyProbedProtocol(protocol: UpstreamProtocol, url: string) {
  protocolEndpoints.value = { [protocol]: url }
  keyAddressDraft.value = ''
}
function applyKeyAddressPreset(preset: KeyAddressPreset) {
  protocolEndpoints.value = { [preset.protocol]: preset.url }
  keyAddressDraft.value = ''
  if (preset.mode === 'payg' || preset.mode === 'coding') keyPlanMode.value = preset.mode
}
async function ensureProtocolDefaults() {
  try {
    protocolDefaults.value = await loadProtocolDefaults()
    protocolDefaultsLoadFailed.value = false
  } catch {
    protocolDefaultsLoadFailed.value = true
  }
}
// 提交前校验协议地址，有问题显示在底部并返回 null。
function validatedProtocolEndpoints(): ProtocolEndpoints | null {
  const issue = validateProtocolEndpoints(protocolEndpoints.value)
  if (issue) {
    submitError.value = describeProtocolEndpointsIssue(issue, t)
    return null
  }
  return trimProtocolEndpoints(protocolEndpoints.value)
}

const editQuotaLimit = ref<number | null>(null)
const editQuotaDailyLimit = ref<number | null>(null)
const editQuotaWeeklyLimit = ref<number | null>(null)
// 池模式同渠道重试次数与状态码写死在后端（channel_features.go），这里只有开关
const poolModeEnabled = ref(false)
const headerOverrideRows = ref<HeaderOverrideRow[]>([])

// 请求头覆写的前置校验，失败时提示并返回 false。
// Grok OAuth 三条创建路径（授权码/RT 批量/SSO 批量）必须在兑换 code 之前调用，
// 避免校验失败时白白消耗一次性授权码。
const validateHeaderOverrideForm = (): boolean => {
  const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
  if (headerError) {
    submitError.value = t(`admin.accounts.headerOverride.${headerError}`)
    return false
  }
  return true
}

// 把已通过校验的请求头覆写写入 credentials（Grok OAuth 成品号只走官方地址，只有这一项上游配置）
const applyGrokOAuthUpstreamConfig = (credentials: Record<string, unknown>) => {
  applyHeaderOverride(credentials, headerOverrideRows.value, 'create')
}

// 第三方 key（含 Antigravity 上游 key）的请求头覆写：校验通过才写入 credentials。
const applyKeyHeaderOverride = (credentials: Record<string, unknown>): boolean => {
  if (!validateHeaderOverrideForm()) {
    return false
  }
  applyHeaderOverride(credentials, headerOverrideRows.value, 'create')
  return true
}
const interceptWarmupRequests = ref(false)
type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'
const anthropicAPIKeyAuthScheme = ref<AnthropicAPIKeyAuthScheme>('x_api_key')
const bedrockCCCompatEnabled = ref(false)


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
// 初始与重置同一个值：第一张卡 Google One（之前重置成 Code Assist，第二次打开默认值就变了）
const geminiOAuthType = ref<'code_assist' | 'google_one' | 'ai_studio'>('google_one')
const geminiAIStudioOAuthEnabled = ref(false)
function buildAntigravityExtra(): Record<string, unknown> | undefined {
  const extra: Record<string, unknown> = {}
  if (allowOverages.value) extra.allow_overages = true
  return Object.keys(extra).length > 0 ? extra : undefined
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

const geminiQuotaDocs = {
  codeAssist: 'https://developers.google.com/gemini-code-assist/resources/quotas',
  aiStudio: 'https://ai.google.dev/pricing',
  vertex: 'https://cloud.google.com/vertex-ai/generative-ai/docs/quotas'
}

const geminiHelpLinks = {
  gcpProject: 'https://console.cloud.google.com/welcome/new',
  geminiWebActivation: 'https://gemini.google.com/gems/create?hl=en-US&pli=1',
  countryCheck: 'https://policies.google.com/terms',
  countryChange: 'https://policies.google.com/country-association-form'
}

const form = reactive({
  name: '',
  notes: '',
  platform: DEFAULT_ACCESS_SOURCE.platform as AccountPlatform,
  type: 'oauth' as AccountType, // 由下面的 watcher 按类别 / 添加方式 / 平台同步
  credentials: {} as Record<string, unknown>,
  proxy_id: null as number | null,
  concurrency: 10,
  priority: 1,
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

const headerOverrideCapable = computed(() => isHeaderOverrideCapable(form.platform, form.type))
const poolModeCapable = computed(() => form.type === 'apikey' && form.platform !== 'antigravity')
const interceptWarmupCapable = computed(() => form.platform === 'anthropic' || form.platform === 'antigravity')
const zhipuTeamCapable = computed(() => keyVendor.value === 'zhipu' && keyPlanMode.value === 'coding')
// 「高级」收起时标题下列出里面有哪几项；条件与模板里各项的 v-if 一一对应（改一处两处一起改）
const advancedItems = computed(() => {
  const items: string[] = []
  if (headerOverrideCapable.value) items.push(t('admin.accounts.headerOverride.title'))
  if (poolModeCapable.value) items.push(t('admin.accounts.poolMode'))
  if (anthropicKeySettingsVisible.value) {
    items.push(t('admin.accounts.anthropic.apiKeyAuthScheme'), t('admin.accounts.anthropic.bedrockCCCompat'))
  }
  if (interceptWarmupCapable.value) items.push(t('admin.accounts.interceptWarmupRequests'))
  if (zhipuTeamCapable.value) items.push(t('admin.accounts.cnProviders.zhipuTeam.title'))
  if (form.platform === 'antigravity') items.push(t('admin.accounts.antigravityProjectIdLabel'))
  return items
})

const isGrokSSOInputMethod = computed(() => form.platform === 'grok' && oauthFlowRef.value?.inputMethod === 'sso_cookie')

const isManualInputMethod = computed(() => {
  return oauthFlowRef.value?.inputMethod === 'manual'
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
    // 第三方 key 也能配的协议设置（Anthropic 认证方式 / Bedrock CC 兼容）：切换平台一律清空
    // （不看切到哪个平台），与请求头覆写一致；同一平台内删掉地址行导致的隐藏，由提交时的可见性判断保证不写入。
    anthropicAPIKeyAuthScheme.value = 'x_api_key'
    bedrockCCCompatEnabled.value = false
    // 请求头覆写为平台相关配置（常用头集合不同），切换平台时清空，
    // 避免上一平台的配置行被提交到新平台账号
    headerOverrideRows.value = []
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
    submitError.value = t('admin.accounts.oauth.gemini.aiStudioNotConfigured')
    return
  }
  geminiOAuthType.value = oauthType
}


// 所有单个建号都走这里：第三方 key 不带平台（后端按地址认厂商，认不出的中转按协议归族）。
// 建成的渠道记下来：这次只建了一个时，下一步「承接模型」就给它。
const createdAccounts = ref<Account[]>([])
const createAccountRecord = async (payload: CreateAccountRequest): Promise<Account> => {
  const body: CreateAccountRequest = { ...payload }
  if (body.type === 'apikey') delete body.platform
  const account = await adminAPI.accounts.create(body)
  createdAccounts.value.push(account)
  return account
}

// 建号全部成功之后：只建了一个渠道就进「承接模型」，批量建了多个（或走导入接口没拿到渠道）就直接关
const finishCreated = () => {
  emit('created')
  if (createdAccounts.value.length === 1) void enterBindStep(createdAccounts.value[0])
  else handleClose()
}

const submitCreateAccount = async (payload: CreateAccountRequest) => {
  submitting.value = true
  try {
    await createAccountRecord(payload)
    finishCreated()
  } catch (error) {
    submitError.value = extractApiErrorMessage(error, t('admin.accounts.failedToCreate'))
  } finally {
    submitting.value = false
  }
}

// Methods
const resetForm = () => {
  step.value = 1
  submitError.value = ''
  createdAccounts.value = []
  detected.value = null
  bindAccountId.value = null
  overview.value = null
  channelState.value = null
  bindLoadError.value = ''
  catalogEntries.value = []
  creatingModelId.value = ''
  createdModelId.value = null
  officialLookup.value = null
  officialLookupState.value = 'idle'
  addingOfficial.value = false
  addOfficialError.value = ''
  nameSuggestion = ''
  form.name = ''
  form.notes = ''
  // form.type 不在这里设：它由 watcher 按类别 / 添加方式 / 平台同步，三者没变时它本来就对
  form.platform = DEFAULT_ACCESS_SOURCE.platform
  form.credentials = {}
  form.proxy_id = null
  form.concurrency = 10
  form.priority = 1
  form.expires_at = null
  accountCategory.value = DEFAULT_ACCESS_SOURCE.category
  addMethod.value = 'oauth'
  keyPlanMode.value = 'payg'
  protocolEndpoints.value = {}
  keyAddressDraft.value = ''
  zhipuOrganization.value = ''
  zhipuProject.value = ''
  apiKeyValue.value = ''
  editQuotaLimit.value = null
  editQuotaDailyLimit.value = null
  editQuotaWeeklyLimit.value = null
  accessSourceId.value = DEFAULT_ACCESS_SOURCE_ID
  poolModeEnabled.value = false
  headerOverrideRows.value = []
  interceptWarmupRequests.value = false
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
  geminiOAuthType.value = 'google_one'
  oauth.resetState()
  openaiOAuth.resetState()
  geminiOAuth.resetState()
  antigravityOAuth.resetState()
  grokOAuth.resetState()
  oauthFlowRef.value?.reset()
}

// 第 1 / 2 步填过东西（改过的名称、备注、Key、地址、各平台凭据）：关之前确认，误点取消 / Esc 不再静默丢掉
const showDiscardConfirm = ref(false)
const hasUnsavedInput = () => {
  const typed = [
    form.notes,
    apiKeyValue.value,
    keyAddressDraft.value,
    zhipuOrganization.value,
    zhipuProject.value,
    antigravityProjectId.value,
    bedrockAccessKeyId.value,
    bedrockSecretAccessKey.value,
    bedrockApiKeyValue.value,
    vertexServiceAccountJson.value
  ]
  const renamed = form.name.trim() !== '' && form.name !== nameSuggestion
  return renamed || hasKeyAddress.value || typed.some((value) => (value ?? '').trim() !== '')
}

// 承接那一块改了没保存时不关（右上角关闭也一样）：先保存或点那一块的「撤销」
const handleClose = () => {
  if (step.value === 3 && channelState.value && channelDraftChanges(channelState.value) > 0) {
    submitError.value = t('admin.accounts.dialog.bind.unsaved')
    return
  }
  if (step.value < 3 && hasUnsavedInput()) {
    showDiscardConfirm.value = true
    return
  }
  emit('close')
}
const confirmDiscard = () => {
  showDiscardConfirm.value = false
  emit('close')
}

// ── 第 3 步「承接模型」：价格页「按渠道」那一块 ──
// 检测上游的结果（名单 + 对照目录）：没检测或检测失败为 null
const detected = ref<{ names: string[]; classified: DetectedModels } | null>(null)
const bindAccountId = ref<number | null>(null)
const overview = ref<PricingOverview | null>(null)
const channelState = ref<BlockState<ChannelDraft> | null>(null)
const bindLoadError = ref('')
// 全部目录条目：判断「目录里没有」、给新建模型弹窗做厂商选项和重名校验
const catalogEntries = ref<ModelCatalogEntry[]>([])
const creatingModelId = ref('')
// 叠层里刚建好的模型：回来时这个渠道能承接就直接加一行（没上架也加，承接与上架无关）
const createdModelId = ref<number | null>(null)

const bindAccount = computed(() => overview.value?.accounts.find((account) => account.id === bindAccountId.value) ?? null)
const catalogVendors = computed(() => catalogVendorChoices(catalogEntries.value))
const catalogModelIds = computed(() => catalogEntries.value.map((entry) => entry.model_id))
const bindProtocolLabel = computed(() => {
  const protocol = bindAccount.value?.protocol
  return protocol ? t(`admin.accounts.protocolEndpoints.protocols.${protocol}`) : ''
})
// 这个渠道能承接的目录条目（价格页那一块只收这些）
const bindableEntryIds = computed(() => {
  const accountId = bindAccountId.value
  return (overview.value?.entries ?? []).filter((entry) => accountId != null && entry.bindable_account_ids.includes(accountId)).map((entry) => entry.id)
})

// 上游名单里目录没有的：查一次是不是官方模型 ID（联网的 LiteLLM 公开价格表）
const officialLookup = ref<OfficialModelLookupResult | null>(null)
const officialLookupState = ref<'idle' | 'loading' | 'done'>('idle')
const addingOfficial = ref(false)
const addOfficialError = ref('')

async function lookupOfficialModels() {
  const missing = detected.value ? classifyUpstreamModels(detected.value.names, catalogEntries.value).missing : []
  if (missing.length === 0) {
    officialLookupState.value = 'done'
    return
  }
  officialLookupState.value = 'loading'
  try {
    officialLookup.value = await adminAPI.modelCatalog.officialLookup(missing)
  } catch {
    // 查不到就当联网名单不可用：两种做法（加进目录 / 映射）都给，由管理员判断
    officialLookup.value = null
  } finally {
    officialLookupState.value = 'done'
  }
}

async function enterBindStep(account: Account) {
  bindAccountId.value = account.id
  step.value = 3
  submitError.value = ''
  await loadBindData(true)
  void lookupOfficialModels()
}

/** 新承接行：同上游渠道承接过这个模型的带上它的价 */
function newBindRow(entryId: number, upstreamModel: string) {
  const data = overview.value
  const account = bindAccount.value
  const entry = data?.entries.find((item) => item.id === entryId)
  const sibling = entry && account && data ? siblingBindingOf(entry, account, data.accounts) : null
  return { id: entryId, upstreamModel, prices: sibling ? priceRowFrom(sibling) : emptyPriceRow() }
}

/** 上游名单里目录有的（已上架与未上架都算）、这个渠道能承接、还没加的：默认都加成新行（muqian 2026-10-06 定默认勾上） */
function addDetectedRows(draft: ChannelDraft, data: PricingOverview, account: PricingOverview['accounts'][number]) {
  if (!detected.value) return
  const result = classifyUpstreamModels(detected.value.names, catalogEntries.value)
  for (const match of [...result.listed, ...result.unlisted]) {
    const entry = data.entries.find((item) => item.id === match.entry.id)
    if (!entry || !entry.bindable_account_ids.includes(account.id)) continue
    if (draft.rows.some((row) => row.id === entry.id)) continue
    draft.rows.push(newBindRow(entry.id, upstreamModelFor(match)))
  }
}

// ---- 上游名单那一块的操作：都改下面价格块的草稿，保存仍在那一块
function onToggleDetected(match: DetectedMatch, checked: boolean) {
  const draft = channelState.value?.draft
  if (!draft) return
  if (!checked) {
    draft.rows = draft.rows.filter((row) => row.id !== match.entry.id)
  } else if (!draft.rows.some((row) => row.id === match.entry.id)) {
    draft.rows.push(newBindRow(match.entry.id, upstreamModelFor(match)))
  }
}

/** 非官方名字映射到目录模型：这个渠道承接那个模型，上游模型名填这个名字 */
function onMapUpstream(name: string, entryId: number) {
  const draft = channelState.value?.draft
  if (!draft) return
  const existing = draft.rows.find((row) => row.id === entryId)
  if (existing) existing.upstreamModel = name
  else draft.rows.push(newBindRow(entryId, name))
}

function onUnmapUpstream(name: string) {
  const draft = channelState.value?.draft
  if (!draft) return
  draft.rows = draft.rows.filter((row) => row.upstreamModel !== name)
}

/** 官方模型加进目录：有官方价的直接建（未上架、带官方价），读不出价的打开新建弹窗填价 */
async function onAddOfficial(candidate: OfficialCandidate) {
  if (!candidate.priced || !candidate.entry) {
    creatingModelId.value = candidate.name
    return
  }
  await addOfficialEntries([candidate])
}

async function onAddAllOfficial(candidates: OfficialCandidate[]) {
  await addOfficialEntries(candidates.filter((candidate) => candidate.priced && candidate.entry))
}

async function addOfficialEntries(candidates: OfficialCandidate[]) {
  addingOfficial.value = true
  addOfficialError.value = ''
  const createdIds: number[] = []
  try {
    for (const candidate of candidates) {
      if (!candidate.entry) continue
      const created = await adminAPI.modelCatalog.createEntry({
        ...entryToRequest(candidate.entry),
        model_id: candidate.name,
        billing_mode: 'token',
        status: 'unlisted'
      })
      createdIds.push(created.id)
    }
  } catch (error) {
    addOfficialError.value = extractApiErrorMessage(error, t('admin.accounts.upstreamBind.addFailed'), {
      MODEL_CATALOG_ENTRY_EXISTS: t('admin.modelCatalog.dialog.exists')
    })
  } finally {
    // 建好的（哪怕中途失败也有一部分）重拉目录与价格，能承接的加成新行
    if (createdIds.length > 0) {
      await loadBindData(false)
      const draft = channelState.value?.draft
      for (const id of createdIds) {
        if (draft && bindableEntryIds.value.includes(id) && !draft.rows.some((row) => row.id === id)) draft.rows.push(newBindRow(id, ''))
      }
    }
    addingOfficial.value = false
  }
}

async function loadBindData(prefill: boolean) {
  bindLoadError.value = ''
  try {
    const [data, catalog] = await Promise.all([adminAPI.pricing.overview(), adminAPI.modelCatalog.listEntries()])
    overview.value = data
    catalogEntries.value = catalog
    const account = data.accounts.find((item) => item.id === bindAccountId.value)
    if (!account) {
      bindLoadError.value = t('admin.accounts.dialog.bind.notFound')
      return
    }
    const current = channelState.value
    if (!current || channelDraftChanges(current) === 0) {
      const initial = channelDraftFrom(account.id, data.entries)
      channelState.value = { initial, draft: cloneChannelDraft(initial) }
    } else {
      // 改到一半：把服务端新出现的承接（比如在叠层「新建模型」里给这个渠道加的）并进来——
      // 这一块保存是整份覆盖，不并进来的话一保存就把它删了
      for (const row of channelDraftFrom(account.id, data.entries).rows) {
        if (current.initial.rows.some((item) => item.id === row.id)) continue
        current.initial.rows.push(...cloneKeyedRows([row]))
        if (!current.draft.rows.some((item) => item.id === row.id)) current.draft.rows.push(...cloneKeyedRows([row]))
      }
    }
    if (prefill && channelState.value) addDetectedRows(channelState.value.draft, data, account)
  } catch (error) {
    bindLoadError.value = extractApiErrorMessage(error, t('admin.pricing.loadFailed'))
  }
}

function onBindSaved() {
  emit('created')
  void loadBindData(false)
}

// 叠层建好模型回来：重拉价格与目录，新模型这个渠道能承接、还没加的话加成新行
async function onModelDialogClose() {
  creatingModelId.value = ''
  await loadBindData(true)
  const entryId = createdModelId.value
  createdModelId.value = null
  const data = overview.value
  const account = bindAccount.value
  const state = channelState.value
  if (entryId == null || !data || !account || !state) return
  const entry = data.entries.find((item) => item.id === entryId)
  if (!entry || !entry.bindable_account_ids.includes(account.id) || state.draft.rows.some((row) => row.id === entry.id)) return
  state.draft.rows.push(newBindRow(entry.id, ''))
}

watch(
  () => [step.value, channelState.value ? channelDraftChanges(channelState.value) : 0] as const,
  ([currentStep, changes]) => {
    if (currentStep === 3 && changes === 0 && submitError.value === t('admin.accounts.dialog.bind.unsaved')) submitError.value = ''
  }
)

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
      submitError.value = t('admin.accounts.vertexSaJsonMissingFields')
      return false
    }
    vertexServiceAccountJson.value = JSON.stringify(parsed)
    return true
  } catch {
    submitError.value = t('admin.accounts.vertexSaJsonInvalid')
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
  submitError.value = ''
  // For OAuth-based type, handle OAuth flow (goes to step 2)
  if (isOAuthFlow.value) {
    if (!isGrokSSOInputMethod.value && !form.name.trim()) {
      submitError.value = t('admin.accounts.pleaseEnterAccountName')
      return
    }
    step.value = 2
    return
  }

  // For Bedrock type, create directly
  if (form.platform === 'anthropic' && accountCategory.value === 'bedrock') {
    if (!form.name.trim()) {
      submitError.value = t('admin.accounts.pleaseEnterAccountName')
      return
    }

    const credentials: Record<string, unknown> = {
      auth_mode: bedrockAuthMode.value,
      aws_region: bedrockRegion.value.trim() || 'us-east-1',
    }

    if (bedrockAuthMode.value === 'sigv4') {
      if (!bedrockAccessKeyId.value.trim()) {
        submitError.value = t('admin.accounts.bedrockAccessKeyIdRequired')
        return
      }
      if (!bedrockSecretAccessKey.value.trim()) {
        submitError.value = t('admin.accounts.bedrockSecretAccessKeyRequired')
        return
      }
      credentials.aws_access_key_id = bedrockAccessKeyId.value.trim()
      credentials.aws_secret_access_key = bedrockSecretAccessKey.value.trim()
    } else {
      if (!bedrockApiKeyValue.value.trim()) {
        submitError.value = t('admin.accounts.bedrockApiKeyRequired')
        return
      }
      credentials.api_key = bedrockApiKeyValue.value.trim()
    }

    if (bedrockForceGlobal.value) {
      credentials.aws_force_global = 'true'
    }

    applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')

    await createAccountAndFinish('anthropic', 'bedrock' as AccountType, credentials)
    return
  }

  if ((form.platform === 'gemini' || form.platform === 'anthropic') && accountCategory.value === 'service_account') {
    if (!form.name.trim()) {
      submitError.value = t('admin.accounts.pleaseEnterAccountName')
      return
    }
    if (!parseVertexServiceAccountJson()) {
      return
    }
    if (!vertexLocation.value.trim()) {
      submitError.value = t('admin.accounts.vertexLocationRequired')
      return
    }
    const credentials: Record<string, unknown> = {
      service_account_json: vertexServiceAccountJson.value.trim(),
      location: vertexLocation.value.trim(),
      tier_id: 'vertex'
    }
    // Vertex · Claude 也有「拦截预热请求」开关（anthropic 平台都有），之前这里漏写了
    applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')
    await createAccountAndFinish(form.platform, 'service_account' as AccountType, credentials)
    return
  }

  // For apikey type, create directly
  if (!apiKeyValue.value.trim()) {
    submitError.value = t('admin.accounts.pleaseEnterApiKey')
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

  // 池模式：同渠道重试次数与状态码写死在后端（channel_features.go），这里只写开关
  if (poolModeEnabled.value) {
    credentials.pool_mode = true
  }

  // 请求头覆写对任何第三方 key 开放
  if (!applyKeyHeaderOverride(credentials)) {
    return
  }

  applyInterceptWarmup(credentials, interceptWarmupRequests.value, 'create')

  form.credentials = credentials
  const extra = buildAnthropicExtra()

  await doCreateAccount({
    ...form,
    protocol_endpoints: apiKeyEndpoints,
    extra: withQuotaExtra(extra)
  })
}

const goBackToBasicInfo = () => {
  step.value = 1
  submitError.value = ''
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
      geminiOAuthType.value
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


// 限额（日 / 周 / 总）写进 extra（重置方式固定滚动、提醒固定用到 80% 发一次，都不用写）。
// 第三方 key 的提交分支和 Bedrock / Vertex 共用，任何一条漏调，界面上填的限额就会被静默丢掉。
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
  // Inject quota limits for apikey/bedrock accounts
  let finalExtra = extra
  if (type === 'apikey' || type === 'bedrock') {
    finalExtra = withQuotaExtra(finalExtra)
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
    priority: form.priority,
    expires_at: form.expires_at
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

        await createAccountRecord({
          name: accountName,
          notes: form.notes,
          platform: 'grok',
          type: 'oauth',
          credentials,
          extra,
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          priority: form.priority,
          expires_at: form.expires_at
        })
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = extractApiErrorMessage(error, 'Unknown error')
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    if (successCount > 0 && failedCount === 0) {
      finishCreated()
    } else if (successCount > 0) {
      grokOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = errors.join('\n')
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

  try {
    const result = await adminAPI.grok.createFromSSO({
      sso_tokens: ssoTokens,
      name: form.name || undefined,
      notes: form.notes || undefined,
      proxy_id: form.proxy_id,
      credentials,
      concurrency: form.concurrency,
      priority: form.priority,
      expires_at: form.expires_at
    })

    const successCount = result.created?.length || 0
    const failedCount = result.failed?.length || 0
    if (successCount > 0 && failedCount === 0) {
      finishCreated()
    } else if (successCount > 0 && failedCount > 0) {
      // Same as OpenAI/Grok RT: keep input, show failures, refresh list.
      grokOAuth.error.value = (result.failed || [])
        .map((item) => `#${item.index}: ${item.error || 'Unknown error'}`)
        .join('\n')
      emit('created')
    } else {
      grokOAuth.error.value = (result.failed || [])
        .map((item) => `#${item.index}: ${item.error || 'Unknown error'}`)
        .join('\n') || t('admin.accounts.oauth.grok.failedToConvertSSO')
    }
  } catch (error: any) {
    grokOAuth.error.value = extractApiErrorMessage(error, t('admin.accounts.oauth.grok.failedToConvertSSO'))
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
    const extra = oauthClient.buildExtraInfo(tokenInfo) as Record<string, unknown> | undefined
    const shouldCreateOpenAI = form.platform === 'openai'

    if (shouldCreateOpenAI) {
      await createAccountRecord({
        name: form.name,
        notes: form.notes,
        platform: 'openai',
        type: 'oauth',
        credentials,
        extra,
        proxy_id: form.proxy_id,
        concurrency: form.concurrency,
        priority: form.priority,
        expires_at: form.expires_at
      })
    }

    finishCreated()
  } catch (error: any) {
    oauthClient.error.value = extractApiErrorMessage(error, t('admin.accounts.oauth.authFailed'))
  } finally {
    oauthClient.loading.value = false
  }
}

// OpenAI 手动 RT 批量验证和创建
// OpenAI Mobile RT client_id
const OPENAI_MOBILE_RT_CLIENT_ID = 'app_LlGpXReQgckcGGUo2JrYvtJK'

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

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  try {
    const result = await adminAPI.accounts.importCodexSession({
      content: trimmed,
      name: form.name,
      notes: form.notes || null,
      proxy_id: form.proxy_id,
      concurrency: form.concurrency,
      priority: form.priority,
      expires_at: form.expires_at,
      update_existing: true
    })

    const successCount = result.created + result.updated

    if (successCount > 0 && result.failed === 0) {
      finishCreated()
      return
    }

    const errorText = formatCodexImportMessages(result.errors)
    const warningText = formatCodexImportMessages(result.warnings)
    oauthClient.error.value = [errorText, warningText].filter(Boolean).join('\n')

    if (result.failed === 0) return

    if (successCount > 0) {
      emit('created')
      return
    }

    if (!oauthClient.error.value) oauthClient.error.value = t('admin.accounts.oauth.openai.codexSessionImportFailed')
  } catch (error: any) {
    oauthClient.error.value =
      extractApiErrorMessage(error, t('admin.accounts.oauth.openai.codexSessionImportFailed'))
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

  oauthClient.loading.value = true
  oauthClient.error.value = ''

  try {
    await adminAPI.accounts.createOpenAICodexPAT({
      access_token: trimmed,
      name: form.name,
      notes: form.notes || null,
      proxy_id: form.proxy_id,
      concurrency: form.concurrency,
      priority: form.priority,
      expires_at: form.expires_at
    })

    finishCreated()
  } catch (error: any) {
    oauthClient.error.value =
      extractApiErrorMessage(error, t('admin.accounts.oauth.openai.codexPatImportFailed'))
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
        const extra = oauthClient.buildExtraInfo(tokenInfo) as Record<string, unknown> | undefined

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
            extra,
            proxy_id: form.proxy_id,
            concurrency: form.concurrency,
            priority: form.priority,
            expires_at: form.expires_at
          })
        }

        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = extractApiErrorMessage(error, 'Unknown error')
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    // Show results
    if (successCount > 0 && failedCount === 0) {
      finishCreated()
    } else if (successCount > 0 && failedCount > 0) {
      oauthClient.error.value = errors.join('\n')
      emit('created')
    } else {
      oauthClient.error.value = errors.join('\n')
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
          extra: {},
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          priority: form.priority,
          expires_at: form.expires_at
        }
        await createAccountRecord(createPayload)
        successCount++
      } catch (error: any) {
        failedCount++
        const errMsg = extractApiErrorMessage(error, 'Unknown error')
        errors.push(`#${i + 1}: ${errMsg}`)
      }
    }

    // Show results
    if (successCount > 0 && failedCount === 0) {
      finishCreated()
    } else if (successCount > 0 && failedCount > 0) {
      antigravityOAuth.error.value = errors.join('\n')
      emit('created')
    } else {
      antigravityOAuth.error.value = errors.join('\n')
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
      return
    }

    const tokenInfo = await geminiOAuth.exchangeAuthCode({
      code: authCode.trim(),
      sessionId: geminiOAuth.sessionId.value,
      state: stateToUse,
      proxyId: form.proxy_id,
      oauthType: geminiOAuthType.value
    })
    if (!tokenInfo) return

    const credentials = geminiOAuth.buildCredentials(tokenInfo)
    const extra = geminiOAuth.buildExtraInfo(tokenInfo)
    await createAccountAndFinish('gemini', 'oauth', credentials, extra)
  } catch (error: any) {
    geminiOAuth.error.value = extractApiErrorMessage(error, t('admin.accounts.oauth.authFailed'))
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
		const extra = buildAntigravityExtra()
		await createAccountAndFinish('antigravity', 'oauth', credentials, extra)
  } catch (error: any) {
    antigravityOAuth.error.value = extractApiErrorMessage(error, t('admin.accounts.oauth.authFailed'))
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
    grokOAuth.error.value = extractApiErrorMessage(error, t('admin.accounts.oauth.authFailed'))
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
    oauth.error.value = extractApiErrorMessage(error, t('admin.accounts.oauth.authFailed'))
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

        await createAccountRecord({
          name: accountName,
          notes: form.notes,
          platform: form.platform,
          type: addMethod.value, // Use addMethod as type: 'oauth' or 'setup-token'
          credentials,
          extra,
          proxy_id: form.proxy_id,
          concurrency: form.concurrency,
          priority: form.priority,
          expires_at: form.expires_at
        })

        successCount++
      } catch (error: any) {
        failedCount++
        errors.push(
          t('admin.accounts.oauth.keyAuthFailed', {
            index: i + 1,
            error: extractApiErrorMessage(error, t('admin.accounts.oauth.authFailed'))
          })
        )
      }
    }

    if (successCount > 0) {
      if (failedCount === 0) {
        finishCreated()
      } else {
        emit('created')
      }
    }

    if (failedCount > 0) {
      oauth.error.value = errors.join('\n')
    }
  } catch (error: any) {
    oauth.error.value = extractApiErrorMessage(error, t('admin.accounts.oauth.cookieAuthFailed'))
  } finally {
    oauth.loading.value = false
  }
}
</script>
