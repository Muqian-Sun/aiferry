import { describe, expect, it } from 'vitest'

import type { PricingBinding, PricingEntry } from '@/api/admin/pricing'
import { bindByDefault } from '@/components/account/channel/upstreamModels'
import {
  bindingRowIssues,
  emptyPriceRow,
  entryHasProblem,
  fillByDiscount,
  fillSaleByRatio,
  modelDraftFrom,
  officialIssues,
  perRequestTierPrice,
  priceRowChanges,
  priceRowFrom,
  priceRowToRequest,
  saleDefaults,
  saleRowFrom,
  saleRowToRequest,
  scaledTierPrices
} from '../pricingDraft'

function tier(label: string, price: number, sortOrder: number) {
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

function binding(overrides: Partial<PricingBinding> = {}): PricingBinding {
  return {
    entry_id: 2,
    account_id: 1,
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

/** 生图条目：标价 0.06 / 张，1K 0.06、2K 0.08；token 价是照抄的（价格页不改它） */
function imageEntry(overrides: Partial<PricingEntry> = {}): PricingEntry {
  return {
    id: 2,
    model_id: 'grok-imagine-image-2.0',
    display_name: '',
    vendor: 'xai',
    status: 'listed',
    billing_mode: 'image',
    input_price: 2e-6,
    output_price: 10e-6,
    cache_write_price: null,
    cache_write_1h_price: null,
    cache_read_price: null,
    per_request_price: 0.06,
    intervals: [tier('2K', 0.08, 1), tier('1K', 0.06, 0)],
    search_defaults: null,
    sale_prices: { input_price: null, output_price: null, cache_write_price: null, cache_write_1h_price: null, cache_read_price: null, segments: [] },
    time_pricing: null,
    bindings: [binding()],
    bindable_account_ids: [1],
    ...overrides
  }
}

describe('pricingDraft: per-request models', () => {
  it('reads the per-request price and tiers, never token segments', () => {
    const row = priceRowFrom(imageEntry(), 'image')
    expect(row.per_request_price).toBe(0.06)
    expect(row.tiers.map((t) => [t.label, t.price])).toEqual([
      ['1K', 0.06],
      ['2K', 0.08]
    ])
    expect(row.segments).toEqual([])
    expect(row.input_price).toBeNull()

    const token = priceRowFrom(imageEntry({ billing_mode: 'token', intervals: [] }), 'token')
    expect(token.per_request_price).toBeNull()
    expect(token.input_price).toBe(2e-6)
  })

  it('binding rows carry the upstream per-request price', () => {
    const draft = modelDraftFrom(imageEntry(), (id) => id)
    expect(draft.rows[0].prices.per_request_price).toBe(0.002)
    expect(draft.rows[0].prices.tiers).toEqual([])
  })

  it('requires the official and the upstream per-request price', () => {
    const official = priceRowFrom(imageEntry({ per_request_price: null }), 'image')
    official.tiers[1].price = null
    const issues = officialIssues(official, 'image')
    expect(issues.missing).toEqual(['per_request_price'])
    expect(issues.tiers).toEqual([null, 'missing'])

    const row = { id: 1, upstreamModel: '', prices: emptyPriceRow(), peak: null }
    expect(bindingRowIssues(row, { ...official, per_request_price: 0.06 }, 'image').missing).toEqual(['per_request_price'])
    row.prices.per_request_price = 0
    expect(bindingRowIssues(row, { ...official, per_request_price: 0.06 }, 'image').missing).toEqual([])
    // 按 Token 计费的模型规则不变：输入 / 输出必填，不看按次价
    expect(bindingRowIssues(row, { ...official, input_price: 1e-6 }, 'token').missing).toEqual(['input_price', 'output_price'])
  })

  it('submits only the per-request price (and tiers on the official row)', () => {
    const official = priceRowFrom(imageEntry(), 'image')
    const body = priceRowToRequest(official, 'image')
    expect(body.per_request_price).toBe(0.06)
    expect(body.input_price).toBeNull()
    expect(body.intervals.map((iv) => [iv.tier_label, iv.per_request_price, iv.sort_order])).toEqual([
      ['1K', 0.06, 0],
      ['2K', 0.08, 1]
    ])
    const token = priceRowToRequest(priceRowFrom(imageEntry({ billing_mode: 'token', intervals: [] })))
    expect('per_request_price' in token).toBe(false)
  })

  it('counts per-request and tier edits as changes', () => {
    const initial = priceRowFrom(imageEntry(), 'image')
    const current = priceRowFrom(imageEntry(), 'image')
    expect(priceRowChanges(current, initial)).toBe(0)
    current.per_request_price = 0.07
    current.tiers[1].price = 0.09
    expect(priceRowChanges(current, initial)).toBe(2)
  })

  it('sale price: one per-request price, default = official × ratio', () => {
    const official = priceRowFrom(imageEntry(), 'image')
    const sale = saleRowFrom({ ...imageEntry().sale_prices, per_request_price: 0.0045 })
    expect(sale.per_request_price).toBe(0.0045)
    expect(saleDefaults(official, sale, 1 / 15).perRequest).toBeCloseTo(0.004, 12)
    const body = saleRowToRequest(sale, official, 'image')
    expect(body).toEqual({
      input_price: null,
      output_price: null,
      cache_write_price: null,
      cache_write_1h_price: null,
      cache_read_price: null,
      segments: [],
      per_request_price: 0.0045
    })
    const fresh = saleRowFrom(null)
    expect(fillSaleByRatio(fresh, official, 0.1)).toBe(1)
    expect(fresh.per_request_price).toBeCloseTo(0.006, 12)
    expect('per_request_price' in saleRowToRequest(saleRowFrom(null), priceRowFrom(imageEntry({ billing_mode: 'token' })))).toBe(false)
  })

  it('discount fill sets the upstream per-request price', () => {
    const row = emptyPriceRow()
    expect(fillByDiscount(row, priceRowFrom(imageEntry(), 'image'), 0.5)).toBe(1)
    expect(row.per_request_price).toBeCloseTo(0.03, 12)
  })

  it('tiers scale with the official prices (same rule as the backend)', () => {
    expect(perRequestTierPrice(0.03, 0.08, 0.06)).toBeCloseTo(0.04, 12)
    expect(perRequestTierPrice(0.03, 0.08, null)).toBe(0.03)
    expect(perRequestTierPrice(0.03, 0.08, 0)).toBe(0.03)
    const scaled = scaledTierPrices(0.03, priceRowFrom(imageEntry(), 'image'))
    expect(scaled.map((t) => t.label)).toEqual(['1K', '2K'])
    expect(scaled[1].price).toBeCloseTo(0.04, 12)
  })

  it('flags a per-request model without an official per-request price', () => {
    expect(entryHasProblem(imageEntry(), 1 / 15, 0)).toBe(false)
    expect(entryHasProblem(imageEntry({ per_request_price: null }), 1 / 15, 0)).toBe(true)
  })
})

describe('bindByDefault (channel wizard step 3)', () => {
  it('leaves image / video / per-request and extension-endpoint models unchecked', () => {
    expect(bindByDefault({ billing_mode: 'token', extension_endpoints: false })).toBe(true)
    expect(bindByDefault({ billing_mode: 'token' })).toBe(true)
    expect(bindByDefault({ billing_mode: 'image', extension_endpoints: false })).toBe(false)
    expect(bindByDefault({ billing_mode: 'video' })).toBe(false)
    expect(bindByDefault({ billing_mode: 'per_request' })).toBe(false)
    expect(bindByDefault({ billing_mode: 'token', extension_endpoints: true })).toBe(false)
  })
})
