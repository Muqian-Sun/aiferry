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

  it('keeps the footer to one row: copyright only when no agreement documents are configured', () => {
    const wrapper = mountShell()
    expect(wrapper.find('[data-testid="footer-brand"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="footer-links"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="footer-legal"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="site-footer"]').text()).toContain(`© ${new Date().getFullYear()} Test site`)
  })

  it('lists every login agreement document in the bottom row next to the copyright', () => {
    appStore.cachedPublicSettings = {
      login_agreement_documents: [
        { id: 'tos', title: 'Terms' },
        { id: 'privacy', title: 'Privacy' }
      ]
    }
    const wrapper = mountShell()
    expect(wrapper.findAll('[data-testid="footer-legal"] a').map((a) => a.text())).toEqual(['Terms', 'Privacy'])
    expect(linkTargets(wrapper)).toEqual(expect.arrayContaining(['/legal/tos', '/legal/privacy']))
  })

  it('has no footer at all on the console shell', () => {
    expect(mountShell('console').find('[data-testid="site-footer"]').exists()).toBe(false)
  })
})
