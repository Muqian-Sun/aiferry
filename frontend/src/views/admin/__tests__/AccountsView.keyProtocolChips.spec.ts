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

const { routerPush } = vi.hoisted(() => ({ routerPush: vi.fn() }))

// 渠道页读 ?status= 作为初始筛选（仪表盘「需要处理」跳转用）；新建 / 编辑渠道走路由（A5）
vi.mock('vue-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-router')>()),
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ push: routerPush })
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

// 只渲染名称单元格（A5 起厂商 / 类型 / 协议并进名称下面那行小字），看 vendor 是否透传给徽章、协议地址是否变成协议标签。
const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: `
    <div>
      <div v-for="row in data" :key="row.id" :data-account-id="row.id">
        <slot name="cell-name" :row="row" :value="row.name" />
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
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="bulk" /><slot name="pagination" /></div>' },
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

  // 2026-09-25 起名称下面只写「厂商 · 接入方式」；协议地址在详情抽屉「概况」里
  it('shows the identified vendor and the access type under the name', async () => {
    const wrapper = mountView()
    await flushPromises()

    const line = (id: number) => wrapper.get(`[data-account-id="${id}"]`).get('[data-testid="account-vendor-line"]').text()
    expect(line(1)).toBe('DeepSeek · admin.accounts.access.apikey')
    // 没认出官方厂商的第三方 key 是中转，不看平台标签
    expect(line(2)).toBe('admin.accounts.vendorRelay · admin.accounts.access.apikey')
    expect(line(3)).toBe('Anthropic · admin.accounts.access.oauth')
    wrapper.unmount()
  })
})
