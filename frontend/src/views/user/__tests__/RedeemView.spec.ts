import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import RedeemView from '../RedeemView.vue'

const { redeem, getHistory, refreshUser, fetchActiveSubscriptions } = vi.hoisted(() => ({
  redeem: vi.fn(),
  getHistory: vi.fn(),
  refreshUser: vi.fn(),
  fetchActiveSubscriptions: vi.fn(),
}))

vi.mock('@/api', () => ({
  redeemAPI: { redeem, getHistory },
  authAPI: { getPublicSettings: vi.fn().mockResolvedValue({}) },
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { balance: 10, concurrency: 2 }, refreshUser }),
}))
vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({ fetchActiveSubscriptions }),
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({}),
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

async function submitCode() {
  const wrapper = mount(RedeemView, {
    global: { stubs: { SiteShell: { template: '<div><slot /></div>' }, Icon: true } },
  })
  await flushPromises()
  await wrapper.get('input#code').setValue(' REDEEM-CODE ')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  return wrapper
}

describe('RedeemView refresh after redemption', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    redeem.mockResolvedValue({ id: 9, code: 'REDEEM-CODE', type: 'balance', value: 20, status: 'used', used_at: '2026-03-08T00:00:00Z', created_at: '2026-03-01T00:00:00Z' })
    getHistory.mockResolvedValue([])
    refreshUser.mockResolvedValue({ balance: 30, concurrency: 2 })
    fetchActiveSubscriptions.mockResolvedValue([])
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it.each(['balance', 'concurrency', 'subscription'])(
    'keeps a successful %s redemption when profile refresh fails', async (type) => {
      redeem.mockResolvedValue({ id: 9, code: 'REDEEM-CODE', type, value: 20, status: 'used', used_at: '2026-03-08T00:00:00Z', created_at: '2026-03-01T00:00:00Z', validity_days: type === 'subscription' ? 10 : undefined, plan: type === 'subscription' ? { id: 3, name: 'E2E Pro' } : undefined })
      refreshUser.mockRejectedValue({ status: 503, message: 'Service unavailable' })
      getHistory.mockResolvedValueOnce([]).mockResolvedValueOnce([{
        id: 1, code: 'REDEEM-CODE', type, value: 20, used_at: '2026-03-08T00:00:00Z',
      }])

      const wrapper = await submitCode()

      expect(redeem).toHaveBeenCalledWith('REDEEM-CODE')
      expect(wrapper.text()).toContain('redeem.redeemSuccess')
      if (type === 'subscription') {
        expect(wrapper.get('[data-testid="redeem-plan-name"]').text()).toContain('E2E Pro')
      }
      expect(wrapper.text()).not.toContain('redeem.failedToRedeem')
      expect((wrapper.get('input#code').element as HTMLInputElement).value).toBe('')
      expect((wrapper.get('input#code').element as HTMLInputElement).disabled).toBe(false)
      expect(getHistory).toHaveBeenCalledTimes(2)
      expect(wrapper.text()).toContain('REDEEM-C…')
      if (type === 'subscription') {
        expect(fetchActiveSubscriptions).toHaveBeenCalledWith(true)
      } else {
        expect(fetchActiveSubscriptions).not.toHaveBeenCalled()
      }
      wrapper.unmount()
    }
  )

  it('finishes normally without a warning when profile refresh succeeds', async () => {
    const wrapper = await submitCode()

    expect(refreshUser).toHaveBeenCalledOnce()
    expect((wrapper.get('input#code').element as HTMLInputElement).value).toBe('')
    wrapper.unmount()
  })

  it('preserves the existing subscription refresh warning after successful redemption', async () => {
    redeem.mockResolvedValue({ id: 9, code: 'REDEEM-CODE', type: 'subscription', value: 20, status: 'used', used_at: '2026-03-08T00:00:00Z', created_at: '2026-03-01T00:00:00Z', plan_id: 3, validity_days: 10 })
    fetchActiveSubscriptions.mockRejectedValue(new Error('Network Error'))
    const wrapper = await submitCode()

    expect(getHistory).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('keeps the code and reports failure when the redemption request itself fails', async () => {
    redeem.mockRejectedValue({ status: 400, code: 400, message: 'Invalid code' })
    const wrapper = await submitCode()

    // 后端 message 不上屏：显示前端的兜底文案
    expect(wrapper.text()).toContain('redeem.failedToRedeem')
    expect(wrapper.text()).not.toContain('Invalid code')
    expect(wrapper.text()).not.toContain('redeem.redeemSuccess')
    expect((wrapper.get('input#code').element as HTMLInputElement).value).toBe(' REDEEM-CODE ')
    expect(refreshUser).not.toHaveBeenCalled()
    expect(fetchActiveSubscriptions).not.toHaveBeenCalled()
    expect(getHistory).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
})
