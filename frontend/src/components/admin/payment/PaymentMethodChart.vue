<template>
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-af-ink">
      {{ t('payment.admin.paymentDistribution') }}
    </h3>
    <div
      v-if="!methods?.length"
      class="flex h-32 items-center justify-center text-sm text-af-ink-3"
    >
      {{ t('payment.admin.noData') }}
    </div>
    <div v-else class="space-y-3">
      <div v-for="method in methods" :key="method.type" class="space-y-1">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <span :class="['inline-block h-3 w-3 rounded-full', colorMap[method.type] || 'bg-af-ink-4']"></span>
            <span class="text-sm text-af-ink-2">
              {{ t('payment.methods.' + method.type, method.type) }}
            </span>
          </div>
          <div class="space-y-1 text-right">
            <span v-for="[currency, amount] in sortedAmounts(method.amount)" :key="currency" class="block text-sm font-medium text-af-ink">
              {{ formatMoney(currency, amount) }}
            </span>
            <span class="ml-2 text-xs text-af-ink-3">
              ({{ method.count }})
            </span>
          </div>
        </div>
        <div v-for="[currency, amount] in sortedAmounts(method.amount)" :key="currency" class="flex items-center gap-2">
          <span class="w-10 text-xs text-af-ink-3">{{ currency }}</span>
          <div class="h-2 flex-1 overflow-hidden rounded-full bg-af-sunken">
            <div
              :class="['h-full rounded-full transition-all', barColorMap[method.type] || 'bg-af-ink-4']"
              :style="{ width: barWidth(currency, amount) + '%' }"
            ></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CurrencyAmounts, PaymentMethodStats } from '@/types/payment'

const { t } = useI18n()

const props = defineProps<{
  methods: PaymentMethodStats[]
}>()

const colorMap: Record<string, string> = {
  alipay: 'bg-af-ink',
  wxpay: 'bg-af-ink-3',
  alipay_direct: 'bg-af-ink',
  wxpay_direct: 'bg-af-ink-3',
  stripe: 'bg-af-ink-4',
}

const barColorMap: Record<string, string> = {
  alipay: 'bg-af-ink',
  wxpay: 'bg-af-ink-3',
  alipay_direct: 'bg-af-ink',
  wxpay_direct: 'bg-af-ink-3',
  stripe: 'bg-af-ink-4',
}

const maxAmounts = computed<CurrencyAmounts>(() => {
  return props.methods.reduce<CurrencyAmounts>((maximums, method) => {
    for (const [currency, amount] of Object.entries(method.amount)) {
      maximums[currency] = Math.max(maximums[currency] || 0, amount)
    }
    return maximums
  }, {})
})

function sortedAmounts(amounts: CurrencyAmounts): [string, number][] {
  return Object.entries(amounts).sort(([left], [right]) => left.localeCompare(right))
}

function barWidth(currency: string, amount: number): number {
  return Math.min((amount / (maxAmounts.value[currency] || 1)) * 100, 100)
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}
</script>
