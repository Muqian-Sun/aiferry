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
      heroTitle: 'One API key\nfor every major model.',
      heroDescription: 'Requests pass through unchanged over the official Anthropic, OpenAI and Gemini protocols. Keep your SDK, change one base_url. Every call is logged, so usage and cost live in one ledger.',
      getStarted: 'Get started',
      goToConsole: 'Open console',
      viewPricing: 'Models & pricing',
      vendorsLabel: 'Vendors with listed models',
      clients: {
        title: 'Clients that work out of the box',
        description: 'Point these tools at this base_url with your key and they just work; each one has a copyable config snippet under Keys → Use key.',
        cta: 'Get the config on the keys page',
        items: {
          claude: 'Environment variables pointed at this site; Anthropic Messages passes through untouched.',
          codex: 'OpenAI Responses protocol, with configs for both HTTP and WebSocket transport.',
          gemini: 'Gemini generateContent protocol; an API key is all it needs.',
          grok: 'OpenAI Chat compatible protocol pointed at /v1 on this site.',
          opencode: 'One provider config; pick model names from the models page.'
        }
      },
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
        title: 'Models and list prices',
        description: 'List prices in USD per million tokens, same source as the models page; you pay list price × your account multiplier.',
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
        pricing: { title: 'Public pricing', body: 'The models page lists every listed model\'s list price; you pay list price × your account multiplier.' },
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
      description: 'Listed models and their list prices, billed per token or per request',
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
