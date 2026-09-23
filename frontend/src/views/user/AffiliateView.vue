<template>
  <!--
    邀请返利（muqian 2026-09-23 两栏设置式，与兑换码同一套）：摘要带 → 「邀请」行（左：说明 + 规则；右：邀请码 / 邀请链接，
    与概览「快速开始」同样的一行一项 + 文字复制）→ 「转入余额」行（有额度才出页面唯一主按钮）→ 「已邀请用户」行。
    不再重复页头标题，不用灰底框和描边按钮。
  -->
  <div>
    <StatusState v-if="loading" kind="loading" :title="t('userUi.status.loading')" />

    <template v-else-if="detail">
      <StatRow :items="stats" class="pb-8" />

      <div class="divide-y divide-af-hairline border-t border-af-hairline">
        <SettingsRow :title="t('affiliate.inviteTitle')" :description="t('affiliate.tips.line1')">
          <template #aside>
            <ul class="mt-4 space-y-1.5 text-13 leading-5 text-af-ink-3">
              <li>{{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
              <li>{{ t('affiliate.tips.line3') }}</li>
              <li v-if="detail.aff_frozen_quota > 0">{{ t('affiliate.tips.line4') }}</li>
            </ul>
          </template>

          <dl class="-my-3.5 divide-y divide-af-hairline">
            <div class="flex flex-wrap items-center gap-x-6 gap-y-1 py-3.5">
              <dt class="w-24 shrink-0 text-13 text-af-ink-3">{{ t('affiliate.yourCode') }}</dt>
              <dd class="min-w-0 flex-1"><code class="block truncate font-mono text-sm font-medium text-af-ink">{{ detail.aff_code }}</code></dd>
              <button type="button" :class="COPY_BUTTON" @click="copyCode">
                <Icon name="copy" size="sm" />
                <span>{{ t('affiliate.copyCode') }}</span>
              </button>
            </div>
            <div class="flex flex-wrap items-center gap-x-6 gap-y-1 py-3.5">
              <dt class="w-24 shrink-0 text-13 text-af-ink-3">{{ t('affiliate.inviteLink') }}</dt>
              <dd class="min-w-0 flex-1"><code class="block truncate font-mono text-sm text-af-ink-2" :title="inviteLink">{{ inviteLink }}</code></dd>
              <button type="button" :class="COPY_BUTTON" @click="copyInviteLink">
                <Icon name="copy" size="sm" />
                <span>{{ t('affiliate.copyLink') }}</span>
              </button>
            </div>
          </dl>
        </SettingsRow>

        <SettingsRow :title="t('affiliate.transfer.title')" :description="t('affiliate.transfer.description')">
          <div v-if="detail.aff_quota > 0" class="flex flex-wrap items-center gap-x-6 gap-y-3">
            <p class="text-sm text-af-ink-3">
              {{ t('affiliate.stats.availableQuota') }}
              <span class="ml-2 text-lg font-semibold tabular-nums text-af-ink">{{ formatCurrency(detail.aff_quota) }}</span>
            </p>
            <button class="btn btn-primary btn-md" :disabled="transferring" @click="transferQuota">
              {{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}
            </button>
          </div>
          <p v-else class="text-13 text-af-ink-3">{{ t('affiliate.transfer.empty') }}</p>
        </SettingsRow>

        <SettingsRow :title="t('affiliate.invitees.title')">
          <StatusState v-if="detail.invitees.length === 0" kind="empty" :title="t('affiliate.invitees.empty')" />
          <div v-else class="overflow-x-auto">
            <table class="w-full min-w-[480px] text-left text-13">
              <thead>
                <tr class="border-b border-af-hairline text-af-ink-3">
                  <th class="py-2 pr-4 font-medium">{{ t('affiliate.invitees.columns.email') }}</th>
                  <th class="py-2 pr-4 font-medium">{{ t('affiliate.invitees.columns.username') }}</th>
                  <th class="py-2 pr-4 text-right font-medium">{{ t('affiliate.invitees.columns.rebate') }}</th>
                  <th class="py-2 font-medium">{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-af-hairline">
                <tr v-for="item in detail.invitees" :key="item.user_id" class="h-11">
                  <td class="pr-4 text-af-ink">{{ item.email || '-' }}</td>
                  <td class="pr-4 text-af-ink-2">{{ item.username || '-' }}</td>
                  <td class="pr-4 text-right font-medium tabular-nums text-af-ink">{{ formatCurrency(item.total_rebate) }}</td>
                  <td class="tabular-nums text-af-ink-2">{{ formatDateTime(item.created_at) || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </SettingsRow>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import SettingsRow from '@/components/user/shell/SettingsRow.vue'
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

/** 与概览「快速开始」的复制按钮同一套：图标 + 文字，无框 */
const COPY_BUTTON = 'inline-flex shrink-0 items-center gap-1.5 text-13 font-medium text-af-ink-2 transition-colors hover:text-af-ink'

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
    { key: 'rebate-rate', label: t('affiliate.stats.rebateRate'), value: `${formattedRebateRate.value}%` },
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
