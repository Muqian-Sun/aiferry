import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'
import { getAccountPlanType } from '@/components/admin/account/accountDisplay'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

// 外审 F2:AccountActionMenu emit 'create-spark-shadow',但 AccountsView 此前未监听,
// 导致按钮点击无效。本测试通过真实组件引用 emit 该事件,断言父页面接线调用 API。
const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  duplicateAccount,
  createSparkShadow,
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  duplicateAccount: vi.fn(),
  createSparkShadow: vi.fn(),
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
      listWithEtag,
      getBatchTodayStats,
      duplicate: duplicateAccount,
      createSparkShadow,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: { getAll: getAllProxies },
    modelCatalog: { listEntries: vi.fn().mockResolvedValue([]) }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({})
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token' })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const mountView = () =>
  mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="bulk" /><slot name="pagination" /></div>'
        },
        DataTable: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: { template: '<div></div>' },
        AccountBulkActionsBar: true,
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
        Icon: true
      }
    }
  })

describe('admin AccountsView — 外审 F2:spark 影子创建接线', () => {
  beforeEach(() => {
    localStorage.clear()
    for (const fn of [listAccounts, listWithEtag, getBatchTodayStats, getAllProxies, duplicateAccount, createSparkShadow]) {
      fn.mockReset()
    }
    listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    duplicateAccount.mockResolvedValue({ id: 998, name: 'parent-acc (Copy)' })
    createSparkShadow.mockResolvedValue({ id: 999, name: 'parent-acc (Spark)' })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('AccountActionMenu 的 duplicate 事件一键复制账号并刷新列表', async () => {
    const wrapper = mountView()
    await flushPromises()

    wrapper.findComponent(AccountActionMenu).vm.$emit('duplicate', { id: 42, name: 'parent-acc' })
    await flushPromises()

    expect(duplicateAccount).toHaveBeenCalledTimes(1)
    expect(duplicateAccount).toHaveBeenCalledWith(42)
    expect(listAccounts.mock.calls.length).toBeGreaterThan(1)
    wrapper.unmount()
  })

  it('同一账号复制请求未完成时忽略重复点击', async () => {
    let resolveDuplicate!: (account: { id: number; name: string }) => void
    duplicateAccount.mockImplementationOnce(() => new Promise(resolve => { resolveDuplicate = resolve }))
    const wrapper = mountView()
    await flushPromises()

    const menu = wrapper.findComponent(AccountActionMenu)
    menu.vm.$emit('duplicate', { id: 42, name: 'parent-acc' })
    menu.vm.$emit('duplicate', { id: 42, name: 'parent-acc' })
    await flushPromises()

    expect(duplicateAccount).toHaveBeenCalledTimes(1)
    resolveDuplicate({ id: 998, name: 'parent-acc (Copy)' })
    await flushPromises()
    wrapper.unmount()
  })


  it('AccountActionMenu 的 create-spark-shadow 事件触发 createSparkShadow API + 成功提示', async () => {
    const wrapper = mountView()
    await flushPromises()

    const menu = wrapper.findComponent(AccountActionMenu)
    expect(menu.exists()).toBe(true)

    menu.vm.$emit('create-spark-shadow', { id: 42, name: 'parent-acc' })
    await flushPromises()

    // 不再用原生 confirm,改用应用内 ConfirmDialog:先弹出,点确认才调 API
    const dialog = wrapper.findAllComponents(ConfirmDialog).find(d => d.props('show'))
    expect(dialog).toBeTruthy()
    dialog?.vm.$emit('confirm')
    await flushPromises()

    expect(createSparkShadow).toHaveBeenCalledTimes(1)
    expect(createSparkShadow).toHaveBeenCalledWith(42, { name: 'parent-acc (Spark)' })
    wrapper.unmount()
  })

  it('用户取消确认时不调用 API', async () => {
    const wrapper = mountView()
    await flushPromises()

    wrapper.findComponent(AccountActionMenu).vm.$emit('create-spark-shadow', { id: 42, name: 'parent-acc' })
    await flushPromises()

    // 弹出 ConfirmDialog 后点取消,不应调用 API
    const dialog = wrapper.findAllComponents(ConfirmDialog).find(d => d.props('show'))
    expect(dialog).toBeTruthy()
    dialog?.vm.$emit('cancel')
    await flushPromises()

    expect(createSparkShadow).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})

// 账号行展示
// 使用能透传 row 数据的自定义 DataTable stub，以便渲染 cell 插槽
const RowDataTableStub = {
  props: ['data', 'columns', 'loading'],
  template: `<div>
    <div v-for="(row, idx) in (data || [])" :key="idx">
      <slot name="cell-name" :row="row" :value="row.name" />
      <slot name="cell-platform_type" :row="row" />
    </div>
  </div>`
}
// 套餐（2026-09-25 起在详情抽屉副标题里显示）按列表行数据推导
const planTypesOf = (wrapper: ReturnType<typeof mountViewWithRow>) =>
  (wrapper.getComponent(RowDataTableStub).props('data') as Array<Record<string, unknown>>).map((row) => getAccountPlanType(row))
const mountViewWithRow = () =>
  mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="bulk" /><slot name="pagination" /></div>'
        },
        DataTable: RowDataTableStub,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: { template: '<div></div>' },
        AccountBulkActionsBar: true,
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
        Icon: true
      }
    }
  })

describe('admin AccountsView — 账号行展示', () => {
  beforeEach(() => {
    localStorage.clear()
    for (const fn of [listAccounts, listWithEtag, getBatchTodayStats, getAllProxies, duplicateAccount, createSparkShadow]) {
      fn.mockReset()
    }
    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    vi.stubGlobal('confirm', vi.fn(() => true))
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('仅将协议地址安全的 API Key 账号名称链接到站点主页', async () => {
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 101,
          name: 'relay-account',
          platform: 'openai',
          type: 'apikey',
          // 凭据里残留的旧 base_url 不再被读取；按协议顺序取第一个已配置的地址
          credentials: { base_url: 'https://stale.example.com/v1' },
          protocol_endpoints: { responses: 'https://second.example.com/v1', chat_completions: 'https://relay.example.com/api/v1/' },
        },
        { id: 102, name: 'oauth-account', platform: 'openai', type: 'oauth', credentials: { base_url: 'https://oauth.example.com/v1' } },
        { id: 103, name: 'invalid-url', platform: 'openai', type: 'apikey', protocol_endpoints: { chat_completions: 'javascript:alert(1)' } },
        { id: 104, name: 'legacy-base-url-only', platform: 'openai', type: 'apikey', credentials: { base_url: 'https://legacy.example.com/v1' } },
      ],
      total: 4,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountViewWithRow()
    await flushPromises()

    const links = wrapper.findAll('a')
    expect(links).toHaveLength(1)
    const [link] = links
    expect(link.text()).toBe('relay-account')
    expect(link.attributes()).toMatchObject({
      href: 'https://relay.example.com',
      target: '_blank',
      rel: 'noopener noreferrer',
    })
    // A5：名称是墨色文字链接（悬停下划线），完整地址放在 title 里，不再套 HelpTooltip
    expect(link.classes()).toEqual(expect.arrayContaining(['text-af-ink']))
    expect(link.classes()).not.toContain('text-af-brand')
    expect(link.attributes('title')).toBe('https://relay.example.com')
    expect(wrapper.text()).toContain('oauth-account')
    expect(wrapper.text()).toContain('invalid-url')

    wrapper.unmount()
  })

  it('prefers persisted Grok JWT tier over lagging billing/quota snapshots', async () => {
    const grokAccounts = [
      {
        id: 201,
        name: 'oauth-tier',
        platform: 'grok',
        type: 'oauth',
        credentials: { subscription_tier: 'FREE', plan_type: 'legacy' },
        extra: {
          grok_billing_snapshot: { plan: 'SuperGrok' },
          subscription_tier: 'BASIC',
        },
      },
      {
        id: 202,
        name: 'billing-tier',
        platform: 'grok',
        type: 'oauth',
        credentials: {},
        extra: {
          grok_billing_snapshot: { plan: 'SuperGrok Heavy' },
          subscription_tier: 'BASIC',
        },
      },
      {
        id: 203,
        name: 'quota-tier',
        platform: 'grok',
        type: 'oauth',
        credentials: { subscription_tier: 'FREE' },
        extra: {
          grok_quota_snapshot: { subscription_tier: 'SuperGrok' },
          subscription_tier: 'BASIC',
        },
      },
      {
        id: 204,
        name: 'extra-tier',
        platform: 'grok',
        type: 'oauth',
        credentials: { plan_type: 'SuperGrok' },
        extra: { subscription_tier: 'BASIC' },
      },
      {
        id: 205,
        name: 'legacy-tier',
        platform: 'grok',
        type: 'oauth',
        credentials: { plan_type: 'SuperGrok' },
      },
      {
        id: 206,
        name: 'supergrokpro-responses-quota',
        platform: 'grok',
        type: 'oauth',
        credentials: { subscription_tier: 'SuperGrokPro' },
        extra: {
          grok_billing_snapshot: { plan: 'SuperGrok' },
          grok_usage_snapshot: {
            model: 'grok-4.5',
            last_headers_seen_at: new Date().toISOString(),
            requests: { limit: 8300 },
            tokens: { limit: 53_000_000 },
          },
          grok_quota_snapshot: {
            model: 'grok-4.6',
            last_headers_seen_at: new Date().toISOString(),
            requests: { limit: 8300 },
            tokens: { limit: 53_000_000 },
          },
        },
      },
      {
        id: 207,
        name: 'supergrokpro-other-model-quota',
        platform: 'grok',
        type: 'oauth',
        credentials: { subscription_tier: 'SuperGrokPro' },
        extra: {
          grok_billing_snapshot: { plan: 'SuperGrok' },
          grok_usage_snapshot: {
            model: 'grok-4.6',
            last_headers_seen_at: new Date().toISOString(),
            requests: { limit: 8300 },
            tokens: { limit: 53_000_000 },
          },
        },
      },
      {
        id: 208,
        name: 'usage-over-legacy-quota',
        platform: 'grok',
        type: 'oauth',
        credentials: {},
        extra: {
          grok_usage_snapshot: { subscription_tier: 'SuperGrok' },
          grok_quota_snapshot: { subscription_tier: 'Free' },
        },
      },
      {
        id: 209,
        name: 'legacy-quota-alias',
        platform: 'grok',
        type: 'oauth',
        credentials: {},
        extra: { grok_quota_snapshot: { subscription_tier: 'SuperGrok' } },
      },
    ]

    listAccounts.mockResolvedValue({
      items: grokAccounts,
      total: grokAccounts.length,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountViewWithRow()
    await flushPromises()

    const planTypes = planTypesOf(wrapper)
    expect(planTypes).toEqual([
      'FREE',
      'SuperGrok Heavy',
      'FREE',
      'BASIC',
      'SuperGrok',
      'SuperGrok Heavy',
      'SuperGrok',
      'SuperGrok',
      'SuperGrok',
    ])

    wrapper.unmount()
  })

  it('skips malformed Grok plan fields and safely uses the next valid fallback', async () => {
    const grokAccounts = [
      {
        id: 210,
        name: 'legacy-fallback',
        platform: 'grok',
        type: 'oauth',
        credentials: {},
        extra: {
          grok_usage_snapshot: { subscription_tier: { name: 'SuperGrok Heavy' } },
          grok_quota_snapshot: { subscription_tier: 'SuperGrok' },
        },
      },
      {
        id: 211,
        name: 'credential-plan-fallback',
        platform: 'grok',
        type: 'oauth',
        credentials: { subscription_tier: 0, plan_type: 'SuperGrok Heavy' },
        extra: {
          grok_billing_snapshot: { plan: {} },
          grok_usage_snapshot: { subscription_tier: 1 },
          grok_quota_snapshot: { subscription_tier: [] },
          subscription_tier: '   ',
        },
      },
      {
        id: 212,
        name: 'no-valid-plan',
        platform: 'grok',
        type: 'oauth',
        credentials: { subscription_tier: {}, plan_type: 2 },
        parent_plan_type: [],
        extra: {
          grok_billing_snapshot: { plan: [] },
          grok_usage_snapshot: { subscription_tier: 1 },
          grok_quota_snapshot: { subscription_tier: {} },
          subscription_tier: null,
        },
      },
    ]

    listAccounts.mockResolvedValue({
      items: grokAccounts,
      total: grokAccounts.length,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountViewWithRow()
    await flushPromises()

    expect(planTypesOf(wrapper)).toEqual([
      'SuperGrok',
      'SuperGrok Heavy',
      undefined,
    ])
    wrapper.unmount()
  })

  it('replaces a Grok row when auto refresh returns a changed canonical usage snapshot', async () => {
    vi.useFakeTimers()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))

    const initialAccount = {
      id: 213,
      name: 'refresh-tier',
      platform: 'grok',
      type: 'oauth',
      extra: { grok_usage_snapshot: { subscription_tier: 'Free', status_code: 200 } },
    }
    const refreshedAccount = {
      ...initialAccount,
      extra: { grok_usage_snapshot: { subscription_tier: 'SuperGrok', status_code: 200 } },
    }
    listAccounts.mockResolvedValue({ items: [initialAccount], total: 1, page: 1, page_size: 20, pages: 1 })
    listWithEtag.mockResolvedValueOnce({
      notModified: false,
      etag: 'grok-snapshot-2',
      data: { items: [refreshedAccount], total: 1, page: 1, page_size: 20, pages: 1 },
    })

    const wrapper = mountViewWithRow()
    await flushPromises()
    expect(planTypesOf(wrapper)).toEqual(['Free'])

    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()

    expect(listWithEtag).toHaveBeenCalledTimes(1)
    expect(planTypesOf(wrapper)).toEqual(['SuperGrok'])
    wrapper.unmount()
  })
})
