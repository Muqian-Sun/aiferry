<template>
  <SidebarFrame :sections="sections" home-path="/dashboard">
    <template #version>
      <span v-if="siteVersion" class="text-xs tabular-nums text-af-ink-3" data-testid="admin-version">v{{ siteVersion }}</span>
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
import { SITE_FEATURES } from '@/utils/siteFeatures'
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
  UsersIcon
} from '@/components/layout/sidebar/navIcons'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()

const siteVersion = computed(() => appStore.siteVersion)

const flagChannelMonitor = makeSidebarFlag(FeatureFlags.channelMonitor)
const flagRiskControl = makeSidebarFlag(FeatureFlags.riskControl)
// Admin-only flags (not in public settings)
const flagOpsMonitoring = () => adminSettingsStore.opsMonitoringEnabled
const flagAdminPayment = () => adminSettingsStore.paymentEnabled

// 设置里的功能开关没开时入口变灰（applyFeatureFlags）；简易模式再收起 hideInSimpleMode 的项。
function visibleItems(items: NavItem[]): NavItem[] {
  const visible = applyFeatureFlags(items)
  return authStore.isSimpleMode ? visible.filter((item) => !item.hideInSimpleMode) : visible
}

// A3 导航（管理站改造方案，muqian 2026-09-24「渠道在前」）：概览 / 供给 / 用户 / 运营 / 安全 / 设置。
// 订阅、订单、审查各是一个入口，同组的页面在页头页签里切（activePaths 让同组页面都点亮这一项）；
// 账号安全只在右上角头像菜单里。功能没开时入口不消失，显示为灰色，点进去是「未开启 · 去设置打开」。
const sections = computed((): NavSection[] => {
  const groups: NavSection[] = [
    {
      key: 'overview',
      title: t('nav.sections.overview'),
      items: [
        { path: '/dashboard', label: t('nav.overview'), icon: DashboardIcon },
        { path: '/ops', label: t('nav.ops'), icon: ChartIcon, featureFlag: flagOpsMonitoring },
      ],
    },
    {
      key: 'supply',
      title: t('nav.sections.supply'),
      items: [
        // 渠道 = 资源（成品号 / 第三方 key）；模型决定上架与标价
        { path: '/accounts', label: t('nav.channels'), icon: GlobeIcon },
        { path: '/model-catalog', label: t('nav.models'), icon: PriceTagIcon },
        { path: '/channels/monitor', label: t('nav.channelHealth'), icon: SignalIcon, featureFlag: flagChannelMonitor },
        { path: '/proxies', label: t('nav.proxies'), icon: ServerIcon },
      ],
    },
    {
      key: 'users',
      title: t('nav.sections.users'),
      items: [
        { path: '/users', label: t('nav.users'), icon: UsersIcon, hideInSimpleMode: true },
        // 订阅由代码决定显不显示（utils/siteFeatures.ts），不显示时入口直接不出现；套餐属于订阅，不挂支付门
        ...(SITE_FEATURES.subscription
          ? [{ path: '/subscriptions', label: t('nav.subscriptions'), icon: CreditCardIcon, hideInSimpleMode: true, activePaths: ['/orders/plans'] }]
          : []),
        { path: '/orders', label: t('nav.orders'), icon: OrderIcon, hideInSimpleMode: true, featureFlag: flagAdminPayment, activePaths: ['/orders/dashboard'] },
      ],
    },
    {
      key: 'operations',
      title: t('nav.sections.operations'),
      items: [
        { path: '/usage', label: t('nav.usage'), icon: ChartIcon },
        { path: '/announcements', label: t('nav.announcements'), icon: BellIcon },
      ],
    },
    {
      key: 'security',
      title: t('nav.sections.security'),
      items: [
        { path: '/risk-control', label: t('nav.review'), icon: ShieldIcon, featureFlag: flagRiskControl, activePaths: ['/prompt-audit'] },
        { path: '/audit-logs', label: t('nav.auditLogs'), icon: ShieldIcon, hideInSimpleMode: true },
      ],
    },
    {
      key: 'settings',
      items: [{ path: '/settings', label: t('nav.settings'), icon: CogIcon }],
    },
  ]
  return groups.map((group) => ({ ...group, items: visibleItems(group.items) })).filter((group) => group.items.length > 0)
})

onMounted(() => {
  // 功能开关类菜单项（运维监控、支付管理）依赖管理设置
  void adminSettingsStore.fetch()
})
</script>
