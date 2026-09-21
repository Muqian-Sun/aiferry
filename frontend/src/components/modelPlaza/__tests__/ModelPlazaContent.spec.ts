import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ModelPlazaResponse } from '@/api/modelPlaza'

const copyToClipboard = vi.fn().mockResolvedValue(true)
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: { value: false }, copyToClipboard }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAuthenticated: false }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})

import ModelPlazaContent from '../ModelPlazaContent.vue'

const response: ModelPlazaResponse = {
  description: '**Prices** update weekly',
  groups: [
    {
      id: 1, name: 'default', description: '', platform: 'openai', subscription_type: 'standard', rate_multiplier: 0.5,
      peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: false,
      image_rate_independent: false, image_rate_multiplier: 1, long_context_pricing_enabled: false,
      models: [
        { name: 'gpt-5.5', platform: 'openai', pricing: null, official_pricing: { input_price: 0.00001, output_price: 0.00003, cache_read_price: 0.0000025 } },
        { name: 'claude-opus-5', platform: 'anthropic', pricing: null, official_pricing: null }
      ]
    },
    {
      id: 2, name: 'vip', description: '', platform: 'openai', subscription_type: 'standard', rate_multiplier: 0.8,
      peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: true,
      image_rate_independent: false, image_rate_multiplier: 1, long_context_pricing_enabled: false,
      models: [{ name: 'gpt-5.5', platform: 'openai', pricing: null, official_pricing: null }]
    }
  ]
}

function mountContent(props: Partial<{ response: ModelPlazaResponse | null; loading: boolean; error: boolean; embedded: boolean }> = {}) {
  return mount(ModelPlazaContent, {
    props: { response, loading: false, error: false, ...props },
    global: { stubs: { Icon: true, Select: true, SearchInput: true } }
  })
}

describe('ModelPlazaContent (model-first catalog)', () => {
  it('renders one row per exact model id regardless of how many groups list it, with official prices per 1M tokens', () => {
    const wrapper = mountContent()
    const rows = wrapper.findAll('[data-testid="catalog-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('claude-opus-5')
    expect(rows[1].text()).toContain('gpt-5.5')
    expect(rows[1].text()).toContain('$10.00')
    expect(rows[1].text()).toContain('$30.00')
    expect(rows[1].text()).toContain('$2.5')
    // 分组倍率 0.5 / 0.8 不能出现在页面上
    expect(wrapper.text()).not.toMatch(/×\s?0\.[58]/)
    expect(wrapper.find('[data-testid="catalog-count"]').text()).toContain('"count":2')
  })

  it('shows a dash for models the official catalog does not cover', () => {
    const wrapper = mountContent()
    expect(wrapper.findAll('[data-testid="catalog-row"]')[0].text()).toContain('—')
  })

  it('renders the admin markdown note sanitized', () => {
    const wrapper = mountContent()
    expect(wrapper.find('.plaza-description').html()).toContain('<strong>Prices</strong>')
  })

  it('copies a model id from the row button', async () => {
    const wrapper = mountContent()
    await wrapper.findAll('[data-testid="catalog-row"]')[1].find('button').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('gpt-5.5')
  })

  it('draws its own title only in the public form', () => {
    expect(mountContent({ embedded: false }).find('h1').exists()).toBe(true)
    expect(mountContent({ embedded: true }).find('h1').exists()).toBe(false)
  })

  it('has loading, error and empty states', () => {
    expect(mountContent({ loading: true }).find('[data-testid="status-loading"]').exists()).toBe(true)
    expect(mountContent({ error: true }).find('[data-testid="status-error"]').exists()).toBe(true)
    expect(mountContent({ response: { description: '', groups: [] } }).find('[data-testid="status-empty"]').exists()).toBe(true)
  })
})
