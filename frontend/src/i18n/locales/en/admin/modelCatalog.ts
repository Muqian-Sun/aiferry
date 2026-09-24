export default {
  modelCatalog: {
    description: 'Canonical model prices and aliases. Billing reads from here.',
    search: 'Search by model id, display name, vendor or alias',
    create: 'New model',
    noMatch: 'No entries match',
    summaryStats: {
      total: 'Models',
      listed: 'Listed',
      listedWithoutResources: 'Listed without channels',
      showThem: 'Filter'
    },
    filtered: '{count} after filters',
    aliasCount: '{count} aliases',
    filters: {
      noVendor: '(no vendor)',
      withResources: 'With channels',
      withoutResources: 'Without channels'
    },
    columns: {
      price: 'List price',
      perMillion: '$ / 1M tokens',
      perUnit: {
        per_request: 'per request',
        image: 'per image',
        video: 'per second'
      },
      tiers: '{count} tiers'
    },
    bulk: {
      list: 'List',
      unlist: 'Unlist',
      nothingToDo: 'The selected entries already have that status',
      listedDone: 'Listed {count} models',
      unlistedDone: 'Unlisted {count} models',
      partial: '{done} succeeded, {failed} failed (failures stay selected): {errors}'
    },
    editor: {
      basics: 'Basics',
      pricing: 'Billing & list price',
      vendorHint: 'Use the lowercase vendor tag (anthropic / openai / gemini / xai…); the user site matches vendor tabs and icons on it.',
      perMillion: '= ${price} per 1M tokens',
      morePrices: 'More prices',
      morePricesFilled: '{count} set',
      morePricesHint: 'Per-token prices for cache, image and audio; leave empty for not configured.'
    },
    edit: 'Edit model',
    empty: 'The catalog is empty. Seed it or create an entry.',
    // Model detail drawer (A5)
    drawer: {
      eyebrow: 'Model #{id}',
      tabs: {
        overview: 'Overview',
        channels: 'Channels'
      },
      unpricedBanner: 'Listed without a price: users cannot see this model.',
      unboundBanner: 'Listed without channels: user calls to this model will fail.',
      listedDone: 'Listed {model}',
      unlistedDone: 'Unlisted {model}',
      protocols: 'Protocols',
      aliases: 'Aliases',
      notes: 'Notes',
      updatedAt: 'Updated',
      prices: 'Prices',
      perMillionHint: 'Token prices in $ / 1M tokens',
      price: {
        input: 'Input',
        output: 'Output',
        cacheWrite: 'Cache write · 5 min',
        cacheWrite1h: 'Cache write · 1 hour',
        cacheRead: 'Cache read',
        imageInput: 'Image input',
        imageOutput: 'Image output',
        imageCacheRead: 'Image cache read',
        audioInput: 'Audio input',
        audioOutput: 'Audio output',
        inputPriority: 'Priority · input',
        outputPriority: 'Priority · output',
        cacheWritePriority: 'Priority · cache write',
        cacheReadPriority: 'Priority · cache read',
        perRequest: 'Per request',
        perCall: '{price} / call',
        searchPerCall: 'Built-in search',
        longContext: 'Long context {op} {threshold}',
        defaultPrice: 'Default price',
        fast: 'Fast multiplier',
        flex: 'Flex multiplier',
        maxReasoning: 'Max reasoning multiplier'
      },
      tiers: 'Tiers',
      tokenTier: '{min} – {max} tokens',
      tokenTierOpen: '{min}+ tokens',
      timePricing: 'Time-of-day pricing',
      timezone: 'Time zone {timezone}',
      weekdaysOnly: 'Weekdays only',
      bind: 'Bind channels',
      priority: 'Priority {value}',
      notSchedulable: 'Not schedulable',
      channelsFallback: 'Could not load channel status; showing the bound channels only.'
    },
    diagnose: 'Diagnose',
    diagnosis: {
      title: 'Channel diagnosis · {model}',
      empty: 'No channels are bound to this model.',
      followAccount: 'Follows account',
      columns: {
        account: 'Channel',
        priority: 'Priority',
        schedulable: 'Schedulable'
      },
      inbound: {
        anthropic: 'Messages',
        chat_completions: 'Chat',
        responses: 'Responses',
        gemini: 'Gemini'
      },
      reasons: {
        disabled: 'Disabled',
        unschedulable: 'Unschedulable',
        expired: 'Expired',
        overloaded: 'Overloaded (cooling down)',
        rate_limited: 'Rate limited',
        temp_unschedulable: 'Temporarily unschedulable',
        quota_exceeded: 'Quota exhausted'
      }
    },
    seed: 'Seed from pricing file',
    seeding: 'Seeding…',
    seedDone: 'Seed finished: inserted {inserted}, refreshed {refreshed}, skipped admin-edited {skipped}',
    seedPartial: '{summary}; {failed} rows failed to write: {errors}',
    deleteTitle: 'Delete catalog entry',
    deleteConfirm: 'Aliases, intervals, and time pricing will be deleted with it. Continue?',
    fullReplaceHint: 'Save replaces the whole entry. Fields not shown here (token context intervals, time pricing, priority prices, long-context and multipliers) are written back unchanged; image / video tiers are edited above.',
    listedRequiresPrice: 'A listed model must have a price before users can see and call it.',
    noResources: 'No channels',
    fields: {
      modelId: 'Model id',
      displayName: 'Display name',
      vendor: 'Vendor',
      billingMode: 'Billing mode',
      status: 'Listing status',
      managedBy: 'Managed by',
      resources: 'Channels',
      inputPrice: 'Input price ($/token)',
      outputPrice: 'Output price ($/token)',
      perRequestPrice: 'Default price per request ($)',
      perImagePrice: 'Default price per image ($, used when no tier matches)',
      perSecondPrice: 'Default price per second ($, used when no tier matches)',
      searchPricePerCall: 'Built-in search price per call ($, empty = built-in 0.01)',
      cacheWritePrice: 'Cache write price · 5 min ($/token)',
      cacheWrite1hPrice: 'Cache write price · 1 hour ($/token)',
      cacheReadPrice: 'Cache read price ($/token)',
      imageInputPrice: 'Image input price ($/token)',
      imageOutputPrice: 'Image output price ($/token)',
      imageCacheReadPrice: 'Image cache read price ($/token)',
      audioInputPrice: 'Audio input price ($/token)',
      audioOutputPrice: 'Audio output price ($/token)'
    },
    tiers: {
      title: 'Tier prices',
      hint: {
        image: 'Per image by output size; a listed entry must have the default price.',
        video: 'Per second by resolution; a listed entry must have the default price.'
      },
      tier: 'Tier',
      price: 'Price ($)',
      add: 'Add tier',
      remove: 'Remove',
      empty: 'No tiers; the default price applies.'
    },
    bindings: {
      title: 'Bound channels',
      hint: 'Requests for a listed model are served by these channels; leave priority empty to follow the channel.',
      search: 'Search channels by name',
      noResults: 'No matching channels',
      add: 'Add',
      priority: 'Priority',
      remove: 'Remove',
      empty: 'No channels bound yet'
    },
    status: {
      listed: 'Listed',
      unlisted: 'Unlisted'
    },
    billingModes: {
      token: 'Per token',
      per_request: 'Per request',
      image: 'Per image',
      video: 'Per video'
    },
    managedBy: {
      seed: 'Seed',
      admin: 'Admin'
    }
  }
}
