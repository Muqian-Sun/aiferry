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

  it('renders the stats band from the catalog, each number as a CSS count-up plus an sr-only value', async () => {
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

    // 模型 2 / 厂商 2 从目录算；协议 4 / 客户端 5 是产品事实；数字段整体 v-reveal，跳数由 .count-up 的 --count-to 驱动
    const stats = wrapper.get('[data-testid="home-stats"]')
    expect(stats.attributes('data-reveal')).toBe('single')
    // 数字在上、名目在下，且不装进卡片
    expect(stats.classes()).not.toContain('sheet-card')
    expect(stats.get('div').element.firstElementChild?.tagName).toBe('DD')
    expect(stats.findAll('[data-testid="home-stat-value"]').map((v) => v.text())).toEqual(['2', '2', '4', '5'])
    const counters = stats.findAll('.count-up')
    expect(counters.map((c) => (c.element as HTMLElement).style.getPropertyValue('--count-to'))).toEqual(['2', '2', '4', '5'])
    expect(counters.every((c) => c.attributes('aria-hidden') === 'true')).toBe(true)
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

  it('splits the hero into copy + icon cloud only when the catalog has vendors with icons', async () => {
    // 有图标的厂商（anthropic）→ 出云，文字列 lg 起左对齐，厂商行只留给小屏
    getModelPlaza.mockResolvedValue({
      description: '',
      models: [
        { model_id: 'claude-opus-5', display_name: '', vendor: 'anthropic', billing_mode: 'token', pricing: null, aliases: [] },
        { model_id: 'kimi-k2', display_name: '', vendor: 'some-new-provider', billing_mode: 'token', pricing: null, aliases: [] }
      ]
    })
    const withCloud = mountHome({})
    await flushPromises()
    const cloud = withCloud.get('[data-testid="vendor-cloud"]')
    // 云里只放有图标的厂商：每块瓷砖都有 svg
    const tiles = cloud.findAll('[data-testid="vendor-cloud-tile"]')
    expect(tiles.length).toBeGreaterThanOrEqual(6)
    expect(tiles.every((tile) => tile.find('svg').exists())).toBe(true)
    // 图标用厂商品牌色，不是 currentColor
    expect(tiles.every((tile) => /^#/.test(tile.get('svg').attributes('fill') ?? ''))).toBe(true)
    // 各自飘：每块自带周期与相位，且不预设 transform（没有鼠标视差）
    expect(tiles.every((tile) => {
      const style = (tile.element as HTMLElement).style
      return style.getPropertyValue('--drift-dur') !== '' && style.transform === ''
    })).toBe(true)
    expect(withCloud.get('[data-testid="hero-copy"]').classes()).toContain('lg:text-left')
    expect(withCloud.get('[data-testid="vendor-strip"]').element.parentElement?.classList.contains('lg:hidden')).toBe(true)

    // 只有没图标的厂商 → 不出云，首屏居中
    getModelPlaza.mockResolvedValue({
      description: '',
      models: [{ model_id: 'kimi-k2', display_name: '', vendor: 'some-new-provider', billing_mode: 'token', pricing: null, aliases: [] }]
    })
    const centred = mountHome({})
    await flushPromises()
    expect(centred.find('[data-testid="vendor-cloud"]').exists()).toBe(false)
    expect(centred.get('[data-testid="hero-copy"]').classes()).not.toContain('lg:text-left')
    expect(centred.get('[data-testid="vendor-strip"]').element.parentElement?.classList.contains('lg:hidden')).toBe(false)
  })

  it('hides the stats band when the catalog is unavailable, leaving hero + features', async () => {
    const wrapper = mountHome({})
    await flushPromises()
    expect(getModelPlaza).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-testid="home-stats"]').exists()).toBe(false)
    const blocks = Array.from(wrapper.get('[data-testid="default-home"]').element.children)
    expect(blocks.map((b) => b.getAttribute('data-testid') || b.className.split(' ')[0])).toEqual(['home-hero', 'home-features'])
  })

  it('marks every section for scroll reveal: headers reveal as a unit, grids stagger their children', async () => {
    const wrapper = mountHome({})
    await flushPromises()
    // 首屏文字列与特色网格是 stagger 容器，子元素编号从 0 起；特色段标题整块渐现
    const heroCopy = wrapper.get('[data-testid="hero-copy"]').element as HTMLElement
    expect(heroCopy.getAttribute('data-reveal')).toBe('stagger')
    expect((heroCopy.children[0] as HTMLElement).style.getPropertyValue('--reveal-i')).toBe('0')
    // 特色是整幅一行一行地渐现：每行自己一个 single，外层 ul 不再是 stagger 容器（两者并存会叠两次动画）
    const rows = wrapper.findAll('[data-testid="home-feature"]')
    expect(rows.every((row) => row.attributes('data-reveal') === 'single')).toBe(true)
    expect((rows[0].element.parentElement as HTMLElement).hasAttribute('data-reveal')).toBe(false)
    expect(wrapper.get('[data-testid="home-features"]').element.querySelector('[data-reveal="single"] .section-title')).not.toBeNull()
  })

  it('keeps the body to three blocks: animated hero, numbers, five feature rows', async () => {
    const wrapper = mountHome({})
    await flushPromises()
    // 标题第二行走流动渐变原语（12ai 式），第一行留纯色
    const accent = wrapper.get('[data-testid="hero-title-accent"]')
    expect(accent.classes()).toContain('text-flow')
    expect(accent.text()).toBe('userUi.home.hero.titleAccent')
    // 五条特色各一幅示意图：透传 / 不换模型 / 缓存 / 不记录 / 不出售；整幅一行，图文左右交错，不用卡片
    const features = wrapper.findAll('[data-testid="home-feature"]')
    expect(features).toHaveLength(5)
    for (const kind of ['passthrough', 'failover', 'cache', 'privacy', 'noSale']) {
      expect(wrapper.find(`[data-testid="home-figure-${kind}"]`).exists()).toBe(true)
    }
    expect(features.every((row) => !row.classes().includes('sheet-card'))).toBe(true)
    // 奇数行把文字挪到右边（图在左），偶数行相反
    const copyOrder = features.map((row) => row.get('div').classes().includes('lg:order-2'))
    expect(copyOrder).toEqual([false, true, false, true, false])
    // 拿不到目录：眉题与数字段不出现
    expect(wrapper.find('[data-testid="hero-eyebrow"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="home-stats"]').exists()).toBe(false)

    // 有目录：正文恰好三块，顺序 首屏 → 数字 → 特色
    getModelPlaza.mockResolvedValue({
      description: '',
      models: [{ model_id: 'claude-opus-5', display_name: '', vendor: 'anthropic', billing_mode: 'token', pricing: null, aliases: [] }]
    })
    const full = mountHome({})
    await flushPromises()
    const blocks = Array.from(full.get('[data-testid="default-home"]').element.children)
    expect(blocks).toHaveLength(3)
    expect(blocks[0].classList.contains('home-hero')).toBe(true)
    expect(blocks[1].getAttribute('data-testid')).toBe('home-stats-section')
    expect(blocks[2].getAttribute('data-testid')).toBe('home-features')
  })
})
