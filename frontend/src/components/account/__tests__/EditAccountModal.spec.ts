import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

const { updateAccountMock, authIsSimpleMode, getProtocolDefaultsMock, showErrorMock } = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  authIsSimpleMode: { value: true },
  getProtocolDefaultsMock: vi.fn(),
  showErrorMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
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
      update: updateAccountMock
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
  getAntigravityDefaultModelMapping: vi.fn(),
  accountsAPI: { getProtocolDefaults: getProtocolDefaultsMock }
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

import { adminAPI } from '@/api/admin'
import EditAccountModal from '../EditAccountModal.vue'
import { resetProtocolDefaultsCacheForTest } from '../protocolEndpoints'

// 与后端 GET /admin/accounts/protocol-defaults 同形（节选）。
const PROTOCOL_DEFAULTS = {
  protocols: ['anthropic', 'chat_completions', 'responses', 'gemini'],
  defaults: {
    openai: { default: { chat_completions: 'https://api.openai.com', responses: 'https://api.openai.com' } },
    kimi: {
      default: {
        anthropic: 'https://api.moonshot.cn/anthropic',
        chat_completions: 'https://api.moonshot.cn/v1',
        responses: 'https://api.moonshot.cn/v1'
      },
      coding: {
        anthropic: 'https://api.kimi.com/coding',
        chat_completions: 'https://api.kimi.com/coding/v1',
        responses: 'https://api.kimi.com/coding/v1'
      }
    }
  }
}

beforeEach(() => {
  resetProtocolDefaultsCacheForTest()
  getProtocolDefaultsMock.mockReset().mockResolvedValue(PROTOCOL_DEFAULTS)
  showErrorMock.mockReset()
})

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

const ModelWhitelistSelectorStub = defineComponent({
  name: 'ModelWhitelistSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <div>
      <button
        type="button"
        data-testid="rewrite-to-snapshot"
        @click="$emit('update:modelValue', ['gpt-5.2-2025-12-11'])"
      >
        rewrite
      </button>
      <span data-testid="model-whitelist-value">
        {{ Array.isArray(modelValue) ? modelValue.join(',') : '' }}
      </span>
    </div>
  `
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: ''
    },
    options: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      v-bind="$attrs"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

function buildAccount() {
  return {
    id: 1,
    name: 'OpenAI Key',
    notes: '',
    platform: 'openai',
    type: 'apikey',
    credentials: {
      api_key: 'sk-test',
      model_mapping: {
        'gpt-5.2': 'gpt-5.2'
      }
    },
    protocol_endpoints: { chat_completions: 'https://api.openai.com', responses: 'https://api.openai.com' },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildOpenAISparkShadowAccount() {
  const account = buildAccount()
  return {
    ...account,
    id: 4,
    name: 'OpenAI Spark Shadow',
    type: 'oauth',
    parent_account_id: 1,
    credentials: {
      access_token: 'parent-access-token',
      refresh_token: 'parent-refresh-token',
      api_key: 'sk-parent',
      base_url: 'https://api.openai.com',
      model_mapping: {
        'gpt-5.3-codex-spark': 'gpt-5.3-codex-spark'
      },
      compact_model_mapping: {
        'gpt-5.3-codex-spark': 'gpt-5.3-codex-spark-compact'
      }
    },
  } as any
}

function buildVertexAccount() {
  return {
    id: 2,
    name: 'Vertex SA',
    notes: '',
    platform: 'gemini',
    type: 'service_account',
    credentials: {
      service_account_json: '{"type":"service_account","client_email":"sa@example.iam.gserviceaccount.com","private_key":"-----BEGIN PRIVATE KEY-----\\nMIIE\\n-----END PRIVATE KEY-----\\n"}',
      project_id: 'demo-project',
      client_email: 'sa@example.iam.gserviceaccount.com',
      location: 'us-central1',
      tier_id: 'vertex'
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildAntigravityAccount(projectId = 'configured-project') {
  return {
    id: 3,
    name: 'Antigravity OAuth',
    notes: '',
    platform: 'antigravity',
    type: 'oauth',
    credentials: {
      antigravity_project_id: projectId,
      model_mapping: {
        'gemini-2.5-flash': 'gemini-2.5-flash'
      }
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildGrokOAuthAccount() {
  return {
    id: 5,
    name: 'Grok OAuth',
    notes: '',
    platform: 'grok',
    type: 'oauth',
    credentials: {
      refresh_token: 'grok-rt',
      base_url: 'https://api.x.ai/v1',
      model_mapping: {
        'grok-latest': 'grok-4.3'
      }
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildGrokAPIKeyAccount() {
  return {
    ...buildAccount(),
    id: 6,
    name: 'Grok API Key',
    platform: 'grok',
    credentials: {},
    protocol_endpoints: { chat_completions: 'https://api.x.ai/v1', responses: 'https://api.x.ai/v1' },
    credentials_status: { has_api_key: true },
    concurrency: 2
  } as any
}

function buildOpenAISetupTokenAccount() {
  return {
    ...buildAccount(),
    type: 'setup-token',
    extra: {
      openai_oauth_responses_websockets_v2_mode: 'ctx_pool',
      openai_oauth_responses_websockets_v2_enabled: true
    }
  } as any
}

function buildOpenAIOAuthParentAccount() {
  return {
    ...buildAccount(),
    id: 7,
    name: 'OpenAI OAuth Parent',
    type: 'oauth',
    parent_account_id: null,
    credentials: { access_token: 'oauth-token' },
    extra: {}
  } as any
}

function mountModal(account = buildAccount(), catalogEntries?: ModelCatalogEntry[]) {
  return mount(EditAccountModal, {
    props: {
      show: true,
      account,
      proxies: [],
      catalogEntries
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Select: SelectStub,
        Icon: true,
        ProxySelector: true,
        ModelWhitelistSelector: ModelWhitelistSelectorStub
      }
    }
  })
}

describe('EditAccountModal', () => {
  beforeEach(() => {
    authIsSimpleMode.value = true
  })

  afterEach(() => vi.useRealTimers())

  it('sets expiry presets from now instead of extending the saved expiry', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2028-02-29T12:34:00'))
    const account = buildAccount()
    account.expires_at = new Date('2030-06-15T09:00:00').getTime() / 1000
    updateAccountMock.mockReset().mockResolvedValue(account)
    const wrapper = mountModal(account)
    const input = wrapper.get<HTMLInputElement>('input[type="datetime-local"]')

    for (const [label, expected] of [
      ['payment.oneMonth', '2028-03-29T12:34'],
      ['payment.oneYear', '2029-02-28T12:34'],
    ]) {
      const button = wrapper.findAll('button').find((candidate) => candidate.text() === label)!
      expect(button.attributes('type')).toBe('button')
      await button.trigger('click')
      expect(input.element.value).toBe(expected)
      expect(updateAccountMock).not.toHaveBeenCalled()
    }

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock.mock.calls[0]?.[1]?.expires_at).toBe(new Date('2029-02-28T12:34:00').getTime() / 1000)
    wrapper.unmount()
  })

  it('can clear a selected expiry preset before saving the account', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset().mockResolvedValue(account)
    const wrapper = mountModal(account)
    const button = wrapper.findAll('button').find((candidate) => candidate.text() === 'payment.oneYear')!
    await button.trigger('click')
    const input = wrapper.get<HTMLInputElement>('input[type="datetime-local"]')
    expect(input.element.value).not.toBe('')
    await input.setValue('')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock.mock.calls[0]?.[1]?.expires_at).toBe(0)
    wrapper.unmount()
  })

  it('reopening the same account rehydrates the OpenAI whitelist from props', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2-2025-12-11')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2': 'gpt-5.2'
    })
  })

  it('preserves OpenCode Zen account type and endpoints on submit', async () => {
    const account = buildAccount()
    account.platform = 'opencode_go'
    account.protocol_endpoints = {
      chat_completions: 'https://opencode.ai/zen/v1',
      anthropic: 'https://opencode.ai/zen',
      responses: 'https://opencode.ai/zen/v1'
    }
    account.credentials = {
      api_key: 'sk-opencode',
      account_mode: 'zen'
    }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual(account.protocol_endpoints)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toMatchObject({
      account_mode: 'zen'
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('protocol_rules')
  })

  it('treats a legacy OpenCode account without account_mode as GO', async () => {
    const account = buildAccount()
    account.platform = 'opencode_go'
    account.protocol_endpoints = { chat_completions: 'https://opencode.ai/zen/go/v1' }
    account.credentials = {
      api_key: 'sk-opencode'
    }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toMatchObject({ account_mode: 'go' })
  })

  it('preserves Kimi Responses endpoint on submit', async () => {
    const account = buildAccount()
    account.platform = 'kimi'
    account.protocol_endpoints = { ...PROTOCOL_DEFAULTS.defaults.kimi.default }
    account.credentials = {
      api_key: 'sk-kimi',
      account_mode: 'payg'
    }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual(PROTOCOL_DEFAULTS.defaults.kimi.default)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toMatchObject({ account_mode: 'payg' })
  })

  it('preserves GLM endpoints on submit', async () => {
    const account = buildAccount()
    account.platform = 'zhipu'
    account.protocol_endpoints = {
      chat_completions: 'https://open.bigmodel.cn/api/coding/paas/v4',
      anthropic: 'https://open.bigmodel.cn/api/anthropic'
    }
    account.credentials = {
      api_key: 'sk-glm',
      account_mode: 'coding'
    }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual({
      chat_completions: 'https://open.bigmodel.cn/api/coding/paas/v4',
      anthropic: 'https://open.bigmodel.cn/api/anthropic'
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toMatchObject({ account_mode: 'coding' })
  })

  it('keeps a custom CN relay address and payg mode on save', async () => {
    const account = buildAccount()
    account.platform = 'zhipu'
    account.protocol_endpoints = { chat_completions: 'https://relay.example.com/v1' }
    account.credentials = { api_key: 'sk-glm', account_mode: 'payg' }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const payload = updateAccountMock.mock.calls[0]?.[1]
    expect(payload?.protocol_endpoints).toEqual({ chat_completions: 'https://relay.example.com/v1' })
    expect(showErrorMock).not.toHaveBeenCalled()
    expect(payload?.credentials).toMatchObject({ account_mode: 'payg' })
  })

  it('has no API protocol selector for Chinese provider keys', async () => {
    const account = buildAccount()
    account.platform = 'zhipu'
    account.protocol_endpoints = { chat_completions: 'https://relay.example.com/v1' }
    account.credentials = { api_key: 'sk-glm', account_mode: 'payg' }

    const wrapper = mountModal(account)
    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.cnProviders.accountMode.title')
    expect(wrapper.text()).not.toContain('admin.accounts.cnProviders.apiProtocol.title')
  })

  it('keeps stored relay endpoints after the official addresses finish loading', async () => {
    const account = buildAccount()
    account.platform = 'kimi'
    account.protocol_endpoints = { chat_completions: 'https://relay.example.com/v1' }
    account.credentials = { api_key: 'sk-kimi', account_mode: 'payg' }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await flushPromises()
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual({
      chat_completions: 'https://relay.example.com/v1'
    })
  })

  it('does not replace stored endpoints while switching to another account', async () => {
    // 官方地址已加载后换一个账号：回填会让官方地址从 coding 变成 payg。B 账号存的恰好是
    // coding 的官方地址（模式与地址不一致的存量数据），回填窗口内不能被 payg 官方地址覆盖。
    const first = buildAccount()
    first.platform = 'kimi'
    first.protocol_endpoints = { ...PROTOCOL_DEFAULTS.defaults.kimi.coding }
    first.credentials = { api_key: 'sk-kimi', account_mode: 'coding' }
    const second = buildAccount()
    second.id = 99
    second.platform = 'kimi'
    second.protocol_endpoints = { ...PROTOCOL_DEFAULTS.defaults.kimi.coding }
    second.credentials = { api_key: 'sk-kimi-2', account_mode: 'payg' }
    updateAccountMock.mockReset().mockResolvedValue(second)

    const wrapper = mountModal(first)
    await flushPromises()
    await wrapper.setProps({ account: second })
    await flushPromises()
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock.mock.calls[0]?.[0]).toBe(99)
    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual(PROTOCOL_DEFAULTS.defaults.kimi.coding)
  })

  it('fills the preset protocol endpoint when a Chinese provider preset is picked', async () => {
    const account = buildAccount()
    account.platform = 'minimax'
    account.protocol_endpoints = { chat_completions: 'https://api.minimaxi.com/v1' }
    account.credentials = { api_key: 'sk-minimax', account_mode: 'payg' }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    const preset = wrapper
      .findAll('[data-testid="cn-base-url-preset"]')
      .find(button => button.text().startsWith('MiniMax Intl Anthropic (api.minimax.io/anthropic)'))
    expect(preset).toBeDefined()
    await preset!.trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    const payload = updateAccountMock.mock.calls[0]?.[1]
    expect(payload?.credentials).not.toHaveProperty('api_protocol')
    expect(payload?.protocol_endpoints).toEqual({
      chat_completions: 'https://api.minimaxi.com/v1',
      anthropic: 'https://api.minimax.io/anthropic'
    })
  })

  it('applies a Grok preset to the configured Grok endpoints', async () => {
    const account = buildGrokAPIKeyAccount()
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    const preset = wrapper.findAll('[data-testid="grok-base-url-preset"]').find(button => button.text().includes('eu-west-1'))
    expect(preset).toBeDefined()
    await preset!.trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual({
      chat_completions: 'https://eu-west-1.api.x.ai/v1',
      responses: 'https://eu-west-1.api.x.ai/v1'
    })
  })

  it('switches unedited official endpoints when the admin changes the account mode', async () => {
    const account = buildAccount()
    account.platform = 'kimi'
    account.protocol_endpoints = { ...PROTOCOL_DEFAULTS.defaults.kimi.default }
    account.credentials = { api_key: 'sk-kimi', account_mode: 'payg' }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await flushPromises()
    const codingButton = wrapper
      .findAll('button')
      .find(button => button.text().includes('admin.accounts.cnProviders.accountMode.coding'))
    expect(codingButton).toBeDefined()
    await codingButton!.trigger('click')
    await flushPromises()
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual(PROTOCOL_DEFAULTS.defaults.kimi.coding)
  })

  it('preserves model mappings when editing the whitelist', async () => {
    const account = buildAccount()
    account.credentials.model_mapping = {
      'gpt-5.2': 'gpt-5.2',
      'gpt-latest': 'gpt-5.2'
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2-2025-12-11': 'gpt-5.2-2025-12-11',
      'gpt-latest': 'gpt-5.2'
    })
  })

  it('submits OpenAI compact mode and compact-only model mapping', async () => {
    const account = buildAccount()
    account.extra = {
      openai_compact_mode: 'force_on'
    }
    account.credentials = {
      ...account.credentials,
      compact_model_mapping: {
        'gpt-5.4': 'gpt-5.4-openai-compact'
      }
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_compact_mode).toBe('force_on')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.compact_model_mapping).toEqual({
      'gpt-5.4': 'gpt-5.4-openai-compact'
    })
  })

  it('loads and clears the OAuth-only Codex namespace flatten toggle', async () => {
    const account = buildAccount()
    account.type = 'oauth'
    account.extra = {
      openai_responses_flatten_namespaces: true
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const toggle = wrapper.get('[data-testid="edit-openai-flatten-namespaces-toggle"]')

    // 关闭后应从 extra 中删除该键，而不是写入 false
    await toggle.trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty(
      'openai_responses_flatten_namespaces'
    )
  })

  it('submits the Codex namespace flatten toggle when switched on', async () => {
    const account = buildAccount()
    account.type = 'oauth'
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('[data-testid="edit-openai-flatten-namespaces-toggle"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_responses_flatten_namespaces).toBe(
      true
    )
  })

  it('writes the upstream request id header into extra only when it changes', async () => {
    const account = buildAccount()
    account.extra = { openai_compact_mode: 'force_on' }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const untouched = mountModal(account)
    await untouched.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.upstream_request_id_header).toBeUndefined()

    updateAccountMock.mockClear()
    const wrapper = mountModal(account)
    await wrapper.get('[data-testid="upstream-request-id-header"]').setValue(' X-Oneapi-Request-Id ')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toMatchObject({
      openai_compact_mode: 'force_on',
      upstream_request_id_header: 'X-Oneapi-Request-Id'
    })
  })

  it('removes the upstream request id header from extra when cleared', async () => {
    const account = buildAccount()
    account.extra = { upstream_request_id_header: 'X-Request-ID' }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    expect((wrapper.get('[data-testid="upstream-request-id-header"]').element as HTMLInputElement).value).toBe('X-Request-ID')
    await wrapper.get('[data-testid="upstream-request-id-header"]').setValue('')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toBeDefined()
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('upstream_request_id_header')
  })

  it('writes images_url_to_b64_json into extra when toggled on', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const toggle = wrapper.get('[data-testid="openai-images-url-to-b64-json-toggle"]')
    expect(toggle.attributes('aria-checked')).toBe('false')
    await toggle.trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.images_url_to_b64_json).toBe(true)
  })

  it('removes images_url_to_b64_json from extra when toggled off', async () => {
    const account = buildAccount()
    account.extra = { images_url_to_b64_json: true }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const toggle = wrapper.get('[data-testid="openai-images-url-to-b64-json-toggle"]')
    expect(toggle.attributes('aria-checked')).toBe('true')
    await toggle.trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toBeDefined()
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('images_url_to_b64_json')
  })

  it('hides the Codex namespace flatten toggle for non-OAuth OpenAI accounts', async () => {
    const account = buildAccount()
    const wrapper = mountModal(account)

    expect(wrapper.find('[data-testid="edit-openai-flatten-namespaces-toggle"]').exists()).toBe(
      false
    )
  })

  // 长上下文计费开关按协议地址露出，不看标签：kimi 标签 + Chat Completions 地址的 key 能改；
  // 只配 Anthropic 地址的 openai 标签 key 看不到开关，保存时保留已存值。
  it('loads and submits Grok OAuth model mapping edits', async () => {
    const account = buildGrokOAuthAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    expect(wrapper.text()).toContain('Imagine Image')
    expect(wrapper.text()).toContain('Imagine Video')

    const inputWithValue = (value: string) => {
      const input = wrapper
        .findAll('input')
        .find((input) => (input.element as HTMLInputElement).value === value)
      expect(input).toBeTruthy()
      return input!
    }

    await inputWithValue('grok-latest').setValue('grok')
    await inputWithValue('grok-4.3').setValue('grok-build-0.1')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      grok: 'grok-build-0.1'
    })
  })

  it('saves a Grok API-key account with its stored endpoints and no base_url fallback', async () => {
    const account = buildGrokAPIKeyAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(
      (wrapper.get('[data-testid="protocol-endpoint-input-chat_completions"]').element as HTMLInputElement).value
    ).toBe('https://api.x.ai/v1')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual({
      chat_completions: 'https://api.x.ai/v1',
      responses: 'https://api.x.ai/v1'
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('base_url')
  })

  it('only submits model mapping credentials when saving an OpenAI spark shadow account', async () => {
    authIsSimpleMode.value = false
    const account = buildOpenAISparkShadowAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const payload = updateAccountMock.mock.calls[0]?.[1]
    // 分组绑定段已删：编辑弹窗不再碰 group_ids
    expect(payload).not.toHaveProperty('group_ids')
    expect(payload?.credentials).toEqual({
      model_mapping: {
        'gpt-5.3-codex-spark': 'gpt-5.3-codex-spark'
      },
      compact_model_mapping: {
        'gpt-5.3-codex-spark': 'gpt-5.3-codex-spark-compact'
      }
    })
  })

  it('has no OpenAI APIKey Responses routing control and drops the retired routing flags on save', async () => {
    const account = buildAccount()
    account.extra = {
      openai_responses_mode: 'force_chat_completions',
      openai_responses_supported: false
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.find('[data-testid="openai-responses-mode-select"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('admin.accounts.openai.responsesMode')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('openai_responses_mode')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('openai_responses_supported')
  })

  it('submits the account upstream billing auto-probe setting', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const toggle = wrapper.get('[data-testid="upstream-billing-auto-probe"]')
    expect(toggle.attributes('aria-checked')).toBe('false')

    await toggle.trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.upstream_billing_probe_enabled).toBe(true)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty(
      'upstream_billing_probe_enabled'
    )
  })

  it('exposes the upstream billing auto-probe toggle for non-OpenAI API-key accounts', async () => {
    // 探测已放宽到全部 API-key 平台：grok 账号同样能开启并保存。
    const account = buildAccount()
    account.platform = 'grok'
    account.name = 'grok-relay'
    account.credentials = { api_key: 'sk-grok' }
    account.protocol_endpoints = { chat_completions: 'https://relay.example/v1' }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const toggle = wrapper.get('[data-testid="upstream-billing-auto-probe"]')
    expect(toggle.attributes('aria-checked')).toBe('false')

    await toggle.trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.upstream_billing_probe_enabled).toBe(true)
  })

  it('enabling rate sync also enables probing and stops submitting a manual rate', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const syncToggle = wrapper.get('[data-testid="upstream-billing-rate-sync"]')
    const probeToggle = wrapper.get('[data-testid="upstream-billing-auto-probe"]')
    const rateInput = wrapper.get<HTMLInputElement>('[data-testid="account-rate-multiplier"]')
    expect(syncToggle.attributes('aria-checked')).toBe('false')
    expect(probeToggle.attributes('aria-checked')).toBe('false')
    expect(rateInput.element.disabled).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.billingRateMultiplierHint')
    expect(wrapper.text()).not.toContain('admin.accounts.upstreamBilling.syncRateManagedHint')

    await syncToggle.trigger('click')
    expect(syncToggle.attributes('aria-checked')).toBe('true')
    expect(probeToggle.attributes('aria-checked')).toBe('true')
    expect(rateInput.element.disabled).toBe(true)
    expect(wrapper.text()).toContain('admin.accounts.upstreamBilling.syncRateManagedHint')
    expect(wrapper.text()).not.toContain('admin.accounts.billingRateMultiplierHint')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    const payload = updateAccountMock.mock.calls[0]?.[1]
    expect(payload?.upstream_billing_probe_enabled).toBe(true)
    expect(payload?.upstream_billing_rate_sync_enabled).toBe(true)
    expect(payload).not.toHaveProperty('rate_multiplier')
  })

  it('disabling probing also disables rate sync and restores manual rate editing', async () => {
    const account = buildAccount()
    account.extra = {
      upstream_billing_probe_enabled: true,
      upstream_billing_rate_sync_enabled: true
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const syncToggle = wrapper.get('[data-testid="upstream-billing-rate-sync"]')
    const probeToggle = wrapper.get('[data-testid="upstream-billing-auto-probe"]')
    const rateInput = wrapper.get<HTMLInputElement>('[data-testid="account-rate-multiplier"]')
    expect(syncToggle.attributes('aria-checked')).toBe('true')
    expect(rateInput.element.disabled).toBe(true)

    await probeToggle.trigger('click')
    expect(probeToggle.attributes('aria-checked')).toBe('false')
    expect(syncToggle.attributes('aria-checked')).toBe('false')
    expect(rateInput.element.disabled).toBe(false)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    const payload = updateAccountMock.mock.calls[0]?.[1]
    expect(payload?.upstream_billing_probe_enabled).toBe(false)
    expect(payload?.upstream_billing_rate_sync_enabled).toBe(false)
    expect(payload?.rate_multiplier).toBe(1)
  })

  it('disabling only rate sync keeps automatic probing enabled', async () => {
    const account = buildAccount()
    account.extra = {
      upstream_billing_probe_enabled: true,
      upstream_billing_rate_sync_enabled: true
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('[data-testid="upstream-billing-rate-sync"]').trigger('click')
    expect(wrapper.get('[data-testid="upstream-billing-auto-probe"]').attributes('aria-checked')).toBe(
      'true'
    )
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    const payload = updateAccountMock.mock.calls[0]?.[1]
    expect(payload?.upstream_billing_probe_enabled).toBe(true)
    expect(payload?.upstream_billing_rate_sync_enabled).toBe(false)
    expect(payload?.rate_multiplier).toBe(1)
  })

  it('submits OpenAI APIKey endpoint capabilities from credentials', async () => {
    const account = buildAccount()
    account.credentials.openai_capabilities = ['chat_completions']
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.findAll('input[type="checkbox"]').some((input) => (input.element as HTMLInputElement).checked)).toBe(true)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.openai_capabilities).toEqual([
      'chat_completions'
    ])
  })

	it('submits OpenAI quota auto-pause thresholds in extra', async () => {
	  const account = buildAccount()
	  account.extra = {
		auto_pause_5h_threshold: 0.9,
		auto_pause_7d_threshold: 0.8
	  }
	  updateAccountMock.mockReset()
	  updateAccountMock.mockResolvedValue(account)

	  const wrapper = mountModal(account)

	  await wrapper.get('[data-testid="auto-pause-5h-threshold"]').setValue('95')
	  await wrapper.get('[data-testid="auto-pause-7d-threshold"]').setValue('96')
	  await wrapper.get('form#edit-account-form').trigger('submit.prevent')

	  expect(updateAccountMock).toHaveBeenCalledTimes(1)
	  expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.auto_pause_5h_threshold).toBe(0.95)
	  expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.auto_pause_7d_threshold).toBe(0.96)
	})

	it('submits OpenAI quota auto-pause disable flag in extra', async () => {
	  // Toggling the per-account disable flag must persist as auto_pause_5h_disabled
	  // so an admin can exempt one account from auto-pause even when a global default
	  // threshold is configured (otherwise leaving the threshold blank would silently
	  // fall back to the global default).
	  const account = buildAccount()
	  updateAccountMock.mockReset()
	  updateAccountMock.mockResolvedValue(account)

	  const wrapper = mountModal(account)

	  await wrapper.get('[data-testid="auto-pause-5h-disabled"]').trigger('click')
	  await wrapper.get('form#edit-account-form').trigger('submit.prevent')

	  expect(updateAccountMock).toHaveBeenCalledTimes(1)
	  expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.auto_pause_5h_disabled).toBe(true)
	  expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.auto_pause_7d_disabled).toBeUndefined()
	})

  it('keeps at least one OpenAI APIKey endpoint capability selected', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    const chatCheckbox = wrapper.get<HTMLInputElement>(
      '[data-testid="openai-endpoint-capability-chat_completions"]'
    )
    const embeddingsCheckbox = wrapper.get<HTMLInputElement>(
      '[data-testid="openai-endpoint-capability-embeddings"]'
    )

    expect(chatCheckbox.element.checked).toBe(true)
    expect(embeddingsCheckbox.element.checked).toBe(true)

    await embeddingsCheckbox.setValue(false)

    expect(chatCheckbox.element.checked).toBe(true)
    expect(embeddingsCheckbox.element.checked).toBe(false)

    await chatCheckbox.setValue(false)

    expect(chatCheckbox.element.checked).toBe(true)
    expect(embeddingsCheckbox.element.checked).toBe(false)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.openai_capabilities).toEqual([
      'chat_completions'
    ])
  })

  it('submits an embeddings-only OpenAI APIKey endpoint capability', async () => {
    const account = buildAccount()
    account.credentials.openai_capabilities = ['embeddings']
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.text()).toContain('admin.accounts.openai.capabilityText')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.openai_capabilities).toEqual([
      'embeddings'
    ])
  })

  it('submits Codex image tool force-inject mode as bridge override', async () => {
    const account = buildAccount()
    account.extra = {
      codex_image_generation_bridge: false,
      codex_image_generation_bridge_enabled: true
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.text()).toContain('admin.accounts.openai.codexImageTool')
    expect(wrapper.text()).toContain('admin.accounts.openai.codexImageToolDesc')
    expect(wrapper.text()).toContain('admin.accounts.openai.codexImageToolEnabledDesc')

    await wrapper.get('button[data-testid="codex-image-tool-enabled"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.codex_image_generation_bridge).toBe(true)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('codex_image_generation_bridge_enabled')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('codex_image_generation_explicit_tool_policy')
  })

  it('submits Codex image tool no-injection mode without strip policy', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('button[data-testid="codex-image-tool-disabled"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.codex_image_generation_bridge).toBe(false)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('codex_image_generation_explicit_tool_policy')
  })

  it('submits Codex image tool block mode as strip policy and clears bridge override', async () => {
    const account = buildAccount()
    account.extra = {
      codex_image_generation_bridge: true
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(wrapper.text()).toContain('admin.accounts.openai.codexImageToolBlock')
    expect(wrapper.text()).toContain('admin.accounts.openai.codexImageToolBlockDesc')

    await wrapper.get('button[data-testid="codex-image-tool-block"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.codex_image_generation_explicit_tool_policy).toBe('strip')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('codex_image_generation_bridge')
  })

  it('loads strip policy as block mode and clears both keys when reset to inherit', async () => {
    const account = buildAccount()
    account.extra = {
      codex_image_generation_explicit_tool_policy: 'strip'
    }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('button[data-testid="codex-image-tool-inherit"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('codex_image_generation_explicit_tool_policy')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('codex_image_generation_bridge')
  })

  it('setup-token account can select and submit OAuth WS mode', async () => {
    const account = buildOpenAISetupTokenAccount()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="edit-openai-ws-mode-select"]').setValue('http_bridge')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_oauth_responses_websockets_v2_mode).toBe('http_bridge')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_oauth_responses_websockets_v2_enabled).toBe(true)
  })

  it('allows saving apikey account when backend redacted api_key but credentials_status reports it exists', async () => {
    // 新前端 + 新后端：响应已脱敏，credentials 里没有 api_key，credentials_status.has_api_key=true
    const account = buildAccount()
    account.credentials = {
      base_url: 'https://api.openai.com',
      model_mapping: { 'gpt-5.2': 'gpt-5.2' }
    }
    account.credentials_status = { has_api_key: true }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    // 用户未输入新 key 时，payload 不应带 api_key，由后端合并保留旧值
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('api_key')
  })

  it('allows saving apikey account against legacy backend without credentials_status', async () => {
    // 新前端 + 旧后端：credentials_status 缺失，但 credentials.api_key 仍是明文，应允许保存
    const account = buildAccount()
    // 显式确保没有 credentials_status
    expect(account.credentials_status).toBeUndefined()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    // 旧后端响应未脱敏，原 api_key 会随 currentCredentials 一起传回去（旧行为，等价于无操作）
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.api_key).toBe('sk-test')
  })

  it('blocks apikey save when neither credentials_status nor legacy api_key indicates existence', async () => {
    const account = buildAccount()
    account.credentials = {
      base_url: 'https://api.openai.com'
    }
    // 既没有 credentials_status 也没有旧的 api_key
    updateAccountMock.mockReset()

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).not.toHaveBeenCalled()
  })

  it('allows saving Vertex SA account when backend redacted service_account_json but credentials_status reports it exists', async () => {
    // 新前端 + 新后端：响应已脱敏，credentials 里没有 service_account_json，credentials_status.has_service_account_json=true
    const account = buildVertexAccount()
    account.credentials = {
      project_id: 'demo-project',
      client_email: 'sa@example.iam.gserviceaccount.com',
      location: 'us-central1',
      tier_id: 'vertex'
    }
    account.credentials_status = { has_service_account_json: true }
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.project_id).toBe('demo-project')
  })

  it('allows saving Vertex SA account against legacy backend without credentials_status', async () => {
    // 新前端 + 旧后端：credentials_status 缺失，但 credentials.service_account_json 仍是明文，应允许保存
    const account = buildVertexAccount()
    expect(account.credentials_status).toBeUndefined()
    expect(account.credentials.service_account_json).toBeTruthy()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
  })

  it('blocks Vertex SA save when neither credentials_status nor legacy json indicates existence', async () => {
    const account = buildVertexAccount()
    account.credentials = {
      project_id: 'demo-project',
      client_email: 'sa@example.iam.gserviceaccount.com',
      location: 'us-central1',
      tier_id: 'vertex'
    }
    // 既没有 credentials_status 也没有旧的 service_account_json
    updateAccountMock.mockReset()

    const wrapper = mountModal(account)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).not.toHaveBeenCalled()
  })

  it('loads and submits Antigravity configured project fallback', async () => {
    const account = buildAntigravityAccount('configured-project')
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const input = wrapper.get<HTMLInputElement>('[data-testid="antigravity-project-id-input"]')
    expect(input.element.value).toBe('configured-project')

    await input.setValue('  updated-project  ')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.antigravity_project_id).toBe(
      'updated-project'
    )
  })

  it('clears Antigravity configured project fallback when input is empty', async () => {
    const account = buildAntigravityAccount('configured-project')
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    const input = wrapper.get<HTMLInputElement>('[data-testid="antigravity-project-id-input"]')

    await input.setValue('')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty(
      'antigravity_project_id'
    )
  })
})

describe('EditAccountModal OpenAI 自动使用重置卡', () => {
  beforeEach(() => {
    authIsSimpleMode.value = true
    updateAccountMock.mockReset()
  })

  it('仅对 OpenAI OAuth 母账号显示，默认关闭且阈值为 100/100', () => {
    const parent = mountModal(buildOpenAIOAuthParentAccount())
    expect(parent.find('[data-testid="auto-reset-credit-settings"]').exists()).toBe(true)
    expect((parent.get('[data-testid="auto-reset-credit-5h-threshold"]').element as HTMLInputElement).value).toBe('100')
    expect((parent.get('[data-testid="auto-reset-credit-7d-threshold"]').element as HTMLInputElement).value).toBe('100')
    expect(parent.get('[data-testid="auto-reset-credit-5h-threshold"]').attributes('disabled')).toBeDefined()
    parent.unmount()

    for (const account of [buildAccount(), buildOpenAISetupTokenAccount(), buildOpenAISparkShadowAccount()]) {
      const wrapper = mountModal(account)
      expect(wrapper.find('[data-testid="auto-reset-credit-settings"]').exists()).toBe(false)
      wrapper.unmount()
    }
  })

  it('独立保存两个阈值，并禁止把运行态回写到管理请求', async () => {
    const account = buildOpenAIOAuthParentAccount()
    account.extra = {
      codex_auto_reset_credit_state: {
        status: 'success',
        trigger_window: '5h',
        available_count: 1
      }
    }
    updateAccountMock.mockResolvedValue(account)
    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="auto-reset-credit-enabled"]').trigger('click')
    await wrapper.get('[data-testid="auto-reset-credit-5h-threshold"]').setValue('75.5')
    await wrapper.get('[data-testid="auto-reset-credit-7d-threshold"]').setValue('92')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const extra = updateAccountMock.mock.calls[0]?.[1]?.extra
    expect(extra).toMatchObject({
      auto_reset_credit_enabled: true,
      auto_reset_credit_5h_threshold: 0.755,
      auto_reset_credit_7d_threshold: 0.92
    })
    expect(extra).not.toHaveProperty('codex_auto_reset_credit_state')
    wrapper.unmount()
  })

  it('开启后拒绝超出 0.1–100 范围的任一阈值', async () => {
    const wrapper = mountModal(buildOpenAIOAuthParentAccount())
    await wrapper.get('[data-testid="auto-reset-credit-enabled"]').trigger('click')
    await wrapper.get('[data-testid="auto-reset-credit-5h-threshold"]').setValue('0')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})

describe('EditAccountModal third-party key settings do not follow the platform label', () => {
  beforeEach(() => {
    authIsSimpleMode.value = true
    updateAccountMock.mockReset()
  })

  function buildKey(platform: string, protocolEndpoints: Record<string, string>, extra: Record<string, unknown> = {}) {
    const account = {
      ...buildAccount(),
      platform,
      credentials: {},
      credentials_status: { has_api_key: true },
      protocol_endpoints: protocolEndpoints,
      extra
    } as any
    updateAccountMock.mockResolvedValue(account)
    return account
  }

  async function submitPayload(wrapper: ReturnType<typeof mountModal>) {
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    return updateAccountMock.mock.calls[0]?.[1]
  }

  it('offers header overrides for a Gemini-labelled key and submits them', async () => {
    const wrapper = mountModal(buildKey('gemini', { gemini: 'https://generativelanguage.googleapis.com' }))

    await wrapper.get('[data-testid="edit-header-override-toggle"]').trigger('click')
    const section = wrapper.get('[data-testid="edit-header-override"]')
    const addRow = section.findAll('button').find((button) => button.text().includes('admin.accounts.headerOverride.addRow'))
    expect(addRow).toBeDefined()
    await addRow!.trigger('click')
    const [name, value] = section.findAll('input[type="text"]')
    await name.setValue('X-Relay-Tenant')
    await value.setValue('team-a')

    const payload = await submitPayload(wrapper)
    expect(payload?.credentials).toMatchObject({
      header_override_enabled: true,
      header_overrides: { 'x-relay-tenant': 'team-a' }
    })
  })

  it('keeps header overrides limited to Grok OAuth among subscription accounts', () => {
    const openaiOAuth = mountModal(buildOpenAIOAuthParentAccount())
    expect(openaiOAuth.find('[data-testid="edit-header-override"]').exists()).toBe(false)
    openaiOAuth.unmount()

    const grokOAuth = mountModal(buildGrokOAuthAccount())
    expect(grokOAuth.find('[data-testid="edit-header-override"]').exists()).toBe(true)
    grokOAuth.unmount()
  })

  it('shows Anthropic protocol settings for a Kimi-labelled key with an anthropic endpoint and submits them', async () => {
    vi.mocked(adminAPI.settings.getWebSearchEmulationConfig).mockResolvedValueOnce({ enabled: true, providers: [{}] } as any)
    const wrapper = mountModal(buildKey('kimi', {
      anthropic: 'https://api.moonshot.cn/anthropic',
      chat_completions: 'https://api.moonshot.cn/v1'
    }))
    await flushPromises()

    await wrapper.get('[data-testid="edit-anthropic-passthrough-toggle"]').trigger('click')
    await wrapper.get('[data-testid="edit-anthropic-auth-scheme"]').setValue('authorization_bearer')
    await wrapper.get('[data-testid="edit-web-search-emulation-toggle"]').trigger('click')
    await wrapper.get('[data-testid="edit-bedrock-cc-compat-toggle"]').trigger('click')

    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({
      anthropic_passthrough: true,
      anthropic_apikey_auth_scheme: 'authorization_bearer',
      web_search_emulation: true,
      bedrock_cc_compat: true
    })
  })

  it('loads stored Anthropic protocol settings of a non-Anthropic-labelled key', async () => {
    const wrapper = mountModal(buildKey(
      'zhipu',
      { anthropic: 'https://open.bigmodel.cn/api/anthropic' },
      { anthropic_passthrough: true, anthropic_apikey_auth_scheme: 'authorization_bearer' }
    ))
    await flushPromises()

    expect((wrapper.get('[data-testid="edit-anthropic-auth-scheme"]').element as HTMLSelectElement).value)
      .toBe('authorization_bearer')
    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({
      anthropic_passthrough: true,
      anthropic_apikey_auth_scheme: 'authorization_bearer'
    })
  })

  it('loads a legacy string web_search_emulation as on and saves it back as a bool', async () => {
    vi.mocked(adminAPI.settings.getWebSearchEmulationConfig).mockResolvedValueOnce({ enabled: true, providers: [{}] } as any)
    const wrapper = mountModal(buildKey(
      'anthropic',
      { anthropic: 'https://relay.example.com/anthropic' },
      { web_search_emulation: 'enabled', bedrock_cc_compat: true }
    ))
    await flushPromises()

    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({ web_search_emulation: true, bedrock_cc_compat: true })
  })

  it('drops web_search_emulation and bedrock_cc_compat from extra when both toggles are off', async () => {
    vi.mocked(adminAPI.settings.getWebSearchEmulationConfig).mockResolvedValueOnce({ enabled: true, providers: [{}] } as any)
    const wrapper = mountModal(buildKey(
      'anthropic',
      { anthropic: 'https://relay.example.com/anthropic' },
      { web_search_emulation: 'default', bedrock_cc_compat: false }
    ))
    await flushPromises()

    const payload = await submitPayload(wrapper)
    expect(payload?.extra ?? {}).not.toHaveProperty('web_search_emulation')
    expect(payload?.extra ?? {}).not.toHaveProperty('bedrock_cc_compat')
  })

  it('hides Anthropic protocol settings for an Anthropic-labelled key without an anthropic endpoint', async () => {
    const wrapper = mountModal(buildKey(
      'anthropic',
      { chat_completions: 'https://relay.example.com/v1' },
      { anthropic_passthrough: true, anthropic_apikey_auth_scheme: 'authorization_bearer' }
    ))
    await flushPromises()

    expect(wrapper.find('[data-testid="edit-anthropic-passthrough"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="edit-anthropic-auth-scheme"]').exists()).toBe(false)

    // 隐藏区块不写界面值，账号已存的值原样保留
    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({
      anthropic_passthrough: true,
      anthropic_apikey_auth_scheme: 'authorization_bearer'
    })
  })

  it('follows anthropic endpoint rows added or removed in the modal and does not submit hidden edits', async () => {
    const wrapper = mountModal(buildKey('kimi', { chat_completions: 'https://api.moonshot.cn/v1' }))
    await flushPromises()
    expect(wrapper.find('[data-testid="edit-anthropic-passthrough"]').exists()).toBe(false)

    await wrapper.get('[data-testid="protocol-endpoint-add-anthropic"]').trigger('click')
    expect(wrapper.find('[data-testid="edit-anthropic-passthrough"]').exists()).toBe(true)
    await wrapper.get('[data-testid="edit-anthropic-passthrough-toggle"]').trigger('click')

    await wrapper.get('[data-testid="protocol-endpoint-remove-anthropic"]').trigger('click')
    expect(wrapper.find('[data-testid="edit-anthropic-passthrough"]').exists()).toBe(false)

    const payload = await submitPayload(wrapper)
    expect(payload?.extra ?? {}).not.toHaveProperty('anthropic_passthrough')
  })

  it('keeps Anthropic settings when an OpenAI-labelled key also has OpenAI settings to save', async () => {
    const wrapper = mountModal(buildKey('openai', {
      anthropic: 'https://relay.example.com',
      responses: 'https://api.openai.com'
    }))
    await flushPromises()

    await wrapper.get('[data-testid="edit-anthropic-passthrough-toggle"]').trigger('click')

    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({
      anthropic_passthrough: true,
      openai_apikey_responses_websockets_v2_mode: 'off'
    })
  })

  it('shows OpenAI Responses settings with the vendor hint for an Anthropic-labelled key with a responses endpoint', async () => {
    const wrapper = mountModal(buildKey('anthropic', { responses: 'https://relay.example.com/v1' }))
    await flushPromises()

    expect(wrapper.find('[data-testid="edit-openai-key-protocol-hint"]').exists()).toBe(true)
    await wrapper.get('[data-testid="edit-openai-passthrough-toggle"]').trigger('click')
    expect(wrapper.text()).toContain('admin.accounts.openai.modelRestrictionDisabledByPassthrough')
    await wrapper.get('[data-testid="edit-openai-ws-mode-select"]').setValue('ctx_pool')
    await wrapper.get('[data-testid="edit-openai-compact-mode-select"]').setValue('force_on')
    const compact = wrapper.get('[data-testid="edit-openai-compact"]')
    const addCompactMapping = compact.findAll('button').find((button) => button.text().includes('admin.accounts.addMapping'))
    expect(addCompactMapping).toBeDefined()
    await addCompactMapping!.trigger('click')
    const [from, to] = compact.findAll('input[type="text"]')
    await from.setValue('gpt-5.4')
    await to.setValue('gpt-5.4-compact')

    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({
      openai_passthrough: true,
      openai_apikey_responses_websockets_v2_mode: 'ctx_pool',
      openai_apikey_responses_websockets_v2_enabled: true,
      openai_compact_mode: 'force_on'
    })
    expect(payload?.credentials?.compact_model_mapping).toEqual({ 'gpt-5.4': 'gpt-5.4-compact' })
  })

  it('shows OpenAI Responses settings for a key that only has a chat_completions endpoint', async () => {
    const wrapper = mountModal(buildKey('kimi', { chat_completions: 'https://api.moonshot.cn/v1' }))
    await flushPromises()

    expect(wrapper.find('[data-testid="edit-openai-passthrough"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="edit-openai-ws-mode-select"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="edit-openai-compact"]').text()).toContain('admin.accounts.openai.compactAuto')
  })

  it('loads stored OpenAI Responses settings of a non-OpenAI-labelled key', async () => {
    const account = buildKey(
      'deepseek',
      { chat_completions: 'https://relay.example.com/v1' },
      {
        openai_passthrough: true,
        openai_compact_mode: 'force_on',
        openai_apikey_responses_websockets_v2_mode: 'passthrough',
        openai_apikey_responses_websockets_v2_enabled: true
      }
    )
    account.credentials = { compact_model_mapping: { 'gpt-5.4': 'gpt-5.4-compact' } }
    const wrapper = mountModal(account)
    await flushPromises()

    expect((wrapper.get('[data-testid="edit-openai-ws-mode-select"]').element as HTMLSelectElement).value).toBe('passthrough')
    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({
      openai_passthrough: true,
      openai_compact_mode: 'force_on',
      openai_apikey_responses_websockets_v2_mode: 'passthrough'
    })
    expect(payload?.credentials?.compact_model_mapping).toEqual({ 'gpt-5.4': 'gpt-5.4-compact' })
  })

  it('shows endpoint capabilities and the b64 toggle for a Kimi-labelled key with an OpenAI endpoint', async () => {
    const account = buildKey('kimi', { chat_completions: 'https://relay.example.com/v1' })
    const wrapper = mountModal(account)
    await flushPromises()

    expect(wrapper.find('[data-testid="openai-endpoint-capability-embeddings"]').exists()).toBe(true)
    await wrapper.get('[data-testid="openai-endpoint-capability-embeddings"]').setValue(false)
    await wrapper.get('[data-testid="openai-images-url-to-b64-json-toggle"]').trigger('click')

    const payload = await submitPayload(wrapper)
    expect(payload?.credentials?.openai_capabilities).toEqual(['chat_completions'])
    expect(payload?.extra?.images_url_to_b64_json).toBe(true)
  })

  it('hides endpoint capabilities and the b64 toggle for an OpenAI-labelled key without an OpenAI endpoint', async () => {
    const account = buildKey(
      'openai',
      { anthropic: 'https://relay.example.com' },
      { images_url_to_b64_json: true }
    )
    account.credentials = { openai_capabilities: ['chat_completions'] }
    const wrapper = mountModal(account)
    await flushPromises()

    expect(wrapper.find('[data-testid="openai-endpoint-capability-embeddings"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="openai-images-url-to-b64-json-toggle"]').exists()).toBe(false)

    // 区块隐藏时保留账号已存的值，不按界面值改写
    const payload = await submitPayload(wrapper)
    expect(payload?.credentials?.openai_capabilities).toEqual(['chat_completions'])
    expect(payload?.extra?.images_url_to_b64_json).toBe(true)
  })

  it('loads stored endpoint capabilities of a non-OpenAI-labelled key instead of resetting them', async () => {
    const account = buildKey('kimi', { chat_completions: 'https://relay.example.com/v1' })
    account.credentials = { openai_capabilities: ['chat_completions'] }
    const wrapper = mountModal(account)
    await flushPromises()

    const embeddings = wrapper.get('[data-testid="openai-endpoint-capability-embeddings"]')
      .element as HTMLInputElement
    expect(embeddings.checked).toBe(false)

    const payload = await submitPayload(wrapper)
    expect(payload?.credentials?.openai_capabilities).toEqual(['chat_completions'])
  })

  it('does not submit endpoint capabilities or the b64 flag edited before the OpenAI endpoint was removed', async () => {
    const account = buildKey('kimi', {
      chat_completions: 'https://relay.example.com/v1',
      anthropic: 'https://relay.example.com'
    })
    const wrapper = mountModal(account)
    await flushPromises()

    await wrapper.get('[data-testid="openai-endpoint-capability-embeddings"]').setValue(false)
    await wrapper.get('[data-testid="openai-images-url-to-b64-json-toggle"]').trigger('click')

    await wrapper.get('[data-testid="protocol-endpoint-remove-chat_completions"]').trigger('click')
    expect(wrapper.find('[data-testid="openai-endpoint-capability-embeddings"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="openai-images-url-to-b64-json-toggle"]').exists()).toBe(false)

    const payload = await submitPayload(wrapper)
    expect(payload?.credentials ?? {}).not.toHaveProperty('openai_capabilities')
    expect(payload?.extra ?? {}).not.toHaveProperty('images_url_to_b64_json')
  })

  it('hides OpenAI Responses settings for an OpenAI-labelled key without responses or chat_completions endpoints', async () => {
    const account = buildKey(
      'openai',
      { anthropic: 'https://relay.example.com' },
      { openai_passthrough: true, openai_compact_mode: 'force_off' }
    )
    account.credentials = { compact_model_mapping: { 'gpt-5.4': 'gpt-5.4-compact' } }
    const wrapper = mountModal(account)
    await flushPromises()

    expect(wrapper.find('[data-testid="edit-openai-passthrough"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="edit-openai-ws-mode-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="edit-openai-compact"]').exists()).toBe(false)
    // 透传区块不可见，就不该因为已存的透传开关锁住模型限制
    expect(wrapper.text()).not.toContain('admin.accounts.openai.modelRestrictionDisabledByPassthrough')

    // 隐藏区块不写界面值，账号已存的值原样保留
    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({ openai_passthrough: true, openai_compact_mode: 'force_off' })
    expect(payload?.extra).not.toHaveProperty('openai_apikey_responses_websockets_v2_mode')
    expect(payload?.credentials?.compact_model_mapping).toEqual({ 'gpt-5.4': 'gpt-5.4-compact' })
  })

  it('follows responses endpoint rows added or removed in the modal and does not submit hidden edits', async () => {
    const wrapper = mountModal(buildKey('kimi', { anthropic: 'https://api.moonshot.cn/anthropic' }))
    await flushPromises()
    expect(wrapper.find('[data-testid="edit-openai-passthrough"]').exists()).toBe(false)

    await wrapper.get('[data-testid="protocol-endpoint-add-responses"]').trigger('click')
    expect(wrapper.find('[data-testid="edit-openai-passthrough"]').exists()).toBe(true)
    await wrapper.get('[data-testid="edit-openai-passthrough-toggle"]').trigger('click')
    await wrapper.get('[data-testid="edit-openai-compact-mode-select"]').setValue('force_on')
    const compact = wrapper.get('[data-testid="edit-openai-compact"]')
    await compact.findAll('button').find((button) => button.text().includes('admin.accounts.addMapping'))!.trigger('click')
    const [from, to] = compact.findAll('input[type="text"]')
    await from.setValue('gpt-5.4')
    await to.setValue('gpt-5.4-compact')

    await wrapper.get('[data-testid="protocol-endpoint-remove-responses"]').trigger('click')
    expect(wrapper.find('[data-testid="edit-openai-passthrough"]').exists()).toBe(false)

    const payload = await submitPayload(wrapper)
    expect(payload?.extra ?? {}).not.toHaveProperty('openai_passthrough')
    expect(payload?.extra ?? {}).not.toHaveProperty('openai_compact_mode')
    expect(payload?.extra ?? {}).not.toHaveProperty('openai_apikey_responses_websockets_v2_mode')
    expect(payload?.credentials ?? {}).not.toHaveProperty('compact_model_mapping')
  })

  it('keeps OpenAI Responses settings for OpenAI subscriptions only, without the key hint', async () => {
    const openaiOAuth = mountModal(buildOpenAIOAuthParentAccount())
    await flushPromises()
    expect(openaiOAuth.find('[data-testid="edit-openai-passthrough"]').exists()).toBe(true)
    expect(openaiOAuth.find('[data-testid="edit-openai-compact"]').exists()).toBe(true)
    expect(openaiOAuth.find('[data-testid="edit-openai-key-protocol-hint"]').exists()).toBe(false)
    openaiOAuth.unmount()

    const grokOAuth = mountModal(buildGrokOAuthAccount())
    await flushPromises()
    expect(grokOAuth.find('[data-testid="edit-openai-passthrough"]').exists()).toBe(false)
    expect(grokOAuth.find('[data-testid="edit-openai-compact"]').exists()).toBe(false)
    grokOAuth.unmount()
  })

  it('never shows the key-only Anthropic settings for subscription accounts', async () => {
    const wrapper = mountModal({ ...buildOpenAIOAuthParentAccount(), platform: 'anthropic', protocol_endpoints: undefined } as any)
    await flushPromises()
    expect(wrapper.find('[data-testid="edit-anthropic-passthrough"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="edit-anthropic-auth-scheme"]').exists()).toBe(false)
  })

  // 已上架模型是只读展示：绑定在模型目录里改，这里只告诉管理员这个资源承接哪些模型。
  it('shows the bound catalog entries read-only and marks unlisted ones', async () => {
    const entry = (id: number, model_id: string, status: string) =>
      ({ id, model_id, status, bindings: [] } as unknown as ModelCatalogEntry)
    const wrapper = mountModal(buildAccount(), [entry(199, 'gpt-5.6', 'listed'), entry(217, 'gpt-5.6-mini', 'unlisted')])
    await flushPromises()
    const section = wrapper.get('[data-testid="edit-account-catalog"]')
    const chips = section.findAll('span').filter((span) => span.text() === 'gpt-5.6' || span.text() === 'gpt-5.6-mini')
    expect(chips.map((chip) => chip.text())).toEqual(['gpt-5.6', 'gpt-5.6-mini'])
    expect(chips[1].classes()).toContain('line-through')
    expect(section.text()).not.toContain('admin.accounts.catalogNone')

    const empty = mountModal(buildAccount(), [])
    await flushPromises()
    expect(empty.get('[data-testid="edit-account-catalog"]').text()).toContain('admin.accounts.catalogNone')

    const hidden = mountModal(buildAccount(), undefined)
    await flushPromises()
    expect(hidden.find('[data-testid="edit-account-catalog"]').exists()).toBe(false)
  })

  // 分组绑定段已删：弹窗里没有分组选择器，保存也不带 group_ids
  it('has no group binding section and never submits group_ids', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset().mockResolvedValue(account)
    const wrapper = mountModal(account)
    await flushPromises()
    expect(wrapper.find('[data-tour="account-form-groups"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('admin.accounts.mixedScheduling')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]).not.toHaveProperty('group_ids')
  })
})
