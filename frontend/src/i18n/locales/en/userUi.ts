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
    footer: {
      register: 'Sign up',
      rights: 'All rights reserved'
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
    home: {
      tabTitle: 'One key, every leading model',
      legalTabTitle: 'Policies',
      hero: {
        title: 'One key,',
        titleAccent: 'every leading model',
        description: 'AiFerry relays your requests on the official protocols — never rewritten, never swapped for another model, never sold. Change one base_url and your existing SDKs keep working.',
        getStarted: 'Get started',
        goToConsole: 'Open console',
        viewPricing: 'Models & pricing',
        points: {
          protocol: 'Official protocols, untouched',
          sameModel: 'Next channel, never another model',
          ledger: 'Pay per use, every request on the ledger'
        },
        vendorsLabel: 'Vendors connected'
      },
      features: {
        eyebrow: 'How we work',
        title: 'One job — relaying — done properly',
        description: 'The less a middle layer does, the better: we do not rewrite your request, swap your model, or sell your data.',
        items: {
          passthrough: { title: 'Pass through as-is', body: 'Four official protocols relayed untouched — no private format, no rewriting. SDKs, streaming, tool calls and multimodal behave exactly as direct.' },
          failover: { title: 'No cross-model fallback', body: 'The model you name is the model that runs. On a timeout or error we only move between channels serving that same model — never a quiet downgrade to something cheaper.' },
          cache: { title: 'Cache to the limit', body: 'A session stays pinned to one upstream so prompt caches keep hitting. Long conversations get faster as they go, and cheaper with them.' },
          privacy: { title: 'Conversations not logged', body: 'Successful requests leave no request or reply on our side — only one ledger line: model, tokens, latency, cost. Failed requests are kept for troubleshooting and deleted after 30 days.' },
          noSale: { title: 'Never sold', body: 'Not sold, not rented, not used for training. Beyond the upstream model that serves your request, your requests and bills go to no third party.' }
        },
        figure: {
          passthrough: { endpoint: 'One base_url', note: 'same protocol, passed through' },
          failover: { channelA: 'Channel A', channelB: 'Channel B', timeout: 'timeout', switched: 'same model, next channel' },
          cache: { session: 'session', pinned: 'pinned upstream', request: 'request', hit: 'cache hit' },
          privacy: { request: 'Request body', notStored: 'not stored', kept: 'only this line' },
          noSale: { yourData: 'your data', barrier: 'not sold · not shared · not trained on', thirdParty: 'third parties', ads: 'ads', brokers: 'brokers' }
        }
      },
      stats: {
        models: 'Models connected',
        vendors: 'Vendors',
        protocols: 'Official protocols',
        clients: 'Client configs'
      },
      protocols: { messages: 'Messages', responses: 'Responses', chat: 'Chat', gemini: 'Gemini' }
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
        recharge: 'Top up',
        totalCost: 'Total spent',
        totalRequests: 'Total requests',
        rate: 'Current rate (last 5 min)',
        todayCost: 'Today cost',
        todayRequests: 'Today requests',
        todayTokens: 'Today tokens',
        avgLatency: 'Avg latency'
      },
      sections: {
        announcements: 'Announcements',
        trend: 'Usage trend',
        models: 'Usage by model',
        records: 'Request details'
      },
      announcements: {
        unread: '{count} unread'
      },
      trend: {
        tokens: 'Tokens',
        requests: 'Requests',
        cost: 'Cost',
        rangeSummary: '{requests} requests · {tokens} tokens · {cost} in this range',
        empty: 'No data in this period'
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
    models: {
      title: 'Models',
      description: 'Listed models and their list prices, billed per token or per request',
      allVendors: 'All',
      allBilling: 'All billing',
      searchHint: 'Search models or aliases (press / to focus)',
      count: '{count} models',
      view: { label: 'View', table: 'Table', grid: 'Grid' },
      yourPrice: 'Your price',
      yourPriceHint: '= list × {multiplier}',
      multiplierNote: 'Your account multiplier is {multiplier}.',
      perMillionShort: '$ / 1M tokens',
      timePricing: 'Time-based',
      weekdaysOnly: 'weekdays only',
      columns: {
        model: 'Model',
        vendor: 'Vendor',
        billing: 'Billing',
        input: 'Input',
        output: 'Output',
        cacheRead: 'Cache read'
      },
      perMillion: 'USD per 1M tokens',
      listPrice: 'List price',
      priceNote: 'List prices are this site\'s catalog prices (USD per million tokens); per-request models show no token rates. You pay list price × your account multiplier, recorded per request on the usage page.',
      copyId: 'Copy model ID',
      copied: 'Copied',
      empty: 'No models available',
      noSearchResult: 'No models match',
      loadFailed: 'The model catalog did not load'
    },
    notFound: {
      title: 'Page not found',
      description: 'The page you are looking for does not exist or has been moved.',
      back: 'Go back',
      home: 'Back to home'
    },
    status: {
      loading: 'Loading'
    }
  }
}
