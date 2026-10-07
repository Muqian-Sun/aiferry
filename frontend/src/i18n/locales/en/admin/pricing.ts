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
    basis: '$ / 1M tokens · minimum margin {margin}',
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
    costGroup: 'Cost',
    webSearchDelegate: 'Web search rate',
    webSearchDelegateHint: 'Runs web search for Claude Code on third-party models; not listed to users',
    search: {
      toggle: 'Web search',
      title: 'Web search',
      units: {
        search_price_per_call: '$ / 1K searches',
        x_post_price: '$ / 1K posts',
        x_user_price: '$ / 1K profiles'
      },
      defaultPlaceholder: 'Public {price}',
      officialRef: 'Official {price}',
      officialNone: 'Not charged',
      free: 'Free'
    },
    sameName: 'Same name',
    upstreamModelHint: 'Blank = same as the catalog ID',
    officialRef: 'Official {price}',
    officialUnset: 'No official',
    required: 'Required',
    newRow: 'New',
    noChannels: 'No channels yet',
    noModels: 'No models yet',
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
    remove: 'Remove',
    addChannel: 'Add channel',
    addModel: 'Add model',
    searchModel: 'Search models',
    noMatch: 'Nothing to add',
    copyFromSibling: 'Copy prices from same upstream',
    fillFromPriceFile: 'Fill official price from price file',
    sale: {
      label: 'Sale price',
      cell: 'Sale price · {item}',
      clear: 'Clear all',
      segmentAbove: 'Above {tokens} tokens',
      invalid: 'Sale price: some prices are not valid numbers',
      peakInvalid: 'Sale peak hours: a period or date is not filled in correctly',
      reasoningInvalid: 'Sale max-reasoning multiplier must be greater than 0'
    },
    saleFill: {
      trigger: 'Fill sale prices by ratio',
      ratio: 'Ratio'
    },
    discountFill: {
      trigger: 'Fill cost by discount',
      prefix: 'Official ×',
      ratio: 'Discount',
      apply: 'Fill'
    },
    priceFileMissing: 'The price file has no per-token price for this model',
    marginAfterSave: 'After save',
    gateSkips: 'Profit gate skips',
    reasoning: {
      field: 'Whole request × at effort max',
      summary: 'Max reasoning ×{multiplier}',
      none: 'No max-reasoning markup',
      followShort: 'Max reasoning: follow official',
      follow: 'Follow official ×{multiplier}',
      followNone: 'Follow official (no markup)',
      margin: 'Max reasoning {margin}',
      gateSkips: 'Profit gate skips it at max reasoning',
      official: { title: 'Official max reasoning' },
      sale: { title: 'Sale max reasoning' },
      upstream: { title: 'Upstream max reasoning' }
    },
    peak: {
      toggle: 'Peak hours',
      none: 'No peak hours',
      summary: 'Peak ×{multiplier}',
      official: {
        title: 'Official peak hours',
        noneHint: 'One price all day'
      },
      sale: {
        title: 'Sale peak hours',
        noneHint: 'One price all day',
        follow: 'Follow official peak hours ({summary})',
        followShort: 'Follow official',
        custom: 'Set separately',
        noneShort: 'One price all day'
      },
      upstream: {
        title: 'Upstream peak hours',
        noneHint: 'One price all day'
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
      peakDates: 'Peak-hours holidays contain an invalid date',
      reasoning: 'Max-reasoning multiplier must be greater than 0'
    },
    discardTitle: 'Discard unsaved changes?',
    discardMessage: '{count} blocks have unsaved changes; continuing discards them.',
    discard: 'Discard'
  }
}
