<template>
  <!-- 余额以文字呈现、点击去充值；有冻结金额时 hover 展开明细 -->
  <div class="group relative">
    <RouterLink
      to="/billing/recharge"
      class="flex h-8 items-center gap-1.5 rounded-md px-2 text-13 font-medium text-af-ink tabular-nums transition-colors hover:bg-af-sunken"
      :aria-label="`${t('userUi.topbar.balance')} ${formatMoney(available)}`"
      data-testid="balance-link"
    >
      <span class="text-af-ink-3">{{ t('userUi.topbar.balance') }}</span>
      <span>{{ formatMoney(available) }}</span>
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
        <div class="flex items-center justify-between">
          <dt class="text-af-ink-3">{{ t('common.availableBalance') }}</dt>
          <dd class="font-medium text-af-ink">{{ formatMoney(available) }}</dd>
        </div>
        <div class="flex items-center justify-between">
          <dt class="text-af-ink-3">{{ t('common.frozenBalance') }}</dt>
          <dd class="font-medium text-af-warning">{{ formatMoney(frozen) }}</dd>
        </div>
        <div class="flex items-center justify-between border-t border-af-hairline pt-2">
          <dt class="text-af-ink-3">{{ t('common.totalBalance') }}</dt>
          <dd class="font-semibold text-af-ink">{{ formatMoney(available + frozen) }}</dd>
        </div>
      </dl>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { formatCurrency } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()

const available = computed(() => Number(authStore.user?.balance || 0))
const frozen = computed(() => Number(authStore.user?.frozen_balance || 0))

const formatMoney = (value: number) => formatCurrency(Number.isFinite(value) ? value : 0)
</script>
