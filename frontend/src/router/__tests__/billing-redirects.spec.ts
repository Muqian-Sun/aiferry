/**
 * 旧账务路径的 redirect：/purchase 按意图分流到充值 / 订阅页签，其余旧路径一对一映射，query 全部保留。
 * 后端微信支付授权的 redirect 仍写死 /purchase，所以这些 redirect 是长期契约，不是过渡期兼容。
 */
import { describe, expect, it, vi } from 'vitest'
import type { RouteLocation, RouteRecordRaw } from 'vue-router'

vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: { payment: 'payment_enabled', subscription: 'subscription_enabled', affiliate: 'affiliate_enabled' },
  isFeatureFlagEnabled: () => true,
  resolveFeatureFlag: () => true,
}))

import { userRoutes } from '@/apps/user/routes'

function redirectOf(path: string) {
  const record = userRoutes.find((r) => r.path === path)
  if (!record || typeof record.redirect !== 'function') throw new Error(`no redirect function for ${path}`)
  return record.redirect as (to: RouteLocation) => { path: string; query: Record<string, unknown> }
}

function to(path: string, query: Record<string, string>): RouteLocation {
  return { path, query } as unknown as RouteLocation
}

describe('legacy billing redirects', () => {
  it('sends plain /purchase (with its query) to the recharge tab', () => {
    expect(redirectOf('/purchase')(to('/purchase', { resume_token: 'abc' }))).toEqual({
      path: '/billing/recharge',
      query: { resume_token: 'abc' }
    })
  })

  it('sends ?tab=subscription to the subscriptions tab, keeps other params and drops the tab param', () => {
    expect(redirectOf('/purchase')(to('/purchase', { tab: 'subscription', plan_id: '3' }))).toEqual({
      path: '/billing/subscriptions',
      query: { plan_id: '3' }
    })
  })

  it('sends a WeChat subscription resume (order_type=subscription) to the subscriptions tab with every param kept', () => {
    const query = { from: 'wechat', payment_type: 'wxpay', order_type: 'subscription', plan_id: '7' }
    expect(redirectOf('/purchase')(to('/purchase', query))).toEqual({ path: '/billing/subscriptions', query })
  })

  it('maps /subscriptions to /billing/subscriptions keeping the query', () => {
    expect(redirectOf('/subscriptions')(to('/subscriptions', { page: '2' }))).toEqual({ path: '/billing/subscriptions', query: { page: '2' } })
  })

  it('mounts the recharge tab as the payment engine in recharge mode', () => {
    const billing = userRoutes.find((r) => r.path === '/billing') as RouteRecordRaw & { children: RouteRecordRaw[] }
    const recharge = billing.children.find((c) => c.path === 'recharge')
    expect(recharge?.props).toEqual({ mode: 'recharge' })
  })
})
