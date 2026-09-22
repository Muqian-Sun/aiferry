import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'

import { resetRevealObserverForTests, vReveal } from '../reveal'

type IOCallback = (entries: Array<{ isIntersecting: boolean; target: Element }>) => void

/** 能手动触发回调的 IntersectionObserver 假件；observe / unobserve 可断言 */
function installObserverMock() {
  const state = { callback: null as IOCallback | null, observe: vi.fn(), unobserve: vi.fn() }
  class FakeObserver {
    constructor(callback: IOCallback) {
      state.callback = callback
    }
    observe = state.observe
    unobserve = state.unobserve
    disconnect = vi.fn()
  }
  vi.stubGlobal('IntersectionObserver', FakeObserver)
  return state
}

function mountWith(template: string) {
  const Comp = defineComponent({
    directives: { reveal: vReveal },
    template
  })
  return mount(Comp)
}

const Cards = defineComponent({
  directives: { reveal: vReveal },
  render() {
    return h('ul', { 'data-testid': 'list' }, [h('li', 'a'), h('li', 'b'), h('li', 'c')])
  }
})

describe('v-reveal', () => {
  beforeEach(() => {
    resetRevealObserverForTests()
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false })))
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('hides the element until the observer reports it in view, then shows it once and stops observing', () => {
    const io = installObserverMock()
    const wrapper = mountWith('<section v-reveal data-testid="box">hi</section>')
    const el = wrapper.get('[data-testid="box"]').element
    expect(el.getAttribute('data-reveal')).toBe('single')
    expect(el.hasAttribute('data-revealed')).toBe(false)
    expect(io.observe).toHaveBeenCalledWith(el)

    io.callback!([{ isIntersecting: false, target: el }])
    expect(el.hasAttribute('data-revealed')).toBe(false)

    io.callback!([{ isIntersecting: true, target: el }])
    expect(el.hasAttribute('data-revealed')).toBe(true)
    expect(io.unobserve).toHaveBeenCalledWith(el)
  })

  it('reveals on the next frame, without waiting for the observer, when the element is already on screen', () => {
    const io = installObserverMock()
    vi.stubGlobal('innerHeight', 800)
    // jsdom 的 rect 默认全 0（视为不在视口）；这里假装元素就在首屏
    const rect = vi.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue({ top: 40, bottom: 400 } as DOMRect)
    const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb: FrameRequestCallback) => {
      cb(0)
      return 1
    })
    const wrapper = mountWith('<section v-reveal data-testid="box">hi</section>')
    expect(wrapper.get('[data-testid="box"]').element.hasAttribute('data-revealed')).toBe(true)
    expect(io.observe).not.toHaveBeenCalled()
    rect.mockRestore()
    raf.mockRestore()
  })

  it('numbers the children of a .stagger container so CSS can offset each one', () => {
    installObserverMock()
    const wrapper = mount(defineComponent({
      directives: { reveal: vReveal },
      template: '<ul v-reveal.stagger="120" data-testid="list"><li>a</li><li>b</li><li>c</li></ul>'
    }))
    const list = wrapper.get('[data-testid="list"]').element as HTMLElement
    expect(list.getAttribute('data-reveal')).toBe('stagger')
    expect(list.style.getPropertyValue('--reveal-delay')).toBe('120ms')
    const indexes = Array.from(list.children).map((child) => (child as HTMLElement).style.getPropertyValue('--reveal-i'))
    expect(indexes).toEqual(['0', '1', '2'])
  })

  it('shows immediately when the user prefers reduced motion, without observing', () => {
    const io = installObserverMock()
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true })))
    const wrapper = mountWith('<div v-reveal data-testid="box">hi</div>')
    const el = wrapper.get('[data-testid="box"]').element
    expect(el.hasAttribute('data-revealed')).toBe(true)
    expect(io.observe).not.toHaveBeenCalled()
  })

  it('shows immediately when IntersectionObserver is unavailable', () => {
    vi.stubGlobal('IntersectionObserver', undefined)
    const wrapper = mountWith('<div v-reveal data-testid="box">hi</div>')
    expect(wrapper.get('[data-testid="box"]').element.hasAttribute('data-revealed')).toBe(true)
  })

  it('survives a :class re-render on the same element (Vue rewrites className wholesale)', async () => {
    const io = installObserverMock()
    const wrapper = mount(defineComponent({
      directives: { reveal: vReveal },
      data: () => ({ wide: false }),
      template: '<ul v-reveal.stagger :class="wide ? \'is-wide\' : \'is-narrow\'" data-testid="list"><li>a</li></ul>'
    }))
    const el = wrapper.get('[data-testid="list"]').element as HTMLElement
    await wrapper.setData({ wide: true })
    expect(el.className).toBe('is-wide')
    expect(el.getAttribute('data-reveal')).toBe('stagger')
    io.callback!([{ isIntersecting: true, target: el }])
    await wrapper.setData({ wide: false })
    expect(el.hasAttribute('data-revealed')).toBe(true)
  })

  it('stops observing an element that unmounts before it was revealed', async () => {
    const io = installObserverMock()
    const wrapper = mount(defineComponent({
      components: { Cards },
      data: () => ({ show: true }),
      template: '<div><Cards v-if="show" v-reveal /></div>',
      directives: { reveal: vReveal }
    }))
    const el = wrapper.get('[data-testid="list"]').element
    expect(io.observe).toHaveBeenCalledWith(el)
    await wrapper.setData({ show: false })
    expect(io.unobserve).toHaveBeenCalledWith(el)
  })
})
