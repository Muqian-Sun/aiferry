<template>
  <SidebarFrame :sections="sections" home-path="/dashboard">
    <template #version>
      <VersionBadge :version="siteVersion" />
    </template>
  </SidebarFrame>
</template>

<script setup lang="ts">
/**
 * 管理后台侧边栏。管理后台是独立站点，只有管理导航，不再混排「我的账户」个人菜单。
 */
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { FeatureFlags, makeSidebarFlag } from '@/utils/featureFlags'
import SidebarFrame from '@/components/layout/sidebar/SidebarFrame.vue'
import { applyFeatureFlags, type NavItem, type NavSection } from '@/components/layout/sidebar/navTypes'
import {
  BellIcon,
  ChannelIcon,
  ChartIcon,
  CogIcon,
  CreditCardIcon,
  DashboardIcon,
  FolderIcon,
  GiftIcon,
  GlobeIcon,
  OrderIcon,
  PluginIcon,
  PriceTagIcon,
  ServerIcon,
  ShieldIcon,
  SignalIcon,
  TicketIcon,
  UserIcon,
  UsersIcon
} from '@/components/layout/sidebar/navIcons'
import VersionBadge from './VersionBadge.vue'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()

const siteVersion = computed(() => appStore.siteVersion)

const flagChannelMonitor = makeSidebarFlag(FeatureFlags.channelMonitor)
const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)
const flagAffiliate = makeSidebarFlag(FeatureFlags.affiliate)
const flagRiskControl = makeSidebarFlag(FeatureFlags.riskControl)
const flagPluginManagement = makeSidebarFlag(FeatureFlags.pluginManagement)
// Admin-only flags (not in public settings)
const flagOpsMonitoring = () => adminSettingsStore.opsMonitoringEnabled
const flagAdminPayment = () => adminSettingsStore.paymentEnabled

const customMenuItems = computed(() => {
  return adminSettingsStore.customMenuItems
    .filter((item) => item.visibility === 'admin')
    .sort((a, b) => a.sort_order - b.sort_order)
})

const navItems = computed((): NavItem[] => {
  const baseItems: NavItem[] = [
    { path: '/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
    { path: '/ops', label: t('nav.ops'), icon: ChartIcon, featureFlag: flagOpsMonitoring },
    { path: '/users', label: t('nav.users'), icon: UsersIcon, hideInSimpleMode: true },
    { path: '/groups', label: t('nav.groups'), icon: FolderIcon, elementId: 'sidebar-group-manage' },
    {
      path: '/channels',
      label: t('nav.channelManagement'),
      icon: ChannelIcon,
      hideInSimpleMode: true,
      expandOnly: true,
      children: [
        { path: '/channels/pricing', label: t('nav.channelPricing'), icon: PriceTagIcon },
        { path: '/model-catalog', label: t('nav.modelCatalog'), icon: PriceTagIcon },
        { path: '/channels/monitor', label: t('nav.channelMonitor'), icon: SignalIcon, featureFlag: flagChannelMonitor },
      ],
    },
    // 「仅充值」站点连管理端的「订阅管理」入口也一并收起（路由本身不拦截）。
    { path: '/subscriptions', label: t('nav.subscriptions'), icon: CreditCardIcon, hideInSimpleMode: true, featureFlag: flagSubscription },
    { path: '/accounts', label: t('nav.accounts'), icon: GlobeIcon, elementId: 'sidebar-channel-manage' },
    { path: '/plugins', label: t('nav.plugins'), icon: PluginIcon, featureFlag: flagPluginManagement },
    { path: '/announcements', label: t('nav.announcements'), icon: BellIcon },
    { path: '/proxies', label: t('nav.proxies'), icon: ServerIcon },
    {
      path: '/security-audit',
      label: t('nav.securityAudit'),
      icon: ShieldIcon,
      expandOnly: true,
      featureFlag: flagRiskControl,
      children: [
        { path: '/risk-control', label: t('nav.contentModeration'), icon: ShieldIcon },
        { path: '/prompt-audit', label: t('nav.promptAudit'), icon: ShieldIcon },
      ],
    },
    { path: '/redeem', label: t('nav.redeemCodes'), icon: TicketIcon, hideInSimpleMode: true, elementId: 'sidebar-wallet' },
    { path: '/promo-codes', label: t('nav.promoCodes'), icon: GiftIcon, hideInSimpleMode: true },
    {
      path: '/affiliates',
      label: t('nav.affiliateManagement'),
      icon: UsersIcon,
      hideInSimpleMode: true,
      expandOnly: true,
      featureFlag: flagAffiliate,
      children: [
        { path: '/affiliates/invites', label: t('nav.affiliateInviteRecords'), icon: UsersIcon },
        { path: '/affiliates/rebates', label: t('nav.affiliateRebateRecords'), icon: OrderIcon },
        { path: '/affiliates/transfers', label: t('nav.affiliateTransferRecords'), icon: CreditCardIcon },
      ],
    },
    {
      path: '/orders',
      label: t('nav.orderManagement'),
      icon: OrderIcon,
      hideInSimpleMode: true,
      expandOnly: true,
      featureFlag: flagAdminPayment,
      children: [
        { path: '/orders/dashboard', label: t('nav.paymentDashboard'), icon: ChartIcon },
        { path: '/orders', label: t('nav.orderManagement'), icon: OrderIcon },
        { path: '/orders/plans', label: t('nav.paymentPlans'), icon: CreditCardIcon },
      ],
    },
    { path: '/usage', label: t('nav.usage'), icon: ChartIcon },
    { path: '/audit-logs', label: t('nav.auditLogs'), icon: ShieldIcon, hideInSimpleMode: true },
  ]

  const visible = applyFeatureFlags(baseItems)
  const items = authStore.isSimpleMode ? visible.filter((item) => !item.hideInSimpleMode) : visible
  items.push(
    { path: '/settings', label: t('nav.settings'), icon: CogIcon },
    // 管理员自己的账号安全（密码、双因素、Passkey）
    { path: '/profile', label: t('nav.accountSecurity'), icon: UserIcon },
  )
  for (const cm of customMenuItems.value) {
    items.push({ path: `/custom/${cm.id}`, label: cm.label, icon: null, iconSvg: cm.icon_svg })
  }
  return items
})

const sections = computed((): NavSection[] => [{ key: 'admin', items: navItems.value }])

onMounted(() => {
  // 功能开关类菜单项（运维监控、支付管理）依赖管理设置
  void adminSettingsStore.fetch()
})
</script>
