/**
 * Copy for the user-site shell and redesigned pages. Legacy nav.* keys stay for the admin console.
 */
export default {
  userUi: {
    nav: {
      overview: 'Overview',
      usage: 'Usage details',
      keys: 'API keys',
      billing: 'Billing',
      account: 'Account',
      batchImage: 'Batch images',
      status: 'Status',
      more: 'More',
      product: 'Product',
      pricing: 'Models & pricing',
      docs: 'Docs',
      console: 'Console',
      login: 'Sign in',
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
      debt: 'Owed',
      frozen: 'Frozen',
      rechargeViaSupport: 'Contact support to top up',
      language: 'Language',
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
          privacy: { title: 'Conversations not logged', body: 'We store neither your requests nor the model replies — only one ledger line: model, tokens, latency, cost. Failed requests keep only the error for troubleshooting, deleted after 30 days; cross-protocol calls cache the reasoning summary for 7 days; content-audit retention, if enabled, is set out in the privacy policy.' },
          noSale: { title: 'Never sold', body: 'Not sold, not rented, not used for training. Beyond the upstream model that serves your request (and the moderation service, if content audit is on), your requests and bills go to no third party.' }
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
      greeting: {
        lateNight: 'Working late, {name}',
        morning: 'Good morning, {name}',
        noon: 'Good afternoon, {name}',
        afternoon: 'Good afternoon, {name}',
        evening: 'Good evening, {name}'
      },
      range: {
        label: 'Time range',
        days: '{days} days'
      },
      numbers: {
        today: 'Tokens today',
        range: 'Tokens, last {days} days',
        total: 'Tokens, all time'
      },
      trend: {
        title: 'Usage trend'
      },
      attention: {
        title: 'Needs attention',
        view: 'View',
        recharge: 'Top up',
        renew: 'Renew',
        keyExpiredOne: 'Key “{name}” has expired',
        keyExpired: '{count} keys have expired',
        keyQuotaOne: 'Key “{name}” has used up its quota',
        keyQuota: '{count} keys have used up their quota',
        keyQuotaNearOne: 'Key “{name}” has used {percent}% of its quota',
        keyNearLimitOne: 'Key “{name}” has used {percent}% of its {limit} limit',
        keyNearLimit: '{count} keys are close to a limit',
        keyExpiringOne: 'Key “{name}” expires in {days} days',
        keyExpiring: '{count} keys expire within 7 days',
        balanceEmpty: 'Your balance is used up',
        balanceZero: 'Your balance is 0, so calls will be rejected',
        contactAdmin: 'Online top-up is not available here. Ask the administrator to top up your balance.',
        contactAdminWith: 'Online top-up is not available here. Ask the administrator to top up your balance: {contact}',
        balanceRunway: 'Balance {balance} lasts about {days} more days at the last 7 days’ pace',
        balanceBelowThreshold: 'Balance {balance} is below your alert threshold of {threshold}',
        subscriptionQuota: 'Subscription “{name}”: {percent}% of the quota used ({window})',
        subscriptionExpiring: 'Subscription “{name}” expires in {days} days',
        failuresToday: '{count} failed requests today'
      },
      subscriptions: {
        title: 'Subscription quota',
        manage: 'Manage',
        unlimited: 'No limit',
        daily: 'Today',
        weekly: 'This week',
        monthly: 'This month',
        fallbackName: 'Subscription',
        noExpiry: 'No expiry',
        expiresIn: 'Expires in {days} days'
      },
      gettingStarted: {
        title: 'Get started',
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
      }
    },
    billing: {
      title: 'Billing',
      tabs: {
        recharge: 'Top up',
        subscriptions: 'Subscriptions'
      }
    },
    usage: {
      title: 'Usage details',
      moreActions: 'More actions',
      moreFilters: 'More filters',
      clearFilters: 'Clear filters',
      costShare: 'Share of spend',
      stats: {
        requests: 'Requests',
        tokens: 'Tokens',
        actualCost: 'Billed',
        cacheHitRate: 'Cache hit rate',
        avgLatency: 'Avg latency',
        failures: 'Failed requests',
        viewFailures: 'View'
      },
      sections: {
        spend: 'Spend by model',
        records: 'Request details'
      },
      detail: {
        eyebrow: 'Request',
        request: 'Request info',
        requestId: 'Request ID',
        copy: 'Copy'
      },
      trend: {
        tokens: 'Tokens',
        requests: 'Requests',
        cost: 'Cost',
        empty: 'No data in this period'
      },
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
      notificationsOff: 'Balance alerts are not enabled on this site',
      rows: {
        overview: 'Account',
        overviewDesc: 'Sign-in identity, balance and concurrency',
        usernameDesc: 'Your email is shown when this is empty',
        passwordDesc: 'Used for email sign-in, at least 6 characters'
      }
    },
    summary: {
      redeemHistoryDesc: 'Redemptions and admin adjustments'
    },
    models: {
      title: 'Models',
      hero: {
        title: 'Every model, ',
        titleAccent: 'priced in the open',
        description: 'Copy a model ID and call it.'
      },
      allVendors: 'All',
      allBilling: 'All billing',
      searchHint: 'Search models or aliases (press / to focus)',
      vendorTabsLabel: 'Vendor',
      priceUnit: 'Prices in USD per 1M tokens',
      weekdaysOnly: 'weekdays only',
      openDetail: 'See every billed item',
      prices: {
        input: 'Input',
        output: 'Output',
        cacheWrite: 'Cache write',
        cacheRead: 'Cache read',
        imageInput: 'Image input',
        imageOutput: 'Image output',
        imageCacheRead: 'Image cache read',
        audioInput: 'Audio input',
        audioOutput: 'Audio output',
        perRequest: 'Per request',
        perImage: 'Per image',
        perSecond: 'Per second'
      },
      tags: {
        segments: '{count} segments',
        tiers: '{count} tiers',
        cache1h: '1h cache',
        image: 'Image',
        audio: 'Audio',
        search: 'Web search',
        maxReasoning: 'Max-reasoning surcharge',
        timePricing: 'Time-based'
      },
      detail: {
        standard: 'Standard',
        media: 'Image & audio',
        unitPrice: 'Unit price',
        tier: 'Tier',
        defaultTier: 'Other tiers',
        tools: 'Tools',
        search: 'Web search',
        searchOfficialPrice: 'Billed per search call',
        xPosts: 'X search · posts',
        xUsers: 'X search · profiles',
        perThousandCalls: '{price} / 1K calls',
        perThousandPosts: '{price} / 1K posts',
        perThousandUsers: '{price} / 1K profiles',
        claudeCodeSearch: 'Claude Code web search',
        claudeCodeSearchPrice: 'Input {input} · output {output} / 1M tokens, plus {search} / 1K searches',
        claudeCodeSearchNote: 'With Claude Code, search requests are run by the platform and billed separately',
        other: 'Other',
        maxReasoning: 'Max reasoning',
        maxReasoningRule: 'Whole request × {multiplier} when reasoning_effort=max',
        timePricing: 'Time-based multiplier',
        aliases: 'Aliases',
        ttl5m: '5m',
        ttl1h: '1h',
        unitPerMillion: 'USD / 1M tokens',
        unitPerRequest: 'USD / request',
        unitPerImage: 'USD / image',
        unitPerSecond: 'USD / second',
        noPricing: 'The catalog has no price for this model yet.'
      },
      segmentRange: 'Input tokens',
      segmentNote: 'Segmented models bill the whole request at the segment its input tokens (input + cache write + cache read) fall into.',
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
    },
    serviceStatus: {
      title: 'Service status',
      description: 'Measured from real requests on this site',
      range: { label: 'Time range', '90m': '90 min', '24h': '24 hours', '7d': '7 days', '30d': '30 days' },
      stats: { availability: 'Availability', ttft: 'Time to first token (median)', ttftP90: '90% within {value}', cache: 'Cache hit rate' },
      models: {
        title: 'Models',
        search: 'Search models',
        filter: { label: 'Filter by status', all: 'All', issues: 'Problems {count}', healthy: 'Normal {count}' },
        empty: 'No model data for this period yet',
        noMatch: 'No matching models',
        idle: '{count} more model(s) had no requests in this period',
        showIdle: 'Show',
        hideIdle: 'Hide',
        stripLabel: '{model} status by period'
      },
      columns: { availability: 'Availability', ttft: 'First token', cache: 'Cache hits' },
      legend: { healthy: 'Normal', warning: 'Unstable', critical: 'Problems', unknown: 'Too few requests' },
      slot: { detail: '{time}  Availability {availability} · First token {ttft} · Cache hits {cache}', fewRequests: ' (too few requests to rate)', noRequests: '{time}  No requests' },
      trend: { title: 'Overall trend', empty: 'No data for this period yet' },
      loadFailed: 'Could not load service status',
    }
  }
}
