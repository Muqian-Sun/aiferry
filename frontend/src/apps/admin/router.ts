import { createSiteRouter } from '@/router/createSiteRouter'
import { adminRoutes } from './routes'

/** 管理站只做管理，不挂自定义页（muqian 2026-09-22 定）；标题解析不需要菜单项。 */
export function adminCustomMenuItems() {
  return []
}

const router = createSiteRouter(adminRoutes, {
  site: 'admin',
  getCustomMenuItems: adminCustomMenuItems,
})

export default router
