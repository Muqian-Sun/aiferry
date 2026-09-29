import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ModelCatalogView from '../ModelCatalogView.vue'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

const { listEntries, updateEntry, seed, getBindings, updateBindings, listAccounts } = vi.hoisted(() => ({
  listEntries: vi.fn(),
  updateEntry: vi.fn(),
  seed: vi.fn(),
  getBindings: vi.fn(),
  updateBindings: vi.fn(),
  listAccounts: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    modelCatalog: { listEntries, createEntry: vi.fn(), updateEntry, deleteEntry: vi.fn(), seed, getBindings, updateBindings },
    accounts: { list: listAccounts }
  }
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => ({}) }))
vi.mock('@/api', () => ({}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key)
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
    input_price: 0.000015,
    output_price: 0.000075,
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

const binding = { entry_id: 0, account_id: 9, priority: null }

const fixtures = [
  entry({ id: 1, model_id: 'claude-opus-4-6', vendor: 'anthropic', status: 'listed', bindings: [binding] }),
  entry({ id: 2, model_id: 'gpt-5.6', vendor: 'openai', status: 'listed', bindings: [], aliases: [{ id: 1, entry_id: 2, alias: 'gpt-5.6-sol', source: 'seed', created_at: '', updated_at: '' }] }),
  entry({ id: 3, model_id: 'gpt-image-2', vendor: 'openai', status: 'unlisted', billing_mode: 'image', input_price: null, output_price: null, per_request_price: 0.04, intervals: [] }),
  entry({ id: 4, model_id: 'nameless', vendor: '', status: 'unlisted', input_price: null, output_price: null }),
  entry({ id: 5, model_id: 'veo-x', vendor: 'google', status: 'unlisted', billing_mode: 'video', input_price: null, output_price: null, per_request_price: null, intervals: [] })
]

/** DataTable 桩：渲染每行的 model_id / price 格，并给一个「全选」按钮往外发 update:selected-keys */
const DataTableStub = {
  props: ['data', 'selectedKeys'],
  emits: ['update:selected-keys'],
  template: `
    <div>
      <button data-testid="select-all" @click="$emit('update:selected-keys', data.map((r) => r.id))">all</button>
      <div v-for="row in data" :key="row.id" data-testid="row">
        <slot name="cell-model_id" :row="row" />
        <slot name="cell-price" :row="row" />
        <slot name="cell-resources" :row="row" />
      </div>
    </div>`
}

function mountView() {
  return mount(ModelCatalogView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot name="header-actions" /><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="summary" /><slot name="filters" /><slot name="table" /><slot name="bulk" /><slot name="pagination" /></div>' },
        PopoverMenu: { template: '<div><slot name="trigger" :open="false" /><slot :close="() => {}" /></div>' },
        RouterLink: true,
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: { props: ['show'], template: '<div v-if="show" />' },
        EmptyState: true,
        Icon: true,
        PlatformTypeBadge: true,
        CatalogEntryDiagnosisModal: true
      }
    }
  })
}

const rowIds = (wrapper: ReturnType<typeof mountView>) => wrapper.findAll('[data-testid="row"]').map((row) => row.find('.font-mono').text())

/** 筛选标签（A4）：空串 = 全部 */
async function pickFilter(wrapper: ReturnType<typeof mountView>, testid: string, value: string) {
  const chip = wrapper.findAllComponents({ name: 'FilterChip' }).find((c) => c.props('testId') === testid)!
  await chip.vm.$emit('update:modelValue', value)
  await flushPromises()
}

describe('ModelCatalogView filters, summary, prices and bulk status', () => {
  beforeEach(() => {
    listEntries.mockReset().mockResolvedValue(fixtures.map((f) => ({ ...f })))
    updateEntry.mockReset().mockImplementation(async (_id: number, body: { model_id: string }) => entry({ model_id: body.model_id }))
    getBindings.mockReset().mockResolvedValue([])
    updateBindings.mockReset().mockResolvedValue([])
    listAccounts.mockReset().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
  })

  it('summarises total / listed / listed-without-resources and lists only listed entries by default', async () => {
    const wrapper = mountView()
    await flushPromises()
    const summary = wrapper.get('[data-testid="model-catalog-summary"]')
    expect(summary.get('[data-testid="stat-total"]').text()).toContain('5')
    expect(summary.get('[data-testid="stat-listed"]').text()).toContain('2')
    expect(summary.get('[data-testid="stat-unbound"]').text()).toContain('1')
    expect(rowIds(wrapper)).toEqual(['claude-opus-4-6', 'gpt-5.6'])

    // 「上架但无渠道」旁的「筛选」一键筛出这些条目
    await summary.get('[data-testid="stat-unbound-action"]').trigger('click')
    await flushPromises()
    expect(rowIds(wrapper)).toEqual(['gpt-5.6'])
  })

  it('filters by status, vendor (including the blank vendor), billing mode and resources', async () => {
    const wrapper = mountView()
    await flushPromises()

    await pickFilter(wrapper, 'model-catalog-filter-status', 'unlisted')
    expect(rowIds(wrapper)).toEqual(['gpt-image-2', 'nameless', 'veo-x'])
    await pickFilter(wrapper, 'model-catalog-filter-status', '')

    await pickFilter(wrapper, 'model-catalog-filter-vendor', 'OpenAI')
    expect(rowIds(wrapper)).toEqual(['gpt-5.6', 'gpt-image-2'])
    // 「无厂商」是单独一个选项（空串已表示不筛）
    const vendorChip = wrapper.findAllComponents({ name: 'FilterChip' }).find((c) => c.props('testId') === 'model-catalog-filter-vendor')!
    const noVendor = (vendorChip.props('options') as { value: string; label: string }[]).find((o) => o.label === 'admin.modelCatalog.filters.noVendor')!
    await pickFilter(wrapper, 'model-catalog-filter-vendor', noVendor.value)
    expect(rowIds(wrapper)).toEqual(['nameless'])
    await pickFilter(wrapper, 'model-catalog-filter-vendor', '')

    await pickFilter(wrapper, 'model-catalog-filter-billing', 'image')
    expect(rowIds(wrapper)).toEqual(['gpt-image-2'])
    await pickFilter(wrapper, 'model-catalog-filter-billing', '')

    await pickFilter(wrapper, 'model-catalog-filter-resources', 'bound')
    expect(rowIds(wrapper)).toEqual(['claude-opus-4-6'])
    expect(wrapper.text()).toContain('admin.modelCatalog.filtered:{"count":1}')
  })

  it('searches aliases too', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('update:modelValue', 'sol')
    await flushPromises()
    expect(rowIds(wrapper)).toEqual(['gpt-5.6'])
    expect(wrapper.text()).toContain('admin.modelCatalog.aliasCount:{"count":1}')
  })

  it('shows list prices per million tokens, per-unit prices for media, and flags unpriced entries', async () => {
    const wrapper = mountView()
    await flushPromises()
    await pickFilter(wrapper, 'model-catalog-filter-status', '')
    const prices = wrapper.findAll('[data-testid="model-catalog-price"]')
    expect(prices[0].text()).toContain('$15.00')
    expect(prices[0].text()).toContain('$75.00')
    expect(prices[0].find('.text-af-danger').exists()).toBe(false)
    expect(prices[2].text()).toContain('$0.04')
    expect(prices[2].text()).toContain('admin.modelCatalog.columns.perUnit.image')
    // 没配价：写「未配价」（token 与媒体模式各一条）；这两条未上架，不标红
    expect(prices[3].text()).toContain('admin.modelCatalog.columns.unpriced')
    expect(prices[4].text()).toContain('admin.modelCatalog.columns.unpriced')
    expect(prices[3].find('.text-af-danger').exists()).toBe(false)
    expect(prices[4].find('.text-af-danger').exists()).toBe(false)
  })

  it('bulk-lists the selection with full-entry PUTs, skipping entries already in that state', async () => {
    const wrapper = mountView()
    await flushPromises()
    await pickFilter(wrapper, 'model-catalog-filter-status', '')
    await wrapper.get('[data-testid="select-all"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('common.selectedItems:{"count":5}')

    await wrapper.get('[data-testid="model-catalog-bulk-list"]').trigger('click')
    await flushPromises()

    // 1 / 2 已上架跳过；3 / 4 / 5 整条覆盖 + status=listed
    expect(updateEntry).toHaveBeenCalledTimes(3)
    expect(updateEntry).toHaveBeenCalledWith(3, expect.objectContaining({ model_id: 'gpt-image-2', status: 'listed', per_request_price: 0.04 }))
    expect(updateEntry).toHaveBeenCalledWith(4, expect.objectContaining({ model_id: 'nameless', status: 'listed' }))
    expect(listEntries).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="bulk-bar"]').exists()).toBe(false)
  })

  it('keeps the failed entries selected and reports each reason when a bulk update is partly rejected', async () => {
    updateEntry.mockImplementation(async (id: number, body: { model_id: string }) => {
      if (id === 4) throw { response: { data: { message: 'listed model requires a price' } } }
      return entry({ model_id: body.model_id })
    })
    const wrapper = mountView()
    await flushPromises()
    await pickFilter(wrapper, 'model-catalog-filter-status', '')
    await wrapper.get('[data-testid="select-all"]').trigger('click')
    await wrapper.get('[data-testid="model-catalog-bulk-list"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('common.selectedItems:{"count":1}')
  })

  it('does nothing but say so when every selected entry already has the target status', async () => {
    listEntries.mockResolvedValue([fixtures[0], fixtures[1]])
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="select-all"]').trigger('click')
    await wrapper.get('[data-testid="model-catalog-bulk-list"]').trigger('click')
    await flushPromises()
    expect(updateEntry).not.toHaveBeenCalled()
  })
})
