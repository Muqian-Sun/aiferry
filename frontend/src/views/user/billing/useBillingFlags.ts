import { computed } from 'vue'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import type { BillingFlags } from './billingTabs'

/** 账务相关的三个功能开关（宽容语义：设置未加载时按各自默认值）。 */
export function useBillingFlags() {
  return computed<BillingFlags>(() => ({
    payment: isFeatureFlagEnabled(FeatureFlags.payment),
    subscription: isFeatureFlagEnabled(FeatureFlags.subscription),
    affiliate: isFeatureFlagEnabled(FeatureFlags.affiliate)
  }))
}

/** 路由 redirect 用的非响应式快照。 */
export function readBillingFlags(): BillingFlags {
  return {
    payment: isFeatureFlagEnabled(FeatureFlags.payment),
    subscription: isFeatureFlagEnabled(FeatureFlags.subscription),
    affiliate: isFeatureFlagEnabled(FeatureFlags.affiliate)
  }
}
