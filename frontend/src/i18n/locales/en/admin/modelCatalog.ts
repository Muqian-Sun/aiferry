export default {
  modelCatalog: {
    title: 'Model catalog',
    description: 'Canonical model prices and aliases. Billing reads from here.',
    search: 'Search by model id, display name, or vendor',
    create: 'New model',
    edit: 'Edit model',
    empty: 'The catalog is empty. Seed it or create an entry.',
    seed: 'Seed from pricing file',
    seeding: 'Seeding…',
    seedDone: 'Seed finished: inserted {inserted}, refreshed {refreshed}, skipped admin-edited {skipped}',
    seedPartial: '{summary}; {failed} rows failed to write: {errors}',
    deleteTitle: 'Delete catalog entry',
    deleteConfirm: 'Aliases, intervals, and time pricing will be deleted with it. Continue?',
    fullReplaceHint: 'Save replaces the whole entry. Intervals and time pricing are written back unchanged; this page does not edit them yet.',
    fields: {
      modelId: 'Model id',
      displayName: 'Display name',
      vendor: 'Vendor',
      billingMode: 'Billing mode',
      status: 'Listing',
      managedBy: 'Managed by',
      inputPrice: 'Input price ($/token)',
      outputPrice: 'Output price ($/token)'
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
