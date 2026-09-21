/**
 * 全站主题（明 / 暗）的唯一实现。
 *
 * 之前 bootstrap、侧栏、首页、密钥用量页各自读写 localStorage.theme 并切换 html.dark，
 * 彼此不知道对方的状态；这里收成一个模块级单例，所有消费者共享同一个 isDark。
 * 颜色本身由 styles/tokens.css 在 html.dark 下切换变量，组件不需要再写 dark: 变体。
 */
import { readonly, ref } from 'vue'

const STORAGE_KEY = 'theme'
const DARK_QUERY = '(prefers-color-scheme: dark)'

const isDark = ref(false)
let initialized = false

function systemPrefersDark(): boolean {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function' && window.matchMedia(DARK_QUERY).matches
}

function readSavedTheme(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

/** 启动时应当使用的主题：有保存值按保存值，否则跟随系统。 */
export function resolveInitialTheme(): boolean {
  const saved = readSavedTheme()
  return saved === 'dark' || (!saved && systemPrefersDark())
}

/** 切换到指定主题；persist=false 用于跟随系统时不写入用户偏好。 */
export function applyTheme(dark: boolean, persist = true): void {
  isDark.value = dark
  document.documentElement.classList.toggle('dark', dark)
  if (!persist) return
  try {
    localStorage.setItem(STORAGE_KEY, dark ? 'dark' : 'light')
  } catch {
    /* 隐私模式等场景写不进去也不影响当前页 */
  }
}

/** 在 app 挂载前调用一次，避免首屏闪烁；重复调用无副作用。 */
export function initTheme(): void {
  if (initialized) return
  initialized = true
  applyTheme(resolveInitialTheme(), false)
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return
  const media = window.matchMedia(DARK_QUERY)
  // 用户没有显式选择时跟随系统切换
  media.addEventListener?.('change', (event) => {
    if (!readSavedTheme()) applyTheme(event.matches, false)
  })
}

export function useTheme() {
  if (!initialized) initTheme()
  return {
    isDark: readonly(isDark),
    toggleTheme: () => applyTheme(!isDark.value),
    setTheme: (dark: boolean) => applyTheme(dark)
  }
}
