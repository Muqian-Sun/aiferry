export default {
  modelCatalog: {
    title: 'Model catalog',
    description: 'Canonical model prices and aliases. Billing reads from here.',
    search: 'Search by model id, display name, or vendor',
    create: 'New model',
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
    fullReplaceHint: 'Save replaces the whole entry. Token context intervals and time pricing are written back unchanged; image / video tiers are edited above.',
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
      searchPricePerCall: 'Built-in search price per call ($, empty = built-in 0.01)'
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
