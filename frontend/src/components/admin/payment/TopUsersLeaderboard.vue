<template>
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-af-ink">
      {{ t('payment.admin.topUsers') }}
    </h3>
    <div
      v-if="!hasUsers(props.users)"
      class="flex h-32 items-center justify-center text-sm text-af-ink-3"
    >
      {{ t('payment.admin.noData') }}
    </div>
    <div v-else class="space-y-2">
      <div v-for="[currency, currencyUsers] in sortedUsers(props.users)" :key="currency" class="space-y-2">
        <p class="text-xs font-semibold text-af-ink-3">{{ currency }}</p>
        <div
          v-for="(user, idx) in currencyUsers"
          :key="user.user_id"
          class="flex items-center justify-between rounded-lg px-3 py-2 hover:bg-af-sunken"
        >
          <div class="flex items-center gap-3">
            <span
              :class="[
                'flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold',
                rankClass(idx),
              ]"
            >
              {{ idx + 1 }}
            </span>
            <span class="text-sm text-af-ink-2">{{ user.email }}</span>
          </div>
          <span class="text-sm font-medium text-af-ink">
            {{ formatMoney(currency, user.amount) }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { TopUserPaymentStats } from '@/types/payment'

const { t } = useI18n()

const props = defineProps<{
  users: Record<string, TopUserPaymentStats[]>
}>()

function rankClass(idx: number): string {
  if (idx === 0) return 'bg-af-ink text-af-on-brand'
  if (idx === 1) return 'bg-af-hairline-strong text-af-ink'
  if (idx === 2) return 'bg-af-hairline text-af-ink-2'
  return 'bg-af-sunken text-af-ink-3'
}

function hasUsers(usersByCurrency: Record<string, TopUserPaymentStats[]>): boolean {
  return Object.values(usersByCurrency).some(users => users.length > 0)
}

function sortedUsers(usersByCurrency: Record<string, TopUserPaymentStats[]>): [string, TopUserPaymentStats[]][] {
  return Object.entries(usersByCurrency).sort(([left], [right]) => left.localeCompare(right))
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}
</script>
