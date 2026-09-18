import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ModelCatalogView from '../ModelCatalogView.vue'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

const { listEntries, createEntry, updateEntry, deleteEntry, seed } = vi.hoisted(() => ({
  listEntries: vi.fn(),
  createEntry: vi.fn(),
  updateEntry: vi.fn(),
  deleteEntry: vi.fn(),
  seed: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    modelCatalog: { listEntries, createEntry, updateEntry, deleteEntry, seed }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('@/api', () => ({}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key
  }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'zh' } } })
}))

function entry(overrides: Partial<ModelCatalogEntry> = {}): ModelCatalogEntry {
  return {
    id: 1,
    model_id: 'claude-opus-4-6',
    display_name: 'Opus',
    vendor: 'anthropic',
    protocols: ['anthropic'],
    billing_mode: 'token',
    status: 'listed',
    managed_by: 'seed',
    input_price: 15,
    output_price: 75,
    cache_write_price: null,
    cache_write_1h_price: null,
    cache_read_price: null,
    image_input_price: null,
    image_output_price: null,
    image_cache_read_price: null,
    input_price_priority: null,
    output_price_priority: null,
    cache_write_price_priority: null,
    cache_read_price_priority: null,
    per_request_price: null,
    long_context_input_threshold: null,
    long_context_threshold_inclusive: false,
    long_context_input_multiplier: null,
    long_context_output_multiplier: null,
    fast_multiplier: null,
    flex_multiplier: null,
    max_reasoning_effort_multiplier: null,
    intervals: [],
    aliases: [],
    created_at: '2026-09-18T00:00:00Z',
    updated_at: '2026-09-18T00:00:00Z',
    ...overrides
  }
}

function mountView() {
  return mount(ModelCatalogView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
        DataTable: {
          props: ['data'],
          template: '<div><slot name="cell-model_id" :row="data[0]" v-if="data[0]" /><slot name="cell-actions" :row="data[0]" v-if="data[0]" /></div>'
        },
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: { props: ['show'], template: '<div v-if="show" />' },
        EmptyState: true,
        Icon: true
      }
    }
  })
}

describe('ModelCatalogView', () => {
  beforeEach(() => {
    listEntries.mockReset().mockResolvedValue([entry()])
    createEntry.mockReset().mockResolvedValue(entry())
    updateEntry.mockReset().mockResolvedValue(entry())
    deleteEntry.mockReset().mockResolvedValue(undefined)
    seed.mockReset().mockResolvedValue({ inserted: 10, refreshed: 2, skipped_admin: 1, skipped_invalid: 0 })
  })

  it('lists catalog entries from the admin API', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(listEntries).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('claude-opus-4-6')
  })

  it('creates an entry with the form values', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-create"]').trigger('click')
    await wrapper.get('[data-testid="model-catalog-model-id"]').setValue('gpt-5')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()
    expect(createEntry).toHaveBeenCalledTimes(1)
    expect(createEntry.mock.calls[0][0]).toMatchObject({ model_id: 'gpt-5', billing_mode: 'token', status: 'listed' })
  })

  it('seeds the catalog and reloads the list', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-seed"]').trigger('click')
    await flushPromises()
    expect(seed).toHaveBeenCalledTimes(1)
    expect(listEntries).toHaveBeenCalledTimes(2)
  })
})
