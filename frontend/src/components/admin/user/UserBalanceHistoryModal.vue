<template>
  <!-- 余额记录弹窗：用量页点用户打开。正文是 UserBalanceHistoryPanel（用户详情抽屉的「余额流水」页签也用它），这里只多一行用户信息。 -->
  <BaseDialog :show="show" :title="t('admin.users.balanceHistoryTitle')" width="wide" :close-on-click-outside="true" :z-index="40" @close="$emit('close')">
    <div v-if="user" class="space-y-5">
      <div class="flex items-start justify-between gap-4 border-b border-af-hairline pb-4">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <p class="truncate font-medium text-af-ink">{{ user.email }}</p>
            <span
              v-if="user.deleted_at"
              class="inline-flex flex-shrink-0 items-center rounded px-1 py-px text-xs font-medium leading-tight bg-af-danger-tint text-af-danger ring-1 ring-inset ring-af-danger/30"
            >
              {{ t('admin.usage.userDeletedBadge') }}
            </span>
          </div>
          <p class="mt-0.5 truncate text-xs text-af-ink-3">
            <template v-if="user.username">{{ user.username }} · </template>{{ t('admin.users.createdAt') }}: {{ formatDateTime(user.created_at) }}
          </p>
          <p v-if="user.notes" class="mt-0.5 truncate text-xs text-af-ink-3" :title="user.notes">
            {{ t('admin.users.notes') }}: {{ user.notes }}
          </p>
        </div>
        <div class="flex-shrink-0 text-right">
          <p class="text-xs text-af-ink-3">{{ t('admin.users.currentBalance') }}</p>
          <p class="text-xl font-semibold tabular-nums" :class="balanceTextClass(user.balance)">{{ formatBalance(user.balance) }}</p>
        </div>
      </div>
      <UserBalanceHistoryPanel
        :user-id="user.id"
        :hide-actions="hideActions"
        @deposit="emit('deposit')"
        @withdraw="emit('withdraw')"
      />
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import { balanceTextClass, formatBalance } from '@/utils/money'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import UserBalanceHistoryPanel from './UserBalanceHistoryPanel.vue'

defineProps<{ show: boolean; user: AdminUser | null; hideActions?: boolean }>()
const emit = defineEmits(['close', 'deposit', 'withdraw'])
const { t } = useI18n()
</script>
