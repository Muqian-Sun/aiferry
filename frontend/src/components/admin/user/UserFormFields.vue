<template>
  <!--
    新建 / 编辑用户共用的一张表（muqian 2026-09-30 方案第四节）：两边字段与顺序一样——
    邮箱、密码、用户名 / 角色、用户倍率 / 余额、并发 / RPM、备注。
    余额那一格：新建是「初始余额」（会记进余额流水），编辑是当前余额 +「充值 / 扣减」（打开现有的余额弹窗）。
    报错显示在对应字段下面。
  -->
  <div class="space-y-4">
    <div>
      <label class="input-label">{{ t('admin.users.email') }} <span class="text-af-danger">*</span></label>
      <input
        v-model="form.email"
        type="email"
        :class="['input', errors.email ? 'border-af-danger' : '']"
        :placeholder="t('admin.users.enterEmail')"
        autocomplete="off"
        data-testid="user-form-email"
      />
      <FormError :message="errors.email" />
    </div>

    <div>
      <label class="input-label">
        {{ t('admin.users.password') }} <span v-if="mode === 'create'" class="text-af-danger">*</span>
      </label>
      <div class="flex gap-2">
        <div class="relative flex-1">
          <input
            v-model="form.password"
            type="text"
            :class="['input pr-10', errors.password ? 'border-af-danger' : '']"
            :placeholder="mode === 'create' ? t('admin.users.enterPassword') : t('admin.users.enterNewPassword')"
            autocomplete="new-password"
            data-1p-ignore
            data-lpignore="true"
            data-bwignore="true"
            data-testid="user-form-password"
          />
          <button
            v-if="form.password"
            type="button"
            class="absolute right-2 top-1/2 -translate-y-1/2 rounded-lg p-1 transition-colors hover:bg-af-sunken"
            :class="passwordCopied ? 'text-af-success' : 'text-af-ink-3'"
            :aria-label="t('admin.users.form.copyPassword')"
            @click="copyPassword"
          >
            <Icon :name="passwordCopied ? 'check' : 'copy'" size="sm" />
          </button>
        </div>
        <button type="button" class="btn btn-secondary px-3" :aria-label="t('admin.users.form.generatePassword')" @click="form.password = randomPassword()">
          <Icon name="refresh" size="md" />
        </button>
      </div>
      <FormError v-if="errors.password" :message="errors.password" />
      <p v-else-if="mode === 'edit'" class="input-hint">{{ t('admin.users.form.passwordKeepHint') }}</p>
    </div>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.users.username') }}</label>
        <input v-model="form.username" type="text" class="input" :placeholder="t('admin.users.form.usernamePlaceholder')" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.form.roleLabel') }}</label>
        <select v-model="form.role" class="input" data-testid="user-form-role">
          <option value="user">{{ t('admin.users.roles.user') }}</option>
          <option value="admin">{{ t('admin.users.roles.admin') }}</option>
        </select>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.users.form.rateMultiplier') }}</label>
        <input
          v-model="form.rate_multiplier"
          type="number"
          min="0"
          step="any"
          :class="['input', errors.rate_multiplier ? 'border-af-danger' : '']"
          :placeholder="t('admin.users.form.rateMultiplierDefaultPlaceholder')"
          data-testid="user-rate-multiplier"
        />
        <FormError :message="errors.rate_multiplier" />
      </div>
      <div v-if="mode === 'create'">
        <label class="input-label">{{ t('admin.users.form.initialBalance') }}</label>
        <div class="relative">
          <span class="pointer-events-none absolute inset-y-0 left-3 flex items-center text-af-ink-3">$</span>
          <input
            v-model="form.balance"
            type="number"
            min="0"
            step="any"
            :class="['input pl-7', errors.balance ? 'border-af-danger' : '']"
            placeholder="0"
            data-testid="user-form-balance"
          />
        </div>
        <FormError v-if="errors.balance" :message="errors.balance" />
        <p v-else class="input-hint">{{ t('admin.users.form.initialBalanceHint') }}</p>
      </div>
      <div v-else>
        <label class="input-label">{{ t('admin.users.currentBalance') }}</label>
        <div class="flex items-center gap-2">
          <span class="flex-1 rounded-lg bg-af-sunken px-3 py-2 text-sm tabular-nums text-af-ink" data-testid="user-form-current-balance">
            {{ formatMoneyExact(currentBalance ?? 0) }}
          </span>
          <button type="button" class="btn btn-secondary btn-sm" data-testid="user-form-deposit" @click="emit('adjust-balance', 'add')">
            {{ t('admin.users.deposit') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" data-testid="user-form-withdraw" @click="emit('adjust-balance', 'subtract')">
            {{ t('admin.users.withdraw') }}
          </button>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.users.columns.concurrency') }}</label>
        <input
          v-model="form.concurrency"
          type="number"
          min="0"
          step="1"
          :class="['input', errors.concurrency ? 'border-af-danger' : '']"
          placeholder="0"
          data-test="concurrency-input"
        />
        <FormError v-if="errors.concurrency" :message="errors.concurrency" />
        <p v-else class="input-hint">{{ t('admin.users.form.zeroUnlimited') }}</p>
      </div>
      <div>
        <label class="input-label">{{ t('admin.users.form.rpmLimit') }}</label>
        <input
          v-model="form.rpm_limit"
          type="number"
          min="0"
          step="1"
          :class="['input', errors.rpm_limit ? 'border-af-danger' : '']"
          placeholder="0"
          data-testid="user-form-rpm"
        />
        <FormError v-if="errors.rpm_limit" :message="errors.rpm_limit" />
        <p v-else class="input-hint">{{ t('admin.users.form.zeroUnlimited') }}</p>
      </div>
    </div>

    <div>
      <label class="input-label">{{ t('admin.users.notes') }}</label>
      <textarea v-model="form.notes" rows="3" class="input" :placeholder="t('admin.users.form.notesPlaceholder')" data-testid="user-form-notes"></textarea>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import FormError from '@/components/common/FormError.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { formatMoneyExact } from '@/utils/money'
import { randomPassword, type UserFormErrors, type UserFormState } from './userForm'

defineProps<{
  mode: 'create' | 'edit'
  errors: UserFormErrors
  /** 编辑时的当前余额 */
  currentBalance?: number
}>()

const form = defineModel<UserFormState>('form', { required: true })

const emit = defineEmits<{ 'adjust-balance': [operation: 'add' | 'subtract'] }>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const passwordCopied = ref(false)
async function copyPassword() {
  if (form.value.password && (await copyToClipboard(form.value.password))) {
    passwordCopied.value = true
    setTimeout(() => (passwordCopied.value = false), 2000)
  }
}
</script>
