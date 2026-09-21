<template>
  <!--
    模型页：已登录走控制台壳（页头由壳提供，内容不再画自己的标题），未登录走公开壳。
    旧的 ?embedded=1 参数不再需要——已登录即控制台形态；未登录带该参数时自然降级为公开形态。
  -->
  <SiteShell :variant="isAuthenticated ? 'console' : 'public'">
    <ModelPlazaContent :response="data" :loading="loading" :error="loadFailed" :embedded="isAuthenticated" />
  </SiteShell>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import ModelPlazaContent from '@/components/modelPlaza/ModelPlazaContent.vue'
import { getModelPlaza, type ModelPlazaResponse } from '@/api/modelPlaza'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const appStore = useAppStore()
const authStore = useAuthStore()

const isAuthenticated = computed(() => authStore.isAuthenticated)

const data = ref<ModelPlazaResponse | null>(null)
const loading = ref(true)
const loadFailed = ref(false)

onMounted(async () => {
  // 顶栏需要站点名 / Logo；有 __APP_CONFIG__ 注入时同步命中缓存
  void appStore.fetchPublicSettings()
  try {
    data.value = await getModelPlaza()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
})
</script>
