<template>
  <!--
    首屏右侧的「全球模型厂商」环形图（muqian 2026-09-23：要全球所有知名厂商的图标，不止目录里那几家）。
    圆心是本站 logo（一把 Key），外面三圈轨道，每圈一个旋转层匀速转（内顺 / 中逆 / 外顺，越外越慢），
    图标反向自转保持正立；图标全部来自 @lobehub/icons 的官方 SVG（见 vendorLogos.ts 与生成脚本），用厂商原色。
    纯装饰：aria-hidden；不跟随鼠标；减少动态效果偏好下整幅静止。
  -->
  <div class="relative aspect-square w-full max-w-[540px] select-none" aria-hidden="true" data-testid="vendor-orbit">
    <!-- 轨道线 -->
    <span
      v-for="ring in rings"
      :key="`track-${ring.radius}`"
      class="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full border border-af-hairline"
      :style="{ width: `${ring.radius * 2}%`, height: `${ring.radius * 2}%` }"
    />

    <!-- 圆心：本站 logo + 两道向外扩散的光环 -->
    <div class="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2">
      <span class="orbit-pulse absolute inset-0 rounded-full bg-af-brand/20" />
      <span class="orbit-pulse orbit-pulse-late absolute inset-0 rounded-full bg-af-brand/20" />
      <span class="relative flex h-20 w-20 items-center justify-center rounded-full border border-af-hairline bg-af-sheet shadow-[0_12px_32px_-16px_rgb(var(--af-ink)/0.45)]">
        <img :src="logo" alt="" class="h-10 w-10 object-contain" data-testid="vendor-orbit-center" />
      </span>
    </div>

    <!-- 三圈图标 -->
    <div
      v-for="ring in rings"
      :key="`ring-${ring.radius}`"
      class="orbit-spin absolute inset-0"
      :style="{ '--orbit-dur': `${ring.duration}s`, animationDirection: ring.reverse ? 'reverse' : 'normal' }"
      data-testid="vendor-orbit-ring"
    >
      <span
        v-for="slot in ring.slots"
        :key="slot.logo.key"
        class="absolute -translate-x-1/2 -translate-y-1/2"
        :style="{ left: `${slot.left}%`, top: `${slot.top}%` }"
        :title="slot.logo.title"
        data-testid="vendor-orbit-logo"
      >
        <!-- 反向自转保持正立；圆底用页面底色，把轨道线在图标处断开 -->
        <span
          class="orbit-spin flex items-center justify-center rounded-full bg-af-sheet"
          :style="{ '--orbit-dur': `${ring.duration}s`, animationDirection: ring.reverse ? 'normal' : 'reverse', width: `${ring.size + 12}px`, height: `${ring.size + 12}px` }"
        >
          <img
            v-if="slot.logo.kind === 'color'"
            :src="slot.src"
            alt=""
            :width="ring.size"
            :height="ring.size"
            draggable="false"
            :class="slot.logo.inkOnDark ? 'orbit-invert-dark' : ''"
            :data-logo="slot.logo.key"
          />
          <span
            v-else
            :class="['orbit-mask block', slot.logo.tint ? '' : 'bg-af-ink']"
            :style="maskStyle(slot.logo, slot.src, ring.size)"
            :data-logo="slot.logo.key"
          />
        </span>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useTheme } from '@/composables/useTheme'
import { VENDOR_LOGOS, type VendorLogo } from './vendorLogos'

defineProps<{
  /** 圆心的站点 logo 地址 */
  logo: string
}>()

/**
 * 三圈的几何：半径是容器边长的百分比，数量 / 图标尺寸 / 一圈时长按「相邻图标间距 ≈ 80–90px（容器 540px 时）」取的。
 * 参数是拍的，看着不挤也不空；增减厂商时只改 vendorLogos 的清单，这里按顺序切片。
 */
const RING_SPECS = [
  { radius: 20, count: 8, size: 30, duration: 60, reverse: false },
  { radius: 33.5, count: 14, size: 24, duration: 90, reverse: true },
  { radius: 47, count: 18, size: 20, duration: 120, reverse: false }
] as const

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

const rings = computed(() => {
  let offset = 0
  return RING_SPECS.map((spec, ringIndex) => {
    const logos = VENDOR_LOGOS.slice(offset, offset + spec.count)
    offset += spec.count
    // 每圈起始角错开一点，三圈的图标不会排成一条射线
    const start = -90 + ringIndex * 17
    const slots = logos.map((logo, i) => {
      const angle = ((start + (360 / logos.length) * i) * Math.PI) / 180
      return {
        logo,
        src: toSrc(logo),
        left: 50 + spec.radius * Math.cos(angle),
        top: 50 + spec.radius * Math.sin(angle)
      }
    })
    return { ...spec, slots }
  }).filter((ring) => ring.slots.length > 0)
})

/** 单色图标的遮罩样式：data URI 经 encodeURIComponent，里面不会有双引号 */
function maskStyle(logo: VendorLogo, src: string, size: number) {
  return {
    width: `${size}px`,
    height: `${size}px`,
    backgroundColor: logo.tint ?? undefined,
    '--orbit-mask': `url("${src}")`
  }
}
</script>

<style scoped>
/* 匀速转一圈；方向由内联 animation-direction 决定（图标层与所在圈反向，抵消后保持正立） */
.orbit-spin {
  animation: orbit-spin var(--orbit-dur, 90s) linear infinite;
}

@keyframes orbit-spin {
  to {
    transform: rotate(360deg);
  }
}

/*
 * 全彩图标里写死了近黑色块（Luma）：深色模式下反相，色相转回来，彩色部分基本不变。
 * 整个选择器都包进 :global()——写成 `:global(.dark) .x` 会被 Vue 编成 `.dark{…}`，把整页反相。
 */
:global(.dark .orbit-invert-dark) {
  filter: invert(1) hue-rotate(180deg);
}

/* 单色图标：官方 SVG 当遮罩，颜色取背景色（品牌色或墨色 token） */
.orbit-mask {
  -webkit-mask: var(--orbit-mask) center / contain no-repeat;
  mask: var(--orbit-mask) center / contain no-repeat;
}

/* 圆心光环：从 logo 大小向外扩到 2.2 倍并淡出，两道错开半个周期 */
.orbit-pulse {
  animation: orbit-pulse 3.2s cubic-bezier(0.22, 1, 0.36, 1) infinite;
}

.orbit-pulse-late {
  animation-delay: 1.6s;
}

@keyframes orbit-pulse {
  0% {
    transform: scale(1);
    opacity: 0.9;
  }
  100% {
    transform: scale(2.2);
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .orbit-spin,
  .orbit-pulse {
    animation: none;
  }

  .orbit-pulse {
    opacity: 0;
  }
}
</style>
