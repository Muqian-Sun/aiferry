<template>
  <!-- Claude 成品号（OAuth / Setup Token）的会话数上限与 RPM：开关打开才填数值 -->
  <div class="space-y-4" data-testid="anthropic-subscription-limits">
    <ChannelSettingToggle
      :model-value="sessionLimitEnabled"
      :label="t('admin.accounts.quotaControl.sessionLimit.label')"
      :description="t('admin.accounts.quotaControl.sessionLimit.hint')"
      test-id="session-limit"
      @update:model-value="emit('update:sessionLimitEnabled', $event)"
    >
      <label class="input-label">{{ t('admin.accounts.quotaControl.sessionLimit.maxSessions') }}</label>
      <input
        :value="maxSessions ?? ''"
        type="number"
        min="1"
        step="1"
        class="input"
        :placeholder="t('admin.accounts.quotaControl.sessionLimit.maxSessionsPlaceholder')"
        @input="emit('update:maxSessions', toNumberOrNull(($event.target as HTMLInputElement).value))"
      />
      <p class="input-hint">{{ t('admin.accounts.quotaControl.sessionLimit.maxSessionsHint') }}</p>
    </ChannelSettingToggle>
    <ChannelSettingToggle
      :model-value="rpmLimitEnabled"
      :label="t('admin.accounts.quotaControl.rpmLimit.label')"
      :description="t('admin.accounts.quotaControl.rpmLimit.hint')"
      test-id="rpm-limit"
      @update:model-value="emit('update:rpmLimitEnabled', $event)"
    >
      <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpm') }}</label>
      <input
        :value="baseRpm ?? ''"
        type="number"
        min="1"
        max="1000"
        step="1"
        class="input"
        :placeholder="t('admin.accounts.quotaControl.rpmLimit.baseRpmPlaceholder')"
        @input="emit('update:baseRpm', toNumberOrNull(($event.target as HTMLInputElement).value))"
      />
      <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpmHint') }}</p>
    </ChannelSettingToggle>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import ChannelSettingToggle from './ChannelSettingToggle.vue'

defineProps<{
  sessionLimitEnabled: boolean
  maxSessions: number | null
  rpmLimitEnabled: boolean
  baseRpm: number | null
}>()

const emit = defineEmits<{
  'update:sessionLimitEnabled': [value: boolean]
  'update:maxSessions': [value: number | null]
  'update:rpmLimitEnabled': [value: boolean]
  'update:baseRpm': [value: number | null]
}>()

const { t } = useI18n()

function toNumberOrNull(raw: string): number | null {
  if (raw.trim() === '') return null
  const value = Number(raw)
  return Number.isFinite(value) ? value : null
}
</script>
