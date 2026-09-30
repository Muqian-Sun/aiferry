import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import CatalogEntryDiagnosisModal from '../CatalogEntryDiagnosisModal.vue'

const { diagnose } = vi.hoisted(() => ({ diagnose: vi.fn() }))

vi.mock('@/api/admin/modelCatalog', () => ({ default: { diagnose } }))
vi.mock('@/api', () => ({}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => (params?.model ? `${key}:${params.model}` : key),
    te: (key: string) => key.endsWith('.rate_limited')
  })
}))

const serves = (anthropic: boolean, chat: boolean, responses: boolean, gemini: boolean) => ({
  anthropic, chat_completions: chat, responses, gemini
})

function mountModal(entryId: number | null = 199) {
  return mount(CatalogEntryDiagnosisModal, {
    props: { show: true, entryId, modelId: 'gpt-5.6' },
    global: {
      stubs: {
        BaseDialog: { props: ['show', 'title'], template: '<div data-test="dialog" :data-title="title"><slot /><slot name="footer" /></div>' },
        Teleport: true
      }
    }
  })
}

describe('CatalogEntryDiagnosisModal', () => {
  beforeEach(() => {
    diagnose.mockReset().mockResolvedValue({
      entry_id: 199,
      accounts: [
        { id: 3, name: 'K1', platform: 'openai', type: 'apikey', vendor: '', status: 'active', schedulable: true, serves: serves(true, true, true, false) },
        { id: 5, name: 'K4', platform: 'openai', type: 'apikey', vendor: 'openai', status: 'active', schedulable: false, blocked_reason: 'rate_limited', serves: serves(true, true, true, false) },
        { id: 9, name: 'unknown', platform: 'anthropic', type: 'oauth', vendor: 'anthropic', status: 'active', schedulable: false, blocked_reason: 'mystery', serves: serves(false, false, false, false) }
      ]
    })
  })

  it('lists bound resources with schedulability and the four inbound protocols', async () => {
    const wrapper = mountModal()
    await flushPromises()

    expect(diagnose).toHaveBeenCalledWith(199)
    expect(wrapper.get('[data-test="dialog"]').attributes('data-title')).toBe('admin.modelCatalog.diagnosis.title:gpt-5.6')

    const rows = wrapper.findAll('[data-testid="model-catalog-diagnosis-row"]')
    expect(rows).toHaveLength(3)

    expect(rows[0].text()).toContain('K1')
    expect(rows[0].find('[data-testid="model-catalog-diagnosis-schedulable"]').exists()).toBe(true)
    expect(rows[0].get('[data-testid="model-catalog-diagnosis-serves-responses"]').attributes('data-serves')).toBe('true')
    expect(rows[0].get('[data-testid="model-catalog-diagnosis-serves-gemini"]').attributes('data-serves')).toBe('false')

    expect(rows[1].get('[data-testid="model-catalog-diagnosis-blocked"]').text()).toContain('admin.modelCatalog.diagnosis.reasons.rate_limited')

    // 未知原因原样显示，不吞掉。
    expect(rows[2].get('[data-testid="model-catalog-diagnosis-blocked"]').text()).toContain('mystery')
    expect(rows[2].get('[data-testid="model-catalog-diagnosis-serves-anthropic"]').attributes('data-serves')).toBe('false')
  })

  it('shows the empty state when nothing is bound and does not fetch without an entry', async () => {
    diagnose.mockResolvedValue({ entry_id: 199, accounts: [] })
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-diagnosis-empty"]').exists()).toBe(true)

    diagnose.mockClear()
    mountModal(null)
    await flushPromises()
    expect(diagnose).not.toHaveBeenCalled()
  })

  it('surfaces a fetch failure instead of an empty table', async () => {
    diagnose.mockRejectedValue(new Error('boom'))
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.find('[data-testid="model-catalog-diagnosis-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="model-catalog-diagnosis-empty"]').exists()).toBe(false)
  })
})
