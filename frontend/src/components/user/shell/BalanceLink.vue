<template>
  <!--
    顶栏余额。在线充值开着时点击去充值页；关着时（先人工充值，muqian 2026-10-05）点击弹出「联系客服充值」+ 客服联系方式。
    有冻结金额时：充值页模式 hover 展开明细，弹出层模式把明细放进弹出层。手机宽只显示金额，「余额」二字从 sm 起才出现。
    余额为负是透支的欠款（允许一次请求透支，充值先抵欠款、补上才能再用，muqian 2026-09-29）：写「欠费」+ 金额并标红
  -->
  <div v-if="canRecharge" class="group relative">
    <RouterLink
      to="/billing/recharge"
      class="flex h-8 items-center gap-1.5 rounded-md px-2 text-13 font-medium text-af-ink tabular-nums transition-colors hover:bg-af-sunken"
      :aria-label="ariaLabel"
      data-testid="balance-link"
    >
      <span class="hidden text-af-ink-3 sm:inline">{{ balanceLabel }}</span>
      <span :class="{ 'text-af-danger': inDebt }" data-testid="balance-amount">{{ formatMoney(inDebt ? -available : available) }}</span>
      <span v-if="frozen > 0" class="text-af-warning" data-testid="balance-frozen">
        {{ t('userUi.topbar.frozen') }} {{ formatMoney(frozen) }}
      </span>
    </RouterLink>
    <div
      v-if="frozen > 0"
      class="pointer-events-none absolute right-0 top-full z-40 mt-1 hidden w-56 rounded-lg border border-af-hairline bg-af-sheet p-3 text-xs shadow-lg group-hover:block"
      role="tooltip"
    >
      <dl class="space-y-2">
        <div v-for="row in breakdown" :key="row.key" class="flex items-center justify-between" :class="row.key === 'total' ? 'border-t border-af-hairline pt-2' : ''">
          <dt class="text-af-ink-3">{{ row.label }}</dt>
          <dd :class="row.class">{{ row.value }}</dd>
        </div>
      </dl>
    </div>
  </div>

  <PopoverMenu v-else align="end" width-class="w-64" :close-on-select="false">
    <template #trigger="{ open }">
      <button
        type="button"
        class="flex h-8 items-center gap-1.5 rounded-md px-2 text-13 font-medium text-af-ink tabular-nums transition-colors hover:bg-af-sunken"
        :class="open ? 'bg-af-sunken' : ''"
        :aria-expanded="open"
        :aria-label="ariaLabel"
        data-testid="balance-link"
      >
        <span class="hidden text-af-ink-3 sm:inline">{{ balanceLabel }}</span>
        <span :class="{ 'text-af-danger': inDebt }" data-testid="balance-amount">{{ formatMoney(inDebt ? -available : available) }}</span>
        <span v-if="frozen > 0" class="text-af-warning" data-testid="balance-frozen">
          {{ t('userUi.topbar.frozen') }} {{ formatMoney(frozen) }}
        </span>
      </button>
    </template>
    <div class="space-y-3 px-4 py-3 text-13" data-testid="balance-recharge-panel">
      <dl v-if="frozen > 0" class="space-y-2 border-b border-af-hairline pb-3 text-xs">
        <div v-for="row in breakdown" :key="row.key" class="flex items-center justify-between">
          <dt class="text-af-ink-3">{{ row.label }}</dt>
          <dd :class="row.class">{{ row.value }}</dd>
        </div>
      </dl>
      <div>
        <p class="font-medium text-af-ink">{{ t('userUi.topbar.rechargeViaSupport') }}</p>
        <!-- 联系方式点一下整段选中，方便复制 -->
        <p v-if="contactInfo" class="mt-1 select-all text-af-ink-2" data-testid="balance-contact">{{ contactInfo }}</p>
      </div>
    </div>
  </PopoverMenu>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import PopoverMenu from '@/components/common/PopoverMenu.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'
import { formatCurrency } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

// 与概览「充值」入口同一个判断：在线支付开着才链到充值页
const canRecharge = computed(() => resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.payment))
const contactInfo = computed(() => appStore.contactInfo.trim())

const available = computed(() => Number(authStore.user?.balance || 0))
const frozen = computed(() => Number(authStore.user?.frozen_balance || 0))
const inDebt = computed(() => available.value < 0)
const balanceLabel = computed(() => (inDebt.value ? t('userUi.topbar.debt') : t('userUi.topbar.balance')))
const ariaLabel = computed(() => `${balanceLabel.value} ${formatMoney(Math.abs(available.value))}`)

const formatMoney = (value: number) => formatCurrency(Number.isFinite(value) ? value : 0)

/** 有冻结金额时的明细：可用 / 冻结 / 合计 */
const breakdown = computed(() => [
  { key: 'available', label: t('common.availableBalance'), value: formatMoney(available.value), class: 'font-medium text-af-ink' },
  { key: 'frozen', label: t('common.frozenBalance'), value: formatMoney(frozen.value), class: 'font-medium text-af-warning' },
  { key: 'total', label: t('common.totalBalance'), value: formatMoney(available.value + frozen.value), class: 'font-semibold text-af-ink' },
])
</script>
