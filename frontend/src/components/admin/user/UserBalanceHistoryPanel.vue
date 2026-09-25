<template>
  <!--
    用户余额流水（A5）：兑换 / 在线充值、返利转入、管理员调整（充值 / 扣减余额）、并发与订阅变动，按类型筛选、分页；工具行写累计充值。
    用户详情抽屉的「余额流水」页签与用量页的余额记录弹窗共用这一块。
  -->
  <div class="space-y-3" data-testid="user-balance-history">
    <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
      <FilterChip
        v-model="typeFilter"
        :label="t('admin.users.detail.historyType')"
        :options="typeOptions"
        test-id="balance-history-type"
        @change="loadHistory(1)"
      />
      <span class="text-13 text-af-ink-3">
        {{ t('admin.users.totalRecharged') }}
        <span class="ml-1 font-medium tabular-nums text-af-ink" data-testid="balance-history-total">{{ formatMoney(totalRecharged) }}</span>
      </span>
      <div v-if="!hideActions" class="ml-auto flex items-center gap-2">
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('deposit')">
          <Icon name="plus" size="sm" />
          {{ t('admin.users.deposit') }}
        </button>
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('withdraw')">
          <Icon name="arrowDown" size="sm" />
          {{ t('admin.users.withdraw') }}
        </button>
      </div>
    </div>

    <StatusState v-if="loading" kind="loading" :title="t('common.loading')" />
    <StatusState
      v-else-if="failed"
      kind="error"
      :title="t('admin.users.failedToLoadBalanceHistory')"
      :action-label="t('admin.users.detail.retry')"
      @action="loadHistory(currentPage)"
    />
    <StatusState v-else-if="history.length === 0" kind="empty" :title="t('admin.users.noBalanceHistory')" />

    <ul v-else class="divide-y divide-af-hairline border-t border-af-hairline" data-testid="balance-history-list">
      <li v-for="item in history" :key="item.id" class="flex items-start justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="text-sm text-af-ink">{{ getItemTitle(item) }}</p>
          <p v-if="item.notes" class="mt-0.5 break-words text-xs text-af-ink-3">{{ item.notes }}</p>
          <p class="mt-0.5 text-xs tabular-nums text-af-ink-3">{{ formatDateTime(item.used_at || item.created_at) }}</p>
        </div>
        <div class="shrink-0 text-right">
          <p class="text-sm font-medium tabular-nums" :class="item.value < 0 ? 'text-af-ink-2' : 'text-af-ink'">
            {{ formatValue(item) }}
          </p>
          <p v-if="isAdminType(item.type)" class="text-xs text-af-ink-3">{{ t('redeem.adminAdjustment') }}</p>
          <p v-else-if="item.code" class="font-mono text-xs text-af-ink-4" :title="item.code">{{ item.code.slice(0, 8) }}…</p>
        </div>
      </li>
    </ul>

    <div v-if="!loading && !failed && totalPages > 1" class="flex items-center justify-center gap-3 pt-1">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="currentPage <= 1" @click="loadHistory(currentPage - 1)">
        {{ t('pagination.previous') }}
      </button>
      <span class="text-13 tabular-nums text-af-ink-3">{{ currentPage }} / {{ totalPages }}</span>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="currentPage >= totalPages" @click="loadHistory(currentPage + 1)">
        {{ t('pagination.next') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type BalanceHistoryItem } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import { formatMoney } from '@/utils/money'
import Icon from '@/components/icons/Icon.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import { FilterChip } from '@/components/admin/list'
import type { FilterOption } from '@/components/admin/list'

const props = withDefaults(
  defineProps<{
    userId: number
    hideActions?: boolean
    /** 外面改了余额等数据后加一，面板回到当前筛选的第一页重新取 */
    refreshKey?: number
  }>(),
  { hideActions: false, refreshKey: 0 }
)
const emit = defineEmits<{ (e: 'deposit'): void; (e: 'withdraw'): void }>()
const { t } = useI18n()

const PAGE_SIZE = 15
const history = ref<BalanceHistoryItem[]>([])
const loading = ref(false)
const failed = ref(false)
const currentPage = ref(1)
const total = ref(0)
const totalRecharged = ref(0)
const typeFilter = ref<string | number>('')
let requestSeq = 0

const totalPages = computed(() => Math.ceil(total.value / PAGE_SIZE) || 1)

const typeOptions = computed<FilterOption[]>(() => [
  { value: 'balance', label: t('admin.users.typeBalance') },
  { value: 'affiliate_balance', label: t('admin.users.typeAffiliateBalance') },
  { value: 'admin_balance', label: t('admin.users.typeAdminBalance') },
  { value: 'concurrency', label: t('admin.users.typeConcurrency') },
  { value: 'admin_concurrency', label: t('admin.users.typeAdminConcurrency') },
  { value: 'subscription', label: t('admin.users.typeSubscription') }
])

async function loadHistory(page: number) {
  const seq = ++requestSeq
  loading.value = true
  failed.value = false
  currentPage.value = page
  try {
    const res = await adminAPI.users.getUserBalanceHistory(props.userId, page, PAGE_SIZE, String(typeFilter.value) || undefined)
    if (seq !== requestSeq) return
    history.value = res.items || []
    total.value = res.total || 0
    totalRecharged.value = res.total_recharged || 0
  } catch (error) {
    if (seq !== requestSeq) return
    failed.value = true
    console.error('Failed to load balance history:', error)
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

// 换了用户就清掉筛选；外面刷新只回第一页
watch(
  () => props.userId,
  () => {
    typeFilter.value = ''
    void loadHistory(1)
  },
  { immediate: true }
)
watch(
  () => props.refreshKey,
  () => void loadHistory(1)
)

const isAdminType = (type: string) => type === 'admin_balance' || type === 'admin_concurrency'
const isBalanceType = (type: string) => type === 'balance' || type === 'admin_balance' || type === 'affiliate_balance'

// 在线支付到账也记成 balance 型兑换码，码是下单时生成的「PAY-订单号-…」（backend service/payment_order.go），
// 按前缀和真正的兑换码分开叫
const isOnlineRecharge = (item: BalanceHistoryItem) => item.type === 'balance' && item.code.startsWith('PAY-')

function getItemTitle(item: BalanceHistoryItem): string {
  switch (item.type) {
    case 'balance':
      return isOnlineRecharge(item) ? t('admin.users.detail.historyOnlineRecharge') : t('redeem.balanceAddedRedeem')
    case 'affiliate_balance':
      return t('redeem.balanceAddedAffiliate')
    case 'admin_balance':
      return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
    case 'concurrency':
      return t('redeem.concurrencyAddedRedeem')
    case 'admin_concurrency':
      return item.value >= 0 ? t('redeem.concurrencyAddedAdmin') : t('redeem.concurrencyReducedAdmin')
    case 'subscription':
      return t('redeem.subscriptionAssigned')
    default:
      return t('common.unknown')
  }
}

function formatValue(item: BalanceHistoryItem): string {
  if (isBalanceType(item.type)) return item.value < 0 ? formatMoney(item.value) : `+${formatMoney(item.value)}`
  if (item.type === 'subscription') return t('redeem.subscriptionDays', { days: item.validity_days || Math.round(item.value) })
  return `${item.value < 0 ? '' : '+'}${item.value}`
}
</script>
