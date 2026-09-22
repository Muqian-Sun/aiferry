import type { RouteLocationNormalized } from 'vue-router'
import { createSiteRouter } from '@/router/createSiteRouter'
import { useAdminComplianceStore } from '@/stores/adminCompliance'
import { adminRoutes } from './routes'

/** 管理站只做管理，不挂自定义页（muqian 2026-09-22 定）；标题解析不需要菜单项。 */
export function adminCustomMenuItems() {
  return []
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
