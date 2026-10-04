<template>
  <!--
    支付引擎：mode=recharge 渲染充值表单（/billing/recharge），mode=subscription 渲染套餐列表与确认
    （嵌在 /billing/subscriptions 里）。两种模式共用同一套下单 / 支付中 / 微信续接 / 恢复快照逻辑。
    这里只用平铺区块（不用 SheetSection 等子组件，让 shallowMount 的旧用例仍能读到文案）。
  -->
  <div>
    <div :class="['space-y-8', mode === 'recharge' || selectedPlan ? 'max-w-form' : '']">
      <div v-if="loading" class="flex items-center justify-center py-16" role="status" aria-busy="true">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-af-brand border-t-transparent"></div>
      </div>

      <!-- 支付中：充值与订阅共用 -->
      <PaymentStatusPanel
        v-else-if="paymentPhase === 'paying'"
        :order-id="paymentState.orderId"
        :amount="paymentState.amount"
        :pay-amount="paymentState.payAmount"
        :qr-code="paymentState.qrCode"
        :expires-at="paymentState.expiresAt"
        :payment-type="paymentState.paymentType"
        :pay-url="paymentState.payUrl"
        :order-type="paymentState.orderType"
        :currency="paymentState.currency || selectedCurrency"
        :out-trade-no="paymentState.outTradeNo"
        :mobile-alipay-deep-link="paymentState.alipayMobilePrecreateDeepLink"
        @done="onPaymentDone"
        @success="onPaymentSuccess"
        @settled="onPaymentSettled"
      />

      <template v-else>
        <!-- ===== 充值 ===== -->
        <template v-if="mode === 'recharge'">
          <dl class="grid grid-cols-2 divide-x divide-af-hairline pb-4">
            <div class="min-w-0 pr-6">
              <dt class="text-13 text-af-ink-3">{{ t('payment.rechargeAccount') }}</dt>
              <dd class="mt-1 truncate text-base font-semibold text-af-ink">{{ user?.username || user?.email || '' }}</dd>
            </div>
            <div class="min-w-0 pl-6">
              <dt class="text-13 text-af-ink-3">{{ t('payment.currentBalance') }}</dt>
              <dd class="mt-1 text-base font-semibold tabular-nums text-af-ink">{{ formatCurrency(user?.balance ?? 0) }}</dd>
            </div>
          </dl>

          <p v-if="enabledMethods.length === 0" class="py-12 text-center text-sm text-af-ink-3">{{ t('payment.notAvailable') }}</p>

          <template v-else>
            <section>
              <h2 class="mb-4 text-base font-semibold text-af-ink">{{ t('payment.amountLabel') }}</h2>
              <AmountInput
                v-model="amount"
                :amounts="[5, 10, 20, 50, 100, 200, 500, 1000]"
                :min="globalMinAmount"
                :max="globalMaxAmount"
              />
              <p v-if="amountError" class="mt-2 text-xs text-af-warning">{{ amountError }}</p>
            </section>

            <section class="border-t border-af-hairline pt-6">
              <h2 class="mb-4 text-base font-semibold text-af-ink">{{ t('payment.paymentMethod') }}</h2>
              <PaymentMethodSelector
                :methods="methodOptions"
                :selected="selectedMethod"
                @select="selectedMethod = $event"
              />
            </section>

            <section v-if="validAmount > 0" class="border-t border-af-hairline pt-6">
              <!-- 充值金额是美元（= 到账余额）；实付按通道币种换算，人民币通道注明汇率；没配汇率就不能付 -->
              <p v-if="usdRateMissing" class="text-13 text-af-danger" data-testid="usd-rate-missing">{{ t('payment.usdRateMissing') }}</p>
              <template v-else>
                <dl class="space-y-2 text-sm">
                  <div class="flex justify-between">
                    <dt class="text-af-ink-3">{{ t('payment.creditedBalance') }}</dt>
                    <dd class="tabular-nums text-af-ink">{{ formatUsd(validAmount) }}</dd>
                  </div>
                  <div v-if="feeRate > 0" class="flex justify-between">
                    <dt class="text-af-ink-3">{{ t('payment.fee') }} ({{ feeRate }}%)</dt>
                    <dd class="tabular-nums text-af-ink">{{ formatSelectedPaymentAmount(feeAmount) }}</dd>
                  </div>
                  <div class="flex items-baseline justify-between border-t border-af-hairline pt-2">
                    <dt class="font-medium text-af-ink-2">{{ t('payment.actualPay') }}</dt>
                    <dd class="text-xl font-semibold tabular-nums text-af-ink">{{ formatSelectedPaymentAmount(totalAmount) }}</dd>
                  </div>
                </dl>
                <p v-if="selectedCurrency === DEFAULT_PAYMENT_CURRENCY" class="mt-2 text-xs text-af-ink-3">
                  {{ t('payment.usdRateNote', { rate: usdToCnyRate }) }}
                </p>
              </template>
              <FormError class="mt-4" :message="paymentErrorText" />
              <button class="btn btn-primary btn-md mt-6 w-full" :disabled="!canSubmit || submitting" @click="handleSubmitRecharge">
                <span v-if="submitting" class="flex items-center justify-center gap-2">
                  <span class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
                  {{ t('common.processing') }}
                </span>
                <span v-else>{{ t('payment.createOrder') }} {{ formatSelectedPaymentAmount(totalAmount) }}</span>
              </button>
            </section>
          </template>
        </template>

        <!-- ===== 订阅 ===== -->
        <template v-else>
          <!-- 确认购买：替换套餐列表 -->
          <template v-if="selectedPlan">
            <section>
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-base font-semibold text-af-ink">{{ selectedPlan.name }}</h2>
              </div>
              <div class="mt-2 flex items-baseline gap-2">
                <span class="text-2xl font-semibold tabular-nums text-af-ink">{{ formatUsd(selectedPlan.price) }}</span>
                <span v-if="selectedPlan.original_price" class="text-sm tabular-nums text-af-ink-3 line-through">
                  {{ formatUsd(selectedPlan.original_price) }}
                </span>
                <span class="text-13 text-af-ink-3">/ {{ planValiditySuffix }}</span>
              </div>
              <p v-if="selectedPlan.description" class="mt-2 text-13 leading-5 text-af-ink-3">{{ selectedPlan.description }}</p>
              <!-- 续费时套餐的模型与额度已在上方订阅面板里，这里不重复 -->
              <dl v-if="renewPlanId == null" class="mt-4 grid grid-cols-2 gap-x-6 gap-y-2 text-13 sm:grid-cols-3">
                <div class="col-span-2 sm:col-span-3" data-testid="checkout-plan-models">
                  <dt class="text-af-ink-3">{{ t('payment.planCard.models') }}</dt>
                  <dd class="font-medium text-af-ink-2">{{ (selectedPlan.models || []).map(m => m.display_name || m.model_id).join(' / ') || '-' }}</dd>
                </div>
                <div v-if="selectedPlan.daily_limit_usd != null">
                  <dt class="text-af-ink-3">{{ t('payment.planCard.dailyLimit') }}</dt>
                  <dd class="font-medium tabular-nums text-af-ink-2">${{ selectedPlan.daily_limit_usd }}</dd>
                </div>
                <div v-if="selectedPlan.weekly_limit_usd != null">
                  <dt class="text-af-ink-3">{{ t('payment.planCard.weeklyLimit') }}</dt>
                  <dd class="font-medium tabular-nums text-af-ink-2">${{ selectedPlan.weekly_limit_usd }}</dd>
                </div>
                <div v-if="selectedPlan.monthly_limit_usd != null">
                  <dt class="text-af-ink-3">{{ t('payment.planCard.monthlyLimit') }}</dt>
                  <dd class="font-medium tabular-nums text-af-ink-2">${{ selectedPlan.monthly_limit_usd }}</dd>
                </div>
                <div v-if="selectedPlan.daily_limit_usd == null && selectedPlan.weekly_limit_usd == null && selectedPlan.monthly_limit_usd == null">
                  <dt class="text-af-ink-3">{{ t('payment.planCard.quota') }}</dt>
                  <dd class="font-medium text-af-ink-2">{{ t('payment.planCard.unlimited') }}</dd>
                </div>
              </dl>
            </section>

            <section v-if="enabledMethods.length >= 1" class="border-t border-af-hairline pt-6">
              <h2 class="mb-4 text-base font-semibold text-af-ink">{{ t('payment.paymentMethod') }}</h2>
              <PaymentMethodSelector
                :methods="subMethodOptions"
                :selected="selectedMethod"
                @select="selectedMethod = $event"
              />
            </section>

            <section class="border-t border-af-hairline pt-6">
              <!-- 价格是美元（上方）；这里是通道实付：人民币通道按汇率换算并注明，没配汇率就不能付 -->
              <p v-if="usdRateMissing" class="text-13 text-af-danger" data-testid="usd-rate-missing">{{ t('payment.usdRateMissing') }}</p>
              <template v-else-if="subTotalAmount !== null && selectedPlan.price > 0">
                <dl class="space-y-2 text-sm">
                  <div v-if="feeRate > 0" class="flex justify-between">
                    <dt class="text-af-ink-3">{{ t('payment.amountLabel') }}</dt>
                    <dd class="tabular-nums text-af-ink">{{ formatSelectedPaymentAmount(subPaymentAmount ?? 0) }}</dd>
                  </div>
                  <div v-if="feeRate > 0" class="flex justify-between">
                    <dt class="text-af-ink-3">{{ t('payment.fee') }} ({{ feeRate }}%)</dt>
                    <dd class="tabular-nums text-af-ink">{{ formatSelectedPaymentAmount(subFeeAmount) }}</dd>
                  </div>
                  <div :class="['flex items-baseline justify-between', feeRate > 0 ? 'border-t border-af-hairline pt-2' : '']">
                    <dt class="font-medium text-af-ink-2">{{ t('payment.actualPay') }}</dt>
                    <dd class="text-xl font-semibold tabular-nums text-af-ink">{{ formatSelectedPaymentAmount(subTotalAmount) }}</dd>
                  </div>
                </dl>
                <p v-if="selectedCurrency === DEFAULT_PAYMENT_CURRENCY" class="mt-2 text-xs text-af-ink-3">
                  {{ t('payment.usdRateNote', { rate: usdToCnyRate }) }}
                </p>
              </template>
              <FormError class="mt-4" :message="paymentErrorText" />
              <div class="mt-6 flex flex-col gap-2 sm:flex-row-reverse">
                <button class="btn btn-primary btn-md w-full sm:w-auto" :disabled="!canSubmitSubscription || submitting" @click="confirmSubscribe">
                  <span v-if="submitting" class="flex items-center justify-center gap-2">
                    <span class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
                    {{ t('common.processing') }}
                  </span>
                  <span v-else>{{ t('payment.createOrder') }} {{ formatSelectedPaymentAmount(subTotalAmount ?? 0) }}</span>
                </button>
                <button class="btn btn-ghost btn-md w-full sm:w-auto" @click="cancelSelection">{{ t('common.cancel') }}</button>
              </div>
            </section>
          </template>

          <!-- 续费模式下套餐已下架：不列其他套餐，只说明 -->
          <p v-else-if="renewPlanId != null" class="py-6 text-sm text-af-ink-3" data-testid="renew-plan-unavailable">{{ t('payment.renewPlanUnavailable') }}</p>

          <!-- 套餐列表 -->
          <template v-else>
            <p v-if="checkout.plans.length === 0" class="py-12 text-center text-sm text-af-ink-3">{{ t('payment.noPlans') }}</p>
            <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" data-testid="plan-list">
              <SubscriptionPlanCard v-for="plan in checkout.plans" :key="plan.id" :plan="plan" :active-subscriptions="activeSubscriptions" @select="selectPlan" />
            </div>
          </template>
        </template>

        <!-- 管理员配置的支付说明 -->
        <div v-if="(checkout.help_text || checkout.help_image_url) && !selectedPlan" class="border-t border-af-hairline pt-6">
          <img v-if="checkout.help_image_url" :src="checkout.help_image_url" alt=""
            class="mb-3 h-40 max-w-full cursor-pointer rounded-md object-contain"
            @click="previewImage = checkout.help_image_url" />
          <div v-if="checkout.help_text" class="markdown-body w-full overflow-x-auto break-words text-13" v-html="renderedHelpText"></div>
        </div>
      </template>
    </div>

    <!-- 说明图预览 -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="previewImage" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70" @click="previewImage = ''">
          <img :src="previewImage" alt="" class="max-h-[85vh] max-w-[90vw] rounded-md object-contain shadow-2xl" />
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import '@/styles/announcement-markdown.css'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { usePaymentStore } from '@/stores/payment'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { SITE_FEATURES } from '@/utils/siteFeatures'
import { loadCheckoutInfo } from './billing/checkoutPreload'
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'
import { isMobileDevice } from '@/utils/device'
import { formatCurrency } from '@/utils/format'
import type { SubscriptionPlan, CheckoutInfoResponse, CreateOrderResult, OrderType } from '@/types/payment'
import AmountInput from '@/components/payment/AmountInput.vue'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'
import { METHOD_ORDER, getPaymentPopupFeatures } from '@/components/payment/providerConfig'
import {
  PAYMENT_RECOVERY_STORAGE_KEY,
  buildCreateOrderPayload,
  clearPaymentRecoverySnapshot,
  decidePaymentLaunch,
  getVisibleMethods,
  normalizeVisibleMethod,
  readPaymentRecoverySnapshot,
  type PaymentRecoverySnapshot,
  writePaymentRecoverySnapshot,
} from '@/components/payment/paymentFlow'
import SubscriptionPlanCard from '@/components/payment/SubscriptionPlanCard.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import FormError from '@/components/common/FormError.vue'
import { DEFAULT_PAYMENT_CURRENCY, USD_PAYMENT_CURRENCY, formatPaymentAmount, normalizePaymentCurrency } from '@/components/payment/currency'
import { planValiditySuffix as validitySuffixOf } from '@/components/payment/validity'
import type { PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'
import { buildPaymentErrorMessage, describePaymentScenarioError } from './paymentUx'
import { hasWechatResumeQuery, parseWechatResumeRoute, stripWechatResumeQuery } from './paymentWechatResume'

/** 由路由决定：/billing/recharge 传 recharge，SubscriptionsView 嵌入时传 subscription。 */
const props = defineProps<{
  mode: 'recharge' | 'subscription'
  /** 续费模式：只为这个套餐确认购买，不列其他套餐（订阅页面板上的「续费」传入） */
  renewPlanId?: number | null
}>()
const emit = defineEmits<{ cancel: [] }>()

const i18n = useI18n()
const { t } = i18n
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const paymentStore = usePaymentStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)
const activeSubscriptions = computed(() => subscriptionStore.activeSubscriptions)

const loading = ref(true)
const submitting = ref(false)
const errorMessage = ref('')
const errorHintMessage = ref('')
const amount = ref<number | null>(null)
const selectedMethod = ref('')
const selectedPlan = ref<SubscriptionPlan | null>(null)
// 下单失败的报错就近显示在下单按钮上方；改金额 / 支付方式 / 套餐时清掉
const paymentErrorText = computed(() =>
  errorMessage.value ? buildPaymentErrorMessage(errorMessage.value, errorHintMessage.value) : ''
)
watch([amount, selectedMethod, selectedPlan], () => {
  errorMessage.value = ''
  errorHintMessage.value = ''
})
const previewImage = ref('')

const paymentPhase = ref<'select' | 'paying'>('select')

interface CreateOrderOptions {
  openid?: string
  wechatResumeToken?: string
  paymentType?: string
  isResume?: boolean
  mobileQrFallbackAttempted?: boolean
}

interface WeixinJSBridgeLike {
  invoke(
    action: string,
    payload: Record<string, unknown>,
    callback: (result: Record<string, unknown>) => void,
  ): void
}

function emptyPaymentState(): PaymentRecoverySnapshot {
  return {
    orderId: 0,
    amount: 0,
    qrCode: '',
    expiresAt: '',
    paymentType: '',
    payUrl: '',
    outTradeNo: '',
    clientSecret: '',
    intentId: '',
    currency: '',
    countryCode: '',
    paymentEnv: '',
    payAmount: 0,
    orderType: '',
    paymentMode: '',
    resumeToken: '',
    alipayMobilePrecreateDeepLink: false,
    createdAt: 0,
  }
}

function getWeixinJSBridge(): WeixinJSBridgeLike | undefined {
  return (window as Window & { WeixinJSBridge?: WeixinJSBridgeLike }).WeixinJSBridge
}

function waitForWeixinJSBridge(timeoutMs = 4000): Promise<WeixinJSBridgeLike | null> {
  const existing = getWeixinJSBridge()
  if (existing) return Promise.resolve(existing)

  return new Promise((resolve) => {
    let settled = false
    const finish = (bridge: WeixinJSBridgeLike | null) => {
      if (settled) return
      settled = true
      document.removeEventListener('WeixinJSBridgeReady', handleReady)
      document.removeEventListener('onWeixinJSBridgeReady', handleReady)
      window.clearTimeout(timer)
      resolve(bridge)
    }
    const handleReady = () => finish(getWeixinJSBridge() ?? null)
    const timer = window.setTimeout(() => finish(getWeixinJSBridge() ?? null), timeoutMs)
    document.addEventListener('WeixinJSBridgeReady', handleReady, false)
    document.addEventListener('onWeixinJSBridgeReady', handleReady, false)
  })
}

async function invokeWechatJsapiPayment(payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  const bridge = await waitForWeixinJSBridge()
  if (!bridge) {
    throw new Error('WECHAT_JSAPI_UNAVAILABLE')
  }
  return new Promise((resolve) => {
    bridge.invoke('getBrandWCPayRequest', payload, (result) => resolve(result || {}))
  })
}

const paymentState = ref<PaymentRecoverySnapshot>(emptyPaymentState())

function persistRecoverySnapshot(snapshot: PaymentRecoverySnapshot) {
  if (typeof window === 'undefined' || !snapshot.orderId) return
  writePaymentRecoverySnapshot(window.localStorage, snapshot, PAYMENT_RECOVERY_STORAGE_KEY)
}

function removeRecoverySnapshot() {
  if (typeof window === 'undefined') return
  clearPaymentRecoverySnapshot(window.localStorage, PAYMENT_RECOVERY_STORAGE_KEY)
}

function resetPayment() {
  paymentPhase.value = 'select'
  paymentState.value = emptyPaymentState()
  removeRecoverySnapshot()
}

async function redirectToPaymentResult(state: PaymentRecoverySnapshot): Promise<void> {
  const query: Record<string, string | undefined> = {}
  if (state.orderId > 0) {
    query.order_id = String(state.orderId)
  }
  if (state.outTradeNo) {
    query.out_trade_no = state.outTradeNo
  }
  if (state.resumeToken) {
    query.resume_token = state.resumeToken
  }
  await router.push({
    path: '/payment/result',
    query,
  })
}

function buildWechatOAuthAuthorizeUrl(
  authorizeUrl: string,
  context: { paymentType: string; orderType: OrderType; planId?: number; orderAmount: number },
): string {
  const normalizedUrl = authorizeUrl.trim()
  if (!normalizedUrl || typeof window === 'undefined') {
    return normalizedUrl
  }

  try {
    const targetUrl = new URL(normalizedUrl, window.location.origin)
    const redirectPath = targetUrl.searchParams.get('redirect') || '/purchase'
    const redirectUrl = new URL(redirectPath, window.location.origin)
    const paymentType = normalizeVisibleMethod(context.paymentType) || context.paymentType.trim() || 'wxpay'

    redirectUrl.searchParams.set('payment_type', paymentType)
    redirectUrl.searchParams.set('order_type', context.orderType)

    if (context.planId) {
      redirectUrl.searchParams.set('plan_id', String(context.planId))
    } else {
      redirectUrl.searchParams.delete('plan_id')
    }

    if (context.orderAmount > 0) {
      redirectUrl.searchParams.set('amount', String(context.orderAmount))
    } else {
      redirectUrl.searchParams.delete('amount')
    }

    targetUrl.searchParams.set('redirect', `${redirectUrl.pathname}${redirectUrl.search}`)
    return targetUrl.toString()
  } catch {
    return normalizedUrl
  }
}

function onPaymentDone() {
  const wasSubscription = paymentState.value.orderType === 'subscription'
  resetPayment()
  selectedPlan.value = null
  if (wasSubscription) {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
  }
}

async function onPaymentSuccess() {
  const completedPayment = { ...paymentState.value }
  removeRecoverySnapshot()
  authStore.refreshUser()
  if (paymentState.value.orderType === 'subscription') {
    subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
  }
  await redirectToPaymentResult(completedPayment)
}

function onPaymentSettled() {
  removeRecoverySnapshot()
}

// All checkout data from single API call
const checkout = ref<CheckoutInfoResponse>({
  methods: {}, global_min: 0, global_max: 0,
  plans: [], usd_to_cny_rate: 0, recharge_fee_rate: 0, help_text: '', help_image_url: '', stripe_publishable_key: '',
})

const renderedHelpText = computed(() => DOMPurify.sanitize(
  marked.parse(checkout.value.help_text || '', { async: false, gfm: true, breaks: false }),
))

const visibleMethods = computed(() => getVisibleMethods(checkout.value.methods))
const enabledMethods = computed(() => Object.keys(visibleMethods.value))
const validAmount = computed(() => amount.value ?? 0)
// 美元汇率（1 USD = X CNY）。站内金额（充值到账额、套餐价格）一律是美元；0 = 未配置，人民币通道不能下单。
// 与后端 convertUSDToGatewayAmount 严格镜像。
const usdToCnyRate = computed(() => {
  const rate = checkout.value.usd_to_cny_rate
  return Number.isFinite(rate) && rate > 0 ? rate : 0
})

// Check if a gateway amount fits a method's [min, max]. 0 = no limit.
function amountFitsMethod(amt: number, methodType: string): boolean {
  if (amt <= 0) return true
  const ml = visibleMethods.value[methodType]
  if (!ml) return false
  if (ml.single_min > 0 && amt < ml.single_min) return false
  if (ml.single_max > 0 && amt > ml.single_max) return false
  return true
}

// 支付方式的单笔限额是通道币种；金额输入框是美元，按汇率折回美元（启用的方式币种一致，后端强制）
function gatewayLimitToUsd(value: number, currency: string, round: 'up' | 'down'): number {
  if (value <= 0) return 0
  if (currency === USD_PAYMENT_CURRENCY) return value
  if (currency !== DEFAULT_PAYMENT_CURRENCY || usdToCnyRate.value <= 0) return 0
  const usd = value / usdToCnyRate.value
  return round === 'up' ? Math.ceil(usd * 100) / 100 : Math.floor(usd * 100) / 100
}

// Visible methods decide the amount range shown to users (in USD).
const globalMinAmount = computed(() => {
  const limits = Object.values(visibleMethods.value)
  if (limits.length === 0) return 0
  if (limits.some(limit => limit.single_min <= 0)) return 0
  return Math.min(...limits.map(limit => gatewayLimitToUsd(limit.single_min, normalizePaymentCurrency(limit.currency), 'up')))
})
const globalMaxAmount = computed(() => {
  const limits = Object.values(visibleMethods.value)
  if (limits.length === 0) return 0
  if (limits.some(limit => limit.single_max <= 0)) return 0
  return Math.max(...limits.map(limit => gatewayLimitToUsd(limit.single_max, normalizePaymentCurrency(limit.currency), 'down')))
})

// Selected method's limits (for validation and error messages)
const selectedLimit = computed(() => visibleMethods.value[selectedMethod.value])
const selectedCurrency = computed(() => normalizePaymentCurrency(selectedLimit.value?.currency))
const localeCode = computed(() => {
  const raw = i18n.locale as unknown
  if (typeof raw === 'string') return raw
  if (raw && typeof raw === 'object' && 'value' in raw) {
    return String((raw as { value?: string }).value || '')
  }
  return undefined
})

function currencyFractionDigits(currency: string): number {
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency,
    }).resolvedOptions().maximumFractionDigits ?? 2
  } catch {
    return 2
  }
}

function roundPaymentAmount(value: number, currency: string): number {
  if (!Number.isFinite(value)) return 0
  const factor = 10 ** currencyFractionDigits(currency)
  return Math.round(value * factor) / factor
}

function ceilPaymentAmount(value: number, currency: string): number {
  if (!Number.isFinite(value)) return 0
  const factor = 10 ** currencyFractionDigits(currency)
  return Math.ceil(value * factor) / factor
}

const feeRate = computed(() => checkout.value?.recharge_fee_rate ?? 0)

/** 美元金额换算成通道币种金额（不含手续费）：USD 原价；CNY × 汇率；汇率未配置或其他币种返回 null（后端会拒单） */
function gatewayAmountForCurrency(valueUsd: number, currency: string): number | null {
  if (currency === USD_PAYMENT_CURRENCY) return roundPaymentAmount(valueUsd, currency)
  if (currency === DEFAULT_PAYMENT_CURRENCY && usdToCnyRate.value > 0) return roundPaymentAmount(valueUsd * usdToCnyRate.value, currency)
  return null
}

/** 通道实付 = 换算后金额 + 手续费（手续费按换算后金额向上取整，与后端 CalculatePayAmountForCurrency 一致） */
function gatewayTotalForCurrency(valueUsd: number, currency: string): number | null {
  const base = gatewayAmountForCurrency(valueUsd, currency)
  if (base === null) return null
  if (feeRate.value <= 0 || base <= 0) return base
  return roundPaymentAmount(base + ceilPaymentAmount((base * feeRate.value) / 100, currency), currency)
}

/** 某支付方式能否收这笔美元金额：能换算出实付，且实付在该方式的单笔限额内 */
function usdAmountFitsMethod(valueUsd: number, methodType: string): boolean {
  if (valueUsd <= 0) return true
  const total = gatewayTotalForCurrency(valueUsd, normalizePaymentCurrency(visibleMethods.value[methodType]?.currency))
  return total !== null && amountFitsMethod(total, methodType)
}

/** 选中的是人民币通道但没配汇率：显示提示、不能提交 */
const usdRateMissing = computed(() => gatewayAmountForCurrency(1, selectedCurrency.value) === null)

function formatSelectedPaymentAmount(value: number): string {
  return formatPaymentAmount(value, selectedCurrency.value, localeCode.value)
}

function formatUsd(value: number): string {
  return formatPaymentAmount(value, USD_PAYMENT_CURRENCY, localeCode.value)
}

const methodOptions = computed<PaymentMethodOption[]>(() =>
  enabledMethods.value.map((type) => {
    const ml = visibleMethods.value[type]
    return {
      type,
      display_name: ml?.display_name,
      fee_rate: ml?.fee_rate ?? 0,
      available: ml?.available !== false && usdAmountFitsMethod(validAmount.value, type),
    }
  })
)

// 充值：输入的美元金额 = 到账余额；实付按选中通道换算
const rechargeBaseAmount = computed(() => gatewayAmountForCurrency(validAmount.value, selectedCurrency.value))
const feeAmount = computed(() => {
  const base = rechargeBaseAmount.value
  if (base === null || feeRate.value <= 0 || base <= 0) return 0
  return ceilPaymentAmount((base * feeRate.value) / 100, selectedCurrency.value)
})
const totalAmount = computed(() => gatewayTotalForCurrency(validAmount.value, selectedCurrency.value) ?? 0)

const amountError = computed(() => {
  if (validAmount.value <= 0 || usdRateMissing.value) return ''
  // No method can handle this amount
  if (!enabledMethods.value.some((m) => usdAmountFitsMethod(validAmount.value, m))) {
    return t('payment.amountNoMethod')
  }
  // Selected method can't handle this amount (but others can); limits are in the channel currency
  const ml = selectedLimit.value
  if (ml) {
    const total = totalAmount.value
    if (ml.single_min > 0 && total < ml.single_min) return t('payment.amountTooLow', { min: formatSelectedPaymentAmount(ml.single_min) })
    if (ml.single_max > 0 && total > ml.single_max) return t('payment.amountTooHigh', { max: formatSelectedPaymentAmount(ml.single_max) })
  }
  return ''
})

const canSubmit = computed(() =>
  validAmount.value > 0
    && usdAmountFitsMethod(validAmount.value, selectedMethod.value)
    && selectedLimit.value?.available !== false
)

// 订阅：价格是美元；实付按选中通道换算
const subPaymentAmount = computed(() => gatewayAmountForCurrency(selectedPlan.value?.price ?? 0, selectedCurrency.value))

const subFeeAmount = computed(() => {
  const base = subPaymentAmount.value
  if (base === null || feeRate.value <= 0 || base <= 0) return 0
  return ceilPaymentAmount((base * feeRate.value) / 100, selectedCurrency.value)
})

const subTotalAmount = computed(() => gatewayTotalForCurrency(selectedPlan.value?.price ?? 0, selectedCurrency.value))

// Subscription-specific: method options based on gateway pay amount
const subMethodOptions = computed<PaymentMethodOption[]>(() => {
  const price = selectedPlan.value?.price ?? 0
  return enabledMethods.value.map((type) => {
    const ml = visibleMethods.value[type]
    return {
      type,
      display_name: ml?.display_name,
      fee_rate: ml?.fee_rate ?? 0,
      available: ml?.available !== false && usdAmountFitsMethod(price, type),
    }
  })
})

const canSubmitSubscription = computed(() =>
  selectedPlan.value !== null
    && subTotalAmount.value !== null
    && usdAmountFitsMethod(selectedPlan.value.price, selectedMethod.value)
    && selectedLimit.value?.available !== false
)

// Auto-switch to first available method when current selection can't handle the amount
watch(() => [validAmount.value, selectedMethod.value] as const, ([amt, method]) => {
  if (amt <= 0 || usdAmountFitsMethod(amt, method)) return
  const available = enabledMethods.value.find((m) => usdAmountFitsMethod(amt, m))
  if (available) selectedMethod.value = available
})

const planValiditySuffix = computed(() => {
  if (!selectedPlan.value) return ''
  return validitySuffixOf(selectedPlan.value, t)
})

function selectPlan(plan: SubscriptionPlan) {
  selectedPlan.value = plan
  errorMessage.value = ''
}

// 续费模式：套餐列表加载后直接进这个套餐的确认购买
watch(() => [props.renewPlanId, checkout.value.plans] as const, ([planId, plans]) => {
  if (planId == null) return
  const plan = plans.find((p) => p.id === planId)
  if (plan) selectPlan(plan)
}, { immediate: true })

function cancelSelection() {
  selectedPlan.value = null
  if (props.renewPlanId != null) emit('cancel')
}

async function handleSubmitRecharge() {
  if (!canSubmit.value || submitting.value) return
  await createOrder(validAmount.value, 'balance')
}

async function confirmSubscribe() {
  if (!selectedPlan.value || submitting.value) return
  await createOrder(selectedPlan.value.price, 'subscription', selectedPlan.value.id)
}

async function createOrder(orderAmount: number, orderType: OrderType, planId?: number, options: CreateOrderOptions = {}) {
  submitting.value = true
  errorMessage.value = ''
  errorHintMessage.value = ''
  const requestType = normalizeVisibleMethod(options.paymentType || selectedMethod.value) || options.paymentType || selectedMethod.value
  try {
    const payload = buildCreateOrderPayload({
      amount: orderAmount,
      paymentType: requestType,
      orderType,
      planId,
      origin: typeof window !== 'undefined' ? window.location.origin : '',
      isMobile: isMobileDevice(),
      isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
      forceQRCode: !!(checkout.value.alipay_force_qrcode && normalizeVisibleMethod(requestType) === 'alipay'),
      mobilePrecreateDeepLink: checkout.value.alipay_mobile_precreate_deep_link === true,
    })
    if (options.openid) {
      payload.openid = options.openid
    }
    if (options.wechatResumeToken) {
      payload.wechat_resume_token = options.wechatResumeToken
    }

    const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
    const openWindow = (url: string) => {
      const win = window.open(url, 'paymentPopup', getPaymentPopupFeatures())
      if (!win || win.closed) {
        window.location.href = url
      }
    }
    const visibleMethod = normalizeVisibleMethod(requestType) || requestType
    // When user clicks the dedicated Stripe button, leave method blank so the
    // landing page renders Stripe's full Payment Element (card/link/alipay/wxpay).
    const stripeMethod = visibleMethod === 'stripe'
      ? ''
      : visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
    const stripeRouteUrl = result.client_secret && visibleMethod !== 'airwallex'
      ? router.resolve({
        path: '/payment/stripe',
        query: {
          order_id: String(result.order_id),
          client_secret: result.client_secret,
          method: stripeMethod || undefined,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const airwallexRouteUrl = result.client_secret && result.intent_id
      ? router.resolve({
        path: '/payment/airwallex',
        query: {
          order_id: String(result.order_id),
          out_trade_no: result.out_trade_no || undefined,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const decision = decidePaymentLaunch(result, {
      visibleMethod,
      orderType,
      isMobile: isMobileDevice(),
      isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
      forceQRCode: !!(checkout.value.alipay_force_qrcode && visibleMethod === 'alipay'),
      mobilePrecreateDeepLink: checkout.value.alipay_mobile_precreate_deep_link === true,
      stripePopupUrl: stripeRouteUrl,
      stripeRouteUrl,
      airwallexRouteUrl,
    })

    if (decision.kind === 'wechat_oauth' && decision.oauth?.authorize_url) {
      window.location.href = buildWechatOAuthAuthorizeUrl(decision.oauth.authorize_url, {
        paymentType: visibleMethod,
        orderType,
        planId,
        orderAmount,
      })
      return
    }

    if (decision.kind === 'unhandled') {
      applyScenarioError({ reason: 'UNHANDLED_PAYMENT_SCENARIO' }, visibleMethod)
      return
    }

    paymentState.value = decision.paymentState
    paymentPhase.value = 'paying'
    persistRecoverySnapshot(decision.recovery)

    if (decision.kind === 'stripe_popup') {
      openWindow(decision.paymentState.payUrl)
      return
    }
    if (decision.kind === 'stripe_route') {
      window.location.href = decision.paymentState.payUrl
      return
    }
    if (decision.kind === 'airwallex_route') {
      window.location.href = decision.paymentState.payUrl
      return
    }
    if (decision.kind === 'wechat_jsapi' && decision.jsapi) {
      try {
        const jsapiResult = await invokeWechatJsapiPayment(decision.jsapi as Record<string, unknown>)
        const errMsg = String(jsapiResult.err_msg || '').toLowerCase()
        if (errMsg.includes('cancel')) {
          resetPayment()
        } else if (errMsg && !errMsg.includes('ok')) {
          resetPayment()
          const fallbackApplied = await attemptMobileQrFallback(
            { reason: 'WECHAT_JSAPI_FAILED', message: errMsg },
            {
              orderAmount,
              orderType,
              planId,
              paymentType: visibleMethod,
              attempted: options.mobileQrFallbackAttempted === true,
            },
          )
          if (!fallbackApplied) {
            applyScenarioError({ reason: 'WECHAT_JSAPI_FAILED', message: errMsg }, visibleMethod)
          }
        } else {
          const resultState = { ...decision.paymentState }
          resetPayment()
          await redirectToPaymentResult(resultState)
        }
      } catch (err: unknown) {
        resetPayment()
        const fallbackApplied = await attemptMobileQrFallback(err, {
          orderAmount,
          orderType,
          planId,
          paymentType: visibleMethod,
          attempted: options.mobileQrFallbackAttempted === true,
        })
        if (!fallbackApplied) {
          throw err
        }
      }
      return
    }
    if (decision.kind === 'redirect_waiting' && decision.paymentState.payUrl) {
      if (isMobileDevice()) {
        window.location.href = decision.paymentState.payUrl
        return
      }
      openWindow(decision.paymentState.payUrl)
    }
  } catch (err: unknown) {
    const apiErr = err as Record<string, unknown>
    if (apiErr.reason === 'TOO_MANY_PENDING') {
      const metadata = apiErr.metadata as Record<string, unknown> | undefined
      errorMessage.value = t('payment.errors.tooManyPending', { max: metadata?.max || '' })
      errorHintMessage.value = ''
    } else if (apiErr.reason === 'CANCEL_RATE_LIMITED') {
      errorMessage.value = t('payment.errors.cancelRateLimited')
      errorHintMessage.value = ''
    } else if (await attemptMobileQrFallback(err, {
      orderAmount,
      orderType,
      planId,
      paymentType: requestType,
      attempted: options.mobileQrFallbackAttempted === true,
    })) {
      return
    } else {
      const handled = applyScenarioError(
        err,
        normalizeVisibleMethod(options.paymentType || selectedMethod.value) || selectedMethod.value,
      )
      if (!handled) {
        errorMessage.value = extractI18nErrorMessage(err, t, 'payment.errors', extractApiErrorMessage(err, t('payment.result.failed')))
        errorHintMessage.value = ''
      }
      if (handled) {
        return
      }
    }
    console.error(buildPaymentErrorMessage(errorMessage.value, errorHintMessage.value), err)
  } finally {
    submitting.value = false
  }
}

interface MobileQrFallbackContext {
  orderAmount: number
  orderType: OrderType
  planId?: number
  paymentType: string
  attempted: boolean
}

function shouldFallbackToDesktopQr(err: unknown, paymentMethod: string, attempted: boolean): boolean {
  if (attempted || !isMobileDevice()) {
    return false
  }

  const normalizedMethod = normalizeVisibleMethod(paymentMethod) || paymentMethod
  const reason = typeof err === 'object' && err && 'reason' in err && typeof err.reason === 'string'
    ? err.reason
    : ''
  const message = err instanceof Error
    ? err.message
    : (typeof err === 'object' && err && 'message' in err && typeof err.message === 'string'
      ? err.message
      : '')
  const normalizedMessage = message.toLowerCase()

  if (normalizedMethod === 'wxpay') {
    return reason === 'WECHAT_H5_NOT_AUTHORIZED'
      || reason === 'WECHAT_PAYMENT_MP_NOT_CONFIGURED'
      || reason === 'WECHAT_JSAPI_FAILED'
      || reason === 'PAYMENT_GATEWAY_ERROR'
      || reason === 'UNHANDLED_PAYMENT_SCENARIO'
      || normalizedMessage.includes('weixinjsbridge is unavailable')
      || normalizedMessage.includes('wechat_jsapi_unavailable')
  }

  if (normalizedMethod === 'alipay') {
    return reason === 'PAYMENT_GATEWAY_ERROR' || reason === 'UNHANDLED_PAYMENT_SCENARIO'
  }

  return false
}

async function attemptMobileQrFallback(err: unknown, context: MobileQrFallbackContext): Promise<boolean> {
  if (!shouldFallbackToDesktopQr(err, context.paymentType, context.attempted)) {
    return false
  }

  try {
    const visibleMethod = normalizeVisibleMethod(context.paymentType) || context.paymentType
    const payload = buildCreateOrderPayload({
      amount: context.orderAmount,
      paymentType: visibleMethod,
      orderType: context.orderType,
      planId: context.planId,
      origin: typeof window !== 'undefined' ? window.location.origin : '',
      isMobile: false,
      isWechatBrowser: false,
    })
    const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
    const stripeMethod = visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
    const stripeRouteUrl = result.client_secret
      ? router.resolve({
        path: '/payment/stripe',
        query: {
          order_id: String(result.order_id),
          client_secret: result.client_secret,
          method: stripeMethod,
          resume_token: result.resume_token || undefined,
        },
      }).href
      : ''
    const decision = decidePaymentLaunch(result, {
      visibleMethod,
      orderType: context.orderType,
      isMobile: false,
      isWechatBrowser: false,
      stripePopupUrl: stripeRouteUrl,
      stripeRouteUrl,
    })

    if (decision.kind !== 'qr_waiting' || !decision.paymentState.qrCode) {
      return false
    }

    errorMessage.value = ''
    errorHintMessage.value = ''
    paymentState.value = decision.paymentState
    paymentPhase.value = 'paying'
    persistRecoverySnapshot(decision.recovery)
    return true
  } catch {
    return false
  }
}

function applyScenarioError(err: unknown, paymentMethod: string): boolean {
  const descriptor = describePaymentScenarioError(err, {
    paymentMethod,
    isMobile: isMobileDevice(),
    isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
  })
  if (!descriptor) {
    errorMessage.value = ''
    errorHintMessage.value = ''
    return false
  }
  errorMessage.value = t(descriptor.messageKey)
  errorHintMessage.value = descriptor.hintKey ? t(descriptor.hintKey) : ''
  console.error(buildPaymentErrorMessage(errorMessage.value, errorHintMessage.value))
  return true
}

async function resumeWechatPaymentFromQuery() {
  const resume = parseWechatResumeRoute(route.query, checkout.value.plans, validAmount.value)
  if (!resume) {
    return
  }

  selectedMethod.value = resume.paymentType
  if (resume.orderType === 'balance' && resume.orderAmount > 0) {
    amount.value = resume.orderAmount
  }
  if (resume.orderType === 'subscription' && resume.planId) {
    selectedPlan.value = checkout.value.plans.find(plan => plan.id === resume.planId) ?? null
  }

  await router.replace({ path: route.path, query: stripWechatResumeQuery(route.query) })

  if (resume.wechatResumeToken) {
    await createOrder(0, resume.orderType, resume.planId, {
      wechatResumeToken: resume.wechatResumeToken,
      paymentType: resume.paymentType,
      isResume: true,
    })
    return
  }

  if (resume.orderAmount > 0 && resume.openid) {
    await createOrder(resume.orderAmount, resume.orderType, resume.planId, {
      openid: resume.openid,
      paymentType: resume.paymentType,
      isResume: true,
    })
  }
}

onMounted(async () => {
  try {
    const res = await loadCheckoutInfo()
    checkout.value = res.data
    if (enabledMethods.value.length) {
      const order: readonly string[] = METHOD_ORDER
      const sorted = [...enabledMethods.value].sort((a, b) => {
        const ai = order.indexOf(a)
        const bi = order.indexOf(b)
        return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
      })
      selectedMethod.value = sorted[0]
    }
    if (typeof window !== 'undefined') {
      if (hasWechatResumeQuery(route.query)) {
        removeRecoverySnapshot()
      }
      const routeResumeToken = typeof route.query.resume_token === 'string'
        ? route.query.resume_token
        : typeof route.query.wechat_resume_token === 'string'
          ? route.query.wechat_resume_token
          : undefined
      const restored = readPaymentRecoverySnapshot(
        window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY),
        { resumeToken: routeResumeToken },
      )
      if (restored) {
        paymentState.value = restored
        paymentPhase.value = 'paying'
        const restoredMethod = normalizeVisibleMethod(restored.paymentType)
          || (visibleMethods.value[restored.paymentType] ? restored.paymentType : '')
        if (restoredMethod) {
          selectedMethod.value = restoredMethod
        }
      } else {
        removeRecoverySnapshot()
      }
    }
    await resumeWechatPaymentFromQuery()
  } catch (err: unknown) { console.error(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')), err) }
  finally { loading.value = false }
  // Fetch active subscriptions (uses cache, non-blocking); skipped while subscriptions are hidden
  if (SITE_FEATURES.subscription) {
    subscriptionStore.fetchActiveSubscriptions().catch(() => {})
  }
})
</script>
