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
      title: 'Home'
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
      title: 'Verify Email'
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
      title: 'Reset Password'
    }
  },
  {
    path: '/key-usage',
    name: 'KeyUsage',
    component: () => import('@/views/KeyUsageView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Key Usage',
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
  {
    path: '/model-plaza',
    name: 'ModelPlaza',
    component: () => import('@/views/ModelPlazaView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Model Plaza',
      titleKey: 'modelPlaza.title'
    }
  },

  // ==================== User Routes ====================
  // 控制台五个页签：用量（落地页）· 密钥 · 模型 · 账务 · 账户。
  // 旧路径（/dashboard /purchase /subscriptions /orders /redeem /affiliate）长期保留 redirect，
  // 书签、邮件、支付回跳都不断。
  {
    path: '/',
    redirect: '/home'
  },
  {
    path: '/dashboard',
    redirect: '/usage'
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
      descriptionKey: 'keys.description'
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
      descriptionKey: 'batchImageGuide.description'
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
      descriptionKey: 'userUi.usage.description'
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
      // 索引落到第一个可见页签（支付关闭时是兑换码，不会被 requiresPayment 守卫弹走）
      { path: '', redirect: () => firstBillingPath(readBillingFlags()) },
      {
        // 路由名沿用 PurchaseSubscription：resolveRouteMetaKeys 据此按计费模式切换标题
        path: 'recharge',
        name: 'PurchaseSubscription',
        component: () => import('@/views/user/PaymentView.vue'),
        meta: {
          requiresAuth: true,
          requiresAdmin: false,
          title: 'Top up',
          titleKey: 'nav.buySubscription',
          descriptionKey: 'purchase.description',
          requiresPayment: true
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
          requiresSubscription: true
        }
      },
      {
        path: 'orders',
        name: 'OrderList',
        component: () => import('@/views/user/UserOrdersView.vue'),
        meta: {
          requiresAuth: true,
          requiresAdmin: false,
          title: 'My Orders',
          titleKey: 'nav.myOrders',
          requiresPayment: true
        }
      },
      {
        path: 'redeem',
        name: 'Redeem',
        component: () => import('@/views/user/RedeemView.vue'),
        meta: {
          requiresAuth: true,
          requiresAdmin: false,
          title: 'Redeem Code',
          titleKey: 'redeem.title',
          descriptionKey: 'redeem.description'
        }
      },
      {
        path: 'affiliate',
        name: 'Affiliate',
        component: () => import('@/views/user/AffiliateView.vue'),
        meta: {
          requiresAuth: true,
          requiresAdmin: false,
          title: 'Affiliate',
          titleKey: 'affiliate.title',
          descriptionKey: 'affiliate.description'
        }
      }
    ]
  },
  // 旧账务路径 → 新页签（保留 query，支付回跳 / 订阅购买入口都带参数）
  // /purchase?tab=subscription 仍由充值页签内的 PaymentView 处理（套餐购买并入订阅页签是 S5 的事）
  { path: '/purchase', redirect: (to) => ({ path: '/billing/recharge', query: to.query }) },
  { path: '/subscriptions', redirect: (to) => ({ path: '/billing/subscriptions', query: to.query }) },
  { path: '/orders', redirect: (to) => ({ path: '/billing/orders', query: to.query }) },
  { path: '/redeem', redirect: (to) => ({ path: '/billing/redeem', query: to.query }) },
  { path: '/affiliate', redirect: (to) => ({ path: '/billing/affiliate', query: to.query }) },
  {
    path: '/available-channels',
    name: 'UserAvailableChannels',
    component: () => import('@/views/user/AvailableChannelsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Available Channels',
      titleKey: 'availableChannels.title',
      descriptionKey: 'availableChannels.description'
    }
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('@/views/user/ProfileView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Account',
      titleKey: 'userUi.account.title',
      descriptionKey: 'userUi.account.description'
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
    component: () => import('@/views/user/ChannelStatusView.vue'),
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
