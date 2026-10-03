import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const {
  createAccountMock,
  importCodexSessionMock,
  createOpenAICodexPATMock,
  authIsSimpleMode,
  getProtocolDefaultsMock,
} = vi.hoisted(() => ({
  createAccountMock: vi.fn(),
  importCodexSessionMock: vi.fn(),
  createOpenAICodexPATMock: vi.fn(),
  authIsSimpleMode: { value: true },
  getProtocolDefaultsMock: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({}),
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
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false }),
      importCodexSession: importCodexSessionMock,
      createOpenAICodexPAT: createOpenAICodexPATMock,
    },
    settings: {
      getSettings: vi.fn().mockResolvedValue({}),
    },
    tlsFingerprintProfiles: {
      list: vi.fn().mockResolvedValue([]),
    },
    modelCatalog: {
      listEntries: vi.fn().mockResolvedValue([]),
    },
  },
}))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: { getProtocolDefaults: getProtocolDefaultsMock },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import CreateAccountModal from '../CreateAccountModal.vue'
import { findAccessSource } from '../accessSources'
import { resetProtocolDefaultsCacheForTest } from '../protocolEndpoints'

// 与后端 GET /admin/accounts/protocol-defaults 同形；取自开发实例的真实返回（节选）。
const PROTOCOL_DEFAULTS = {
  protocols: ['anthropic', 'chat_completions', 'responses', 'gemini'],
  defaults: {
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
  vendor_hosts: {
    'api.moonshot.cn': 'kimi',
    'api.kimi.com': 'kimi',
    'api.minimaxi.com': 'minimax',
    'api.minimax.io': 'minimax',
    'opencode.ai': 'opencode_go',
  },
}

// 第三方 key 常用官方地址（选「第三方 key」后从地址菜单里挑，厂商按地址识别）；
// 官方地址只剩国产厂商与 OpenCode，其余都是中转，手填（typed）
const KEY = {
  relayAnthropic: { protocol: 'anthropic', url: 'https://relay.example.com', typed: true },
  relayResponses: { protocol: 'responses', url: 'https://relay.example.com/v1', typed: true },
  kimi: { protocol: 'chat_completions', url: 'https://api.moonshot.cn/v1' },
  kimiCoding: { protocol: 'chat_completions', url: 'https://api.kimi.com/coding/v1' },
  minimax: { protocol: 'chat_completions', url: 'https://api.minimaxi.com/v1', mode: 'payg' },
  minimaxIntlAnthropic: { protocol: 'anthropic', url: 'https://api.minimax.io/anthropic', mode: 'payg' },
  opencodeZen: { protocol: 'chat_completions', url: 'https://opencode.ai/zen/v1' },
  opencodeGo: { protocol: 'chat_completions', url: 'https://opencode.ai/zen/go/v1' },
} as const
type KeyAddress = { protocol: string; url: string; mode?: string; typed?: boolean }

beforeEach(() => {
  resetProtocolDefaultsCacheForTest()
  getProtocolDefaultsMock.mockReset().mockResolvedValue(PROTOCOL_DEFAULTS)
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

function mountModal() {
  return mount(CreateAccountModal, {
    props: { show: true, proxies: [] },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
        ConfirmDialog: true,
        Select: true,
        Icon: true,
        PlatformIcon: true,
        ProxySelector: true,
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

// 新建渠道第一步：先选接入方式（第三方 key / 成品号）；成品号再选哪家的账号（见 accessSources.ts）
async function selectSource(wrapper: ReturnType<typeof mountModal>, sourceId: string) {
  let button = wrapper.find(`[data-testid="access-source-${sourceId}"]`)
  if (!button.exists()) {
    await wrapper.get(`[data-testid="access-kind-${findAccessSource(sourceId).kind}"]`).trigger('click')
    await flushPromises()
    button = wrapper.get(`[data-testid="access-source-${sourceId}"]`)
  }
  await button.trigger('click')
  await flushPromises()
}

// 第三方 key 不选平台 / 来源：选「第三方 key」，再从常用官方地址里挑一条或手填中转地址；不传地址就留空
async function selectKey(wrapper: ReturnType<typeof mountModal>, address?: KeyAddress) {
  await wrapper.get('[data-testid="access-kind-key"]').trigger('click')
  await flushPromises()
  if (address?.typed) await typeKeyAddress(wrapper, address)
  else if (address) await pickKeyAddress(wrapper, address)
}

async function typeKeyAddress(wrapper: ReturnType<typeof mountModal>, address: KeyAddress) {
  await switchProtocol(wrapper, address.protocol)
  await wrapper.get(`[data-testid="protocol-endpoint-input-${address.protocol}"]`).setValue(address.url)
  await flushPromises()
}

async function pickKeyAddress(wrapper: ReturnType<typeof mountModal>, address: KeyAddress) {
  const menu = wrapper.get('[data-testid="key-address-preset"]')
  const option = menu.findAll('option').find((candidate) =>
    candidate.attributes('data-protocol') === address.protocol &&
    candidate.attributes('data-url') === address.url &&
    (address.mode === undefined || candidate.attributes('data-mode') === address.mode)
  )
  expect(option).toBeDefined()
  await menu.setValue(option!.attributes('value'))
  await flushPromises()
}

// 一个 key 只承接一个协议：换协议走下拉，地址没改过就换成新协议的官方地址
async function switchProtocol(wrapper: ReturnType<typeof mountModal>, protocol: string) {
  await wrapper.get('[data-testid="protocol-endpoint-protocol"]').setValue(protocol)
}

async function openCodexImportStep() {
  const wrapper = mountModal()
  await selectSource(wrapper, 'chatgpt')
  await wrapper.get('[data-testid="channel-name"]').setValue('Codex import')
  await wrapper.get('form#create-account-form').trigger('submit.prevent')
  return wrapper
}

describe('CreateAccountModal OpenAI account creation', () => {
  beforeEach(() => {
    authIsSimpleMode.value = true
    createAccountMock.mockReset().mockResolvedValue({ id: 42, platform: 'openai', type: 'apikey' })
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
    await selectKey(wrapper, KEY.relayResponses)
    await wrapper.get('[data-testid="channel-name"]').setValue('expiry account')
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
    await selectKey(wrapper, KEY.relayResponses)
    await wrapper.get('[data-testid="channel-name"]').setValue('custom expiry account')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('test-api-key')
    await selectButtonByText(wrapper, 'payment.oneMonth')
    await wrapper.get('input[type="datetime-local"]').setValue('2030-04-15T09:20')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(createAccountMock.mock.calls[0]?.[0]?.expires_at).toBe(new Date('2030-04-15T09:20:00').getTime() / 1000)
    wrapper.unmount()
  })

  it('submits OpenCode Zen default protocol rules with adaptive endpoints', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper, KEY.opencodeZen)
    await wrapper.get('[data-testid="channel-name"]').setValue('oc')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-opencode-zen')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual({ chat_completions: PROTOCOL_DEFAULTS.defaults.opencode_go.zen.chat_completions })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('base_url')
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_base_urls')
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({
      account_mode: 'zen'
    })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('protocol_rules')
  })

  it('submits OpenCode GO endpoints with the mode taken from the address', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper, KEY.opencodeZen)
    await pickKeyAddress(wrapper, KEY.opencodeGo)
    await wrapper.get('[data-testid="channel-name"]').setValue('oc-go')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-opencode-go')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual({ chat_completions: PROTOCOL_DEFAULTS.defaults.opencode_go.go.chat_completions })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({
      account_mode: 'go'
    })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('protocol_rules')
  })

  it('submits Kimi protocol endpoints without an API protocol', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper, KEY.kimi)
    await wrapper.get('[data-testid="channel-name"]').setValue('Kimi adaptive')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-kimi')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual({ chat_completions: PROTOCOL_DEFAULTS.defaults.kimi.default.chat_completions })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({ account_mode: 'payg' })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
  })

  it('submits Kimi Coding Plan Responses endpoint', async () => {
    const wrapper = mountModal()
    // Kimi 的套餐看地址就知道（api.kimi.com 是 Coding），不用再选
    await selectKey(wrapper, KEY.kimiCoding)
    expect(wrapper.find('[data-testid="key-plan-mode"]').exists()).toBe(false)
    await wrapper.get('[data-testid="channel-name"]').setValue('Kimi coding')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-kimi-coding')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual({ chat_completions: PROTOCOL_DEFAULTS.defaults.kimi.coding.chat_completions })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({ account_mode: 'coding' })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
  })

  it('submits MiniMax protocol endpoints', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper, KEY.minimax)
    await wrapper.get('[data-testid="channel-name"]').setValue('MiniMax adaptive')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-minimax')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock.mock.calls[0]?.[0]?.protocol_endpoints).toEqual({ chat_completions: PROTOCOL_DEFAULTS.defaults.minimax.default.chat_completions })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({ account_mode: 'payg' })
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('api_protocol')
  })

  it('refuses to create a third-party key without any protocol endpoint', async () => {
    const wrapper = mountModal()
    // 自定义中转不预填地址
    await selectKey(wrapper)
    await wrapper.get('[data-testid="channel-name"]').setValue('no endpoint')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-test')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).not.toHaveBeenCalled()
  })

  it('has no API protocol selector: a Chinese provider key keeps whichever endpoint it configures', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper, KEY.kimi)
    await flushPromises()
    expect(wrapper.text()).not.toContain('admin.accounts.cnProviders.apiProtocol.title')
    await switchProtocol(wrapper, 'responses')
    await wrapper.get('[data-testid="channel-name"]').setValue('kimi without anthropic')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-kimi')

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    const payload = createAccountMock.mock.calls[0]?.[0]
    expect(payload?.protocol_endpoints).toEqual({ responses: PROTOCOL_DEFAULTS.defaults.kimi.default.responses })
    expect(payload?.credentials).not.toHaveProperty('api_protocol')
  })

  // 套餐只在地址分不出来时问：MiniMax 按量与 Coding 同一个地址，要管理员选，选了不动地址
  it('asks for the plan only when the address cannot tell it and keeps the address', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper, KEY.minimax)
    expect(wrapper.find('[data-testid="key-plan-mode"]').exists()).toBe(true)
    await wrapper.get('[data-testid="key-plan-mode-coding"]').trigger('click')
    await flushPromises()
    expect(
      (wrapper.get('[data-testid="protocol-endpoint-input-chat_completions"]').element as HTMLInputElement).value
    ).toBe('https://api.minimaxi.com/v1')
    await wrapper.get('[data-testid="channel-name"]').setValue('minimax coding')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-minimax')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(createAccountMock.mock.calls[0]?.[0]?.credentials).toMatchObject({ account_mode: 'coding' })
  })

  it('fills only the preset protocol when a Chinese provider preset is picked', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper, KEY.minimax)
    await flushPromises()
    // 国际站地址：常用地址菜单里有，官方地址表里没有
    await pickKeyAddress(wrapper, KEY.minimaxIntlAnthropic)

    await wrapper.get('[data-testid="channel-name"]').setValue('minimax preset')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-minimax')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    const payload = createAccountMock.mock.calls[0]?.[0]
    expect(payload?.credentials).toMatchObject({ account_mode: 'payg' })
    expect(payload?.credentials).not.toHaveProperty('api_protocol')
    expect(payload?.protocol_endpoints).toEqual({ anthropic: 'https://api.minimax.io/anthropic' })
  })

  it('asks for manual endpoints when official addresses fail to load', async () => {
    getProtocolDefaultsMock.mockReset().mockRejectedValue(new Error('offline'))
    const wrapper = mountModal()
    await selectKey(wrapper)

    expect(wrapper.find('[data-testid="protocol-defaults-load-failed"]').exists()).toBe(true)
    // 官方地址没拿到就没有常用地址菜单，地址只能手填
    expect(wrapper.find('[data-testid="key-address-preset"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="protocol-endpoint-input-anthropic"]').exists()).toBe(false)
  })

  it('exposes Agent Identity in the OpenAI authorization methods', async () => {
    const wrapper = mountModal()
    await selectSource(wrapper, 'chatgpt')
    await wrapper.get('[data-testid="channel-name"]').setValue('OpenAI account')
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

  it('自定义中转 key 不带 Antigravity 成品号的混合调度 / 超量键', async () => {
    const wrapper = mountModal()
    await selectSource(wrapper, 'antigravity')
    // 先在成品号（OAuth）形态下勾上两项，再切到第三方 key：残留的勾选不能写进 key 的 extra。
    const checkboxes = wrapper.findAll('form#create-account-form input[type="checkbox"]')
    for (const checkbox of checkboxes) {
      await checkbox.setValue(true)
    }
    await wrapper.get('[data-testid="allow-overages-toggle"]').trigger('click')
    await selectKey(wrapper)
    expect(wrapper.text()).not.toContain('admin.accounts.mixedScheduling')
    expect(wrapper.text()).not.toContain('admin.accounts.allowOverages')
    await wrapper.get('[data-testid="channel-name"]').setValue('antigravity relay')
    await switchProtocol(wrapper, 'anthropic')
    await wrapper.get('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example/antigravity')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-upstream')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    const extra = createAccountMock.mock.calls[0]?.[0]?.extra ?? {}
    expect(extra).not.toHaveProperty('mixed_scheduling')
    expect(extra).not.toHaveProperty('allow_overages')
  })

  it('自定义中转 key 创建不带平台', async () => {
    // 平台由后端按地址推导。
    const wrapper = mountModal()
    await selectKey(wrapper)
    await wrapper.get('[data-testid="channel-name"]').setValue('antigravity relay')
    await switchProtocol(wrapper, 'anthropic')
    await wrapper.get('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example/antigravity')
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-upstream')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createAccountMock).toHaveBeenCalledTimes(1)
    const payload = createAccountMock.mock.calls[0]?.[0]
    expect(payload).not.toHaveProperty('platform')
    expect(payload?.type).toBe('apikey')
    expect(payload?.protocol_endpoints).toEqual({ anthropic: 'https://relay.example/antigravity' })
    expect(payload?.credentials).not.toHaveProperty('base_url')
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
  })

  async function fillKeyBasics(wrapper: ReturnType<typeof mountModal>, name: string) {
    await wrapper.get('[data-testid="channel-name"]').setValue(name)
    await wrapper.get('form#create-account-form input[type="password"]').setValue('sk-test')
  }

  async function submitPayload(wrapper: ReturnType<typeof mountModal>) {
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(createAccountMock).toHaveBeenCalledTimes(1)
    return createAccountMock.mock.calls[0]?.[0]
  }

  it('offers header overrides for a custom relay key and submits them', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper)
    await switchProtocol(wrapper, 'anthropic')
    await wrapper.get('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example/antigravity')
    await fillKeyBasics(wrapper, 'antigravity relay')

    const section = wrapper.get('[data-testid="create-header-override"]')
    await selectButtonByText(wrapper, 'admin.accounts.headerOverride.addRow')
    const [name, value] = section.findAll('input[type="text"]')
    await name.setValue('X-Relay-Tenant')
    await value.setValue('team-a')

    const payload = await submitPayload(wrapper)
    expect(payload?.type).toBe('apikey')
    expect(payload?.credentials).toMatchObject({
      header_overrides: { 'x-relay-tenant': 'team-a' }
    })
    expect(payload?.credentials).not.toHaveProperty('header_override_enabled')
  })

  it('keeps header overrides limited to Grok OAuth among subscription accounts', async () => {
    const wrapper = mountModal()
    await selectSource(wrapper, 'claude')
    expect(wrapper.find('[data-testid="create-header-override"]').exists()).toBe(false)
    await selectSource(wrapper, 'grok')
    expect(wrapper.find('[data-testid="create-header-override"]').exists()).toBe(true)
  })

  it('shows Anthropic protocol settings for a Kimi key with an anthropic endpoint and submits them', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper, KEY.kimi)
    await switchProtocol(wrapper, 'anthropic')
    await fillKeyBasics(wrapper, 'kimi relay')

    await wrapper.get('[data-testid="create-anthropic-auth-scheme"]').setValue('authorization_bearer')
    await wrapper.get('[data-testid="create-bedrock-cc-compat-toggle"]').trigger('click')

    const payload = await submitPayload(wrapper)
    expect(payload).not.toHaveProperty('platform')
    expect(payload?.protocol_endpoints).toEqual({ anthropic: PROTOCOL_DEFAULTS.defaults.kimi.default.anthropic })
    expect(payload?.extra).toMatchObject({
      anthropic_apikey_auth_scheme: 'authorization_bearer',
      bedrock_cc_compat: true
    })
  })

  it('hides Anthropic protocol settings once a relay key drops its anthropic endpoint', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper, KEY.relayAnthropic)
    expect(wrapper.find('[data-testid="create-anthropic-auth-scheme"]').exists()).toBe(true)
    await wrapper.get('[data-testid="create-anthropic-auth-scheme"]').setValue('authorization_bearer')

    await switchProtocol(wrapper, 'chat_completions')
    expect(wrapper.find('[data-testid="create-anthropic-auth-scheme"]').exists()).toBe(false)

    await wrapper.get('[data-testid="protocol-endpoint-input-chat_completions"]').setValue('https://relay.example.com/v1')
    await fillKeyBasics(wrapper, 'relay without anthropic endpoint')
    const payload = await submitPayload(wrapper)
    expect(payload?.extra ?? {}).not.toHaveProperty('anthropic_apikey_auth_scheme')
  })

  it('submits Anthropic protocol settings for a custom relay key', async () => {
    const wrapper = mountModal()
    await selectKey(wrapper)
    await switchProtocol(wrapper, 'anthropic')
    await wrapper.get('[data-testid="protocol-endpoint-input-anthropic"]').setValue('https://relay.example/antigravity')
    await fillKeyBasics(wrapper, 'antigravity relay')

    await wrapper.get('[data-testid="create-anthropic-auth-scheme"]').setValue('authorization_bearer')

    const payload = await submitPayload(wrapper)
    expect(payload?.extra).toMatchObject({ anthropic_apikey_auth_scheme: 'authorization_bearer' })
  })

  it('never shows the key-only Anthropic settings for Anthropic subscription accounts', async () => {
    const wrapper = mountModal()
    await selectSource(wrapper, 'claude')
    // Anthropic OAuth 成品号；协议地址预填了官方 anthropic 地址也不展示
    expect(wrapper.find('[data-testid="create-anthropic-passthrough"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-anthropic-auth-scheme"]').exists()).toBe(false)
  })
})
