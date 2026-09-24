<template>
  <!-- 渠道健康：只有 V2 被动监控（按真实请求聚合）。V1 主动探测已从管理端下线（A3），标题由 AppLayout 的页头给出。 -->
  <AppLayout>
    <div class="w-full min-w-0 pb-8">
      <MonitorSettingsPanel />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import MonitorSettingsPanel from '@/features/channel-monitor-v2/MonitorSettingsPanel.vue'
</script>
