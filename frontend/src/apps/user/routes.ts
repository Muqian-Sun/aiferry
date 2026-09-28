/**
 * 用户站路由。管理页面不在此注册，见 apps/admin/routes.ts。
 */
import type { RouteRecordRaw } from 'vue-router'
import { firstBillingPath } from '@/views/user/billing/billingTabs'
import { readBillingFlags } from '@/views/user/billing/useBillingFlags'

export const userRoutes: RouteRecordRaw[] = [

  // ==================== Public Routes ====================
  {
    path: '/home',
    name: 'Home',
    component: () => import('@/views/HomeView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Home',
      titleKey: 'userUi.home.tabTitle',
      titleBrandFirst: true
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
    path: '/register',
    name: 'Register',
    component: () => import('@/views/auth/RegisterView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Register',
      titleKey: 'auth.createAccount'
    }
  },
  {
    path: '/email-verify',
    name: 'EmailVerify',
    component: () => import('@/views/auth/EmailVerifyView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Verify Email',
      titleKey: 'auth.verifyYourEmail'
    }
  },
  {
    path: '/auth/callback',
    name: 'OAuthCallback',
    alias: '/auth/oauth/callback',
    component: () => import('@/views/auth/OAuthCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'OAuth Callback',
      titleKey: 'auth.oauthCallbackPageTitle'
    }
  },
  {
    path: '/auth/linuxdo/callback',
    name: 'LinuxDoOAuthCallback',
    component: () => import('@/views/auth/LinuxDoCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'LinuxDo OAuth Callback',
      titleKey: 'auth.linuxdoCallbackPageTitle'
    }
  },
  {
    path: '/auth/wechat/callback',
    name: 'WeChatOAuthCallback',
    component: () => import('@/views/auth/WechatCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'WeChat OAuth Callback',
      titleKey: 'auth.wechatCallbackPageTitle'
    }
  },
  {
    path: '/auth/wechat/payment/callback',
    name: 'WeChatPaymentOAuthCallback',
    component: () => import('@/views/auth/WechatPaymentCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'WeChat Payment Callback',
      titleKey: 'auth.wechatPaymentCallbackPageTitle'
    }
  },
  {
    path: '/auth/dingtalk/callback',
    name: 'DingTalkOAuthCallback',
    component: () => import('@/views/auth/DingTalkCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'DingTalk OAuth Callback',
      titleKey: 'auth.dingtalkCallbackPageTitle'
    }
  },
  {
    path: '/auth/dingtalk/email-completion',
    name: 'dingtalk-email-completion',
    component: () => import('@/views/auth/DingTalkEmailCompletionView.vue'),
    meta: {
      requiresAuth: false,
      title: 'DingTalk Email Completion'
    }
  },
  {
    path: '/auth/oidc/callback',
    name: 'OIDCOAuthCallback',
    component: () => import('@/views/auth/OidcCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'OIDC OAuth Callback',
      titleKey: 'auth.oidcCallbackPageTitle'
    }
  },
  {
    path: '/forgot-password',
    name: 'ForgotPassword',
    component: () => import('@/views/auth/ForgotPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Forgot Password',
      titleKey: 'auth.forgotPasswordTitle'
    }
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    component: () => import('@/views/auth/ResetPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Reset Password',
      titleKey: 'auth.resetPasswordTitle'
    }
  },
  {
    path: '/key-usage',
    name: 'KeyUsage',
    component: () => import('@/views/KeyUsageView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Key Usage',
      titleKey: 'keyUsage.title',
    }
  },
  {
    path: '/legal/:documentId',
    name: 'LegalDocument',
    component: () => import('@/views/public/LegalDocumentView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Legal Document',
      titleKey: 'userUi.home.legalTabTitle'
    }
  },
  {
    path: '/model-plaza',
    name: 'ModelPlaza',
    component: () => import('@/views/ModelPlazaView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Models',
      titleKey: 'userUi.models.title',
      descriptionKey: 'userUi.models.description',
      preload: (_to, prefetch) => import('@/views/modelPlazaQuery').then((m) => m.preloadModelPlaza(prefetch))
    }
  },

  // ==================== User Routes ====================
  // 控制台五个页签：用量（落地页）· 密钥 · 模型 · 账务 · 账户。
  // 旧路径（/purchase /subscriptions）长期保留 redirect，
  // 书签、邮件、支付回跳都不断。
  {
    path: '/',
    redirect: '/home'
  },
  {
    // 控制台落地页（muqian 2026-09-23 新增「概览」：余额 / 今日 / 快速开始 / 趋势 / 公告）
    path: '/dashboard',
    name: 'Overview',
    component: () => import('@/views/user/OverviewView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Overview',
      titleKey: 'userUi.overview.title',
      preload: (to, prefetch) => import('@/views/user/overviewQuery').then((m) => m.preloadOverview(to, prefetch))
    }
  },
  {
    path: '/keys',
    name: 'Keys',
    component: () => import('@/views/user/KeysView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'API Keys',
      titleKey: 'keys.title',
      descriptionKey: 'keys.description',
      preload: (to, prefetch) => import('@/views/user/keysQuery').then((m) => m.preloadKeys(to, prefetch))
    }
  },
  {
    path: '/batch-image',
    name: 'BatchImageGuide',
    alias: '/docs/batch-image',
    component: () => import('@/views/user/BatchImageGuideView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Batch Image Guide',
      titleKey: 'batchImageGuide.title',
      descriptionKey: 'batchImageGuide.description',
      siteFeature: 'batchImage'
    }
  },
  {
    path: '/usage',
    name: 'Usage',
    component: () => import('@/views/user/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Usage',
      titleKey: 'userUi.usage.title',
      descriptionKey: 'userUi.usage.description',
      // 数据到了再换页：首屏几个请求在进入前发完（muqian 2026-09-27）
      preload: (to, prefetch) => import('@/views/user/usageQuery').then((m) => m.preloadUsage(to, prefetch))
    }
  },

  // ---------- 账务：四个页签共用一个页头 ----------
  {
    path: '/billing',
    component: () => import('@/views/user/billing/BillingView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Billing',
      titleKey: 'userUi.billing.title',
      descriptionKey: 'userUi.billing.description'
    },
    children: [
      // 索引落到第一个可见页签（不会被 requiresPayment / siteFeature 守卫弹走）
      { path: '', redirect: () => firstBillingPath(readBillingFlags()) },
      {
        // 路由名沿用 PurchaseSubscription：resolveRouteMetaKeys 据此按计费模式切换标题
        path: 'recharge',
        name: 'PurchaseSubscription',
        component: () => import('@/views/user/PaymentView.vue'),
        props: { mode: 'recharge' },
        meta: {
          requiresAuth: true,
          requiresAdmin: false,
          title: 'Top up',
          titleKey: 'nav.recharge',
          descriptionKey: 'purchase.rechargeDescription',
          requiresPayment: true,
          preload: (_to, prefetch) => import('@/views/user/billing/checkoutPreload').then((m) => m.preloadCheckoutInfo(prefetch))
        }
      },
      {
        path: 'subscriptions',
        name: 'Subscriptions',
        component: () => import('@/views/user/SubscriptionsView.vue'),
        meta: {
          requiresAuth: true,
          requiresAdmin: false,
          title: 'My Subscriptions',
          titleKey: 'userSubscriptions.title',
          descriptionKey: 'userSubscriptions.description',
          siteFeature: 'subscription'
        }
      }
    ]
  },
  // 旧账务路径 → 新页签（保留 query，支付回跳 / 订阅购买入口都带参数）
  // /purchase 按意图分流：?tab=subscription（续费入口）或 order_type=subscription（微信授权回跳）去订阅页签，
  // 其余去充值页签。后端微信支付授权的 redirect 仍写死 /purchase，所以这条 redirect 长期保留。
  {
    path: '/purchase',
    redirect: (to) => {
      const subscription = to.query.tab === 'subscription' || to.query.order_type === 'subscription'
      const { tab: _tab, ...query } = to.query
      return { path: subscription ? '/billing/subscriptions' : '/billing/recharge', query }
    }
  },
  { path: '/subscriptions', redirect: (to) => ({ path: '/billing/subscriptions', query: to.query }) },
  {
    // 账户拆成三个子页（muqian 2026-09-23：侧栏「账户」组），共用 ProfileView 按 section 渲染
    path: '/profile',
    name: 'Profile',
    component: () => import('@/views/user/ProfileView.vue'),
    props: { section: 'profile' },
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Profile',
      titleKey: 'userUi.account.sections.profile',
      descriptionKey: 'userUi.account.descriptions.profile'
    }
  },
  {
    path: '/profile/security',
    name: 'ProfileSecurity',
    component: () => import('@/views/user/ProfileView.vue'),
    props: { section: 'security' },
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Security',
      titleKey: 'userUi.account.sections.security',
      descriptionKey: 'userUi.account.descriptions.security',
      siteFeature: 'accountSecurity'
    }
  },
  {
    path: '/profile/notifications',
    name: 'ProfileNotifications',
    component: () => import('@/views/user/ProfileView.vue'),
    props: { section: 'notifications' },
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Notifications',
      titleKey: 'userUi.account.sections.notifications',
      descriptionKey: 'userUi.account.descriptions.notifications'
    }
  },
  {
    path: '/payment/qrcode',
    name: 'PaymentQRCode',
    component: () => import('@/views/user/PaymentQRCodeView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Payment',
      titleKey: 'payment.qr.scanToPay',
      requiresPayment: true
    }
  },
  {
    path: '/payment/result',
    name: 'PaymentResult',
    component: () => import('@/views/user/PaymentResultView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Payment Result',
      titleKey: 'payment.result.success',
      requiresPayment: false
    }
  },
  {
    path: '/payment/stripe',
    name: 'StripePayment',
    component: () => import('@/views/user/StripePaymentView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Stripe Payment',
      titleKey: 'payment.stripePay',
      requiresPayment: false
    }
  },
  {
    path: '/payment/airwallex',
    name: 'AirwallexPayment',
    component: () => import('@/views/user/AirwallexPaymentView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Airwallex Payment',
      titleKey: 'payment.airwallexPay',
      requiresPayment: false
    }
  },
  {
    path: '/payment/stripe-popup',
    name: 'StripePopup',
    component: () => import('@/views/user/StripePopupView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Payment',
      requiresPayment: false
    }
  },
  {
    path: '/custom/:id',
    name: 'CustomPage',
    component: () => import('@/views/user/CustomPageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Custom Page',
      titleKey: 'customPage.title',
    }
  },
  {
    path: '/monitor',
    name: 'ChannelStatus',
    component: () => import('@/views/user/ChannelStatusV2View.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Channel Status',
      titleKey: 'nav.channelStatus'
    }
  },

  // ==================== 404 Not Found ====================
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
    meta: {
      title: '404 Not Found'
    }
  }
]
