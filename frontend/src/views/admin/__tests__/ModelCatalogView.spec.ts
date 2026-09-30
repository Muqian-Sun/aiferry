import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ModelCatalogView from '../ModelCatalogView.vue'
import CatalogEntryEditor from '@/components/admin/catalog/CatalogEntryEditor.vue'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

const { listEntries, createEntry, updateEntry, deleteEntry, seed, priceLookup, routerPush } = vi.hoisted(() => ({
  priceLookup: vi.fn(),
  routerPush: vi.fn(),
  listEntries: vi.fn(),
  createEntry: vi.fn(),
  updateEntry: vi.fn(),
  deleteEntry: vi.fn(),
  seed: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    modelCatalog: { listEntries, createEntry, updateEntry, deleteEntry, seed, priceLookup }
  }
}))


vi.mock('@/stores/app', () => ({
  useAppStore: () => ({})
}))

vi.mock('@/api', () => ({}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush })
}))

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
    per_request_price: null,
    search_price_per_call: null,
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
        AppLayout: { template: '<div><slot name="header-actions" /><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="summary" /><slot name="filters" /><slot name="table" /><slot name="bulk" /></div>' },
        // 弹出层（页头工具菜单、行尾「⋯」、筛选标签）桩成内联渲染，菜单项直接可点
        PopoverMenu: { template: '<div><slot name="trigger" :open="false" /><slot :close="() => {}" /></div>' },
        RouterLink: true,
        DataTable: {
          props: ['data'],
          template:
            '<div><slot name="cell-model_id" :row="data[0]" v-if="data[0]" /><slot name="cell-resources" :row="data[0]" v-if="data[0]" /><slot name="cell-actions" :row="data[0]" v-if="data[0]" /></div>'
        },
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: { props: ['show'], template: '<div v-if="show" />' },
        EmptyState: true,
        Icon: true,
        PlatformTypeBadge: true,
        CatalogEntryDiagnosisModal: {
          props: ['show', 'entryId', 'modelId'],
          template: '<div data-testid="diagnosis-stub" :data-show="show" :data-entry-id="entryId ?? \'\'" :data-model-id="modelId" />'
        }
      }
    }
  })
}

beforeEach(() => {
  listEntries.mockReset().mockResolvedValue([entry()])
  createEntry.mockReset().mockResolvedValue(entry())
  updateEntry.mockReset().mockResolvedValue(entry())
  deleteEntry.mockReset().mockResolvedValue(undefined)
  seed.mockReset().mockResolvedValue({ inserted: 10, refreshed: 2, skipped_admin: 1, skipped_invalid: 0, failed: 0 })
  priceLookup.mockReset().mockResolvedValue(null)
  routerPush.mockReset()
  vi.useRealTimers()
})

function mountEditor(initial: ModelCatalogEntry | null) {
  return mount(CatalogEntryEditor, {
    props: { entry: initial, vendorOptions: ['anthropic', 'openai'] },
    global: {
      stubs: {
        FormPageShell: { props: ['show'], template: '<div><slot /><slot name="footer" /></div>' },
        Icon: true,
        PlatformTypeBadge: true
      }
    }
  })
}

describe('ModelCatalogView', () => {
  it('lists catalog entries from the admin API', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(listEntries).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('claude-opus-4-6')
  })

  // 新建 / 编辑是独立页（2026-09-25，原来是列表页里的弹窗）
  it('opens the model form pages for create and edit', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-create"]').trigger('click')
    expect(routerPush).toHaveBeenCalledWith('/model-catalog/new')
    await wrapper.get('[data-testid="row-action-edit"]').trigger('click')
    expect(routerPush).toHaveBeenCalledWith('/model-catalog/1/edit')
  })

  it('shows the resource count and flags listed entries without resources', async () => {
    listEntries.mockResolvedValue([entry({ status: 'listed', bindings: [] })])
    let wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-no-resources"]').exists()).toBe(true)

    listEntries.mockResolvedValue([
      entry({ status: 'listed', bindings: [{ entry_id: 1, account_id: 7 }, { entry_id: 1, account_id: 8 }] })
    ])
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-no-resources"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="model-catalog-resource-count"]').text()).toBe('2')

    listEntries.mockResolvedValue([entry({ status: 'unlisted', bindings: [] })])
    wrapper = mountView()
    await flushPromises()
    // 默认只看已上架：先切到「全部」
    await wrapper.findAllComponents({ name: 'FilterChip' }).find((c) => c.props('testId') === 'model-catalog-filter-status')!.vm.$emit('update:modelValue', '')
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-no-resources"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="model-catalog-resource-count"]').text()).toBe('0')
  })

  it('seeds the catalog and reloads the list', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-seed"]').trigger('click')
    await flushPromises()
    expect(seed).toHaveBeenCalledTimes(1)
    expect(listEntries).toHaveBeenCalledTimes(2)
  })

  // 单条写库失败不拖垮整批，但不能静默：失败数和原因要作为错误提示摆出来。
})

// 模型表单（独立页 /model-catalog/new、/model-catalog/:id/edit 的表单本体）
describe('CatalogEntryEditor', () => {
  it('creates an entry with the form values', async () => {
    const wrapper = mountEditor(null)
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-model-id"]').setValue('gpt-5')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()
    expect(createEntry).toHaveBeenCalledTimes(1)
    expect(createEntry.mock.calls[0][0]).toMatchObject({ model_id: 'gpt-5', billing_mode: 'token', status: 'listed' })
  })

  // 编辑只改表单里露出的字段；Token 分段 / 分时 / 其余价格字段必须按原值写回，不能在保存时丢掉。
  it('keeps intervals, time pricing and hidden prices when editing', async () => {
    const existing = entry({
      cache_read_price: 1.5,
      cache_write_1h_price: 2.5,
      image_output_price: 4,
      max_reasoning_effort_multiplier: 3,
      intervals: [
        { min_tokens: 200000, max_tokens: null, tier_label: '', input_price: 0.000006, output_price: 0.0000225, cache_write_price: null, cache_write_1h_price: null, cache_read_price: null, input_multiplier: null, output_multiplier: null, cache_write_multiplier: null, cache_read_multiplier: null, per_request_price: null, sort_order: 0 }
      ],
      time_pricing: { timezone: 'Asia/Shanghai', weekdays_only: false, periods: [{ start_time: '09:00', end_time: '12:00', multiplier: 2 }] }
    })
    const wrapper = mountEditor(existing)
    await flushPromises()

    // 价格按每百万 Token 输入，存 $/token
    await wrapper.get('[data-testid="model-catalog-output-price"]').setValue('80')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()

    expect(updateEntry).toHaveBeenCalledTimes(1)
    expect(updateEntry.mock.calls[0][0]).toBe(existing.id)
    expect(updateEntry.mock.calls[0][1]).toMatchObject({
      model_id: existing.model_id,
      output_price: 0.00008,
      cache_read_price: 1.5,
      cache_write_1h_price: 2.5,
      image_output_price: 4,
      max_reasoning_effort_multiplier: 3,
      intervals: existing.intervals,
      time_pricing: existing.time_pricing
    })
    // 旧后端没有音频价：条目里没有就不发这两个键
    expect(updateEntry.mock.calls[0][1]).not.toHaveProperty('audio_input_price')
    expect(updateEntry.mock.calls[0][1]).not.toHaveProperty('audio_output_price')
  })

  // 价格按每百万 Token 输入、按 $/token 存：按 Token 计费时缓存价直接露出；图片 / 音频在收起的「更多价格」里，音频价有值才发
  it('edits cache prices directly and image / audio prices in the collapsed "more prices" section', async () => {
    const wrapper = mountEditor(entry({ cache_read_price: 0.0000015, audio_input_price: 0.00004 }))
    await flushPromises()

    const cacheRead = wrapper.get('[data-testid="model-catalog-cache-read-price"]')
    expect((cacheRead.element as HTMLInputElement).value).toBe('1.5')
    expect(cacheRead.element.parentElement?.textContent).toContain('admin.modelCatalog.editor.units.perMillion')
    expect(wrapper.find('[data-testid="model-catalog-image-input-price"]').exists()).toBe(false)
    const toggle = wrapper.get('[data-testid="model-catalog-more-prices-toggle"]')
    expect(toggle.text()).toContain('admin.modelCatalog.editor.morePricesFilled:{"count":1}')
    await toggle.trigger('click')

    await wrapper.get('[data-testid="model-catalog-cache-write-price"]').setValue('3.75')
    await wrapper.get('[data-testid="model-catalog-image-input-price"]').setValue('10')
    await wrapper.get('[data-testid="model-catalog-audio-output-price"]').setValue('80')
    await wrapper.get('[data-testid="model-catalog-audio-input-price"]').setValue('')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()

    const sent = updateEntry.mock.calls[0][1]
    expect(sent).toMatchObject({
      cache_write_price: 0.00000375,
      cache_read_price: 0.0000015,
      image_input_price: 0.00001,
      audio_output_price: 0.00008
    })
    expect(sent).not.toHaveProperty('audio_input_price')
  })

  // 新建用的空表单要清掉上一次编辑留下的隐藏字段（价格、分档、分时），否则会被带进新条目
  it('does not carry hidden fields from a previously edited entry into a new one', async () => {
    const wrapper = mountEditor(
      entry({
        cache_read_price: 1.5,
        audio_input_price: 0.00004,
        max_reasoning_effort_multiplier: 3,
        intervals: [{ min_tokens: 0, max_tokens: 200000, input_price: 3, output_price: 15 }],
        time_pricing: { timezone: 'Asia/Shanghai', weekdays_only: false, periods: [] }
      })
    )
    await flushPromises()
    await wrapper.setProps({ entry: null })
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-model-id"]').setValue('gpt-5')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()

    const sent = createEntry.mock.calls[0][0]
    expect(sent).toMatchObject({ model_id: 'gpt-5', cache_read_price: null, max_reasoning_effort_multiplier: null, intervals: [], time_pricing: null })
    expect(sent).not.toHaveProperty('audio_input_price')
  })

  // 数字输入清空后 v-model.number 给的是 ''，后端会报 400：清空要当成「未配置」发 null。
  it('sends null instead of an empty string for cleared number inputs', async () => {
    const wrapper = mountEditor(entry())
    await flushPromises()
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
    const wrapper = mountEditor(existing)
    await flushPromises()

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
    const wrapper = mountEditor(entry())
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-media-tiers"]').exists()).toBe(false)
    await wrapper.get('[data-testid="model-catalog-search-price-per-call"]').setValue('0.02')
    await wrapper.get('#model-catalog-form').trigger('submit.prevent')
    await flushPromises()

    expect(updateEntry).toHaveBeenCalledTimes(1)
    expect(updateEntry.mock.calls[0][1].search_price_per_call).toBe(0.02)
    // token 条目没配分段：intervals 为空
    expect(updateEntry.mock.calls[0][1].intervals).toEqual([])
  })

  it('offers only the four billing modes', async () => {
    const wrapper = mountEditor(null)
    await flushPromises()
    const options = wrapper.get('[data-testid="model-catalog-billing-mode"]').findAll('option')
    expect(options.map((o) => o.attributes('value'))).toEqual(['token', 'per_request', 'image', 'video'])
  })

  // 接口错误提示要走 extractApiErrorMessage：拦截器给的 { message, error } 与裸 Error 都能取到文案。

})
