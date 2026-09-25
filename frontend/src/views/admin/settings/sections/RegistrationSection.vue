<template>
  <div class="space-y-6">
    <!-- Registration Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.registration.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.registration.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <!-- Email Verification -->
        <div
          class="flex items-center justify-between border-t border-af-hairline pt-4"
        >
          <div>
            <label class="font-medium text-af-ink">{{
              t("admin.settings.registration.emailVerification")
            }}</label>
            <p class="text-sm text-af-ink-3">
              {{ t("admin.settings.registration.emailVerificationHint") }}
            </p>
          </div>
          <Toggle v-model="form.email_verify_enabled" />
        </div>

        <!-- Email Suffix Whitelist -->
        <div class="border-t border-af-hairline pt-4">
          <label class="font-medium text-af-ink">{{
            t("admin.settings.registration.emailSuffixWhitelist")
          }}</label>
          <p class="mt-1 text-sm text-af-ink-3">
            {{
              t("admin.settings.registration.emailSuffixWhitelistHint")
            }}
          </p>
          <div
            class="mt-3 rounded-lg border border-af-hairline-strong bg-af-sheet p-2"
          >
            <div class="flex flex-wrap items-center gap-2">
              <span
                v-for="suffix in registrationEmailSuffixWhitelistTags"
                :key="suffix"
                class="inline-flex items-center gap-1 rounded bg-af-sunken px-2 py-1 text-xs font-mono text-af-ink-2"
              >
                <span>{{ suffix }}</span>
                <button
                  type="button"
                  class="rounded-full text-af-ink-3 hover:bg-af-hairline hover:text-af-ink-2"
                  @click="
                    removeRegistrationEmailSuffixWhitelistTag(suffix)
                  "
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
                  v-model="registrationEmailSuffixWhitelistDraft"
                  type="text"
                  class="w-full bg-transparent text-sm font-mono text-af-ink outline-none placeholder:text-af-ink-3"
                  :placeholder="
                    t(
                      'admin.settings.registration.emailSuffixWhitelistPlaceholder',
                    )
                  "
                  @input="
                    handleRegistrationEmailSuffixWhitelistDraftInput
                  "
                  @keydown="
                    handleRegistrationEmailSuffixWhitelistDraftKeydown
                  "
                  @blur="commitRegistrationEmailSuffixWhitelistDraft"
                  @paste="handleRegistrationEmailSuffixWhitelistPaste"
                />
              </div>
            </div>
          </div>
          <p class="mt-2 text-xs text-af-ink-3">
            {{
              t(
                "admin.settings.registration.emailSuffixWhitelistInputHint",
              )
            }}
          </p>
        </div>

        <!-- Email Domain Quota -->
        <div
          class="flex items-center justify-between border-t border-af-hairline pt-4"
        >
          <div>
            <label class="font-medium text-af-ink">{{
              t("admin.settings.registration.emailDomainQuota")
            }}</label>
            <p class="text-sm text-af-ink-3">
              {{ t("admin.settings.registration.emailDomainQuotaHint") }}
            </p>
          </div>
          <Toggle
            v-model="form.registration_email_domain_quota_enabled"
          />
        </div>

        <!-- Frontend URL - Only show when password reset is enabled -->
        <div
          v-if="form.email_verify_enabled"
          class="border-t border-af-hairline pt-4"
        >
          <label
            class="mb-2 block text-sm font-medium text-af-ink-2"
          >
            {{ t("admin.settings.registration.frontendUrl") }}
          </label>
          <input
            v-model="form.frontend_url"
            type="url"
            class="input"
            :placeholder="
              t('admin.settings.registration.frontendUrlPlaceholder')
            "
          />
          <p class="mt-1.5 text-xs text-af-ink-3">
            {{ t("admin.settings.registration.frontendUrlHint") }}
          </p>
        </div>

        <!-- 敏感操作 step-up 2FA -->
        <div
          class="flex items-center justify-between border-t border-af-hairline pt-4"
        >
          <div>
            <label class="font-medium text-af-ink">{{
              t("admin.settings.security.stepUp")
            }}</label>
            <p class="text-sm text-af-ink-3">
              {{ t("admin.settings.security.stepUpHint") }}
            </p>
          </div>
          <Toggle v-model="form.step_up_enabled" />
        </div>

      </div>
    </div>

    <!-- 人机验证 Settings -->
    <div class="card">
      <div
        class="border-b border-af-hairline px-6 py-4"
      >
        <h2 class="text-lg font-semibold text-af-ink">
          {{ t("admin.settings.captcha.title") }}
        </h2>
        <p class="mt-1 text-sm text-af-ink-3">
          {{ t("admin.settings.captcha.description") }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <!-- Enable Captcha -->
        <div class="flex items-center justify-between">
          <div>
            <label class="font-medium text-af-ink">{{
              t("admin.settings.captcha.enable")
            }}</label>
            <p class="text-sm text-af-ink-3">
              {{ t("admin.settings.captcha.enableHint") }}
            </p>
          </div>
          <Toggle
            v-model="captchaMasterEnabled"
            data-testid="captcha-enabled-toggle"
          />
        </div>

        <!-- Provider fields - Only show when enabled -->
        <div
          v-if="captchaMasterEnabled"
          class="border-t border-af-hairline pt-4"
        >
          <!-- Provider Selector -->
          <div class="mb-6">
            <label
              class="mb-2 block text-sm font-medium text-af-ink-2"
            >
              {{ t("admin.settings.captcha.provider") }}
            </label>
            <div
              class="grid grid-cols-3 gap-2 rounded-lg bg-af-sunken p-1"
            >
              <button
                type="button"
                data-testid="captcha-provider-turnstile"
                class="inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
                :class="
                  captchaProviderSelection === 'turnstile'
                    ? 'bg-af-sheet text-af-brand'
                    : 'text-af-ink-2 hover:text-af-ink'
                "
                @click="selectCaptchaProvider('turnstile')"
              >
                {{ t("admin.settings.captcha.providerTurnstile") }}
              </button>
              <button
                type="button"
                data-testid="captcha-provider-tencent"
                class="inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
                :class="
                  captchaProviderSelection === 'tencent'
                    ? 'bg-af-sheet text-af-brand'
                    : 'text-af-ink-2 hover:text-af-ink'
                "
                @click="selectCaptchaProvider('tencent')"
              >
                {{ t("admin.settings.captcha.providerTencent") }}
              </button>
              <button
                type="button"
                data-testid="captcha-provider-aliyun"
                class="inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition"
                :class="
                  captchaProviderSelection === 'aliyun'
                    ? 'bg-af-sheet text-af-brand'
                    : 'text-af-ink-2 hover:text-af-ink'
                "
                @click="selectCaptchaProvider('aliyun')"
              >
                {{ t("admin.settings.captcha.providerAliyun") }}
              </button>
            </div>
          </div>

          <!-- Cloudflare Turnstile fields -->
          <div
            v-if="captchaProviderSelection === 'turnstile'"
            class="grid grid-cols-1 gap-6"
          >
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.turnstile.siteKey") }}
              </label>
              <input
                v-model="form.turnstile_site_key"
                type="text"
                class="input font-mono text-sm"
                placeholder="0x4AAAAAAA..."
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{ t("admin.settings.turnstile.siteKeyHint") }}
                <a
                  href="https://dash.cloudflare.com/"
                  target="_blank"
                  class="text-af-brand hover:text-af-brand-hover"
                  >{{
                    t("admin.settings.turnstile.cloudflareDashboard")
                  }}</a
                >
              </p>
            </div>
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.turnstile.secretKey") }}
              </label>
              <input
                v-model="form.turnstile_secret_key"
                type="password"
                class="input font-mono text-sm"
                placeholder="0x4AAAAAAA..."
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{
                  form.turnstile_secret_key_configured
                    ? t(
                        "admin.settings.turnstile.secretKeyConfiguredHint",
                      )
                    : t("admin.settings.turnstile.secretKeyHint")
                }}
              </p>
            </div>
          </div>

          <!-- Tencent Captcha fields -->
          <div v-else-if="captchaProviderSelection === 'tencent'">
            <div class="mb-6 max-w-sm">
              <label class="mb-2 block text-sm font-medium text-af-ink-2">
                {{ t("admin.settings.tencentCaptcha.region") }}
              </label>
              <div class="grid grid-cols-2 gap-2 rounded-lg bg-af-sunken p-1">
                <button
                  type="button"
                  data-testid="tencent-captcha-region-cn"
                  class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition"
                  :class="
                    form.tencent_captcha_region !== 'intl'
                      ? 'bg-af-sheet text-af-brand'
                      : 'text-af-ink-2 hover:text-af-ink'
                  "
                  @click="form.tencent_captcha_region = 'cn'"
                >
                  {{ t("admin.settings.tencentCaptcha.regionCn") }}
                </button>
                <button
                  type="button"
                  data-testid="tencent-captcha-region-intl"
                  class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition"
                  :class="
                    form.tencent_captcha_region === 'intl'
                      ? 'bg-af-sheet text-af-brand'
                      : 'text-af-ink-2 hover:text-af-ink'
                  "
                  @click="form.tencent_captcha_region = 'intl'"
                >
                  {{ t("admin.settings.tencentCaptcha.regionIntl") }}
                </button>
              </div>
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{ t("admin.settings.tencentCaptcha.regionHint") }}
              </p>
            </div>
            <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
              <div class="md:col-span-2">
                <h3 class="text-sm font-semibold text-af-ink">
                  {{ t("admin.settings.tencentCaptcha.appCredentialsTitle") }}
                </h3>
                <p class="mt-1 text-xs text-af-ink-3">
                  {{ t("admin.settings.tencentCaptcha.appCredentialsHint") }}
                </p>
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-af-ink-2">
                  {{ t("admin.settings.tencentCaptcha.appId") }}
                </label>
                <input
                  v-model="form.tencent_captcha_app_id"
                  type="text"
                  inputmode="numeric"
                  class="input font-mono text-sm"
                  placeholder="123456789"
                />
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-af-ink-2">
                  {{ t("admin.settings.tencentCaptcha.appSecretKey") }}
                </label>
                <input
                  v-model="form.tencent_captcha_app_secret_key"
                  type="password"
                  autocomplete="new-password"
                  class="input font-mono text-sm"
                  :placeholder="t('admin.settings.tencentCaptcha.keepExisting')"
                />
                <p class="mt-1.5 text-xs text-af-ink-3">
                  {{ form.tencent_captcha_app_secret_key_configured ? t("admin.settings.tencentCaptcha.configured") : t("admin.settings.tencentCaptcha.required") }}
                </p>
              </div>
              <div class="border-t border-af-hairline pt-5 md:col-span-2">
                <h3 class="text-sm font-semibold text-af-ink">
                  {{ t("admin.settings.tencentCaptcha.cloudCredentialsTitle") }}
                </h3>
                <p class="mt-1 text-xs text-af-ink-3">
                  {{ t("admin.settings.tencentCaptcha.cloudCredentialsHint") }}
                </p>
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-af-ink-2">
                  {{ t("admin.settings.tencentCaptcha.cloudSecretId") }}
                </label>
                <input
                  v-model="form.tencent_captcha_cloud_secret_id"
                  type="password"
                  autocomplete="new-password"
                  class="input font-mono text-sm"
                  :placeholder="t('admin.settings.tencentCaptcha.keepExisting')"
                />
                <p class="mt-1.5 text-xs text-af-ink-3">
                  {{ form.tencent_captcha_cloud_secret_id_configured ? t("admin.settings.tencentCaptcha.configured") : t("admin.settings.tencentCaptcha.required") }}
                </p>
              </div>
              <div>
                <label class="mb-2 block text-sm font-medium text-af-ink-2">
                  {{ t("admin.settings.tencentCaptcha.cloudSecretKey") }}
                </label>
                <input
                  v-model="form.tencent_captcha_cloud_secret_key"
                  type="password"
                  autocomplete="new-password"
                  class="input font-mono text-sm"
                  :placeholder="t('admin.settings.tencentCaptcha.keepExisting')"
                />
                <p class="mt-1.5 text-xs text-af-ink-3">
                  {{ form.tencent_captcha_cloud_secret_key_configured ? t("admin.settings.tencentCaptcha.configured") : t("admin.settings.tencentCaptcha.required") }}
                </p>
              </div>
            </div>
            <p class="mt-5 text-xs text-af-ink-3">
              {{ t("admin.settings.tencentCaptcha.camPermissionHint") }}
            </p>
            <p class="mt-2 text-xs text-af-ink-3">
              {{ t("admin.settings.tencentCaptcha.aidEncryptedHint") }}
            </p>
            <div class="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-sm">
              <a
                :href="tencentCaptchaLinks.console"
                target="_blank"
                rel="noopener noreferrer"
                class="text-af-brand hover:text-af-brand-hover"
              >
                {{ t("admin.settings.tencentCaptcha.openCaptchaConsole") }}
              </a>
              <a
                :href="tencentCaptchaLinks.cloudKeys"
                target="_blank"
                rel="noopener noreferrer"
                class="text-af-brand hover:text-af-brand-hover"
              >
                {{ t("admin.settings.tencentCaptcha.createCloudKeys") }}
              </a>
              <a
                :href="tencentCaptchaLinks.webDocs"
                target="_blank"
                rel="noopener noreferrer"
                class="text-af-brand hover:text-af-brand-hover"
              >
                {{ t("admin.settings.tencentCaptcha.openWebDocs") }}
              </a>
            </div>
          </div>

          <!-- Aliyun Captcha 2.0 fields -->
          <div v-else class="grid grid-cols-1 gap-6">
            <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
              <div>
                <label
                  class="mb-2 block text-sm font-medium text-af-ink-2"
                >
                  {{ t("admin.settings.aliyunCaptcha.region") }}
                </label>
                <div
                  class="grid grid-cols-2 gap-2 rounded-lg bg-af-sunken p-1"
                >
                  <button
                    type="button"
                    class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition"
                    :class="
                      form.aliyun_captcha_region !== 'sgp'
                        ? 'bg-af-sheet text-af-brand'
                        : 'text-af-ink-2 hover:text-af-ink'
                    "
                    @click="form.aliyun_captcha_region = 'cn'"
                  >
                    {{ t("admin.settings.aliyunCaptcha.regionCn") }}
                  </button>
                  <button
                    type="button"
                    class="inline-flex items-center justify-center rounded-md px-3 py-1.5 text-sm font-medium transition"
                    :class="
                      form.aliyun_captcha_region === 'sgp'
                        ? 'bg-af-sheet text-af-brand'
                        : 'text-af-ink-2 hover:text-af-ink'
                    "
                    @click="form.aliyun_captcha_region = 'sgp'"
                  >
                    {{ t("admin.settings.aliyunCaptcha.regionSgp") }}
                  </button>
                </div>
                <p class="mt-1.5 text-xs text-af-ink-3">
                  {{ t("admin.settings.aliyunCaptcha.regionHint") }}
                </p>
              </div>
              <div>
                <label
                  class="mb-2 block text-sm font-medium text-af-ink-2"
                >
                  {{ t("admin.settings.aliyunCaptcha.prefix") }}
                </label>
                <input
                  v-model="form.aliyun_captcha_prefix"
                  type="text"
                  class="input font-mono text-sm"
                  placeholder="14xxxxx"
                />
                <p class="mt-1.5 text-xs text-af-ink-3">
                  {{ t("admin.settings.aliyunCaptcha.prefixHint") }}
                </p>
              </div>
            </div>
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.aliyunCaptcha.sceneId") }}
              </label>
              <input
                v-model="form.aliyun_captcha_scene_id"
                type="text"
                class="input font-mono text-sm"
                placeholder="1cxxxxxx"
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{ t("admin.settings.aliyunCaptcha.sceneIdHint") }}
              </p>
            </div>
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.aliyunCaptcha.accessKeyId") }}
              </label>
              <input
                v-model="form.aliyun_captcha_access_key_id"
                type="text"
                class="input font-mono text-sm"
                placeholder="LTAI..."
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{ t("admin.settings.aliyunCaptcha.accessKeyIdHint") }}
              </p>
            </div>
            <div>
              <label
                class="mb-2 block text-sm font-medium text-af-ink-2"
              >
                {{ t("admin.settings.aliyunCaptcha.accessKeySecret") }}
              </label>
              <input
                v-model="form.aliyun_captcha_access_key_secret"
                type="password"
                autocomplete="new-password"
                class="input font-mono text-sm"
                placeholder="••••••••"
              />
              <p class="mt-1.5 text-xs text-af-ink-3">
                {{
                  form.aliyun_captcha_access_key_secret_configured
                    ? t(
                        "admin.settings.aliyunCaptcha.accessKeySecretConfiguredHint",
                      )
                    : t("admin.settings.aliyunCaptcha.accessKeySecretHint")
                }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// 系统设置 › registration（A6 从 SettingsView 拆出，卡片模板原样搬来；状态与逻辑在 useSettingsPage）
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useSettingsPageContext } from '../useSettingsPage'

const {
  captchaMasterEnabled,
  captchaProviderSelection,
  commitRegistrationEmailSuffixWhitelistDraft,
  form,
  handleRegistrationEmailSuffixWhitelistDraftInput,
  handleRegistrationEmailSuffixWhitelistDraftKeydown,
  handleRegistrationEmailSuffixWhitelistPaste,
  registrationEmailSuffixWhitelistDraft,
  registrationEmailSuffixWhitelistTags,
  removeRegistrationEmailSuffixWhitelistTag,
  selectCaptchaProvider,
  t,
  tencentCaptchaLinks
} = useSettingsPageContext()
</script>
