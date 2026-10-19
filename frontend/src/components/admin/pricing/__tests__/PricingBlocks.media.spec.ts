import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'

import type { PricingAccount, PricingBinding, PricingEntry, PricingInterval } from '@/api/admin/pricing'
import PricingModelBlock from '../PricingModelBlock.vue'
import PricingChannelBlock from '../PricingChannelBlock.vue'
import { channelDraftFrom, cloneChannelDraft, cloneModelDraft, modelDraftFrom, type BlockState, type ChannelDraft, type ModelDraft } from '../pricingDraft'

const { saveModel, saveChannel } = vi.hoisted(() => ({ saveModel: vi.fn(), saveChannel: vi.fn() }))
vi.mock('@/api/admin', () => ({
  adminAPI: { pricing: { saveModel, saveChannel }, modelCatalog: { priceLookup: vi.fn() } }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) })
  }
})

function tier(label: string, price: number, sortOrder: number): PricingInterval {
  return {
    min_tokens: 0,
    max_tokens: null,
    tier_label: label,
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    input_multiplier: null,
    output_multiplier: null,
    cache_write_multiplier: null,
    cache_read_multiplier: null,
    per_request_price: price,
    sort_order: sortOrder
  }
}

function binding(entryId: number, accountId: number, overrides: Partial<PricingBinding> = {}): PricingBinding {
  return {
    entry_id: entryId,
    account_id: accountId,
    upstream_model: '',
    input_price: 0,
    output_price: 0,
    cache_write_price: null,
    cache_write_1h_price: null,
    cache_read_price: null,
    intervals: [],
    per_request_price: 0.002,
    cost_ratio: 0.002 / 0.06,
    peak_cost_ratio: null,
    max_reasoning_cost_ratio: null,
    time_pricing: null,
    ...overrides
  }
}

const emptySale = { input_price: null, output_price: null, cache_write_price: null, cache_write_1h_price: null, cache_read_price: null, segments: [] }

function imageEntry(): PricingEntry {
  return {
    id: 2,
    model_id: 'grok-imagine-image-2.0',
    display_name: '',
    vendor: 'xai',
    status: 'listed',
    billing_mode: 'image',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_write_1h_price: null,
    cache_read_price: null,
    per_request_price: 0.06,
    intervals: [tier('1K', 0.06, 0), tier('2K', 0.08, 1)],
    search_defaults: null,
    sale_prices: { ...emptySale },
    time_pricing: null,
    bindings: [binding(2, 1)],
    bindable_account_ids: [1, 3]
  }
}

function tokenEntry(): PricingEntry {
  return {
    id: 1,
    model_id: 'grok-4.5',
    display_name: '',
    vendor: 'xai',
    status: 'listed',
    billing_mode: 'token',
    input_price: 3e-6,
    output_price: 15e-6,
    cache_write_price: null,
    cache_write_1h_price: null,
    cache_read_price: 0.75e-6,
    per_request_price: null,
    intervals: [],
    search_defaults: null,
    sale_prices: { ...emptySale },
    time_pricing: null,
    bindings: [binding(1, 1, { input_price: 0.1e-6, output_price: 0.5e-6, cache_read_price: 0.02e-6, per_request_price: null, cost_ratio: 0.03 })],
    bindable_account_ids: [1]
  }
}

function account(id: number, name: string): PricingAccount {
  return { id, name, platform: 'grok', type: 'apikey', vendor: '', status: 'active', schedulable: true, priority: id, protocol: 'chat_completions', upstream_host: `h${id}.example` }
}
const accounts = [account(1, 'relay-a'), account(3, 'relay-b')]

function mountModelBlock(entry: PricingEntry) {
  const initial = modelDraftFrom(entry, (id) => id)
  const state = reactive<BlockState<ModelDraft>>({ initial, draft: cloneModelDraft(initial) })
  const wrapper = mount(PricingModelBlock, {
    props: {
      entry,
      state,
      accounts: new Map(accounts.map((a) => [a.id, a])),
      accountOrder: (id: number) => id,
      defaultSaleRatio: 1 / 15,
      defaultMediaSaleRatio: 1,
      minMargin: 0
    }
  })
  return { wrapper, state }
}

beforeEach(() => {
  saveModel.mockReset().mockResolvedValue({})
  saveChannel.mockReset().mockResolvedValue([])
})

describe('PricingModelBlock: per-request models', () => {
  it('shows per-request fields instead of the five token prices', async () => {
    const { wrapper } = mountModelBlock(imageEntry())
    expect(wrapper.find('[data-testid="pricing-official-per_request_price"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pricing-official-input_price"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pricing-sale-per_request_price"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pricing-sale-input_price"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pricing-binding-1-per_request_price"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pricing-binding-1-input_price"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pricing-official-peak-toggle"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pricing-model-lookup"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.pricing.media.column:{"unit":"admin.pricing.media.units.image"}')

    // 承接行写各档按官方同比例折算出的上游价：0.002 × 0.08 / 0.06
    const hint = wrapper.find('[data-testid="pricing-binding-1-tier-hint"]').text()
    expect(hint).toContain('1K 0.002')
    expect(hint).toContain('2K 0.00266667')

    await wrapper.find('[data-testid="pricing-official-tiers-toggle"]').trigger('click')
    expect(wrapper.findAll('[data-testid="pricing-official-tier"]')).toHaveLength(2)
    expect(wrapper.find('[data-testid="pricing-official-tier-add-4K"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pricing-official-tier-add-1K"]').exists()).toBe(false)
  })

  // 媒体没定按次售价按官方原价收（default_media_sale_ratio = 1），不跟文本的 1/15
  it('shows the official per-request price as the default sale price', () => {
    const { wrapper } = mountModelBlock(imageEntry())
    expect(wrapper.find('[data-testid="pricing-sale-per_request_price"]').attributes('placeholder')).toBe('0.06')
    const hint = wrapper.find('[data-testid="pricing-sale-tier-hint"]').text()
    expect(hint).toContain('1K 0.06')
    expect(hint).toContain('2K 0.08')

    const token = mountModelBlock(tokenEntry()).wrapper
    expect(token.find('[data-testid="pricing-sale-input_price"]').attributes('placeholder')).toBe('0.2')
  })

  it('saves per-request official, sale and upstream prices', async () => {
    const { wrapper } = mountModelBlock(imageEntry())
    await wrapper.find('[data-testid="pricing-official-per_request_price"]').setValue('0.05')
    await wrapper.find('[data-testid="pricing-sale-per_request_price"]').setValue('0.0045')
    await wrapper.find('[data-testid="pricing-binding-1-per_request_price"]').setValue('0.003')
    await wrapper.find('[data-testid="pricing-official-tiers-toggle"]').trigger('click')
    await wrapper.find('[data-testid="pricing-official-tier-add-4K"]').trigger('click')
    await wrapper.find('[data-testid="pricing-official-tier-4K"]').setValue('0.12')

    await wrapper.find('[data-testid="pricing-model-save"]').trigger('click')
    await flushPromises()
    expect(saveModel).toHaveBeenCalledTimes(1)
    const [entryId, body] = saveModel.mock.calls[0]
    expect(entryId).toBe(2)
    expect(body.per_request_price).toBe(0.05)
    expect(body.input_price).toBeNull()
    expect(body.intervals.map((iv: PricingInterval) => [iv.tier_label, iv.per_request_price])).toEqual([
      ['1K', 0.06],
      ['2K', 0.08],
      ['4K', 0.12]
    ])
    expect(body.sale_prices.per_request_price).toBe(0.0045)
    expect(body.sale_prices.segments).toEqual([])
    expect(body.bindings).toEqual([
      expect.objectContaining({ account_id: 1, per_request_price: 0.003, input_price: null, output_price: null, intervals: [], time_pricing: null })
    ])
  })

  // 视频分档扩到 768p / 2k / 4k（官方视频厂商：MiniMax 768P / 2K，Seedance / 可灵 4k）
  it('offers 768p / 2k / 4k video tiers and saves them with the backend labels', async () => {
    const entry: PricingEntry = {
      ...imageEntry(),
      id: 5,
      model_id: 'MiniMax-H3',
      vendor: 'minimax',
      billing_mode: 'video',
      per_request_price: 0.08,
      intervals: [tier('768p', 0.08, 0)],
      bindings: [binding(5, 1, { per_request_price: 0.04 })]
    }
    const { wrapper } = mountModelBlock(entry)
    await wrapper.find('[data-testid="pricing-official-tiers-toggle"]').trigger('click')
    for (const label of ['480p', '720p', '1080p', '2k', '4k']) {
      expect(wrapper.find(`[data-testid="pricing-official-tier-add-${label}"]`).exists(), label).toBe(true)
    }
    expect(wrapper.find('[data-testid="pricing-official-tier-add-768p"]').exists(), '已有的档不再列').toBe(false)
    await wrapper.find('[data-testid="pricing-official-tier-add-2k"]').trigger('click')
    await wrapper.find('[data-testid="pricing-official-tier-2k"]').setValue('0.13')
    await wrapper.find('[data-testid="pricing-official-tier-add-4k"]').trigger('click')
    await wrapper.find('[data-testid="pricing-official-tier-4k"]').setValue('0.5')
    await wrapper.find('[data-testid="pricing-model-save"]').trigger('click')
    await flushPromises()
    const body = saveModel.mock.calls[0][1]
    expect(body.intervals.map((iv: PricingInterval) => [iv.tier_label, iv.per_request_price])).toEqual([
      ['768p', 0.08],
      ['2k', 0.13],
      ['4k', 0.5]
    ])
  })

  it('blocks saving while the upstream per-request price is missing', async () => {
    const { wrapper } = mountModelBlock(imageEntry())
    await wrapper.find('[data-testid="pricing-binding-1-per_request_price"]').setValue('')
    const save = wrapper.find('[data-testid="pricing-model-save"]')
    expect(save.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('admin.pricing.issues.missing:{"fields":"admin.pricing.columns.per_request_price"}')
  })

  it('token models keep the five token prices and the same payload', async () => {
    const { wrapper } = mountModelBlock(tokenEntry())
    expect(wrapper.find('[data-testid="pricing-official-input_price"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pricing-official-per_request_price"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pricing-binding-1-per_request_price"]').exists()).toBe(false)
    await wrapper.find('[data-testid="pricing-binding-1-output_price"]').setValue('0.6')
    await wrapper.find('[data-testid="pricing-model-save"]').trigger('click')
    await flushPromises()
    const body = saveModel.mock.calls[0][1]
    expect(body.input_price).toBe(3e-6)
    expect('per_request_price' in body).toBe(false)
    expect('per_request_price' in body.sale_prices).toBe(false)
    expect('per_request_price' in body.bindings[0]).toBe(false)
    expect(body.bindings[0].output_price).toBeCloseTo(0.6e-6, 18)
  })
})

describe('PricingChannelBlock: per-request rows next to token rows', () => {
  it('renders and saves an upstream per-request price for the media row only', async () => {
    const entries = [tokenEntry(), imageEntry()]
    const initial = channelDraftFrom(1, entries)
    const state = reactive<BlockState<ChannelDraft>>({ initial, draft: cloneChannelDraft(initial) })
    const wrapper = mount(PricingChannelBlock, {
      props: { account: accounts[0], state, accounts, entries, defaultSaleRatio: 1 / 15, minMargin: 0 }
    })
    expect(wrapper.find('[data-testid="pricing-channel-row-2-per_request_price"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="pricing-channel-row-2-input_price"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="pricing-channel-row-1-input_price"]').exists()).toBe(true)

    await wrapper.find('[data-testid="pricing-channel-row-2-per_request_price"]').setValue('0')
    await wrapper.find('[data-testid="pricing-channel-save"]').trigger('click')
    await flushPromises()
    const body = saveChannel.mock.calls[0][1]
    const media = body.bindings.find((b: { entry_id: number }) => b.entry_id === 2)
    const token = body.bindings.find((b: { entry_id: number }) => b.entry_id === 1)
    expect(media).toEqual(expect.objectContaining({ per_request_price: 0, input_price: null, time_pricing: null }))
    expect('per_request_price' in token).toBe(false)
    expect(token.input_price).toBe(0.1e-6)
  })
})
