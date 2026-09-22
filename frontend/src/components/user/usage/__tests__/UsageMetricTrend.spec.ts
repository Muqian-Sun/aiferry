import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { TrendDataPoint } from '@/types'

vi.mock('vue-chartjs', () => ({
  Line: { name: 'Line', props: ['data', 'options'], template: '<canvas data-testid="line" />' }
}))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import UsageMetricTrend from '../UsageMetricTrend.vue'

const trend: TrendDataPoint[] = [
  { date: '09-21', requests: 5, input_tokens: 1, output_tokens: 1, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 2, cost: 0.5, actual_cost: 0.25 },
  { date: '09-22', requests: 7, input_tokens: 1, output_tokens: 1, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 2, cost: 1, actual_cost: 0.5 }
]

function lineData(wrapper: ReturnType<typeof mount>) {
  return wrapper.findComponent({ name: 'Line' }).props('data') as { labels: string[]; datasets: Array<{ label: string; data: number[] }> }
}

describe('UsageMetricTrend', () => {
  it('draws one line of request counts', () => {
    const data = lineData(mount(UsageMetricTrend, { props: { trendData: trend, metric: 'requests' } }))
    expect(data.labels).toEqual(['09-21', '09-22'])
    expect(data.datasets).toHaveLength(1)
    expect(data.datasets[0].label).toBe('userUi.usage.trend.requests')
    expect(data.datasets[0].data).toEqual([5, 7])
  })

  it('draws the actual (charged) cost, not the list-price cost', () => {
    const data = lineData(mount(UsageMetricTrend, { props: { trendData: trend, metric: 'cost' } }))
    expect(data.datasets[0].data).toEqual([0.25, 0.5])
  })

  it('shows the empty state without data and a spinner while loading', () => {
    expect(mount(UsageMetricTrend, { props: { trendData: [], metric: 'cost' } }).text()).toContain('userUi.usage.trend.empty')
    expect(mount(UsageMetricTrend, { props: { trendData: trend, metric: 'cost', loading: true } }).find('.spinner').exists()).toBe(true)
  })
})
