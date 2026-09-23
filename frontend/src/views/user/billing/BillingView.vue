<template>
  <!--
    账务的父路由：子页面（充值 / 订阅 / 订单 / 兑换 / 邀请）由 router-view 渲染。
    子页入口在控制台左侧栏的「账务」组（与子路由守卫共用 billingTabs 的开关判断），这里不再画第二层页签；
    页头标题与说明取当前子路由的 meta。
  -->
  <SiteShell>
    <RouterView />
  </SiteShell>
</template>

<script setup lang="ts">
import SiteShell from '@/components/user/shell/SiteShell.vue'
</script>
