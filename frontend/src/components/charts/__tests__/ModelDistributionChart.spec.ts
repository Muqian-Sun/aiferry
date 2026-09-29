import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import ModelDistributionChart from '../ModelDistributionChart.vue'

const messages: Record<string, string> = {
  'admin.dashboard.modelDistribution': 'Model Distribution',
  'admin.dashboard.model': 'Model',
  'admin.dashboard.requests': 'Requests',
  'admin.dashboard.tokens': 'Tokens',
  'admin.dashboard.noDataAvailable': 'No data available',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const message = messages[key] ?? key
        return params
          ? message.replace(/\{(\w+)\}/g, (_, name: string) => String(params[name]))
          : message
      },
    }),
  }
})

describe('ModelDistributionChart', () => {
  const modelStats = [
    {
      model: 'model-a',
      requests: 8,
      input_tokens: 100,
      output_tokens: 50,
      cache_creation_tokens: 0,
      cache_read_tokens: 0,
      total_tokens: 1000,
      cost: 1.5,
      actual_cost: 0.2,
    },
    {
      model: 'model-b',
      requests: 3,
      input_tokens: 40,
      output_tokens: 20,
      cache_creation_tokens: 0,
      cache_read_tokens: 0,
      total_tokens: 500,
      cost: 0.5,
      actual_cost: 1.4,
    },
  ]

  it('uses total_tokens and token ordering by default', () => {
    const wrapper = mount(ModelDistributionChart, {
      props: {
        modelStats,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('model-a')
    expect(rows[1].text()).toContain('model-b')
    // 占比按 Token：1000 / 1500
    expect(rows[0].text()).toContain('67%')
  })
})
