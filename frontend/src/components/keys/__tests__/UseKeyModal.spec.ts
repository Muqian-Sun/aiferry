import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

const { copyToClipboardMock, saveAsMock } = vi.hoisted(() => ({
  copyToClipboardMock: vi.fn().mockResolvedValue(true),
  saveAsMock: vi.fn()
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: copyToClipboardMock
  })
}))

vi.mock('file-saver', () => ({
  saveAs: saveAsMock
}))

import UseKeyModal from '../UseKeyModal.vue'

function readBlobAsText(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener('load', () => resolve(String(reader.result || '')))
    reader.addEventListener('error', () => reject(reader.error))
    reader.readAsText(blob)
  })
}

const stubs = {
  BaseDialog: {
    template: '<div><slot /><slot name="footer" /></div>'
  },
  Icon: {
    template: '<span />'
  }
}

// 密钥没有分组平台（PR-7a）：客户端标签页固定六个，测试通过点标签页切换。
function mountModal(apiKey: string, show = true) {
  return mount(UseKeyModal, {
    props: { show, apiKey, baseUrl: 'https://example.com/v1' },
    global: { stubs }
  })
}

async function clickClientTab(wrapper: ReturnType<typeof mountModal>, tabKey: string) {
  const tab = wrapper.findAll('button').find((button) =>
    button.text().includes(`keys.useKeyModal.cliTabs.${tabKey}`)
  )
  expect(tab).toBeDefined()
  await tab!.trigger('click')
  await nextTick()
}

// 只取各客户端的配置文件；底部「查询可用模型」的 curl 示例不是配置
const codeBlocks = (wrapper: ReturnType<typeof mountModal>) =>
  wrapper.findAll('pre code').filter((code) => !code.element.closest('[data-testid="use-key-models-api"]')).map((code) => code.text())

describe('UseKeyModal', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    saveAsMock.mockClear()
  })

  // 裸 <template> 在浏览器里是不渲染子节点的原生元素，jsdom 却能查到它的子节点——用例直接盯它不存在。
  it('renders the body without an inert template element', () => {
    const wrapper = mountModal('sk-anthropic-test')
    expect(wrapper.find('template').exists()).toBe(false)
    expect(wrapper.findAll('button').some((button) => button.text().includes('keys.useKeyModal.cliTabs.codexCli'))).toBe(true)
  })

  it('omits the attribution override from every standard Claude Code setup form', async () => {
    const wrapper = mountModal('sk-anthropic-test')

    for (const [shell, trafficSetting] of [
      ['macOS / Linux', 'export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1'],
      ['Windows CMD', 'set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1'],
      ['PowerShell', '$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1']
    ]) {
      if (shell !== 'macOS / Linux') {
        const shellTab = wrapper.findAll('button').find(
          (button) => button.text().trim() === shell
        )
        expect(shellTab).toBeDefined()
        await shellTab!.trigger('click')
        await nextTick()
      }

      const codeBlocks = wrapper.findAll('pre code').map((code) => code.text())
      const allCode = codeBlocks.join('\n')
      const settings = JSON.parse(codeBlocks.find((content) => content.includes('"$schema"'))!)

      expect(allCode).not.toContain('CLAUDE_CODE_ATTRIBUTION_HEADER')
      expect(allCode).toContain(trafficSetting)
      expect(settings.env.CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC).toBe('1')
      expect(settings.env).not.toHaveProperty('CLAUDE_CODE_ATTRIBUTION_HEADER')
    }
  })

  it('renders Grok Build and OpenCode setup on the Grok tab', async () => {
    const wrapper = mountModal('sk-grok-test')
    await clickClientTab(wrapper, 'grokCli')

    const allCode = codeBlocks(wrapper).join('\n')
    expect(allCode).toContain('GROK_MODELS_BASE_URL')
    expect(allCode).toContain('XAI_API_KEY')
    expect(allCode).toContain('[model."grok-4.5"]')
    expect(allCode).toContain('[model."grok-build-0.1"]')
    expect(allCode).toContain('[model."grok-4.20-multi-agent-0309"]')
    expect(allCode).toContain('[model."grok-4.3"]')
    expect(allCode).toContain('default = "grok-4.5"')
    expect(allCode).toContain('models_base_url = "https://example.com/v1"')
    expect(allCode).toContain('models_list_url = "https://example.com/v1/models"')
    expect(allCode).toContain('xai_api_base_url = "https://example.com/v1"')
    expect(allCode).toContain('cli_chat_proxy_base_url = "https://example.com/v1"')
    expect(allCode).toContain('preferred_method = "api_key"')
    expect(allCode).toContain('image_description = "grok-4.5"')
    expect(allCode).toContain('auto_compact_threshold_percent = 80')
    expect(allCode).toContain('image_gen = true')
    expect(allCode).toContain('video_gen = true')
    expect(allCode).toContain('image_gen_model_override = "grok-imagine-image-quality"')
    expect(allCode).toContain('image_edit_model_override = "grok-imagine-edit"')
    expect(allCode).toContain('env_key = "XAI_API_KEY"')
    expect(allCode).toContain('Keep api_backend = "responses" on every model entry.')
    expect(allCode).toContain('grok-imagine-image')
    expect(allCode).toContain('grok-imagine-edit')
    expect(allCode).toMatch(/\[model\."grok-4\.5"\][\s\S]*?context_window = 500000/)
    expect(allCode).toMatch(/\[model\."grok-build-0\.1"\][\s\S]*?context_window = 256000/)
    // Prefer env_key; hardcode api_key only as commented alternative
    expect(allCode).not.toMatch(/^api_key = "sk-grok-test"$/m)

    const modelBlocks = allCode
      .split(/(?=^\[model\.)/m)
      .filter((block) => block.startsWith('[model."'))
    expect(modelBlocks.length).toBeGreaterThanOrEqual(4)
    for (const block of modelBlocks) {
      if (block.includes('# [model.')) continue
      expect(block).toContain('api_backend = "responses"')
    }

    const windowsTab = wrapper.findAll('button').find(
      (button) => button.text().trim() === 'Windows'
    )
    expect(windowsTab).toBeDefined()
    await windowsTab!.trigger('click')
    await nextTick()
    expect(wrapper.text().toLowerCase()).toContain('%userprofile%\\.grok\\config.toml')

    await clickClientTab(wrapper, 'opencode')

    // OpenCode 标签页给四份 provider 配置（Claude / OpenAI / Gemini / Grok）
    const opencodeConfigs = codeBlocks(wrapper).map((content) => JSON.parse(content))
    expect(opencodeConfigs.map((config) => Object.keys(config.provider)[0])).toEqual(['anthropic', 'openai', 'gemini', 'grok'])
    const parsed = opencodeConfigs[3]
    expect(parsed.provider.grok.npm).toBe('@ai-sdk/openai-compatible')
    expect(parsed.provider.grok.name).toBe('Grok via AiFerry')
    expect(parsed.provider.grok.options).toEqual({
      baseURL: 'https://example.com/v1',
      apiKey: 'sk-grok-test'
    })
    expect(parsed.provider.grok.models['grok-4.5']).toBeDefined()
    expect(parsed.provider.grok.models['grok-4.5'].limit.context).toBe(500000)
    expect(parsed.provider.grok.models['grok-build-0.1']).toBeDefined()
    expect(parsed.provider.grok.models['grok-4.20-multi-agent-0309']).toBeDefined()
    expect(parsed.provider.grok.models['grok-composer-2.5-fast']).toBeDefined()
    expect(parsed.provider.grok.models['gpt-5.6']).toBeUndefined()
  })

  it('keeps legacy OpenAI Codex config as the default', async () => {
    const wrapper = mountModal('sk-test')
    await clickClientTab(wrapper, 'codexCli')

    const blocks = codeBlocks(wrapper)
    const configToml = blocks.find((content) => content.includes('model_provider = "OpenAI"'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('model = "gpt-5.5"')
    expect(configToml).toContain('review_model = "gpt-5.5"')
    expect(configToml).not.toContain('model = "gpt-5.4"')
    expect(configToml).not.toContain('model_context_window')
    expect(configToml).not.toContain('model_auto_compact_token_limit')
    expect(configToml).toContain('requires_openai_auth = true')
    expect(configToml).not.toContain('experimental_bearer_token')
    expect(configToml).not.toContain('x-openai-actor-authorization')
    expect(configToml).not.toContain('env_key')
    expect(configToml).not.toContain('image_generation')
    expect(configToml).not.toContain('supports_websockets')
    expect(configToml).not.toContain('responses_websockets_v2')
    expect(configToml).toContain('[features]\ngoals = true')
    expect(configToml).not.toContain('model_reasoning_effort = "xhigh"')
    expect(blocks).toContain('{\n  "OPENAI_API_KEY": "sk-test"\n}')
    expect(wrapper.text()).toContain('auth.json')
    expect(wrapper.find('[data-testid="codex-api-key-restart-notice"]').exists()).toBe(false)
  })

  it('renders API Key Mode authorization in OpenAI Codex config', async () => {
    const wrapper = mountModal('sk-test')
    await clickClientTab(wrapper, 'codexCli')

    const apiKeyMode = wrapper.get('[data-testid="codex-auth-mode-api-key"]')
    await apiKeyMode.trigger('click')
    await nextTick()

    const blocks = codeBlocks(wrapper)
    const configToml = blocks.find((content) => content.includes('model_provider = "OpenAI"'))

    expect(apiKeyMode.attributes('aria-checked')).toBe('true')
    expect(configToml).toBeDefined()
    expect(configToml).toContain('requires_openai_auth = false')
    expect(configToml).toContain('experimental_bearer_token = "sk-test"')
    expect(configToml).toContain('http_headers = { "x-openai-actor-authorization" = "local-image-extension" }')
    expect(configToml).not.toContain('env_key')
    expect(configToml).not.toContain('image_generation')
    expect(blocks).not.toContain('{\n  "OPENAI_API_KEY": "sk-test"\n}')
    expect(wrapper.text()).not.toContain('auth.json')

    const restartNotice = wrapper.get('[data-testid="codex-api-key-restart-notice"]')
    expect(restartNotice.text()).toContain(
      'keys.useKeyModal.openai.authModeApiKeyRestartNotice'
    )

    await wrapper.get('[data-testid="codex-auth-mode-legacy"]').trigger('click')
    await nextTick()

    expect(wrapper.find('[data-testid="codex-api-key-restart-notice"]').exists()).toBe(false)
    expect(wrapper.findAll('pre code').map((code) => code.text()).join('\n')).not.toContain(
      'x-openai-actor-authorization'
    )
  })

  it('keeps legacy OpenAI Codex WebSocket config as the default', async () => {
    const wrapper = mountModal('sk-test')

    await clickClientTab(wrapper, 'codexCliWs')

    const blocks = codeBlocks(wrapper)
    const configToml = blocks.find((content) => content.includes('supports_websockets = true'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('model = "gpt-5.5"')
    expect(configToml).toContain('review_model = "gpt-5.5"')
    expect(configToml).not.toContain('model = "gpt-5.4"')
    expect(configToml).not.toContain('model_context_window')
    expect(configToml).not.toContain('model_auto_compact_token_limit')
    expect(configToml).toContain('requires_openai_auth = true')
    expect(configToml).not.toContain('experimental_bearer_token')
    expect(configToml).not.toContain('x-openai-actor-authorization')
    expect(configToml).not.toContain('env_key')
    expect(configToml).not.toContain('image_generation')
    expect(configToml).toContain('supports_websockets = true')
    expect(configToml).toContain('[features]\nresponses_websockets_v2 = true\ngoals = true')
    expect(blocks).toContain('{\n  "OPENAI_API_KEY": "sk-test"\n}')
    expect(wrapper.text()).toContain('auth.json')
  })

  it('preserves API Key Mode when switching to OpenAI Codex WebSocket config', async () => {
    const wrapper = mountModal('sk-test')
    await clickClientTab(wrapper, 'codexCli')

    const apiKeyMode = wrapper.get('[data-testid="codex-auth-mode-api-key"]')
    await apiKeyMode.trigger('click')

    await clickClientTab(wrapper, 'codexCliWs')

    const blocks = codeBlocks(wrapper)
    const configToml = blocks.find((content) => content.includes('supports_websockets = true'))

    expect(wrapper.get('[data-testid="codex-auth-mode-api-key"]').attributes('aria-checked')).toBe('true')
    expect(configToml).toBeDefined()
    expect(configToml).toContain('requires_openai_auth = false')
    expect(configToml).toContain('experimental_bearer_token = "sk-test"')
    expect(configToml).toContain('http_headers = { "x-openai-actor-authorization" = "local-image-extension" }')
    expect(configToml).not.toContain('env_key')
    expect(configToml).not.toContain('image_generation')
    expect(configToml).toContain('supports_websockets = true')
    expect(configToml).toContain('[features]\nresponses_websockets_v2 = true\ngoals = true')
    expect(blocks).not.toContain('{\n  "OPENAI_API_KEY": "sk-test"\n}')
    expect(wrapper.text()).not.toContain('auth.json')
  })

  it('resets the client tab and Codex authentication mode when the modal reopens', async () => {
    const wrapper = mountModal('sk-test')
    await clickClientTab(wrapper, 'codexCli')

    await wrapper.get('[data-testid="codex-auth-mode-api-key"]').trigger('click')
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await nextTick()

    // 重开回到 Claude Code 标签页
    expect(wrapper.find('[data-testid="codex-auth-mode-legacy"]').exists()).toBe(false)
    expect(codeBlocks(wrapper).join('\n')).toContain('ANTHROPIC_BASE_URL')

    await clickClientTab(wrapper, 'codexCli')
    expect(wrapper.get('[data-testid="codex-auth-mode-legacy"]').attributes('aria-checked')).toBe('true')
    expect(codeBlocks(wrapper).join('\n')).toContain('requires_openai_auth = true')
    expect(codeBlocks(wrapper).join('\n')).not.toContain('x-openai-actor-authorization')
  })

  it('renders GPT-5.4 mini entry in OpenCode config', async () => {
    const wrapper = mountModal('sk-test')
    await clickClientTab(wrapper, 'opencode')

    const openaiConfig = codeBlocks(wrapper).find((content) => content.includes('"openai": {'))
    expect(openaiConfig).toBeDefined()
    expect(openaiConfig).toContain('"name": "GPT-5.4 Mini"')
    expect(openaiConfig).not.toContain('"name": "GPT-5.4 Nano"')
  })

  it('renders GPT-5.6 and GPT-6 Astra capabilities in OpenCode config', async () => {
    const wrapper = mountModal('sk-test')
    await clickClientTab(wrapper, 'opencode')

    const openaiConfig = codeBlocks(wrapper).find((content) => content.includes('"openai": {'))
    expect(openaiConfig).toBeDefined()
    const parsed = JSON.parse(openaiConfig!)
    const models = parsed.provider.openai.models
    for (const model of ['gpt-5.6', 'gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna']) {
      expect(models[model]).toBeDefined()
      expect(models[model].variants).toHaveProperty('max')
      expect(models[model].variants).toHaveProperty('xhigh')
    }
    expect(models['gpt-5.6'].name).toBe('GPT-5.6 (Sol)')
    expect(models['gpt-6']).toEqual({
      name: 'GPT-6 (Astra)',
      limit: { context: 1050000, output: 128000 },
      options: { store: false },
      variants: { low: {}, medium: {}, high: {}, xhigh: {}, max: {} }
    })
    expect(models['gpt-6-astra']).toEqual({
      name: 'GPT-6 Astra',
      limit: { context: 1050000, output: 128000 },
      options: { store: false },
      variants: { low: {}, medium: {}, high: {}, xhigh: {}, max: {} }
    })
  })

  // Scenario: any key can fetch the catalog-driven Codex manifest and reference it from config.toml.
  it('offers a downloadable Codex catalog on the Codex tab', async () => {
    const manifest = {
      models: [
        {
          slug: 'claude-opus-4-8',
          default_reasoning_level: 'medium',
          supported_reasoning_levels: [{ effort: 'max', description: 'Maximum reasoning depth' }],
          input_modalities: ['text'],
          model_messages: { instructions_template: 'Use the routed model.' }
        },
        {
          slug: 'grok-4.6',
          default_reasoning_level: 'high',
          supported_reasoning_levels: [{ effort: 'xhigh', description: 'Extra-high reasoning depth' }],
          input_modalities: ['text'],
          model_messages: { instructions_template: 'Use the routed model.' }
        }
      ]
    }
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => manifest
    })
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mountModal('sk-composite-test')
    await clickClientTab(wrapper, 'codexCli')

    const unixConfig = codeBlocks(wrapper).find((content) => content.includes('[model_providers.OpenAI]'))
    expect(unixConfig).toContain('model_catalog_json = "~/.codex/codex-models.json"')
    expect(unixConfig).toContain('base_url = "https://example.com/v1"')
    expect(unixConfig).toContain('wire_api = "responses"')

    await wrapper.get('[data-testid="codex-model-catalog-fetch"]').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(
      'https://example.com/v1/models?client_version=0.147.0',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer sk-composite-test' })
      })
    )
    expect(wrapper.get('[data-testid="codex-model-catalog"]').text())
      .toContain('keys.useKeyModal.codexModelCatalog.download')

    const loadedUnixConfig = codeBlocks(wrapper).find((content) => content.includes('[model_providers.OpenAI]'))
    expect(loadedUnixConfig).toContain('model = "claude-opus-4-8"')
    expect(loadedUnixConfig).toContain('review_model = "claude-opus-4-8"')
    expect(loadedUnixConfig).not.toContain('model = "gpt-5.5"')

    const downloadButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.codexModelCatalog.download')
    )
    expect(downloadButton).toBeDefined()
    await downloadButton!.trigger('click')
    expect(saveAsMock).toHaveBeenCalledWith(expect.any(Blob), 'codex-models.json')
    const downloadedBlob = saveAsMock.mock.calls[0]?.[0] as Blob
    expect(JSON.parse(await readBlobAsText(downloadedBlob))).toEqual(manifest)

    const windowsTab = wrapper.findAll('button').find((button) => button.text().trim() === 'Windows')
    expect(windowsTab).toBeDefined()
    await windowsTab!.trigger('click')
    await nextTick()

    const windowsConfig = codeBlocks(wrapper).find((content) => content.includes('[model_providers.OpenAI]'))
    expect(windowsConfig).toContain(
      'model_catalog_json = "%userprofile%\\\\.codex\\\\codex-models.json"'
    )
  })

  // Scenario: the preferred default model remains selected when the downloaded catalog contains it.
  it('keeps the preferred Codex default when it exists in the catalog', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        models: [
          { slug: 'claude-opus-4-8' },
          { slug: 'gpt-5.5' }
        ]
      })
    }))

    const wrapper = mountModal('sk-composite-test')
    await clickClientTab(wrapper, 'codexCli')
    await wrapper.get('[data-testid="codex-model-catalog-fetch"]').trigger('click')
    await flushPromises()

    const config = codeBlocks(wrapper).find((content) => content.includes('[model_providers.OpenAI]'))
    expect(config).toContain('model = "gpt-5.5"')
    expect(config).toContain('review_model = "gpt-5.5"')
  })

  it('derives OpenAI Codex reasoning effort from the selected catalog descriptor', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        models: [
          {
            slug: 'glm-5.3',
            default_reasoning_level: 'none',
            supported_reasoning_levels: [{ effort: 'none' }]
          }
        ]
      })
    }))

    const wrapper = mountModal('sk-openai-test')
    await clickClientTab(wrapper, 'codexCli')

    await wrapper.get('[data-testid="codex-model-catalog-fetch"]').trigger('click')
    await flushPromises()

    const configToml = codeBlocks(wrapper).find((content) => content.includes('model_provider = "OpenAI"'))
    expect(configToml).toContain('model = "glm-5.3"')
    expect(configToml).not.toContain('model_reasoning_effort')
  })
})
