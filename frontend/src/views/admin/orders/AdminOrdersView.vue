<template>
  <!--
    订单（A4 列表模板）：与「收款概览」同组共用页头，没有新建类操作，标题右侧留空；
    工具行 = 搜索 + 状态 / 支付方式 / 订单类型筛选标签 + 刷新；
    行尾「详情」图标 + 「⋯」（按状态：取消、重试、批准退款 / 重试退款 / 查询退款状态、退款）。
    表格沿用 OrderTable，只从外面塞操作列与空状态，表格本身的样子不动。
  -->
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <ListToolbar>
          <SearchInput
            v-model="orderSearch"
            compact
            class="w-full sm:w-64"
            :placeholder="t('payment.admin.searchOrders')"
            @update:model-value="debounceLoadOrders"
          />
          <FilterChip
            v-model="orderFilters.status"
            :label="t('payment.orders.status')"
            :options="statusFilterOptions"
            test-id="filter-order-status"
            @change="loadOrders"
          />
          <FilterChip
            v-model="orderFilters.payment_type"
            :label="t('payment.orders.paymentMethod')"
            :options="paymentTypeFilterOptions"
            test-id="filter-payment-type"
            @change="loadOrders"
          />
          <FilterChip
            v-model="orderFilters.order_type"
            :label="t('payment.admin.orderType')"
            :options="orderTypeFilterOptions"
            test-id="filter-order-type"
            @change="loadOrders"
          />

          <template #end>
            <button
              type="button"
              class="rounded-md p-2 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink disabled:opacity-40"
              :disabled="ordersLoading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadOrders"
            >
              <Icon name="refresh" size="md" :class="ordersLoading ? 'animate-spin' : ''" />
            </button>
          </template>
        </ListToolbar>
        <!-- 列表加载、取消 / 重试 / 查询退款失败的原因；原来只打控制台 -->
        <FormError class="mt-2" :message="loadError || actionError" data-testid="orders-action-error" />
      </template>

      <template #table>
        <OrderTable :orders="orders" :loading="ordersLoading" show-user>
          <template #actions="{ row }">
            <RowActions :actions="rowActions(row)" />
          </template>
          <template #empty>
            <EmptyState :title="t('payment.orders.empty')" :description="t('payment.admin.ordersEmptyHint')" />
          </template>
        </OrderTable>
      </template>

      <template #pagination>
        <Pagination v-if="orderPagination.total > 0" :page="orderPagination.page" :total="orderPagination.total" :page-size="orderPagination.page_size" @update:page="handleOrderPageChange" @update:pageSize="handleOrderPageSizeChange" />
      </template>
    </TablePageLayout>

    <!-- Order Detail Dialog -->
    <BaseDialog :show="showDetailDialog" :title="t('payment.admin.orderDetail')" width="wide" @close="showDetailDialog = false">
      <div v-if="selectedOrder" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <!-- 订单只认订单编号（和支付宝 / 微信账单对得上），不写内部 ID；编号可复制 -->
          <div class="col-span-2">
            <p class="text-xs text-af-ink-3">{{ t('payment.orders.orderNo') }}</p>
            <p class="inline-flex max-w-full items-center gap-1.5 text-sm font-medium text-af-ink">
              <span class="break-all font-mono" data-testid="order-detail-out-trade-no">{{ selectedOrder.out_trade_no }}</span>
              <CopyButton
                :text="selectedOrder.out_trade_no"
                class="shrink-0 rounded p-0.5 text-af-ink-3 transition-colors hover:bg-af-sunken hover:text-af-ink-2"
                test-id="order-detail-copy-out-trade-no"
              />
            </p>
          </div>
          <div><p class="text-xs text-af-ink-3">{{ t('payment.orders.status') }}</p><OrderStatusBadge :status="selectedOrder.status" /></div>
          <div><p class="text-xs text-af-ink-3">{{ t('payment.orders.amount') }}</p><p class="text-sm font-medium text-af-ink">{{ creditedAmountSymbol }}{{ selectedOrder.amount.toFixed(2) }}</p></div>
          <div><p class="text-xs text-af-ink-3">{{ t('payment.orders.payAmount') }}</p><p class="text-sm font-medium text-af-ink">{{ paymentAmountSymbol(selectedOrder) }}{{ selectedOrder.pay_amount.toFixed(2) }}</p></div>
          <div><p class="text-xs text-af-ink-3">{{ t('payment.orders.paymentMethod') }}</p><p class="text-sm text-af-ink-2">{{ t('payment.methods.' + selectedOrder.payment_type, selectedOrder.payment_type) }}</p></div>
          <div><p class="text-xs text-af-ink-3">{{ t('payment.admin.feeRate') }}</p><p class="text-sm text-af-ink-2">{{ selectedOrder.fee_rate }}%</p></div>
          <div><p class="text-xs text-af-ink-3">{{ t('payment.orders.createdAt') }}</p><p class="text-sm text-af-ink-2">{{ formatDateTime(selectedOrder.created_at) }}</p></div>
          <div><p class="text-xs text-af-ink-3">{{ t('payment.admin.expiresAt') }}</p><p class="text-sm text-af-ink-2">{{ formatDateTime(selectedOrder.expires_at) }}</p></div>
          <div v-if="selectedOrder.paid_at"><p class="text-xs text-af-ink-3">{{ t('payment.admin.paidAt') }}</p><p class="text-sm text-af-ink-2">{{ formatDateTime(selectedOrder.paid_at) }}</p></div>
          <div v-if="selectedOrder.refund_amount"><p class="text-xs text-af-ink-3">{{ t('payment.admin.refundAmount') }}</p><p class="text-sm font-medium text-af-danger">{{ creditedAmountSymbol }}{{ selectedOrder.refund_amount.toFixed(2) }}</p></div>
          <div v-if="selectedOrder.refund_reason" class="col-span-2"><p class="text-xs text-af-ink-3">{{ t('payment.admin.refundReason') }}</p><p class="text-sm text-af-ink-2">{{ selectedOrder.refund_reason }}</p></div>
          <!-- Refund request info -->
          <div v-if="selectedOrder.refund_requested_at" class="col-span-2 border-t border-af-hairline pt-3">
            <p class="mb-2 text-xs font-medium text-af-ink-2">{{ t('payment.admin.refundRequestInfo') }}</p>
            <div class="grid grid-cols-2 gap-4">
              <div>
                <p class="text-xs text-af-ink-3">{{ t('payment.admin.refundRequestedAt') }}</p>
                <p class="text-sm text-af-ink-2">{{ formatDateTime(selectedOrder.refund_requested_at) }}</p>
              </div>
              <div>
                <p class="text-xs text-af-ink-3">{{ t('payment.admin.refundRequestedBy') }}</p>
                <p class="text-sm text-af-ink-2">{{ refundRequesterLabel(selectedOrder) }}</p>
              </div>
              <div class="col-span-2">
                <p class="text-xs text-af-ink-3">{{ t('payment.admin.refundRequestReason') }}</p>
                <p class="text-sm text-af-ink-2">{{ selectedOrder.refund_request_reason }}</p>
              </div>
            </div>
          </div>
        </div>
        <!-- Audit Logs -->
        <div v-if="orderAuditLogs.length > 0" class="border-t border-af-hairline pt-4">
          <p class="mb-2 text-xs font-medium text-af-ink-3">{{ t('payment.admin.auditLogs') }}</p>
          <div class="max-h-48 space-y-2 overflow-y-auto">
            <div v-for="log in orderAuditLogs" :key="log.id" class="rounded-lg border border-af-hairline bg-af-sunken p-2.5">
              <div class="flex items-center justify-between">
                <span class="text-xs font-medium text-af-ink-2">{{ log.action }}</span>
                <span class="text-xs text-af-ink-3">{{ formatDateTime(log.created_at) }}</span>
              </div>
              <div v-if="log.detail" class="mt-1 break-all text-xs text-af-ink-3">{{ log.detail }}</div>
              <div v-if="log.operator" class="mt-1 text-xs text-af-ink-3">{{ t('payment.admin.operator') }}: {{ operatorLabel(selectedOrder, log.operator) }}</div>
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>

    <AdminRefundDialog :show="showRefundDialog" :order="selectedOrder" :submitting="refundSubmitting" :require-force="refundRequireForce" :warning="refundWarning" @confirm="handleRefund" @cancel="closeRefundDialog" />
  </AppLayout>
</template>

<script setup lang="ts">
import CopyButton from '@/components/common/CopyButton.vue'
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import FormError from '@/components/common/FormError.vue'
import { formatOrderDateTime } from '@/components/payment/orderUtils'
import type { PaymentOrder } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Icon from '@/components/icons/Icon.vue'
import { FilterChip, ListToolbar, RowActions } from '@/components/admin/list'
import type { RowAction } from '@/components/admin/list'
import AdminRefundDialog from '@/components/admin/payment/AdminRefundDialog.vue'
import OrderStatusBadge from '@/components/payment/OrderStatusBadge.vue'
import OrderTable from '@/components/payment/OrderTable.vue'
import { currencySymbol } from '@/components/payment/currency'

interface AuditLog {
  id: number
  action: string
  detail: string | null
  operator: string | null
  created_at: string
}

const { t, te } = useI18n()

const ordersLoading = ref(false)
const orders = ref<PaymentOrder[]>([])
const orderSearch = ref('')
const orderFilters = reactive({ status: '', payment_type: '', order_type: '' })
const orderPagination = reactive({ page: 1, page_size: 20, total: 0 })
const selectedOrder = ref<PaymentOrder | null>(null)
const showDetailDialog = ref(false)
const showRefundDialog = ref(false)
const refundSubmitting = ref(false)
const refundRequireForce = ref(false)
const refundWarning = ref('')
const refundQueryingIds = ref(new Set<number>())
const orderAuditLogs = ref<AuditLog[]>([])
const creditedAmountSymbol = currencySymbol('USD')

function paymentAmountSymbol(order: PaymentOrder | null | undefined): string {
  return currencySymbol(order?.currency)
}

let debounceTimer: ReturnType<typeof setTimeout> | null = null
function debounceLoadOrders() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => loadOrders(), 300)
}

const loadError = ref('')
const actionError = ref('')

async function loadOrders() {
  ordersLoading.value = true
  loadError.value = ''
  try {
    const res = await adminPaymentAPI.getOrders({
      page: orderPagination.page, page_size: orderPagination.page_size,
      keyword: orderSearch.value || undefined, status: orderFilters.status || undefined,
      payment_type: orderFilters.payment_type || undefined, order_type: orderFilters.order_type || undefined,
    })
    orders.value = res.data.items || []
    orderPagination.total = res.data.total || 0
  } catch (err: unknown) {
    loadError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))
  } finally { ordersLoading.value = false }
}

function handleOrderPageChange(page: number) { orderPagination.page = page; loadOrders() }
function handleOrderPageSizeChange(size: number) { orderPagination.page_size = size; orderPagination.page = 1; loadOrders() }

// 筛选标签的选项（「全部」= 清掉标签，不作为选项）
const statusFilterOptions = computed(() => [
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'PAID', label: t('payment.status.paid') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'EXPIRED', label: t('payment.status.expired') },
  { value: 'CANCELLED', label: t('payment.status.cancelled') },
  { value: 'FAILED', label: t('payment.status.failed') },
  { value: 'REFUNDED', label: t('payment.status.refunded') },
  { value: 'REFUND_REQUESTED', label: t('payment.status.refund_requested') },
  { value: 'REFUND_PENDING', label: t('payment.status.refund_pending') },
  { value: 'REFUND_FAILED', label: t('payment.status.refund_failed') },
])

const paymentTypeFilterOptions = computed(() => [
  { value: 'alipay', label: t('payment.methods.alipay') },
  { value: 'wxpay', label: t('payment.methods.wxpay') },
  { value: 'stripe', label: t('payment.methods.stripe') },
  { value: 'airwallex', label: t('payment.methods.airwallex') },
])

const orderTypeFilterOptions = computed(() => [
  { value: 'balance', label: t('payment.admin.balanceOrder') },
  { value: 'subscription', label: t('payment.admin.subscriptionOrder') },
])

// 行操作（A4）：「详情」是图标；其余按订单状态进「⋯」。退款红字，走退款确认框
function rowActions(order: PaymentOrder): RowAction[] {
  const actions: RowAction[] = [
    { key: 'detail', label: t('payment.admin.orderDetail'), icon: 'eye', primary: true, onSelect: () => showOrderDetail(order) }
  ]
  if (order.status === 'PENDING') {
    actions.push({ key: 'cancel', label: t('payment.orders.cancel'), icon: 'x', onSelect: () => handleCancelOrder(order) })
  }
  if (order.status === 'FAILED') {
    actions.push({ key: 'retry', label: t('payment.admin.retry'), icon: 'refresh', onSelect: () => handleRetryOrder(order) })
  }
  if (order.status === 'REFUND_REQUESTED') {
    const amount = order.refund_amount ? ` ${creditedAmountSymbol}${order.refund_amount.toFixed(2)}` : ''
    actions.push({ key: 'approve-refund', label: `${t('payment.admin.approveRefund')}${amount}`, icon: 'check', onSelect: () => openRefundDialog(order) })
  } else if (order.status === 'REFUND_FAILED') {
    actions.push({ key: 'retry-refund', label: t('payment.admin.retryRefund'), icon: 'refresh', onSelect: () => openRefundDialog(order) })
  } else if (order.status === 'REFUND_PENDING') {
    actions.push({
      key: 'query-refund',
      label: t('payment.admin.queryRefundStatus'),
      icon: 'refresh',
      disabled: refundQueryingIds.value.has(order.id),
      onSelect: () => handleQueryRefund(order)
    })
  } else if (order.status === 'COMPLETED' || order.status === 'PARTIALLY_REFUNDED') {
    actions.push({ key: 'refund', label: t('payment.admin.refund'), icon: 'dollar', danger: true, onSelect: () => openRefundDialog(order) })
  }
  return actions
}

async function showOrderDetail(order: PaymentOrder) {
  selectedOrder.value = order
  orderAuditLogs.value = []
  showDetailDialog.value = true
  try {
    const res = await adminPaymentAPI.getOrder(order.id)
    const data = res.data as unknown as Record<string, unknown>
    if (data.order) selectedOrder.value = data.order as PaymentOrder
    orderAuditLogs.value = ((data.auditLogs || data.audit_logs || []) as unknown) as AuditLog[]
  } catch (_err: unknown) { /* keep cached order data */ }
}

async function handleCancelOrder(order: PaymentOrder) {
  actionError.value = ''
  try { await adminPaymentAPI.cancelOrder(order.id); loadOrders() }
  catch (err: unknown) { actionError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')) }
}

async function handleRetryOrder(order: PaymentOrder) {
  actionError.value = ''
  try { await adminPaymentAPI.retryRecharge(order.id); loadOrders() }
  catch (err: unknown) { actionError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')) }
}

function openRefundDialog(order: PaymentOrder) {
  selectedOrder.value = order
  refundRequireForce.value = false
  refundWarning.value = ''
  showRefundDialog.value = true
}

function closeRefundDialog() {
  showRefundDialog.value = false
  refundRequireForce.value = false
  refundWarning.value = ''
}

function isRefundPendingWarning(warning: string | undefined): boolean {
  return /pending|处理中|待/.test(String(warning || '').toLowerCase())
}

async function handleRefund(data: { amount: number; reason: string; deduct_balance: boolean; force: boolean }) {
  if (!selectedOrder.value) return
  refundSubmitting.value = true
  try {
    const res = await adminPaymentAPI.refundOrder(selectedOrder.value.id, { amount: data.amount, reason: data.reason, deduct_balance: data.deduct_balance, force: data.force })
    if (res.data.success) {
      closeRefundDialog()
      loadOrders()
      return
    }
    if (isRefundPendingWarning(res.data.warning)) {
      closeRefundDialog()
      loadOrders()
      return
    }
    if (res.data.require_force) {
      // Backend needs an explicit force confirmation (e.g. the user spent their
      // balance after requesting the refund). Keep the dialog open and surface
      // the force checkbox instead of dropping the admin back to the list.
      refundRequireForce.value = true
      refundWarning.value = t('payment.admin.refundRequireForce')
      return
    }
    // 退款没成功：提示写进退款弹窗，弹窗保持打开（后端的 warning 是英文原因，不上屏）
    refundWarning.value = t('payment.admin.refundGatewayFailed')
  } catch (err: unknown) { refundWarning.value = extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')) }
  finally { refundSubmitting.value = false }
}

async function handleQueryRefund(order: PaymentOrder) {
  refundQueryingIds.value = new Set(refundQueryingIds.value).add(order.id)
  actionError.value = ''
  try {
    const res = await adminPaymentAPI.queryRefund(order.id)
    if (!res.data.success && !isRefundPendingWarning(res.data.warning)) {
      actionError.value = t('payment.admin.refundQueryFailed')
    }
    loadOrders()
  } catch (err: unknown) {
    actionError.value = extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))
  } finally {
    const next = new Set(refundQueryingIds.value)
    next.delete(order.id)
    refundQueryingIds.value = next
  }
}

function formatDateTime(dateStr: string): string { return formatOrderDateTime(dateStr) }

// 订单里出现的「某个用户」只可能是下单用户本人（下单、取消、申请退款都只能本人操作），用下单时记下的邮箱；
// 对不上就只写「用户」，不露内部编号
function orderUserEmail(order: PaymentOrder, userId: string): string {
  return userId === String(order.user_id) ? order.user_email || '' : ''
}

function refundRequesterLabel(order: PaymentOrder): string {
  return orderUserEmail(order, order.refund_requested_by ?? '') || t('payment.admin.operatorUser')
}

// 操作日志的操作人：后端写 user:<编号> / admin / system / 支付服务商标识（易支付、Stripe…），翻成人话
function operatorLabel(order: PaymentOrder, operator: string): string {
  if (operator === 'admin') return t('payment.admin.operatorAdmin')
  if (operator === 'system') return t('payment.admin.operatorSystem')
  const user = /^user:(\d+)$/.exec(operator)
  if (user) {
    const email = orderUserEmail(order, user[1])
    return email ? t('payment.admin.operatorUserWithEmail', { email }) : t('payment.admin.operatorUser')
  }
  const methodKey = `payment.methods.${operator}`
  return te(methodKey) ? t(methodKey) : operator
}

onMounted(() => loadOrders())
</script>
