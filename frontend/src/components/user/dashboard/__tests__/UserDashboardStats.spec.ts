import { describe, it, expect, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'

// t() 回显 key；带参数时附上 JSON，便于断言计数
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

import UserDashboardStats from '../UserDashboardStats.vue'
import type { UserDashboardStats as UserStatsType, PlatformDashboardStats } from '@/api/usage'

function makeStats(over: Partial<UserStatsType> = {}): UserStatsType {
  return {
    total_api_keys: 1,
    active_api_keys: 1,
    total_requests: 0,
    total_input_tokens: 0,
    total_output_tokens: 0,
    total_cache_creation_tokens: 0,
    total_cache_read_tokens: 0,
    total_tokens: 0,
    total_cost: 0,
    total_actual_cost: 0,
    today_requests: 0,
    today_input_tokens: 0,
    today_output_tokens: 0,
    today_cache_creation_tokens: 0,
    today_cache_read_tokens: 0,
    today_tokens: 0,
    today_cost: 0,
    today_actual_cost: 0,
    average_duration_ms: 0,
    rpm: 0,
    tpm: 0,
    by_platform: [],
    ...over,
  }
}

function usage(platform: string, cost: number): PlatformDashboardStats {
  return {
    platform,
    total_requests: 1,
    total_tokens: 10,
    total_actual_cost: cost,
    today_requests: 1,
    today_tokens: 10,
    today_actual_cost: cost,
  }
}

function mountStats(stats: UserStatsType, isSimple = false) {
  return mount(UserDashboardStats, {
    props: { stats, balance: 0, isSimple },
    global: { stubs: { Icon: true } },
  })
}

/** 渲染出的平台卡片，按 DOM 顺序返回 data-platform */
function cardPlatforms(w: VueWrapper): string[] {
  return w.findAll('[data-testid="platform-card"]').map((c) => c.attributes('data-platform') ?? '')
}

describe('UserDashboardStats 按平台拆分', () => {
  it('只有用量的平台才产生卡片', () => {
    const w = mountStats(
      makeStats({ total_actual_cost: 0.03, today_actual_cost: 0.03, by_platform: [usage('grok', 0.03)] })
    )
    expect(cardPlatforms(w)).toEqual(['grok'])
    expect(w.text()).toContain('dashboard.platformCount:{"count":1}')
  })

  it('固定顺序之外的平台也产生卡片，并排在固定顺序之后', () => {
    const w = mountStats(
      makeStats({ total_actual_cost: 0.5, today_actual_cost: 0, by_platform: [usage('kimi', 0.3), usage('anthropic', 0.2)] })
    )
    expect(cardPlatforms(w)).toEqual(['anthropic', 'kimi'])
    expect(w.text()).toContain('Kimi')
  })

  it('总值大于各平台之和时追加"其他"卡片，且不计入平台计数', () => {
    const w = mountStats(
      makeStats({ total_actual_cost: 1.0, today_actual_cost: 0, by_platform: [usage('anthropic', 0.4)] })
    )
    expect(cardPlatforms(w)).toEqual(['anthropic', '__other__'])
    expect(w.text()).toContain('dashboard.platformOther')
    expect(w.text()).toContain('dashboard.platformCount:{"count":1}')
  })

  it('没有任何用量时不渲染整块', () => {
    const w = mountStats(makeStats())
    expect(w.html()).not.toContain('dashboard.platformBreakdown')
    expect(cardPlatforms(w)).toEqual([])
  })

  it('简易模式不渲染整块', () => {
    const w = mountStats(makeStats({ by_platform: [usage('openai', 1)] }), true)
    expect(w.html()).not.toContain('dashboard.platformBreakdown')
  })
})
