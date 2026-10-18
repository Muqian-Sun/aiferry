import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'

import type { SchedulingOverview } from '@/api/admin/scheduling'
import SchedulingView from '../SchedulingView.vue'

const { getSchedulingOverview } = vi.hoisted(() => ({ getSchedulingOverview: vi.fn() }))
vi.mock('@/api/admin/scheduling', () => ({ getSchedulingOverview }))

const visibility = ref<'visible' | 'hidden'>('visible')
vi.mock('@vueuse/core', async () => {
  const actual = await vi.importActual<typeof import('@vueuse/core')>('@vueuse/core')
  return { ...actual, useDocumentVisibility: () => visibility }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key),
      te: (key: string) => !key.endsWith('.brand_new_reason')
    })
  }
})

const inTenMinutes = () => new Date(Date.now() + 10 * 60_000 + 30_000).toISOString()

function overview(): SchedulingOverview {
  return {
    generated_at: new Date().toISOString(),
    window: '1h',
    data_through: new Date().toISOString(),
    warnings: ['sticky_unavailable'],
    channels: [
      { id: 1, name: 'alpha', platform: 'anthropic', type: 'apikey', status: 'active', priority: 1, concurrency: 10, in_use: 3, waiting: 1, requests: 9, failures: 1 },
      { id: 2, name: 'beta', platform: 'openai', type: 'apikey', status: 'active', priority: 1, concurrency: 10, in_use: 10, waiting: 0, requests: 1, failures: 0 },
      { id: 3, name: 'gamma', platform: 'anthropic', type: 'apikey', status: 'active', priority: 1, concurrency: 10, in_use: 0, waiting: 0, blocked_reason: 'rate_limited', blocked_until: inTenMinutes(), requests: 0, failures: 0 },
      { id: 4, name: 'delta', platform: 'antigravity', type: 'oauth', status: 'active', priority: 2, concurrency: 10, in_use: 0, waiting: 0, requests: 0, failures: 0 }
    ],
    models: [
      {
        entry_id: 7,
        model_id: 'claude-sonnet-5-5',
        display_name: 'Claude Sonnet 5.5',
        vendor: 'anthropic',
        status: 'listed',
        requests: 10,
        failures: 1,
        unrouted: 0,
        lanes: [
          {
            protocol: 'anthropic',
            items: [
              { account_id: 1, state: 'ready', rank: 1, direct: true },
              { account_id: 4, state: 'ready', rank: 2, direct: false },
              { account_id: 2, state: 'queue', direct: false },
              { account_id: 3, state: 'blocked', direct: true, reason: 'rate_limited', until: inTenMinutes() },
              { account_id: 5, state: 'blocked', direct: false, reason: 'brand_new_reason' }
            ]
          },
          {
            protocol: 'chat_completions',
            items: [{ account_id: 2, state: 'ready', rank: 1, direct: true }]
          }
        ],
        channels: [
          { account_id: 1, requests: 9, failures: 1, sticky_sessions: 2 },
          { account_id: 2, requests: 1, failures: 0, sticky_sessions: 0 }
        ]
      }
    ]
  }
}

const mounted: Array<ReturnType<typeof mount>> = []
const mountView = () => {
  const wrapper = mount(SchedulingView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot name="header-actions" /><slot /></div>' },
        RouterLink: { template: '<a><slot /></a>' },
        PlatformIcon: true
      }
    }
  })
  mounted.push(wrapper)
  return wrapper
}

describe('admin SchedulingView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    visibility.value = 'visible'
    getSchedulingOverview.mockReset()
    getSchedulingOverview.mockImplementation(async () => overview())
  })
  afterEach(() => {
    mounted.splice(0).forEach((wrapper) => wrapper.unmount())
    vi.useRealTimers()
  })

  it('draws each lane in scheduler order with ranks, queue and blocked reasons', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getSchedulingOverview).toHaveBeenCalledWith('1h', expect.anything())
    const chips = wrapper.findAll('[data-testid^="scheduling-item-"]')
    expect(chips.map((chip) => chip.attributes('data-account-id'))).toEqual(['1', '4', '2', '3', '5'])
    expect(chips.map((chip) => chip.attributes('data-testid'))).toEqual([
      'scheduling-item-ready',
      'scheduling-item-ready',
      'scheduling-item-queue',
      'scheduling-item-blocked',
      'scheduling-item-blocked'
    ])
    expect(chips[0].text()).toContain('#1')
    expect(chips[0].text()).toContain('admin.scheduling.item.direct')
    expect(chips[0].text()).toContain('admin.scheduling.item.load:{"inUse":3,"max":10}')
    expect(chips[0].text()).toContain('admin.scheduling.item.share:{"count":"9","percent":90}')
    expect(chips[0].text()).toContain('admin.scheduling.item.sticky:{"count":2}')
    expect(chips[1].text()).toContain('#2')
    expect(chips[2].text()).toContain('admin.scheduling.item.queue')
    expect(chips[3].text()).toContain('admin.scheduling.item.recoversIn:{"reason":"admin.scheduling.reasons.rate_limited","duration":"admin.accounts.duration.minutes:{\\"m\\":10}"}')
    expect(chips[4].text()).toContain('admin.scheduling.reasons.other:{"reason":"brand_new_reason"}')
    expect(chips[4].text()).toContain('#5')
    expect(wrapper.find('[data-testid="scheduling-warning"]').text()).toBe('admin.scheduling.warnings.sticky_unavailable')
  })

  it('switches the lane by inbound protocol', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="scheduling-lane-7-chat_completions"]').trigger('click')
    const chips = wrapper.findAll('[data-testid^="scheduling-item-"]')
    expect(chips.map((chip) => chip.attributes('data-account-id'))).toEqual(['2'])
  })

  it('refreshes every 10 seconds while visible, pauses when hidden and reloads on return', async () => {
    mountView()
    await flushPromises()
    expect(getSchedulingOverview).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(10_000)
    expect(getSchedulingOverview).toHaveBeenCalledTimes(2)

    visibility.value = 'hidden'
    await flushPromises()
    await vi.advanceTimersByTimeAsync(30_000)
    expect(getSchedulingOverview).toHaveBeenCalledTimes(2)

    visibility.value = 'visible'
    await flushPromises()
    expect(getSchedulingOverview).toHaveBeenCalledTimes(3)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(getSchedulingOverview).toHaveBeenCalledTimes(4)
  })

  it('reloads with the chosen time range', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="scheduling-window-15m"]').trigger('click')
    await flushPromises()
    expect(getSchedulingOverview).toHaveBeenLastCalledWith('15m', expect.anything())
  })
})
