<template>
  <!-- 调度与限额的公共字段：优先级、并发、到期时间（新建 / 编辑同一套） -->
  <div class="space-y-4">
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <div>
        <label class="input-label">{{ t('admin.accounts.priority') }}</label>
        <input
          :value="priority"
          type="number"
          min="1"
          class="input"
          data-testid="channel-priority"
          @input="emit('update:priority', toPositiveInt(($event.target as HTMLInputElement).value))"
        />
        <p class="input-hint">{{ t('admin.accounts.priorityHint') }}</p>
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.concurrency') }}</label>
        <input
          :value="concurrency"
          type="number"
          min="1"
          class="input"
          data-testid="channel-concurrency"
          @input="emit('update:concurrency', toPositiveInt(($event.target as HTMLInputElement).value))"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.expiresAt') }}</label>
        <input v-model="expiresAtInput" type="datetime-local" class="input" data-testid="channel-expires-at" />
        <div class="mt-2 flex gap-2">
          <button type="button" class="btn btn-secondary btn-sm" @click="emit('update:expiresAt', getAccountExpiryTimestamp(1))">
            {{ t('payment.oneMonth') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="emit('update:expiresAt', getAccountExpiryTimestamp(12))">
            {{ t('payment.oneYear') }}
          </button>
        </div>
      </div>
    </div>
    <p class="input-hint">
      {{ t('admin.accounts.expiresAtHint') }}
      {{ t('admin.accounts.expiresAtTimezoneHint', { timezone: browserTimeZone }) }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTimeLocalInput, getBrowserTimeZone, parseDateTimeLocalInput } from '@/utils/format'
import { getAccountExpiryTimestamp } from '@/components/account/accountExpiry'

const props = defineProps<{
  priority: number
  concurrency: number
  /** 到期时间（Unix 秒），null = 不过期 */
  expiresAt: number | null
}>()

const emit = defineEmits<{
  'update:priority': [value: number]
  'update:concurrency': [value: number]
  'update:expiresAt': [value: number | null]
}>()

const { t } = useI18n()
const browserTimeZone = getBrowserTimeZone()

const expiresAtInput = computed({
  get: () => formatDateTimeLocalInput(props.expiresAt),
  set: (value: string) => emit('update:expiresAt', parseDateTimeLocalInput(value))
})

// 优先级、并发都是 >= 1 的整数；清空或乱填按 1
function toPositiveInt(raw: string): number {
  const value = Math.floor(Number(raw))
  return Number.isFinite(value) && value >= 1 ? value : 1
}
</script>
