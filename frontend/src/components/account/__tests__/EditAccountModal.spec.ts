import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const {
  updateAccountMock,
  authIsSimpleMode,
  getProtocolDefaultsMock
} = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  authIsSimpleMode: { value: true },
  getProtocolDefaultsMock: vi.fn(),
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
      update: updateAccountMock
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({})
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([])
    },
    modelCatalog: {
      listEntries: vi.fn().mockResolvedValue([])
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
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

import EditAccountModal from '../EditAccountModal.vue'
import { resetProtocolDefaultsCacheForTest } from '../protocolEndpoints'

// 与后端 GET /admin/accounts/protocol-defaults 同形（节选）。
const PROTOCOL_DEFAULTS = {
  protocols: ['anthropic', 'chat_completions', 'responses', 'gemini'],
  defaults: {
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
  },
  vendor_hosts: {
    'api.moonshot.cn': 'kimi',
    'api.kimi.com': 'kimi',
    'api.minimaxi.com': 'minimax',
    'api.minimax.io': 'minimax'
  }
}

beforeEach(() => {
  resetProtocolDefaultsCacheForTest()
  getProtocolDefaultsMock.mockReset().mockResolvedValue(PROTOCOL_DEFAULTS)
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
    protocol_endpoints: { responses: 'https://relay.example.com/v1' },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
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
    status: 'active',
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildRelayKeyWithoutCredentials() {
  return {
    ...buildAccount(),
    id: 6,
    name: 'Relay key',
    credentials: {},
    protocol_endpoints: { responses: 'https://relay.example.com/grok/v1' },
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

function mountModal(account = buildAccount()) {
  return mount(EditAccountModal, {
    props: {
      show: true,
      account,
      proxies: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Select: SelectStub,
        Icon: true,
        ProxySelector: true
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

  it('preserves OpenCode Zen account type and endpoints on submit', async () => {
    const account = buildAccount()
    account.platform = 'opencode_go'
    account.protocol_endpoints = { chat_completions: 'https://opencode.ai/zen/v1' }
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

  it('preserves Kimi Responses endpoint on submit', async () => {
    const account = buildAccount()
    account.platform = 'kimi'
    account.protocol_endpoints = { responses: PROTOCOL_DEFAULTS.defaults.kimi.default.responses }
    account.credentials = {
      api_key: 'sk-kimi',
      account_mode: 'payg'
    }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual({
      responses: PROTOCOL_DEFAULTS.defaults.kimi.default.responses
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toMatchObject({ account_mode: 'payg' })
  })

  it('preserves GLM endpoints on submit', async () => {
    const account = buildAccount()
    account.platform = 'zhipu'
    account.protocol_endpoints = { chat_completions: 'https://open.bigmodel.cn/api/coding/paas/v4' }
    account.credentials = {
      api_key: 'sk-glm',
      account_mode: 'coding'
    }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual({
      chat_completions: 'https://open.bigmodel.cn/api/coding/paas/v4'
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toMatchObject({ account_mode: 'coding' })
  })

  // 计费方式与新建同一规则（2026-09-28 P5 · A1-10）：地址指向中转就不写 account_mode，平台标签不算数
  it('keeps a custom CN relay address and drops account_mode on save', async () => {
    const account = buildAccount()
    account.platform = 'zhipu'
    account.protocol_endpoints = { chat_completions: 'https://relay.example.com/v1' }
    account.credentials = { api_key: 'sk-glm', account_mode: 'payg' }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await flushPromises()
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const payload = updateAccountMock.mock.calls[0]?.[1]
    expect(payload?.protocol_endpoints).toEqual({ chat_completions: 'https://relay.example.com/v1' })
    expect(payload?.credentials).toMatchObject({ api_key: 'sk-glm' })
    expect(payload?.credentials).not.toHaveProperty('account_mode')
  })

  it('has no API protocol selector for Chinese provider keys', async () => {
    // 计费方式按地址识别（与新建同一规则）：地址认得出是智谱、又定不了套餐时才出选择
    getProtocolDefaultsMock.mockReset().mockResolvedValue({ ...PROTOCOL_DEFAULTS, vendor_hosts: { 'open.bigmodel.cn': 'zhipu' } })
    const account = buildAccount()
    account.platform = 'zhipu'
    account.protocol_endpoints = { chat_completions: 'https://open.bigmodel.cn/api/paas/v4' }
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
    first.protocol_endpoints = { chat_completions: PROTOCOL_DEFAULTS.defaults.kimi.coding.chat_completions }
    first.credentials = { api_key: 'sk-kimi', account_mode: 'coding' }
    const second = buildAccount()
    second.id = 99
    second.platform = 'kimi'
    second.protocol_endpoints = { chat_completions: PROTOCOL_DEFAULTS.defaults.kimi.coding.chat_completions }
    second.credentials = { api_key: 'sk-kimi-2', account_mode: 'payg' }
    updateAccountMock.mockReset().mockResolvedValue(second)

    const wrapper = mountModal(first)
    await flushPromises()
    await wrapper.setProps({ account: second })
    await flushPromises()
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock.mock.calls[0]?.[0]).toBe(99)
    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual({
      chat_completions: PROTOCOL_DEFAULTS.defaults.kimi.coding.chat_completions
    })
  })

  it('fills the preset protocol endpoint when a Chinese provider preset is picked', async () => {
    const account = buildAccount()
    account.platform = 'minimax'
    account.protocol_endpoints = { chat_completions: 'https://api.minimaxi.com/v1' }
    account.credentials = { api_key: 'sk-minimax', account_mode: 'payg' }
    updateAccountMock.mockReset().mockResolvedValue(account)

    const wrapper = mountModal(account)
    await flushPromises()
    const preset = wrapper
      .findAll('[data-testid="cn-base-url-preset"]')
      .find(button => button.text().startsWith('MiniMax Intl Anthropic (api.minimax.io/anthropic)'))
    expect(preset).toBeDefined()
    await preset!.trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    const payload = updateAccountMock.mock.calls[0]?.[1]
    expect(payload?.credentials).not.toHaveProperty('api_protocol')
    // 一个 key 只承接一个协议：预设换成它的协议和地址
    expect(payload?.protocol_endpoints).toEqual({ anthropic: 'https://api.minimax.io/anthropic' })
  })

  // 渠道上的模型改名已删（2026-10-01，改名在价格页）：已存凭据里的旧键不带回去（后端会拒收）
  it('drops stale model_mapping keys from credentials on save', async () => {
    const account = buildAccount()
    account.credentials.model_mapping = {
      'gpt-5.2': 'gpt-5.2',
      'gpt-latest': 'gpt-5.2'
    }
    account.credentials.model_mapping_rename_only = true
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const credentials = updateAccountMock.mock.calls[0]?.[1]?.credentials
    expect(credentials).toMatchObject({ api_key: 'sk-test' })
    expect(credentials).not.toHaveProperty('model_mapping')
    expect(credentials).not.toHaveProperty('model_mapping_rename_only')
  })

  it('saves a relay key with its stored endpoints and no base_url fallback', async () => {
    const account = buildRelayKeyWithoutCredentials()
    updateAccountMock.mockReset()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)

    expect(
      (wrapper.get('[data-testid="protocol-endpoint-input-responses"]').element as HTMLInputElement).value
    ).toBe('https://relay.example.com/grok/v1')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.protocol_endpoints).toEqual({
      responses: 'https://relay.example.com/grok/v1'
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('base_url')
  })

  // spark 影子号的 model_mapping 是系统维护的模型列表：表单不改它，也不带 credentials（带了后端会整份替换）
  it('submits no credentials when saving an OpenAI spark shadow account', async () => {
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
    expect(payload).not.toHaveProperty('credentials')
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

  it('allows saving apikey account when backend redacted api_key but credentials_status reports it exists', async () => {
    // 新前端 + 新后端：响应已脱敏，credentials 里没有 api_key，credentials_status.has_api_key=true
    const account = buildAccount()
    account.credentials = {
      base_url: 'https://relay.example.com/v1',
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
      base_url: 'https://relay.example.com/v1'
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

  it('仅对 OpenAI OAuth 母账号显示，默认关闭', () => {
    const parent = mountModal(buildOpenAIOAuthParentAccount())
    expect(parent.find('[data-testid="auto-reset-credit-settings"]').exists()).toBe(true)
    expect(parent.get('[data-testid="auto-reset-credit-enabled"]').classes()).toContain('bg-af-hairline')
    parent.unmount()

    for (const account of [buildAccount(), buildOpenAISetupTokenAccount(), buildOpenAISparkShadowAccount()]) {
      const wrapper = mountModal(account)
      expect(wrapper.find('[data-testid="auto-reset-credit-settings"]').exists()).toBe(false)
      wrapper.unmount()
    }
  })

  it('保存开关，并禁止把运行态回写到管理请求', async () => {
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
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    const extra = updateAccountMock.mock.calls[0]?.[1]?.extra
    expect(extra).toMatchObject({
      auto_reset_credit_enabled: true
    })
    expect(extra).not.toHaveProperty('codex_auto_reset_credit_state')
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
    const wrapper = mountModal(buildKey('gemini', { gemini: 'https://relay.example.com/gemini' }))

    const section = wrapper.get('[data-testid="edit-header-override"]')
    const addRow = section.findAll('button').find((button) => button.text().includes('admin.accounts.headerOverride.addRow'))
    expect(addRow).toBeDefined()
    await addRow!.trigger('click')
    const [name, value] = section.findAll('input[type="text"]')
    await name.setValue('X-Relay-Tenant')
    await value.setValue('team-a')

    const payload = await submitPayload(wrapper)
    expect(payload?.credentials).toMatchObject({
      header_overrides: { 'x-relay-tenant': 'team-a' }
    })
    expect(payload?.credentials).not.toHaveProperty('header_override_enabled')
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
    const wrapper = mountModal(buildKey('kimi', { anthropic: 'https://api.moonshot.cn/anthropic' }))
    await flushPromises()

    await wrapper.get('[data-testid="edit-anthropic-auth-scheme"]').setValue('authorization_bearer')
    await wrapper.get('[data-testid="edit-bedrock-cc-compat-toggle"]').trigger('click')

    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({
      anthropic_apikey_auth_scheme: 'authorization_bearer',
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

  it('drops bedrock_cc_compat from extra when the toggle is off', async () => {
    const wrapper = mountModal(buildKey(
      'anthropic',
      { anthropic: 'https://relay.example.com/anthropic' },
      { bedrock_cc_compat: false }
    ))
    await flushPromises()

    const payload = await submitPayload(wrapper)
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

  it('follows the protocol switched in the modal and does not submit hidden Anthropic edits', async () => {
    const wrapper = mountModal(buildKey('kimi', { chat_completions: 'https://api.moonshot.cn/v1' }))
    await flushPromises()
    expect(wrapper.find('[data-testid="edit-anthropic-auth-scheme"]').exists()).toBe(false)

    await wrapper.get('[data-testid="protocol-endpoint-protocol"]').setValue('anthropic')
    expect(wrapper.find('[data-testid="edit-anthropic-auth-scheme"]').exists()).toBe(true)
    await wrapper.get('[data-testid="edit-anthropic-auth-scheme"]').setValue('authorization_bearer')

    await wrapper.get('[data-testid="protocol-endpoint-protocol"]').setValue('chat_completions')
    expect(wrapper.find('[data-testid="edit-anthropic-auth-scheme"]').exists()).toBe(false)

    const payload = await submitPayload(wrapper)
    expect(payload?.extra ?? {}).not.toHaveProperty('anthropic_apikey_auth_scheme')
  })

  it('saves Anthropic settings for an OpenAI-labelled key with an anthropic endpoint', async () => {
    const wrapper = mountModal(buildKey('openai', { anthropic: 'https://relay.example.com' }))
    await flushPromises()

    await wrapper.get('[data-testid="edit-anthropic-auth-scheme"]').setValue('authorization_bearer')

    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({ anthropic_apikey_auth_scheme: 'authorization_bearer' })
  })

  it('never shows the key-only Anthropic settings for subscription accounts', async () => {
    const wrapper = mountModal({ ...buildOpenAIOAuthParentAccount(), platform: 'anthropic', protocol_endpoints: undefined } as any)
    await flushPromises()
    expect(wrapper.find('[data-testid="edit-anthropic-passthrough"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="edit-anthropic-auth-scheme"]').exists()).toBe(false)
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
