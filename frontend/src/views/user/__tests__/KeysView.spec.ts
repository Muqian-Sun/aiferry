import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'

import type { ApiKey } from '@/types'
import { keysAPI } from '@/api'
import FilterChip from '@/components/common/FilterChip.vue'
import KeysView from '../KeysView.vue'

const {
  listKeys,
  updateKey,
  getPublicSettings,
  getDashboardApiKeysUsage,
  getMyApiKeyDailyUsage,
  copyToClipboard,
  isCurrentStep,
  nextStep,
} = vi.hoisted(() => ({
  listKeys: vi.fn(),
  updateKey: vi.fn(),
  getPublicSettings: vi.fn(),
  getDashboardApiKeysUsage: vi.fn(),
  getMyApiKeyDailyUsage: vi.fn(),
  copyToClipboard: vi.fn(),
  isCurrentStep: vi.fn(),
  nextStep: vi.fn(),
}))

const messages: Record<string, string> = {
  'common.actions': 'Actions',
  'common.name': 'Name',
  'common.refresh': 'Refresh',
  'common.status': 'Status',
  'keys.apiKey': 'API Key',
  'keys.allStatus': 'All Status',
  'keys.columnSettings': 'Column Settings',
  'keys.createKey': 'Create API Key',
  'keys.created': 'Created',
  'keys.expiresAt': 'Expires',
  'keys.id': 'ID',
  'keys.currentConcurrency': 'Current Concurrency',
  'keys.lastUsedAt': 'Last Used',
  'keys.lastUsedIP': 'Last Used IP',
  'keys.rateLimitColumn': 'Rate Limit',
  'keys.searchPlaceholder': 'Search name or key...',
  'keys.status.active': 'Active',
  'keys.status.expired': 'Expired',
  'keys.status.inactive': 'Inactive',
  'keys.status.quota_exhausted': 'Quota exhausted',
  'keys.usage': 'Usage',
}

vi.mock('@/api', () => ({
  keysAPI: {
    list: listKeys,
    create: vi.fn(),
    update: updateKey,
    delete: vi.fn(),
    toggleStatus: vi.fn(),
  },
  authAPI: {
    getPublicSettings,
  },
  usageAPI: {
    getDashboardApiKeysUsage,
    getMyApiKeyDailyUsage,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({}),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep,
    nextStep,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const createApiKey = (): ApiKey => ({
  id: 1,
  user_id: 1,
  key: 'sk-test-key',
  name: 'test-key',
  group_id: null,
  subscription_id: null,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  last_used_ip: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-06-27T00:00:00Z',
  updated_at: '2026-06-27T00:00:00Z',
  current_concurrency: 3,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
})

const SiteShellStub = {
  template: '<div><slot name="actions" /><slot /></div>',
}

const DataTableStub = {
  name: 'DataTable',
  props: { columns: Array, data: Array, selectedKeys: Array, selectable: Boolean },
  emits: ['sort', 'update:selectedKeys', 'rowClick'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map((col) => col.key).join(',') }}</div>
      <div data-test="columns-meta">{{ JSON.stringify(columns.map((col) => ({ key: col.key, sortable: !!col.sortable }))) }}</div>
      <button data-test="sort-current-concurrency" @click="$emit('sort', 'current_concurrency', 'asc')">
        Sort Current Concurrency
      </button>
      <div v-for="row in data" :key="row.id" data-test="row">
        <button data-test="row-open" @click="$emit('rowClick', row)">Open {{ row.name }}</button>
        <div
          v-if="columns.some((col) => col.key === 'id')"
          data-test="key-id"
        >
          <slot name="cell-id" :value="row.id" :row="row" />
        </div>
        <slot name="cell-name" :value="row.name" :row="row" />
        <slot name="cell-actions" :row="row" />
        <div data-test="current-concurrency">
          <slot name="cell-current_concurrency" :value="row.current_concurrency" :row="row" />
        </div>
        <div
          v-if="columns.some((col) => col.key === 'last_used_ip')"
          data-test="last-used-ip"
        >
          <slot name="cell-last_used_ip" :value="row.last_used_ip" :row="row" />
        </div>
      </div>
      <slot name="empty" />
    </div>
  `,
}

const SelectStub = {
  name: 'Select',
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"></select>',
}

const SearchInputStub = {
  name: 'SearchInput',
  props: ['modelValue'],
  emits: ['update:modelValue', 'search'],
  template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
}

const PaginationStub = {
  name: 'Pagination',
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div>
      <button data-test="page-size-50" @click="$emit('update:pageSize', 50)">50</button>
    </div>
  `,
}

const IconStub = {
  props: ['name'],
  template: '<span data-test="icon">{{ name }}</span>',
}

const mountView = async () => {
  const wrapper = mount(KeysView, {
    global: {
      stubs: {
        SiteShell: SiteShellStub,
        DataTable: DataTableStub,
        Pagination: PaginationStub,
        BaseDialog: {
          props: ['show', 'title'],
          emits: ['close'],
          template: '<div v-if="show" role="dialog"><button data-test="close-dialog" @click="$emit(\'close\')">Close</button><slot /><slot name="footer" /></div>',
        },
        ConfirmDialog: true,
        StatusState: true,
        Select: SelectStub,
        SearchInput: SearchInputStub,
        Icon: IconStub,
        UseKeyModal: true,
        UsageMetricTrend: true,
        RouterLink: { props: ['to'], template: '<a data-test="router-link" :data-to="JSON.stringify(to)"><slot /></a>' },
        BulkEditKeysModal: true,
        EndpointPopover: true,
        Teleport: true,
      },
    },
  })
  await flushPromises()
  await nextTick()
  return wrapper
}

const visibleColumnKeys = (wrapper: VueWrapper) =>
  wrapper.get('[data-test="columns"]').text().split(',').filter(Boolean)

const visibleColumnMeta = (wrapper: VueWrapper): Array<{ key: string; sortable: boolean }> =>
  JSON.parse(wrapper.get('[data-test="columns-meta"]').text())

const toggleColumn = async (wrapper: VueWrapper, key: string) => {
  await wrapper.get('[data-testid="column-settings"]').trigger('click')
  await nextTick()
  await wrapper.get(`[data-testid="column-toggle-${key}"]`).trigger('click')
  await nextTick()
}

describe('user KeysView column settings', () => {
  beforeEach(() => {
    localStorage.clear()

    listKeys.mockReset()
    updateKey.mockReset()
    vi.mocked(keysAPI.create).mockReset()
    getPublicSettings.mockReset()
    getDashboardApiKeysUsage.mockReset()
    getMyApiKeyDailyUsage.mockReset().mockResolvedValue({ items: [], days: 30, start_date: '', end_date: '' })
    copyToClipboard.mockReset()
    isCurrentStep.mockReset()
    nextStep.mockReset()

    listKeys.mockResolvedValue({
      items: [createApiKey()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getPublicSettings.mockResolvedValue({})
    getDashboardApiKeysUsage.mockResolvedValue({ stats: {} })
    isCurrentStep.mockReturnValue(false)
  })

  // 额度 / 速率的「已用」与重置在详情抽屉里（muqian 2026-09-25），编辑表单只放设置
  it('resets used quota from the key drawer and updates the row in place', async () => {
    const key: ApiKey = { ...createApiKey(), quota: 10, quota_used: 10, status: 'quota_exhausted' }
    listKeys.mockResolvedValue({ items: [key], total: 1, page: 1, page_size: 20, pages: 1 })
    updateKey.mockResolvedValue({ ...key, status: 'active', quota_used: 0 })
    const wrapper = await mountView()

    await wrapper.get('[data-test="row-open"]').trigger('click')
    await flushPromises()
    expect(getMyApiKeyDailyUsage).toHaveBeenCalledWith(key.id, 30)
    expect(wrapper.get('[data-testid="key-drawer-limit-quota"]').text()).toContain('$10.00')

    await wrapper.get('[data-testid="key-drawer-reset-quota"]').trigger('click')
    const confirmation = wrapper.findAllComponents({ name: 'ConfirmDialog' })
      .find((dialog) => dialog.props('title') === 'keys.resetQuotaTitle')!
    expect(confirmation.props('show')).toBe(true)
    confirmation.vm.$emit('confirm')
    await flushPromises()

    expect(updateKey).toHaveBeenCalledWith(key.id, { reset_quota: true })
    expect(wrapper.findComponent({ name: 'DataTable' }).props('data')[0]).toMatchObject({ status: 'active', quota_used: 0 })
    // 已用归零后抽屉里不再提供重置
    expect(wrapper.find('[data-testid="key-drawer-reset-quota"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps usage and reset controls out of the edit form and submits the settings', async () => {
    const key: ApiKey = { ...createApiKey(), quota: 10, quota_used: 4, rate_limit_5h: 5, usage_5h: 1 }
    listKeys.mockResolvedValue({ items: [key], total: 1, page: 1, page_size: 20, pages: 1 })
    updateKey.mockResolvedValue(key)
    const wrapper = await mountView()

    await wrapper.get('[data-testid="edit-key"]').trigger('click')
    expect(wrapper.find('button[title="keys.resetQuotaUsed"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="key-form-more"]').exists()).toBe(false)
    await wrapper.get('[data-tour="key-form-name"]').setValue('Renamed')
    await wrapper.get('#key-form').trigger('submit')
    await flushPromises()

    expect(updateKey).toHaveBeenCalledWith(key.id, expect.objectContaining({ name: 'Renamed', status: 'active', quota: 10, rate_limit_5h: 5 }))
    wrapper.unmount()
  })

  it('opens the drawer on the usage-instructions tab from the row action', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="use-key"]').trigger('click')
    await flushPromises()

    const useKey = wrapper.findComponent({ name: 'UseKeyModal' })
    expect(useKey.props('show')).toBe(true)
    expect(useKey.props('layout')).toBe('inline')
    expect(useKey.props('apiKey')).toBe('sk-test-key')
    expect(wrapper.find('[data-testid="key-drawer-overview"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('links the drawer to the usage page filtered by this key', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-test="row-open"]').trigger('click')
    await flushPromises()

    const link = wrapper.get('[data-testid="key-drawer-usage-link"]')
    expect(JSON.parse(link.attributes('data-to')!)).toEqual({ path: '/usage', query: { key: '1' } })
    wrapper.unmount()
  })

  it('offers near-limit / expiring in the status filter and filters to them', async () => {
    const soon = new Date(Date.now() + 2 * 24 * 60 * 60 * 1000).toISOString()
    const keys: ApiKey[] = [
      { ...createApiKey(), id: 1, name: 'fine' },
      { ...createApiKey(), id: 2, name: 'expired', status: 'expired' },
      { ...createApiKey(), id: 3, name: 'near-limit', quota: 10, quota_used: 9 },
      { ...createApiKey(), id: 4, name: 'expiring', expires_at: soon },
    ]
    listKeys.mockResolvedValue({ items: keys, total: 4, page: 1, page_size: 20, pages: 1 })
    const wrapper = await mountView()

    // 没有筛选且一页装得下：直接用这一页，不再多拉
    expect(listKeys).toHaveBeenCalledTimes(1)
    const statusChip = wrapper.findAllComponents(FilterChip).find((chip) => chip.props('testId') === 'keys-filter-status')!
    expect(statusChip.props('options').map((option: { value: string }) => option.value)).toEqual(
      expect.arrayContaining(['expired', 'quota_exhausted', 'near_limit', 'expiring'])
    )

    // 限额将满：没有后端筛选，从全部密钥里挑出来，不分页
    statusChip.vm.$emit('update:modelValue', 'near_limit')
    await flushPromises()
    const table = wrapper.findComponent({ name: 'DataTable' })
    expect(table.props('data').map((key: ApiKey) => key.name)).toEqual(['near-limit'])
    expect(wrapper.findComponent({ name: 'Pagination' }).exists()).toBe(false)
    expect(statusChip.props('modelValue')).toBe('near_limit')

    // 已过期：走后端状态筛选
    listKeys.mockClear()
    statusChip.vm.$emit('update:modelValue', 'expired')
    await flushPromises()
    expect(listKeys).toHaveBeenLastCalledWith(1, 20, expect.objectContaining({ status: 'expired' }), expect.anything())
    wrapper.unmount()
  })

  it('loads every key for the near-limit / expiring filters when the list is paged', async () => {
    listKeys.mockImplementation((page: number, pageSize: number) =>
      Promise.resolve({ items: [{ ...createApiKey(), id: page * 1000 + pageSize, status: 'expired' }], total: 30, page, page_size: pageSize, pages: pageSize === 100 ? 1 : 2 })
    )
    const wrapper = await mountView()

    expect(listKeys).toHaveBeenCalledWith(1, 100)
    const statusChip = wrapper.findAllComponents(FilterChip).find((chip) => chip.props('testId') === 'keys-filter-status')!
    expect(statusChip.props('options').map((option: { value: string }) => option.value)).toContain('near_limit')
    wrapper.unmount()
  })

  // 订阅 key 随订阅生成：列表里带「订阅 · 套餐」标签且没有删除按钮；余额 key 有删除按钮
  it('marks subscription keys and hides their delete action', async () => {
    listKeys.mockResolvedValue({
      items: [
        { ...createApiKey(), id: 11, name: 'E2E Pro', subscription_id: 5, subscription_plan_name: 'E2E Pro' },
        { ...createApiKey(), id: 12, name: 'balance-key' },
      ],
      total: 2, page: 1, page_size: 20, pages: 1,
    })
    const wrapper = await mountView()

    const badges = wrapper.findAll('[data-testid="subscription-key-badge"]')
    expect(badges).toHaveLength(1)
    expect(badges[0].text()).toContain('keys.subscriptionKey')
    // 删除藏在每行的「更多」菜单里：订阅 key 的菜单没有删除项，余额 key 的有
    const menus = wrapper.findAll('[data-testid="key-menu"]')
    expect(menus).toHaveLength(2)
    await menus[0].trigger('click')
    expect(wrapper.find('[data-testid="delete-key"]').exists()).toBe(false)
    await menus[1].trigger('click')
    expect(wrapper.findAll('[data-testid="delete-key"]')).toHaveLength(1)
    // 一次只开一个菜单
    expect(wrapper.findAll('[data-testid="key-menu-items"]')).toHaveLength(1)
    wrapper.unmount()
  })

  it('uses the default API key columns with low-frequency columns hidden', async () => {
    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'status',
      'usage',
      'last_used_at',
      'expires_at',
      'actions',
    ])
    expect(visibleColumnKeys(wrapper)).not.toContain('current_concurrency')
    expect(visibleColumnKeys(wrapper)).not.toContain('created_at')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_ip')
    expect(visibleColumnKeys(wrapper)).not.toContain('id')
  })

  it('opens bulk editing with only selected visible keys', async () => {
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'DataTable' })
    expect(table.props('selectable')).toBe(true)
    table.vm.$emit('update:selectedKeys', [1, 99])
    await nextTick()
    await wrapper.get('[data-test="bulk-edit-keys"]').trigger('click')
    const modal = wrapper.findComponent({ name: 'BulkEditKeysModal' })
    expect(modal.props('show')).toBe(true)
    expect(modal.props('selectedKeys').map((key: ApiKey) => key.id)).toEqual([1])
    wrapper.unmount()
  })

  it.each(['filter', 'page size', 'sort'])('clears selection on %s changes', async (change) => {
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'DataTable' })
    table.vm.$emit('update:selectedKeys', [1])
    await nextTick()
    if (change === 'filter') {
      wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
    } else if (change === 'page size') {
      await wrapper.get('[data-test="page-size-50"]').trigger('click')
    } else {
      table.vm.$emit('sort', 'created_at', 'asc')
    }
    await flushPromises()
    expect(table.props('selectedKeys')).toEqual([])
    expect(wrapper.find('[data-test="bulk-edit-keys"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('removes successful keys from the selection and refreshes the table', async () => {
    listKeys.mockResolvedValue({
      items: [createApiKey(), { ...createApiKey(), id: 2, name: 'Second' }],
      total: 2, pages: 1
    })
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'DataTable' })
    table.vm.$emit('update:selectedKeys', [1, 2])
    await nextTick()
    await wrapper.get('[data-test="bulk-edit-keys"]').trigger('click')
    wrapper.findComponent({ name: 'BulkEditKeysModal' }).vm.$emit('updated', [1])
    await flushPromises()
    expect(listKeys).toHaveBeenCalledTimes(2)
    expect(table.props('selectedKeys')).toEqual([2])
    wrapper.unmount()
  })

  it('drops keys that are no longer visible after a refresh', async () => {
    const wrapper = await mountView()
    const table = wrapper.findComponent({ name: 'DataTable' })
    table.vm.$emit('update:selectedKeys', [1])
    await nextTick()
    listKeys.mockResolvedValue({ items: [], total: 0, pages: 0 })
    await wrapper.get('button[title="Refresh"]').trigger('click')
    await flushPromises()
    expect(table.props('selectedKeys')).toEqual([])
    wrapper.unmount()
  })

  it('shows a hidden column when toggled and persists the preference', async () => {
    const wrapper = await mountView()

    await toggleColumn(wrapper, 'current_concurrency')

    expect(visibleColumnKeys(wrapper)).toContain('current_concurrency')
    expect(JSON.parse(localStorage.getItem('user-keys-columns')!)).toEqual({
      version: 2,
      hidden: ['last_used_ip', 'created_at'],
      shown: [],
    })
  })

  it('shows the last used IP column when toggled', async () => {
    listKeys.mockResolvedValue({
      items: [{ ...createApiKey(), last_used_ip: '203.0.113.10' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = await mountView()

    await toggleColumn(wrapper, 'last_used_ip')

    expect(visibleColumnKeys(wrapper)).toContain('last_used_ip')
    expect(wrapper.get('[data-test="last-used-ip"]').text()).toBe('203.0.113.10')
  })

  it('restores column preferences from localStorage on mount and ignores other versions', async () => {
    localStorage.setItem('user-keys-columns', JSON.stringify({ version: 2, hidden: ['created_at'] }))
    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'status',
      'usage',
      'current_concurrency',
      'last_used_at',
      'last_used_ip',
      'expires_at',
      'actions',
    ])
    wrapper.unmount()

    localStorage.setItem('user-keys-columns', JSON.stringify({ version: 0, hidden: [] }))
    const fresh = await mountView()
    expect(visibleColumnKeys(fresh)).not.toContain('created_at')
    expect(visibleColumnKeys(fresh)).not.toContain('id')
  })

  it('does not include always-visible columns in the toggleable menu', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-testid="column-settings"]').trigger('click')
    await nextTick()

    const toggles = wrapper.findAll('[data-testid^="column-toggle-"]').map((item) => item.attributes('data-testid'))
    expect(toggles).toContain('column-toggle-key')
    expect(toggles).not.toContain('column-toggle-id')
    expect(toggles).toContain('column-toggle-current_concurrency')
    expect(toggles).toContain('column-toggle-last_used_ip')
    expect(toggles).not.toContain('column-toggle-name')
    expect(toggles).not.toContain('column-toggle-actions')
  })

  it('renders the current concurrency value', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-test="current-concurrency"]').text()).toBe('3')
  })

  it('marks current concurrency as sortable', async () => {
    const wrapper = await mountView()
    await toggleColumn(wrapper, 'current_concurrency')

    const currentConcurrencyColumn = visibleColumnMeta(wrapper).find(
      (column) => column.key === 'current_concurrency'
    )
    expect(currentConcurrencyColumn?.sortable).toBe(true)
  })

  it('keeps filters and selected page size when sorting by current concurrency', async () => {
    const wrapper = await mountView()

    await wrapper.get('[data-test="page-size-50"]').trigger('click')
    await flushPromises()

    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('update:modelValue', 'target')
    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
    await flushPromises()

    const statusChip = wrapper.findAllComponents(FilterChip).find((chip) => chip.props('testId') === 'keys-filter-status')!
    await statusChip.vm.$emit('update:modelValue', 'active')
    await flushPromises()

    listKeys.mockClear()

    await wrapper.get('[data-test="sort-current-concurrency"]').trigger('click')
    await flushPromises()

    expect(listKeys).toHaveBeenLastCalledWith(
      1,
      50,
      {
        search: 'target',
        status: 'active',
        sort_by: 'current_concurrency',
        sort_order: 'asc',
      },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })

  // 密钥不再有分组（PR-7a）：创建表单没有厂商步与分组选择，提交只带名字。
  describe('create without groups', () => {
    it('has no provider or group step and submits the key by name only', async () => {
      const wrapper = await mountView()
      await wrapper.get('[data-tour="keys-create-btn"]').trigger('click')
      expect(wrapper.find('[data-tour="key-form-provider"]').exists()).toBe(false)
      expect(wrapper.find('[data-tour="key-form-group"]').exists()).toBe(false)
      expect(wrapper.findAll('input[name="key-provider"]')).toHaveLength(0)

      await wrapper.get('[data-tour="key-form-name"]').setValue('My key')
      vi.mocked(keysAPI.create).mockResolvedValue(createApiKey())
      await wrapper.get('#key-form').trigger('submit')
      await flushPromises()
      expect(keysAPI.create).toHaveBeenCalledOnce()
      expect(vi.mocked(keysAPI.create).mock.calls[0][0]).toBe('My key')
    })
  })
})
