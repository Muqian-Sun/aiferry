import { createSiteRouter } from '@/router/createSiteRouter'
import { useAppStore } from '@/stores/app'
import { userRoutes } from './routes'

/** 用户站只展示公开的自定义菜单项。 */
export function userCustomMenuItems() {
  return useAppStore().cachedPublicSettings?.custom_menu_items ?? []
}

const router = createSiteRouter(userRoutes, {
  site: 'user',
  getCustomMenuItems: userCustomMenuItems,
})

export default router
