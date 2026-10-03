<template>
  <!-- 请求头覆写（任何第三方 key 与 Grok 成品号）：转发时用配置值覆盖同名请求头 -->
  <div :data-testid="testId">
    <label class="input-label mb-0">{{ t('admin.accounts.headerOverride.title') }}</label>
    <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.accounts.headerOverride.hint') }}</p>
    <div class="mt-3 space-y-3">
      <div class="rounded-lg bg-af-sunken p-3">
        <p class="text-xs text-af-ink-2">
          <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
          {{ t('admin.accounts.headerOverride.info') }}
        </p>
      </div>
      <HeaderOverrideEditor :rows="rows" @update:rows="emit('update:rows', $event)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import HeaderOverrideEditor from '@/components/account/HeaderOverrideEditor.vue'
import type { HeaderOverrideRow } from '@/components/account/credentialsBuilder'

defineProps<{ rows: HeaderOverrideRow[]; testId?: string }>()

const emit = defineEmits<{ 'update:rows': [rows: HeaderOverrideRow[]] }>()

const { t } = useI18n()
</script>
