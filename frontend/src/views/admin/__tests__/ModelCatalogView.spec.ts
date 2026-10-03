import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ModelCatalogView from '../ModelCatalogView.vue'
import type { ModelCatalogEntry } from '@/api/admin/modelCatalog'

const { listEntries, createEntry, updateEntry, deleteEntry, seed, priceLookup } = vi.hoisted(() => ({
  priceLookup: vi.fn(),
  listEntries: vi.fn(),
  createEntry: vi.fn(),
  updateEntry: vi.fn(),
  deleteEntry: vi.fn(),
  seed: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    modelCatalog: { listEntries, createEntry, updateEntry, deleteEntry, seed, priceLookup }
  }
}))


vi.mock('@/stores/app', () => ({
  useAppStore: () => ({})
}))

vi.mock('@/api', () => ({}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key
  }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'zh' } } })
}))

function entry(overrides: Partial<ModelCatalogEntry> = {}): ModelCatalogEntry {
  return {
    id: 1,
    model_id: 'claude-opus-4-6',
    display_name: 'Opus',
    vendor: 'anthropic',
    protocols: ['anthropic'],
    billing_mode: 'token',
    status: 'listed',
    managed_by: 'seed',
    input_price: 15,
    output_price: 75,
    cache_write_price: null,
    cache_write_1h_price: null,
    cache_read_price: null,
    image_input_price: null,
    image_output_price: null,
    image_cache_read_price: null,
    per_request_price: null,
    search_price_per_call: null,
    max_reasoning_effort_multiplier: null,
    intervals: [],
    aliases: [],
    bindings: [],
    created_at: '2026-09-18T00:00:00Z',
    updated_at: '2026-09-18T00:00:00Z',
    ...overrides
  }
}

function mountView() {
  return mount(ModelCatalogView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot name="header-actions" /><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="summary" /><slot name="filters" /><slot name="table" /><slot name="bulk" /></div>' },
        // 弹出层（页头工具菜单、行尾「⋯」、筛选标签）桩成内联渲染，菜单项直接可点
        PopoverMenu: { template: '<div><slot name="trigger" :open="false" /><slot :close="() => {}" /></div>' },
        RouterLink: true,
        DataTable: {
          props: ['data'],
          template:
            '<div><slot name="cell-model_id" :row="data[0]" v-if="data[0]" /><slot name="cell-resources" :row="data[0]" v-if="data[0]" /><slot name="cell-actions" :row="data[0]" v-if="data[0]" /></div>'
        },
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: { props: ['show'], template: '<div v-if="show" />' },
        EmptyState: true,
        Icon: true,
        PlatformTypeBadge: true,
        CatalogEntryDiagnosisModal: {
          props: ['show', 'entryId', 'modelId'],
          template: '<div data-testid="diagnosis-stub" :data-show="show" :data-entry-id="entryId ?? \'\'" :data-model-id="modelId" />'
        }
      }
    }
  })
}

beforeEach(() => {
  listEntries.mockReset().mockResolvedValue([entry()])
  createEntry.mockReset().mockResolvedValue(entry())
  updateEntry.mockReset().mockResolvedValue(entry())
  deleteEntry.mockReset().mockResolvedValue(undefined)
  seed.mockReset().mockResolvedValue({ inserted: 10, refreshed: 2, skipped_admin: 1, skipped_invalid: 0, failed: 0 })
  priceLookup.mockReset().mockResolvedValue(null)
  vi.useRealTimers()
})

describe('ModelCatalogView', () => {
  it('lists catalog entries from the admin API', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(listEntries).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('claude-opus-4-6')
  })

  // 新建 / 编辑是弹窗（2026-10-03）：新建两步（模型 → 定价与渠道），编辑一步
  it('opens the model dialogs for create and edit', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-create"]').trigger('click')
    expect(wrapper.find('[data-testid="model-create-steps"]').exists()).toBe(true)
    await wrapper.get('[data-testid="row-action-edit"]').trigger('click')
    expect(wrapper.get<HTMLInputElement>('#model-edit-form [data-testid="model-catalog-model-id"]').element.value).toBe('claude-opus-4-6')
  })

  it('shows the resource count and flags listed entries without resources', async () => {
    listEntries.mockResolvedValue([entry({ status: 'listed', bindings: [] })])
    let wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-no-resources"]').exists()).toBe(true)

    listEntries.mockResolvedValue([
      entry({ status: 'listed', bindings: [{ entry_id: 1, account_id: 7 }, { entry_id: 1, account_id: 8 }] })
    ])
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-no-resources"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="model-catalog-resource-count"]').text()).toBe('2')

    listEntries.mockResolvedValue([entry({ status: 'unlisted', bindings: [] })])
    wrapper = mountView()
    await flushPromises()
    // 默认只看已上架：先切到「全部」
    await wrapper.findAllComponents({ name: 'FilterChip' }).find((c) => c.props('testId') === 'model-catalog-filter-status')!.vm.$emit('update:modelValue', '')
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-no-resources"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="model-catalog-resource-count"]').text()).toBe('0')
  })

  it('seeds the catalog and reloads the list', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="model-catalog-seed"]').trigger('click')
    await flushPromises()
    expect(seed).toHaveBeenCalledTimes(1)
    expect(listEntries).toHaveBeenCalledTimes(2)
  })

  // 单条写库失败不拖垮整批，但不能静默：失败数和原因要作为错误提示摆出来。
})
