import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ModelPlazaResponse } from '@/api/modelPlaza'

const copyToClipboard = vi.fn().mockResolvedValue(true)
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: { value: false }, copyToClipboard }) }))

const { authState, routeState, replace } = vi.hoisted(() => ({
  authState: { isAuthenticated: false, user: null as { rate_multiplier?: number } | null },
  routeState: { query: {} as Record<string, string> },
  replace: vi.fn()
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }))
vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ replace })
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key) }) }
})

import ModelPlazaContent from '../ModelPlazaContent.vue'

const tokenPricing = (input: number, output: number, cacheRead: number | null = null) => ({
  billing_mode: 'token' as const,
  input_price: input,
  output_price: output,
  cache_write_price: null,
  cache_read_price: cacheRead,
  image_input_price: null,
  image_output_price: null,
  per_request_price: null,
  intervals: []
})

const response: ModelPlazaResponse = {
  description: '**Prices** update weekly',
  models: [
    {
      model_id: 'gpt-5.5', display_name: 'GPT-5.5', vendor: 'openai', billing_mode: 'token', aliases: ['gpt-5.5-sol'],
      pricing: tokenPricing(0.00001, 0.00003, 0.0000025),
      time_pricing: { timezone: 'Asia/Shanghai', weekdays_only: true, periods: [{ start_time: '09:00', end_time: '18:00', multiplier: 1.5 }] }
    },
    { model_id: 'claude-opus-5', display_name: 'Opus 5', vendor: 'anthropic', billing_mode: 'token', pricing: null, aliases: [] },
    { model_id: 'gpt-image-2', display_name: '', vendor: 'openai', billing_mode: 'image', pricing: null, aliases: [] }
  ]
}

function mountContent(props: Partial<{ response: ModelPlazaResponse | null; loading: boolean; error: boolean; embedded: boolean }> = {}) {
  return mount(ModelPlazaContent, {
    props: { response, loading: false, error: false, ...props },
    global: { stubs: { Icon: true, Select: true, SearchInput: true } }
  })
}

const rowIds = (wrapper: ReturnType<typeof mountContent>) =>
  wrapper.findAll('[data-testid="catalog-cell"]').map((cell) => cell.find('.font-mono').text())

describe('ModelPlazaContent', () => {
  beforeEach(() => {
    authState.isAuthenticated = false
    authState.user = null
    routeState.query = {}
    replace.mockClear()
    copyToClipboard.mockClear()
  })

  it('renders one grid cell per listed entry (vendor, then id) with list prices per 1M tokens', async () => {
    const wrapper = mountContent()
    expect(rowIds(wrapper)).toEqual(['claude-opus-5', 'gpt-5.5', 'gpt-image-2'])
    const gpt = wrapper.findAll('[data-testid="catalog-cell"]')[1]
    expect(gpt.text()).toContain('OpenAI')
    expect(gpt.text()).toContain('$10.00')
    expect(gpt.text()).toContain('$30.00')
    // 缓存读与别名在详情抽屉里
    await gpt.trigger('click')
    const detail = document.body.querySelector('[data-testid="model-pricing-detail"]')
    expect(detail?.textContent).toContain('$2.5')
    expect(detail?.textContent).toContain('gpt-5.5-sol')
    expect(wrapper.get('[data-testid="price-unit"]').text()).toContain('userUi.models.priceUnit')
    // 未登录按新用户默认倍率折算（这份夹具没给 default_rate_multiplier，按 1）
    expect(wrapper.get('[data-testid="your-price-note"]').text()).toContain('userUi.models.defaultPriceApplied')
    // 网格单元不是卡片：没有圆角大盒子，只有 hairline 分格
    expect(wrapper.get('[data-testid="catalog-cell"]').classes().join(' ')).not.toMatch(/rounded|shadow/)
    wrapper.unmount()
  })

  it('shows vendor tabs with counts and filters by the selected vendor, writing it to the URL', async () => {
    const wrapper = mountContent()
    const tabs = wrapper.findAll('[data-testid="vendor-tabs"] [role="tab"]')
    expect(tabs.map((tab) => tab.text().replace(/\s+/g, ' '))).toEqual(['userUi.models.allVendors 3', 'Anthropic 1', 'OpenAI 2'])

    await wrapper.get('[data-testid="vendor-tab-openai"]').trigger('click')
    expect(rowIds(wrapper)).toEqual(['gpt-5.5', 'gpt-image-2'])
    expect(replace).toHaveBeenCalledWith({ query: { vendor: 'openai' } })

    await wrapper.get('[data-testid="vendor-tab-all"]').trigger('click')
    expect(rowIds(wrapper)).toHaveLength(3)
    expect(replace).toHaveBeenLastCalledWith({ query: {} })
  })

  it('reads the vendor from the URL', () => {
    routeState.query = { vendor: 'anthropic' }
    const wrapper = mountContent()
    const cells = wrapper.findAll('[data-testid="catalog-cell"]')
    expect(cells).toHaveLength(1)
    expect(cells[0].text()).toContain('claude-opus-5')
  })

  it('shows prices at the account multiplier when signed in', () => {
    authState.isAuthenticated = true
    authState.user = { rate_multiplier: 2 }
    const wrapper = mountContent()
    expect(wrapper.get('[data-testid="your-price-note"]').text()).toContain('userUi.models.yourPriceApplied')
    expect(wrapper.get('[data-testid="your-price-note"]').text()).toContain('"multiplier":"2.00"')
    const gpt = wrapper.findAll('[data-testid="catalog-cell"]')[1]
    expect(gpt.get('[data-testid="price-input"]').text()).toBe('$20.00')
    expect(wrapper.text()).toContain('userUi.models.multiplierNote')
  })

  it('hints time-priced models in the cell footer (the periods are in the drawer)', () => {
    const wrapper = mountContent()
    const extras = wrapper.findAll('[data-testid="price-extras"]').filter((el) => el.text().includes('userUi.models.tags.timePricing'))
    expect(extras).toHaveLength(1)
  })

  it('shows a dash for entries without token prices', () => {
    const wrapper = mountContent()
    expect(wrapper.findAll('[data-testid="catalog-cell"]')[0].text()).toContain('—')
  })

  it('renders the admin markdown note sanitized', () => {
    expect(mountContent().find('.plaza-description').html()).toContain('<strong>Prices</strong>')
  })

  it('copies a model id from the cell button', async () => {
    const wrapper = mountContent()
    await wrapper.findAll('[data-testid="catalog-cell"]')[1].get('[data-testid="copy-model-id"]').trigger('click')
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
