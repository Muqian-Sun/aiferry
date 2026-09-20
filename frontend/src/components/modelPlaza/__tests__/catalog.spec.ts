import { describe, expect, it } from 'vitest'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import { buildCatalog, catalogPlatforms, filterCatalog } from '../catalog'

function model(name: string, platform: string, extra: Partial<PlazaModel> = {}): PlazaModel {
  return { name, platform, pricing: null, official_pricing: null, ...extra }
}

function group(id: number, models: PlazaModel[], rate = 1): ModelPlazaGroup {
  return {
    id,
    name: `g${id}`,
    description: '',
    platform: models[0]?.platform ?? 'openai',
    subscription_type: 'standard',
    rate_multiplier: rate,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    long_context_pricing_enabled: false,
    models
  }
}

describe('buildCatalog', () => {
  it('dedupes exact model ids across groups and sorts platforms and ids', () => {
    const catalog = buildCatalog([
      group(1, [model('gpt-5.5', 'openai'), model('claude-opus-5', 'anthropic')]),
      group(2, [model('gpt-5.5', 'azure')], 0.5)
    ])
    expect(catalog.map((c) => c.id)).toEqual(['claude-opus-5', 'gpt-5.5'])
    expect(catalog[1].platforms).toEqual(['azure', 'openai'])
  })

  it('converts official prices to USD per million tokens and never applies group multipliers', () => {
    const [entry] = buildCatalog([
      group(1, [model('gpt-5.5', 'openai', { official_pricing: { input_price: 0.00001, output_price: 0.00003, cache_read_price: null } })], 0.5)
    ])
    expect(entry.official).toEqual({ input: 10, output: 30, cacheRead: null })
  })

  it('leaves official null when the catalog has no price, and fills it from a later group', () => {
    const catalog = buildCatalog([
      group(1, [model('m', 'openai')]),
      group(2, [model('m', 'openai', { official_pricing: { input_price: 0.000001, output_price: null, cache_read_price: null } })])
    ])
    expect(catalog[0].official).toEqual({ input: 1, output: null, cacheRead: null })
    expect(buildCatalog([group(1, [model('x', 'openai')])])[0].official).toBeNull()
  })

  it('carries the billing mode from pricing, defaulting to token', () => {
    const catalog = buildCatalog([
      group(1, [
        model('img', 'openai', { pricing: { billing_mode: 'image' } as PlazaModel['pricing'] }),
        model('txt', 'openai')
      ])
    ])
    expect(catalog.find((c) => c.id === 'img')?.billingMode).toBe('image')
    expect(catalog.find((c) => c.id === 'txt')?.billingMode).toBe('token')
  })
})

describe('filterCatalog / catalogPlatforms', () => {
  const catalog = buildCatalog([
    group(1, [model('gpt-5.5', 'openai'), model('Claude-Opus-5', 'anthropic'), model('gemini-3-pro', 'gemini')])
  ])

  it('matches case-insensitively and by platform', () => {
    expect(filterCatalog(catalog, 'CLAUDE', 'all').map((c) => c.id)).toEqual(['Claude-Opus-5'])
    expect(filterCatalog(catalog, '', 'gemini').map((c) => c.id)).toEqual(['gemini-3-pro'])
    expect(filterCatalog(catalog, 'gpt', 'gemini')).toEqual([])
  })

  it('lists platforms sorted and unique', () => {
    expect(catalogPlatforms(catalog)).toEqual(['anthropic', 'gemini', 'openai'])
  })
})
