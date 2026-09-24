<template>
  <!--
    管理站列表页共用的弹出层（A4）：筛选标签、列设置、行操作「⋯」、页头工具菜单都用它。
    面板挂到 body 上（表格滚动容器会裁掉普通的 absolute 下拉），按触发器位置定位，放不下就往上翻。
    点外面、按 Esc、任何地方滚动都会关掉，免得面板和触发器错位。
  -->
  <span ref="triggerEl" class="inline-flex" @click.stop="toggle">
    <slot name="trigger" :open="open" />
  </span>
  <Teleport to="body">
    <div
      v-if="open"
      ref="panelEl"
      role="menu"
      class="fixed z-[9999] overflow-hidden rounded-lg border border-af-hairline bg-af-sheet py-1 shadow-lg"
      :class="widthClass"
      :style="panelStyle"
      @click="onPanelClick"
      @keydown.esc="close"
    >
      <slot :close="close" />
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    /** 面板与触发器哪一侧对齐 */
    align?: 'start' | 'end'
    widthClass?: string
    /** 点了面板里的按钮就关（多选类菜单传 false） */
    closeOnSelect?: boolean
  }>(),
  { align: 'end', widthClass: 'w-48', closeOnSelect: true }
)

const emit = defineEmits<{ (e: 'open'): void; (e: 'close'): void }>()

const open = ref(false)
const triggerEl = ref<HTMLElement | null>(null)
const panelEl = ref<HTMLElement | null>(null)
const panelStyle = ref<Record<string, string>>({})

const GAP = 4
const MARGIN = 8

function position() {
  const trigger = triggerEl.value?.firstElementChild as HTMLElement | null
  const panel = panelEl.value
  if (!trigger || !panel) return
  const rect = trigger.getBoundingClientRect()
  const panelRect = panel.getBoundingClientRect()
  let top = rect.bottom + GAP
  if (top + panelRect.height > window.innerHeight - MARGIN && rect.top - GAP - panelRect.height > MARGIN) {
    top = rect.top - GAP - panelRect.height
  }
  let left = props.align === 'end' ? rect.right - panelRect.width : rect.left
  left = Math.min(Math.max(left, MARGIN), window.innerWidth - panelRect.width - MARGIN)
  panelStyle.value = { top: `${Math.round(top)}px`, left: `${Math.round(left)}px` }
}

function onDocumentPointer(event: Event) {
  const target = event.target as Node | null
  if (!target) return
  if (panelEl.value?.contains(target) || triggerEl.value?.contains(target)) return
  close()
}

function onScroll(event: Event) {
  // 面板内部自己的滚动（长列表）不关
  if (panelEl.value && event.target instanceof Node && panelEl.value.contains(event.target)) return
  close()
}

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') close()
}

function bind(on: boolean) {
  if (on) {
    document.addEventListener('pointerdown', onDocumentPointer, true)
    document.addEventListener('keydown', onKey)
    window.addEventListener('scroll', onScroll, true)
    window.addEventListener('resize', close)
  } else {
    document.removeEventListener('pointerdown', onDocumentPointer, true)
    document.removeEventListener('keydown', onKey)
    window.removeEventListener('scroll', onScroll, true)
    window.removeEventListener('resize', close)
  }
}

watch(open, async (value) => {
  if (value) {
    // 先放到屏幕外量尺寸，再定位，避免闪一下
    panelStyle.value = { top: '-9999px', left: '-9999px' }
    await nextTick()
    position()
    bind(true)
    emit('open')
  } else {
    bind(false)
    emit('close')
  }
})

function toggle() {
  open.value = !open.value
}

function close() {
  open.value = false
}

function onPanelClick(event: MouseEvent) {
  if (!props.closeOnSelect) return
  if ((event.target as HTMLElement | null)?.closest('button, a')) close()
}

onBeforeUnmount(() => bind(false))

defineExpose({ close, open })
</script>
