<template>
  <!--
    首屏下的厂商行：目录里出现过的厂商，单色图标 + 品牌名；没有图标的厂商只出文字。
    数据全部来自 /model-plaza 的 vendor，目录空时整行不出现（由父组件 v-if）。
  -->
  <ul class="flex flex-wrap items-center gap-x-6 gap-y-3" data-testid="vendor-strip">
    <li v-for="vendor in vendors" :key="vendor" class="flex items-center gap-2 text-sm text-af-ink-2">
      <VendorIcon :vendor="vendor" class="text-af-ink-3" />
      <span>{{ vendorLabel(vendor) }}</span>
    </li>
  </ul>
</template>

<script setup lang="ts">
import VendorIcon from '@/components/common/VendorIcon.vue'
import { vendorLabel } from '@/components/modelPlaza/catalog'

defineProps<{ vendors: string[] }>()
</script>
