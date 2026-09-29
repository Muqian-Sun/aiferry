import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BulkEditAccountModal from '../BulkEditAccountModal.vue'
import { adminAPI } from '@/api/admin'

const { translate } = vi.hoisted(() => ({
  translate: vi.fn((key: string) => key)
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({})
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

function mountModal(extraProps: Record<string, unknown> = {}) {
  return mount(BulkEditAccountModal, {
    props: {
      show: true,
      accountIds: [1, 2],
      selectedPlatforms: ['antigravity'],
      selectedTypes: ['apikey'],
      proxies: [],
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
        Icon: true
      }
    }
  })
}

describe('BulkEditAccountModal', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.accounts.bulkUpdate).mockReset()
    vi.mocked(adminAPI.accounts.checkMixedChannelRisk).mockReset()
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


  // 去掉白名单后只有改名：Antigravity 自带模型表，同名预设（把表外模型加进来）也保留
  it('antigravity 改名预设包含图片映射并过滤 OpenAI 预设', async () => {
    const wrapper = mountModal()

    expect(wrapper.text()).toContain('3.1-Flash-Image透传')
    expect(wrapper.text()).toContain('3-Pro-Image→3.1')
    expect(wrapper.text()).not.toContain('GPT-5.3 Codex Spark')
  })

  it('仅勾选模型改名且不填时，提交空 model_mapping 覆盖各账号的映射，并带只改名标记', async () => {
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
        model_mapping: {},
        model_mapping_rename_only: true
      }
    })
  })

  it('不再提供批量修改 Base URL 的入口', () => {
    // 成品号只走官方地址，第三方 key 的地址只在协议映射里（批量更新不改映射）
    const wrapper = mountModal({ selectedPlatforms: ['grok'], selectedTypes: ['oauth'] })

    expect(wrapper.find('#bulk-edit-base-url').exists()).toBe(false)
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
})

describe('BulkEditAccountModal third-party key settings do not follow the platform label', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.accounts.bulkUpdate).mockReset().mockResolvedValue({ success: 2, failed: 0, results: [] } as any)
    vi.mocked(adminAPI.accounts.checkMixedChannelRisk).mockReset().mockResolvedValue({ has_risk: false } as any)
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
