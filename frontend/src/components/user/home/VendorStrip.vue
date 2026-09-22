<template>
  <!--
    首屏下的厂商行：目录里出现过的厂商，单色图标 + 品牌名；没有图标的厂商只出文字。
    数据全部来自 /model-plaza 的 vendor，目录空时整行不出现（由父组件 v-if）。
  -->
  <ul class="flex flex-wrap items-center gap-x-6 gap-y-3" data-testid="vendor-strip">
    <li v-for="item in items" :key="item.vendor" class="flex items-center gap-2 text-sm text-af-ink-2">
      <svg
        v-if="item.icon"
        width="18"
        height="18"
        viewBox="0 0 24 24"
        xmlns="http://www.w3.org/2000/svg"
        fill="currentColor"
        fill-rule="evenodd"
        class="shrink-0 text-af-ink-3"
        aria-hidden="true"
      >
        <path v-for="(d, index) in item.icon.paths" :key="index" :d="d" />
      </svg>
      <span>{{ item.label }}</span>
    </li>
  </ul>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { iconData, vendorIconKey } from '@/components/common/modelIconData'
import { vendorLabel } from '@/components/modelPlaza/catalog'

const props = defineProps<{ vendors: string[] }>()

const items = computed(() =>
  props.vendors.map((vendor) => {
    const key = vendorIconKey(vendor)
    return { vendor, label: vendorLabel(vendor), icon: key ? iconData[key] : null }
  })
)
</script>
