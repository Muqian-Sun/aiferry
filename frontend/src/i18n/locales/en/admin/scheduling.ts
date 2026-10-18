export default {
  scheduling: {
    loadFailed: 'Scheduling failed to load',
    refreshFailed: 'The last refresh failed; showing the previous data',
    window: { label: 'Time range', '15m': '15 min', '1h': '1 hour', '24h': '24 hours' },
    refresh: {
      auto: 'Refreshes every {seconds}s · {time}',
      paused: 'Paused while the page is in the background'
    },
    warnings: {
      load_unavailable: 'Concurrency data is unavailable: order falls back to priority and least recently used; in-flight and queued show "—".',
      sticky_unavailable: 'Sticky session counts are unavailable and are hidden below.'
    },
    stats: {
      models: 'Models',
      channels: 'Schedulable channels',
      inUse: 'In flight',
      waiting: 'Queued',
      requests: 'Requests',
      unrouted: 'No channel selected'
    },
    models: {
      title: 'Scheduling order by model',
      hint: '#1 is where the next new request lands; requests with a sticky session return to their channel first. Queued = can serve but concurrency is full; dashed channels will not be picked right now.',
      search: 'Search models',
      empty: 'No listed model has a channel yet',
      noMatch: 'No matching models',
      noLane: 'No channel can serve this model'
    },
    protocols: {
      label: 'Inbound protocol',
      only: '{protocol} only',
      anthropic: 'Messages',
      chat_completions: 'Chat',
      responses: 'Responses',
      gemini: 'Gemini'
    },
    traffic: {
      requests: '{count} requests',
      failures: '{count} failed',
      unrouted: '{count} with no channel'
    },
    item: {
      direct: 'direct',
      queue: 'Queued',
      blocked: 'Not picked',
      load: 'Concurrency {inUse}/{max}',
      waiting: 'Queued {count}',
      loadUnknown: 'Concurrency —',
      requests: '{count} requests',
      share: '{count} requests ({percent}%)',
      sticky: '{count} sticky sessions',
      recoversIn: '{reason}, recovers in {duration}'
    },
    tabs: { label: 'View', realtime: 'Live', traces: 'Request traces' },
    trace: {
      title: 'Recent requests',
      loadFailed: 'Request traces failed to load',
      empty: 'No request traces in this range',
      total: '{count} total',
      prev: 'Previous',
      next: 'Next',
      noChannel: 'No channel',
      stickyHit: 'sticky session',
      user: 'User {id}',
      key: 'Key {id}',
      filters: { model: 'Model', protocol: 'Protocol', outcome: 'Outcome' },
      outcomeFilters: { all: 'All', switched: 'Switched', failed: 'Failed', no_channel: 'No channel' },
      outcomes: { success: 'Success', failed: 'Failed', no_channel: 'No channel', canceled: 'Canceled' },
      fields: { outcome: 'Outcome', duration: 'Duration', finalChannel: 'Final channel', switches: 'Channel switches', requestId: 'Request ID' },
      pickTitle: 'Pick {n} · +{at}',
      pickResult: 'picked {channel} ({layer})',
      layers: { sticky: 'sticky session', load: 'by load', legacy: 'by priority', queue: 'queued for a slot' },
      errors: { no_available_accounts: 'No channel available', no_compact_accounts: 'No channel supports compact', canceled: 'Request canceled', error: 'Channel selection failed', no_account: 'No channel picked' },
      columns: { channel: 'Channel', decision: 'Decision', load: 'Load' },
      decisions: {
        excluded: 'Excluded (failed earlier)',
        skipped: 'Skipped',
        eligible: 'Eligible, not reached',
        full: 'Concurrency full',
        slot_busy: 'Slot not acquired',
        session_full: 'Session limit full',
        picked: 'Picked',
        queued: 'Picked, queued'
      },
      sticky: {
        hit: 'Sticky session: back to {channel}',
        wait: 'Sticky session: back to {channel}, queued for a slot',
        miss: 'Sticky session: {channel} unavailable this time ({reason})'
      },
      attemptsTitle: 'Failed upstream attempts'
    },
    reasons: {
      missing: 'Channel missing',
      disabled: 'Disabled',
      error: 'Error',
      unschedulable: 'Scheduling off',
      expired: 'Expired',
      overloaded: 'Overload cooldown',
      rate_limited: 'Rate limited',
      temp_unschedulable: 'Temporarily paused',
      model_rate_limited: 'Rate limited for this model',
      model_transient_blocked: 'Short circuit-break for this model',
      grok_model_blocked: 'Model cooling down',
      proxy_quarantined: 'Proxy quarantined',
      profit_threshold: 'Below minimum margin',
      model_not_supported: 'Model not supported',
      shadow_parent_unhealthy: 'Parent account unavailable',
      compact_unsupported: 'Compact not supported',
      protocol_unsupported: 'Cannot serve this protocol',
      rpm_limited: 'RPM full',
      cleared: 'Binding cleared',
      not_in_pool: 'Not among schedulable channels',
      excluded: 'Failed earlier in this request',
      session_full: 'Session limit full',
      slot_busy: 'Concurrency and queue full',
      other: 'Other ({reason})'
    }
  }
}
