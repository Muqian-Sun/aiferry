export default {
  modelCatalog: {
    description: 'Official model prices and aliases.',
    search: 'Search by model id, display name, vendor or alias',
    create: 'New model',
    noMatch: 'No entries match',
    noneListed: 'No models are listed yet',
    showAll: 'Show all models',
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
      model: 'Model',
      price: 'List price',
      channels: 'Serving channels',
      status: 'Status',
      perMillion: 'input / output · per 1M tokens',
      perUnit: {
        per_request: 'per request',
        image: 'per image',
        video: 'per second'
      },
      tiers: '{count} tiers',
      segments: '{count} segments',
      unpriced: 'No price'
    },
    bulk: {
      list: 'List',
      unlist: 'Unlist',
      selectEntry: 'Select {model}',
      nothingToDo: 'The selected entries already have that status',
      listedDone: 'Listed {count} models',
      unlistedDone: 'Unlisted {count} models',
      partial: '{done} succeeded, {failed} failed (failures stay selected): {errors}'
    },
    editor: {
      vendorHint: 'Use the lowercase vendor tag (anthropic / openai / gemini / xai…); the user site matches vendor tabs and icons on it.',
      vendorNone: '(No vendor)',
      vendorCustom: 'Other (type it in)…',
      vendorCustomPlaceholder: 'Vendor tag, e.g. anthropic',
      modelIdPlaceholder: 'e.g. claude-sonnet-4-5',
      units: {
        perMillion: '$ / 1M tokens',
        perCall: '$ / call'
      },
      lookup: {
        loading: 'Looking up the price file…'
      }
    },
    edit: 'Edit model',
    empty: 'The catalog is empty. Import from the price file or create an entry.',
    // Create / edit model dialogs (2026-10-03): create has two steps (model → pricing & channels), edit has one
    dialog: {
      steps: {
        model: 'Model',
        pricing: 'Pricing & channels'
      },
      next: 'Next: pricing & channels',
      done: 'Done',
      lookup: {
        idle: 'Enter the model ID to fill the official price from the price file.',
        found: 'Found in the price file: {prices} (per million tokens). It becomes the official price; you can change it in the next step.',
        missing: 'The price file has no per-token price for this model; fill the official price in the next step.',
        error: 'Price lookup failed; fill the official price in the next step.'
      },
      createFailed: 'Failed to create the model',
      exists: 'This model is already in the catalog: edit or list it from the list.',
      saveFailed: 'Failed to save the model',
      pricingHint: 'This is the model\'s block on the pricing page: edit the official price here; adding a channel makes it serve this model. Save the block when done.',
      pricingLoading: 'Loading prices…',
      pricingMissing: 'This model is not on the pricing page (only per-token models are listed).',
      listing: 'Listed',
      listingReady: 'Once on, users can see and call this model.',
      listedHint: 'Listed: users can see and call this model.',
      listingBlocked: {
        unsaved: 'Save the block above before listing.',
        price: 'Fill the official input and output prices before listing.',
        channel: 'No channel serves this model yet, so it cannot be listed.'
      },
      listingFailed: 'Failed to change the listing status',
      listingRule: 'Listing requires the official price and at least one serving channel.',
      unsavedPricing: 'The pricing block has unsaved changes: save it or undo them first.',
      pricingElsewhere: 'Official prices, segments and serving channels are edited on the pricing page.',
      openPricing: 'Open pricing →'
    },
    // Model detail drawer (A5)
    drawer: {
      tabs: {
        overview: 'Overview',
        channels: 'Channels'
      },
      unpricedBanner: 'Listed without a price: users cannot see this model.',
      unboundBanner: 'Listed without channels: user calls to this model will fail.',
      listedDone: 'Listed {model}',
      unlistedDone: 'Unlisted {model}',
      aliases: 'Aliases',
      notes: 'Notes',
      updatedAt: 'Updated',
      prices: 'List price',
      perMillionHint: 'Token list prices, in $ / 1M tokens',
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
        perRequest: 'Per request',
        per: {
          per_request: '{price} / request',
          image: '{price} / image',
          video: '{price} / second'
        },
        searchPerCall: 'Built-in search',
        listPrice: 'List price',
        maxReasoning: 'Max reasoning multiplier'
      },
      segments: 'Token segments',
      segmentsHint: 'A request is billed entirely at the segment its input tokens (input + cache write + cache read) fall into; the first segment is the list price above. Blank cache prices scale with the segment input price. In $ / 1M tokens',
      segmentColumns: {
        range: 'Input tokens',
        input: 'Input',
        output: 'Output',
        cacheWrite: 'Cache write 5m',
        cacheWrite1h: 'Cache write 1h',
        cacheRead: 'Cache read'
      },
      tiers: 'Tiers',
      mediaTiersHint: 'A matching tier uses its own price; otherwise the list price above applies.',
      timePricing: 'Time-of-day pricing',
      timezone: 'Time zone {timezone}',
      weekdaysOnly: 'Weekdays only',
      notSchedulable: 'Not schedulable',
      channelsHint: 'Whether each channel can be scheduled right now; use Diagnose to check each inbound protocol.',
      channelsFallback: 'Could not load channel status; switch tabs to retry.'
    },
    diagnose: 'Diagnose',
    diagnosis: {
      title: 'Channel diagnosis · {model}',
      empty: 'No channels are bound to this model.',
      columns: {
        account: 'Channel',
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
    seed: 'Import from price file',
    seeding: 'Importing…',
    seedDone: 'Import finished: {inserted} added, {refreshed} updated, {skipped} skipped (edited by hand)',
    seedPartial: '{summary}; {failed} rows failed to write: {errors}',
    deleteTitle: 'Delete catalog entry',
    deleteConfirm: 'Aliases, intervals, and time pricing will be deleted with it. Continue?',
    noResources: 'No channels',
    fields: {
      modelId: 'Model id',
      displayName: 'Display name',
      vendor: 'Vendor',
      billingMode: 'Billing mode',
      status: 'Listing status',
      resources: 'Serving channels',
      searchPricePerCall: 'Built-in search price per call (empty = built-in 0.01)'
    },
    segments: {
      abovePlaceholder: 'e.g. 272000',
      errors: {
        required: 'Enter the token count this segment starts above.',
        integer: 'The token count must be a positive integer.',
        notAscending: 'The token count must be larger than in the previous segment.',
        noPrice: 'Enter at least one price for this segment.'
      }
    },
    bindings: {
      title: 'Channels serving this model'
    },
    status: {
      listed: 'Listed',
      unlisted: 'Not listed'
    },
    billingModes: {
      token: 'Per token',
      per_request: 'Per request',
      image: 'Per image',
      video: 'Per video'
    }
  }
}
