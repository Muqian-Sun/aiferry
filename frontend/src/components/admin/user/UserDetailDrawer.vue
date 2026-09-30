<template>
  <!--
    用户详情抽屉（A5）：用户列表点行打开，看完即关。页签：概况（余额 + 充值 / 扣减余额、资料、自定义属性）、
    余额流水、API 密钥、订阅（SITE_FEATURES.subscription 关着时不出现）、用量（近 30 天：请求、Token、收入 / 成本 / 利润）。
    改资料点「编辑」，停用 / 删除在「⋯」里；这些动作都 emit 给列表页，由列表页弹原有的对话框，改完刷新列表和抽屉。
  -->
  <DetailDrawer
    :show="show && !!user"
    :title="user?.email ?? ''"
    :eyebrow="eyebrow"
    :subtitle="user?.username ?? ''"
    :tabs="tabs"
    :tab="tab"
    @update:tab="emit('update:tab', $event)"
    @close="emit('close')"
  >
    <template #actions>
      <button type="button" class="btn btn-secondary btn-sm" data-testid="user-drawer-edit" @click="emit('edit')">
        {{ t('common.edit') }}
      </button>
      <PopoverMenu v-if="user && user.role !== 'admin'" width-class="w-40">
        <template #trigger="{ open }">
          <button
            type="button"
            class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink"
            :class="open ? 'bg-af-sunken text-af-ink' : ''"
            :title="t('common.more')"
            :aria-label="t('common.more')"
            data-testid="user-drawer-more"
          >
            <Icon name="more" size="md" />
          </button>
        </template>
        <MenuItem
          :icon="user.status === 'active' ? 'ban' : 'checkCircle'"
          data-testid="user-drawer-toggle-status"
          @click="emit('toggle-status')"
        >
          {{ user.status === 'active' ? t('admin.users.disable') : t('admin.users.enable') }}
        </MenuItem>
        <MenuItem divider />
        <MenuItem icon="trash" danger data-testid="user-drawer-delete" @click="emit('delete')">
          {{ t('common.delete') }}
        </MenuItem>
      </PopoverMenu>
    </template>

    <template v-if="user?.status === 'disabled'" #banner>
      <p class="flex items-center gap-2 rounded-md bg-af-danger-tint px-3 py-2 text-13 text-af-danger" data-testid="user-drawer-banner">
        <Icon name="ban" size="sm" class="shrink-0" />
        {{ t('admin.users.detail.disabledBanner') }}
      </p>
    </template>

    <template v-if="user">
      <!-- 概况 -->
      <div v-if="tab === 'overview'" class="space-y-8" data-testid="user-drawer-overview">
        <section class="flex flex-wrap items-end justify-between gap-4">
          <div>
            <p class="text-13 text-af-ink-3">{{ t('admin.users.columns.balance') }}</p>
            <p class="mt-1 text-3xl font-semibold tracking-[-0.01em] tabular-nums" :class="balanceTextClass(user.balance)" data-testid="user-drawer-balance">
              {{ formatBalance(user.balance) }}
            </p>
            <p v-if="user.frozen_balance" class="mt-1 text-xs tabular-nums text-af-ink-3">
              {{ t('admin.users.detail.frozen', { amount: formatMoney(user.frozen_balance) }) }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-secondary btn-sm" data-testid="user-drawer-deposit" @click="emit('deposit')">
              <Icon name="plus" size="sm" />
              {{ t('admin.users.deposit') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" data-testid="user-drawer-withdraw" @click="emit('withdraw')">
              <Icon name="arrowDown" size="sm" />
              {{ t('admin.users.withdraw') }}
            </button>
          </div>
        </section>

        <!-- 邮箱、用户名已在抽屉标题和副标题里，这里不再列一遍 -->
        <dl class="divide-y divide-af-hairline border-t border-af-hairline">
          <DetailField :label="t('admin.users.columns.role')" :value="t('admin.users.roles.' + user.role)" />
          <DetailField :label="t('admin.users.columns.status')">
            <span class="inline-flex items-center gap-1.5">
              <span :class="['inline-block h-2 w-2 rounded-full', user.status === 'active' ? 'bg-af-ink-4' : 'bg-af-danger']"></span>
              <span :class="user.status === 'active' ? 'text-af-ink' : 'text-af-danger'">
                {{ user.status === 'active' ? t('common.active') : t('admin.users.disabled') }}
              </span>
            </span>
          </DetailField>
          <DetailField :label="t('admin.users.columns.concurrency')">
            <span class="tabular-nums">
              {{ t('admin.users.detail.concurrencyValue', { current: user.current_concurrency ?? 0, max: limitLabel(user.concurrency) }) }}
            </span>
          </DetailField>
          <DetailField :label="t('admin.users.detail.rpmLimit')">
            <span class="tabular-nums">{{ limitLabel(user.rpm_limit ?? 0) }}</span>
          </DetailField>
          <DetailField :label="t('admin.users.columns.rateMultiplier')">
            <span class="tabular-nums">× {{ user.rate_multiplier }}</span>
          </DetailField>
          <DetailField :label="t('admin.users.columns.created')">
            <span class="tabular-nums">{{ formatDateTime(user.created_at) }}</span>
          </DetailField>
          <!-- 最近活跃 = 最近一次调用 API（last_used_at）；最近访问控制台 = 登录 / 打开控制台（last_active_at） -->
          <DetailField :label="t('admin.users.columns.lastUsed')" :title="t('admin.users.columns.lastUsedHint')">
            <span v-if="user.last_used_at" :title="formatDateTime(user.last_used_at)">{{ formatRelativeTime(user.last_used_at) }}</span>
            <template v-else>—</template>
          </DetailField>
          <DetailField :label="t('admin.users.columns.lastActive')">
            <span v-if="user.last_active_at" :title="formatDateTime(user.last_active_at)">{{ formatRelativeTime(user.last_active_at) }}</span>
            <template v-else>—</template>
          </DetailField>
          <DetailField :label="t('admin.users.notes')">
            <span v-if="user.notes" class="whitespace-pre-wrap">{{ user.notes }}</span>
            <template v-else>—</template>
          </DetailField>
        </dl>

        <SheetSection v-if="attributeDefinitions.length" :title="t('admin.users.detail.attributes')">
          <StatusState v-if="attributesLoading" kind="loading" :title="t('common.loading')" />
          <dl v-else class="divide-y divide-af-hairline" data-testid="user-drawer-attributes">
            <DetailField
              v-for="def in attributeDefinitions"
              :key="def.id"
              :label="def.name"
              :value="formatAttributeValue(def, attributeValues[def.id] ?? '')"
            />
          </dl>
        </SheetSection>
      </div>

      <!-- 余额流水 -->
      <UserBalanceHistoryPanel
        v-else-if="tab === 'balance'"
        :user-id="user.id"
        :refresh-key="refreshKey"
        @deposit="emit('deposit')"
        @withdraw="emit('withdraw')"
      />

      <!-- API 密钥 -->
      <UserApiKeysPanel v-else-if="tab === 'keys'" :user-id="user.id" :refresh-key="refreshKey" />

      <!-- 订阅（只读；分配 / 调整在订阅页） -->
      <div v-else-if="tab === 'subscriptions'" data-testid="user-drawer-subscriptions">
        <StatusState v-if="subscriptionsLoading" kind="loading" :title="t('common.loading')" />
        <StatusState
          v-else-if="subscriptionsFailed"
          kind="error"
          :title="t('admin.users.detail.loadFailed')"
          :action-label="t('admin.users.detail.retry')"
          @action="loadSubscriptions"
        />
        <StatusState v-else-if="subscriptions.length === 0" kind="empty" :title="t('admin.users.noSubscription')" />
        <ul v-else class="divide-y divide-af-hairline">
          <li v-for="sub in subscriptions" :key="sub.id" class="py-3 first:pt-0" data-testid="user-drawer-subscription">
            <div class="flex items-baseline justify-between gap-4">
              <span class="truncate font-medium text-af-ink">{{ sub.plan?.name || t('common.deletedPlan') }}</span>
              <span class="inline-flex shrink-0 items-center gap-1.5 text-xs">
                <span class="inline-block h-2 w-2 rounded-full" :class="subscriptionTone(sub.status).dot"></span>
                <span :class="subscriptionTone(sub.status).text">{{ t(`admin.subscriptions.status.${sub.status}`) }}</span>
              </span>
            </div>
            <p class="mt-1 text-xs tabular-nums text-af-ink-3">
              {{
                t('admin.users.detail.subscriptionPeriod', {
                  start: formatDateOnly(sub.starts_at),
                  end: sub.expires_at ? formatDateOnly(sub.expires_at) : t('admin.subscriptions.noExpiration')
                })
              }}
            </p>
            <p class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-xs tabular-nums text-af-ink-2">
              <template v-if="usageWindows(sub).length">
                <span v-for="w in usageWindows(sub)" :key="w.key">
                  <span class="text-af-ink-3">{{ w.label }}</span>
                  {{ formatMoney(w.used) }}<span class="text-af-ink-4"> / {{ formatMoney(w.limit) }}</span>
                </span>
              </template>
              <span v-else class="text-af-ink-3">{{ t('admin.users.detail.unlimited') }}</span>
            </p>
          </li>
        </ul>
      </div>

      <!-- 用量：近 30 天汇总，明细去使用记录页（按用户筛好） -->
      <SheetSection
        v-else-if="tab === 'usage'"
        :title="t('admin.users.detail.usagePeriod')"
        :description="t('admin.users.detail.usageRange', { start: usageRange.start, end: usageRange.end })"
        data-testid="user-drawer-usage"
      >
        <template #actions>
          <RouterLink
            :to="{ path: '/usage', query: { user_id: String(user.id), start_date: usageRange.start, end_date: usageRange.end } }"
            class="inline-flex items-center gap-1 text-13 font-medium text-af-brand hover:text-af-brand-hover"
            data-testid="user-drawer-usage-link"
          >
            {{ t('admin.users.detail.viewUsage') }}
            <Icon name="arrowRight" size="xs" />
          </RouterLink>
        </template>
        <StatusState v-if="usageLoading" kind="loading" :title="t('common.loading')" />
        <StatusState
          v-else-if="usageFailed"
          kind="error"
          :title="t('admin.users.detail.loadFailed')"
          :action-label="t('admin.users.detail.retry')"
          @action="loadUsage"
        />
        <template v-else-if="usageStats">
          <StatRow :items="usageItems" />
          <!-- 金额只剩三个数：收入（actual_cost）、成本（标价 × 渠道成本倍率）、利润（为负标红） -->
          <dl class="mt-6 divide-y divide-af-hairline border-t border-af-hairline" data-testid="user-drawer-usage-money">
            <DetailField :label="t('common.money.revenue')" :title="t('common.money.revenueHint')">
              <span class="tabular-nums">{{ formatMoney(usageStats.total_actual_cost) }}</span>
            </DetailField>
            <DetailField :label="t('common.money.cost')" :title="t('common.money.costHint')">
              <span class="tabular-nums">{{ formatMoney(usageStats.total_account_cost) }}</span>
            </DetailField>
            <DetailField :label="t('common.money.profit')" :title="t('common.money.profitHint')">
              <span class="tabular-nums" :class="profitTextClass(usageProfit)" data-testid="user-drawer-usage-profit">
                {{ formatMoney(usageProfit) }}
              </span>
            </DetailField>
          </dl>
        </template>
      </SheetSection>
    </template>
  </DetailDrawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import type { AdminUser, UserAttributeDefinition, UserSubscription } from '@/types'
import { formatCompactNumber, formatDateOnly, formatDateTime, formatRelativeTime } from '@/utils/format'
import { balanceTextClass, formatBalance, formatMoney, profitOf, profitTextClass } from '@/utils/money'
import { SITE_FEATURES } from '@/utils/siteFeatures'
import Icon from '@/components/icons/Icon.vue'
import SheetSection from '@/components/user/shell/SheetSection.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import StatusState from '@/components/user/shell/StatusState.vue'
import type { SectionTab, StatItem } from '@/components/user/shell/types'
import { DetailDrawer, DetailField, MenuItem, PopoverMenu } from '@/components/admin/list'
import UserBalanceHistoryPanel from './UserBalanceHistoryPanel.vue'
import UserApiKeysPanel from './UserApiKeysPanel.vue'
import { formatAttributeValue } from './attributeValue'

const props = withDefaults(
  defineProps<{
    show: boolean
    user: AdminUser | null
    tab: string
    /** 已启用的自定义属性定义（列表页本来就会加载） */
    attributeDefinitions?: UserAttributeDefinition[]
    /** 列表页改完数据（充值、编辑、启停）后加一，抽屉重新取当前页签的数据 */
    refreshKey?: number
  }>(),
  { attributeDefinitions: () => [], refreshKey: 0 }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:tab', tab: string): void
  (e: 'edit'): void
  (e: 'deposit'): void
  (e: 'withdraw'): void
  (e: 'toggle-status'): void
  (e: 'delete'): void
}>()

const { t } = useI18n()

// 眉题写「角色 · 状态」（和列表的角色、状态两列同样的叫法），不写内部编号
const eyebrow = computed(() => {
  const user = props.user
  if (!user) return ''
  const status = user.status === 'active' ? t('common.active') : t('admin.users.disabled')
  return `${t('admin.users.roles.' + user.role)} · ${status}`
})

// 订阅功能由代码关着时（SITE_FEATURES.subscription = false）不出「订阅」页签
const tabs = computed<SectionTab[]>(() => [
  { key: 'overview', label: t('admin.users.detail.tabs.overview') },
  { key: 'balance', label: t('admin.users.detail.tabs.balance') },
  { key: 'keys', label: t('admin.users.detail.tabs.keys') },
  ...(SITE_FEATURES.subscription ? [{ key: 'subscriptions', label: t('admin.users.detail.tabs.subscriptions') }] : []),
  { key: 'usage', label: t('admin.users.detail.tabs.usage') }
])

/** 并发 / RPM：0 表示不限制 */
function limitLabel(value: number): string {
  return value > 0 ? String(value) : t('admin.users.detail.unlimited')
}

// ---- 自定义属性（概况页签） ----
const attributeValues = ref<Record<number, string>>({})
const attributesLoading = ref(false)
let attributesSeq = 0

async function loadAttributes(userId: number) {
  const seq = ++attributesSeq
  attributesLoading.value = true
  try {
    const values = await adminAPI.userAttributes.getUserAttributeValues(userId)
    if (seq !== attributesSeq) return
    attributeValues.value = Object.fromEntries((values ?? []).map((v) => [v.attribute_id, v.value]))
  } catch (error) {
    if (seq !== attributesSeq) return
    attributeValues.value = {}
    console.error('Failed to load user attribute values:', error)
  } finally {
    if (seq === attributesSeq) attributesLoading.value = false
  }
}

// ---- 订阅 ----
const subscriptions = ref<UserSubscription[]>([])
const subscriptionsLoading = ref(false)
const subscriptionsFailed = ref(false)
let subscriptionsSeq = 0

async function loadSubscriptions() {
  if (!props.user) return
  const seq = ++subscriptionsSeq
  subscriptionsLoading.value = true
  subscriptionsFailed.value = false
  try {
    const res = await adminAPI.subscriptions.list(1, 100, { user_id: props.user.id })
    if (seq !== subscriptionsSeq) return
    subscriptions.value = res.items ?? []
  } catch (error) {
    if (seq !== subscriptionsSeq) return
    subscriptionsFailed.value = true
    console.error('Failed to load user subscriptions:', error)
  } finally {
    if (seq === subscriptionsSeq) subscriptionsLoading.value = false
  }
}

// 与订阅页一致：生效中是常态灰点；撤销红、过期 / 暂停橙
function subscriptionTone(status: string): { dot: string; text: string } {
  if (status === 'active') return { dot: 'bg-af-ink-4', text: 'text-af-ink-2' }
  if (status === 'revoked') return { dot: 'bg-af-danger', text: 'text-af-danger' }
  return { dot: 'bg-af-warning', text: 'text-af-warning' }
}

function usageWindows(sub: UserSubscription): { key: string; label: string; used: number; limit: number }[] {
  const plan = sub.plan
  if (!plan) return []
  const windows = [
    { key: 'daily', label: t('admin.subscriptions.daily'), used: sub.daily_usage_usd, limit: plan.daily_limit_usd },
    { key: 'weekly', label: t('admin.subscriptions.weekly'), used: sub.weekly_usage_usd, limit: plan.weekly_limit_usd },
    { key: 'monthly', label: t('admin.subscriptions.monthly'), used: sub.monthly_usage_usd, limit: plan.monthly_limit_usd }
  ]
  return windows
    .filter((w): w is typeof w & { limit: number } => !!w.limit)
    .map((w) => ({ ...w, used: w.used ?? 0 }))
}

// ---- 用量（近 30 天，按本地日期；与使用记录页的日期筛选同一格式） ----
function localDate(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
const usageRange = ref({ start: '', end: '' })
const usageStats = ref<AdminUsageStatsResponse | null>(null)
const usageLoading = ref(false)
const usageFailed = ref(false)
let usageSeq = 0

async function loadUsage() {
  if (!props.user) return
  const end = new Date()
  const start = new Date(end.getTime() - 29 * 24 * 60 * 60 * 1000)
  usageRange.value = { start: localDate(start), end: localDate(end) }
  const seq = ++usageSeq
  usageLoading.value = true
  usageFailed.value = false
  try {
    const stats = await adminAPI.usage.getStats({
      user_id: props.user.id,
      start_date: usageRange.value.start,
      end_date: usageRange.value.end
    })
    if (seq !== usageSeq) return
    usageStats.value = stats
  } catch (error) {
    if (seq !== usageSeq) return
    usageFailed.value = true
    console.error('Failed to load user usage stats:', error)
  } finally {
    if (seq === usageSeq) usageLoading.value = false
  }
}

const usageItems = computed<StatItem[]>(() => {
  const stats = usageStats.value
  return [
    { key: 'requests', label: t('admin.users.detail.usageRequests'), value: (stats?.total_requests ?? 0).toLocaleString() },
    { key: 'tokens', label: t('admin.users.detail.usageTokens'), value: formatCompactNumber(stats?.total_tokens ?? 0) }
  ]
})
const usageProfit = computed(() => profitOf(usageStats.value?.total_actual_cost, usageStats.value?.total_account_cost))

// 打开、换用户、换页签、外面刷新：只取当前页签要的数据（余额流水 / 密钥由面板自己取）
watch(
  () => [props.show, props.user?.id, props.tab, props.refreshKey] as const,
  ([show, userId, tab], previous) => {
    if (!show || !userId) return
    const [, prevUserId] = previous ?? []
    if (userId !== prevUserId) {
      subscriptions.value = []
      usageStats.value = null
      attributeValues.value = {}
    }
    if (tab === 'overview' && props.attributeDefinitions.length) void loadAttributes(userId)
    else if (tab === 'subscriptions') void loadSubscriptions()
    else if (tab === 'usage') void loadUsage()
  },
  { immediate: true }
)
</script>
