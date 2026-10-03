/**
 * 管理后台路由。管理后台是独立站点，路径不带 /admin 前缀。
 */
import type { RouteRecordRaw } from 'vue-router'

export const adminRoutes: RouteRecordRaw[] = [
  // ==================== Setup Routes ====================
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/setup/SetupWizardView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Setup'
    }
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Login',
      titleKey: 'home.login'
    }
  },
  {
    path: '/legal/:documentId',
    name: 'LegalDocument',
    component: () => import('@/views/public/LegalDocumentView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Legal Document'
    }
  },

  // ==================== Admin Routes ====================
  {
    path: '/',
    redirect: '/dashboard'
  },
  {
    path: '/dashboard',
    name: 'AdminDashboard',
    component: () => import('@/views/admin/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Admin Dashboard',
      titleKey: 'nav.overview',
      descriptionKey: 'admin.dashboard.description'
    }
  },
  {
    path: '/ops',
    name: 'AdminOps',
    component: () => import('@/views/admin/ops/OpsDashboard.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Ops Monitoring',
      titleKey: 'nav.ops',
      descriptionKey: 'admin.ops.description',
      hidePageHeader: true
    }
  },
  {
    path: '/users',
    name: 'AdminUsers',
    component: () => import('@/views/admin/UsersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'User Management',
      titleKey: 'nav.users',
      descriptionKey: 'admin.users.description'
    }
  },
  {
    path: '/channels',
    redirect: '/model-catalog'
  },
  {
    path: '/model-catalog',
    name: 'AdminModelCatalog',
    component: () => import('@/views/admin/ModelCatalogView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Model Catalog',
      titleKey: 'nav.models',
      descriptionKey: 'admin.modelCatalog.description'
    }
  },
  // 新建 / 编辑模型整页（2026-09-25，原来是列表页里的弹窗）；侧栏按前缀匹配，/model-catalog/* 仍点亮「模型」
  {
    path: '/model-catalog/new',
    name: 'AdminModelCatalogCreate',
    component: () => import('@/views/admin/ModelCatalogEntryFormView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Add Model',
      titleKey: 'admin.modelCatalog.create'
    }
  },
  {
    path: '/model-catalog/:id/edit',
    name: 'AdminModelCatalogEdit',
    component: () => import('@/views/admin/ModelCatalogEntryFormView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Edit Model',
      titleKey: 'admin.modelCatalog.edit'
    }
  },
  {
    path: '/subscriptions',
    name: 'AdminSubscriptions',
    component: () => import('@/views/admin/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Subscription Management',
      titleKey: 'admin.subscriptions.title',
      descriptionKey: 'admin.subscriptions.description',
      pageGroup: 'subscriptions',
      siteFeature: 'subscription'
    }
  },
  {
    path: '/accounts',
    name: 'AdminAccounts',
    component: () => import('@/views/admin/AccountsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Account Management',
      titleKey: 'nav.channels',
      descriptionKey: 'admin.accounts.description'
    }
  },
  // 渠道状态（muqian 2026-09-30）：按渠道看可用率 / 首字延迟 / 缓存命中率，数据与用户站服务状态同源
  {
    path: '/channels/status',
    name: 'AdminChannelStatus',
    component: () => import('@/views/admin/ChannelStatusView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Channel Status',
      titleKey: 'nav.channelStatus',
      descriptionKey: 'admin.channelStatus.description'
    }
  },
  {
    path: '/announcements',
    name: 'AdminAnnouncements',
    component: () => import('@/views/admin/AnnouncementsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Announcements',
      titleKey: 'nav.announcements',
      descriptionKey: 'admin.announcements.description'
    }
  },
  {
    path: '/pricing',
    name: 'AdminPricing',
    component: () => import('@/views/admin/PricingView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Pricing',
      titleKey: 'nav.pricing',
      descriptionKey: 'admin.pricing.description'
    }
  },
  {
    path: '/proxies',
    name: 'AdminProxies',
    component: () => import('@/views/admin/ProxiesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Proxy Management',
      titleKey: 'nav.proxies',
      descriptionKey: 'admin.proxies.description'
    }
  },
  {
    // A6：每个小节一个地址，缺省落到第一节（settings/sections.ts）
    path: '/settings/:section?',
    name: 'AdminSettings',
    component: () => import('@/views/admin/SettingsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'System Settings',
      titleKey: 'nav.settings',
      descriptionKey: 'admin.settings.description'
    }
  },
  {
    path: '/risk-control',
    name: 'AdminRiskControl',
    component: () => import('@/views/admin/RiskControlView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Risk Control',
      titleKey: 'admin.riskControl.title',
      descriptionKey: 'admin.riskControl.description',
      requiresRiskControl: true,
      pageGroup: 'review'
    }
  },
  {
    path: '/prompt-audit',
    name: 'AdminPromptAudit',
    component: () => import('@/features/prompt-audit/PromptAuditView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Prompt Audit',
      titleKey: 'admin.promptAudit.title',
      descriptionKey: 'admin.promptAudit.description',
      requiresRiskControl: true,
      pageGroup: 'review'
    }
  },
  {
    path: '/usage',
    name: 'AdminUsage',
    component: () => import('@/views/admin/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Usage Records',
      titleKey: 'nav.usage',
      descriptionKey: 'admin.usage.description'
    }
  },


  // ==================== Payment Admin Routes ====================
  {
    path: '/orders/dashboard',
    name: 'AdminPaymentDashboard',
    component: () => import('@/views/admin/orders/AdminPaymentDashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Payment Dashboard',
      titleKey: 'nav.tabs.collections',
      requiresPayment: true,
      pageGroup: 'orders'
    }
  },
  {
    path: '/orders',
    name: 'AdminOrders',
    component: () => import('@/views/admin/orders/AdminOrdersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Order Management',
      titleKey: 'nav.orderManagement',
      requiresPayment: true,
      pageGroup: 'orders'
    }
  },
  {
    path: '/orders/plans',
    name: 'AdminPaymentPlans',
    component: () => import('@/views/admin/orders/AdminPaymentPlansView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Subscription Plans',
      titleKey: 'nav.paymentPlans',
      pageGroup: 'subscriptions',
      siteFeature: 'subscription'
    }
  },

  // 侧栏灰色入口（功能未开启）的落地页（A3）
  {
    path: '/feature-off',
    name: 'AdminFeatureOff',
    component: () => import('@/views/admin/FeatureOffView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Feature off',
      titleKey: 'admin.featureOff.title'
    }
  },

  // ==================== 404 Not Found ====================
  {
    path: '/profile',
    name: 'AdminAccountSecurity',
    component: () => import('@/views/admin/AccountSecurityView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Account Security',
      titleKey: 'nav.accountSecurity'
    }
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
    meta: {
      title: '404 Not Found'
    }
  }
]
