<template>
  <!-- 第三方 key 的 Anthropic 协议设置（配了 anthropic 协议地址才出现）：认证头写法、Bedrock CC 兼容 -->
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <label class="input-label mb-0">{{ t('admin.accounts.anthropic.apiKeyAuthScheme') }}</label>
        <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.accounts.anthropic.apiKeyAuthSchemeDesc') }}</p>
      </div>
      <select
        :value="authScheme"
        :data-testid="`${testIdPrefix}-anthropic-auth-scheme`"
        class="input w-52 text-sm"
        @change="emit('update:authScheme', ($event.target as HTMLSelectElement).value as AnthropicAPIKeyAuthScheme)"
      >
        <option value="x_api_key">{{ t('admin.accounts.anthropic.apiKeyAuthSchemeXApiKey') }}</option>
        <option value="authorization_bearer">{{ t('admin.accounts.anthropic.apiKeyAuthSchemeBearer') }}</option>
      </select>
    </div>
    <ChannelSettingToggle
      :model-value="bedrockCcCompat"
      :label="t('admin.accounts.anthropic.bedrockCCCompat')"
      :description="t('admin.accounts.anthropic.bedrockCCCompatDesc')"
      :test-id="`${testIdPrefix}-bedrock-cc-compat`"
      @update:model-value="emit('update:bedrockCcCompat', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import ChannelSettingToggle from './ChannelSettingToggle.vue'

export type AnthropicAPIKeyAuthScheme = 'x_api_key' | 'authorization_bearer'

defineProps<{
  authScheme: AnthropicAPIKeyAuthScheme
  bedrockCcCompat: boolean
  /** testid 前缀：create / edit */
  testIdPrefix: string
}>()

const emit = defineEmits<{
  'update:authScheme': [value: AnthropicAPIKeyAuthScheme]
  'update:bedrockCcCompat': [value: boolean]
}>()

const { t } = useI18n()
</script>
