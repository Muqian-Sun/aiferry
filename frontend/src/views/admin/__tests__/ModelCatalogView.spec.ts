import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ModelCatalogView from '../ModelCatalogView.vue'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

const { listEntries, createEntry, updateEntry, deleteEntry, seed, getBindings, updateBindings, listAccounts } = vi.hoisted(() => ({
  listEntries: vi.fn(),
  createEntry: vi.fn(),
  updateEntry: vi.fn(),
  deleteEntry: vi.fn(),
  seed: vi.fn(),
  getBindings: vi.fn(),
  updateBindings: vi.fn(),
  listAccounts: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    modelCatalog: { listEntries, createEntry, updateEntry, deleteEntry, seed, getBindings, updateBindings },
    accounts: { list: listAccounts }
  }
}))

const { showError, showSuccess } = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess })
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
    search_price_per_call: null,
    long_context_input_threshold: null,
    long_context_threshold_inclusive: false,
    long_context_input_multiplier: null,
    long_context_output_multiplier: null,
    fast_multiplier: null,
    flex_multiplier: null,
    max_reasoning_effort_multiplier: null,
    intervals: [],
    aliases: [],
    bindings: [],
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
          template:
            '<div><slot name="cell-model_id" :row="data[0]" v-if="data[0]" /><slot name="cell-resources" :row="data[0]" v-if="data[0]" /><slot name="cell-actions" :row="data[0]" v-if="data[0]" /></div>'
        },
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: { props: ['show'], template: '<div v-if="show" />' },
        EmptyState: true,
        Icon: true,
        PlatformTypeBadge: true
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
    seed.mockReset().mockResolvedValue({ inserted: 10, refreshed: 2, skipped_admin: 1, skipped_invalid: 0, failed: 0 })
    getBindings.mockReset().mockResolvedValue([])
    updateBindings.mockReset().mockResolvedValue([])
    listAccounts.mockReset().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    showError.mockReset()
    showSuccess.mockReset()
    vi.useRealTimers()
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
    // 新建成功后用返回的 ID 写绑定（空列表也要写，保证条目与绑定同一份来源）。
    expect(updateBindings).toHaveBeenCalledWith(entry().id, [])
  })

  it('shows the resource count and flags listed entries without resources', async () => {
    listEntries.mockResolvedValue([entry({ status: 'listed', bindings: [] })])
    let wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-no-resources"]').exists()).toBe(true)

    listEntries.mockResolvedValue([
      entry({ status: 'listed', bindings: [{ entry_id: 1, account_id: 7, priority: null }, { entry_id: 1, account_id: 8, priority: 3 }] })
    ])
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-no-resources"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="model-catalog-resource-count"]').text()).toBe('2')

    listEntries.mockResolvedValue([entry({ status: 'unlisted', bindings: [] })])
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-no-resources"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="model-catalog-resource-count"]').text()).toBe('0')
  })

  // 编辑时先读绑定预填；保存先存条目再整份覆盖绑定，payload 只带 account_id 与 priority。
  it('loads bindings on edit and saves them after the entry', async () => {
    vi.useFakeTimers()
    getBindings.mockResolvedValue([
      { entry_id: 1, account_id: 7, priority: 5, account: { id: 7, name: 'relay-a', platform: 'openai', type: 'apikey', vendor: '', status: 'active' } }
    ])
    listAccounts.mockResolvedValue({
      items: [
        { id: 7, name: 'relay-a', platform: 'openai', type: 'apikey', status: 'active' },
        { id: 9, name: 'oauth-b', platform: 'anthropic', type: 'oauth', vendor: 'anthropic', status: 'active' }
      ],
      total: 2, page: 1, page_size: 20, pages: 1
    })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-actions-edit"]').trigger('click')
    await flushPromises()
    expect(getBindings).toHaveBeenCalledWith(1)
    expect(wrapper.findAll('[data-testid="model-catalog-binding-remove"]')).toHaveLength(1)

    await wrapper.get('[data-testid="model-catalog-resource-search"]').setValue('relay')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledWith(1, 20, { search: 'relay', lite: 'true' })
    const addButtons = wrapper.findAll('[data-testid="model-catalog-resource-add"]')
    expect(addButtons).toHaveLength(2)
    expect(addButtons[0].attributes('disabled')).toBeDefined()
    await addButtons[1].trigger('click')
    expect(wrapper.findAll('[data-testid="model-catalog-binding-remove"]')).toHaveLength(2)

    const calls: string[] = []
    updateEntry.mockImplementation(async () => { calls.push('entry'); return entry() })
    updateBindings.mockImplementation(async () => { calls.push('bindings'); return [] })
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()
    expect(calls).toEqual(['entry', 'bindings'])
    expect(updateBindings).toHaveBeenCalledWith(1, [
      { account_id: 7, priority: 5 },
      { account_id: 9, priority: null }
    ])
  })

  it('removing a binding drops it from the saved list', async () => {
    getBindings.mockResolvedValue([
      { entry_id: 1, account_id: 7, priority: null, account: { id: 7, name: 'relay-a', platform: 'openai', type: 'apikey', vendor: '', status: 'active' } },
      { entry_id: 1, account_id: 8, priority: null, account: { id: 8, name: 'relay-b', platform: 'openai', type: 'apikey', vendor: '', status: 'active' } }
    ])
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-actions-edit"]').trigger('click')
    await flushPromises()
    await wrapper.findAll('[data-testid="model-catalog-binding-remove"]')[0].trigger('click')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()
    expect(updateBindings).toHaveBeenCalledWith(1, [{ account_id: 8, priority: null }])
  })

  // 绑定被后端拒绝（资源承接不了该网关族）：弹出后端原因，编辑器保持打开。
  it('keeps the editor open and shows the reason when bindings are rejected', async () => {
    updateBindings.mockRejectedValue({ message: 'account 7 has no upstream address usable on the anthropic gateway', error: 'CATALOG_BINDING_UNSERVABLE' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-actions-edit"]').trigger('click')
    await flushPromises()
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('account 7 has no upstream address usable on the anthropic gateway')
    expect(wrapper.find('#model-catalog-form').exists()).toBe(true)
  })

  it('seeds the catalog and reloads the list', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-seed"]').trigger('click')
    await flushPromises()
    expect(seed).toHaveBeenCalledTimes(1)
    expect(listEntries).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalledTimes(1)
    expect(showError).not.toHaveBeenCalled()
  })

  // 单条写库失败不拖垮整批，但不能静默：失败数和原因要作为错误提示摆出来。
  it('surfaces seed row failures instead of reporting plain success', async () => {
    seed.mockResolvedValue({
      inserted: 9, refreshed: 0, skipped_admin: 0, skipped_invalid: 0,
      failed: 1, errors: ['bad-model: value too long']
    })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-seed"]').trigger('click')
    await flushPromises()
    expect(showSuccess).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledTimes(1)
    expect(String(showError.mock.calls[0][0])).toContain('bad-model: value too long')
  })

  // 编辑只改表单里露出的字段；分档 / 分时 / 其余价格字段必须按原值写回，不能在保存时丢掉。
  it('keeps intervals, time pricing and hidden prices when editing', async () => {
    const existing = entry({
      cache_read_price: 1.5,
      intervals: [{ min_tokens: 0, max_tokens: 200000, input_price: 3, output_price: 15 }],
      time_pricing: { timezone: 'Asia/Shanghai', weekdays_only: false, periods: [{ start_time: '09:00', end_time: '12:00', multiplier: 2 }] }
    })
    listEntries.mockResolvedValue([existing])
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="model-catalog-actions-edit"]').trigger('click')
    await wrapper.get('[data-testid="model-catalog-output-price"]').setValue('80')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()

    expect(updateEntry).toHaveBeenCalledTimes(1)
    expect(updateEntry.mock.calls[0][0]).toBe(existing.id)
    expect(updateEntry.mock.calls[0][1]).toMatchObject({
      model_id: existing.model_id,
      output_price: 80,
      cache_read_price: 1.5,
      intervals: existing.intervals,
      time_pricing: existing.time_pricing
    })
  })

  // 数字输入清空后 v-model.number 给的是 ''，后端会报 400：清空要当成「未配置」发 null。
  it('sends null instead of an empty string for cleared number inputs', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-actions-edit"]').trigger('click')
    await wrapper.get('[data-testid="model-catalog-input-price"]').setValue('')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()

    expect(updateEntry).toHaveBeenCalledTimes(1)
    expect(updateEntry.mock.calls[0][1].input_price).toBeNull()
    expect(updateEntry.mock.calls[0][1].output_price).toBe(75)
  })

  // 图片 / 视频条目的分档在本页编辑：档位下拉只给后端认的标签，保存时按行序写成 intervals。
  it('edits image tiers and sends them as intervals with the default per-image price', async () => {
    const existing = entry({
      model_id: 'grok-imagine-image-quality',
      billing_mode: 'image',
      input_price: null,
      output_price: null,
      per_request_price: 0.05,
      intervals: [
        { min_tokens: 0, max_tokens: null, tier_label: '1K', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, input_multiplier: null, output_multiplier: null, cache_write_multiplier: null, cache_read_multiplier: null, per_request_price: 0.05, sort_order: 0 }
      ]
    })
    listEntries.mockResolvedValue([existing])
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="model-catalog-actions-edit"]').trigger('click')
    expect(wrapper.findAll('[data-testid="model-catalog-media-tier-row"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="model-catalog-search-price-per-call"]').exists()).toBe(false)

    await wrapper.get('[data-testid="model-catalog-per-request-price"]').setValue('0.06')
    await wrapper.get('[data-testid="model-catalog-media-tier-add"]').trigger('click')
    const rows = wrapper.findAll('[data-testid="model-catalog-media-tier-row"]')
    expect(rows).toHaveLength(2)
    const labels = rows[1].get('[data-testid="model-catalog-media-tier-label"]').findAll('option').map((o) => o.attributes('value'))
    expect(labels).toEqual(['1K', '2K', '4K'])
    await rows[1].get('[data-testid="model-catalog-media-tier-label"]').setValue('2K')
    await rows[1].get('[data-testid="model-catalog-media-tier-price"]').setValue('0.07')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()

    expect(updateEntry).toHaveBeenCalledTimes(1)
    const sent = updateEntry.mock.calls[0][1]
    expect(sent.per_request_price).toBe(0.06)
    expect(sent.intervals).toEqual([
      expect.objectContaining({ tier_label: '1K', per_request_price: 0.05, sort_order: 0 }),
      expect.objectContaining({ tier_label: '2K', per_request_price: 0.07, sort_order: 1 })
    ])
  })

  it('shows search price per call only for token entries and sends it', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-actions-edit"]').trigger('click')
    expect(wrapper.find('[data-testid="model-catalog-media-tiers"]').exists()).toBe(false)
    await wrapper.get('[data-testid="model-catalog-search-price-per-call"]').setValue('0.02')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()

    expect(updateEntry).toHaveBeenCalledTimes(1)
    expect(updateEntry.mock.calls[0][1].search_price_per_call).toBe(0.02)
    // token 条目的区间分档按原值写回
    expect(updateEntry.mock.calls[0][1].intervals).toEqual([])
  })

  it('offers only the four billing modes', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-create"]').trigger('click')
    const options = wrapper.get('[data-testid="model-catalog-billing-mode"]').findAll('option')
    expect(options.map((o) => o.attributes('value'))).toEqual(['token', 'per_request', 'image', 'video'])
  })

  // 接口错误提示要走 extractApiErrorMessage：拦截器给的 { message, error } 与裸 Error 都能取到文案。
  it('shows the API error message when saving fails', async () => {
    createEntry.mockRejectedValue({ message: 'model catalog entry already exists', error: 'MODEL_CATALOG_ENTRY_EXISTS' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-create"]').trigger('click')
    await wrapper.get('[data-testid="model-catalog-model-id"]').setValue('gpt-5')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('model catalog entry already exists')
  })
})
