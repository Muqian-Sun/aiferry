export interface NavItem {
  path: string
  label: string
  icon: unknown
  iconSvg?: string
  hideInSimpleMode?: boolean
  children?: NavItem[]
  /**
   * When true, the parent item only toggles the expand/collapse state and
   * does NOT navigate to its `path`. The `path` is purely a stable key.
   */
  expandOnly?: boolean
  /**
   * 可选的功能开关 getter。返回 false 时菜单项被隐藏；返回 undefined/true 时显示。
   * 宽容策略（undefined → 显示）避免 public settings 未加载完成时菜单闪烁消失。
   * Getter 里访问的 reactive 来源（store / composable）会被 computed 自动追踪，
   * 开关切换时菜单自动更新。
   */
  featureFlag?: () => boolean | undefined
  /** 元素 id，供新手引导定位（如 #sidebar-channel-manage）。 */
  elementId?: string
  /** data-tour 属性，供新手引导定位。 */
  dataTour?: string
}

export interface NavSection {
  key: string
  /** 分区标题；不传则不渲染标题。 */
  title?: string
  items: NavItem[]
}

// applyFeatureFlags 递归过滤掉 featureFlag() === false 的节点（含子节点）。
// 使用 `!== false` 宽容语义：undefined（设置未加载）或 true 都视为显示。
export function applyFeatureFlags(items: NavItem[]): NavItem[] {
  const out: NavItem[] = []
  for (const item of items) {
    if (item.featureFlag && item.featureFlag() === false) continue
    if (item.children) {
      out.push({ ...item, children: applyFeatureFlags(item.children) })
    } else {
      out.push(item)
    }
  }
  return out
}

/** 新手引导选择器：点击对应菜单项时推进引导步骤。 */
export function tourSelectorOf(item: Pick<NavItem, 'elementId' | 'dataTour'>): string | undefined {
  if (item.elementId) return `#${item.elementId}`
  if (item.dataTour) return `[data-tour="${item.dataTour}"]`
  return undefined
}
