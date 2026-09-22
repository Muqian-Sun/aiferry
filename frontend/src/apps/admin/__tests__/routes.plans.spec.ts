import { describe, expect, it } from 'vitest'
import { adminRoutes } from '../routes'

// 订阅套餐属于「订阅」：支付关着时管理员也要能进套餐编辑器（手工分配订阅要先有套餐），不挂支付门。
describe('admin routes: subscription plans', () => {
  it('does not gate /orders/plans behind payment', () => {
    const plans = adminRoutes.find((route) => route.path === '/orders/plans')
    expect(plans).toBeDefined()
    expect(plans?.meta?.requiresPayment).toBeUndefined()
    expect(plans?.meta?.requiresAdmin).toBe(true)
  })
})
