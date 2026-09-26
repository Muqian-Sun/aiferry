<template>
  <!--
    套餐（A4 列表模板）：与「订阅」同组共用页头，本页签的主操作「新建套餐」在标题右侧；
    工具行只有刷新；上架开关留在行内；行尾「编辑」图标 + 「⋯」（删除，红字、走确认框）。
  -->
  <AppLayout>
    <template #header-actions>
      <button type="button" class="btn btn-primary btn-md" @click="openPlanEdit(null)">
        <Icon name="plus" size="md" />
        {{ t('payment.admin.createPlan') }}
      </button>
    </template>

    <TablePageLayout>
      <template #filters>
        <ListToolbar>
          <template #end>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="plansLoading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadPlans"
            >
              <Icon name="refresh" size="md" :class="plansLoading ? 'animate-spin' : ''" />
            </button>
          </template>
        </ListToolbar>
      </template>

      <template #table>
        <DataTable :columns="planColumns" :data="plans" :loading="plansLoading">
          <template #cell-name="{ value }">
            <span class="font-medium text-af-ink">{{ value }}</span>
          </template>
          <template #cell-limits="{ row }">
            <span class="whitespace-nowrap text-af-ink-2">{{ formatPlanLimits(row) }}</span>
          </template>
          <template #cell-models="{ row }">
            <span
              class="block max-w-[15rem] truncate text-af-ink-2"
              :title="(row.models || []).map(modelLabel).join(' / ')"
            >
              {{ (row.models || []).map(modelLabel).join(' / ') || '-' }}
            </span>
          </template>
          <template #cell-price="{ value, row }">
            <div class="whitespace-nowrap tabular-nums">
              <span class="font-medium text-af-ink">${{ (value ?? 0).toFixed(2) }}</span>
              <span v-if="row.original_price" class="ml-1 text-xs text-af-ink-4 line-through">${{ row.original_price.toFixed(2) }}</span>
            </div>
          </template>
          <template #cell-validity_days="{ value, row }">
            <span class="whitespace-nowrap tabular-nums">{{ value }} {{ t('payment.admin.' + (row.validity_unit || 'days')) }}</span>
          </template>
          <template #cell-for_sale="{ value, row }">
            <button
              type="button"
              role="switch"
              :aria-checked="value ? 'true' : 'false'"
              :aria-label="value ? t('payment.admin.onSale') : t('payment.admin.offSale')"
              :title="value ? t('payment.admin.onSale') : t('payment.admin.offSale')"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-af-brand focus:ring-offset-2',
                value ? 'bg-af-brand' : 'bg-af-ink-4'
              ]"
              @click="toggleForSale(row)"
            >
              <span :class="[
                'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-af-sheet ring-0 transition duration-200 ease-in-out',
                value ? 'translate-x-4' : 'translate-x-0'
              ]" />
            </button>
          </template>
          <template #cell-sort_order="{ value }">
            <span class="tabular-nums text-af-ink-3">{{ value }}</span>
          </template>
          <template #cell-actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>

          <template #empty>
            <EmptyState
              :title="t('payment.admin.noPlansYet')"
              :description="t('payment.admin.createFirstPlan')"
              :action-text="t('payment.admin.createPlan')"
              @action="openPlanEdit(null)"
            />
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <!-- Plan Edit Dialog -->
    <PlanEditDialog :show="showPlanDialog" :plan="editingPlan" :payment-config="paymentConfig" @close="showPlanDialog = false" @saved="loadPlans" />

    <ConfirmDialog :show="showDeletePlanDialog" :title="t('payment.admin.deletePlan')" :message="t('payment.admin.deletePlanConfirm')" :confirm-text="t('common.delete')" danger @confirm="handleDeletePlan" @cancel="showDeletePlanDialog = false" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { AdminPaymentConfig } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { SubscriptionPlan } from '@/types/payment'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { ListToolbar, RowActions } from '@/components/admin/list'
import type { RowAction } from '@/components/admin/list'
import PlanEditDialog from './PlanEditDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()

const paymentConfig = ref<AdminPaymentConfig | null>(null)

async function loadPaymentConfig() {
  try {
    const res = await adminPaymentAPI.getConfig()
    paymentConfig.value = res.data
  } catch { /* preview only */ }
}

/** 三档限额拼串：日 / 周 / 月，null = 不限 */
function formatPlanLimits(plan: SubscriptionPlan): string {
  const fmt = (v: number | null | undefined) => (v == null ? t('payment.admin.unlimited') : `$${v}`)
  return [
    `${t('payment.admin.dailyLimitShort')} ${fmt(plan.daily_limit_usd)}`,
    `${t('payment.admin.weeklyLimitShort')} ${fmt(plan.weekly_limit_usd)}`,
    `${t('payment.admin.monthlyLimitShort')} ${fmt(plan.monthly_limit_usd)}`,
  ].join(' · ')
}

function modelLabel(m: { model_id: string; display_name: string }): string {
  return m.display_name || m.model_id
}


// ==================== Plans ====================

const plansLoading = ref(false)
const plans = ref<SubscriptionPlan[]>([])
const showPlanDialog = ref(false)
const showDeletePlanDialog = ref(false)
const editingPlan = ref<SubscriptionPlan | null>(null)
const deletingPlanId = ref<number | null>(null)

const planColumns = computed((): Column[] => [
  { key: 'name', label: t('payment.admin.planName') },
  { key: 'limits', label: t('payment.admin.limits') },
  { key: 'models', label: t('payment.admin.models') },
  { key: 'price', label: t('payment.admin.price') },
  { key: 'validity_days', label: t('payment.admin.validity') },
  { key: 'for_sale', label: t('payment.admin.forSale') },
  { key: 'sort_order', label: t('payment.admin.sortOrder') },
  { key: 'actions', label: t('common.actions') },
])

async function loadPlans() {
  plansLoading.value = true
  try {
    const res = await adminPaymentAPI.getPlans()
    // Backend returns features as newline-separated string; parse to array
    plans.value = (res.data || []).map((p: Omit<SubscriptionPlan, 'features'> & { features: string | string[] }) => ({
      ...p,
      features: typeof p.features === 'string'
        ? p.features.split('\n').map((f: string) => f.trim()).filter(Boolean)
        : (p.features || []),
    }))
  }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { plansLoading.value = false }
}

function openPlanEdit(plan: SubscriptionPlan | null) {
  editingPlan.value = plan
  showPlanDialog.value = true
}

// 行操作（A4）：编辑是图标；删除进「⋯」、红字、走确认框
function rowActions(plan: SubscriptionPlan): RowAction[] {
  return [
    { key: 'edit', label: t('common.edit'), icon: 'edit', primary: true, onSelect: () => openPlanEdit(plan) },
    { key: 'delete', label: t('common.delete'), icon: 'trash', danger: true, onSelect: () => confirmDeletePlan(plan) }
  ]
}


/** Quick toggle for_sale from the list */
async function toggleForSale(plan: SubscriptionPlan) {
  try {
    await adminPaymentAPI.updatePlan(plan.id, { for_sale: !plan.for_sale })
    plan.for_sale = !plan.for_sale
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

function confirmDeletePlan(plan: SubscriptionPlan) { deletingPlanId.value = plan.id; showDeletePlanDialog.value = true }
async function handleDeletePlan() {
  if (!deletingPlanId.value) return
  try { await adminPaymentAPI.deletePlan(deletingPlanId.value); appStore.showSuccess(t('common.deleted')); showDeletePlanDialog.value = false; loadPlans() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

// ==================== Lifecycle ====================

onMounted(() => {
  loadPaymentConfig()
  loadPlans()
})
</script>
