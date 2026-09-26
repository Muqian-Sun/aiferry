import { computed } from 'vue'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { SITE_FEATURES } from '@/utils/siteFeatures'
import type { BillingFlags } from './billingTabs'

/** 账务页签的开关：支付看公开设置（宽容语义：未加载时按默认值），订阅由代码决定。 */
export function useBillingFlags() {
  return computed<BillingFlags>(() => ({
    payment: isFeatureFlagEnabled(FeatureFlags.payment),
    subscription: SITE_FEATURES.subscription
  }))
}

/** 路由 redirect 用的非响应式快照。 */
export function readBillingFlags(): BillingFlags {
  return {
    payment: isFeatureFlagEnabled(FeatureFlags.payment),
    subscription: SITE_FEATURES.subscription
  }
}
