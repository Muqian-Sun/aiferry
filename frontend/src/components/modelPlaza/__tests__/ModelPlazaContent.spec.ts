import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ModelPlazaResponse } from '@/api/modelPlaza'

const copyToClipboard = vi.fn().mockResolvedValue(true)
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: { value: false }, copyToClipboard }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})

import ModelPlazaContent from '../ModelPlazaContent.vue'

const response: ModelPlazaResponse = {
  description: '**Prices** update weekly',
  models: [
    {
      model_id: 'gpt-5.5', display_name: 'GPT-5.5', vendor: 'openai', billing_mode: 'token', aliases: ['gpt-5.5-sol'],
      pricing: {
        billing_mode: 'token', input_price: 0.00001, output_price: 0.00003, cache_write_price: null, cache_read_price: 0.0000025,
        image_input_price: null, image_output_price: null, per_request_price: null, intervals: []
      }
    },
    { model_id: 'claude-opus-5', display_name: 'Opus 5', vendor: 'anthropic', billing_mode: 'token', pricing: null, aliases: [] }
  ]
}

function mountContent(props: Partial<{ response: ModelPlazaResponse | null; loading: boolean; error: boolean; embedded: boolean }> = {}) {
  return mount(ModelPlazaContent, {
    props: { response, loading: false, error: false, ...props },
    global: { stubs: { Icon: true, Select: true, SearchInput: true } }
  })
}

describe('ModelPlazaContent (model-first catalog)', () => {
  it('renders one row per listed entry (vendor, then id) with list prices per 1M tokens', () => {
    const wrapper = mountContent()
    const rows = wrapper.findAll('[data-testid="catalog-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('claude-opus-5')
    expect(rows[0].text()).toContain('Anthropic')
    expect(rows[1].text()).toContain('gpt-5.5')
    expect(rows[1].text()).toContain('OpenAI')
    expect(rows[1].text()).toContain('$10.00')
    expect(rows[1].text()).toContain('$30.00')
    expect(rows[1].text()).toContain('$2.5')
    expect(wrapper.find('[data-testid="catalog-count"]').text()).toContain('"count":2')
    expect(wrapper.text()).toContain('userUi.models.listPrice')
  })

  it('shows a dash for entries without token prices', () => {
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
    expect(mountContent({ response: { description: '', models: [] } }).find('[data-testid="status-empty"]').exists()).toBe(true)
  })
})
