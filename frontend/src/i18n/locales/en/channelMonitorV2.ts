/** Channel health (V2 passive monitor): admin settings panel copy and error category names. User-site service status copy lives in userUi.serviceStatus */
export default {
  channelMonitorV2: {
    errorCategories: {
      content_policy: 'Content policy', authentication: 'Authentication', context_limit: 'Context limit', invalid_request: 'Invalid request', model_unsupported: 'Unsupported model', quota_or_balance: 'Quota or balance', account_pool_unavailable: 'Account pool unavailable', rate_or_capacity: 'Rate or capacity', timeout: 'Timeout', transport_or_stream: 'Transport or stream', upstream_forbidden: 'Upstream forbidden', not_found: 'Not found', client_cancelled: 'Client cancelled', upstream_5xx: 'Upstream 5xx', internal: 'Internal', other: 'Other'
    },
    settings: {
      visibility: {
        title: 'What users see',
        description: 'What users can see on the Channel Status page.',
        hideThroughput: 'Hide throughput rates from users (RPM / TPM)',
        hideThroughputHint:
          'When on, the user Channel Status page and user APIs omit RPM and TPM so fleet volume cannot be reverse-estimated from rates × window. Admins still see full metrics. Error rates, latency, and cache rates remain visible.',
        hideUserRanking: 'Hide user ranking from users',
        hideUserRankingHint:
          'When on, the user Channel Status page hides the user ranking and the user API returns no ranking rows. Admins still see the ranking.',
      },
      save: 'Save',
      loading: 'Loading…',
      loadFailed: 'Failed to load config',
      saveSuccess: 'Config saved',
      saveFailed: 'Failed to save config',
      enableTitle: 'Enable aggregation',
      enableHint:
        'Turning this off only stops this aggregation; the switch for the whole channel health feature is under Settings › Switches.',
      refreshTitle: 'Aggregation interval',
      refreshHint: 'Affects matrix time granularity and refresh cadence',
      refreshAria: 'Aggregation interval',
      platformsTitle: 'Platforms and models',
      platformsHint:
        'Leave empty = show all real model names; when filled, only listed models get their own rows and the rest roll into “Other”',
      modelsPlaceholder: 'Empty = all real models; or list popular models (rest → Other)',
      badgeAllModels: 'All models',
      badgeOther: '+ Other',
      errorsTitle: 'Error categories and ignores',
      errorsHint:
        'Checked “ignore” categories are excluded from error rate and health score, but still appear greyed in the error breakdown. Unmatched errors roll into “Other”.',
      ignoredSummary: 'Ignored {ignored} categories · counted in error rate {counted} categories',
      healthTitle: 'Health thresholds',
      healthHint:
        'Controls user-facing color bands and overall score. Defaults are tolerant so small error rates or low cache do not immediately show as unhealthy.',
      fields: {
        minimumSample: 'Minimum samples',
        warningError: 'Error rate watch %',
        criticalError: 'Error rate critical %',
        targetTtft: 'TTFT target ms',
        warningTtft: 'TTFT watch ms',
        criticalTtft: 'TTFT critical ms',
        warningCache: 'Cache rate watch %',
        criticalCache: 'Cache rate critical %',
      },
      namedModelsEmpty: 'Platform model lists are empty: every real model name will be shown (not folded into “Other”).',
      namedModelsCount: 'Showing {count} named model dimensions; unlisted models fold into per-platform “Other”.',
      userContractTitle: 'User-facing display contract',
      userContract: {
        health: 'Health color weights: error rate 60% + first-token P50 20% + cache rate 20% (thresholds configurable above)',
        trend: 'Trend can switch between pulse matrix and line chart (error · cache · first token)',
        latency: 'Latency shows AVG · P50 · P90; absolute request / error counts are not shown',
        models: 'Empty model lists show real names and never dump everything into “Other”',
      },
    },
  },
}
