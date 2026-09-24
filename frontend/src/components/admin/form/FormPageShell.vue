<template>
  <!--
    表单页壳（A5-c）：插槽与 BaseDialog 一致（默认插槽 = 表单体，#footer = 按钮），
    表单组件用 <component :is="layout === 'page' ? FormPageShell : BaseDialog"> 在弹窗和整页之间切换。
    lg 以上两栏：左侧分区导航（sticky），右侧表单栏（max-w-3xl）；lg 以下导航变成表单上方一行可横滑的链接。
    导航从表单体里的 data-form-section 标记生成：字段随平台 / 类型出现消失时重新扫描，只列出页面上真有的分区。
    底部保存条贴住视口底边（sticky bottom-0），和表单栏同宽。一张面、hairline 分隔，不画卡片。
  -->
  <div v-if="show" class="lg:flex lg:gap-10" data-testid="form-page-shell">
    <!-- 没有分区时（如 OAuth 第二步）lg 以上仍占住导航栏宽度，表单栏不左右跳；窄屏直接不占位 -->
    <nav
      :class="['lg:w-44 lg:shrink-0', sections.length ? 'mb-6 lg:mb-0' : 'hidden lg:block']"
      :aria-label="title || undefined"
      data-testid="form-page-nav"
    >
      <ul
        v-if="sections.length"
        class="flex gap-1 overflow-x-auto border-b border-af-hairline scrollbar-hide lg:sticky lg:top-[calc(var(--af-topbar-h)+1rem)] lg:flex-col lg:gap-0 lg:overflow-visible lg:border-b-0 lg:border-l"
      >
        <li v-for="item in sections" :key="item.id" class="shrink-0">
          <a
            :href="`#${item.id}`"
            :class="[
              '-mb-px flex h-10 items-center whitespace-nowrap border-b-2 px-3 text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-af-brand/40',
              'lg:-ml-px lg:mb-0 lg:h-auto lg:border-b-0 lg:border-l-2 lg:py-1.5',
              item.id === activeId
                ? 'border-af-brand font-medium text-af-ink'
                : 'border-transparent text-af-ink-3 hover:text-af-ink'
            ]"
            :aria-current="item.id === activeId ? 'location' : undefined"
            :data-testid="`form-page-nav-${item.id}`"
            @click.prevent="goTo(item.id)"
          >
            {{ item.label }}
          </a>
        </li>
      </ul>
    </nav>

    <div class="min-w-0 max-w-3xl flex-1">
      <div ref="bodyRef">
        <slot />
      </div>

      <div
        v-if="$slots.footer"
        class="sticky bottom-0 z-10 mt-8 border-t border-af-hairline bg-af-sheet py-3"
        data-testid="form-page-footer"
      >
        <slot name="footer" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue'

interface Props {
  show: boolean
  /** 与 BaseDialog 同名：页面标题在页头，这里只拿来给导航做 aria-label */
  title?: string
  /** 与 BaseDialog 同名，整页布局不用 */
  width?: string
}

interface SectionItem {
  id: string
  label: string
  el: HTMLElement
}

const props = defineProps<Props>()
// 与 BaseDialog 同一组事件；整页没有关闭按钮 / Esc，关闭由表单自己的「取消」触发
defineEmits<{ (e: 'close'): void }>()

const bodyRef = ref<HTMLElement | null>(null)
const sections = shallowRef<SectionItem[]>([])
const activeId = ref('')

// 滚动容器：管理站是整页滚动（AppLayout 没有 overflow 容器，顶栏 sticky），
// 这里仍然往上找最近的可滚动祖先，找不到就用 window，放进别的滚动容器里也能用。
let scrollTarget: HTMLElement | Window | null = null
let observer: MutationObserver | null = null
let scanFrame = 0
let scrollFrame = 0
// 点导航后的平滑滚动期间锁住高亮，免得最后几个矮分区滚不到顶时高亮落在前一个分区上
let lockedId = ''
let unlockTimer: ReturnType<typeof setTimeout> | undefined

function findScrollTarget(el: HTMLElement): HTMLElement | Window {
  let node = el.parentElement
  while (node && node !== document.body && node !== document.documentElement) {
    const { overflowY } = getComputedStyle(node)
    if ((overflowY === 'auto' || overflowY === 'scroll') && node.scrollHeight > node.clientHeight + 1) {
      return node
    }
    node = node.parentElement
  }
  return window
}

function scrollMetrics() {
  if (!scrollTarget || scrollTarget === window) {
    const doc = document.scrollingElement || document.documentElement
    return { top: 0, scrollTop: doc.scrollTop, clientHeight: window.innerHeight, scrollHeight: doc.scrollHeight }
  }
  const el = scrollTarget as HTMLElement
  return {
    top: el.getBoundingClientRect().top,
    scrollTop: el.scrollTop,
    clientHeight: el.clientHeight,
    scrollHeight: el.scrollHeight
  }
}

function updateActive() {
  const list = sections.value
  if (!list.length) {
    activeId.value = ''
    return
  }
  if (lockedId && list.some((item) => item.id === lockedId)) {
    activeId.value = lockedId
    return
  }
  const { top, scrollTop, clientHeight, scrollHeight } = scrollMetrics()
  let current = list[0].id
  for (const item of list) {
    // 标题滚到 scroll-margin-top 那条线（点导航时停的位置）就算进入这个分区
    const margin = parseFloat(getComputedStyle(item.el).scrollMarginTop) || 0
    if (item.el.getBoundingClientRect().top - top <= margin + 8) current = item.id
  }
  // 滚到底了：最后一个分区可能永远到不了那条线，直接算最后一个
  if (scrollTop > 0 && scrollTop + clientHeight >= scrollHeight - 2) current = list[list.length - 1].id
  activeId.value = current
}

function scan() {
  scanFrame = 0
  const root = bodyRef.value
  if (!root) {
    sections.value = []
    updateActive()
    return
  }
  const seen = new Set<string>()
  const next: SectionItem[] = []
  root.querySelectorAll<HTMLElement>('[data-form-section]').forEach((el) => {
    const id = el.dataset.formSection || ''
    if (!id || seen.has(id)) return
    seen.add(id)
    next.push({ id, label: el.dataset.formSectionLabel || el.textContent?.trim() || id, el })
  })
  const prev = sections.value
  const changed =
    prev.length !== next.length ||
    next.some((item, i) => item.id !== prev[i].id || item.label !== prev[i].label || item.el !== prev[i].el)
  if (changed) sections.value = next
  updateActive()
}

function scheduleScan() {
  if (scanFrame) return
  scanFrame = requestAnimationFrame(scan)
}

function unlock() {
  lockedId = ''
  if (unlockTimer) clearTimeout(unlockTimer)
  unlockTimer = undefined
}

function onScroll() {
  // 锁定期间：滚动停下 150ms 后解锁（平滑滚动结束），不重算，保留点中的那个
  if (lockedId) {
    if (unlockTimer) clearTimeout(unlockTimer)
    unlockTimer = setTimeout(unlock, 150)
    return
  }
  if (scrollFrame) return
  scrollFrame = requestAnimationFrame(() => {
    scrollFrame = 0
    updateActive()
  })
}

function goTo(id: string) {
  const item = sections.value.find((section) => section.id === id)
  if (!item) return
  lockedId = id
  activeId.value = id
  if (unlockTimer) clearTimeout(unlockTimer)
  // 已经在目标位置、不会再有滚动事件时也要解锁
  unlockTimer = setTimeout(unlock, 1000)
  item.el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const USER_SCROLL_EVENTS = ['wheel', 'touchstart', 'keydown'] as const

function attach() {
  const root = bodyRef.value
  if (!root) return
  scrollTarget = findScrollTarget(root)
  scrollTarget.addEventListener('scroll', onScroll, { passive: true })
  window.addEventListener('resize', onScroll, { passive: true })
  // 用户自己动了滚轮 / 触摸 / 键盘，立刻交还高亮
  USER_SCROLL_EVENTS.forEach((name) => window.addEventListener(name, unlock, { passive: true }))
  observer = new MutationObserver(scheduleScan)
  observer.observe(root, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ['data-form-section', 'data-form-section-label']
  })
  scan()
}

function detach() {
  scrollTarget?.removeEventListener('scroll', onScroll)
  window.removeEventListener('resize', onScroll)
  USER_SCROLL_EVENTS.forEach((name) => window.removeEventListener(name, unlock))
  scrollTarget = null
  observer?.disconnect()
  observer = null
  if (scanFrame) cancelAnimationFrame(scanFrame)
  if (scrollFrame) cancelAnimationFrame(scrollFrame)
  scanFrame = 0
  scrollFrame = 0
  unlock()
  sections.value = []
  activeId.value = ''
}

watch(
  () => props.show,
  async (show) => {
    detach()
    if (!show) return
    await nextTick()
    attach()
  },
  { immediate: true }
)

onBeforeUnmount(detach)
</script>
