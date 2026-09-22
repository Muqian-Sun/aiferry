import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'

import HomeView from '../HomeView.vue'

const { getModelPlaza } = vi.hoisted(() => ({ getModelPlaza: vi.fn() }))
vi.mock('@/api/modelPlaza', () => ({ getModelPlaza }))

const { appStore, authStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Fallback site',
    siteLogo: '',
    docUrl: '',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
  },
  authStore: {
    isAuthenticated: false,
    isAdmin: false,
    user: null as { email?: string } | null,
    checkAuth: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('vue-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-router')>()
  return {
    ...actual,
    useRoute: () => ({ path: '/home', fullPath: '/home', name: 'Home', params: {}, meta: {} }),
    useRouter: () => ({ push: vi.fn() }),
  }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountHome(settings: Record<string, unknown> = {}) {
  appStore.cachedPublicSettings = {
    site_name: 'Test site',
    site_subtitle: 'Test subtitle',
    ...settings,
  }

  return mount(HomeView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        LocaleSwitcher: { template: '<div data-testid="locale-switcher" />' },
        Icon: { template: '<span data-testid="icon" />' },
      },
    },
  })
}

function compactDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper.get('[data-testid="compact-home"]').findComponent(RouterLinkStub).props('to')
}

function modelPlazaDestination(wrapper: ReturnType<typeof mountHome>) {
  return wrapper
    .findAllComponents(RouterLinkStub)
    .find((link) => link.props('to') === '/model-plaza')
    ?.props('to')
}

describe('HomeView compact mode', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
    authStore.user = null
    authStore.checkAuth.mockClear()
    appStore.fetchPublicSettings.mockClear()
    getModelPlaza.mockReset().mockRejectedValue(new Error('plaza disabled'))
    localStorage.clear()
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
  })

  it('renders custom HTML ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: '<section id="custom-home">Custom home</section>',
    })

    expect(wrapper.get('#custom-home').text()).toBe('Custom home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('renders custom URL content ahead of compact mode', () => {
    const wrapper = mountHome({
      compact_home_enabled: true,
      home_content: ' https://example.com/home ',
    })

    expect(wrapper.get('iframe').attributes('src')).toBe('https://example.com/home')
    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
  })

  it('treats whitespace-only custom content as empty and selects compact mode', () => {
    const wrapper = mountHome({ compact_home_enabled: true, home_content: ' \n\t ' })

    expect(wrapper.get('[data-testid="compact-home"]').text()).toContain('Test site')
  })

  it.each([undefined, false])('selects the default home when compact mode is %s', (enabled) => {
    const settings = enabled === undefined ? {} : { compact_home_enabled: enabled }
    const wrapper = mountHome(settings)

    expect(wrapper.find('[data-testid="compact-home"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="default-home"]').exists()).toBe(true)
  })

  it('links unauthenticated visitors to login', () => {
    expect(compactDestination(mountHome({ compact_home_enabled: true }))).toBe('/login')
  })

  it('links authenticated users to their dashboard', () => {
    authStore.isAuthenticated = true

    const wrapper = mountHome({ compact_home_enabled: true })
    // 首页只在用户站提供，管理员不会登录用户站，已登录一律去用户仪表盘
    expect(compactDestination(wrapper)).toBe('/usage')
    expect(authStore.checkAuth).toHaveBeenCalledOnce()
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
  })

  // 模型广场没有开关：匿名、登录、紧凑首页都有入口
  it('always links to the model plaza, for anonymous and signed-in visitors alike', () => {
    expect(modelPlazaDestination(mountHome({ compact_home_enabled: true }))).toBe('/model-plaza')
    expect(modelPlazaDestination(mountHome({}))).toBe('/model-plaza')
    authStore.isAuthenticated = true
    expect(modelPlazaDestination(mountHome({ compact_home_enabled: true }))).toBe('/model-plaza')
  })

  it('renders the stats band and the price preview only when the model catalog loads', async () => {
    getModelPlaza.mockResolvedValue({
      description: '',
      models: [
        {
          model_id: 'gpt-5.5', display_name: 'GPT-5.5', vendor: 'openai', billing_mode: 'token', aliases: [],
          pricing: {
            billing_mode: 'token', input_price: 0.00001, output_price: 0.00003, cache_write_price: null, cache_read_price: null,
            image_input_price: null, image_output_price: null, per_request_price: null, intervals: []
          }
        },
        { model_id: 'claude-opus-5', display_name: 'Opus 5', vendor: 'anthropic', billing_mode: 'token', pricing: null, aliases: [] }
      ]
    })
    const wrapper = mountHome({})
    await flushPromises()

    // 数字带从目录算：2 个模型、2 个厂商；价目预览只列有标价的模型
    expect(wrapper.get('[data-testid="home-stats"]').text()).toContain('2')
    expect(wrapper.findAll('[data-testid="home-catalog-row"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="home-catalog"]').text()).toContain('gpt-5.5')
    expect(wrapper.get('[data-testid="home-catalog"]').text()).toContain('$10.00')
  })

  it('lists catalog vendors under the hero: icon for known vendors, text only for the rest', async () => {
    getModelPlaza.mockResolvedValue({
      description: '',
      models: [
        { model_id: 'claude-opus-5', display_name: '', vendor: 'anthropic', billing_mode: 'token', pricing: null, aliases: [] },
        { model_id: 'kimi-k2', display_name: '', vendor: 'some-new-provider', billing_mode: 'token', pricing: null, aliases: [] },
        { model_id: 'claude-sonnet-4-5', display_name: '', vendor: 'anthropic', billing_mode: 'token', pricing: null, aliases: [] }
      ]
    })
    const wrapper = mountHome({})
    await flushPromises()

    const items = wrapper.get('[data-testid="vendor-strip"]').findAll('li')
    expect(items.map((item) => item.text())).toEqual(['Anthropic', 'some-new-provider'])
    expect(items[0].find('svg').exists()).toBe(true)
    expect(items[1].find('svg').exists()).toBe(false)
  })

  it('never renders the vendor strip without a catalog', async () => {
    const wrapper = mountHome({})
    await flushPromises()
    expect(wrapper.find('[data-testid="vendor-strip"]').exists()).toBe(false)
  })

  it('lists the clients that have config snippets in the use-key modal, one entry per client', async () => {
    const wrapper = mountHome({})
    await flushPromises()
    const clients = wrapper.findAll('[data-testid="home-client"]')
    expect(clients.map((c) => c.find('h3').text())).toEqual([
      'keys.useKeyModal.cliTabs.claudeCode',
      'keys.useKeyModal.cliTabs.codexCli',
      'keys.useKeyModal.cliTabs.geminiCli',
      'keys.useKeyModal.cliTabs.grokCli',
      'keys.useKeyModal.cliTabs.opencode'
    ])
    // Codex 的 WebSocket 传输只是同一个客户端的另一种配置，首页不单列
    expect(wrapper.get('[data-testid="home-clients"]').text()).not.toContain('codexCliWs')
  })

  it('sends the clients call-to-action to the keys page, via login when anonymous', async () => {
    const ctaOf = (wrapper: ReturnType<typeof mountHome>) =>
      wrapper.get('[data-testid="home-clients"]').findComponent(RouterLinkStub).props('to')
    expect(ctaOf(mountHome({}))).toEqual({ path: '/login', query: { redirect: '/keys' } })
    authStore.isAuthenticated = true
    expect(ctaOf(mountHome({}))).toBe('/keys')
  })

  it('hides the stats band and the price preview when the catalog is unavailable', async () => {
    const wrapper = mountHome({})
    await flushPromises()
    expect(getModelPlaza).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-testid="home-stats"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="home-catalog"]').exists()).toBe(false)
  })

  it('renders the protocol sample with the site API base URL and switches protocols', async () => {
    const wrapper = mountHome({ api_base_url: 'https://api.example.test/' })
    await flushPromises()
    const sample = wrapper.get('[data-testid="protocol-sample"]')
    expect(sample.text()).toContain('https://api.example.test/v1/messages')
    expect(sample.text()).toContain('userUi.home.quickstart.sample.request')
    expect(sample.text()).toContain('userUi.home.quickstart.sample.response')
    await sample.get('[data-testid="protocol-tab-gemini"]').trigger('click')
    expect(sample.text()).toContain('https://api.example.test/v1beta/models/gemini-3-pro:generateContent')
    expect(sample.text()).toContain('usageMetadata')
  })

  it('lays the landing page out as centred sections: hero, features, quickstart, clients, CTA', async () => {
    const wrapper = mountHome({})
    await flushPromises()
    expect(wrapper.findAll('[data-testid="home-feature"]')).toHaveLength(6)
    expect(wrapper.findAll('[data-testid="home-step"]')).toHaveLength(3)
    expect(wrapper.find('[data-testid="home-quickstart"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="home-client-sdk"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="home-cta"]').exists()).toBe(true)
    // 文档链接卡只在配置了 doc_url 时出现；密钥入口卡恒在
    expect(wrapper.find('[data-testid="home-link-docs"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="home-link-clients"]').exists()).toBe(true)
    expect(mountHome({ doc_url: 'https://docs.example' }).find('[data-testid="home-link-docs"]').exists()).toBe(true)
    // 拿不到目录：眉题、模型段、数字卡都不出现
    expect(wrapper.find('[data-testid="hero-eyebrow"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="home-catalog-section"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="home-stats"]').exists()).toBe(false)
  })
})
