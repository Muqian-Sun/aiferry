<template>
  <!--
    厂商图标：按目录 vendor 取 lobehub 路径；colored 时用厂商品牌色，否则 currentColor。没有图标的厂商不渲染。
    深色模式下品牌色本身就是黑 / 近黑的（OpenAI、xAI 这类单色标）在深底上看不见，改用 currentColor（跟文字色走）。
  -->
  <svg
    v-if="icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    xmlns="http://www.w3.org/2000/svg"
    :fill="fill"
    fill-rule="evenodd"
    class="shrink-0"
    aria-hidden="true"
  >
    <path v-for="(d, index) in icon.paths" :key="index" :d="d" />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useTheme } from '@/composables/useTheme'
import { iconData, vendorIconKey } from './modelIconData'

const props = withDefaults(defineProps<{ vendor: string; size?: number; colored?: boolean }>(), { size: 18, colored: false })

/** WCAG 2.x 1.4.11 非文字内容的最低对比度 */
const MIN_GRAPHIC_CONTRAST = 3

const { isDark } = useTheme()

const icon = computed(() => {
  const key = vendorIconKey(props.vendor)
  return key ? iconData[key] : null
})

/**
 * 只在深色模式下判：浅色模式里对比度不足的是黄 / 青 / 橙这类真彩色标，照样认得出，保留品牌色；
 * 深色模式里不足的是本来就设计成黑色的单色标，换成文字色才看得见。读不到页面底色（测试环境）时保留品牌色。
 */
const fill = computed(() => {
  if (!props.colored || !icon.value) return 'currentColor'
  if (!isDark.value) return icon.value.color
  const background = readSheetRgb()
  const brand = parseHex(icon.value.color)
  if (!background || !brand) return icon.value.color
  return contrastRatio(brand, background) < MIN_GRAPHIC_CONTRAST ? 'currentColor' : icon.value.color
})

type Rgb = [number, number, number]

/** 页面底色 = token --af-sheet（写法是 "R G B" 三个通道）；applyTheme 先改 isDark 再同步切 html.dark，渲染时已是新值 */
function readSheetRgb(): Rgb | null {
  const channels = getComputedStyle(document.documentElement).getPropertyValue('--af-sheet').trim().split(/\s+/).map(Number)
  return channels.length === 3 && channels.every((c) => Number.isFinite(c)) ? (channels as Rgb) : null
}

function parseHex(hex: string): Rgb | null {
  const match = /^#([0-9a-f]{6})$/i.exec(hex.trim())
  if (!match) return null
  const value = parseInt(match[1], 16)
  return [(value >> 16) & 255, (value >> 8) & 255, value & 255]
}

/** WCAG 2.x 相对亮度与对比度公式 */
function relativeLuminance([r, g, b]: Rgb): number {
  const linear = (channel: number) => {
    const c = channel / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  }
  return 0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b)
}

function contrastRatio(a: Rgb, b: Rgb): number {
  const [high, low] = [relativeLuminance(a), relativeLuminance(b)].sort((x, y) => y - x)
  return (high + 0.05) / (low + 0.05)
}
</script>
