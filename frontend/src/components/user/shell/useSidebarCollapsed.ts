import { ref } from 'vue'

const STORAGE_KEY = 'user-console-sidebar-collapsed'

function readSaved(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

/** 模块级单例：换页（壳重新挂载）时保持收起 / 展开状态 */
const collapsed = ref(readSaved())

/** 控制台侧栏收起为图标栏；偏好存在本浏览器，存不了（隐私模式等）也照常切换 */
export function useSidebarCollapsed() {
  function toggle() {
    collapsed.value = !collapsed.value
    try {
      localStorage.setItem(STORAGE_KEY, collapsed.value ? '1' : '0')
    } catch {
      // 只是偏好，存不下就算了
    }
  }
  return { collapsed, toggle }
}
