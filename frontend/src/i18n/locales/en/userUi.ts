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
      home: 'Home',
      product: 'Product',
      help: 'Docs & help',
      legal: 'Legal',
      register: 'Sign up'
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
      hero: {
        eyebrow: '{models} models · 4 official protocols · per-request ledger',
        title: 'One API key,\nevery major model.',
        description: 'Requests pass through on the official Anthropic, OpenAI and Gemini protocols — keep your SDK, change one base_url. Every request is billed and recorded line by line.',
        getStarted: 'Get started',
        goToConsole: 'Open console',
        viewPricing: 'Models & pricing',
        vendorsLabel: 'Vendors with listed models'
      },
      features: {
        eyebrow: 'Why here',
        title: 'We take the complexity, you keep the interface',
        description: 'Behind one endpoint sits unified scheduling, pricing and a ledger; all you change is base_url.',
        items: {
          protocol: { title: 'Official protocols, untouched', body: 'Same-protocol resources come first and nothing is rewritten into a private format; SDKs, streaming and tool calls behave exactly as direct.' },
          noFallback: { title: 'Never swaps your model', body: 'The model you ask for is the model you get. The only fallback is another channel for the same model — never a silent downgrade.' },
          pricing: { title: 'Listed means priced', body: 'Every listed model shows its list price; you pay list price × your account multiplier, visible before you call.' },
          sticky: { title: 'One conversation, one upstream', body: 'A session stays on the same upstream so cache hits keep landing — long conversations get cheaper and steadier.' },
          ledger: { title: 'Line-by-line records', body: 'The usage page lists model, tokens, latency and cost per request, with filters and CSV export.' },
          balance: { title: 'Transparent balance', body: 'Available and frozen balance are shown separately; every charge traces back to a request.' }
        }
      },
      quickstart: {
        eyebrow: 'Quick start',
        title: 'A few lines of code, every model',
        description: 'Four official protocols supported as-is: swap the API key and base_url and call.',
        steps: {
          create: { title: 'Create a key', body: 'Generate an API key under Keys in the console, with optional quota and rate limits.' },
          baseUrl: { title: 'Replace base_url', body: 'Keep your SDK and request format; only point the upstream at this site.' },
          call: { title: 'Call, then check the ledger', body: 'Tokens, latency and cost are recorded per request and exportable any time.' }
        },
        links: {
          docs: { title: 'Integration docs', body: 'Calling from IDEs and agents' },
          clients: { title: 'Client configs', body: 'Using desktop and CLI clients' }
        },
        sample: {
          request: 'REQUEST',
          response: 'RESPONSE',
          copy: 'Copy request',
          copied: 'Copied',
          tabs: { messages: 'Messages', responses: 'Responses', chat: 'Chat', gemini: 'Gemini' }
        }
      },
      catalog: {
        eyebrow: 'Public pricing',
        title: 'Models and list prices',
        description: 'List prices are this site\'s catalog prices (USD per 1M tokens), same source as the models page; you pay list price × your account multiplier.',
        viewAll: 'See all {count} models'
      },
      clients: {
        eyebrow: 'Integrations',
        title: 'Use it from the clients you already have',
        description: 'Point these tools at this base_url with your key and they just work; each one has a copyable config snippet under Keys → Use key.',
        cta: 'Get the config on the keys page',
        items: {
          claude: 'Environment variables pointed at this site; Anthropic Messages passes through untouched.',
          codex: 'OpenAI Responses protocol, with configs for both HTTP and WebSocket transport.',
          gemini: 'Gemini generateContent protocol; an API key is all it needs.',
          grok: 'OpenAI Chat compatible protocol pointed at /v1 on this site.',
          opencode: 'One provider config; pick model names from the models page.'
        },
        sdk: { title: 'Any OpenAI / Anthropic SDK', body: 'Official SDKs and every compatible library work directly: change base_url, leave the rest.' }
      },
      stats: {
        models: 'Listed models',
        vendors: 'Vendors',
        protocols: 'Official protocols, untouched',
        ledgerLabel: 'Requests recorded, exportable',
        ledgerValue: 'Every one'
      },
      cta: {
        eyebrow: 'Official protocols · listed means priced · per-request ledger',
        title: 'Ready to start?',
        getStarted: 'Start for free',
        goToConsole: 'Open console'
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
