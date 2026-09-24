import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

import AccountsView from '../AccountsView.vue'

const { listAccounts, listWithEtag, getById, getBatchTodayStats, getUpstreamBillingProbeSettings, getAllProxies } =
  vi.hoisted(() => ({
    listAccounts: vi.fn(),
    listWithEtag: vi.fn(),
    getById: vi.fn(),
    getBatchTodayStats: vi.fn(),
    getUpstreamBillingProbeSettings: vi.fn(),
    getAllProxies: vi.fn(),
  }))

// 渠道页读 ?status= 作为初始筛选（仪表盘「需要处理」跳转用）
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRoute: () => ({ query: {} })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      getById,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn(),
      refreshCredentials: vi.fn()
    },
    proxies: { getAll: getAllProxies },
    modelCatalog: { listEntries: vi.fn().mockResolvedValue([]) }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showWarning: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

// 只渲染「厂商/类型」单元格，看 vendor 是否透传给徽章、协议地址是否变成协议标签。
const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: `
    <div>
      <div v-for="row in data" :key="row.id" :data-account-id="row.id">
        <slot name="cell-platform_type" :row="row" />
      </div>
    </div>
  `
})

const PlatformTypeBadgeStub = defineComponent({
  props: { platform: String, type: String, vendor: String },
  template: '<span data-test="badge" :data-platform="platform" :data-vendor="vendor ?? \'\'">{{ vendor || platform }}</span>'
})

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        DataTable: DataTableStub,
        AccountTableActions: true,
        AccountTableFilters: true,
        AccountBulkActionsBar: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        PlatformTypeBadge: PlatformTypeBadgeStub,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        UpstreamBillingRateCell: true,
        HelpTooltip: true,
        Icon: true,
        Teleport: true
      }
    }
  })
}

const base = { status: 'active', schedulable: true, concurrency: 1, priority: 1, group_ids: [], extra: {}, credentials: {} }

describe('AccountsView vendor/type cell follows the address-based model', () => {
  beforeEach(() => {
    listAccounts.mockReset().mockResolvedValue({
      items: [
        {
          ...base, id: 1, name: 'deepseek-key', platform: 'openai', type: 'apikey', vendor: 'deepseek',
          protocol_endpoints: { chat_completions: 'https://api.deepseek.com', anthropic: 'https://api.deepseek.com/anthropic' }
        },
        { ...base, id: 2, name: 'relay-key', platform: 'kimi', type: 'apikey', vendor: '', protocol_endpoints: { responses: 'https://relay.example/v1' } },
        { ...base, id: 3, name: 'oauth', platform: 'anthropic', type: 'oauth', vendor: 'anthropic' }
      ],
      total: 3, page: 1, page_size: 20, pages: 1
    })
    listWithEtag.mockReset().mockResolvedValue({ notModified: true, etag: 'e', data: null })
    getById.mockReset()
    getBatchTodayStats.mockReset().mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockReset().mockResolvedValue({ enabled: false })
    getAllProxies.mockReset().mockResolvedValue([])
  })

  it('passes the identified vendor to the badge and lists configured protocols with their hosts', async () => {
    const wrapper = mountView()
    await flushPromises()

    const deepseek = wrapper.get('[data-account-id="1"]')
    expect(deepseek.get('[data-test="badge"]').attributes('data-vendor')).toBe('deepseek')
    const chips = deepseek.findAll('[data-testid="key-protocol-chips"] span')
    expect(chips.map((chip) => chip.text())).toEqual([
      'admin.accounts.protocolShort.anthropic',
      'admin.accounts.protocolShort.chat_completions'
    ])
    expect(chips.map((chip) => chip.attributes('title'))).toEqual(['api.deepseek.com', 'api.deepseek.com'])

    const relay = wrapper.get('[data-account-id="2"]')
    expect(relay.get('[data-test="badge"]').attributes('data-vendor')).toBe('')
    expect(relay.findAll('[data-testid="key-protocol-chips"] span').map((chip) => chip.text())).toEqual([
      'admin.accounts.protocolShort.responses'
    ])

    const oauth = wrapper.get('[data-account-id="3"]')
    expect(oauth.find('[data-testid="key-protocol-chips"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
