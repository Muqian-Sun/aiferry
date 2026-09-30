<template>
  <!--
    模型页：入口只在顶栏。从控制台点进来留在控制台壳（页头由壳画），从首页等公开页进来走公开壳（muqian 2026-09-30），
    见 consoleShell.ts；展示价按访问者倍率折算在内容组件里。
  -->
  <SiteShell :variant="shell">
    <ModelPlazaContent :response="data" :loading="loading" :error="loadFailed" :embedded="shell === 'console'" />
  </SiteShell>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import ModelPlazaContent from '@/components/modelPlaza/ModelPlazaContent.vue'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import { loadModelPlaza } from './modelPlazaQuery'
import { useAppStore } from '@/stores/app'
import { useShellVariant } from '@/components/user/shell/consoleShell'

const appStore = useAppStore()
const shell = useShellVariant()

const data = ref<ModelPlazaResponse | null>(null)
const loading = ref(true)
const loadFailed = ref(false)

onMounted(async () => {
  // 顶栏需要站点名 / Logo；有 __APP_CONFIG__ 注入时同步命中缓存
  void appStore.fetchPublicSettings()
  try {
    data.value = await loadModelPlaza()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
})
</script>
