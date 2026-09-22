import type { Directive } from 'vue'

/**
 * v-reveal：元素进入视口时标 `data-revealed`，淡入上浮由 style.css 的 `[data-reveal]` 规则负责。
 *
 * - 状态用 data 属性而不是 class：元素带 `:class` 绑定时，Vue 重渲染会整份覆盖 className，指令加的类会被抹掉；
 *   没绑定的 data-* 属性 Vue 不碰。
 * - 只播一次：显示后立刻 unobserve。
 * - 挂载时已经在视口里的（首屏），下一帧直接放，不等 IntersectionObserver：观察者回调要等一次绘制，
 *   页面刚打开又没人滚动时可能迟迟不来，首屏就一直是透明的（dev 上实测过）。
 * - `.stagger` 修饰符：`data-reveal="stagger"`，容器本身不动，子元素按 DOM 顺序写 `--reveal-i`，CSS 用它逐个错开延时。
 * - 值（可选，毫秒）：整体延时，写进 `--reveal-delay`。
 * - 没有 IntersectionObserver、或用户偏好减少动态效果：直接可见，不做动画——宁可不动，不能看不见。
 */
type RevealEl = HTMLElement & { __reveal?: () => void }

const OBSERVER_OPTIONS: IntersectionObserverInit = { threshold: 0.12, rootMargin: '0px 0px -8% 0px' }
let observer: IntersectionObserver | null = null

function getObserver(): IntersectionObserver {
  if (!observer) {
    observer = new IntersectionObserver((entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) (entry.target as RevealEl).__reveal?.()
      }
    }, OBSERVER_OPTIONS)
  }
  return observer
}

function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/** 是否已经在视口里（判据与 observer 的 rootMargin 对齐：底部收 8%） */
function inViewport(el: HTMLElement): boolean {
  const rect = el.getBoundingClientRect()
  const height = window.innerHeight || document.documentElement.clientHeight
  return rect.bottom > 0 && rect.top < height * 0.92
}

export const vReveal: Directive<RevealEl, number | undefined> = {
  mounted(el, binding) {
    const stagger = Boolean(binding.modifiers.stagger)
    el.dataset.reveal = stagger ? 'stagger' : 'single'
    if (stagger) {
      Array.from(el.children).forEach((child, index) => (child as HTMLElement).style.setProperty('--reveal-i', String(index)))
    }
    if (typeof binding.value === 'number') el.style.setProperty('--reveal-delay', `${binding.value}ms`)

    if (typeof IntersectionObserver === 'undefined' || prefersReducedMotion()) {
      el.dataset.revealed = ''
      return
    }
    el.__reveal = () => {
      el.dataset.revealed = ''
      getObserver().unobserve(el)
      delete el.__reveal
    }
    if (inViewport(el)) {
      // 下一帧再放，浏览器才会把 opacity 0 → 1 当成过渡来播（同一帧内设置不会有动画）
      requestAnimationFrame(() => el.__reveal?.())
      return
    }
    getObserver().observe(el)
  },
  unmounted(el) {
    if (el.__reveal) {
      observer?.unobserve(el)
      delete el.__reveal
    }
  }
}

/** 仅测试用：让下一次挂载重新建 observer（setup 里的 mock 每个用例可能不同） */
export function resetRevealObserverForTests(): void {
  observer = null
}
