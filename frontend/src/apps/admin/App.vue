<script setup lang="ts">
import { onMounted, provide } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppShell from '@/app/AppShell.vue'
import { SITE_LAYOUT } from '@/app/siteLayout'
import AdminSidebar from '@/components/admin/layout/AdminSidebar.vue'
import { getSetupStatus } from '@/api/setup'

provide(SITE_LAYOUT, { sidebar: AdminSidebar })

const router = useRouter()
const route = useRoute()

onMounted(async () => {
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
  <AppShell />
</template>
