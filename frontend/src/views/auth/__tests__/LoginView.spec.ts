import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LoginView from '@/views/auth/LoginView.vue'

const { getPublicSettingsMock, pushMock, site } = vi.hoisted(() => ({
  getPublicSettingsMock: vi.fn(),
  pushMock: vi.fn(),
  site: { admin: false }
}))

vi.mock('@/app/site', () => ({
  get IS_ADMIN_SITE() {
    return site.admin
  },
  get APP_SITE() {
    return site.admin ? 'admin' : 'user'
  }
}))

const publicSettings = {
  registration_enabled: true,
  turnstile_enabled: false,
  turnstile_site_key: '',
  tencent_captcha_enabled: false,
  tencent_captcha_app_id: '',
  aliyun_captcha_enabled: false,
  aliyun_captcha_scene_id: '',
  aliyun_captcha_prefix: '',
  wechat_oauth_enabled: false,
  backend_mode_enabled: false,
  github_oauth_enabled: false,
  google_oauth_enabled: false,
  password_reset_enabled: false,
  passkey_enabled: false,
  login_agreement_enabled: false,
  login_agreement_documents: []
}

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: pushMock,
    currentRoute: { value: { query: {} } }
  })
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key
    }
  }),
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    login: vi.fn(),
    loginWithPasskey: vi.fn(),
    login2FA: vi.fn()
  }),
  useAppStore: () => ({})
}))

vi.mock('@/api/auth', () => ({
  buildOAuthLoginStartURL: vi.fn(),
  getPublicSettings: (...args: unknown[]) => getPublicSettingsMock(...args),
  isTotp2FARequired: vi.fn(() => false),
  isWeChatWebOAuthEnabled: vi.fn(() => false),
  startOAuthLogin: vi.fn()
}))

function mountLogin() {
  return mount(LoginView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        EmailOAuthButtons: true,
        Icon: true,
        LoginAgreementPrompt: true,
        RouterLink: { template: '<a><slot /></a>' },
        TotpLoginModal: true,
        TurnstileWidget: true,
        WechatOAuthSection: true,
        transition: false
      }
    }
  })
}

describe('LoginView registration entry', () => {
  beforeEach(() => {
    getPublicSettingsMock.mockReset()
    pushMock.mockReset()
    getPublicSettingsMock.mockResolvedValue(publicSettings)
  })

  it('shows the registration entry when registration is enabled', async () => {
    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).toContain('auth.signUp')
  })

  it('hides the registration entry when registration is disabled', async () => {
    getPublicSettingsMock.mockResolvedValueOnce({
      ...publicSettings,
      registration_enabled: false
    })

    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).not.toContain('auth.signUp')
  })
})

describe('LoginView self-service entries by site', () => {
  const allEntriesOn = {
    ...publicSettings,
    registration_enabled: true,
    password_reset_enabled: true,
    github_oauth_enabled: true,
    passkey_enabled: true
  }

  beforeEach(() => {
    getPublicSettingsMock.mockReset()
    getPublicSettingsMock.mockResolvedValue(allEntriesOn)
    site.admin = false
    ;(window as unknown as { PublicKeyCredential?: unknown }).PublicKeyCredential = function PublicKeyCredential() {}
  })

  it('admin console offers only password and passkey sign-in', async () => {
    site.admin = true
    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).toContain('auth.passkeySignIn')
    expect(wrapper.text()).not.toContain('auth.signUp')
    expect(wrapper.text()).not.toContain('auth.forgotPassword')
    expect(wrapper.findComponent({ name: 'EmailOAuthButtons' }).exists()).toBe(false)
  })

  it('user site keeps registration, password reset and third-party sign-in', async () => {
    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).toContain('auth.signUp')
    expect(wrapper.text()).toContain('auth.forgotPassword')
    expect(wrapper.findComponent({ name: 'EmailOAuthButtons' }).exists()).toBe(true)
  })

  it('does not reveal third-party sign-in alongside the passkey entry in backend mode', async () => {
    getPublicSettingsMock.mockResolvedValue({ ...allEntriesOn, backend_mode_enabled: true })
    const wrapper = mountLogin()
    await flushPromises()

    expect(wrapper.text()).toContain('auth.passkeySignIn')
    expect(wrapper.findComponent({ name: 'EmailOAuthButtons' }).exists()).toBe(false)
  })
})
