<template>
  <!--
    与 BaseDialog 同样的接口（show / title / width / close），但只把正文原地渲染出来、不要页脚。
    让原来的对话框组件用 <component :is> 切成抽屉里的一个页签（A5：渠道统计、定时测试）。
  -->
  <div v-if="show"><slot /></div>
</template>

<script setup lang="ts">
defineProps<{ show: boolean; title?: string; width?: string }>()
defineEmits<{ (e: 'close'): void }>()
</script>
