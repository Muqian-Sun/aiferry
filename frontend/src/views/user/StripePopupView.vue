<template>
  <div class="flex min-h-screen items-center justify-center bg-af-sunken p-4">
    <div
      class="w-full max-w-md space-y-4 rounded-lg border border-af-hairline bg-af-sheet p-6"
    >
      <!-- 金额（订单只带内部 ID、没有订单编号，不显示） -->
      <div v-if="amount" class="text-center">
        <p class="text-2xl font-semibold tabular-nums text-af-ink">¥{{ amount }}</p>
      </div>

      <!-- Error -->
      <div v-if="error" class="space-y-3">
        <p class="text-sm text-af-danger" role="alert">{{ error }}</p>
        <button
          class="btn btn-secondary btn-md w-full"
          @click="closeWindow"
        >
          {{ t('common.close') }}
        </button>
      </div>

      <!-- Success -->
      <div v-else-if="success" class="space-y-3 py-4 text-center">
        <p class="text-lg font-semibold text-af-success">{{ t('payment.result.success') }}</p>
        <button
          class="btn btn-secondary btn-md"
          @click="closeWindow"
        >
          {{ t('common.close') }}
        </button>
      </div>

      <!-- Loading / Redirecting -->
      <div v-else class="flex items-center justify-center py-8">
        <LoadingSpinner />
        <span class="ml-3 text-sm text-af-ink-3">{{ hint }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { isMobileDevice } from '@/utils/device'
import { buildApiUrl } from '@/api/client'

interface StripeWithWechatPay {
  confirmWechatPayPayment(clientSecret: string, options: Record<string, unknown>): Promise<{ error?: { message?: string }; paymentIntent?: { status: string } }>
}

const { t } = useI18n()
const route = useRoute()

const orderId = String(route.query.order_id || '')
const method = String(route.query.method || 'alipay')
const amount = String(route.query.amount || '')

const error = ref('')
const success = ref(false)
const hint = ref(t('payment.stripePopup.redirecting'))

let pollTimer: ReturnType<typeof setInterval> | null = null
let initTimeoutTimer: ReturnType<typeof setTimeout> | null = null
let messageHandler: ((event: MessageEvent) => void) | null = null

function closeWindow() { window.close() }

function clearInitTimeout() {
  if (initTimeoutTimer) {
    clearTimeout(initTimeoutTimer)
    initTimeoutTimer = null
  }
}

onMounted(() => {
  messageHandler = (event: MessageEvent) => {
    if (event.origin !== window.location.origin) return
    if (event.data?.type !== 'STRIPE_POPUP_INIT') return
    // INIT 已到达，取消兜底超时，避免长时间的扫码支付被误判为超时。
    clearInitTimeout()
    if (messageHandler) {
      window.removeEventListener('message', messageHandler)
      messageHandler = null
    }
    initStripe(event.data.clientSecret, event.data.publishableKey)
  }
  window.addEventListener('message', messageHandler)

  if (window.opener) {
    window.opener.postMessage({ type: 'STRIPE_POPUP_READY' }, window.location.origin)
  }

  // 仅兜底“父窗口始终未发 STRIPE_POPUP_INIT”的场景。
  initTimeoutTimer = setTimeout(() => {
    if (!error.value && !success.value) {
      error.value = t('payment.stripePopup.timeout')
    }
  }, 15000)
})

onUnmounted(() => {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  clearInitTimeout()
  if (messageHandler) {
    window.removeEventListener('message', messageHandler)
    messageHandler = null
  }
})

async function initStripe(clientSecret: string, publishableKey: string) {
  if (!clientSecret || !publishableKey) {
    error.value = t('payment.stripeMissingParams')
    return
  }
  try {
    const { loadStripe } = await import('@stripe/stripe-js/pure')
    const stripe = await loadStripe(publishableKey)
    if (!stripe) { error.value = t('payment.stripeLoadFailed'); return }

    const returnUrl = window.location.origin + '/payment/result?order_id=' + orderId + '&status=success'

    if (method === 'alipay') {
      // Alipay: redirect this popup to Alipay payment page
      const { error: err } = await stripe.confirmAlipayPayment(clientSecret, { return_url: returnUrl })
      if (err) error.value = err.message || t('payment.result.failed')
    } else if (method === 'wechat_pay') {
      // WeChat: Stripe shows its built-in QR dialog, user scans, promise resolves
      hint.value = t('payment.stripePopup.loadingQr')
      const result = await (stripe as unknown as StripeWithWechatPay).confirmWechatPayPayment(clientSecret, {
        payment_method_options: { wechat_pay: { client: isMobileDevice() ? 'mobile_web' : 'web' } },
      })
      if (result.error) {
        error.value = result.error.message || t('payment.result.failed')
      } else if (result.paymentIntent?.status === 'succeeded') {
        success.value = true
        setTimeout(closeWindow, 2000)
      } else {
        // Payment not completed (user closed QR dialog)
        startPolling()
      }
    }
  } catch (err: unknown) {
    error.value = extractI18nErrorMessage(err, t, 'payment.errors', t('payment.stripeLoadFailed'))
  }
}

function startPolling() {
  let inFlight = false
  pollTimer = setInterval(async () => {
    // 防重入：接口响应慢于轮询间隔时避免并发重叠请求。
    if (inFlight) return
    inFlight = true
    try {
      // access token 存储在 localStorage 的 'auth_token' 键下（见 api/client.ts），
      // 之前误读 'token' 导致轮询请求不带认证、永远 401，支付成功无法被检测到。
      const token = localStorage.getItem('auth_token') || ''
      const res = await fetch(buildApiUrl(`/payment/orders/${orderId}`), {
        headers: token ? { Authorization: 'Bearer ' + token } : {},
        credentials: 'include',
      })
      if (!res.ok) return
      const data = await res.json()
      const status = data?.data?.status
      if (status === 'COMPLETED' || status === 'PAID') {
        if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
        success.value = true
        setTimeout(closeWindow, 2000)
      }
    } catch { /* ignore */ } finally {
      inFlight = false
    }
  }, 3000)
}
</script>
