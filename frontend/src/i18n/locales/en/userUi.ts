/**
 * Copy for the user-site shell and redesigned pages. Legacy nav.* keys stay for the admin console.
 */
export default {
  userUi: {
    nav: {
      overview: 'Overview',
      usage: 'Usage details',
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
      primaryNav: 'Primary',
      collapseSidebar: 'Collapse',
      expandSidebar: 'Expand sidebar'
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
    overview: {
      title: 'Overview',
      description: 'Balance, today\'s usage and how to connect',
      greeting: {
        lateNight: 'Working late, {name}',
        morning: 'Good morning, {name}',
        noon: 'Good afternoon, {name}',
        afternoon: 'Good afternoon, {name}',
        evening: 'Good evening, {name}'
      },
      totals: 'Total spent {cost} · {requests} requests · {rpm} RPM now',
      quickStart: {
        title: 'Quick start',
        description: 'Put the base URL and a key into your SDK or client and start calling.',
        baseUrl: 'Base URL',
        key: 'Your key',
        noKey: 'No active key yet',
        allKeys: 'All keys',
        createKey: 'Create a key',
        copy: 'Copy',
        copied: 'Copied',
        example: 'Example',
        exampleHint: 'Replace $API_KEY with the key copied above, or export API_KEY=your-key first',
        exampleMessage: 'Hello'
      },
      trend: {
        title: 'Usage trend',
        range: 'Last {days} days',
        summary: 'Last {days} days · {requests} requests · {tokens} tokens · {cost}'
      }
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
      title: 'Usage details',
      description: 'Model usage and every request, by date range',
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
      sections: {
        profile: 'Profile',
        security: 'Security',
        notifications: 'Notifications'
      },
      descriptions: {
        profile: 'Avatar, username and account details',
        security: 'Linked sign-ins, password, two-factor and passkeys',
        notifications: 'Email alerts when your balance runs low'
      },
      notificationsOff: 'Balance alerts are not enabled on this site',
      rows: {
        overview: 'Account',
        overviewDesc: 'Sign-in identity, balance and rate multiplier',
        usernameDesc: 'Your email is shown when this is empty',
        passwordDesc: 'Used for email sign-in, at least 8 characters'
      }
    },
    summary: {
      keys: 'Keys',
      activeKeys: 'Active',
      activePlans: 'Active plans',
      nearestExpiry: 'Next expiry',
      daysLeft: '{days} days left',
      noExpiry: 'No expiry',
      planModels: 'Models included',
      redeemHistoryDesc: 'Redemptions and admin adjustments'
    },
    models: {
      title: 'Models',
      description: 'Listed models and their list prices, billed per token or per request',
      hero: {
        title: 'Every model, ',
        titleAccent: 'priced in the open',
        description: 'Every listed model and its list price, in one place. Search or filter by vendor, then copy a model ID and call it.'
      },
      allVendors: 'All',
      allBilling: 'All billing',
      searchHint: 'Search models or aliases (press / to focus)',
      vendorTabsLabel: 'Vendor',
      priceUnit: 'Prices in USD per 1M tokens',
      yourPriceApplied: 'showing your price (list × {multiplier})',
      multiplierNote: 'Your account multiplier is {multiplier}.',
      timePricing: 'Time-based',
      weekdaysOnly: 'weekdays only',
      prices: {
        input: 'Input',
        output: 'Output',
        cacheWrite: 'Cache write',
        cacheWrite1h: 'Cache write (1h)',
        cacheRead: 'Cache read',
        imageInput: 'Image input',
        imageOutput: 'Image output',
        perRequest: 'Per request',
        perImage: 'Per image',
        perSecond: 'Per second'
      },
      priceNote: 'Prices are this site\'s catalog list prices. You pay list price × your account multiplier, recorded per request on the usage page.',
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
