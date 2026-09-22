<template>
  <!--
    首屏右侧的厂商图标云：固定的一组位置（x / y / 远近），按目录里「有图标的厂商」循环填位。
    图标用厂商自己的品牌色，远的更小更淡；每个各自缓慢飘（周期 / 相位 / 幅度都不同），不跟随鼠标。
    纯装饰：aria-hidden；父组件在没有带图标的厂商时不渲染它。
  -->
  <div class="relative aspect-square w-full max-w-[520px] select-none" aria-hidden="true" data-testid="vendor-cloud">
    <span
      v-for="(slot, index) in placed"
      :key="index"
      class="drift absolute -translate-x-1/2 -translate-y-1/2"
      :style="slotStyle(slot, index)"
      data-testid="vendor-cloud-tile"
    >
      <VendorIcon :vendor="slot.vendor" :size="DEPTH[slot.depth].icon" colored />
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import VendorIcon from '@/components/common/VendorIcon.vue'

type Depth = 0 | 1 | 2
interface Slot {
  x: number
  y: number
  depth: Depth
}

const props = defineProps<{
  /** 有图标的厂商键（父组件已过滤） */
  vendors: string[]
}>()

/**
 * 位置按「取任意前缀都散得开」排序：先中心几个大的，再往四周填小的。
 * 坐标是百分比（相对正方形容器），远近 0 近 / 1 中 / 2 远。
 */
const SLOTS: Slot[] = [
  { x: 34, y: 38, depth: 0 },
  { x: 70, y: 52, depth: 0 },
  { x: 24, y: 72, depth: 0 },
  { x: 58, y: 22, depth: 1 },
  { x: 84, y: 30, depth: 1 },
  { x: 12, y: 44, depth: 1 },
  { x: 52, y: 78, depth: 1 },
  { x: 86, y: 74, depth: 1 },
  { x: 40, y: 8, depth: 2 },
  { x: 8, y: 16, depth: 2 },
  { x: 94, y: 52, depth: 2 },
  { x: 20, y: 94, depth: 2 },
  { x: 72, y: 94, depth: 2 },
  { x: 62, y: 60, depth: 2 },
  { x: 6, y: 66, depth: 2 }
]

/** 远近三档：图标尺寸、透明度、飘动幅度 */
const DEPTH: Record<Depth, { icon: number; opacity: number; amp: number }> = {
  0: { icon: 44, opacity: 1, amp: 14 },
  1: { icon: 30, opacity: 0.8, amp: 11 },
  2: { icon: 20, opacity: 0.5, amp: 8 }
}

/** 位数随厂商数走：至少 6 个，每个厂商最多重复 3 次，封顶 15 */
const placed = computed(() => {
  const vendors = props.vendors
  if (!vendors.length) return []
  const count = Math.min(SLOTS.length, Math.max(6, vendors.length * 3))
  return SLOTS.slice(0, count).map((slot, index) => ({ ...slot, vendor: vendors[index % vendors.length] }))
})

function slotStyle(slot: Slot, index: number) {
  const depth = DEPTH[slot.depth]
  return {
    left: `${slot.x}%`,
    top: `${slot.y}%`,
    opacity: depth.opacity,
    // 每个图标周期 7–12s、相位各错开，看起来互不同步
    '--drift-dur': `${7 + (index % 6)}s`,
    '--drift-delay': `-${((index * 1.7) % 9).toFixed(1)}s`,
    '--drift-x': `${(index % 2 ? -1 : 1) * (depth.amp * 0.6)}px`,
    '--drift-y': `${depth.amp}px`
  }
}
</script>

<style scoped>
/* 自己飘：上下为主、左右为辅，来回一个周期 */
.drift > :deep(svg) {
  animation: drift var(--drift-dur, 9s) ease-in-out var(--drift-delay, 0s) infinite;
}

@keyframes drift {
  0%,
  100% {
    transform: translate(0, 0);
  }
  33% {
    transform: translate(var(--drift-x), calc(var(--drift-y) * -1));
  }
  66% {
    transform: translate(calc(var(--drift-x) * -0.6), calc(var(--drift-y) * 0.5));
  }
}

@media (prefers-reduced-motion: reduce) {
  .drift > :deep(svg) {
    animation: none;
  }
}
</style>
