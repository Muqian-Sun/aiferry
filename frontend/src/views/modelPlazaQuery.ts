/**
 * 模型目录（/model-plaza 接口）：进模型页前由路由预加载发出，页面挂载时接手（router/routePreload.ts）。
 * 概览给新用户写调用示例也用这份目录，同一个键。
 */
import { getModelPlaza } from '@/api/modelPlaza'
import { adoptPreloaded, preloadKey, type Prefetch } from '@/router/routePreload'

const MODEL_PLAZA_KEY = preloadKey('model-plaza')

export function preloadModelPlaza(prefetch: Prefetch): void {
  prefetch(MODEL_PLAZA_KEY, () => getModelPlaza())
}

/** 有预加载好的就直接用，否则现取 */
export function loadModelPlaza() {
  return adoptPreloaded(MODEL_PLAZA_KEY, () => getModelPlaza())
}
