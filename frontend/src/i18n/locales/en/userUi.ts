/**
 * Copy for the user-site shell and redesigned pages. Legacy nav.* keys stay for the admin console.
 */
export default {
  userUi: {
    nav: {
      usage: 'Usage',
      keys: 'API keys',
      models: 'Models',
      billing: 'Billing',
      account: 'Account',
      batchImage: 'Batch images',
      more: 'More',
      product: 'Product',
      pricing: 'Models & pricing',
      docs: 'Docs',
      console: 'Console',
      login: 'Sign in',
      openMenu: 'Open navigation',
      primaryNav: 'Primary'
    },
    topbar: {
      balance: 'Balance',
      available: 'Available',
      frozen: 'Frozen',
      total: 'Total',
      language: 'Language',
      theme: 'Theme',
      switchToLight: 'Switch to light',
      switchToDark: 'Switch to dark',
      accountMenu: 'Account menu'
    },
    billing: {
      title: 'Billing',
      description: 'Top up, subscriptions, orders and referrals',
      tabs: {
        recharge: 'Top up',
        subscriptions: 'Subscriptions',
        orders: 'Orders',
        affiliate: 'Referrals'
      }
    },
    usage: {
      title: 'Usage',
      description: 'Requests, tokens and cost, line by line',
      stats: {
        requests: 'Requests',
        tokens: 'Tokens',
        cost: 'Cost',
        standardCost: 'List price',
        balance: 'Available balance',
        avgLatency: 'Avg latency'
      },
      sections: {
        trend: 'Usage trend',
        models: 'Usage by model',
        records: 'Request details'
      },
      share: 'Share',
      retry: 'Retry',
      loadFailed: 'This section did not load',
      loadFailedHint: 'The endpoint is temporarily unavailable. Other sections are unaffected.',
      empty: 'No requests in this period',
      emptyHint: 'Try another date range, or create a key and make your first call.'
    },
    account: {
      title: 'Account',
      description: 'Profile, security and notifications',
      sections: {
        profile: 'Profile',
        security: 'Security',
        notifications: 'Notifications'
      }
    },
    status: {
      loading: 'Loading'
    }
  }
}
