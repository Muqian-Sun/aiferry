import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

import AccountsView from '../AccountsView.vue'

const { listAccounts, listWithEtag, getById, getBatchTodayStats, getAllProxies, listCatalogEntries } =
  vi.hoisted(() => ({
    listAccounts: vi.fn(),
    listWithEtag: vi.fn(),
    getById: vi.fn(),
    getBatchTodayStats: vi.fn(),
    getAllProxies: vi.fn(),
    listCatalogEntries: vi.fn()
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
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn(),
      refreshCredentials: vi.fn()
    },
    proxies: { getAll: getAllProxies },
    modelCatalog: { listEntries: listCatalogEntries }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({})
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

// 只渲染「已上架模型」单元格：条目的 bindings[] 反查到账号，chip 点击开诊断。
const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] }, columns: { type: Array, default: () => [] } },
  template: `
    <div>
      <span v-for="column in columns" :key="column.key" :data-column="column.key" />
      <div v-for="row in data" :key="row.id" :data-account-id="row.id">
        <slot name="cell-catalog" :row="row" />
      </div>
    </div>
  `
})

const DiagnosisModalStub = defineComponent({
  props: { show: Boolean, entryId: { type: Number, default: null }, modelId: { type: String, default: '' } },
  template: '<div data-test="diagnosis" :data-show="show" :data-entry-id="entryId ?? \'\'" :data-model-id="modelId" />'
})

const DetailDrawerStub = defineComponent({
  props: { account: { type: Object, default: null }, tab: { type: String, default: '' }, catalogEntries: { type: Array, default: () => [] } },
  template: '<div data-test="detail-drawer" :data-account-id="account?.id ?? \'\'" :data-tab="tab" :data-entry-count="catalogEntries.length" />'
})

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="bulk" /><slot name="pagination" /></div>' },
        DataTable: DataTableStub,
        CatalogEntryDiagnosisModal: DiagnosisModalStub,
        AccountDetailDrawer: DetailDrawerStub,
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
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        HelpTooltip: true,
        Icon: true,
        Teleport: true
      }
    }
  })
}

const base = { status: 'active', schedulable: true, concurrency: 1, priority: 1, group_ids: [], extra: {}, credentials: {} }
const entry = (id: number, model_id: string, status: string, accountIDs: number[]) => ({
  id, model_id, display_name: model_id, vendor: 'openai', protocols: [], billing_mode: 'token', status, managed_by: 'seed',
  input_price: 1, output_price: 2, cache_write_price: null, cache_write_1h_price: null, cache_read_price: null,
  image_input_price: null, image_output_price: null, image_cache_read_price: null,
  per_request_price: null, search_price_per_call: null, notes: '',
  intervals: [], time_pricing: null,
  bindings: accountIDs.map((account_id) => ({ entry_id: id, account_id, priority: null })),
  created_at: '', updated_at: ''
})

describe('AccountsView listed-models column', () => {
  beforeEach(() => {
    listAccounts.mockReset().mockResolvedValue({
      items: [
        { ...base, id: 1, name: 'K1', platform: 'openai', type: 'apikey' },
        { ...base, id: 2, name: 'K2', platform: 'anthropic', type: 'apikey' }
      ],
      total: 2, page: 1, page_size: 20, pages: 1
    })
    listWithEtag.mockReset().mockResolvedValue({ notModified: true, etag: 'e', data: null })
    getById.mockReset()
    getBatchTodayStats.mockReset().mockResolvedValue({ stats: {} })
    getAllProxies.mockReset().mockResolvedValue([])
    listCatalogEntries.mockReset().mockResolvedValue([
      entry(199, 'gpt-5.6', 'listed', [1]),
      entry(217, 'gpt-5.6-mini', 'unlisted', [1]),
      entry(27, 'claude-sonnet-4-5', 'listed', [])
    ])
  })

  // 2026-09-25 起列表只写已上架模型数，点开是详情抽屉的「上架模型」页签（诊断在抽屉里）
  it('derives each account\'s catalog entries from the entries\' bindings and opens the models tab on click', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-column="catalog"]').exists()).toBe(true)

    const k1 = wrapper.get('[data-account-id="1"]')
    const cell = k1.get('[data-testid="account-catalog-cell"]')
    expect(cell.get('[data-testid="account-catalog-count"]').text()).toBe('1')
    expect(cell.get('[data-testid="account-catalog-unlisted"]').text()).toBe('admin.accounts.catalogUnlistedCount')
    expect(cell.attributes('title')).toBe('gpt-5.6\ngpt-5.6-mini')
    expect(k1.find('[data-testid="account-catalog-none"]').exists()).toBe(false)

    const k2 = wrapper.get('[data-account-id="2"]')
    expect(k2.find('[data-testid="account-catalog-cell"]').exists()).toBe(false)
    expect(k2.find('[data-testid="account-catalog-none"]').exists()).toBe(true)

    const drawer = wrapper.get('[data-test="detail-drawer"]')
    expect(drawer.attributes('data-account-id')).toBe('')
    await cell.trigger('click')
    expect(drawer.attributes('data-account-id')).toBe('1')
    expect(drawer.attributes('data-tab')).toBe('models')
    expect(drawer.attributes('data-entry-count')).toBe('2')
  })

  // 分组随目录下线：渠道页不再有「分组」列，目录列是唯一的归属信息
  it('has no groups column any more', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-column="groups"]').exists()).toBe(false)
  })
})
