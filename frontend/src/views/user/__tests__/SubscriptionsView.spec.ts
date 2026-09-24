/**
 * 订阅页签：我的订阅一行一个（role=meter 进度），「续费」把分组交给嵌入的支付引擎；支付关闭时不渲染购买区与续费按钮。
 */
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { UserSubscription } from '@/types'

const { getMySubscriptions, startRenewal, flags } = vi.hoisted(() => ({
  getMySubscriptions: vi.fn(),
  startRenewal: vi.fn(),
  flags: { payment: true, subscription: true, affiliate: false }
}))

vi.mock('@/api/subscriptions', () => ({ default: { getMySubscriptions } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), cachedPublicSettings: null }) }))
vi.mock('@/views/user/billing/useBillingFlags', async () => {
  const { computed } = await import('vue')
  return { useBillingFlags: () => computed(() => flags) }
})
vi.mock('@/views/user/PaymentView.vue', async () => {
  const { h } = await import('vue')
  return {
    default: {
      name: 'PaymentView',
      props: ['mode', 'renewPlanId'],
      setup(_props: unknown, { expose }: { expose: (exposed: Record<string, unknown>) => void }) {
        expose({ startRenewal })
        return () => h('div', { 'data-testid': 'payment-engine' })
      }
    }
  }
})
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import SubscriptionsView from '@/views/user/SubscriptionsView.vue'

function subscription(overrides: Partial<UserSubscription> = {}): UserSubscription {
  return {
    id: 1,
    user_id: 5,
    plan_id: 42,
    status: 'active',
    starts_at: '2026-09-01T00:00:00Z',
    daily_usage_usd: 3,
    weekly_usage_usd: 0,
    monthly_usage_usd: 0,
    daily_window_start: null,
    weekly_window_start: null,
    monthly_window_start: null,
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
    expires_at: '2099-01-01T00:00:00Z',
    plan: { id: 42, name: 'Pro', daily_limit_usd: 10, weekly_limit_usd: null, monthly_limit_usd: null, models: [{ entry_id: 199, model_id: 'gpt-5.6', display_name: 'GPT 5.6' }, { entry_id: 27, model_id: 'claude-sonnet-4-5', display_name: '' }] },
    api_key: { id: 7, name: 'Pro', key_masked: 'sk-abc****wxyz' },
    ...overrides
  }
}

async function mountView() {
  const wrapper = mount(SubscriptionsView, {
    global: { stubs: { Icon: true } },
    attachTo: document.body
  })
  await flushPromises()
  return wrapper
}

describe('SubscriptionsView', () => {
  beforeEach(() => {
    getMySubscriptions.mockReset().mockResolvedValue([subscription()])
    startRenewal.mockReset()
    flags.payment = true
  })

  it('renders each subscription as a row with an accessible usage meter', async () => {
    const wrapper = await mountView()
    const rows = wrapper.findAll('[data-testid="subscription-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain('Pro')
    const meter = rows[0].get('[role="meter"]')
    expect(meter.attributes('aria-valuemax')).toBe('10')
    expect(meter.attributes('aria-valuenow')).toBe('3')
    expect(wrapper.get('[data-testid="subscription-key"]').text()).toContain('sk-abc****wxyz')
    // 周、月两块固定出现（这个套餐没设，写无限制）
    const limits = rows[0].get('[data-testid="subscription-limits"]').text()
    expect(limits).toContain('payment.planCard.weeklyLimit')
    expect(limits).toContain('payment.planCard.monthlyLimit')
    expect(limits).toContain('payment.planCard.unlimited')
  })

  it('hides the plan list while a subscription is active and renews from the panel', async () => {
    const wrapper = await mountView()
    expect(wrapper.find('[data-testid="payment-engine"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="subscription-models"]').text()).toContain('GPT 5.6 / claude-sonnet-4-5')
    await wrapper.get('[data-testid="subscription-renew"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="payment-engine"]').exists()).toBe(true)
  })

  it('renders neither the purchase section nor renew buttons when payment is disabled', async () => {
    flags.payment = false
    const wrapper = await mountView()
    expect(wrapper.find('[data-testid="payment-engine"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="subscription-row"] button').exists()).toBe(false)
    // 没有套餐卡片时，模型集留在面板里
    expect(wrapper.get('[data-testid="subscription-models"]').text()).toContain('GPT 5.6 / claude-sonnet-4-5')
  })

  it('shows the empty state when the user has no subscriptions', async () => {
    getMySubscriptions.mockResolvedValue([])
    const wrapper = await mountView()
    expect(wrapper.find('[data-testid="status-empty"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="subscription-row"]').exists()).toBe(false)
    // 没有订阅时列出可选套餐
    expect(wrapper.find('[data-testid="payment-engine"]').exists()).toBe(true)
  })
})
