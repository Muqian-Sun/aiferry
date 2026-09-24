import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ProxiesView from '../ProxiesView.vue'

const { listProxies, getAllWithCount } = vi.hoisted(() => ({
  listProxies: vi.fn(),
  getAllWithCount: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { proxies: { list: listProxies, getAllWithCount } }
}))
// A4：ListToolbar.vue 只有模板、没有组件名，shallowMount 按名字桩不到；换成透传两个插槽的实现
vi.mock('@/components/admin/list/ListToolbar.vue', () => ({
  default: { name: 'ListToolbar', template: '<div><slot /><slot name="end" /></div>' }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const mountView = () => shallowMount(ProxiesView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: {
        template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
      },
      DataTable: {
        props: ['data'],
        template: '<div data-test="rows">{{ data.map(row => row.name).join(",") }}</div>'
      },
      Pagination: {
        props: ['page'],
        emits: ['update:page'],
        template: '<button data-test="page" @click="$emit(\'update:page\', 2)">{{ page }}</button>'
      },
      // A4：工具行换成 ListToolbar + 筛选标签（选项里没有「全部」，清空 = 空串）
      ListToolbar: false,
      FilterChip: {
        props: ['modelValue', 'options', 'testId'],
        emits: ['update:modelValue', 'change'],
        template: `<select :data-filter="testId" :value="modelValue"
          @change="$emit('update:modelValue', $event.target.value); $emit('change', $event.target.value)">
          <option value=""></option>
          <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>`
      }
    }
  }
})

let wrapper: ReturnType<typeof mountView>

beforeEach(() => {
  vi.clearAllMocks()
  getAllWithCount.mockResolvedValue([])
  listProxies.mockResolvedValue({ items: [], total: 100, pages: 5 })
})

afterEach(() => wrapper?.unmount())

describe('proxy list filter pagination', () => {
  it.each([
    ['protocol', 'filter-protocol', 'socks5', ''],
    ['protocol', 'filter-protocol', '', 'http'],
    ['status', 'filter-status', 'expired', ''],
    ['status', 'filter-status', '', 'active']
  ])('starts at page one when changing %s through %s to "%s"', async (field, testId, value, initial) => {
    wrapper = mountView()
    await flushPromises()
    const filter = wrapper.get(`select[data-filter="${testId}"]`)
    if (initial) {
      await filter.setValue(initial)
      await flushPromises()
    }
    await wrapper.get('[data-test="page"]').trigger('click')
    await flushPromises()
    expect(listProxies.mock.lastCall?.[0]).toBe(2)
    const [, pageSize, previousFilters] = listProxies.mock.lastCall!
    listProxies.mockImplementation(async (page) => ({
      items: page === 1 ? [{ id: 1, name: 'matching-proxy' }] : [],
      total: 1,
      pages: 1
    }))

    await filter.setValue(value)
    await flushPromises()

    expect(listProxies).toHaveBeenLastCalledWith(
      1, pageSize, { ...previousFilters, [field]: value || undefined },
      { signal: expect.any(AbortSignal) }
    )
    expect(wrapper.get('[data-test="page"]').text()).toBe('1')
    expect(wrapper.get('[data-test="rows"]').text()).toBe('matching-proxy')
  })

  it('keeps the current page when refreshing the same filter', async () => {
    wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="page"]').trigger('click')
    await flushPromises()

    await wrapper.get('button[title="common.refresh"]').trigger('click')
    await flushPromises()

    expect(listProxies.mock.lastCall?.[0]).toBe(2)
    expect(wrapper.get('[data-test="page"]').text()).toBe('2')
  })
})
