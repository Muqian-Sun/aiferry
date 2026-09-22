import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

import SiteShell from '../SiteShell.vue'

const { appStore, authStore } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: {} as Record<string, unknown>,
    siteName: 'Test site',
    docUrl: '',
    contactInfo: ''
  },
  authStore: { isAuthenticated: false }
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ setReplayCallback: vi.fn() }) }))
vi.mock('@/composables/useOnboardingTour', () => ({ useOnboardingTour: () => ({ replayTour: vi.fn() }) }))
vi.mock('@/composables/usePageTitle', () => ({ usePageTitle: () => ({ title: '', description: '' }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function mountShell(variant: 'public' | 'console' = 'public') {
  return mount(SiteShell, {
    props: { variant },
    global: { stubs: { RouterLink: RouterLinkStub, SiteNav: { template: '<nav />' }, PageHeader: { template: '<header />' } } }
  })
}

/** 横排页脚里链接的文字，按出现顺序（产品 → 帮助 → 协议） */
function linkLabels(wrapper: ReturnType<typeof mountShell>) {
  return wrapper.findAll('[data-testid="footer-links"] > *').map((node) => node.text())
}

function linkTargets(wrapper: ReturnType<typeof mountShell>) {
  return wrapper.findAllComponents(RouterLinkStub).map((link) => link.props('to'))
}

describe('SiteShell public footer', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    appStore.cachedPublicSettings = {}
    appStore.docUrl = ''
    appStore.contactInfo = ''
  })

  it('lays the footer out as one horizontal row: brand, links, copyright', () => {
    const wrapper = mountShell()
    // 没有文档 / 联系方式 / 协议时只剩产品链接，且不再有分栏标题
    expect(linkLabels(wrapper)).toEqual(['userUi.footer.home', 'userUi.nav.pricing', 'userUi.nav.login'])
    expect(linkTargets(wrapper)).toEqual(['/home', '/model-plaza', '/login'])
    expect(wrapper.findAll('[data-testid="site-footer"] h2')).toHaveLength(0)
    expect(wrapper.get('[data-testid="footer-brand"]').text()).toContain('Test site')
    expect(wrapper.get('[data-testid="site-footer"]').text()).toContain(`© ${new Date().getFullYear()}`)
  })

  it('adds sign-up for anonymous visitors when registration is open, and the console when signed in', () => {
    appStore.cachedPublicSettings = { registration_enabled: true }
    expect(linkTargets(mountShell())).toContain('/register')

    authStore.isAuthenticated = true
    const links = linkTargets(mountShell())
    expect(links).toContain('/usage')
    expect(links).not.toContain('/login')
    expect(links).not.toContain('/register')
  })

  it('appends the docs link (external) and the contact text to the same row', () => {
    appStore.cachedPublicSettings = { doc_url: 'https://docs.example', contact_info: 'support@example.test' }
    const wrapper = mountShell()
    expect(linkLabels(wrapper)).toEqual(['userUi.footer.home', 'userUi.nav.pricing', 'userUi.nav.login', 'userUi.nav.docs', 'support@example.test'])
    expect(wrapper.get('[data-testid="footer-links"] a[href="https://docs.example/"]').attributes('target')).toBe('_blank')
  })

  it('appends every login agreement document to the same row', () => {
    appStore.cachedPublicSettings = {
      login_agreement_documents: [
        { id: 'tos', title: 'Terms' },
        { id: 'privacy', title: 'Privacy' }
      ]
    }
    const wrapper = mountShell()
    expect(linkLabels(wrapper)).toEqual(['userUi.footer.home', 'userUi.nav.pricing', 'userUi.nav.login', 'Terms', 'Privacy'])
    expect(linkTargets(wrapper)).toEqual(expect.arrayContaining(['/legal/tos', '/legal/privacy']))
  })

  it('has no footer at all on the console shell', () => {
    expect(mountShell('console').find('[data-testid="site-footer"]').exists()).toBe(false)
  })
})
