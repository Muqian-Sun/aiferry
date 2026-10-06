export default {
  pricing: {
    views: {
      label: 'View',
      model: 'By model',
      channel: 'By channel'
    },
    searchModels: 'Search models or channels',
    searchChannels: 'Search channels or models',
    filters: {
      vendor: 'Vendor',
      status: 'Listing',
      focus: 'Show only',
      problems: 'With problems',
      unsaved: 'Unsaved'
    },
    basis: '$ / 1M tokens · margin uses the sale price (items without one use official × {rate}); the profit gate skips bindings below {margin}',
    basisGateOff: '$ / 1M tokens · margin uses the sale price (items without one use official × {rate}) · profit gate off',
    unsavedBlocks: '{count} unsaved',
    empty: 'No models or channels match',
    loadFailed: 'Failed to load prices',
    reload: 'Reload',
    columns: {
      channel: 'Channel',
      model: 'Model',
      upstreamModel: 'Upstream model',
      input_price: 'Input',
      output_price: 'Output',
      cache_read_price: 'Cache read',
      cache_write_price: 'Cache write 5m',
      cache_write_1h_price: 'Cache write 1h',
      search_price_per_call: 'Web search',
      x_post_price: 'X search · posts',
      x_user_price: 'X search · profiles',
      segments: 'Segments',
      margin: 'Margin',
      status: 'Status',
      actions: 'Actions'
    },
    official: 'Official',
    webSearchDelegate: 'Web search rate',
    webSearchDelegateHint: 'When Claude Code runs on a third-party model, its standalone search request runs on this model: its official price is the web search rate (tokens × user rate + per-search fee at cost) and its channels run the search; not listed to users',
    search: {
      toggle: 'Web search',
      title: 'Web search',
      officialNote: 'Charged at the list price, not multiplied by the user rate; empty = no search charge',
      upstreamNote: 'Required where the official price is set; empty items cost the official price',
      units: {
        search_price_per_call: '$ / 1K searches',
        x_post_price: '$ / 1K posts',
        x_user_price: '$ / 1K profiles'
      },
      defaultPlaceholder: 'Public {price}',
      officialRef: 'Official {price}',
      officialNone: 'Not charged',
      optional: 'Optional'
    },
    catalogName: 'Catalog ID',
    sameName: 'Same name',
    upstreamModelHint: 'The model name this channel uses for this model; blank = same as the catalog model ID. Users can only request catalog model IDs, and the name is converted exactly once when forwarding.',
    officialRef: 'Official {price}',
    officialUnset: 'No official',
    officialReadOnly: 'Official prices are for reference here; switch to “By model” to edit them.',
    required: 'Required',
    newRow: 'New',
    noChannels: 'No channel serves this model yet. Use “Add channel” and fill in its upstream price.',
    noModels: 'This channel serves no models yet. Use “Add model” and fill in the upstream price.',
    noVendor: 'No vendor',
    status: {
      listed: 'Listed',
      unlisted: 'Unlisted'
    },
    channelCount: '{count} channels',
    modelCount: '{count} models',
    priority: 'Priority {priority}',
    segmentsNone: 'None',
    segmentsCount: '{count} segments',
    segmentAbove: 'Above',
    segmentInherit: 'Same as 1st',
    segmentAdd: 'Add segment',
    segmentHint: 'The segment is picked by the request’s input tokens (input + cache write + cache read) and applies to the whole request. The row above is segment 1; blank prices fall back to segment 1.',
    remove: 'Remove',
    addChannel: 'Add channel',
    addModel: 'Add model',
    searchModel: 'Search models',
    noMatch: 'Nothing to add',
    copyFromSibling: 'Copy prices from same upstream',
    fillFromPriceFile: 'Fill official price from price file',
    sale: {
      label: 'Sale price',
      hint: 'Empty = official × {ratio}',
      scope: 'Charged = sale price × user discount',
      cell: 'Sale price · {item}',
      clear: 'Clear all',
      segmentAbove: 'Above {tokens} tokens',
      invalid: 'Sale price: some prices are not valid numbers',
      peakInvalid: 'Sale peak hours: a period or date is not filled in correctly'
    },
    saleFill: {
      trigger: 'Fill sale prices by ratio',
      hint: 'Fills empty sale prices with official price × ratio (segments included); filled cells stay as they are. Save the block afterwards.',
      ratio: 'Ratio'
    },
    discountFill: {
      trigger: 'Fill by discount',
      hint: 'Fills empty upstream prices with official price × discount (segments included); filled cells stay. Enter 0 for a free upstream. Remember to save this block.',
      prefix: 'Official ×',
      ratio: 'Discount',
      apply: 'Fill'
    },
    priceFileMissing: 'The price file has no per-token price for this model',
    marginAfterSave: 'After save',
    gateSkips: 'Profit gate skips',
    peak: {
      toggle: 'Peak hours',
      none: 'No peak hours',
      summary: 'Peak ×{multiplier}',
      official: {
        title: 'Official peak hours',
        note: "The vendor's peak hours. Unless sale peak hours are set separately, users are charged the whole request × multiplier in these periods.",
        noneHint: 'One price all day'
      },
      sale: {
        title: 'Sale peak hours',
        note: 'Users are charged the whole request × multiplier in these periods. Follows the official peak hours by default.',
        noneHint: 'One price all day (no surcharge even at official peak)',
        follow: 'Follow official peak hours ({summary})',
        followShort: 'Official · {summary}',
        custom: 'Set separately',
        noneShort: 'One price all day'
      },
      upstream: {
        title: 'Upstream peak hours',
        note: 'The upstream charges the whole request × multiplier in these periods. Affects channel cost and the profit gate only, not what users pay.',
        noneHint: 'The upstream charges one price all day'
      },
      excludeDates: 'Holidays (off-peak all day)',
      excludeDatesPlaceholder: '2026-10-01 2026-10-02 …',
      excludeDatesInvalid: 'Dates must be YYYY-MM-DD',
      excludeDatesCount: '{count} days',
      useOfficial: 'Use official peak hours',
      clear: 'Remove peak hours',
      timezone: 'Time zone',
      zones: {
        beijing: 'Beijing time',
        utc: 'UTC',
        pacific: 'US Pacific'
      },
      weekdaysOnly: 'Weekdays only (weekends at the normal price)',
      start: 'Start',
      end: 'End',
      multiplier: 'Multiplier',
      add: 'Add a period',
      endHint: 'End 00:00 means end of day',
      margin: 'Peak {margin}',
      gateSkips: 'Profit gate skips at peak',
      errors: {
        time: 'Time missing or invalid',
        order: 'Start must be before end',
        multiplier: 'Multiplier must be above 0 with at most two decimals',
        overlap: 'Overlaps another period'
      }
    },
    channelState: {
      ok: 'Schedulable',
      paused: 'Not schedulable now',
      error: 'Error',
      disabled: 'Disabled',
      missing: 'Channel missing'
    },
    changes: '{count} changes',
    undo: 'Undo',
    saving: 'Saving…',
    listSeparator: ', ',
    issueSeparator: '; ',
    issues: {
      missing: '{fields} missing',
      invalid: '{fields} invalid (non-negative numbers only)',
      segment: 'segment {index}: {error}',
      upstreamModel: 'upstream model must be a single name without * or spaces',
      peak: 'Peak period {index}: {error}',
      peakDates: 'Peak-hours holidays contain an invalid date'
    },
    discardTitle: 'Discard unsaved changes?',
    discardMessage: '{count} blocks have unsaved changes; continuing discards them.',
    discard: 'Discard'
  }
}
