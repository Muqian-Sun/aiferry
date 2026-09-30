import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

const { updateAccountMock, checkMixedChannelRiskMock, authIsSimpleMode } = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  checkMixedChannelRiskMock: vi.fn(),
  authIsSimpleMode: { value: true }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({})
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get isSimpleMode() {
      return authIsSimpleMode.value
    }
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      update: updateAccountMock,
      checkMixedChannelRisk: checkMixedChannelRiskMock
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({})
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

import EditAccountModal from '../EditAccountModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

function buildGrokOAuthAccount(
  credentials: Record<string, unknown> = {},
  extra: Record<string, unknown> = {}
) {
  return {
    id: 5,
    name: 'Grok OAuth',
    notes: '',
    platform: 'grok',
    type: 'oauth',
    credentials: {
      expires_at: '2027-01-01T00:00:00Z',
      token_type: 'Bearer',
      ...credentials
    },
    credentials_status: { has_access_token: true, has_refresh_token: true },
    extra,
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function mountModal(account: any) {
  return mount(EditAccountModal, {
    props: {
      show: true,
      account,
      proxies: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Select: true,
        Icon: true,
        ProxySelector: true,
        ModelWhitelistSelector: true
      }
    }
  })
}

describe('EditAccountModal Grok OAuth upstream config', () => {
  beforeEach(() => {
    authIsSimpleMode.value = true
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
  })

  it('offers no per-account upstream address for Grok OAuth accounts', () => {
    // 成品号只走官方地址；要走中转请按第三方 key 建号
    const wrapper = mountModal(buildGrokOAuthAccount({ base_url: 'https://relay.example.com/v1' }))

    expect(wrapper.find('[data-testid="grok-custom-base-url-toggle"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="grok-custom-base-url-input"]').exists()).toBe(false)
  })

  it('keeps stored header overrides intact on an untouched save', async () => {
    const account = buildGrokOAuthAccount({
      header_override_enabled: true,
      header_overrides: {
        'user-agent': 'grok-pager/0.2.93',
        'x-grok-client-identifier': 'grok-pager',
        'x-grok-client-version': '0.2.93',
        'x-xai-token-auth': 'xai-grok-cli'
      }
    })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await vi.waitFor(() => expect(updateAccountMock).toHaveBeenCalledTimes(1))

    const payload = updateAccountMock.mock.calls[0]?.[1]
    expect(payload?.credentials?.header_override_enabled).toBe(true)
    expect(payload?.credentials?.header_overrides).toEqual({
      'user-agent': 'grok-pager/0.2.93',
      'x-grok-client-identifier': 'grok-pager',
      'x-grok-client-version': '0.2.93',
      'x-xai-token-auth': 'xai-grok-cli'
    })
  })
})
