/**
 * 模型页与服务状态页既是公开页，也能留在控制台里看（muqian 2026-09-30：从控制台点进来不跳离控制台，区分从首页跳转）。
 *
 * 控制台顶栏（含窄屏第二行）的页签跳转时，给这条浏览记录带上 CONSOLE_SHELL_STATE，页面挂载时按它选壳。
 * 标记存在浏览记录里：刷新、前进后退都保持；从首页等公开页点进来、直接打开链接、新标签打开都没有标记，走公开壳。
 */
import { computed, type ComputedRef } from 'vue'
import { useAuthStore } from '@/stores/auth'

export const CONSOLE_SHELL_STATE = { shell: 'console' } as const

/** 从控制台点进来且仍登录着 → 控制台壳，否则公开壳。读的是挂载时这条浏览记录上的标记 */
export function useShellVariant(): ComputedRef<'console' | 'public'> {
  const authStore = useAuthStore()
  const fromConsole = window.history.state?.shell === CONSOLE_SHELL_STATE.shell
  return computed(() => (fromConsole && authStore.isAuthenticated ? 'console' : 'public'))
}
