/**
 * 弹层栈：BaseDialog 与 DetailDrawer 共用，叠着的弹层按打开顺序排成一摞（2026-10-04 体验诊断 H1–H3）。
 *
 * - Esc 只关最上面一层。原来每个弹窗各自在 document 上监听 Esc，二级确认里按一次 Esc 会把外层带表单的弹窗一起关掉。
 * - 滚动锁按层数计，最后一层关掉才解锁。原来内层一关就摘掉 body.modal-open，外层还开着背后就能滚。
 * - Tab / Shift+Tab 在最上层里循环，不跑到背后的页面；开着弹层时页面主体（#app）设 inert，读屏也不会读到背后。
 * - 打开时把焦点放进弹层，关掉后还给打开前的元素。
 *
 * 弹层里的下拉 / 菜单 Teleport 到 body（role=listbox / menu）：它们开着时 Esc 与 Tab 归它们自己处理。
 * 监听挂在 window 的冒泡阶段，排在 document 上各组件自己的 Esc 处理之后；谁处理了 Esc 就 preventDefault，这里不再关弹层。
 */
import { nextTick, onBeforeUnmount, watch } from 'vue'

interface Layer {
  id: symbol
  panel: () => HTMLElement | null
  onEscape: (event: KeyboardEvent) => void
}

interface ModalLayerOptions {
  open: () => boolean
  /** 弹层面板：焦点在它里面循环 */
  panel: () => HTMLElement | null
  /** 最上层收到 Esc 时调用（是否真的关由调用方决定） */
  onEscape: (event: KeyboardEvent) => void
  /** 打开后先聚焦谁；不给或返回 null 时聚焦面板本身（面板要有 tabindex="-1"） */
  initialFocus?: () => HTMLElement | null
}

const layers: Layer[] = []

const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
  '[contenteditable="true"]'
].join(',')

const FLOATING = 'body > [role="menu"], body > [role="listbox"]'

function focusableIn(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => el.getClientRects().length > 0 && !el.closest('[inert]')
  )
}

function trapTab(event: KeyboardEvent, panel: HTMLElement) {
  const active = document.activeElement as HTMLElement | null
  // 焦点在 Teleport 出去的下拉里：交给下拉自己（它在 Tab 时会收起）
  if (active?.closest('[role="listbox"], [role="menu"]') && !panel.contains(active)) return
  const items = focusableIn(panel)
  if (items.length === 0) {
    event.preventDefault()
    panel.focus()
    return
  }
  const first = items[0]
  const last = items[items.length - 1]
  const inside = !!active && panel.contains(active)
  if (event.shiftKey && (!inside || active === first)) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && (!inside || active === last)) {
    event.preventDefault()
    first.focus()
  }
}

function onKeydown(event: KeyboardEvent) {
  const top = layers[layers.length - 1]
  if (!top || event.defaultPrevented) return
  if (event.key === 'Escape') {
    if (document.querySelector(FLOATING)) return
    top.onEscape(event)
    return
  }
  if (event.key === 'Tab') {
    const panel = top.panel()
    if (panel) trapTab(event, panel)
  }
}

function setBackgroundInert(inert: boolean) {
  const app = document.getElementById('app')
  if (!app) return
  if (inert) app.setAttribute('inert', '')
  else app.removeAttribute('inert')
}

function push(layer: Layer) {
  if (layers.length === 0) {
    window.addEventListener('keydown', onKeydown)
    document.body.classList.add('modal-open')
    setBackgroundInert(true)
  }
  layers.push(layer)
}

function remove(id: symbol): boolean {
  const index = layers.findIndex((layer) => layer.id === id)
  if (index < 0) return false
  layers.splice(index, 1)
  if (layers.length === 0) {
    window.removeEventListener('keydown', onKeydown)
    document.body.classList.remove('modal-open')
    setBackgroundInert(false)
  }
  return true
}

export function useModalLayer(options: ModalLayerOptions) {
  const id = Symbol('modal-layer')
  let returnFocus: HTMLElement | null = null

  watch(
    options.open,
    async (isOpen) => {
      if (isOpen) {
        if (layers.some((layer) => layer.id === id)) return
        returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
        push({ id, panel: options.panel, onEscape: options.onEscape })
        await nextTick()
        const target = options.initialFocus?.() ?? options.panel()
        target?.focus({ preventScroll: true })
      } else if (remove(id)) {
        // 先解除背景 inert 再还焦点，否则 focus() 落不到 inert 区域里
        returnFocus?.focus?.({ preventScroll: true })
        returnFocus = null
      }
    },
    { immediate: true }
  )

  onBeforeUnmount(() => {
    remove(id)
  })

  return {
    isTop: () => layers[layers.length - 1]?.id === id
  }
}
