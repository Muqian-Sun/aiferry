export default {
  modelCatalog: {
    description: 'Model list prices and aliases; revenue and cost are both the list price times a multiplier.',
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
      notFound: 'This model does not exist or has been deleted.',
      loadFailed: 'Failed to load the model: {message}',
      retry: 'Retry',
      saved: 'Model saved'
    },
    edit: 'Edit model',
    empty: 'The catalog is empty. Import from the price file or create an entry.',
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
        // Service tier: the priority tier is the Fast tier (same name as the user site's service tier), not the scheduling priority
        inputPriority: 'Fast tier · input',
        outputPriority: 'Fast tier · output',
        cacheWritePriority: 'Fast tier · cache write',
        cacheReadPriority: 'Fast tier · cache read',
        perRequest: 'Per request',
        per: {
          per_request: '{price} / request',
          image: '{price} / image',
          video: '{price} / second'
        },
        searchPerCall: 'Built-in search',
        listPrice: 'List price',
        fast: 'Fast tier multiplier',
        flex: 'Flex tier multiplier',
        maxReasoning: 'Max reasoning multiplier'
      },
      segments: 'Token segments',
      segmentsHint: 'A request is billed entirely at the segment its input tokens (input + cache write + cache read) fall into; the first segment is the list price above. In $ / 1M tokens',
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
      bind: 'Bind channels',
      priority: 'Priority {value}',
      notSchedulable: 'Not schedulable',
      channelsHint: 'Whether each channel can be scheduled right now; use Diagnose to check each inbound protocol.',
      channelsFallback: 'Could not load channel status; showing the bound channels only.'
    },
    diagnose: 'Diagnose',
    diagnosis: {
      title: 'Channel diagnosis · {model}',
      empty: 'No channels are bound to this model.',
      followAccount: 'Follows channel',
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
    seed: 'Import from price file',
    seeding: 'Importing…',
    seedDone: 'Import finished: {inserted} added, {refreshed} updated, {skipped} skipped (edited by hand)',
    seedPartial: '{summary}; {failed} rows failed to write: {errors}',
    deleteTitle: 'Delete catalog entry',
    deleteConfirm: 'Aliases, intervals, and time pricing will be deleted with it. Continue?',
    fullReplaceHint: 'Save replaces the whole entry. Fields not shown here (time pricing, Fast tier prices, tier multipliers, per-request tiers) are written back unchanged; token segments and image / video tiers are edited above.',
    listedRequiresPrice: 'A listed model must have a price before users can see and call it.',
    noResources: 'No channels',
    fields: {
      modelId: 'Model id',
      displayName: 'Display name',
      vendor: 'Vendor',
      billingMode: 'Billing mode',
      status: 'Listing status',
      resources: 'Serving channels',
      inputPrice: 'Input price',
      outputPrice: 'Output price',
      perRequestPrice: 'List price per request',
      perImagePrice: 'List price per image (used when no tier matches)',
      perSecondPrice: 'List price per second (used when no tier matches)',
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
    segments: {
      title: 'Token segments',
      hint: 'A request is billed entirely at the segment its input tokens (input + cache write + cache read) fall into. The prices above are the first segment; any price left empty in a segment falls back to the first segment.',
      empty: 'No segments: every request is billed at the prices above.',
      add: 'Add segment',
      remove: 'Remove',
      first: 'Segment 1 · {range}: the prices above',
      firstUntitled: 'Segment 1: the prices above',
      segment: 'Segment {index} · {range}',
      segmentUntitled: 'Segment {index}',
      above: 'Above (tokens)',
      abovePlaceholder: 'e.g. 272000',
      errors: {
        required: 'Enter the token count this segment starts above.',
        integer: 'The token count must be a positive integer.',
        notAscending: 'The token count must be larger than in the previous segment.',
        noPrice: 'Enter at least one price for this segment.'
      }
    },
    tiers: {
      title: 'Tier prices',
      hint: {
        image: 'Per image by output size; a listed entry must have the list price per image.',
        video: 'Per second by resolution; a listed entry must have the list price per second.'
      },
      tier: 'Tier',
      price: 'Price ($)',
      add: 'Add tier',
      remove: 'Remove',
      empty: 'No tiers; the list price applies.'
    },
    bindings: {
      title: 'Channels serving this model',
      hint: 'Ticked channels serve requests for this model; leave priority empty to follow the channel.',
      selected: '{count} selected',
      search: 'Search channels by name',
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
