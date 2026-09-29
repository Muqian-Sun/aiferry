<template>
  <!--
    兑换码（A4 列表模板）：标题右侧「⋯」（导出 CSV、删除全部未使用）+「生成兑换码」；数字摘要；
    工具行 = 搜索 + 类型 / 状态筛选标签 + 刷新；行尾只有「⋯ → 删除」（仅未使用的码）；选中行时出现批量条。
  -->
  <AppLayout>
    <template #header-actions>
      <PopoverMenu width-class="w-52">
        <template #trigger="{ open }">
          <button
            type="button"
            class="btn btn-ghost btn-md px-2.5"
            :class="open ? 'bg-af-sunken text-af-ink' : ''"
            :title="t('common.more')"
            :aria-label="t('common.more')"
            data-testid="redeem-tools"
          >
            <Icon name="more" size="md" />
          </button>
        </template>
        <MenuItem icon="download" data-testid="redeem-export" @click="handleExportCodes">
          {{ t('admin.redeem.exportCsv') }}
        </MenuItem>
        <MenuItem divider />
        <MenuItem icon="trash" danger data-testid="redeem-delete-unused" @click="showDeleteUnusedDialog = true">
          {{ t('admin.redeem.deleteAllUnused') }}
        </MenuItem>
      </PopoverMenu>
      <button type="button" class="btn btn-primary btn-md" @click="showGenerateDialog = true">
        <Icon name="plus" size="md" />
        {{ t('admin.redeem.generateCodes') }}
      </button>
    </template>

    <TablePageLayout>
      <template v-if="summaryItems" #summary>
        <StatRow :items="summaryItems" data-testid="redeem-summary" />
      </template>

      <template #filters>
        <ListToolbar>
          <SearchInput
            v-model="searchQuery"
            compact
            class="w-full sm:w-64"
            :placeholder="t('admin.redeem.searchCodes')"
            @update:model-value="handleSearch"
          />
          <FilterChip
            v-model="filters.type"
            :label="t('admin.redeem.columns.type')"
            :options="typeOptions"
            test-id="filter-type"
            @change="applyFilter"
          />
          <FilterChip
            v-model="filters.status"
            :label="t('admin.redeem.columns.status')"
            :options="filterStatusOptions"
            test-id="filter-status"
            @change="applyFilter"
          />

          <template #end>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="refresh"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </template>
        </ListToolbar>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="codes"
          :loading="loading"
          row-key="id"
          selectable
          :selected-keys="selectedIds"
          :selection-label="getCodeSelectionLabel"
          :server-side-sort="true"
          default-sort-key="id"
          default-sort-order="desc"
          @sort="handleSort"
          @update:selected-keys="handleSelectedKeysUpdate"
        >
          <!-- 兑换码：等宽字体 + 复制 -->
          <template #cell-code="{ value }">
            <div class="flex items-center gap-1.5">
              <code class="font-mono text-13 text-af-ink">{{ value }}</code>
              <button
                type="button"
                class="rounded p-0.5 transition-colors"
                :class="copiedCode === value ? 'text-af-success' : 'text-af-ink-4 hover:text-af-ink-2'"
                :title="copiedCode === value ? t('admin.redeem.copied') : t('keys.copyToClipboard')"
                :aria-label="copiedCode === value ? t('admin.redeem.copied') : t('keys.copyToClipboard')"
                @click.stop="copyToClipboard(value)"
              >
                <Icon :name="copiedCode === value ? 'check' : 'copy'" size="sm" :stroke-width="2" />
              </button>
            </div>
          </template>

          <template #cell-type="{ value }">
            <span class="text-af-ink-2">{{ t('admin.redeem.types.' + value) }}</span>
          </template>

          <template #cell-value="{ value, row }">
            <span class="tabular-nums text-af-ink">
              <template v-if="row.type === 'balance'">${{ value.toFixed(2) }}</template>
              <template v-else-if="row.type === 'subscription'">
                {{ row.validity_days || 30 }} {{ t('admin.redeem.days') }}
                <span v-if="row.plan" class="ml-1 text-xs text-af-ink-3">{{ row.plan.name }}</span>
              </template>
              <template v-else-if="row.type === 'invitation'"><span class="text-af-ink-4">-</span></template>
              <template v-else>{{ value }}</template>
            </span>
          </template>

          <!-- 状态：未使用是常态（灰点）；已使用 / 已禁用淡色空心点；已过期黄点 -->
          <template #cell-status="{ value }">
            <div class="flex items-center gap-1.5">
              <span :class="['inline-block h-2 w-2 rounded-full', statusDotClass(value)]"></span>
              <span :class="statusTextClass(value)">{{ t('admin.redeem.status.' + value) }}</span>
            </div>
          </template>

          <template #cell-used_by="{ value, row }">
            <span v-if="row.user?.email || value" class="text-af-ink-2">
              {{ row.user?.email || t('common.deletedUser') }}
            </span>
            <span v-else class="text-af-ink-4">-</span>
          </template>

          <template #cell-used_at="{ value }">
            <span v-if="value" class="text-af-ink-2" :title="formatDateTime(value)">{{ formatRelativeTime(value) }}</span>
            <span v-else class="text-af-ink-4">-</span>
          </template>

          <template #cell-expires_at="{ value }">
            <span v-if="value" class="tabular-nums text-af-ink-3" :title="formatDateTime(value)">{{ formatDateOnly(value) }}</span>
            <span v-else class="text-af-ink-3">{{ t('admin.redeem.neverExpires') }}</span>
          </template>

          <!-- 已使用 / 已过期的码没有操作；占住一个图标按钮的高度，行高不跳 -->
          <template #cell-actions="{ row }">
            <div class="flex min-h-7 items-center justify-end">
              <RowActions :actions="rowActions(row)" />
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.redeem.noCodes')"
              :description="t('admin.redeem.noCodesDescription')"
              :action-text="t('admin.redeem.generateCodes')"
              @action="showGenerateDialog = true"
            />
          </template>
        </DataTable>
      </template>

      <template #bulk>
        <BulkBar :count="selectedCount" @clear="clearSelectedCodes">
          <button
            type="button"
            class="bulk-btn"
            data-test="batch-update-open"
            :disabled="batchUpdating"
            @click="openBatchUpdateDialog"
          >
            {{ t('admin.redeem.batchUpdate') }}
          </button>
          <button
            type="button"
            class="bulk-btn bulk-btn-danger"
            data-test="bulk-delete-codes"
            :disabled="bulkDeleting"
            @click="openBulkDelete"
          >
            {{ t('common.delete') }}
          </button>
        </BulkBar>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.redeem.deleteCode')"
      :message="t('admin.redeem.deleteCodeConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Bulk delete: only the selected codes that are still unused -->
    <ConfirmDialog
      :show="bulkDeleteIds.length > 0"
      :title="t('admin.redeem.bulkDelete.title')"
      :message="
        bulkDeleteSkipped > 0
          ? t('admin.redeem.bulkDelete.confirmWithSkipped', { count: bulkDeleteIds.length, skipped: bulkDeleteSkipped })
          : t('admin.redeem.bulkDelete.confirm', { count: bulkDeleteIds.length })
      "
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmBulkDelete"
      @cancel="bulkDeleteIds = []"
    />

    <!-- Delete Unused Codes Dialog -->
    <ConfirmDialog
      :show="showDeleteUnusedDialog"
      :title="t('admin.redeem.deleteAllUnused')"
      :message="t('admin.redeem.deleteAllUnusedConfirm')"
      :confirm-text="t('admin.redeem.deleteAll')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDeleteUnused"
      @cancel="showDeleteUnusedDialog = false"
    />

    <!-- Generate Codes Dialog -->
    <BaseDialog
      :show="showGenerateDialog"
      :title="t('admin.redeem.generateCodesTitle')"
      width="normal"
      @close="showGenerateDialog = false"
    >
      <form id="redeem-generate-form" class="space-y-4" @submit.prevent="handleGenerateCodes">
        <div>
          <label class="input-label">{{ t('admin.redeem.codeType') }}</label>
          <Select v-model="generateForm.type" :options="typeOptions" />
        </div>
        <!-- 余额/并发类型：显示数值输入 -->
        <div v-if="generateForm.type !== 'subscription' && generateForm.type !== 'invitation'">
          <label class="input-label">
            {{
              generateForm.type === 'balance'
                ? t('admin.redeem.amount')
                : t('admin.redeem.columns.value')
            }}
          </label>
          <input
            v-model.number="generateForm.value"
            type="number"
            :step="generateForm.type === 'balance' ? '0.01' : '1'"
            :min="generateForm.type === 'balance' ? '0.01' : '1'"
            required
            class="input"
          />
        </div>
        <!-- 邀请码类型：显示提示信息 -->
        <p v-if="generateForm.type === 'invitation'" class="rounded-lg bg-af-sunken p-3 text-sm text-af-ink-2">
          {{ t('admin.redeem.invitationHint') }}
        </p>
        <!-- 订阅类型：显示套餐选择和有效天数 -->
        <template v-if="generateForm.type === 'subscription'">
          <div>
            <label class="input-label">{{ t('admin.redeem.selectPlan') }}</label>
            <Select
              v-model="generateForm.plan_id"
              :options="planOptions"
              :placeholder="t('admin.redeem.selectPlanPlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.redeem.validityDays') }}</label>
            <input
              v-model.number="generateForm.validity_days"
              type="number"
              min="1"
              max="36500"
              required
              class="input"
            />
          </div>
        </template>
        <div>
          <label class="input-label">{{ t('admin.redeem.codeExpiry') }}</label>
          <div class="grid grid-cols-2 gap-2 sm:grid-cols-5">
            <button
              v-for="option in redeemCodeExpiryOptions"
              :key="option.value"
              type="button"
              :class="[
                'rounded-lg border px-2 py-2 text-sm transition-colors',
                generateForm.expiry_option === option.value
                  ? 'border-af-ink bg-af-sunken text-af-ink'
                  : 'border-af-hairline text-af-ink-2 hover:bg-af-sunken'
              ]"
              @click="generateForm.expiry_option = option.value"
            >
              {{ option.label }}
            </button>
          </div>
          <input
            v-if="generateForm.expiry_option === 'custom'"
            v-model.number="generateForm.custom_expiry_days"
            type="number"
            min="1"
            max="3650"
            required
            class="input mt-2"
            :placeholder="t('admin.redeem.customExpiryDays')"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.redeem.count') }}</label>
          <input
            v-model.number="generateForm.count"
            type="number"
            min="1"
            max="100"
            required
            class="input"
          />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="showGenerateDialog = false">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="redeem-generate-form" :disabled="generating" class="btn btn-primary">
            {{ generating ? t('admin.redeem.generating') : t('admin.redeem.generate') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Batch Update Dialog -->
    <BaseDialog
      :show="showBatchUpdateDialog"
      :title="t('admin.redeem.batchUpdateTitle')"
      width="normal"
      @close="closeBatchUpdateDialog"
    >
      <p class="mb-4 text-sm text-af-ink-3">
        {{ t('admin.redeem.selectedCount', { count: selectedCount }) }}
      </p>

      <form id="redeem-batch-update-form" data-test="batch-update-form" class="space-y-4" @submit.prevent="handleBatchUpdate">
        <div class="space-y-2">
          <label class="flex items-center gap-2 text-sm font-medium text-af-ink-2">
            <input
              v-model="batchUpdateForm.update_status"
              data-test="batch-field-status"
              type="checkbox"
              class="h-4 w-4 rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
            />
            {{ t('admin.redeem.batchFields.status') }}
          </label>
          <Select
            v-if="batchUpdateForm.update_status"
            v-model="batchUpdateForm.status"
            data-test="batch-status-select"
            :options="batchStatusOptions"
          />
        </div>

        <div class="space-y-2">
          <label class="flex items-center gap-2 text-sm font-medium text-af-ink-2">
            <input
              v-model="batchUpdateForm.update_expires_at"
              type="checkbox"
              class="h-4 w-4 rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
            />
            {{ t('admin.redeem.batchFields.expiresAt') }}
          </label>
          <template v-if="batchUpdateForm.update_expires_at">
            <Select v-model="batchUpdateForm.expires_mode" :options="batchExpiryModeOptions" />
            <input
              v-if="batchUpdateForm.expires_mode === 'custom'"
              v-model="batchUpdateForm.expires_at_local"
              type="datetime-local"
              class="input"
            />
            <p v-if="batchUpdateForm.expires_mode === 'custom'" class="input-hint">
              {{ t('admin.redeem.localTimeZoneHint', { timezone: browserTimeZone }) }}
            </p>
          </template>
        </div>

        <div class="space-y-2">
          <label class="flex items-center gap-2 text-sm font-medium text-af-ink-2">
            <input
              v-model="batchUpdateForm.update_notes"
              data-test="batch-field-notes"
              type="checkbox"
              class="h-4 w-4 rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
            />
            {{ t('admin.redeem.batchFields.notes') }}
          </label>
          <textarea
            v-if="batchUpdateForm.update_notes"
            v-model="batchUpdateForm.notes"
            data-test="batch-notes-input"
            rows="3"
            class="input"
            :placeholder="t('admin.redeem.batchNotesPlaceholder')"
          ></textarea>
        </div>

        <div class="space-y-2">
          <label class="flex items-center gap-2 text-sm font-medium text-af-ink-2">
            <input
              v-model="batchUpdateForm.update_plan_id"
              type="checkbox"
              class="h-4 w-4 rounded border-af-hairline-strong text-af-brand focus:ring-af-brand"
            />
            {{ t('admin.redeem.batchFields.plan') }}
          </label>
          <Select
            v-if="batchUpdateForm.update_plan_id"
            v-model="batchUpdateForm.plan_id"
            :options="batchPlanOptions"
            :placeholder="t('admin.redeem.selectPlanPlaceholder')"
          />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeBatchUpdateDialog">
            {{ t('common.cancel') }}
          </button>
          <button
            data-test="batch-update-submit"
            type="submit"
            form="redeem-batch-update-form"
            :disabled="batchUpdating"
            class="btn btn-primary"
          >
            {{ batchUpdating ? t('common.submitting') : t('admin.redeem.batchUpdate') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Generated Codes Result Dialog -->
    <BaseDialog
      :show="showResultDialog"
      :title="t('admin.redeem.generatedSuccessfully')"
      width="normal"
      @close="closeResultDialog"
    >
      <p class="mb-3 text-sm text-af-ink-3">
        {{ t('admin.redeem.codesCreated', { count: generatedCodes.length }) }}
      </p>
      <textarea
        readonly
        :value="generatedCodesText"
        :style="{ height: textareaHeight }"
        class="w-full resize-none rounded-lg border border-af-hairline bg-af-sunken p-3 font-mono text-sm text-af-ink focus:outline-none"
      ></textarea>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="copyGeneratedCodes">
            <Icon :name="copiedAll ? 'check' : 'copy'" size="sm" :stroke-width="2" />
            {{ copiedAll ? t('admin.redeem.copied') : t('admin.redeem.copyAll') }}
          </button>
          <button type="button" class="btn btn-primary" @click="downloadGeneratedCodes">
            <Icon name="download" size="sm" :stroke-width="2" />
            {{ t('admin.redeem.download') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import { useTableSelection } from '@/composables/useTableSelection'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import {
  formatDateOnly,
  formatDateTime,
  formatRelativeTime,
  getBrowserTimeZone,
  parseDateTimeLocalInput
} from '@/utils/format'
import type {
  RedeemCode,
  RedeemCodeType,
  BatchUpdateRedeemCodeFields
} from '@/types'
import type { SubscriptionPlan } from '@/types/payment'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import StatRow from '@/components/user/shell/StatRow.vue'
import type { StatItem } from '@/components/user/shell/types'
import { BulkBar, FilterChip, ListToolbar, MenuItem, PopoverMenu, RowActions } from '@/components/admin/list'
import type { RowAction } from '@/components/admin/list'

const { t } = useI18n()
const { copyToClipboard: clipboardCopy } = useClipboard()
const browserTimeZone = getBrowserTimeZone()

const showGenerateDialog = ref(false)
const showResultDialog = ref(false)
const generatedCodes = ref<RedeemCode[]>([])
const plans = ref<SubscriptionPlan[]>([])

// 订阅类型：套餐选项
const planOptions = computed(() => plans.value.map((p) => ({ value: p.id, label: p.name })))

const batchPlanOptions = computed(() => [
  { value: null, label: t('admin.redeem.clearPlan') },
  ...planOptions.value
])

const generatedCodesText = computed(() => {
  return generatedCodes.value.map((code) => code.code).join('\n')
})

const textareaHeight = computed(() => {
  const lineCount = generatedCodes.value.length
  const lineHeight = 24 // approximate line height in px
  const padding = 24 // top + bottom padding
  const minHeight = 60
  const maxHeight = 240
  const calculatedHeight = Math.min(
    Math.max(lineCount * lineHeight + padding, minHeight),
    maxHeight
  )
  return `${calculatedHeight}px`
})

const copiedAll = ref(false)

const closeResultDialog = () => {
  showResultDialog.value = false
  generatedCodes.value = []
  copiedAll.value = false
}

const copyGeneratedCodes = async () => {
  const success = await clipboardCopy(generatedCodesText.value)
  if (success) {
    copiedAll.value = true
    setTimeout(() => {
      copiedAll.value = false
    }, 2000)
  }
}

const downloadGeneratedCodes = () => {
  const blob = new Blob([generatedCodesText.value], { type: 'text/plain' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.txt`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}

const columns = computed<Column[]>(() => [
  { key: 'code', label: t('admin.redeem.columns.code') },
  { key: 'type', label: t('admin.redeem.columns.type'), sortable: true },
  { key: 'value', label: t('admin.redeem.columns.value'), sortable: true },
  { key: 'status', label: t('admin.redeem.columns.status'), sortable: true },
  { key: 'used_by', label: t('admin.redeem.columns.usedBy') },
  { key: 'used_at', label: t('admin.redeem.columns.usedAt'), sortable: true },
  { key: 'expires_at', label: t('admin.redeem.columns.expiresAt'), sortable: true },
  { key: 'actions', label: t('admin.redeem.columns.actions') }
])

// 生成表单的类型选项，也用作「类型」筛选标签的选项（筛选标签自带「全部」）
const typeOptions = computed(() => [
  { value: 'balance', label: t('admin.redeem.balance') },
  { value: 'concurrency', label: t('admin.redeem.concurrency') },
  { value: 'subscription', label: t('admin.redeem.subscription') },
  { value: 'invitation', label: t('admin.redeem.invitation') }
])

const filterStatusOptions = computed(() => [
  { value: 'unused', label: t('admin.redeem.unused') },
  { value: 'used', label: t('admin.redeem.used') },
  { value: 'expired', label: t('admin.redeem.status.expired') },
  { value: 'disabled', label: t('admin.redeem.status.disabled') }
])

const batchStatusOptions = computed(() => [
  { value: 'unused', label: t('admin.redeem.status.unused') },
  { value: 'disabled', label: t('admin.redeem.status.disabled') }
])

const batchExpiryModeOptions = computed(() => [
  { value: 'clear', label: t('admin.redeem.neverExpires') },
  { value: 'custom', label: t('admin.redeem.customExpiry') }
])

// 状态：未使用是常态（灰点）；已使用 / 已禁用是「用完 / 停掉」的淡色空心点；已过期是没人动它却失效了，黄点
const statusDotClass = (status: string) => {
  if (status === 'unused') return 'bg-af-ink-4'
  if (status === 'expired') return 'bg-af-warning'
  return 'border border-af-ink-4'
}
const statusTextClass = (status: string) => {
  if (status === 'unused') return 'text-af-ink-2'
  if (status === 'expired') return 'text-af-warning'
  return 'text-af-ink-3'
}

const codes = ref<RedeemCode[]>([])
const loading = ref(false)
const generating = ref(false)
const batchUpdating = ref(false)
const bulkDeleting = ref(false)
const searchQuery = ref('')
const filters = reactive({
  type: '',
  status: ''
})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = reactive({
  sort_by: 'id',
  sort_order: 'desc' as 'asc' | 'desc'
})

let abortController: AbortController | null = null

const showDeleteDialog = ref(false)
const showDeleteUnusedDialog = ref(false)
const showBatchUpdateDialog = ref(false)
const deletingCode = ref<RedeemCode | null>(null)
const copiedCode = ref<string | null>(null)

const {
  selectedSet: selectedCodeIds,
  selectedIds,
  selectedCount,
  setSelectedIds,
  clear: clearSelectedCodes,
  removeMany: removeSelectedCodes
} = useTableSelection<RedeemCode>({
  rows: codes,
  getId: (code) => code.id
})

const handleSelectedKeysUpdate = (keys: Array<string | number>) => {
  setSelectedIds(keys.filter((key): key is number => typeof key === 'number'))
}

const getCodeSelectionLabel = (code: RedeemCode) => code.code

// 选中可以跨页，批量删除要知道每个选中码的状态：记下翻过的每一页里码的状态
const knownStatus = new Map<number, RedeemCode['status']>()

const batchUpdateForm = reactive({
  update_status: false,
  status: 'disabled' as 'unused' | 'disabled',
  update_expires_at: false,
  expires_mode: 'clear' as 'clear' | 'custom',
  expires_at_local: '',
  update_notes: false,
  notes: '',
  update_plan_id: false,
  plan_id: null as number | null
})

type RedeemCodeExpiryOption = 'never' | '1' | '3' | '7' | 'custom'

const redeemCodeExpiryOptions = computed<{ value: RedeemCodeExpiryOption; label: string }[]>(() => [
  { value: 'never', label: t('admin.redeem.neverExpires') },
  { value: '1', label: t('admin.redeem.expiryPresetDays', { days: 1 }) },
  { value: '3', label: t('admin.redeem.expiryPresetDays', { days: 3 }) },
  { value: '7', label: t('admin.redeem.expiryPresetDays', { days: 7 }) },
  { value: 'custom', label: t('admin.redeem.customExpiry') }
])

const generateForm = reactive({
  type: 'balance' as RedeemCodeType,
  value: 10,
  count: 1,
  plan_id: null as number | null,
  validity_days: 30,
  expiry_option: 'never' as RedeemCodeExpiryOption,
  custom_expiry_days: 7
})

// 监听类型变化，邀请码类型时自动设置 value 为 0
watch(
  () => generateForm.type,
  (newType) => {
    if (newType === 'invitation') {
      generateForm.value = 0
    } else if (generateForm.value === 0) {
      generateForm.value = 10
    }
  }
)

// 数字摘要：全部 / 未使用 / 已使用 / 已过期。/admin/redeem-codes/stats 后端还是占位（全 0），
// 所以用列表接口按状态各取 1 条读 total；接口失败或一个码都没有就不显示，不摆一排 0。
const summary = ref<{ total: number; unused: number; used: number; expired: number } | null>(null)
const summaryItems = computed<StatItem[] | null>(() => {
  const s = summary.value
  if (!s || s.total === 0) return null
  const fmt = (n: number) => n.toLocaleString()
  return [
    { key: 'total', label: t('admin.redeem.summary.total'), value: fmt(s.total) },
    { key: 'unused', label: t('admin.redeem.status.unused'), value: fmt(s.unused) },
    { key: 'used', label: t('admin.redeem.status.used'), value: fmt(s.used) },
    { key: 'expired', label: t('admin.redeem.status.expired'), value: fmt(s.expired) }
  ]
})
const loadSummary = async () => {
  try {
    const [all, unused, used, expired] = await Promise.all([
      adminAPI.redeem.list(1, 1),
      adminAPI.redeem.list(1, 1, { status: 'unused' }),
      adminAPI.redeem.list(1, 1, { status: 'used' }),
      adminAPI.redeem.list(1, 1, { status: 'expired' })
    ])
    summary.value = { total: all.total, unused: unused.total, used: used.total, expired: expired.total }
  } catch {
    summary.value = null
  }
}

const buildRedeemQueryFilters = () => ({
  type: (filters.type || undefined) as RedeemCodeType | undefined,
  status: (filters.status || undefined) as 'used' | 'expired' | 'unused' | 'disabled' | undefined,
  search: searchQuery.value || undefined,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})

const loadCodes = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentController = new AbortController()
  abortController = currentController
  loading.value = true
  try {
    const response = await adminAPI.redeem.list(
      pagination.page,
      pagination.page_size,
      buildRedeemQueryFilters(),
      {
        signal: currentController.signal
      }
    )
    if (currentController.signal.aborted) {
      return
    }
    codes.value = response.items
    response.items.forEach((code) => knownStatus.set(code.id, code.status))
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error: any) {
    if (
      currentController.signal.aborted ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    console.error('Error loading redeem codes:', error)
  } finally {
    if (abortController === currentController && !currentController.signal.aborted) {
      loading.value = false
      abortController = null
    }
  }
}

/** 列表和摘要一起刷新（增删改之后、点刷新按钮） */
const refresh = () => {
  loadCodes()
  loadSummary()
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadCodes()
  }, 300)
}

const applyFilter = () => {
  pagination.page = 1
  loadCodes()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadCodes()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadCodes()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadCodes()
}

// 行操作（A4）：只有未使用的码能删，删除进「⋯」、红字
const rowActions = (code: RedeemCode): RowAction[] => {
  if (code.status !== 'unused') return []
  return [
    { key: 'delete', label: t('common.delete'), icon: 'trash', danger: true, onSelect: () => handleDelete(code) }
  ]
}

const getRedeemCodeExpiresInDays = () => {
  if (generateForm.expiry_option === 'never') {
    return undefined
  }
  if (generateForm.expiry_option === 'custom') {
    if (
      !Number.isFinite(generateForm.custom_expiry_days) ||
      generateForm.custom_expiry_days < 1
    ) {
      return null
    }
    return Math.floor(generateForm.custom_expiry_days)
  }
  return Number(generateForm.expiry_option)
}

const toDatetimeLocalInputValue = (date: Date) => {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours()
  )}:${pad(date.getMinutes())}`
}

const resetBatchUpdateForm = () => {
  batchUpdateForm.update_status = false
  batchUpdateForm.status = 'disabled'
  batchUpdateForm.update_expires_at = false
  batchUpdateForm.expires_mode = 'clear'
  batchUpdateForm.expires_at_local = toDatetimeLocalInputValue(
    new Date(Date.now() + 24 * 60 * 60 * 1000)
  )
  batchUpdateForm.update_notes = false
  batchUpdateForm.notes = ''
  batchUpdateForm.update_plan_id = false
  batchUpdateForm.plan_id = null
}

const openBatchUpdateDialog = () => {
  if (selectedCount.value === 0) {
    return
  }
  resetBatchUpdateForm()
  showBatchUpdateDialog.value = true
}

const closeBatchUpdateDialog = () => {
  showBatchUpdateDialog.value = false
}

const buildBatchUpdateFields = (): BatchUpdateRedeemCodeFields | null => {
  const fields: BatchUpdateRedeemCodeFields = {}

  if (batchUpdateForm.update_status) {
    fields.status = batchUpdateForm.status
  }
  if (batchUpdateForm.update_expires_at) {
    if (batchUpdateForm.expires_mode === 'clear') {
      fields.expires_at = null
    } else {
      const expiresAt = parseDateTimeLocalInput(batchUpdateForm.expires_at_local)
      if (expiresAt === null) {
        console.error(t('admin.redeem.expiryDateRequired'))
        return null
      }
      fields.expires_at = new Date(expiresAt * 1000).toISOString()
    }
  }
  if (batchUpdateForm.update_notes) {
    fields.notes = batchUpdateForm.notes
  }
  if (batchUpdateForm.update_plan_id) {
    fields.plan_id =
      batchUpdateForm.plan_id == null ? null : Number(batchUpdateForm.plan_id)
  }

  return Object.keys(fields).length > 0 ? fields : null
}

const handleGenerateCodes = async () => {
  // 订阅类型必须选择套餐
  if (generateForm.type === 'subscription' && !generateForm.plan_id) {
    console.error(t('admin.redeem.planRequired'))
    return
  }

  const expiresInDays = getRedeemCodeExpiresInDays()
  if (expiresInDays === null) {
    console.error(t('admin.redeem.expiryDaysRequired'))
    return
  }

  generating.value = true
  try {
    const result = await adminAPI.redeem.generate(
      generateForm.count,
      generateForm.type,
      generateForm.value,
      generateForm.type === 'subscription' ? generateForm.plan_id : undefined,
      generateForm.type === 'subscription' ? generateForm.validity_days : undefined,
      expiresInDays
    )
    showGenerateDialog.value = false
    generatedCodes.value = result
    showResultDialog.value = true
    // 重置表单
    generateForm.plan_id = null
    generateForm.validity_days = 30
    generateForm.expiry_option = 'never'
    generateForm.custom_expiry_days = 7
    refresh()
  } catch (error: any) {
    console.error('Error generating codes:', error)
  } finally {
    generating.value = false
  }
}

const copyToClipboard = async (text: string) => {
  const success = await clipboardCopy(text)
  if (success) {
    copiedCode.value = text
    setTimeout(() => {
      copiedCode.value = null
    }, 2000)
  }
}

const handleExportCodes = async () => {
  try {
    const blob = await adminAPI.redeem.exportCodes(buildRedeemQueryFilters())

    // Create download link
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
  } catch (error: any) {
    console.error('Error exporting codes:', error)
  }
}

const handleDelete = (code: RedeemCode) => {
  deletingCode.value = code
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingCode.value) return

  try {
    await adminAPI.redeem.delete(deletingCode.value.id)
    showDeleteDialog.value = false
    removeSelectedCodes([deletingCode.value.id])
    deletingCode.value = null
    refresh()
  } catch (error: any) {
    console.error('Error deleting code:', error)
  }
}

// 批量删除：和行尾一样只删未使用的码，已使用 / 已过期 / 已禁用的跳过（确认框里写明）
const bulkDeleteIds = ref<number[]>([])
const bulkDeleteSkipped = ref(0)

const openBulkDelete = () => {
  const ids = selectedIds.value.filter((id) => knownStatus.get(id) === 'unused')
  if (ids.length === 0) {
    return
  }
  bulkDeleteSkipped.value = selectedIds.value.length - ids.length
  bulkDeleteIds.value = ids
}

const confirmBulkDelete = async () => {
  const ids = bulkDeleteIds.value
  if (ids.length === 0) return
  bulkDeleting.value = true
  try {
    await adminAPI.redeem.batchDelete(ids)
    bulkDeleteIds.value = []
    clearSelectedCodes()
    refresh()
  } catch (error: any) {
    console.error('Error bulk deleting codes:', error)
  } finally {
    bulkDeleting.value = false
  }
}

const confirmDeleteUnused = async () => {
  try {
    // Get all unused codes and delete them
    const unusedCodesResponse = await adminAPI.redeem.list(1, 1000, { status: 'unused' })
    const unusedCodeIds = unusedCodesResponse.items.map((code) => code.id)

    if (unusedCodeIds.length === 0) {
      showDeleteUnusedDialog.value = false
      return
    }

    await adminAPI.redeem.batchDelete(unusedCodeIds)
    showDeleteUnusedDialog.value = false
    removeSelectedCodes(unusedCodeIds)
    refresh()
  } catch (error: any) {
    console.error('Error deleting unused codes:', error)
  }
}

const handleBatchUpdate = async () => {
  const ids = Array.from(selectedCodeIds.value)
  if (ids.length === 0) {
    return
  }

  const hasSelectedFields =
    batchUpdateForm.update_status ||
    batchUpdateForm.update_expires_at ||
    batchUpdateForm.update_notes ||
    batchUpdateForm.update_plan_id
  if (!hasSelectedFields) {
    console.error(t('admin.redeem.noBatchFieldsSelected'))
    return
  }

  const fields = buildBatchUpdateFields()
  if (!fields) {
    return
  }

  batchUpdating.value = true
  try {
    await adminAPI.redeem.batchUpdate(ids, fields)
    showBatchUpdateDialog.value = false
    clearSelectedCodes()
    refresh()
  } catch (error: any) {
    console.error('Error batch updating codes:', error)
  } finally {
    batchUpdating.value = false
  }
}

// 加载套餐（订阅类兑换码用）
const loadPlans = async () => {
  try {
    const res = await adminPaymentAPI.getPlans()
    plans.value = res.data || []
  } catch (error) {
    console.error('Error loading plans:', error)
  }
}

onMounted(() => {
  loadCodes()
  loadSummary()
  loadPlans()
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>
