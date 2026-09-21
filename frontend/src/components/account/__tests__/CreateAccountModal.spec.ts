import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const {
  createAccountMock,
  probeUpstreamBillingMock,
  syncUpstreamModelsMock,
  showWarningMock,
  importCodexSessionMock,
  createOpenAICodexPATMock,
  authIsSimpleMode,
  getProtocolDefaultsMock,
  showErrorMock,
} = vi.hoisted(() => ({
  createAccountMock: vi.fn(),
  probeUpstreamBillingMock: vi.fn(),
  syncUpstreamModelsMock: vi.fn(),
  showWarningMock: vi.fn(),
  importCodexSessionMock: vi.fn(),
  createOpenAICodexPATMock: vi.fn(),
  authIsSimpleMode: { value: true },
  getProtocolDefaultsMock: vi.fn(),
  showErrorMock: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: vi.fn(),
    showWarning: showWarningMock,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    get isSimpleMode() {
      return authIsSimpleMode.value
    },
  }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      create: createAccountMock,
      probeUpstreamBilling: probeUpstreamBillingMock,
      syncUpstreamModels: syncUpstreamModelsMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false }),
      importCodexSession: importCodexSessionMock,
      createOpenAICodexPAT: createOpenAICodexPATMock,
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({}),
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([]),
    },
  },
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn().mockResolvedValue([]),
  accountsAPI: { getProtocolDefaults: getProtocolDefaultsMock },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import { adminAPI } from '@/api/admin'
import CreateAccountModal from '../CreateAccountModal.vue'
import { resetProtocolDefaultsCacheForTest } from '../protocolEndpoints'

// 与后端 GET /admin/accounts/protocol-defaults 同形；取自开发实例的真实返回（节选）。
const PROTOCOL_DEFAULTS = {
  protocols: ['anthropic', 'chat_completions', 'responses', 'gemini'],
  defaults: {
    anthropic: { default: { anthropic: 'https://api.anthropic.com' } },
    openai: { default: { chat_completions: 'https://api.openai.com', responses: 'https://api.openai.com' } },
    grok: { default: { chat_completions: 'https://api.x.ai/v1', responses: 'https://api.x.ai/v1' } },
    kimi: {
      default: {
        anthropic: 'https://api.moonshot.cn/anthropic',
        chat_completions: 'https://api.moonshot.cn/v1',
        responses: 'https://api.moonshot.cn/v1',
      },
      coding: {
        anthropic: 'https://api.kimi.com/coding',
        chat_completions: 'https://api.kimi.com/coding/v1',
        responses: 'https://api.kimi.com/coding/v1',
      },
    },
    minimax: {
      default: {
        anthropic: 'https://api.minimaxi.com/anthropic',
        chat_completions: 'https://api.minimaxi.com/v1',
        responses: 'https://api.minimaxi.com/v1',
      },
    },
    opencode_go: {
      zen: {
        anthropic: 'https://opencode.ai/zen',
        chat_completions: 'https://opencode.ai/zen/v1',
        responses: 'https://opencode.ai/zen/v1',
      },
      go: {
        anthropic: 'https://opencode.ai/zen/go',
        chat_completions: 'https://opencode.ai/zen/go/v1',
        responses: 'https://opencode.ai/zen/go/v1',
      },
    },
  },
}

beforeEach(() => {
  resetProtocolDefaultsCacheForTest()
  getProtocolDefaultsMock.mockReset().mockResolvedValue(PROTOCOL_DEFAULTS)
  showErrorMock.mockReset()
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

const OAuthAuthorizationFlowStub = defineComponent({
  name: 'OAuthAuthorizationFlow',
  props: {
    showManualOption: Boolean,
    showCodexSessionImportOption: Boolean,
    showAgentIdentityOption: Boolean,
    showCodexPatOption: Boolean,
    initialInputMethod: String,
  },
  data: () => ({ inputMethod: 'manual' }),
  emits: ['import-codex-session', 'import-codex-pat'],
  template: `
    <div>
      <button data-testid="import-codex-session" @click="$emit('import-codex-session', 'session-json')">session</button>
      <button data-testid="import-codex-pat" @click="$emit('import-codex-pat', 'pat-token')">pat</button>
    </div>
  `,
})

const GroupSelectorStub = defineComponent({
  name: 'GroupSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['update:modelValue'],
  template: `
    <button
      type="button"
      data-testid="select-pricing-groups"
      @click="$emit('update:modelValue', [1, 2])"
    >
      groups
    </button>
  `,
})

const ModelWhitelistSelectorStub = defineComponent({
  name: 'ModelWhitelistSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => [],
    },
    platform: String,
    syncCredentials: Object,
  },
  emits: ['update:modelValue', 'upstream-synced'],
  template: `<button
    type="button"
    data-testid="model-whitelist-selector"
    @click="$emit('update:modelValue', ['public-glm']); $emit('upstream-synced')"
  >models</button>`,
})

function mountModal(groups: any[] = []) {
  return mount(CreateAccountModal, {
    props: { show: true, proxies: [], groups },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
        ConfirmDialog: true,
        Select: true,
        Icon: true,
        PlatformIcon: true,
        ProxySelector: true,
        ProxyAdBanner: true,
        GroupSelector: GroupSelectorStub,
        ModelWhitelistSelector: ModelWhitelistSelectorStub,
        QuotaLimitCard: true,
      },
    },
  })
}

async function selectButtonByText(wrapper: ReturnType<typeof mountModal>, text: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text().includes(text))
  expect(button).toBeDefined()
  await button?.trigger('click')
}

async function submitApiKeyAccount(
  platform: 'openai' | 'anthropic',
  disableUpstreamBillingProbe = false
) {
  const wrapper = mountModal()
  await selectButtonByText(wrapper, platform === 'openai' ? 'OpenAI' : 'admin.accounts.claudeConsole')
  if (platform === 'openai') {
    await selectButtonByText(wrapper, 'API Key')
  }
  await wrapper.get('form#create-account-form input[type="text"]').setValue(`${platform} account`)
  await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
  if (disableUpstreamBillingProbe) {
    await wrapper.get('[data-testid="upstream-billing-auto-probe"]').trigger('click')
  }
  await wrapper.get('form#create-account-form').trigger('submit.prevent')
  await flushPromises()
  return wrapper
}

async function openCodexImportStep() {
  const wrapper = mountModal()
  await selectButtonByText(wrapper, 'OpenAI')
  await wrapper.get('form#create-account-form input[type="text"]').setValue('Codex import')
  await wrapper.get('form#create-account-form').trigger('submit.prevent')
  return wrapper
}

describe('CreateAccountModal OpenAI account creation', () => {
  beforeEach(() => {
    authIsSimpleMode.value = true
    createAccountMock.mockReset().mockResolvedValue({ id: 42, platform: 'openai', type: 'apikey' })
    probeUpstreamBillingMock.mockReset().mockResolvedValue({})
    syncUpstreamModelsMock.mockReset().mockResolvedValue({ models: [], metadata: {} })
    showWarningMock.mockReset()
    importCodexSessionMock.mockReset().mockResolvedValue({
      created: 1,
      updated: 0,
      skipped: 0,
      failed: 0,
      errors: [],
      warnings: [],
    })
    createOpenAICodexPATMock.mockReset().mockResolvedValue({})
  })

  afterEach(() => vi.useRealTimers())

  it('sets month and year expiry presets without submitting the account form', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-01-31T12:34:00'))
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('expiry account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
    const input = wrapper.get<HTMLInputElement>('input[type="datetime-local"]')

    for (const [label, expected] of [
      ['payment.oneMonth', '2026-02-28T12:34'],
      ['payment.oneYear', '2027-01-31T12:34'],
    ]) {
      const button = wrapper.findAll('button').find((candidate) => candidate.text() === label)!
      expect(button.attributes('type')).toBe('button')
      await button.trigger('click')
      expect(input.element.value).toBe(expected)
      expect(createAccountMock).not.toHaveBeenCalled()
    }

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(createAccountMock.mock.calls[0]?.[0]?.expires_at).toBe(new Date('2027-01-31T12:34:00').getTime() / 1000)
    wrapper.unmount()
  })

  it('allows a manually entered expiry to override a preset before account creation', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('custom expiry account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
    await selectButtonByText(wrapper, 'payment.oneMonth')
    await wrapper.get('input[type="datetime-local"]').setValue('2030-04-15T09:20')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(createAccountMock.mock.calls[0]?.[0]?.expires_at).toBe(new Date('2030-04-15T09:20:00').getTime() / 1000)
    wrapper.unmount()
  })

  it('omits the upstream request id header from extra when left empty', async () => {
    await submitApiKeyAccount('openai')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra).not.toHaveProperty('upstream_request_id_header')
  })

  it('sends the trimmed upstream request id header in extra when filled', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('openai account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
    await wrapper.get('[data-testid="upstream-request-id-header"]').setValue('  X-Oneapi-Request-Id  ')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.upstream_request_id_header).toBe('X-Oneapi-Request-Id')
  })

  it('omits images_url_to_b64_json from extra by default', async () => {
    await submitApiKeyAccount('openai')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra).not.toHaveProperty('images_url_to_b64_json')
  })

  it('sends images_url_to_b64_json in extra when the toggle is enabled', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('openai account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
    await wrapper.get('[data-testid="openai-images-url-to-b64-json-toggle"]').trigger('click')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.extra?.images_url_to_b64_json).toBe(true)
  })

  it('persists upstream model metadata after creating an account from preview', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('OpenCode account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
    await wrapper.get('[data-testid="model-whitelist-selector"]').trigger('click')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledOnce()
    expect(syncUpstreamModelsMock).toHaveBeenCalledWith(42)
  })

  it('includes the current concrete model mapping in preview credentials', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
    await wrapper.get('[data-testid="model-whitelist-selector"]').trigger('click')
    await flushPromises()

    expect(wrapper.getComponent(ModelWhitelistSelectorStub).props('syncCredentials')).toMatchObject({
      model_mapping: { 'public-glm': 'public-glm' }
    })
  })

  it('runs formal capability sync after creating an account with explicit mappings', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('Mapped account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
    await selectButtonByText(wrapper, 'admin.accounts.modelMapping')
    await selectButtonByText(wrapper, 'admin.accounts.addMapping')
    await wrapper.get('input[placeholder="admin.accounts.requestModel"]').setValue('public-glm')
    await wrapper.get('input[placeholder="admin.accounts.actualModel"]').setValue('glm-5.3')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock.mock.calls[0]?.[0]?.credentials?.model_mapping).toEqual({
      'public-glm': 'glm-5.3'
    })
    expect(syncUpstreamModelsMock).toHaveBeenCalledWith(42)
  })

  it('warns when post-create capability metadata remains incomplete', async () => {
    syncUpstreamModelsMock.mockResolvedValue({
      models: ['x-preview-f-free'],
      warnings: [{ code: 'upstream_model_metadata_incomplete', message: 'metadata incomplete' }],
    })
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('OpenCode account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
    await wrapper.get('[data-testid="model-whitelist-selector"]').trigger('click')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(showWarningMock).toHaveBeenCalledWith(
      'admin.accounts.syncUpstreamModelsMetadataIncomplete'
    )
  })

  // namespace 摊平是仅 OAuth 的兼容开关：API Key 走 chat completions 回退桥时由桥自行摊平
  it('shows the Codex namespace flatten toggle only for OpenAI OAuth accounts', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')

    expect(wrapper.find('[data-testid="create-openai-flatten-namespaces-toggle"]').exists()).toBe(
      true
    )

    await selectButtonByText(wrapper, 'API Key')
    expect(wrapper.find('[data-testid="create-openai-flatten-namespaces-toggle"]').exists()).toBe(
      false
    )
  })

  it('enables upstream billing probes by default for new OpenAI API key accounts', async () => {
    await submitApiKeyAccount('openai')

    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(true)
  })

  it('waits for the initial upstream billing probe before refreshing the account list', async () => {
    let resolveProbe: (() => void) | undefined
    probeUpstreamBillingMock.mockImplementationOnce(
      () => new Promise<void>((resolve) => {
        resolveProbe = resolve
      })
    )

    const wrapper = await submitApiKeyAccount('openai')

    expect(probeUpstreamBillingMock).toHaveBeenCalledWith(42)
    expect(wrapper.emitted('created')).toBeUndefined()

    resolveProbe?.()
    await flushPromises()

    expect(wrapper.emitted('created')).toHaveLength(1)
  })

  it('sends an explicit disabled state when the create toggle is turned off', async () => {
    await submitApiKeyAccount('openai', true)

    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(false)
    expect(probeUpstreamBillingMock).not.toHaveBeenCalled()
  })

  it('submits OpenCode Zen default protocol rules with adaptive endpoints', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenCode')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('oc')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-opencode-zen')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual(PROTOCOL_DEFAULTS.defaults.opencode_go.zen)
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('base_url')
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_base_urls')
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({
      account_mode: 'zen',
      protocol_rules: [
        { pattern: 'grok-*', protocol: 'responses' },
        { pattern: 'gpt-*', protocol: 'responses' },
        { pattern: 'muse-spark-*', protocol: 'responses' },
        { pattern: 'claude-*', protocol: 'anthropic' },
        { pattern: 'qwen*', protocol: 'anthropic' }
      ]
    })
  })

  it('submits OpenCode GO endpoints after switching account type', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenCode')
    await selectButtonByText(wrapper, 'admin.accounts.opencodeGo.accountMode.go')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('oc-go')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-opencode-go')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual(PROTOCOL_DEFAULTS.defaults.opencode_go.go)
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({
      account_mode: 'go',
      protocol_rules: [
        { pattern: 'grok-*', protocol: 'responses' },
        { pattern: 'gpt-*', protocol: 'responses' },
        { pattern: 'muse-spark-*', protocol: 'responses' },
        { pattern: 'minimax-*', protocol: 'anthropic' },
        { pattern: 'qwen*', protocol: 'anthropic' }
      ]
    })
  })

  it('submits Kimi protocol endpoints without an API protocol', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('Kimi adaptive')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-kimi')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual(PROTOCOL_DEFAULTS.defaults.kimi.default)
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({ account_mode: 'payg' })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
  })

  it('submits Kimi Coding Plan Responses endpoint', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await selectButtonByText(wrapper, 'admin.accounts.cnProviders.accountMode.coding')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('Kimi coding')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-kimi-coding')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual(PROTOCOL_DEFAULTS.defaults.kimi.coding)
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({ account_mode: 'coding' })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
  })

  it('submits MiniMax protocol endpoints', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'MiniMax')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('MiniMax adaptive')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-minimax')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual(PROTOCOL_DEFAULTS.defaults.minimax.default)
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({ account_mode: 'payg' })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
  })

  it('previews upstream models with the edited protocol endpoints', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await flushPromises()
    await wrapper
      .get('[data-testid="protocol-endpoint-input-chat_completions"]')
      .setValue('https://relay.example.com/v1')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-relay')

    const syncCredentials = wrapper.getComponent(ModelWhitelistSelectorStub).props('syncCredentials')
    expect(syncCredentials).toMatchObject({
      platform: 'kimi',
      type: 'apikey',
      api_key: 'sk-relay',
      protocol_endpoints: {
        ...PROTOCOL_DEFAULTS.defaults.kimi.default,
        chat_completions: 'https://relay.example.com/v1'
      }
    })
    expect(syncCredentials).not.toHaveProperty('base_url')
  })

  it('submits the official endpoints prefilled from the backend for an OpenAI API key', async () => {
    await submitApiKeyAccount('openai')

    const payload = createAccountMock.mock.calls[0]?.[0]
    expect(payload?.protocol_endpoints).toEqual(PROTOCOL_DEFAULTS.defaults.openai.default)
    expect(payload?.credentials).not.toHaveProperty('base_url')
  })

  it('refuses to create a third-party key without any protocol endpoint', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'admin.accounts.claudeConsole')
    await flushPromises()
    await wrapper.get('[data-testid="protocol-endpoint-remove-anthropic"]').trigger('click')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('no endpoint')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-test')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).not.toHaveBeenCalled()
    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.protocolEndpoints.errors.empty')
  })

  it('has no API protocol selector: a Chinese provider key keeps whichever endpoints it configures', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await flushPromises()
    expect(wrapper.text()).not.toContain('admin.accounts.cnProviders.apiProtocol.title')
    await wrapper.get('[data-testid="protocol-endpoint-remove-anthropic"]').trigger('click')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('kimi without anthropic')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-kimi')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    const payload = createAccountMock.mock.calls[0]?.[0]
    expect(payload?.protocol_endpoints).not.toHaveProperty('anthropic')
    expect(payload?.credentials).not.toHaveProperty('api_protocol')
  })

  it('switches to the new official endpoints on mode change but keeps edited ones', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await flushPromises()
    await selectButtonByText(wrapper, 'admin.accounts.cnProviders.accountMode.coding')
    await flushPromises()
    expect(
      (wrapper.get('[data-testid="protocol-endpoint-input-chat_completions"]').element as HTMLInputElement).value
    ).toBe('https://api.kimi.com/coding/v1')

    await wrapper.get('[data-testid="protocol-endpoint-input-chat_completions"]').setValue('https://relay.example.com/v1')
    await selectButtonByText(wrapper, 'admin.accounts.cnProviders.accountMode.payg')
    await flushPromises()

    expect(
      (wrapper.get('[data-testid="protocol-endpoint-input-chat_completions"]').element as HTMLInputElement).value
    ).toBe('https://relay.example.com/v1')
  })

  it('fills only the preset protocol when a Chinese provider preset is picked', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'MiniMax')
    await flushPromises()
    // 选一个与官方预填不同的地址（国际站），才能区分「预设回填」与「官方预填」。
    const preset = wrapper
      .findAll('[data-testid="cn-base-url-preset"]')
      .find((button) => button.text().startsWith('MiniMax Intl Anthropic (api.minimax.io/anthropic)'))
    expect(preset).toBeDefined()
    await preset!.trigger('click')
    await flushPromises()

    await wrapper.get('form#create-account-form input[type="text"]').setValue('minimax preset')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-minimax')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    const payload = createAccountMock.mock.calls[0]?.[0]
    expect(payload?.credentials).toMatchObject({ account_mode: 'payg' })
    expect(payload?.credentials).not.toHaveProperty('api_protocol')
    expect(payload?.protocol_endpoints).toEqual({
      ...PROTOCOL_DEFAULTS.defaults.minimax.default,
      anthropic: 'https://api.minimax.io/anthropic'
    })
  })

  it('applies a Grok preset to both Chat Completions and Responses endpoints', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Grok')
    await wrapper.get('[data-testid="grok-account-type-api-key"]').trigger('click')
    await flushPromises()
    const preset = wrapper.findAll('[data-testid="grok-base-url-preset"]').find((button) => button.text().includes('us-east-1'))
    expect(preset).toBeDefined()
    await preset!.trigger('click')

    await wrapper.get('form#create-account-form input[type="text"]').setValue('grok preset')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('xai-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual({
      chat_completions: 'https://us-east-1.api.x.ai/v1',
      responses: 'https://us-east-1.api.x.ai/v1'
    })
  })

  it('asks for manual endpoints when official addresses fail to load', async () => {
    getProtocolDefaultsMock.mockReset().mockRejectedValue(new Error('offline'))
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'admin.accounts.claudeConsole')
    await flushPromises()

    expect(wrapper.find('[data-testid="protocol-defaults-load-failed"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="protocol-endpoint-input-anthropic"]').exists()).toBe(false)
  })

  it('exposes Agent Identity in the OpenAI authorization methods', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('OpenAI account')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')

    const flow = wrapper.getComponent(OAuthAuthorizationFlowStub)
    expect(flow.props('showManualOption')).toBe(true)
    expect(flow.props('showCodexSessionImportOption')).toBe(true)
    expect(flow.props('showAgentIdentityOption')).toBe(true)
    expect(flow.props('showCodexPatOption')).toBe(true)
    expect(flow.props('initialInputMethod')).toBe('manual')
  })

  it.each([
    ['camelCase', { authMode: 'agentIdentity', agentIdentity: { agentRuntimeId: 'runtime' } }],
    ['nested identity without auth_mode', { agent_identity: { agent_runtime_id: 'runtime' } }],
  ])('accepts backend-compatible %s Agent Identity imports', async (_name, content) => {
    const wrapper = await openCodexImportStep()
    const flow = wrapper.getComponent(OAuthAuthorizationFlowStub)
    flow.vm.inputMethod = 'agent_identity'

    flow.vm.$emit('import-codex-session', JSON.stringify(content))
    await flushPromises()

    expect(importCodexSessionMock).toHaveBeenCalledTimes(1)
  })

  it('enables the upstream billing probe by default for non-OpenAI account creation', async () => {
    await submitApiKeyAccount('anthropic')

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    // 上游倍率探测已放宽到全部 API-key 平台：非 OpenAI 平台与 OpenAI 一致，默认开启。
    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(true)
  })

  it('sends an explicit disabled state when the non-OpenAI create toggle is turned off', async () => {
    await submitApiKeyAccount('anthropic', true)

    expect(createAccountMock.mock.calls[0]?.[0]?.upstream_billing_probe_enabled).toBe(false)
  })

  it('antigravity 第三方 key 不带成品号的混合调度 / 超量键', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Antigravity')
    // 先在成品号（OAuth）形态下勾上两项，再切到第三方 key：残留的勾选不能写进 key 的 extra。
    const checkboxes = wrapper.findAll('form#create-account-form input[type="checkbox"]')
    for (const checkbox of checkboxes) {
      await checkbox.setValue(true)
    }
    await selectButtonByText(wrapper, 'admin.accounts.types.antigravityApikey')
    expect(wrapper.text()).not.toContain('admin.accounts.mixedScheduling')
    expect(wrapper.text()).not.toContain('admin.accounts.allowOverages')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('antigravity relay')
    await flushPromises()
    await wrapper.get('[data-testid="protocol-endpoint-add-anthropic"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example/antigravity')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-upstream')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    const extra = createAccountMock.mock.calls[0]?.[0]?.extra ?? {}
    expect(extra).not.toHaveProperty('mixed_scheduling')
    expect(extra).not.toHaveProperty('allow_overages')
  })

  it('antigravity upstream 创建默认携带上游倍率探测开关', async () => {
    // antigravity upstream 走独立创建 helper，
    // 也必须与其余 API-key 平台一样默认开启探测并传递开关。
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Antigravity')
    await selectButtonByText(wrapper, 'admin.accounts.types.antigravityApikey')
    await wrapper.get('form#create-account-form input[type="text"]').setValue('antigravity relay')
    await flushPromises()
    await wrapper.get('[data-testid="protocol-endpoint-add-anthropic"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example/antigravity')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-upstream')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    const payload = createAccountMock.mock.calls[0]?.[0]
    expect(payload?.platform).toBe('antigravity')
    expect(payload?.type).toBe('apikey')
    expect(payload?.protocol_endpoints).toEqual({ anthropic: 'https://relay.example/antigravity' })
    expect(payload?.credentials).not.toHaveProperty('base_url')
    expect(payload?.upstream_billing_probe_enabled).toBe(true)
    // 创建成功后前端立即发起一次首探（与其他 apikey 平台一致）。
    expect(probeUpstreamBillingMock).toHaveBeenCalledWith(42)
  })

  it('imports a Codex session without any billing extra', async () => {
    const wrapper = await openCodexImportStep()
    await wrapper.get('[data-testid="import-codex-session"]').trigger('click')
    await flushPromises()

    expect(importCodexSessionMock).toHaveBeenCalledTimes(1)
    expect(importCodexSessionMock.mock.calls[0]?.[0]?.extra ?? {}).not.toHaveProperty('openai_long_context_billing_enabled')
  })

  it('imports a Codex PAT without any billing extra', async () => {
    const wrapper = await openCodexImportStep()
    await wrapper.get('[data-testid="import-codex-pat"]').trigger('click')
    await flushPromises()

    expect(createOpenAICodexPATMock).toHaveBeenCalledTimes(1)
    expect(createOpenAICodexPATMock.mock.calls[0]?.[0]?.extra ?? {}).not.toHaveProperty('openai_long_context_billing_enabled')
  })

})

describe('CreateAccountModal third-party key settings do not follow the platform label', () => {
  beforeEach(() => {
    authIsSimpleMode.value = true
    createAccountMock.mockReset().mockResolvedValue({ id: 42, platform: 'antigravity', type: 'apikey' })
    probeUpstreamBillingMock.mockReset().mockResolvedValue({})
    syncUpstreamModelsMock.mockReset().mockResolvedValue({ models: [], metadata: {} })
  })

  async function fillKeyBasics(wrapper: ReturnType<typeof mountModal>, name: string) {
    await wrapper.get('form#create-account-form input[type="text"]').setValue(name)
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-test')
  }

  async function submitPayload(wrapper: ReturnType<typeof mountModal>) {
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(createAccountMock).toHaveBeenCalledTimes(1)
    return createAccountMock.mock.calls[0]?.[0]
  }

  it('offers header overrides for an Antigravity upstream key and submits them', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Antigravity')
    await selectButtonByText(wrapper, 'admin.accounts.types.antigravityApikey')
    await flushPromises()
    await wrapper.get('[data-testid="protocol-endpoint-add-anthropic"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example/antigravity')
    await fillKeyBasics(wrapper, 'antigravity relay')

    await wrapper.get('[data-testid="create-header-override-toggle"]').trigger('click')
    const section = wrapper.get('[data-testid="create-header-override"]')
    await selectButtonByText(wrapper, 'admin.accounts.headerOverride.addRow')
    const [name, value] = section.findAll('input[type="text"]')
    await name.setValue('X-Relay-Tenant')
    await value.setValue('team-a')

    const payload = await submitPayload(wrapper)
    expect(payload?.type).toBe('apikey')
    expect(payload?.credentials).toMatchObject({
      header_override_enabled: true,
      header_overrides: { 'x-relay-tenant': 'team-a' }
    })
  })

  it('keeps header overrides limited to Grok OAuth among subscription accounts', async () => {
    const wrapper = mountModal()
    // 默认是 Anthropic OAuth 成品号
    expect(wrapper.find('[data-testid="create-header-override"]').exists()).toBe(false)
    await selectButtonByText(wrapper, 'Grok')
    expect(wrapper.find('[data-testid="create-header-override"]').exists()).toBe(true)
  })

  it('shows Anthropic protocol settings for a Kimi key with an anthropic endpoint and submits them', async () => {
    vi.mocked(adminAPI.settings.getWebSearchEmulationConfig).mockResolvedValueOnce({ enabled: true, providers: [{}] } as any)
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await flushPromises()
    await fillKeyBasics(wrapper, 'kimi relay')

    await wrapper.get('[data-testid="create-anthropic-passthrough-toggle"]').trigger('click')
    await wrapper.get('[data-testid="create-anthropic-auth-scheme"]').setValue('authorization_bearer')
    await wrapper.get('[data-testid="create-web-search-emulation"] select').setValue('enabled')

    const payload = await submitPayload(wrapper)
    expect(payload?.platform).toBe('kimi')
    expect(payload?.extra).toMatchObject({
      anthropic_passthrough: true,
      anthropic_apikey_auth_scheme: 'authorization_bearer',
      web_search_emulation: 'enabled'
    })
  })

  it('hides Anthropic protocol settings once an Anthropic-labelled key drops its anthropic endpoint', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'admin.accounts.claudeConsole')
    await flushPromises()
    expect(wrapper.find('[data-testid="create-anthropic-passthrough"]').exists()).toBe(true)
    await wrapper.get('[data-testid="create-anthropic-passthrough-toggle"]').trigger('click')
    await wrapper.get('[data-testid="create-anthropic-auth-scheme"]').setValue('authorization_bearer')

    await wrapper.get('[data-testid="protocol-endpoint-remove-anthropic"]').trigger('click')
    expect(wrapper.find('[data-testid="create-anthropic-passthrough"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-anthropic-auth-scheme"]').exists()).toBe(false)

    await wrapper.get('[data-testid="protocol-endpoint-add-chat_completions"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-input-chat_completions"]').setValue('https://relay.example.com/v1')
    await fillKeyBasics(wrapper, 'anthropic label without anthropic endpoint')
    const payload = await submitPayload(wrapper)
    expect(payload?.extra ?? {}).not.toHaveProperty('anthropic_passthrough')
    expect(payload?.extra ?? {}).not.toHaveProperty('anthropic_apikey_auth_scheme')
  })

  it('clears Anthropic protocol settings when switching to another platform label', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await flushPromises()
    await wrapper.get('[data-testid="create-anthropic-passthrough-toggle"]').trigger('click')
    await wrapper.get('[data-testid="create-anthropic-auth-scheme"]').setValue('authorization_bearer')

    // MiniMax 的官方地址同样带 anthropic，区块一直可见
    await selectButtonByText(wrapper, 'MiniMax')
    await flushPromises()
    expect(wrapper.find('[data-testid="create-anthropic-passthrough"]').exists()).toBe(true)
    await fillKeyBasics(wrapper, 'minimax after kimi')

    const payload = await submitPayload(wrapper)
    expect(payload?.platform).toBe('minimax')
    expect(payload?.extra ?? {}).not.toHaveProperty('anthropic_passthrough')
    expect(payload?.extra ?? {}).not.toHaveProperty('anthropic_apikey_auth_scheme')
  })

  it('submits Anthropic protocol settings for an Antigravity upstream key', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Antigravity')
    await selectButtonByText(wrapper, 'admin.accounts.types.antigravityApikey')
    await flushPromises()
    await wrapper.get('[data-testid="protocol-endpoint-add-anthropic"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example/antigravity')
    await fillKeyBasics(wrapper, 'antigravity relay')

    await wrapper.get('[data-testid="create-anthropic-passthrough-toggle"]').trigger('click')

    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({ anthropic_passthrough: true })
  })

  it('submits endpoint capabilities and the b64 toggle for a Kimi key with an OpenAI endpoint', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Kimi')
    await flushPromises()
    await fillKeyBasics(wrapper, 'kimi relay capabilities')

    expect(wrapper.find('[data-testid="openai-endpoint-capability-embeddings"]').exists()).toBe(true)
    await wrapper.get('[data-testid="openai-endpoint-capability-embeddings"]').setValue(false)
    await wrapper.get('[data-testid="openai-images-url-to-b64-json-toggle"]').trigger('click')

    const payload = await submitPayload(wrapper)
    expect(payload?.platform).toBe('kimi')
    expect(payload?.credentials?.openai_capabilities).toEqual(['chat_completions'])
    expect(payload?.extra?.images_url_to_b64_json).toBe(true)
  })

  it('hides endpoint capabilities and the b64 toggle for an Anthropic-labelled key without an OpenAI endpoint', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'admin.accounts.claudeConsole')
    await flushPromises()
    expect(wrapper.find('[data-testid="openai-endpoint-capability-embeddings"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="openai-images-url-to-b64-json-toggle"]').exists()).toBe(false)

    await wrapper.get('[data-testid="protocol-endpoint-add-chat_completions"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-input-chat_completions"]').setValue('https://relay.example.com/v1')
    expect(wrapper.find('[data-testid="openai-endpoint-capability-embeddings"]').exists()).toBe(true)
  })

  it('shows OpenAI Responses settings with the vendor hint once an Anthropic-labelled key gains a responses endpoint', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'admin.accounts.claudeConsole')
    await flushPromises()
    expect(wrapper.find('[data-testid="create-openai-passthrough"]').exists()).toBe(false)

    await wrapper.get('[data-testid="protocol-endpoint-add-responses"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-input-responses"]').setValue('https://relay.example.com/v1')
    expect(wrapper.find('[data-testid="create-openai-key-protocol-hint"]').exists()).toBe(true)
    wrapper.get('[data-testid="create-openai-ws-mode"]').findComponent({ name: 'Select' }).vm.$emit('update:modelValue', 'ctx_pool')
    await wrapper.get('[data-testid="create-openai-passthrough-toggle"]').trigger('click')
    expect(wrapper.text()).toContain('admin.accounts.openai.modelRestrictionDisabledByPassthrough')
    const compact = wrapper.get('[data-testid="create-openai-compact"]')
    const addCompactMapping = compact.findAll('button').find((button) => button.text().includes('admin.accounts.addMapping'))
    expect(addCompactMapping).toBeDefined()
    await addCompactMapping!.trigger('click')
    const [from, to] = compact.findAll('input[type="text"]')
    await from.setValue('gpt-5.4')
    await to.setValue('gpt-5.4-compact')
    await fillKeyBasics(wrapper, 'anthropic label with responses')

    const payload = await submitPayload(wrapper)
    expect(payload?.platform).toBe('anthropic')
    expect(payload?.extra).toMatchObject({
      openai_passthrough: true,
      openai_apikey_responses_websockets_v2_mode: 'ctx_pool',
      openai_apikey_responses_websockets_v2_enabled: true
    })
    expect(payload?.credentials?.compact_model_mapping).toEqual({ 'gpt-5.4': 'gpt-5.4-compact' })
  })

  it('hides OpenAI Responses settings once an OpenAI-labelled key drops its responses and chat_completions endpoints', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await flushPromises()
    await wrapper.get('[data-testid="create-openai-passthrough-toggle"]').trigger('click')

    await wrapper.get('[data-testid="protocol-endpoint-remove-chat_completions"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-remove-responses"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-add-anthropic"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example.com')
    expect(wrapper.find('[data-testid="create-openai-passthrough"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-openai-ws-mode"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-openai-compact"]').exists()).toBe(false)
    await fillKeyBasics(wrapper, 'openai label without openai endpoints')

    const payload = await submitPayload(wrapper)
    expect(payload?.extra ?? {}).not.toHaveProperty('openai_passthrough')
    expect(payload?.extra ?? {}).not.toHaveProperty('openai_apikey_responses_websockets_v2_mode')
  })

  it('submits OpenAI Responses settings for an Antigravity upstream key with a responses endpoint', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'Antigravity')
    await selectButtonByText(wrapper, 'admin.accounts.types.antigravityApikey')
    await flushPromises()
    await wrapper.get('[data-testid="protocol-endpoint-add-responses"]').trigger('click')
    await wrapper.get('[data-testid="protocol-endpoint-input-responses"]').setValue('https://relay.example/v1')
    await fillKeyBasics(wrapper, 'antigravity responses relay')

    await wrapper.get('[data-testid="create-openai-passthrough-toggle"]').trigger('click')
    const compact = wrapper.get('[data-testid="create-openai-compact"]')
    const addCompactMapping = compact.findAll('button').find((button) => button.text().includes('admin.accounts.addMapping'))
    await addCompactMapping!.trigger('click')
    const [from, to] = compact.findAll('input[type="text"]')
    await from.setValue('gpt-5.4')
    await to.setValue('gpt-5.4-compact')

    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({
      openai_passthrough: true,
      openai_apikey_responses_websockets_v2_mode: 'off'
    })
    expect(payload?.credentials?.compact_model_mapping).toEqual({ 'gpt-5.4': 'gpt-5.4-compact' })
  })

  it('clears OpenAI Responses settings when switching to another platform label', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await selectButtonByText(wrapper, 'API Key')
    await flushPromises()
    await wrapper.get('[data-testid="create-openai-passthrough-toggle"]').trigger('click')

    // Kimi 的官方地址同样带 responses / chat_completions，区块一直可见
    await selectButtonByText(wrapper, 'Kimi')
    await flushPromises()
    expect(wrapper.find('[data-testid="create-openai-passthrough"]').exists()).toBe(true)
    await fillKeyBasics(wrapper, 'kimi after openai')

    const payload = await submitPayload(wrapper)
    expect(payload?.platform).toBe('kimi')
    expect(payload?.extra ?? {}).not.toHaveProperty('openai_passthrough')
  })

  it('keeps OpenAI Responses settings for OpenAI subscriptions only, without the key hint', async () => {
    const wrapper = mountModal()
    await selectButtonByText(wrapper, 'OpenAI')
    await flushPromises()
    expect(wrapper.find('[data-testid="create-openai-passthrough"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="create-openai-compact"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="create-openai-key-protocol-hint"]').exists()).toBe(false)

    // Grok OAuth 成品号：协议地址预填了 chat_completions / responses 也不展示
    await selectButtonByText(wrapper, 'Grok')
    await flushPromises()
    expect(wrapper.find('[data-testid="create-openai-passthrough"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-openai-ws-mode"]').exists()).toBe(false)
  })

  it('never shows the key-only Anthropic settings for Anthropic subscription accounts', async () => {
    const wrapper = mountModal()
    await flushPromises()
    // 默认是 Anthropic OAuth 成品号；协议地址预填了官方 anthropic 地址也不展示
    expect(wrapper.find('[data-testid="create-anthropic-passthrough"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-anthropic-auth-scheme"]').exists()).toBe(false)
  })
})
