import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'
import { buildBillingTabs } from '@/views/user/billing/billingTabs'
import { useBillingFlags } from '@/views/user/billing/useBillingFlags'
import { buildConsoleNav, type NavIcon, type NavTab } from './navItems'

/** 账务子页的侧栏图标（键 = billingTabs 的 key） */
const BILLING_ICONS: Record<string, NavIcon> = {
  recharge: 'creditCard',
  subscriptions: 'badge'
}

/**
 * 控制台导航的响应式数据：左侧栏与窄屏顶栏第二行共用同一份分组。
 * 账务子页与 /billing 的子路由守卫共用 billingTabs 那一份开关判断，不在这里另写。
 */
export function useConsoleNav() {
  const { t } = useI18n()
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const billingFlags = useBillingFlags()
  const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

  const sections = computed(() =>
    buildConsoleNav({
      t,
      simpleMode: authStore.isSimpleMode,
      backendMode: appStore.backendModeEnabled,
      batchImageEnabled: canUseBatchImage.value,
      serviceStatusEnabled: resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.channelMonitor),
      billingItems: buildBillingTabs(billingFlags.value, t).map(
        (tab): NavTab => ({ path: tab.to as string, label: tab.label, icon: BILLING_ICONS[tab.key] })
      ),
      balanceNotifyEnabled: appStore.cachedPublicSettings?.balance_low_notify_enabled === true,
      customItems: appStore.cachedPublicSettings?.custom_menu_items ?? []
    })
  )
  /** 窄屏没有侧栏：所有条目摊平成一行横向滚动 */
  const flatItems = computed(() => sections.value.flatMap((section) => section.items))

  return { sections, flatItems, refreshBatchImageAccess }
}
