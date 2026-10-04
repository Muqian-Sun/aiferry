import { describe, expect, it } from 'vitest'
import type { PlazaModel } from '@/api/modelPlaza'
import {
  buildCatalog,
  catalogBillingModes,
  catalogVendors,
  countByVendor,
  filterCatalog,
  formatCatalogPrice,
  formatTimePricing,
  vendorLabel
} from '../catalog'

function model(id: string, vendor: string, extra: Partial<PlazaModel> = {}): PlazaModel {
  return { model_id: id, display_name: id, vendor, billing_mode: 'token', pricing: null, aliases: [], ...extra }
}

function pricing(input: number | null, output: number | null, cacheRead: number | null = null): PlazaModel['pricing'] {
  return {
    billing_mode: 'token',
    input_price: input,
    output_price: output,
    cache_write_price: null,
    cache_read_price: cacheRead,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: []
  }
}

describe('buildCatalog', () => {
  it('keeps one row per catalog entry, sorted by vendor then id', () => {
    const catalog = buildCatalog([model('gpt-5.6', 'openai'), model('claude-opus-5', 'anthropic'), model('gpt-5.5', 'openai')])
    expect(catalog.map((c) => c.id)).toEqual(['claude-opus-5', 'gpt-5.5', 'gpt-5.6'])
    expect(catalog.map((c) => c.vendor)).toEqual(['anthropic', 'openai', 'openai'])
  })

  it('converts list prices from USD per token to USD per million tokens', () => {
    const [entry] = buildCatalog([model('gpt-5.5', 'openai', { pricing: pricing(0.00001, 0.00003, 0.0000025) })])
    expect(entry.price).toEqual({ input: 10, output: 30, cacheWrite: null, cacheWrite1h: null, cacheRead: 2.5, imageInput: null, imageOutput: null, imageCacheRead: null, audioInput: null, audioOutput: null })
  })

  it('leaves price null when the entry carries no token prices (per-request models)', () => {
    expect(buildCatalog([model('img', 'openai', { billing_mode: 'per_request', pricing: pricing(null, null) })])[0].price).toBeNull()
    expect(buildCatalog([model('x', 'openai')])[0].price).toBeNull()
  })

  it('carries billing mode, display name, aliases and time-pricing presence', () => {
    const [entry] = buildCatalog([
      model('gpt-5.6', 'openai', {
        display_name: 'GPT-5.6',
        billing_mode: 'image',
        aliases: ['gpt-5.6-sol'],
        time_pricing: { timezone: 'Asia/Shanghai', periods: [{ start_time: '09:00', end_time: '18:00', multiplier: 1.5 }] }
      })
    ])
    expect(entry.displayName).toBe('GPT-5.6')
    expect(entry.billingMode).toBe('image')
    expect(entry.aliases).toEqual(['gpt-5.6-sol'])
    expect(entry.timePricing?.periods).toHaveLength(1)
    expect(buildCatalog([model('plain', 'openai', { display_name: '' })])[0]).toMatchObject({ displayName: 'plain', timePricing: null })
  })
})

describe('filterCatalog / catalogVendors', () => {
  const catalog = buildCatalog([
    model('gpt-5.5', 'openai'),
    model('Claude-Opus-5', 'anthropic', { display_name: 'Opus 5' }),
    model('gemini-3-pro', 'gemini', { aliases: ['g3p'] })
  ])

  it('matches id, display name and aliases case-insensitively, and vendor exactly', () => {
    expect(filterCatalog(catalog, 'CLAUDE', 'all').map((c) => c.id)).toEqual(['Claude-Opus-5'])
    expect(filterCatalog(catalog, 'opus 5', 'all').map((c) => c.id)).toEqual(['Claude-Opus-5'])
    expect(filterCatalog(catalog, 'G3P', 'all').map((c) => c.id)).toEqual(['gemini-3-pro'])
    expect(filterCatalog(catalog, '', 'gemini').map((c) => c.id)).toEqual(['gemini-3-pro'])
    expect(filterCatalog(catalog, 'gpt', 'gemini')).toEqual([])
  })

  it('filters by billing mode and counts entries per vendor', () => {
    const withImage = [...catalog, ...buildCatalog([model('img', 'openai', { billing_mode: 'image' })])]
    expect(filterCatalog(withImage, '', 'all', 'image').map((c) => c.id)).toEqual(['img'])
    expect(filterCatalog(withImage, '', 'openai', 'token').map((c) => c.id)).toEqual(['gpt-5.5'])
    expect(catalogBillingModes(withImage)).toEqual(['image', 'token'])
    expect([...countByVendor(withImage)]).toEqual([['anthropic', 1], ['gemini', 1], ['openai', 2]])
  })

  it('lists vendors sorted, unique and without blanks', () => {
    expect(catalogVendors([...catalog, ...buildCatalog([model('nameless', '')])])).toEqual(['anthropic', 'gemini', 'openai'])
  })
})

describe('formatTimePricing', () => {
  it('spells out periods, timezone and the weekdays-only scope', () => {
    expect(
      formatTimePricing({ timezone: 'Asia/Shanghai', weekdays_only: true, periods: [{ start_time: '09:00', end_time: '18:00', multiplier: 1.5 }] }, 'weekdays')
    ).toBe('09:00–18:00 ×1.5 (Asia/Shanghai, weekdays)')
    expect(
      formatTimePricing({ timezone: 'UTC', periods: [{ start_time: '00:00', end_time: '06:00', multiplier: 0.5 }, { start_time: '12:00', end_time: '14:00', multiplier: 2 }] }, 'weekdays')
    ).toBe('00:00–06:00 ×0.5 · 12:00–14:00 ×2 (UTC)')
  })
})

describe('vendorLabel / formatCatalogPrice', () => {
  it('maps known vendors to brand spelling, passes unknown through, dashes the blank', () => {
    expect(vendorLabel('openai')).toBe('OpenAI')
    expect(vendorLabel('xai')).toBe('xAI')
    expect(vendorLabel('vertex_ai-language-models')).toBe('Google')
    expect(vendorLabel('some-new-provider')).toBe('some-new-provider')
    expect(vendorLabel('')).toBe('—')
  })

  it('formats per-million prices by magnitude', () => {
    expect(formatCatalogPrice(null)).toBe('—')
    expect(formatCatalogPrice(150)).toBe('$150')
    expect(formatCatalogPrice(2.5)).toBe('$2.50')
    expect(formatCatalogPrice(0.12345)).toBe('$0.1235')
  })
})
