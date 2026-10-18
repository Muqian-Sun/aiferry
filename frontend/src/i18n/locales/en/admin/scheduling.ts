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
      dataThrough: 'Counted through {time}',
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
    reasons: {
      missing: 'Channel missing',
      disabled: 'Disabled',
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
      other: 'Other ({reason})'
    }
  }
}
