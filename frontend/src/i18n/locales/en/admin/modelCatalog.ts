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
      vendorNone: '(No vendor)',
      vendorCustom: 'Other (type it in)…',
      vendorCustomPlaceholder: 'Vendor tag, e.g. anthropic',
      modelIdPlaceholder: 'e.g. claude-sonnet-4-5',
      channels: 'Channels',
      morePrices: 'More prices',
      morePricesFilled: '{count} set',
      morePricesHint: 'Image and audio prices (and cache prices for non-token billing); leave empty for not configured.',
      units: {
        perMillion: '$ / 1M tokens',
        perCall: '$ / call',
        perImage: '$ / image',
        perSecond: '$ / second'
      },
      lookup: {
        idle: 'Enter a model ID to fill the vendor, billing mode and prices from the price file.',
        editIdle: 'You can refill the vendor, billing mode and prices from the price file.',
        loading: 'Looking up the price file…',
        applied: 'Vendor, billing mode and prices were filled from the price file; you can still change them.',
        found: 'The price file has this model.',
        apply: 'Use the price file prices',
        missing: 'The price file does not have this model; fill in the vendor and prices yourself.',
        error: 'The price lookup failed; fill in the vendor and prices yourself.',
        refill: 'Fill from price file'
      }
    },
    // Add / edit model page
    formPage: {
      backToList: 'Models',
      backToListAction: 'Back to models',
      loading: 'Loading the model…',
      notFound: 'Model #{id} was not found. It may have been deleted.',
      loadFailed: 'Failed to load the model: {message}',
      retry: 'Retry',
      saved: 'Model saved'
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
      inputPrice: 'Input price',
      outputPrice: 'Output price',
      perRequestPrice: 'Default price per request',
      perImagePrice: 'Default price per image (used when no tier matches)',
      perSecondPrice: 'Default price per second (used when no tier matches)',
      searchPricePerCall: 'Built-in search price per call (empty = built-in 0.01)',
      cacheWritePrice: 'Cache write · 5 min',
      cacheWrite1hPrice: 'Cache write · 1 hour',
      cacheReadPrice: 'Cache read',
      imageInputPrice: 'Image input price',
      imageOutputPrice: 'Image output price',
      imageCacheReadPrice: 'Image cache read price',
      audioInputPrice: 'Audio input price',
      audioOutputPrice: 'Audio output price'
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
      title: 'Channels serving this model',
      hint: 'Ticked channels serve requests for this model; leave priority empty to follow the channel.',
      selected: '{count} selected',
      search: 'Search channels by name or ID',
      boundOnly: 'Selected only',
      loading: 'Loading channels…',
      loadFailed: 'Failed to load channels',
      retry: 'Retry',
      noResults: 'No matching channels',
      noChannels: 'No channels yet. Add one on the Channels page first.',
      inactive: 'Disabled',
      priority: 'Priority',
      priorityFollow: 'Follow channel'
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
