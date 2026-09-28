export interface NavItem {
  path: string
  label: string
  icon: unknown
  iconSvg?: string
  hideInSimpleMode?: boolean
  /**
   * 可选的功能开关 getter。返回 false 时菜单项置灰（disabled），点进去是「未开启 · 去设置打开」；
   * 返回 undefined/true 时正常显示。宽容策略（undefined → 正常）避免 public settings 未加载完成时菜单闪烁。
   * Getter 里访问的 reactive 来源（store / composable）会被 computed 自动追踪，开关切换时菜单自动更新。
   */
  featureFlag?: () => boolean | undefined
  /**
   * 由代码或部署配置决定的功能（支付、运维监控、渠道监控）：返回 false 时入口直接不出现，不置灰——
   * 后台没有开关能打开它（上线收口方案 2026-09-26「代码关掉的功能，入口直接不出现」）。
   * 同样用 `=== false` 宽容语义，设置未加载时照常显示，避免闪烁。
   */
  presentWhen?: () => boolean | undefined
  /** 同组的其它页面路径：在这些页面上也点亮本项（如「订阅」在 /orders/plans 上也是当前项） */
  activePaths?: string[]
  /** 功能未开启：由 applyFeatureFlags 标上，侧栏显示为灰色入口 */
  disabled?: boolean
}

export interface NavSection {
  key: string
  /** 分区标题；不传则不渲染标题。 */
  title?: string
  items: NavItem[]
}

// applyFeatureFlags 给 featureFlag() === false 的项标上 disabled（管理站改造方案 A3：没开的功能不再整个消失，
// 显示为灰色入口，免得管理员找不到）。使用 `=== false` 宽容语义：undefined（设置未加载）或 true 都视为开启。
export function applyFeatureFlags(items: NavItem[]): NavItem[] {
  return items
    .filter((item) => item.presentWhen?.() !== false)
    .map((item) => (item.featureFlag?.() === false ? { ...item, disabled: true } : item))
}
