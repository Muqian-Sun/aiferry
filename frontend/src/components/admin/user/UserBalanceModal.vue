<template>
  <BaseDialog :show="show" :title="operation === 'add' ? t('admin.users.deposit') : t('admin.users.withdraw')" width="narrow" @close="$emit('close')">
    <form v-if="user" id="balance-form" @submit.prevent="handleBalanceSubmit" class="space-y-5">
      <div class="flex items-center gap-3 rounded-xl bg-af-sunken p-4">
        <div class="flex h-10 w-10 items-center justify-center rounded-full bg-af-brand-tint"><span class="text-lg font-medium text-af-brand">{{ user.email.charAt(0).toUpperCase() }}</span></div>
        <div class="flex-1"><p class="font-medium text-af-ink">{{ user.email }}</p><p class="text-sm text-af-ink-3">{{ t('admin.users.currentBalance') }}: {{ formatMoneyExact(user.balance) }}</p></div>
      </div>
      <div>
        <label class="input-label">{{ operation === 'add' ? t('admin.users.depositAmount') : t('admin.users.withdrawAmount') }}</label>
        <div class="relative flex gap-2">
          <div class="relative flex-1"><div class="absolute left-3 top-1/2 -translate-y-1/2 font-medium text-af-ink-3">$</div><input v-model.number="form.amount" type="number" step="any" min="0" required class="input pl-8" /></div>
          <button v-if="operation === 'subtract'" type="button" @click="fillAllBalance" class="btn btn-secondary whitespace-nowrap">{{ t('admin.users.withdrawAll') }}</button>
        </div>
      </div>
      <div><label class="input-label">{{ t('admin.users.notes') }}</label><textarea v-model="form.notes" rows="3" class="input"></textarea></div>
      <div v-if="form.amount > 0" class="rounded-xl border border-af-hairline bg-af-sunken p-4">
        <div class="flex items-center justify-between text-sm">
          <span class="text-af-ink-2">{{ t('admin.users.newBalance') }}:</span>
          <span v-if="exceedsBalance" class="font-medium text-af-danger" data-testid="balance-exceeds">{{ t('admin.users.insufficientBalance') }}</span>
          <span v-else class="font-bold text-af-ink">{{ formatMoneyExact(calculateNewBalance()) }}</span>
        </div>
      </div>
    </form>
    <template #footer>
      <div class="flex w-full flex-wrap items-center justify-end gap-3">
        <FormError class="mr-auto min-w-0 flex-1" :message="error" />
        <button @click="$emit('close')" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button type="submit" form="balance-form" :disabled="submitting || !form.amount || exceedsBalance" class="btn" :class="operation === 'add' ? 'btn-primary' : 'btn-danger'">{{ submitting ? t('common.saving') : t('common.confirm') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import FormError from '@/components/common/FormError.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
// 调余额的对话框要看准到分以下的余额（「全部」扣减会填满精确值），用精确格式而不是汇总的两位小数
import { formatMoneyExact } from '@/utils/money'

const props = defineProps<{ show: boolean, user: AdminUser | null, operation: 'add' | 'subtract' }>()
// success 带上改完的用户：编辑弹窗开着时用它刷新「当前余额」
const emit = defineEmits<{ close: []; success: [user: AdminUser] }>(); const { t } = useI18n()

const submitting = ref(false); const form = reactive({ amount: 0, notes: '' })
// 报错显示在弹窗底部（原来只打到控制台）
const error = ref('')
watch(() => props.show, (v) => { if(v) { form.amount = 0; form.notes = ''; error.value = '' } })
// 改了金额，上一次提交的报错就过时了（例如余额不足改回合法值后不该还挂着）
watch(() => form.amount, () => { error.value = '' })

// 扣减超过当前余额：预览处直接标出来、不让提交，不等点了确认才报
const exceedsBalance = computed(() => props.operation === 'subtract' && !!props.user && form.amount > props.user.balance)

// 填入全部余额
const fillAllBalance = () => {
  if (props.user) {
    form.amount = props.user.balance
  }
}

const calculateNewBalance = () => {
  if (!props.user) return 0
  const result = props.operation === 'add' ? props.user.balance + form.amount : props.user.balance - form.amount
  // 避免浮点数精度问题导致的 -0.00 显示
  return Math.abs(result) < 1e-10 ? 0 : result
}
const handleBalanceSubmit = async () => {
  if (!props.user) return
  error.value = ''
  if (!form.amount || form.amount <= 0) {
    error.value = t('admin.users.amountRequired')
    return
  }
  if (exceedsBalance.value) {
    error.value = t('admin.users.insufficientBalance')
    return
  }
  submitting.value = true
  try {
    const updated = await adminAPI.users.updateBalance(props.user.id, form.amount, props.operation, form.notes)
    emit('success', updated); emit('close')
  } catch (e) {
    error.value = extractApiErrorMessage(e, t('admin.users.failedToUpdateBalance'))
  } finally { submitting.value = false }
}
</script>
