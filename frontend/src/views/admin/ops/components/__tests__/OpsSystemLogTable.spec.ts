import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import OpsSystemLogTable from '../OpsSystemLogTable.vue'
import OpsRuntimeLogConfigDialog from '../OpsRuntimeLogConfigDialog.vue'
import enLocale from '@/i18n/locales/en'
import zhLocale from '@/i18n/locales/zh'

const mockListSystemLogs = vi.fn()
const mockCleanupSystemLogs = vi.fn()
const mockGetSystemLogSinkHealth = vi.fn()
const mockGetRuntimeLogConfig = vi.fn()

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    listSystemLogs: (...args: any[]) => mockListSystemLogs(...args),
    cleanupSystemLogs: (...args: any[]) => mockCleanupSystemLogs(...args),
    getSystemLogSinkHealth: (...args: any[]) => mockGetSystemLogSinkHealth(...args),
    getRuntimeLogConfig: (...args: any[]) => mockGetRuntimeLogConfig(...args),
  },
}))

// 用户 / 密钥 / 渠道筛选的 EntityPicker 从 @/api/admin 取搜索接口；本用例不走它
vi.mock('@/api/admin', () => ({ adminAPI: {} }))

vi.mock('@/stores', () => ({
  useAppStore: () => ({}),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const SelectStub = defineComponent({
  name: 'SelectControlStub',
  props: {
    modelValue: {
      type: [String, Number],
      default: '',
    },
  },
  emits: ['update:modelValue'],
  template: '<div class="select-stub" />',
})

const PaginationStub = defineComponent({
  name: 'PaginationStub',
  template: '<div class="pagination-stub" />',
})

// 清理改成页面内确认（2026-10-04）：打开时给一个「确认」按钮
const ConfirmDialogStub = defineComponent({
  name: 'ConfirmDialogStub',
  props: { show: Boolean },
  emits: ['confirm', 'cancel'],
  template: '<button v-if="show" class="confirm-stub" @click="$emit(\'confirm\')">confirm</button>',
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialogStub',
  props: { show: Boolean },
  template: '<div v-if="show"><slot /></div>',
})

const runtimeConfig = {
  level: 'info',
  persist_access_logs: false,
  enable_sampling: false,
  sampling_initial: 100,
  sampling_thereafter: 100,
  caller: true,
  stacktrace_level: 'error',
  retention_days: 30,
}

const sinkHealth = {
  queue_depth: 0,
  queue_capacity: 5000,
  dropped_count: 0,
  write_failed_count: 0,
  written_count: 1,
  avg_write_delay_ms: 0,
}

describe('OpsSystemLogTable host support', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    mockListSystemLogs.mockResolvedValue({
      items: [
        {
          id: 1,
          created_at: '2026-07-14T00:10:01Z',
          host: 'api-node-1',
          level: 'warn',
          component: 'app',
          message: 'request failed',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })
    mockCleanupSystemLogs.mockResolvedValue({ deleted: 1 })
    mockGetSystemLogSinkHealth.mockResolvedValue(sinkHealth)
    mockGetRuntimeLogConfig.mockResolvedValue(runtimeConfig)
  })

  it('renders the host and sends it with list and cleanup filters', async () => {
    const wrapper = mount(OpsSystemLogTable, {
      props: { timeParams: { time_range: '1h' } },
      global: {
        stubs: {
          Select: SelectStub,
          Pagination: PaginationStub,
          ConfirmDialog: ConfirmDialogStub,
          OpsRuntimeLogConfigDialog: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('api-node-1')

    // 主机等筛选收在「更多筛选」里
    await wrapper.findAll('button').find((button) => button.text().startsWith('admin.ops.page.logs.moreFilters'))!.trigger('click')

    const hostLabel = wrapper.findAll('label').find((label) => label.text().includes('admin.ops.systemLogs.host'))
    expect(hostLabel).toBeDefined()
    await hostLabel!.find('input').setValue(' api-node-2 ')

    const searchButton = wrapper.findAll('button').find((button) => button.text() === 'admin.ops.systemLogs.search')
    expect(searchButton).toBeDefined()
    await searchButton!.trigger('click')
    await flushPromises()

    expect(mockListSystemLogs).toHaveBeenLastCalledWith(expect.objectContaining({ host: 'api-node-2' }))

    const cleanupButton = wrapper.findAll('button').find((button) => button.text() === 'admin.ops.systemLogs.cleanCurrentFilters')
    expect(cleanupButton).toBeDefined()
    await cleanupButton!.trigger('click')
    await wrapper.find('.confirm-stub').trigger('click')
    await flushPromises()

    // 页头是相对时间范围时，清理也带上换算出的起止时间，不会删到范围外
    expect(mockCleanupSystemLogs).toHaveBeenCalledWith(
      expect.objectContaining({ host: 'api-node-2', start_time: expect.any(String), end_time: expect.any(String) }),
    )
  })

  // 运行时日志配置挪进「日志配置」弹窗（2026-10-04）
  it('keeps database access-log persistence opt-in', async () => {
    const wrapper = mount(OpsRuntimeLogConfigDialog, {
      props: { show: true },
      global: {
        stubs: {
          Select: SelectStub,
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
        },
      },
    })
    await flushPromises()

    const label = wrapper.findAll('label').find((item) => item.text().includes('admin.ops.systemLogs.persistAccessLogs'))
    expect(label).toBeDefined()
    expect((label!.find('input').element as HTMLInputElement).checked).toBe(false)
  })

  it.each([
    ['zh', zhLocale],
    ['en', enLocale],
  ])('defines the Host translation for %s', (_name, locale) => {
    expect(locale.admin.ops.systemLogs.host).toBe('Host')
  })
})
