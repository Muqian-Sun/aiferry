<template>
  <SidebarFrame :sections="sections" home-path="/dashboard">
    <template #version>
      <span v-if="siteVersion" class="text-xs text-gray-500 dark:text-dark-400">v{{ siteVersion }}</span>
    </template>
  </SidebarFrame>
</template>

<script setup lang="ts">
/**
 * 用户站侧边栏。管理导航见 components/admin/layout/AdminSidebar.vue。
 */
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { FeatureFlags, makeSidebarFlag } from '@/utils/featureFlags'
import { resolveSiteBillingMode } from '@/utils/siteBillingMode'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import SidebarFrame from './sidebar/SidebarFrame.vue'
import { applyFeatureFlags, type NavItem, type NavSection } from './sidebar/navTypes'
import {
  BatchImageIcon,
  ChannelIcon,
  ChartIcon,
  CreditCardIcon,
  DashboardIcon,
  GiftIcon,
  KeyIcon,
  OrderListIcon,
  RechargeSubscriptionIcon,
  SignalIcon,
  UserIcon,
  UsersIcon
} from './sidebar/navIcons'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

const siteVersion = computed(() => appStore.siteVersion)

// Public-settings flags go through the registry in utils/featureFlags.ts,
// which handles the opt-in vs opt-out fallback when settings haven't loaded yet.
const flagChannelMonitor = makeSidebarFlag(FeatureFlags.channelMonitor)
const flagPayment = makeSidebarFlag(FeatureFlags.payment)
const flagAvailableChannels = makeSidebarFlag(FeatureFlags.availableChannels)
const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)
const flagAffiliate = makeSidebarFlag(FeatureFlags.affiliate)
const flagBatchImageAccess = () => canUseBatchImage.value

// 购买入口文案随站点计费模式切换：仅充值 → 「充值」，仅订阅 → 「订阅」，否则「充值/订阅」。
const purchaseNavLabel = computed(() => {
  switch (resolveSiteBillingMode(appStore.cachedPublicSettings)) {
    case 'recharge_only':
      return t('nav.recharge')
    case 'subscription_only':
      return t('nav.subscribe')
    default:
      return t('nav.buySubscription')
  }
})

const customMenuItems = computed(() => {
  const items = appStore.cachedPublicSettings?.custom_menu_items ?? []
  return items
    .filter((item) => item.visibility === 'user')
    .sort((a, b) => a.sort_order - b.sort_order)
})

// 条目顺序：密钥 → 用量 → 可用渠道 → 渠道状态 → 订阅/支付 → 兑换/资料。
// 可用渠道紧挨渠道状态之上，让用户"先看自己能用什么、再看对应状态"。
const navItems = computed((): NavItem[] => {
  const items: NavItem[] = [
    { path: '/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
    { path: '/keys', label: t('nav.apiKeys'), icon: KeyIcon, dataTour: 'sidebar-my-keys' },
    { path: '/batch-image', label: t('nav.batchImage'), icon: BatchImageIcon, hideInSimpleMode: true, featureFlag: flagBatchImageAccess },
    { path: '/usage', label: t('nav.usage'), icon: ChartIcon, hideInSimpleMode: true },
    { path: '/available-channels', label: t('nav.availableChannels'), icon: ChannelIcon, hideInSimpleMode: true, featureFlag: flagAvailableChannels },
    { path: '/monitor', label: t('nav.channelStatus'), icon: SignalIcon, featureFlag: flagChannelMonitor },
    { path: '/subscriptions', label: t('nav.mySubscriptions'), icon: CreditCardIcon, hideInSimpleMode: true, featureFlag: flagSubscription },
    { path: '/purchase', label: purchaseNavLabel.value, icon: RechargeSubscriptionIcon, hideInSimpleMode: true, featureFlag: flagPayment },
    { path: '/orders', label: t('nav.myOrders'), icon: OrderListIcon, hideInSimpleMode: true, featureFlag: flagPayment },
    { path: '/redeem', label: t('nav.redeem'), icon: GiftIcon, hideInSimpleMode: true },
    { path: '/affiliate', label: t('nav.affiliate'), icon: UsersIcon, hideInSimpleMode: true, featureFlag: flagAffiliate },
    { path: '/profile', label: t('nav.profile'), icon: UserIcon },
    ...customMenuItems.value.map((item): NavItem => ({
      path: `/custom/${item.id}`,
      label: item.label,
      icon: null,
      iconSvg: item.icon_svg,
    })),
  ]
  const visible = applyFeatureFlags(items)
  return authStore.isSimpleMode ? visible.filter((item) => !item.hideInSimpleMode) : visible
})

// Backend mode 下普通用户被挡在所有受保护页面之外，不展示导航
const sections = computed((): NavSection[] =>
  appStore.backendModeEnabled ? [] : [{ key: 'user', items: navItems.value }]
)

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>
