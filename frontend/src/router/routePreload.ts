/**
 * 数据到了再换页（muqian 2026-09-27：点侧栏进充值 / 用量明细会先闪一下加载态再换成正文）。
 *
 * 路由 meta.preload 在导航确认前把页面首屏要的请求发出去（prefetch），导航等这些请求落地才切页，旧页面保持不动；
 * 页面挂载后照常调自己的加载函数，只是把同名同参的请求换成已经落地的那份（adoptPreloaded）。
 * 导航确认 → 新页面挂载 → 接手结果都在同一个任务的微任务里完成，浏览器没有机会把中间的加载态画出来。
 * 请求对不上（参数不同）或预加载失败时页面自己再发一次：最多多一个请求，不会拿错数据。
 */
import type { RouteLocationNormalized } from 'vue-router'

/**
 * 最多等多久：超过就先换页，页面显示自己的加载态、接着等同一批请求。
 * 拍的（1 秒）：本机首屏接口几十毫秒；线上慢查询（如大范围用量统计）时不让点击后长时间没反应，按手感再调。
 */
const PRELOAD_WAIT_MS = 1000

let pending = new Map<string, Promise<unknown>>()

/** 预加载与页面两边用同一个键：请求名 + 参数 */
export function preloadKey(name: string, params?: unknown): string {
  return params === undefined ? name : `${name}:${JSON.stringify(params)}`
}

/**
 * meta.preload 拿到的登记函数：登记一个首屏请求并返回它，要按它的结果再发下一个时（如按密钥列表查用量）可以 await。
 * 绑定在这一次导航上：慢的预加载后登记的请求不会混进后一次导航。
 */
export type Prefetch = <T>(key: string, request: () => Promise<T>) => Promise<T>

/** 页面加载函数里用：有同名同参的预加载请求就接过来（只接一次），否则自己发 */
export function adoptPreloaded<T>(key: string, request: () => Promise<T>): Promise<T> {
  const hit = pending.get(key)
  if (!hit) return request()
  pending.delete(key)
  return hit as Promise<T>
}

/** 导航守卫（beforeResolve）里调：进入新页面时跑它的 preload，等请求落地或到上限 */
export async function runRoutePreload(to: RouteLocationNormalized, from: RouteLocationNormalized): Promise<void> {
  const requests = new Map<string, Promise<unknown>>()
  pending = requests
  // 同一页面只改地址栏参数（筛选写回 query）不算进入新页面，页面自己加载
  if (!to.meta.preload || to.matched.at(-1) === from.matched.at(-1)) return

  const preload = to.meta.preload
  const prefetch: Prefetch = (key, request) => {
    const promise = request()
    requests.set(key, promise)
    return promise
  }
  const settled = (async () => {
    try {
      // preload 自己可能 await 前一个请求再登记下一个；它返回后登记的请求就全了
      await preload(to, prefetch)
    } catch (error) {
      console.warn('Route preload failed:', error)
    }
    await Promise.allSettled(requests.values())
  })()
  let timer: ReturnType<typeof setTimeout> | undefined
  await Promise.race([
    settled,
    new Promise((resolve) => {
      timer = setTimeout(resolve, PRELOAD_WAIT_MS)
    })
  ])
  clearTimeout(timer)
  // 接手只限这次挂载：下一个任务就作废没接走的，免得之后同参数的刷新拿到旧数据
  setTimeout(() => requests.clear(), 0)
}
