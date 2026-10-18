import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'

import type { SchedulingOverview, SchedulingTrace, SchedulingTracePage } from '@/api/admin/scheduling'
import SchedulingView from '../SchedulingView.vue'

const { getSchedulingOverview, getSchedulingTraces } = vi.hoisted(() => ({ getSchedulingOverview: vi.fn(), getSchedulingTraces: vi.fn() }))
vi.mock('@/api/admin/scheduling', () => ({ getSchedulingOverview, getSchedulingTraces }))

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
        requests: 14,
        failures: 1,
        unrouted: 2,
        by_protocol: {
          anthropic: { requests: 10, failures: 1, unrouted: 2 },
          chat_completions: { requests: 4, failures: 0, unrouted: 0 }
        },
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
          { protocol: 'chat_completions', items: [{ account_id: 2, state: 'ready', rank: 1, direct: true }] }
        ],
        channels: [
          { account_id: 1, requests: 6, failures: 1, by_protocol: { anthropic: { requests: 6, failures: 1, unrouted: 0 } }, sticky_sessions: 2 },
          { account_id: 2, requests: 6, failures: 0, by_protocol: { anthropic: { requests: 2, failures: 0, unrouted: 0 }, chat_completions: { requests: 4, failures: 0, unrouted: 0 } }, sticky_sessions: 0 }
        ]
      }
    ]
  }
}

function traces(): SchedulingTracePage {
  const switched: SchedulingTrace = {
    id: 101,
    created_at: new Date().toISOString(),
    request_id: 'req-101',
    user_id: 9,
    api_key_id: 8,
    model: 'claude-sonnet-5-5',
    catalog_entry_id: 7,
    inbound_protocol: 'anthropic',
    outcome: 'success',
    status_code: 200,
    final_account_id: 4,
    switch_count: 1,
    sticky_hit: false,
    duration_ms: 1800,
    picks: [
      {
        at_ms: 2,
        layer: 'load',
        account_id: 1,
        sticky: { account_id: 2, outcome: 'miss', reason: 'excluded' },
        candidates: [
          { account_id: 1, decision: 'picked', load_rate: 30 },
          { account_id: 3, decision: 'skipped', reason: 'rate_limited' },
          { account_id: 4, decision: 'eligible', load_rate: 0 }
        ]
      },
      { at_ms: 900, layer: 'load', account_id: 4, candidates: [{ account_id: 1, decision: 'excluded' }, { account_id: 4, decision: 'picked', load_rate: 0 }] }
    ],
    attempts: [{ at_ms: 880, account_id: 1, status_code: 429, kind: 'failover', message: 'rate limited' }]
  }
  const noChannel: SchedulingTrace = {
    ...switched,
    id: 102,
    request_id: 'req-102',
    outcome: 'no_channel',
    status_code: 503,
    final_account_id: undefined,
    switch_count: 0,
    picks: [{ at_ms: 1, error: 'no_available_accounts', candidates: [] }],
    attempts: []
  }
  return { items: [switched, noChannel], total: 2, page: 1, page_size: 20 }
}

const mounted: Array<ReturnType<typeof mount>> = []
const mountView = () => {
  const wrapper = mount(SchedulingView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot name="header-actions" /><slot /></div>' },
        RouterLink: { template: '<a><slot /></a>' },
        PlatformIcon: true,
        teleport: true
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
    getSchedulingTraces.mockReset()
    getSchedulingTraces.mockImplementation(async () => traces())
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
    expect(chips[0].text()).toContain('admin.scheduling.item.sticky:{"count":2}')
    expect(chips[1].text()).toContain('#2')
    expect(chips[2].text()).toContain('admin.scheduling.item.queue')
    expect(chips[3].text()).toContain('admin.scheduling.item.recoversIn:{"reason":"admin.scheduling.reasons.rate_limited","duration":"admin.accounts.duration.minutes:{\\"m\\":10}"}')
    expect(chips[4].text()).toContain('admin.scheduling.reasons.other:{"reason":"brand_new_reason"}')
    expect(chips[4].text()).toContain('#5')
    expect(wrapper.find('[data-testid="scheduling-warning"]').text()).toBe('admin.scheduling.warnings.sticky_unavailable')
  })

  // 卡片上的请求数只算当前协议线：Messages 线上 beta 只有 2 次，占这条线选到渠道的 8 次的 25%。
  it('counts channel requests per inbound protocol lane', async () => {
    const wrapper = mountView()
    await flushPromises()

    const chip = (id: string) => wrapper.find(`[data-account-id="${id}"]`)
    expect(chip('1').text()).toContain('admin.scheduling.item.share:{"count":"6","percent":75}')
    expect(chip('2').text()).toContain('admin.scheduling.item.share:{"count":"2","percent":25}')

    await wrapper.find('[data-testid="scheduling-lane-7-chat_completions"]').trigger('click')
    const chips = wrapper.findAll('[data-testid^="scheduling-item-"]')
    expect(chips.map((c) => c.attributes('data-account-id'))).toEqual(['2'])
    expect(chips[0].text()).toContain('admin.scheduling.item.share:{"count":"4","percent":100}')
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

  it('lists request traces as channel chains and filters by outcome', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(getSchedulingTraces).not.toHaveBeenCalled()

    await wrapper.find('[data-testid="scheduling-tab-traces"]').trigger('click')
    await flushPromises()
    expect(getSchedulingTraces).toHaveBeenLastCalledWith(
      expect.objectContaining({ window: '1h', page: 1, page_size: 20, outcome: '' }),
      expect.anything()
    )
    const chains = wrapper.findAll('[data-testid="scheduling-trace-chain"]').map((chain) => chain.findAll('span').map((span) => span.text()))
    expect(chains[0]).toEqual(['alpha 429', '→', 'delta ✓'])
    expect(chains[1]).toEqual(['admin.scheduling.trace.noChannel'])

    await wrapper.find('[data-testid="scheduling-trace-outcome-switched"]').trigger('click')
    await flushPromises()
    expect(getSchedulingTraces).toHaveBeenLastCalledWith(expect.objectContaining({ outcome: 'switched', page: 1 }), expect.anything())

    // 第一页、没开详情时跟着页面刷新
    const before = getSchedulingTraces.mock.calls.length
    await vi.advanceTimersByTimeAsync(10_000)
    expect(getSchedulingTraces.mock.calls.length).toBe(before + 1)
  })

  it('opens a trace to show every pick with candidate decisions and failed attempts', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="scheduling-tab-traces"]').trigger('click')
    await flushPromises()

    await wrapper.findAll('[data-testid="scheduling-trace-row"]')[0].trigger('click')
    const drawer = wrapper.find('[data-testid="scheduling-trace-drawer"]')
    expect(drawer.exists()).toBe(true)
    const picks = drawer.findAll('[data-testid="scheduling-trace-pick"]')
    expect(picks).toHaveLength(2)
    expect(picks[0].text()).toContain('admin.scheduling.trace.sticky.miss:{"channel":"beta","reason":"admin.scheduling.reasons.excluded"}')
    const skipped = picks[0].find('[data-decision="skipped"]')
    expect(skipped.text()).toContain('gamma')
    expect(skipped.text()).toContain('admin.scheduling.reasons.rate_limited')
    expect(picks[0].find('[data-decision="picked"]').text()).toContain('30%')
    expect(picks[1].find('[data-decision="excluded"]').text()).toContain('alpha')
    expect(drawer.find('[data-testid="scheduling-trace-attempts"]').text()).toContain('429')

    // 开着详情时不跟着刷新，免得内容跳
    const before = getSchedulingTraces.mock.calls.length
    await vi.advanceTimersByTimeAsync(10_000)
    expect(getSchedulingTraces.mock.calls.length).toBe(before)
  })
})
