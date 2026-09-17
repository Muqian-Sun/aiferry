<script setup lang="ts">
import { onBeforeUnmount, onMounted, provide, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppShell from '@/app/AppShell.vue'
import { SITE_LAYOUT } from '@/app/siteLayout'
import AdminComplianceDialog from '@/components/admin/AdminComplianceDialog.vue'
import AdminSidebar from '@/components/admin/layout/AdminSidebar.vue'
import { getSetupStatus } from '@/api/setup'
import { useAuthStore } from '@/stores/auth'
import { useAdminComplianceStore } from '@/stores/adminCompliance'

provide(SITE_LAYOUT, { sidebar: AdminSidebar, onboardingStorageKey: 'admin_guide' })

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const adminComplianceStore = useAdminComplianceStore()

function onAdminComplianceRequired(event: Event) {
  const detail = (event as CustomEvent<Record<string, string>>).detail || {}
  adminComplianceStore.requireAcknowledgement(detail)
}

watch(
  () => authStore.isAuthenticated,
  (isAuthenticated) => {
    if (isAuthenticated) {
      adminComplianceStore.fetchStatus().catch((error) => {
        console.error('Failed to fetch admin compliance status:', error)
      })
    } else {
      adminComplianceStore.reset()
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  window.removeEventListener('admin-compliance-required', onAdminComplianceRequired)
})

onMounted(async () => {
  window.addEventListener('admin-compliance-required', onAdminComplianceRequired)

  // 安装向导只在管理后台提供：尚未安装时引导到 /setup
  try {
    const status = await getSetupStatus()
    if (status.needs_setup && route.path !== '/setup') {
      router.replace('/setup')
    }
  } catch {
    // If setup endpoint fails, assume normal mode and continue
  }
})
</script>

<template>
  <AppShell>
    <AdminComplianceDialog />
  </AppShell>
</template>
