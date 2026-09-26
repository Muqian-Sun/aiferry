import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileView from '@/views/user/ProfileView.vue'

const {
  fetchPublicSettingsMock,
  refreshUserMock,
  authState
} = vi.hoisted(() => ({
  fetchPublicSettingsMock: vi.fn(),
  refreshUserMock: vi.fn(),
  authState: {
    user: null as Record<string, unknown> | null,
    refreshUser: vi.fn()
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    fetchPublicSettings: fetchPublicSettingsMock
  })
}))

vi.mock('@/utils/format', () => ({
  formatDate: () => 'April 2026'
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('ProfileView', () => {
  beforeEach(() => {
    refreshUserMock.mockReset()
    fetchPublicSettingsMock.mockReset()
    refreshUserMock.mockResolvedValue(undefined)
    authState.refreshUser = refreshUserMock
    authState.user = {
      id: 1,
      username: 'alice',
      email: 'alice@example.com',
      role: 'user',
      balance: 10,
      concurrency: 2,
      status: 'active',
      allowed_groups: null,
      balance_notify_enabled: true,
      balance_notify_threshold: null,
      balance_notify_extra_emails: [],
      created_at: '2026-04-20T00:00:00Z',
      updated_at: '2026-04-20T00:00:00Z'
    }
    fetchPublicSettingsMock.mockResolvedValue({
      contact_info: '',
      balance_low_notify_enabled: false,
      balance_low_notify_threshold: 0,
      wechat_oauth_enabled: true,
      wechat_oauth_open_enabled: true,
      wechat_oauth_mp_enabled: false,
    })
  })

  const mountSection = (section: 'profile' | 'security' | 'notifications') => mount(ProfileView, {
      props: { section },
      global: {
        stubs: {
          SiteShell: { template: '<div><slot /></div>' },
          ProfileInfoCard: { template: '<div data-testid="profile-info-card" />' },
          ProfileBalanceNotifyCard: { template: '<div data-testid="profile-balance-notify-card" />' },
          ProfilePasswordForm: { template: '<div data-testid="profile-password-form" />' },
          ProfileTotpCard: { template: '<div data-testid="profile-totp-card" />' },
          ProfilePasskeyCard: { template: '<div data-testid="profile-passkey-card" />' },
          ProfileIdentityBindingsSection: { template: '<div data-testid="profile-identity-bindings" />' },
          Icon: true
        }
      }
    })

  it('renders one section per sub-page without separate stat cards', async () => {
    const profile = mountSection('profile')
    await flushPromises()
    expect(profile.findAll('.stat-card')).toHaveLength(0)
    expect(profile.get('[data-testid="profile-shell"]').html()).toContain('profile-info-card')
    expect(profile.get('[data-testid="profile-shell"]').html()).not.toContain('profile-password-form')

    const security = mountSection('security')
    await flushPromises()
    const shell = security.get('[data-testid="profile-shell"]').html()
    expect(shell).toContain('profile-password-form')
    expect(shell).toContain('profile-totp-card')
    expect(shell).toContain('profile-passkey-card')
    expect(shell).not.toContain('profile-info-card')
    // 登录方式绑定属于「安全」子页，与密码 / 双因素 / Passkey 同列
    expect(security.get('[data-testid="profile-auth-bindings-panel"]').html()).toContain('profile-identity-bindings')
  })
})
