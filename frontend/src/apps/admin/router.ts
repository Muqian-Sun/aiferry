import type { RouteLocationNormalized } from 'vue-router'
import { createSiteRouter } from '@/router/createSiteRouter'
import { useAppStore } from '@/stores/app'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useAdminComplianceStore } from '@/stores/adminCompliance'
import { adminRoutes } from './routes'

/** 管理后台展示公开自定义菜单项与仅管理员可见的菜单项。 */
export function adminCustomMenuItems() {
  return [
    ...(useAppStore().cachedPublicSettings?.custom_menu_items ?? []),
    ...useAdminSettingsStore().customMenuItems,
  ]
}

async function ensureAdminCompliance(_to: RouteLocationNormalized): Promise<void> {
  const adminComplianceStore = useAdminComplianceStore()
  if (adminComplianceStore.initialized) {
    return
  }
  try {
    await adminComplianceStore.fetchStatus()
  } catch (error) {
    const err = error as { status?: number; code?: string; metadata?: Record<string, string> }
    if (err.status === 423 && err.code === 'ADMIN_COMPLIANCE_ACK_REQUIRED') {
      adminComplianceStore.requireAcknowledgement(err.metadata)
    }
  }
}

const router = createSiteRouter(adminRoutes, {
  site: 'admin',
  getCustomMenuItems: adminCustomMenuItems,
  beforeProtectedRoute: ensureAdminCompliance,
})

export default router
