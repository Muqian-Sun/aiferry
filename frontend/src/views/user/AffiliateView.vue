<template>
  <!--
    邀请：指标行 → 邀请码 / 链接（mono + 复制）与规则 → 转入余额（页面唯一主按钮）→ 被邀请人表。
    区块之间只用 hairline。
  -->
  <div class="space-y-8">
    <StatusState v-if="loading" kind="loading" :title="t('userUi.status.loading')" />

    <template v-else-if="detail">
      <StatRow :items="stats" />

      <SheetSection :title="t('affiliate.title')" :description="t('affiliate.description')">
        <div class="grid gap-6 md:grid-cols-2">
          <div class="space-y-2">
            <p class="text-13 font-medium text-af-ink-2">{{ t('affiliate.yourCode') }}</p>
            <div class="flex flex-col items-stretch gap-2 rounded-md border border-af-hairline bg-af-sunken px-3 py-2 sm:flex-row sm:items-center">
              <code class="min-w-0 break-all font-mono text-sm font-medium text-af-ink sm:flex-1 sm:truncate">{{ detail.aff_code }}</code>
              <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyCode">
                <Icon name="copy" size="sm" />
                <span>{{ t('affiliate.copyCode') }}</span>
              </button>
            </div>
          </div>

          <div class="space-y-2">
            <p class="text-13 font-medium text-af-ink-2">{{ t('affiliate.inviteLink') }}</p>
            <div class="flex flex-col items-stretch gap-2 rounded-md border border-af-hairline bg-af-sunken px-3 py-2 sm:flex-row sm:items-center">
              <code class="min-w-0 break-all font-mono text-sm text-af-ink-2 sm:flex-1 sm:truncate">{{ inviteLink }}</code>
              <button class="btn btn-secondary btn-sm w-full sm:w-auto sm:shrink-0" @click="copyInviteLink">
                <Icon name="copy" size="sm" />
                <span>{{ t('affiliate.copyLink') }}</span>
              </button>
            </div>
          </div>
        </div>

        <ol class="mt-5 list-decimal space-y-1 pl-5 text-13 leading-5 text-af-ink-3">
          <li>{{ t('affiliate.tips.line1') }}</li>
          <li>{{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
          <li>{{ t('affiliate.tips.line3') }}</li>
          <li v-if="detail.aff_frozen_quota > 0">{{ t('affiliate.tips.line4') }}</li>
        </ol>
      </SheetSection>

      <SheetSection :title="t('affiliate.transfer.title')" :description="t('affiliate.transfer.description')">
        <template #actions>
          <button
            class="btn btn-primary btn-md"
            :disabled="transferring || detail.aff_quota <= 0"
            @click="transferQuota"
          >
            {{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}
          </button>
        </template>
        <p v-if="detail.aff_quota <= 0" class="text-13 text-af-ink-3">
          {{ t('affiliate.transfer.empty') }}
        </p>
      </SheetSection>

      <SheetSection :title="t('affiliate.invitees.title')">
        <StatusState v-if="detail.invitees.length === 0" kind="empty" :title="t('affiliate.invitees.empty')" />
        <div v-else class="-mx-6 overflow-x-auto">
          <table class="w-full min-w-[560px] text-left text-13">
            <thead>
              <tr class="border-b border-af-hairline text-af-ink-3">
                <th class="py-2 pl-6 pr-4 font-medium">{{ t('affiliate.invitees.columns.email') }}</th>
                <th class="py-2 pr-4 font-medium">{{ t('affiliate.invitees.columns.username') }}</th>
                <th class="py-2 pr-4 text-right font-medium">{{ t('affiliate.invitees.columns.rebate') }}</th>
                <th class="py-2 pr-6 font-medium">{{ t('affiliate.invitees.columns.joinedAt') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-af-hairline">
              <tr v-for="item in detail.invitees" :key="item.user_id" class="h-11 hover:bg-af-sunken">
                <td class="pl-6 pr-4 text-af-ink">{{ item.email || '-' }}</td>
                <td class="pr-4 text-af-ink-2">{{ item.username || '-' }}</td>
                <td class="pr-4 text-right font-medium tabular-nums text-af-ink">{{ formatCurrency(item.total_rebate) }}</td>
                <td class="pr-6 tabular-nums text-af-ink-2">{{ formatDateTime(item.created_at) || '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </SheetSection>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { StatItem } from '@/components/user/shell/types'
import userAPI from '@/api/user'
import type { UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

function formatCount(value: number): string {
  return value.toLocaleString()
}

const stats = computed<StatItem[]>(() => {
  const d = detail.value
  if (!d) return []
  return [
    { key: 'rebate-rate', label: t('affiliate.stats.rebateRate'), value: `${formattedRebateRate.value}%`, hint: t('affiliate.stats.rebateRateHint') },
    { key: 'invited', label: t('affiliate.stats.invitedUsers'), value: formatCount(d.aff_count) },
    { key: 'available', label: t('affiliate.stats.availableQuota'), value: formatCurrency(d.aff_quota) },
    {
      key: 'total',
      label: t('affiliate.stats.totalQuota'),
      value: formatCurrency(d.aff_history_quota),
      hint: d.aff_frozen_quota > 0 ? `${t('affiliate.stats.frozenQuota')} ${formatCurrency(d.aff_frozen_quota)}` : undefined
    }
  ]
})

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

onMounted(() => {
  void loadAffiliateDetail()
})
</script>
