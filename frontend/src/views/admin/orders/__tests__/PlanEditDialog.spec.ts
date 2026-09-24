import { describe, expect, it, vi, beforeEach } from 'vitest'
import { defineComponent } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'

import PlanEditDialog from '../PlanEditDialog.vue'
import type { SubscriptionPlan } from '@/types/payment'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === 'payment.admin.subscriptionCnyPayPreview') return `preview ${params?.amount}`
      if (key === 'payment.admin.subscriptionCnyPayPreviewWithFee') return `fee ${params?.feeRate} ${params?.total}`
      return key
    },
  }),
}))

const showError = vi.fn()
const showSuccess = vi.fn()
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

const createPlan = vi.fn()
const updatePlan = vi.fn()
vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    createPlan: (...args: unknown[]) => createPlan(...args),
    updatePlan: (...args: unknown[]) => updatePlan(...args),
  },
}))

// 目录条目：只有 listed 的会进套餐模型集候选
const listEntries = vi.fn()
vi.mock('@/api/admin', () => ({
  adminAPI: {
    modelCatalog: { listEntries: () => listEntries() },
  },
}))

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: Boolean, title: String, width: String },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

function mountDialog({
  plan = null,
  paymentConfig = null,
}: {
  plan?: SubscriptionPlan | null
  paymentConfig?: Record<string, unknown> | null
} = {}) {
  return mount(PlanEditDialog, {
    props: { show: true, plan, paymentConfig },
    global: {
      stubs: { BaseDialog: BaseDialogStub, Select: true, Icon: true },
    },
  })
}

async function fillRequired(wrapper: ReturnType<typeof mountDialog>) {
  await wrapper.find('input[type="text"]').setValue('Pro')
  await wrapper.find('textarea').setValue('desc')
  await wrapper.find('input[type="number"]').setValue('9.99')
}

describe('PlanEditDialog', () => {
  beforeEach(() => {
    showError.mockReset()
    createPlan.mockReset().mockResolvedValue({})
    updatePlan.mockReset().mockResolvedValue({})
    listEntries.mockReset().mockResolvedValue([
      { id: 199, model_id: 'gpt-5.6', display_name: 'GPT 5.6', status: 'listed' },
      { id: 27, model_id: 'claude-sonnet-4-5', display_name: '', status: 'listed' },
      { id: 300, model_id: 'hidden-model', display_name: 'Hidden', status: 'unlisted' },
    ])
  })

  it('shows CNY channel charge using the configured USD rate and fee', async () => {
    const wrapper = mountDialog({
      paymentConfig: { usd_to_cny_rate: 7.15, recharge_fee_rate: 2.5 },
    })
    await wrapper.find('input[type="number"]').setValue('9.99')
    expect(wrapper.text()).toContain('preview')
    expect(wrapper.text()).toContain('¥71.43')
    expect(wrapper.text()).toContain('fee 2.5')
    expect(wrapper.text()).toContain('¥73.22')
  })

  it('hides the preview when the USD rate is not configured', async () => {
    const wrapper = mountDialog({
      paymentConfig: { usd_to_cny_rate: 0, recharge_fee_rate: 2.5 },
    })
    await wrapper.find('input[type="number"]').setValue('9.99')
    expect(wrapper.text()).not.toContain('preview')
    expect(wrapper.text()).not.toContain('¥71.43')
  })

  it('lists only listed catalog entries as plan model candidates', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.find('[data-testid="plan-models-toggle"]').trigger('click')
    const labels = wrapper.findAll('[data-testid="plan-model-option"]').map(o => o.text())
    expect(labels).toEqual(['GPT 5.6 (gpt-5.6)', 'claude-sonnet-4-5'])
    expect(labels.join(' ')).not.toContain('hidden-model')
  })

  it('refuses to save without at least one model and does not call the API', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    await fillRequired(wrapper)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('payment.admin.modelsRequired')
    expect(createPlan).not.toHaveBeenCalled()
  })

  it('sends entry_ids and limits (empty limit → -1) in the create payload', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    await fillRequired(wrapper)
    await wrapper.find('[data-testid="plan-models-toggle"]').trigger('click')
    await wrapper.findAll('[data-testid="plan-model-option"]')[0].trigger('click')
    // number 输入顺序：price, original_price, validity_days, daily, weekly, monthly, sort_order
    await wrapper.findAll('input[type="number"]')[3].setValue('1.5')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(createPlan).toHaveBeenCalledTimes(1)
    const payload = createPlan.mock.calls[0][0] as Record<string, unknown>
    expect(payload.entry_ids).toEqual([199])
    expect(payload.daily_limit_usd).toBe(1.5)
    expect(payload.weekly_limit_usd).toBe(-1)
    expect(payload.monthly_limit_usd).toBe(-1)
    expect(payload).not.toHaveProperty('group_id')
  })

  it('prefills models and limits from an existing plan and sends updatePlan', async () => {
    const plan: SubscriptionPlan = {
      id: 7, name: 'Pro', description: 'd', price: 9.9, validity_days: 30, validity_unit: 'days', features: [],
      for_sale: true, sort_order: 0, daily_limit_usd: 2, weekly_limit_usd: null, monthly_limit_usd: null,
      entry_ids: [27], models: [{ entry_id: 27, model_id: 'claude-sonnet-4-5', display_name: '' }],
    }
    const wrapper = mountDialog({ plan })
    await flushPromises()
    expect(wrapper.find('[data-testid="plan-models-toggle"]').text()).toContain('claude-sonnet-4-5')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(updatePlan).toHaveBeenCalledTimes(1)
    const [id, payload] = updatePlan.mock.calls[0] as [number, Record<string, unknown>]
    expect(id).toBe(7)
    expect(payload.entry_ids).toEqual([27])
    expect(payload.daily_limit_usd).toBe(2)
  })
})
