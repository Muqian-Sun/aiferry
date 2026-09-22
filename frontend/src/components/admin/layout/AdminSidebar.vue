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
  ChartIcon,
  CogIcon,
  CreditCardIcon,
  DashboardIcon,
  GlobeIcon,
  OrderIcon,
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
const flagRiskControl = makeSidebarFlag(FeatureFlags.riskControl)
// Admin-only flags (not in public settings)
const flagOpsMonitoring = () => adminSettingsStore.opsMonitoringEnabled
const flagAdminPayment = () => adminSettingsStore.paymentEnabled

// 分组侧栏：概览 / 渠道 / 用户与计费 / 运营 / 系统。路由不变，只加组标题（muqian 定 B2 档）。
// 「仅充值」站点连管理端的「订阅」入口也一并收起（路由本身不拦截）。套餐属于订阅，不挂支付门。
function visibleItems(items: NavItem[]): NavItem[] {
  const visible = applyFeatureFlags(items)
  return authStore.isSimpleMode ? visible.filter((item) => !item.hideInSimpleMode) : visible
}

const sections = computed((): NavSection[] => {
  const groups: NavSection[] = [
    {
      key: 'overview',
      title: t('nav.sections.overview'),
      items: [
        { path: '/dashboard', label: t('nav.dashboard'), icon: DashboardIcon },
        { path: '/ops', label: t('nav.ops'), icon: ChartIcon, featureFlag: flagOpsMonitoring },
      ],
    },
    {
      key: 'channels',
      title: t('nav.sections.channels'),
      items: [
        // 渠道 = 资源（成品号 / 第三方 key）；模型目录决定上架与标价；分组已不是导航概念
        { path: '/accounts', label: t('nav.channels'), icon: GlobeIcon },
        { path: '/model-catalog', label: t('nav.modelCatalog'), icon: PriceTagIcon },
        { path: '/channels/monitor', label: t('nav.channelMonitor'), icon: SignalIcon, featureFlag: flagChannelMonitor },
        { path: '/proxies', label: t('nav.proxies'), icon: ServerIcon },
      ],
    },
    {
      key: 'billing',
      title: t('nav.sections.billing'),
      items: [
        { path: '/users', label: t('nav.users'), icon: UsersIcon, hideInSimpleMode: true },
        {
          path: '/subscriptions',
          label: t('nav.subscriptions'),
          icon: CreditCardIcon,
          hideInSimpleMode: true,
          featureFlag: flagSubscription,
          expandOnly: true,
          children: [
            { path: '/subscriptions', label: t('nav.subscriptionRecords'), icon: CreditCardIcon },
            { path: '/orders/plans', label: t('nav.paymentPlans'), icon: PriceTagIcon },
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
          ],
        },
        { path: '/redeem', label: t('nav.redeemCodes'), icon: TicketIcon, hideInSimpleMode: true },
      ],
    },
    {
      key: 'operations',
      title: t('nav.sections.operations'),
      items: [
        { path: '/usage', label: t('nav.usage'), icon: ChartIcon },
        { path: '/announcements', label: t('nav.announcements'), icon: BellIcon },
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
      ],
    },
    {
      key: 'system',
      title: t('nav.sections.system'),
      items: [
        { path: '/settings', label: t('nav.settings'), icon: CogIcon },
        { path: '/audit-logs', label: t('nav.auditLogs'), icon: ShieldIcon, hideInSimpleMode: true },
        // 管理员自己的账号安全（密码、双因素、Passkey）
        { path: '/profile', label: t('nav.accountSecurity'), icon: UserIcon },
      ],
    },
  ]
  return groups.map((group) => ({ ...group, items: visibleItems(group.items) })).filter((group) => group.items.length > 0)
})

onMounted(() => {
  // 功能开关类菜单项（运维监控、支付管理）依赖管理设置
  void adminSettingsStore.fetch()
})
</script>
