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

function columns(wrapper: ReturnType<typeof mountShell>) {
  return wrapper.findAll('[data-testid="site-footer"] h2').map((h) => h.text())
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

  it('shows only the product column when settings carry no docs, contact or legal documents', () => {
    const wrapper = mountShell()
    expect(columns(wrapper)).toEqual(['userUi.footer.product'])
    expect(linkTargets(wrapper)).toEqual(['/home', '/model-plaza', '/login'])
    expect(wrapper.get('[data-testid="site-footer"]').text()).toContain('Test site')
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

  it('builds the help column from the docs URL (external) and the contact text', () => {
    appStore.cachedPublicSettings = { doc_url: 'https://docs.example', contact_info: 'support@example.test' }
    const wrapper = mountShell()
    expect(columns(wrapper)).toEqual(['userUi.footer.product', 'userUi.footer.help'])
    expect(wrapper.get('[data-testid="site-footer"] a[href="https://docs.example/"]').attributes('target')).toBe('_blank')
    expect(wrapper.get('[data-testid="site-footer"]').text()).toContain('support@example.test')
  })

  it('links every login agreement document under the legal column', () => {
    appStore.cachedPublicSettings = {
      login_agreement_documents: [
        { id: 'tos', title: 'Terms' },
        { id: 'privacy', title: 'Privacy' }
      ]
    }
    const wrapper = mountShell()
    expect(columns(wrapper)).toEqual(['userUi.footer.product', 'userUi.footer.legal'])
    expect(linkTargets(wrapper)).toEqual(expect.arrayContaining(['/legal/tos', '/legal/privacy']))
  })

  it('has no footer at all on the console shell', () => {
    expect(mountShell('console').find('[data-testid="site-footer"]').exists()).toBe(false)
  })
})
