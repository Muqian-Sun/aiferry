import { describe, expect, it } from 'vitest'
import { adminRoutes } from '../routes'
import { adminCustomMenuItems } from '../router'

// muqian 2026-09-22 定：管理站删掉插件管理、优惠码、邀请返利、数据备份（设置页签）、自定义页五项；
// 优惠码的后端 2026-09-23 已整链删除，其余几项后端接口暂留。这里只盯前端路由表不再长回来。
describe('admin routes: dropped features', () => {
  const paths = adminRoutes.flatMap((route) => [route.path, ...(route.children ?? []).map((child) => child.path)])

  it.each(['/plugins', '/promo-codes', '/affiliates', '/affiliates/invites', '/affiliates/rebates', '/affiliates/transfers', '/custom/:id'])(
    'has no route for %s',
    (path) => {
      expect(paths).not.toContain(path)
    }
  )

  it('never lazy-loads the deleted views', () => {
    const sources = adminRoutes.map((route) => String(route.component ?? '')).join('\n')
    for (const name of ['PluginsView', 'PromoCodesView', 'AdminAffiliate', 'BackupView', 'CustomPageView']) {
      expect(sources).not.toContain(name)
    }
  })

  it('exposes no custom menu items on the admin site', () => {
    expect(adminCustomMenuItems()).toEqual([])
  })
})
