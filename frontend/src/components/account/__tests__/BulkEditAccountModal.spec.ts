import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import BulkEditAccountModal from '../BulkEditAccountModal.vue'
import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'
import { adminAPI } from '@/api/admin'

const { showError, showSuccess, translate } = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  translate: vi.fn((key: string) => key)
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showInfo: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      bulkUpdate: vi.fn(),
      checkMixedChannelRisk: vi.fn()
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
      t: translate
    })
  }
})

// 选中第三方 key 的协议地址（AccountsView 按所选 key 逐个传入）
const OPENAI_KEY_ENDPOINTS = { chat_completions: 'https://api.openai.com', responses: 'https://api.openai.com' }
const ANTHROPIC_KEY_ENDPOINTS = { anthropic: 'https://relay.example.com' }

function mountModal(extraProps: Record<string, unknown> = {}) {
  // 与真实调用方一致：选中里有 key 才有地址；没显式给时按只配了 anthropic 地址的 key 处理
  const selectedTypes = (extraProps.selectedTypes ?? ['apikey']) as string[]
  return mount(BulkEditAccountModal, {
    props: {
      show: true,
      accountIds: [1, 2],
      selectedPlatforms: ['antigravity'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: selectedTypes.includes('apikey') ? [ANTHROPIC_KEY_ENDPOINTS, ANTHROPIC_KEY_ENDPOINTS] : [],
      proxies: [],
      groups: [],
      ...extraProps
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true,
        Select: {
          props: ['modelValue', 'options'],
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
        },
        ProxySelector: true,
        GroupSelector: true,
        Icon: true
      }
    }
  })
}

describe('BulkEditAccountModal', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.accounts.bulkUpdate).mockReset()
    vi.mocked(adminAPI.accounts.checkMixedChannelRisk).mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    translate.mockClear()

    vi.mocked(adminAPI.accounts.bulkUpdate).mockResolvedValue({
      success: 2,
      failed: 0,
      results: []
    } as any)
    vi.mocked(adminAPI.accounts.checkMixedChannelRisk).mockResolvedValue({
      has_risk: false
    } as any)
  })

  it('批量修改倍率时提示自动同步账号需要先关闭同步', async () => {
    const wrapper = mountModal()

    expect(wrapper.find('[data-testid="bulk-rate-sync-warning"]').exists()).toBe(false)
    await wrapper.get('#bulk-edit-rate-multiplier-enabled').setValue(true)

    expect(wrapper.get('[data-testid="bulk-rate-sync-warning"]').text()).toContain(
      'admin.accounts.bulkEdit.rateSyncWarning'
    )
  })

  it('后端拒绝修改同步账号倍率时展示专用错误', async () => {
    vi.mocked(adminAPI.accounts.bulkUpdate).mockRejectedValueOnce({
      status: 409,
      reason: 'UPSTREAM_BILLING_RATE_SYNC_BULK_CONFLICT',
      metadata: { count: '2' },
      message: 'conflict'
    })
    const wrapper = mountModal()

    await wrapper.get('#bulk-edit-rate-multiplier-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.accounts.bulkEdit.rateSyncConflict')
  })

  it('antigravity 白名单包含 Gemini 图片模型且过滤掉普通 GPT 模型', async () => {
    const wrapper = mountModal()
    const selector = wrapper.findComponent(ModelWhitelistSelector)
    expect(selector.exists()).toBe(true)

    await selector.find('div.cursor-pointer').trigger('click')

    expect(wrapper.text()).toContain('gemini-3.1-flash-image')
    expect(wrapper.text()).toContain('gemini-2.5-flash-image')
    expect(wrapper.text()).not.toContain('gpt-5.3-codex')
  })

  it('antigravity 映射预设包含图片映射并过滤 OpenAI 预设', async () => {
    const wrapper = mountModal()

    const mappingTab = wrapper.findAll('button').find((btn) => btn.text().includes('admin.accounts.modelMapping'))
    expect(mappingTab).toBeTruthy()
    await mappingTab!.trigger('click')

    expect(wrapper.text()).toContain('3.1-Flash-Image透传')
    expect(wrapper.text()).toContain('3-Pro-Image→3.1')
    expect(wrapper.text()).not.toContain('GPT-5.3 Codex Spark')
  })

  it('仅勾选模型限制且白名单留空时，应提交空 model_mapping 以支持所有模型', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['anthropic'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-model-restriction-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      credentials: {
        model_mapping: {}
      }
    })
  })

  it('不再提供批量修改 Base URL 的入口', () => {
    // 成品号只走官方地址，第三方 key 的地址只在协议映射里（批量更新不改映射）
    const wrapper = mountModal({ selectedPlatforms: ['grok'], selectedTypes: ['oauth'] })

    expect(wrapper.find('#bulk-edit-base-url').exists()).toBe(false)
    expect(wrapper.find('[data-testid="grok-base-url-preset"]').exists()).toBe(false)
  })

  it('OpenAI 账号批量编辑可开启自动透传', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-passthrough-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: true
      }
    })
  })

  it('OpenAI OAuth 批量编辑可开启 namespace 摊平兼容开关', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-flatten-namespaces-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-flatten-namespaces-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_responses_flatten_namespaces: true
      }
    })
  })

  it('namespace 摊平开关不对 setup-token 等非 OAuth 选择展示', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth', 'setup-token']
    })

    expect(wrapper.find('#bulk-edit-openai-flatten-namespaces-enabled').exists()).toBe(false)
  })

  it('OpenAI OAuth 批量编辑应提交 OAuth 专属 WS mode 字段（含 http_bridge）', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-ws-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-ws-mode-select"]').setValue('http_bridge')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_oauth_responses_websockets_v2_mode: 'http_bridge',
        openai_oauth_responses_websockets_v2_enabled: true
      }
    })
  })

  it('OpenAI API Key 批量编辑不显示 WS mode 入口', () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    expect(wrapper.find('#bulk-edit-openai-ws-mode-enabled').exists()).toBe(false)
  })

  it('OpenAI OAuth 批量编辑应提交 codex_cli_only 字段', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-codex-cli-only-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-cli-only-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        codex_cli_only: true
      }
    })
  })

  it('OpenAI OAuth 批量编辑应提交 codex_cli_only_allow_app_server 字段（需同时开启父开关）', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    // 子开关从属于 codex_cli_only：必须同时批量开启父开关才写入
    await wrapper.get('#bulk-edit-openai-codex-cli-only-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-cli-only-toggle').trigger('click')
    await wrapper.get('#bulk-edit-openai-codex-app-server-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-app-server-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        codex_cli_only: true,
        codex_cli_only_allow_app_server: true
      }
    })
  })

  it('未同时开启父开关时不应写入 codex_cli_only_allow_app_server', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    // 仅开启子开关、不批量设置父开关 codex_cli_only：不应写入孤立字段，也不应调用接口
    await wrapper.get('#bulk-edit-openai-codex-app-server-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-app-server-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).not.toHaveBeenCalled()
  })

  it('OpenAI API Key 批量编辑应提交 API Key 专属 WS mode 字段', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS, OPENAI_KEY_ENDPOINTS]
    })

    await wrapper.get('#bulk-edit-openai-apikey-ws-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-apikey-ws-mode-select"]').setValue('ctx_pool')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_apikey_responses_websockets_v2_mode: 'ctx_pool',
        openai_apikey_responses_websockets_v2_enabled: true
      }
    })
  })

  it('端点能力仅对全部配了 OpenAI 协议地址的 key 展示，Responses 路由设置已移除', () => {
    const apiKey = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS, OPENAI_KEY_ENDPOINTS]
    })
    expect(apiKey.find('#bulk-edit-openai-endpoint-capabilities-enabled').exists()).toBe(true)
    // 转发协议由协议地址决定，批量编辑不再提供 Responses 路由覆盖。
    expect(apiKey.find('#bulk-edit-openai-responses-mode-enabled').exists()).toBe(false)
    expect(apiKey.text()).not.toContain('admin.accounts.openai.responsesMode')
    apiKey.unmount()

    const oauth = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })
    expect(oauth.find('#bulk-edit-openai-endpoint-capabilities-enabled').exists()).toBe(false)
  })

  it('端点能力默认值提交 null，表示恢复两个默认端点', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS, OPENAI_KEY_ENDPOINTS]
    })

    await wrapper.get('#bulk-edit-openai-endpoint-capabilities-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      credentials: { openai_capabilities: null }
    })
  })

  it('仅启用 Embeddings 时只提交端点能力，不再附带 Responses 路由字段', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS, OPENAI_KEY_ENDPOINTS]
    })

    await wrapper.get('#bulk-edit-openai-endpoint-capabilities-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-endpoint-capability-chat_completions"]').setValue(false)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      credentials: { openai_capabilities: ['embeddings'] }
    })
  })

  it('目标变化后不提交已经隐藏的 OpenAI 设置', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS, OPENAI_KEY_ENDPOINTS]
    })

    await wrapper.get('#bulk-edit-openai-endpoint-capabilities-enabled').setValue(true)
    // 目标换成只配 Anthropic 地址的 key：OpenAI 协议设置随之隐藏，不得提交。
    await wrapper.setProps({ selectedPlatforms: ['anthropic'], selectedTypes: ['apikey'], selectedKeyEndpoints: [ANTHROPIC_KEY_ENDPOINTS, ANTHROPIC_KEY_ENDPOINTS] })
    await wrapper.get('#bulk-edit-status-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      status: 'active'
    })
  })

  it('至少保留一个端点能力', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS, OPENAI_KEY_ENDPOINTS]
    })
    await wrapper.get('#bulk-edit-openai-endpoint-capabilities-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-endpoint-capability-chat_completions"]').setValue(false)
    await wrapper.get('[data-testid="bulk-edit-openai-endpoint-capability-embeddings"]').setValue(false)

    expect((wrapper.get('[data-testid="bulk-edit-openai-endpoint-capability-embeddings"]').element as HTMLInputElement).checked)
      .toBe(true)
  })

  it('关闭弹窗后重置新增设置的启用状态和值', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS, OPENAI_KEY_ENDPOINTS]
    })
    await wrapper.get('#bulk-edit-openai-endpoint-capabilities-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-endpoint-capability-chat_completions"]').setValue(false)

    await wrapper.setProps({ show: false })
    await nextTick()

    expect((wrapper.get('#bulk-edit-openai-endpoint-capabilities-enabled').element as HTMLInputElement).checked).toBe(false)
    expect((wrapper.get('[data-testid="bulk-edit-openai-endpoint-capability-chat_completions"]').element as HTMLInputElement).checked).toBe(true)
  })

  it('OpenAI API Key 批量编辑可统一开启上游倍率自动探测', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-upstream-billing-auto-probe-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      upstream_billing_probe_enabled: true
    })
  })

  it('非 OpenAI 平台的 API Key 批量编辑同样可开启上游倍率自动探测', async () => {
    // 探测已放宽到全部 API-key 平台，混合平台选择只要求类型全为 apikey。
    const wrapper = mountModal({
      selectedPlatforms: ['grok', 'anthropic'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-upstream-billing-auto-probe-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      upstream_billing_probe_enabled: true
    })
  })

  it('OpenAI API Key 批量编辑可统一关闭上游倍率自动探测', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-upstream-billing-auto-probe-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-upstream-billing-auto-probe-select"]').setValue('disabled')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      upstream_billing_probe_enabled: false
    })
  })

  it('非 OpenAI API Key 目标不显示上游倍率自动探测批量开关', () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    expect(wrapper.find('#bulk-edit-upstream-billing-auto-probe-enabled').exists()).toBe(false)
  })

  it('筛选结果批量编辑可统一开启上游倍率自动探测', async () => {
    const wrapper = mountModal({
      accountIds: [],
      selectedPlatforms: [],
      selectedTypes: [],
      target: {
        mode: 'filtered',
        filters: { platform: 'openai', type: 'apikey', status: 'active' },
        previewCount: 20,
        selectedPlatforms: ['openai'],
        selectedTypes: ['apikey']
      }
    })

    await wrapper.get('#bulk-edit-upstream-billing-auto-probe-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith({
      filters: { platform: 'openai', type: 'apikey', status: 'active' },
      upstream_billing_probe_enabled: true
    })
  })

  it('筛选 OpenAI 账号批量编辑应提交 Compact 模式和专属模型映射', async () => {
    const wrapper = mountModal({
      accountIds: [],
      selectedPlatforms: [],
      selectedTypes: [],
      target: {
        mode: 'filtered',
        filters: { platform: 'openai' },
        previewCount: 12,
        selectedPlatforms: ['openai'],
        selectedTypes: ['oauth', 'apikey'],
        selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS]
      }
    })

    await wrapper.get('#bulk-edit-openai-compact-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-compact-mode-select"]').setValue('force_on')
    await wrapper.get('#bulk-edit-openai-compact-model-mapping-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-compact-model-mapping-add"]').trigger('click')
    const inputs = wrapper.findAll('[data-testid="bulk-edit-openai-compact-model-mapping-input"]')
    await inputs[0].setValue('gpt-5.4')
    await inputs[1].setValue('gpt-5.4-openai-compact')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith({
      filters: { platform: 'openai' },
      extra: {
        openai_compact_mode: 'force_on'
      },
      credentials: {
        compact_model_mapping: {
          'gpt-5.4': 'gpt-5.4-openai-compact'
        }
      }
    })
  })

  it('OpenAI 账号批量编辑可关闭自动透传', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS, OPENAI_KEY_ENDPOINTS]
    })

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: false,
        openai_oauth_passthrough: false
      }
    })
  })

  it('开启 OpenAI 自动透传时不再同时提交模型限制', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-passthrough-toggle').trigger('click')
    await wrapper.get('#bulk-edit-model-restriction-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: true
      }
    })
    expect(wrapper.text()).toContain('admin.accounts.openai.modelRestrictionDisabledByPassthrough')
  })

  it('filtered-results 模式下应提交 filters 而不是 account_ids', async () => {
    const wrapper = mountModal({
      accountIds: [],
      target: {
        mode: 'filtered',
        filters: {
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          group: '12',
          search: 'bulk-target',
          privacy_mode: 'training_set_cf_blocked'
        },
        previewCount: 5,
        selectedPlatforms: ['openai'],
        selectedTypes: ['oauth']
      }
    })

    await wrapper.get('#bulk-edit-status-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith({
      filters: {
        platform: 'openai',
        type: 'oauth',
        status: 'active',
        group: '12',
        search: 'bulk-target',
        privacy_mode: 'training_set_cf_blocked'
      },
      status: 'active'
    })
  })
  // issue #6327：批量编辑无法把 Codex 指纹收敛关掉。
  //
  // 批量更新走 JSONB 顶层合并（extra = COALESCE(extra,'{}') || payload），删掉 payload
  // 里的键只表示「本次不更新该键」，清不掉账号已有的 device/session/full；而且只删不写会让
  // payload 退化成 {extra:{}}，被后端 len(req.Extra) > 0 判为空更新直接 400
  // "No updates provided"。Create/Edit 那两个表单能删键，是因为它们提交完整 extra 对象、
  // 后端整体 SetExtra 覆盖——两种持久化语义不能共用同一套写法。
  it('OpenAI OAuth 批量编辑选择「关闭」时应显式提交 codex_fingerprint_mode=off（issue #6327）', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    // 下拉框默认就是 off，用户只勾选「编辑该项」即提交——正是 issue 描述的操作路径。
    await wrapper.get('#bulk-edit-openai-codex-fingerprint-mode-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        codex_fingerprint_mode: 'off'
      }
    })

    // 缺陷时期的形状：extra 为空对象，后端必然回 400。显式钉死不得回退。
    const payload = vi.mocked(adminAPI.accounts.bulkUpdate).mock.calls[0][1] as {
      extra: Record<string, unknown>
    }
    expect(Object.keys(payload.extra).length).toBeGreaterThan(0)
  })

  // 与兄弟字段 codex_cli_only 的写法对齐：关闭态同样落显式值，不靠省略表达。
  it('OpenAI OAuth 批量编辑显式 opt-in 模式仍原样提交', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-codex-fingerprint-mode-enabled').setValue(true)
    await wrapper
      .get('[data-testid="bulk-codex-fingerprint-mode-select"]')
      .setValue('session')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        codex_fingerprint_mode: 'session'
      }
    })
  })

  // 未勾选「编辑该项」时不得写入该键，否则批量编辑别的字段会顺手清掉账号的收敛设置。
  it('未勾选编辑该项时不写入 codex_fingerprint_mode', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-codex-cli-only-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-cli-only-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        codex_cli_only: true
      }
    })
  })
})

describe('BulkEditAccountModal third-party key settings do not follow the platform label', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.accounts.bulkUpdate).mockReset().mockResolvedValue({ success: 2, failed: 0, results: [] } as any)
    vi.mocked(adminAPI.accounts.checkMixedChannelRisk).mockReset().mockResolvedValue({ has_risk: false } as any)
    showError.mockReset()
  })

  it('offers OpenAI Responses settings with the key hint for non-OpenAI-labelled keys with OpenAI endpoints', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['kimi', 'anthropic'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [{ chat_completions: 'https://api.moonshot.cn/v1' }, { responses: 'https://relay.example.com/v1' }]
    })

    expect(wrapper.find('[data-testid="bulk-edit-openai-key-protocol-hint"]').exists()).toBe(true)
    // 端点能力同样按协议地址放开，不看标签
    expect(wrapper.find('#bulk-edit-openai-endpoint-capabilities-enabled').exists()).toBe(true)

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-passthrough-toggle').trigger('click')
    await wrapper.get('#bulk-edit-openai-apikey-ws-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-apikey-ws-mode-select"]').setValue('passthrough')
    await wrapper.get('#bulk-edit-openai-compact-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-compact-mode-select"]').setValue('force_off')
    await wrapper.get('#bulk-edit-openai-compact-model-mapping-enabled').setValue(true)
    await wrapper.get('#bulk-edit-model-restriction-enabled').setValue(true)
    expect(wrapper.text()).toContain('admin.accounts.openai.modelRestrictionDisabledByPassthrough')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: true,
        openai_apikey_responses_websockets_v2_mode: 'passthrough',
        openai_apikey_responses_websockets_v2_enabled: true,
        openai_compact_mode: 'force_off'
      },
      credentials: { compact_model_mapping: {} }
    })
  })

  it('hides OpenAI Responses settings when an OpenAI-labelled key lacks OpenAI endpoints', () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS, ANTHROPIC_KEY_ENDPOINTS]
    })

    expect(wrapper.find('#bulk-edit-openai-passthrough-enabled').exists()).toBe(false)
    expect(wrapper.find('#bulk-edit-openai-apikey-ws-mode-enabled').exists()).toBe(false)
    expect(wrapper.find('#bulk-edit-openai-compact-mode-enabled').exists()).toBe(false)
    expect(wrapper.find('#bulk-edit-openai-compact-model-mapping-enabled').exists()).toBe(false)
  })

  it('does not submit OpenAI Responses settings that became hidden after the target changed', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['kimi'],
      selectedTypes: ['apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS]
    })
    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-apikey-ws-mode-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-compact-mode-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-compact-model-mapping-enabled').setValue(true)

    await wrapper.setProps({ selectedKeyEndpoints: [ANTHROPIC_KEY_ENDPOINTS] })
    await wrapper.get('#bulk-edit-status-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], { status: 'active' })
  })

  it('keeps OpenAI subscription rules and hides the key hint when no key is selected', () => {
    const openaiOAuth = mountModal({ selectedPlatforms: ['openai'], selectedTypes: ['oauth', 'setup-token'] })
    expect(openaiOAuth.find('#bulk-edit-openai-passthrough-enabled').exists()).toBe(true)
    expect(openaiOAuth.find('#bulk-edit-openai-compact-mode-enabled').exists()).toBe(true)
    expect(openaiOAuth.find('[data-testid="bulk-edit-openai-key-protocol-hint"]').exists()).toBe(false)
    openaiOAuth.unmount()

    const grokOAuth = mountModal({ selectedPlatforms: ['grok'], selectedTypes: ['oauth'] })
    expect(grokOAuth.find('#bulk-edit-openai-passthrough-enabled').exists()).toBe(false)
    expect(grokOAuth.find('#bulk-edit-openai-compact-mode-enabled').exists()).toBe(false)
    grokOAuth.unmount()

    // 选中里有成品号时仍要求平台只有 openai（只有平台 / 类型集合，无法逐个配对）
    const mixed = mountModal({
      selectedPlatforms: ['openai', 'kimi'],
      selectedTypes: ['oauth', 'apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS]
    })
    expect(mixed.find('#bulk-edit-openai-passthrough-enabled').exists()).toBe(false)
    mixed.unmount()

    const openaiMixed = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth', 'apikey'],
      selectedKeyEndpoints: [OPENAI_KEY_ENDPOINTS]
    })
    expect(openaiMixed.find('#bulk-edit-openai-passthrough-enabled').exists()).toBe(true)
    expect(openaiMixed.find('[data-testid="bulk-edit-openai-key-protocol-hint"]').exists()).toBe(true)
    // key 专属的 WS mode 只在全部是 key 时出现
    expect(openaiMixed.find('#bulk-edit-openai-apikey-ws-mode-enabled').exists()).toBe(false)
    openaiMixed.unmount()
  })

  it('offers header overrides for keys of any label but not for non-Grok subscriptions', () => {
    const keys = mountModal({ selectedPlatforms: ['gemini', 'antigravity'], selectedTypes: ['apikey'] })
    expect(keys.find('#bulk-edit-header-override-enabled').exists()).toBe(true)
    keys.unmount()

    const subscriptions = mountModal({ selectedPlatforms: ['grok', 'openai'], selectedTypes: ['oauth'] })
    expect(subscriptions.find('#bulk-edit-header-override-enabled').exists()).toBe(false)
    subscriptions.unmount()
  })
})
