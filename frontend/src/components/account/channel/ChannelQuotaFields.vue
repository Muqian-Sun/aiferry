<template>
  <!-- 渠道限额（第三方 key、Bedrock）：日 / 周 / 总，按渠道成本累计，任一维度到限额就暂停调度 -->
  <div class="space-y-3" data-testid="channel-quota">
    <div>
      <h3 class="input-label mb-0">{{ t('admin.accounts.quotaControl.title') }}</h3>
      <p class="mt-1 text-xs text-af-ink-3">{{ t('admin.accounts.quotaLimitHint') }}</p>
    </div>
    <QuotaLimitCard
      :totalLimit="totalLimit"
      :dailyLimit="dailyLimit"
      :weeklyLimit="weeklyLimit"
      @update:totalLimit="emit('update:totalLimit', $event)"
      @update:dailyLimit="emit('update:dailyLimit', $event)"
      @update:weeklyLimit="emit('update:weeklyLimit', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import QuotaLimitCard from '@/components/account/QuotaLimitCard.vue'

defineProps<{
  totalLimit: number | null
  dailyLimit: number | null
  weeklyLimit: number | null
}>()

const emit = defineEmits<{
  'update:totalLimit': [value: number | null]
  'update:dailyLimit': [value: number | null]
  'update:weeklyLimit': [value: number | null]
}>()

const { t } = useI18n()
</script>
