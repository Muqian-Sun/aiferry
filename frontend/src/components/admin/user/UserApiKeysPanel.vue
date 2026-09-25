<template>
  <!-- 用户的 API 密钥（A5，只读）：名称、状态、掩码后的 key、创建 / 最近使用、额度。用户详情抽屉的「API 密钥」页签。 -->
  <div data-testid="user-api-keys">
    <StatusState v-if="loading" kind="loading" :title="t('common.loading')" />
    <StatusState
      v-else-if="failed"
      kind="error"
      :title="t('admin.users.failedToLoadApiKeys')"
      :action-label="t('admin.users.detail.retry')"
      @action="load"
    />
    <StatusState v-else-if="apiKeys.length === 0" kind="empty" :title="t('admin.users.noApiKeys')" />
    <ul v-else class="divide-y divide-af-hairline">
      <li v-for="key in apiKeys" :key="key.id" class="py-3 first:pt-0" data-testid="user-api-key-row">
        <div class="flex items-baseline justify-between gap-4">
          <div class="flex min-w-0 items-baseline gap-2">
            <span class="truncate font-medium text-af-ink">{{ key.name }}</span>
            <span v-if="key.subscription_id" class="shrink-0 text-xs text-af-ink-3" data-testid="user-api-key-subscription">
              {{ t('admin.users.subscriptionKey', { plan: key.subscription_plan_name || '' }) }}
            </span>
          </div>
          <span class="shrink-0 text-xs" :class="keyStatusClass(key.status)">{{ keyStatusLabel(key.status) }}</span>
        </div>
        <p class="mt-1 truncate font-mono text-xs text-af-ink-3">{{ maskKey(key.key) }}</p>
        <p class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-xs tabular-nums text-af-ink-3">
          <span :title="formatDateTime(key.created_at)">{{ t('admin.users.detail.keyCreated', { date: formatDateOnly(key.created_at) }) }}</span>
          <span v-if="key.last_used_at" :title="formatDateTime(key.last_used_at)">
            {{ t('admin.users.detail.keyLastUsed', { time: formatRelativeTime(key.last_used_at) }) }}
          </span>
          <span v-else>{{ t('admin.users.detail.keyNeverUsed') }}</span>
          <span v-if="key.quota > 0">{{ t('admin.users.detail.keyQuota', { used: formatMoney(key.quota_used), quota: formatMoney(key.quota) }) }}</span>
          <span v-if="key.expires_at" :title="formatDateTime(key.expires_at)">
            {{ t('admin.users.detail.keyExpires', { date: formatDateOnly(key.expires_at) }) }}
          </span>
        </p>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { formatDateOnly, formatDateTime, formatRelativeTime } from '@/utils/format'
import { formatMoney } from '@/utils/money'
import type { ApiKey } from '@/types'
import StatusState from '@/components/user/shell/StatusState.vue'

const props = withDefaults(defineProps<{ userId: number; refreshKey?: number }>(), { refreshKey: 0 })
const { t, te } = useI18n()

const apiKeys = ref<ApiKey[]>([])
const loading = ref(false)
const failed = ref(false)
let requestSeq = 0

async function load() {
  const seq = ++requestSeq
  loading.value = true
  failed.value = false
  try {
    const res = await adminAPI.users.getUserApiKeys(props.userId)
    if (seq !== requestSeq) return
    apiKeys.value = res.items || []
  } catch (error) {
    if (seq !== requestSeq) return
    failed.value = true
    console.error('Failed to load API keys:', error)
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

watch(() => [props.userId, props.refreshKey], load, { immediate: true })

function maskKey(key: string): string {
  if (!key) return ''
  return key.length > 28 ? `${key.substring(0, 20)}…${key.substring(key.length - 8)}` : key
}

// 启用 / 禁用和用户状态用同一对词（用户站密钥页把 active 叫「活跃」，在这页会和「最近活跃」= 调用过 API 混）
function keyStatusLabel(status: string): string {
  if (status === 'active') return t('common.active')
  if (status === 'inactive') return t('common.inactive')
  const key = `keys.status.${status}`
  return te(key) ? t(key) : status
}

// 额度耗尽 / 过期是要处理的状态，标橙；启用 / 停用都是常态
function keyStatusClass(status: string): string {
  return status === 'quota_exhausted' || status === 'expired' ? 'text-af-warning' : 'text-af-ink-3'
}
</script>
