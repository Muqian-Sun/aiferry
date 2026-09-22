<template>
  <!-- 厂商单色图标：按目录 vendor 取 lobehub 路径，用 currentColor 画；没有图标的厂商不渲染 -->
  <svg
    v-if="icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    fill="currentColor"
    fill-rule="evenodd"
    class="shrink-0"
    aria-hidden="true"
  >
    <path v-for="(d, index) in icon.paths" :key="index" :d="d" />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { iconData, vendorIconKey } from './modelIconData'

const props = withDefaults(defineProps<{ vendor: string; size?: number }>(), { size: 18 })

const icon = computed(() => {
  const key = vendorIconKey(props.vendor)
  return key ? iconData[key] : null
})
</script>
