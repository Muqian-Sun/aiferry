import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UsageProgressBar from '../UsageProgressBar.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('UsageProgressBar', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-17T00:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('showNowWhenIdle=true 且利用率为 0 时显示“现在”', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '5h',
        utilization: 0,
        resetsAt: '2026-03-17T02:30:00Z',
        showNowWhenIdle: true
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.usageWindow.resetNow')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.resetsIn')
  })

  it('showNowWhenIdle=true 但利用率大于 0 时显示倒计时', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '7d',
        utilization: 12,
        resetsAt: '2026-03-17T02:30:00Z',
        showNowWhenIdle: true
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.usageWindow.resetsIn')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.resetNow')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.resetPending')
  })

  it('showNowWhenIdle=false 时保持原有倒计时行为', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '1d',
        utilization: 0,
        resetsAt: '2026-03-17T02:30:00Z',
        showNowWhenIdle: false
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.usageWindow.resetsIn')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.resetNow')
  })

  it('resetsAt 已过期且利用率大于 0 时显示「待刷新」', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '5h',
        utilization: 53,
        // 早于 fake system time 2026-03-17T00:00:00Z
        resetsAt: '2026-03-16T22:00:00Z'
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.usageWindow.resetPending')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.resetNow')
  })

  it('resetsAt 已过期且利用率为 0 时仍显示「现在」', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '5h',
        utilization: 0,
        resetsAt: '2026-03-16T22:00:00Z'
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.usageWindow.resetNow')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.resetPending')
  })

  it('默认利用率模式仍把超限显示为满格红色', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '5h',
        utilization: 120
      }
    })

    expect(wrapper.text()).toContain('120%')
    expect(wrapper.get('.h-1\\.5 > div').attributes('style')).toContain('width: 100%')
    expect(wrapper.get('.h-1\\.5 > div').classes()).toContain('bg-af-danger')
  })

  it('默认利用率模式按 75/90 阈值提前预警分级', () => {
    const mountAt = (utilization: number) =>
      mount(UsageProgressBar, {
        props: { label: '5h', utilization }
      })

    // 条形配色：74 墨色 / 75 与 89 黄 / 90 红
    expect(mountAt(74).get('.h-1\\.5 > div').classes()).toContain('bg-af-ink-3')
    expect(mountAt(75).get('.h-1\\.5 > div').classes()).toContain('bg-af-warning')
    expect(mountAt(89).get('.h-1\\.5 > div').classes()).toContain('bg-af-warning')
    expect(mountAt(90).get('.h-1\\.5 > div').classes()).toContain('bg-af-danger')

    // 百分比文本同步分级
    expect(mountAt(74).get('.h-1\\.5 + span').classes()).toContain('text-af-ink-2')
    expect(mountAt(75).get('.h-1\\.5 + span').classes()).toContain('text-af-warning')
    expect(mountAt(89).get('.h-1\\.5 + span').classes()).toContain('text-af-warning')
    expect(mountAt(90).get('.h-1\\.5 + span').classes()).toContain('text-af-danger')
  })
})
