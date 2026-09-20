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
    home: {
      heroTitle: 'One API key\nfor every major model.',
      heroDescription: 'Requests pass through unchanged over the official Anthropic, OpenAI and Gemini protocols. Keep your SDK, change one base_url. Every call is logged, so usage and cost live in one ledger.',
      getStarted: 'Get started',
      goToConsole: 'Open console',
      viewPricing: 'Models & pricing',
      codeSample: {
        label: 'Integration examples',
        comment: 'Only these two lines change',
        copy: 'Copy',
        copied: 'Copied',
        tabs: { python: 'Python', curl: 'curl', node: 'Node', claudeCode: 'Claude Code' }
      },
      stats: {
        models: 'Models',
        vendors: 'Vendors',
        protocols: 'Official protocols, passed through',
        ledgerLabel: 'Requests logged, exportable',
        ledgerValue: 'Every one'
      },
      catalog: {
        title: 'Models and official reference prices',
        description: 'USD per million tokens, same source as the models page.',
        viewAll: 'See all {count} models'
      },
      protocols: {
        title: 'Four official protocols, passed through unchanged',
        description: 'No private format conversion: request and response bodies travel exactly as the vendor protocol defines them, so SDKs, streaming and tool calls behave as they do direct.'
      },
      routeMap: {
        routes: {
          messages: 'Anthropic Messages',
          responses: 'OpenAI Responses',
          chat: 'Chat Completions',
          gemini: 'Gemini Generate'
        },
        vendors: {
          messages: 'Claude',
          responses: 'GPT',
          chat: 'GPT, DeepSeek, Qwen, Grok',
          gemini: 'Gemini'
        }
      },
      steps: {
        title: 'Three steps to integrate',
        create: { title: 'Create a key', body: 'Generate an API key in the console; scope it by group and quota.' },
        baseUrl: { title: 'Change one base_url', body: 'Keep your SDK and request format; point the upstream at this site.' },
        watch: { title: 'Send requests, review usage', body: 'Tokens, latency and cost are recorded per request and exportable any time.' }
      },
      facts: {
        title: 'What you can verify',
        protocol: { title: 'Official protocols', body: 'No proprietary translation: request and response bodies pass through as each vendor defines them.' },
        pricing: { title: 'Public pricing', body: 'The models page lists each model\'s official reference price, billed per token.' },
        ledger: { title: 'Line-by-line records', body: 'The usage page lists model, tokens, latency and cost per request, with filters and CSV export.' },
        balance: { title: 'Transparent balance', body: 'Available and frozen balance are shown separately; every charge traces back to a request.' }
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
    models: {
      title: 'Models',
      description: 'Official reference price per model, billed per token',
      allVendors: 'All vendors',
      count: '{count} models',
      columns: {
        model: 'Model',
        vendor: 'Vendor',
        billing: 'Billing',
        input: 'Input',
        output: 'Output',
        cacheRead: 'Cache read'
      },
      perMillion: 'USD per 1M tokens',
      officialPrice: 'Official price',
      priceNote: 'Prices are the vendors\' published reference prices (USD per million tokens); blank cells mean the official catalog does not cover that item. Actual charges follow the per-request records on the usage page; this site\'s discounted price will appear here once unified pricing ships.',
      copyId: 'Copy model ID',
      copied: 'Copied',
      empty: 'No models available',
      noSearchResult: 'No models match',
      loadFailed: 'The model catalog did not load',
      anonymousHint: 'Sign in to see the models available to your account'
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
