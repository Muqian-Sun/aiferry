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
    path: '/audit-logs',
    name: 'AdminAuditLogs',
    component: () => import('@/views/admin/AuditLogView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Audit Logs',
      titleKey: 'nav.auditLogs',
      descriptionKey: 'admin.audit.description'
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
  {
    path: '/channels/monitor',
    name: 'AdminChannelMonitor',
    component: () => import('@/views/admin/ChannelMonitorView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Channel Monitor',
      titleKey: 'nav.channelHealth',
      descriptionKey: 'admin.channelMonitor.description'
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
      pageGroup: 'subscriptions'
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
    path: '/redeem',
    name: 'AdminRedeem',
    component: () => import('@/views/admin/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Redeem Code Management',
      titleKey: 'nav.redeemCodes',
      descriptionKey: 'admin.redeem.description'
    }
  },
  {
    path: '/settings',
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
      titleKey: 'nav.paymentDashboard',
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
      pageGroup: 'subscriptions'
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
