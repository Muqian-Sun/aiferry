<template>
  <!--
    首屏右侧的厂商图标云：muqian 点名的 11 家大模型，散开摆放、各自缓慢飘（周期 / 相位 / 幅度都不同），不跟随鼠标。
    图标全部来自 @lobehub/icons 的官方 SVG（见 vendorLogos.ts 与 scripts/gen-vendor-logos.mjs），用厂商原色；
    远的更小更淡，形成一点纵深。纯装饰：aria-hidden；减少动态效果偏好下静止。
  -->
  <div class="relative aspect-square w-full max-w-[520px] select-none" aria-hidden="true" data-testid="vendor-cloud">
    <span
      v-for="(slot, index) in placed"
      :key="slot.logo.key"
      class="drift absolute -translate-x-1/2 -translate-y-1/2"
      :style="slotStyle(slot, index)"
      :title="slot.logo.title"
      data-testid="vendor-cloud-tile"
    >
      <img
        v-if="slot.logo.kind === 'color'"
        :src="slot.src"
        alt=""
        :width="DEPTH[slot.depth].size"
        :height="DEPTH[slot.depth].size"
        draggable="false"
        :class="slot.logo.inkOnDark ? 'cloud-invert-dark' : ''"
        :data-logo="slot.logo.key"
      />
      <span
        v-else
        :class="['cloud-mask block', slot.logo.tint ? '' : 'bg-af-ink']"
        :style="maskStyle(slot.logo, slot.src, DEPTH[slot.depth].size)"
        :data-logo="slot.logo.key"
      />
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useTheme } from '@/composables/useTheme'
import { VENDOR_LOGOS, type VendorLogo } from './vendorLogos'

type Depth = 0 | 1 | 2
interface Slot {
  x: number
  y: number
  depth: Depth
}

/**
 * 11 个位置（百分比坐标，相对正方形容器），和 VENDOR_LOGOS 的顺序一一对应：
 * 前几个最知名的放近处（大、清晰），其余往四周散开、逐渐变远。位置是拍的，看着不挤不空。
 */
const SLOTS: Slot[] = [
  { x: 30, y: 30, depth: 0 },
  { x: 70, y: 26, depth: 0 },
  { x: 52, y: 54, depth: 0 },
  { x: 22, y: 68, depth: 0 },
  { x: 78, y: 66, depth: 0 },
  { x: 50, y: 12, depth: 1 },
  { x: 90, y: 44, depth: 1 },
  { x: 44, y: 86, depth: 1 },
  { x: 10, y: 44, depth: 1 },
  { x: 74, y: 90, depth: 1 },
  { x: 14, y: 12, depth: 1 }
]

/**
 * 远近三档：图标尺寸、透明度、飘动幅度。都是拍的：最初远档只有 8px、周期 10s 以上，muqian 看着像静止，
 * 现在所有位置只用近两档，幅度 18 / 15px，周期 6–10s；尺寸按 muqian「缩小一点点」从 60 / 46 调到 52 / 40。
 * 第三档保留给以后加厂商时用。
 */
const DEPTH: Record<Depth, { size: number; opacity: number; amp: number }> = {
  0: { size: 52, opacity: 1, amp: 18 },
  1: { size: 40, opacity: 0.9, amp: 15 },
  2: { size: 32, opacity: 0.7, amp: 12 }
}

/**
 * 全彩 SVG 里的墨色笔画写的是 currentColor（lobehub 的设计是跟随文字色），放进 <img> 继承不到文字色会一律变黑，
 * 所以编码前换成当前主题的墨色（读 --af-ink），切主题时重读。
 */
const { isDark } = useTheme()
function readInkColor(): string {
  const channels = typeof document === 'undefined' ? '' : getComputedStyle(document.documentElement).getPropertyValue('--af-ink').trim()
  return channels ? `rgb(${channels.split(/\s+/).join(',')})` : 'currentColor'
}
const inkColor = ref(readInkColor())
// applyTheme 先改 isDark 再切 html.dark，都是同步的；watch 默认在渲染前跑，这时类名已经切好
watch(isDark, () => {
  inkColor.value = readInkColor()
})

/** 原始 SVG 存在 vendorLogos.ts 里（体积小一半），用时再编成 data URI */
function toSrc(logo: VendorLogo): string {
  const svg = logo.kind === 'color' ? logo.svg.replace(/currentColor/g, inkColor.value) : logo.svg
  return `data:image/svg+xml,${encodeURIComponent(svg)}`
}

const placed = computed(() =>
  VENDOR_LOGOS.slice(0, SLOTS.length).map((logo, index) => ({ ...SLOTS[index], logo, src: toSrc(logo) }))
)

function slotStyle(slot: Slot, index: number) {
  const depth = DEPTH[slot.depth]
  return {
    left: `${slot.x}%`,
    top: `${slot.y}%`,
    opacity: depth.opacity,
    // 每个图标周期 6–10s、相位各错开，看起来互不同步
    '--drift-dur': `${6 + (index % 5)}s`,
    '--drift-delay': `-${((index * 1.7) % 9).toFixed(1)}s`,
    '--drift-x': `${(index % 2 ? -1 : 1) * (depth.amp * 0.6)}px`,
    '--drift-y': `${depth.amp}px`
  }
}

/** 单色图标的遮罩样式：data URI 经 encodeURIComponent，里面不会有双引号 */
function maskStyle(logo: VendorLogo, src: string, size: number) {
  return {
    width: `${size}px`,
    height: `${size}px`,
    backgroundColor: logo.tint ?? undefined,
    '--cloud-mask': `url("${src}")`
  }
}
</script>

<style scoped>
/* 自己飘：上下为主、左右为辅，来回一个周期 */
.drift > * {
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

/* 单色图标：官方 SVG 当遮罩，颜色取背景色（品牌色或墨色 token） */
.cloud-mask {
  -webkit-mask: var(--cloud-mask) center / contain no-repeat;
  mask: var(--cloud-mask) center / contain no-repeat;
}

/*
 * 全彩图标里写死了近黑色块：深色模式下反相，色相转回来，彩色部分基本不变。
 * 整个选择器都包进 :global()——写成 `:global(.dark) .x` 会被 Vue 编成 `.dark{…}`，把整页反相。
 */
:global(.dark .cloud-invert-dark) {
  filter: invert(1) hue-rotate(180deg);
}

@media (prefers-reduced-motion: reduce) {
  .drift > * {
    animation: none;
  }
}
</style>
