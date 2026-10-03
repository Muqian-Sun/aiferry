<template>
  <!-- 按量 / Coding 套餐：地址分不出套餐时（MiniMax 两种套餐同地址、智谱 Anthropic 同地址）由管理员选 -->
  <div :data-testid="testId">
    <label class="input-label">{{ t('admin.accounts.cnProviders.accountMode.title') }}</label>
    <div class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2">
      <button
        v-for="mode in MODES"
        :key="mode.value"
        type="button"
        :data-testid="`${testId}-${mode.value}`"
        :class="[
          'flex items-center gap-3 rounded-lg border p-3 text-left transition-colors',
          modelValue === mode.value ? 'border-af-brand bg-af-brand-tint' : 'border-af-hairline hover:border-af-hairline-strong'
        ]"
        @click="emit('update:modelValue', mode.value)"
      >
        <span
          :class="[
            'flex h-8 w-8 shrink-0 items-center justify-center rounded-md',
            modelValue === mode.value ? 'bg-af-ink text-af-on-brand' : 'bg-af-sunken text-af-ink-3'
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
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CnAccountMode } from '@/components/account/credentialsBuilder'

defineProps<{ modelValue: CnAccountMode; testId: string }>()

const emit = defineEmits<{ 'update:modelValue': [value: CnAccountMode] }>()

const { t } = useI18n()

const MODES = [
  { value: 'payg', icon: 'creditCard' },
  { value: 'coding', icon: 'bolt' }
] as const
</script>
