<template>
  <!-- 模型页：不分登录与否都走公开壳，入口只在顶栏（muqian 2026-09-30）；展示价按访问者倍率折算在内容组件里 -->
  <SiteShell variant="public">
    <ModelPlazaContent :response="data" :loading="loading" :error="loadFailed" />
  </SiteShell>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import SiteShell from '@/components/user/shell/SiteShell.vue'
import ModelPlazaContent from '@/components/modelPlaza/ModelPlazaContent.vue'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import { loadModelPlaza } from './modelPlazaQuery'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

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
