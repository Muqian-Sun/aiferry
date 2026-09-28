/**
 * 充值页首屏要的支付配置（checkout-info）：进入页面前由路由预加载发出，页面挂载时接手（router/routePreload.ts）。
 */
import { paymentAPI } from '@/api/payment'
import { adoptPreloaded, preloadKey, type Prefetch } from '@/router/routePreload'

const CHECKOUT_INFO_KEY = preloadKey('payment/checkout-info')

export function preloadCheckoutInfo(prefetch: Prefetch): void {
  prefetch(CHECKOUT_INFO_KEY, () => paymentAPI.getCheckoutInfo())
}

/** 有预加载好的就直接用，否则现取（订阅页里嵌的支付、预加载失败时） */
export function loadCheckoutInfo() {
  return adoptPreloaded(CHECKOUT_INFO_KEY, () => paymentAPI.getCheckoutInfo())
}
