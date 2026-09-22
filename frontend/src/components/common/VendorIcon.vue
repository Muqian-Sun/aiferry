<template>
  <!-- 厂商图标：按目录 vendor 取 lobehub 路径；colored 时用厂商品牌色，否则 currentColor。没有图标的厂商不渲染 -->
  <svg
    v-if="icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    :fill="colored ? icon.color : 'currentColor'"
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

const props = withDefaults(defineProps<{ vendor: string; size?: number; colored?: boolean }>(), { size: 18, colored: false })

const icon = computed(() => {
  const key = vendorIconKey(props.vendor)
  return key ? iconData[key] : null
})
</script>
