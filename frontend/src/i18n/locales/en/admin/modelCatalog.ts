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
      withResources: 'With resources',
      withoutResources: 'Without resources'
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
    diagnose: 'Diagnose',
    diagnosis: {
      title: 'Resource diagnosis · {model}',
      empty: 'No resources are bound to this model.',
      followAccount: 'Follows account',
      columns: {
        account: 'Resource',
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
    noResources: 'No resources',
    fields: {
      modelId: 'Model id',
      displayName: 'Display name',
      vendor: 'Vendor',
      billingMode: 'Billing mode',
      status: 'Listing status',
      managedBy: 'Managed by',
      resources: 'Resources',
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
      title: 'Bound resources',
      hint: 'Requests for a listed model are served by these resources; leave priority empty to follow the resource.',
      search: 'Search resources by name',
      noResults: 'No matching resources',
      add: 'Add',
      priority: 'Priority',
      remove: 'Remove',
      empty: 'No resources bound yet'
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
