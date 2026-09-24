import { computed, ref, unref, type MaybeRef } from 'vue'
import type { Column } from '@/components/common/types'

/**
 * 列表页列设置（A4）：管理站原来有四套各写各的实现（渠道在「更多」里、用户带版本号、订阅带用户列模式、
 * 用量分两个页签），这里收成一个。
 *
 * - 存的是「隐藏了哪些列」，不是「显示哪些列」：以后新增的列默认可见，不会因为本机存过旧列表而消失。
 * - 带版本号：改默认隐藏列时把 version 加一，本机旧设置作废、回到新默认，避免只有清过缓存的人才看到新默认。
 * - alwaysVisible 的列（如用户列、操作列）不能关，也不出现在菜单里。
 */
export interface ColumnSettingsOptions {
  storageKey: string
  version: number
  columns: MaybeRef<Column[]>
  defaultHidden?: string[]
  alwaysVisible?: string[]
}

interface StoredColumnSettings {
  version: number
  hidden: string[]
}

function readStored(storageKey: string, version: number): string[] | null {
  try {
    const raw = localStorage.getItem(storageKey)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<StoredColumnSettings>
    if (parsed?.version !== version || !Array.isArray(parsed.hidden)) return null
    return parsed.hidden.filter((key): key is string => typeof key === 'string')
  } catch {
    return null
  }
}

function writeStored(storageKey: string, version: number, hidden: string[]) {
  try {
    localStorage.setItem(storageKey, JSON.stringify({ version, hidden } satisfies StoredColumnSettings))
  } catch {
    // 隐私模式 / 存储被禁用：本次会话内仍然生效，只是不记住
  }
}

export function useColumnSettings(options: ColumnSettingsOptions) {
  const defaultHidden = options.defaultHidden ?? []
  const alwaysVisible = new Set(options.alwaysVisible ?? [])
  const hidden = ref<Set<string>>(new Set(readStored(options.storageKey, options.version) ?? defaultHidden))

  const isVisible = (key: string) => alwaysVisible.has(key) || !hidden.value.has(key)

  const visibleColumns = computed(() => unref(options.columns).filter((col) => isVisible(col.key)))

  /** 菜单里能开关的列（不含 alwaysVisible）。 */
  const toggleableColumns = computed(() => unref(options.columns).filter((col) => !alwaysVisible.has(col.key)))

  const isDefault = computed(() => {
    const defaults = new Set(defaultHidden)
    if (defaults.size !== hidden.value.size) return false
    for (const key of hidden.value) if (!defaults.has(key)) return false
    return true
  })

  const persist = () => writeStored(options.storageKey, options.version, [...hidden.value])

  const toggle = (key: string) => {
    if (alwaysVisible.has(key)) return
    const next = new Set(hidden.value)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    hidden.value = next
    persist()
  }

  const reset = () => {
    hidden.value = new Set(defaultHidden)
    persist()
  }

  return { visibleColumns, toggleableColumns, isVisible, toggle, reset, isDefault }
}

export type ColumnSettings = ReturnType<typeof useColumnSettings>
